package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"magistrala-agri-agent/workbench"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkbenchReadOnlyPostAndManagedSimulation(t *testing.T) {
	token := "Bearer h." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"reader"}`)) + ".s"
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if strings.Contains(in.Query, "authzCheck") {
			_, _ = w.Write([]byte(`{"data":{"authzCheck":{"allowed":false}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"tenant":{"id":"w"}}}`))
	}))
	defer platform.Close()
	s := NewServer(&Engine{})
	s.Platform = &PlatformAuth{URL: platform.URL}
	if err := s.EnableWorkbench(workbench.Config{KnowledgePath: "examples/workbench-knowledge.json"}, "", "seconds"); err != nil {
		t.Fatal(err)
	}
	call := func(path, auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(`{"task":"qa","question":"气压单位"}`))
		r.Header.Set("Authorization", auth)
		r.Header.Set("X-Workspace-ID", "w")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := call("/v2/workbench/runs", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("/v2/workbench/runs", token); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := call("/v2/workbench/plans", token); w.Code != 403 {
		t.Fatal("management permission bypassed", w.Code, w.Body)
	}
}

func TestBoundedReaderFiltersTimeAndPublisher(t *testing.T) {
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	end := start.Add(time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") == "" || r.URL.Query().Get("to") == "" {
			t.Fatal("missing remote time bounds")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]any{
			{"publisher": "d", "channel": "c", "name": "temp", "time": start.Unix(), "value": 30},
			{"publisher": "d", "channel": "c", "name": "temp", "time": end.Unix(), "value": 99},
			{"publisher": "foreign", "channel": "c", "name": "temp", "time": start.Unix(), "value": 99},
		}})
	}))
	defer server.Close()
	rows, err := (&Reader{BaseURL: server.URL, TimeUnit: "seconds", Until: end}).Read(context.Background(), "w", Binding{ChannelID: "c", PublisherID: "d"}, start)
	if err != nil || len(rows) != 1 || rows[0].Value != 30 {
		b, _ := json.Marshal(rows)
		t.Fatal(err, bytes.NewBuffer(b))
	}
}
