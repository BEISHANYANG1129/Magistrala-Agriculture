package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type sourceFunc func(context.Context, string, Binding, time.Time) ([]Evidence, error)

func (f sourceFunc) Read(c context.Context, w string, b Binding, s time.Time) ([]Evidence, error) {
	return f(c, w, b, s)
}

type analystFunc func(context.Context, string, Region, string, []Evidence, []Knowledge) (Assessment, error)

func (f analystFunc) Analyze(c context.Context, x string, r Region, v string, e []Evidence, k []Knowledge) (Assessment, error) {
	return f(c, x, r, v, e, k)
}

func testEngine(t *testing.T) *Engine {
	t.Helper()
	b, err := os.ReadFile("examples/registry.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "registry.json")
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 4, 0, 0, 0, time.UTC)
	return &Engine{RegistryPath: path, Source: DemoSource{Now: func() time.Time { return now }}, Analyst: DemoAnalyst{}, MaxAge: 15 * time.Minute, Now: func() time.Time { return now }, Mode: "demo"}
}
func northRequest() Request {
	return Request{WorkspaceID: "demo-farm", RegionID: "north", MappingVersion: "demo-v1"}
}
func TestRegionsStayIsolated(t *testing.T) {
	e := testEngine(t)
	north, err := e.Analyze(context.Background(), northRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(north.Tasks) != 1 || len(north.Evidence) != 1 || north.Tasks[0].Target != "north" || north.Tasks[0].ExecutorIDs[0] != "valve-north" {
		t.Fatalf("invalid north result: %+v", north)
	}
	req := northRequest()
	req.RegionID = "south"
	south, err := e.Analyze(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(south.Tasks) != 0 || south.Evidence[0].PublisherID != "sensor-south" {
		t.Fatalf("region leakage: %+v", south)
	}
}
func TestInvalidScopeRejectedBeforeReading(t *testing.T) {
	for _, req := range []Request{{"other-farm", "north", "demo-v1"}, {"demo-farm", "unknown", "demo-v1"}, {"demo-farm", "north", "old"}} {
		t.Run(fmt.Sprint(req), func(t *testing.T) {
			e := testEngine(t)
			e.Source = sourceFunc(func(context.Context, string, Binding, time.Time) ([]Evidence, error) {
				t.Fatal("source must not be called")
				return nil, nil
			})
			if _, err := e.Analyze(context.Background(), req); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
func TestMissingStaleAndFutureEvidenceAvoidModel(t *testing.T) {
	for _, delta := range []time.Duration{-time.Hour, time.Hour} {
		t.Run(delta.String(), func(t *testing.T) {
			e := testEngine(t)
			e.Source = DemoSource{Now: func() time.Time { return e.now().Add(delta) }}
			e.Analyst = analystFunc(func(context.Context, string, Region, string, []Evidence, []Knowledge) (Assessment, error) {
				t.Fatal("model called with invalid evidence")
				return Assessment{}, nil
			})
			r, err := e.Analyze(context.Background(), northRequest())
			if err != nil || r.Status != "insufficient_data" || len(r.Tasks) != 0 {
				t.Fatalf("unexpected result %+v %v", r, err)
			}
		})
	}
}
func TestModelCannotChangeRegionOrInventEvidence(t *testing.T) {
	for _, change := range []string{"region", "evidence", "kind", "knowledge"} {
		t.Run(change, func(t *testing.T) {
			e := testEngine(t)
			e.Analyst = analystFunc(func(c context.Context, x string, r Region, v string, rows []Evidence, k []Knowledge) (Assessment, error) {
				a, _ := (DemoAnalyst{}).Analyze(c, x, r, v, rows, k)
				switch change {
				case "region":
					a.RegionID = "south"
				case "evidence":
					a.Findings[0].EvidenceIDs = []string{"other-region-message"}
				case "kind":
					a.Findings[0].Kind = "open_valve"
				case "knowledge":
					a.KnowledgeIDs = []string{"invented"}
				}
				return a, nil
			})
			if _, err := e.Analyze(context.Background(), northRequest()); err == nil {
				t.Fatal("untrusted model result accepted")
			}
		})
	}
}
func TestMappingChangesDuringInferenceRejectedEvenWithoutVersionChange(t *testing.T) {
	e := testEngine(t)
	e.Analyst = analystFunc(func(c context.Context, x string, r Region, v string, rows []Evidence, k []Knowledge) (Assessment, error) {
		b, _ := os.ReadFile(e.RegistryPath)
		b = []byte(strings.ReplaceAll(string(b), "valve-north", "replacement-valve"))
		if err := os.WriteFile(e.RegistryPath, b, 0600); err != nil {
			t.Fatal(err)
		}
		return (DemoAnalyst{}).Analyze(c, x, r, v, rows, k)
	})
	if _, err := e.Analyze(context.Background(), northRequest()); err == nil {
		t.Fatal("stale mapping accepted")
	}
}
func TestRegistryRejectsSharedExecutorAllowsSharedSensor(t *testing.T) {
	reg, _, err := LoadRegistry("examples/registry.json")
	if err != nil {
		t.Fatal(err)
	}
	reg.Regions[1].Sensors = reg.Regions[0].Sensors
	if err = reg.Validate(); err != nil {
		t.Fatal("shared sensors should be allowed:", err)
	}
	reg.Regions[1].Executors = reg.Regions[0].Executors
	if err = reg.Validate(); err == nil {
		t.Fatal("shared executor should be rejected")
	}
}
func TestReaderPaginationIsolationAndFractionalSeconds(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if r.URL.Path != "/base/farm/channels/channel/messages" || r.Header.Get("Authorization") != "Client test-key" || r.URL.Query().Get("publisher") != "sensor" {
			t.Errorf("wrong request %s", r.URL)
		}
		if r.URL.Query().Get("offset") == "0" {
			fmt.Fprint(w, `{"total":3,"messages":[{"channel":"channel","publisher":"other-region","name":"soil_moisture","unit":"%","value":1,"time":1791432000.5},{"channel":"channel","publisher":"sensor","name":"soil_moisture","unit":"%","value":0,"time":1791432000.25}]}`)
		} else {
			fmt.Fprint(w, `{"total":3,"messages":[{"channel":"channel","publisher":"sensor","name":"soil_temperature","unit":"Cel","value":21,"time":1791432000.75}]}`)
		}
	}))
	defer server.Close()
	reader := Reader{BaseURL: server.URL + "/base", Authorization: "Client test-key", TimeUnit: "seconds", PageSize: 2}
	rows, err := reader.Read(context.Background(), "farm", Binding{"channel", "sensor"}, time.Unix(1791431990, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || count != 2 || rows[0].Value != 0 || rows[0].ObservedAt.Nanosecond() != 250000000 {
		t.Fatalf("invalid rows %+v requests %d", rows, count)
	}
}
func TestReaderRejectsMalformedEnvelopesAndTimeUnits(t *testing.T) {
	for _, body := range []string{`{}`, `{"messages":[{"publisher":"sensor","name":"soil_moisture","value":10,"time":1791432000000000000}]}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			reader := Reader{BaseURL: server.URL, TimeUnit: "seconds"}
			if _, err := reader.Read(context.Background(), "farm", Binding{"channel", "sensor"}, time.Time{}); err == nil {
				t.Fatal("malformed data accepted")
			}
		})
	}
}
func TestLangChainOllamaAdapterUsesStructuredResponse(t *testing.T) {
	e := testEngine(t)
	calls := 0
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var req struct {
			Model     string          `json:"model"`
			Format    json.RawMessage `json:"format"`
			KeepAlive string          `json:"keep_alive"`
			Options   struct {
				NumCtx int `json:"num_ctx"`
			} `json:"options"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Type     string   `json:"type"`
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(req.Format, &schema); err != nil {
			t.Fatal(err)
		}
		if req.Model != "fixture-model" || schema.Type != "object" || len(schema.Required) != 4 || req.KeepAlive != "0" || req.Options.NumCtx != 8192 || len(req.Messages) != 2 {
			t.Errorf("invalid LangChain request %+v", req)
		}
		calls++
		content := `{"region_id":"north","mapping_version":"demo-v1","findings":[{"kind":"soil_dry","explanation":"fixture response","evidence_ids":["demo-sensor-north"]}],"knowledge_ids":[]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture-model", "message": map[string]string{"role": "assistant", "content": content}, "done": true})
	}))
	defer modelServer.Close()
	analyst, err := NewOllama(modelServer.URL, "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	e.Analyst = analyst
	result, err := e.Analyze(context.Background(), northRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 1 || calls != 1 {
		t.Fatal("adapter workflow failed")
	}
}
func post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestSimulationIsIdempotentAndNeverCallsDevices(t *testing.T) {
	s := NewServer(testEngine(t))
	h := s.Handler()
	w := post(t, h, "/v1/analyze", `{"workspace_id":"demo-farm","region_id":"north","mapping_version":"demo-v1"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result Result
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	body := fmt.Sprintf(`{"analysis_id":%q}`, result.AnalysisID)
	first := post(t, h, "/v1/simulate", body)
	second := post(t, h, "/v1/simulate", body)
	if first.Code != 200 || first.Body.String() != second.Body.String() || !strings.Contains(first.Body.String(), "simulated_completed") {
		t.Fatal(first.Body.String(), second.Body.String())
	}
}
func TestSimulationRejectsStaleMappingAndExpiredEvidence(t *testing.T) {
	for _, change := range []string{"mapping", "time"} {
		t.Run(change, func(t *testing.T) {
			e := testEngine(t)
			s := NewServer(e)
			h := s.Handler()
			w := post(t, h, "/v1/analyze", `{"workspace_id":"demo-farm","region_id":"north","mapping_version":"demo-v1"}`)
			var result Result
			_ = json.Unmarshal(w.Body.Bytes(), &result)
			if change == "mapping" {
				b, _ := os.ReadFile(e.RegistryPath)
				_ = os.WriteFile(e.RegistryPath, append(b, ' '), 0600)
			} else {
				now := e.now()
				e.Now = func() time.Time { return now.Add(time.Hour) }
			}
			w = post(t, h, "/v1/simulate", fmt.Sprintf(`{"analysis_id":%q}`, result.AnalysisID))
			if w.Code != 409 {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}
func TestHTTPRejectsUnknownFieldsAndCrossOrigin(t *testing.T) {
	h := NewServer(testEngine(t)).Handler()
	w := post(t, h, "/v1/analyze", `{"workspace_id":"demo-farm","region_id":"north","mapping_version":"demo-v1","execute":true}`)
	if w.Code != 400 {
		t.Fatal("extra field accepted")
	}
	r := httptest.NewRequest("POST", "http://localhost/v1/analyze", strings.NewReader("{}"))
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin write accepted")
	}
}
