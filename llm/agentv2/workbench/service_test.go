package workbench

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, model Model) *Service {
	t.Helper()
	s, err := New(Config{KnowledgePath: "../examples/workbench-knowledge.json", StateDir: t.TempDir(), Model: model})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour).Truncate(time.Second)
	s.dataset = &Dataset{Version: "fixture", WorkspaceID: "w", TimeKind: "receiver_time_not_sensor_sampling_time", Metrics: []Metric{
		{Key: "air_temperature", Label: "温度", Unit: "Cel", Admission: "include"},
		{Key: "co2", Label: "二氧化碳", Unit: "ppm", Admission: "include_with_quality_flag"},
		{Key: "pm25", Admission: "hold_for_validation"},
		{Key: "air_pressure", UnitConfirmation: "pending", Admission: "include"},
		{Key: "wind_direction", Admission: "include"},
	}, Frames: []Frame{{ID: "a", ReceivedAt: now, Values: map[string]float64{"air_temperature": 30, "co2": 350, "pm25": 437, "air_pressure": 101, "wind_direction": 359}}, {ID: "b", ReceivedAt: now.Add(time.Minute), Values: map[string]float64{"air_temperature": 32, "co2": 450, "pm25": 437, "air_pressure": 101, "wind_direction": 1}}}}
	return s
}
func call(t *testing.T, s *Service, method, path, workspace string, body any) (int, []byte) {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://localhost/v2/workbench"+path, bytes.NewReader(b))
	w := httptest.NewRecorder()
	s.Handle(w, r, workspace, "Bearer test")
	return w.Code, w.Body.Bytes()
}
func fixtureQuery(s *Service) Query {
	return Query{Mode: "history", Start: s.dataset.Frames[0].ReceivedAt, End: s.dataset.Frames[1].ReceivedAt.Add(time.Second)}
}
func TestEvidenceStatisticsAndIsolation(t *testing.T) {
	s := fixture(t, nil)
	q := fixtureQuery(s)
	code, b := call(t, s, "POST", "/query", "w", q)
	if code != 200 {
		t.Fatal(string(b))
	}
	var e Evidence
	_ = json.Unmarshal(b, &e)
	if e.Rows != 2 || len(e.Metrics) != 5 {
		t.Fatal(e)
	}
	for _, m := range e.Metrics {
		switch m.Key {
		case "air_temperature":
			if m.Mean == nil || *m.Mean != 31 {
				t.Fatal(m)
			}
		case "co2":
			if m.BelowRange != 1 {
				t.Fatal(m)
			}
		case "pm25", "air_pressure", "wind_direction":
			if m.Mean != nil {
				t.Fatalf("ineligible mean: %+v", m)
			}
		}
	}
	if code, _ = call(t, s, "POST", "/query", "other", q); code != 422 {
		t.Fatal("cross-workspace dataset leaked")
	}
	code, b = call(t, s, "GET", "/status", "other", nil)
	if code != 200 || strings.Contains(string(b), "station_id") {
		t.Fatal(string(b))
	}
	q.Start = q.End
	q.End = q.End.Add(time.Minute)
	code, b = call(t, s, "POST", "/query", "w", q)
	_ = json.Unmarshal(b, &e)
	if code != 200 || e.Rows != 0 || len(e.Warnings) == 0 {
		t.Fatal(string(b))
	}
	q.End = q.Start.Add(8 * 24 * time.Hour)
	if code, _ = call(t, s, "POST", "/query", "w", q); code != 422 {
		t.Fatal("unbounded window accepted")
	}
}
func TestRetrievalScopeAndPairing(t *testing.T) {
	all := []Chunk{{ID: "a", Title: "咖啡病害", Text: "咖啡叶片", Status: "reviewed", Crop: "coffee"}, {ID: "b", Title: "棉花病害", Text: "棉花叶片", Status: "reviewed", Crop: "cotton"}, {ID: "c", Title: "咖啡病害", Status: "reviewed", WorkspaceID: "secret"}, {ID: "d", Title: "咖啡病害", Status: "draft"}}
	got := retrieve(all, "咖啡叶片病害", "w", "coffee")
	if len(got) != 1 || got[0].ID != "a" || got[0].Text != "咖啡叶片" {
		t.Fatal(got)
	}
	if len(retrieve(all, "咖啡叶片病害", "w", "")) != 0 {
		t.Fatal("unknown crop retrieved crop-specific material")
	}
}

type fakeModel struct {
	calls  int
	bad    bool
	review bool
	fail   bool
	image  bool
}

func (m *fakeModel) Generate(ctx context.Context, msg []Message, schema any, dst any) error {
	m.calls++
	if m.fail {
		return fmt.Errorf("model unavailable")
	}
	if m.image && len(msg[len(msg)-1].Images) == 0 {
		return fmt.Errorf("image missing")
	}
	switch v := dst.(type) {
	case *ModelOutput:
		c := Claim{Text: "露点温度是空气冷却到饱和时的温度。", Kind: "common", Citations: []string{}}
		if m.bad {
			c.Kind = "knowledge"
			c.Citations = []string{"fabricated"}
		}
		*v = ModelOutput{Claims: []Claim{c}, Limitations: []string{}}
	case *Review:
		*v = Review{Approved: m.review, Issues: []string{}}
	}
	return nil
}
func TestGenerationReviewAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		model        *fakeModel
	}{{"pass", "completed", &fakeModel{review: true}}, {"citation", "review_rejected", &fakeModel{bad: true}}, {"semantic", "review_rejected", &fakeModel{}}, {"offline", "model_failed", &fakeModel{fail: true}}} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t, tc.model)
			code, b := call(t, s, "POST", "/runs", "w", Input{Task: "qa", Question: "气压单位为什么暂定"})
			var r Run
			_ = json.Unmarshal(b, &r)
			if code != 200 || r.Status != tc.status {
				t.Fatal(string(b))
			}
			if tc.status != "completed" && len(r.Claims) != 0 {
				t.Fatal("unreviewed output published")
			}
			code, b = call(t, s, "GET", "/history", "other", nil)
			if code != 200 || string(b) != "[]\n" {
				t.Fatal("history leaked", string(b))
			}
			recovered, err := New(s.cfg)
			if err != nil || len(recovered.runs) != 1 {
				t.Fatal(err)
			}
		})
	}
}
func TestNumericAndAdmissionValidation(t *testing.T) {
	s := fixture(t, nil)
	e, _ := s.evidence(context.Background(), "w", "", fixtureQuery(s))
	r := Run{Evidence: &e}
	var temp, held Summary
	for _, m := range e.Metrics {
		if m.Key == "air_temperature" {
			temp = m
		}
		if m.Key == "pm25" {
			held = m
		}
	}
	good := ModelOutput{Claims: []Claim{{Kind: "observation", Text: "温度范围30至32，均值31。", Citations: []string{temp.ID}}}}
	if err := validateOutput(good, r); err != nil {
		t.Fatal(err)
	}
	good.Claims[0].Text = "温度为99。"
	if validateOutput(good, r) == nil {
		t.Fatal("invented number accepted")
	}
	good.Claims[0] = Claim{Kind: "observation", Text: "PM正常", Citations: []string{held.ID}}
	if validateOutput(good, r) == nil {
		t.Fatal("held observation admitted")
	}
	good.Claims[0] = Claim{Kind: "common", Text: "当前工作台阀门已接通。", Citations: []string{}}
	if validateOutput(good, r) == nil {
		t.Fatal("project fact laundered as common knowledge")
	}
}
func TestSimulationRequiresExactConfirmationAndPersists(t *testing.T) {
	s := fixture(t, nil)
	code, b := call(t, s, "POST", "/plans", "w", map[string]any{"action": "open", "duration_seconds": 30})
	if code != 200 {
		t.Fatal(string(b))
	}
	var r Run
	_ = json.Unmarshal(b, &r)
	for _, in := range []map[string]any{{"id": r.ID, "revision": r.Plan.Revision, "confirm": false}, {"id": r.ID, "revision": "changed", "confirm": true}} {
		if code, _ = call(t, s, "POST", "/simulate", "w", in); code != 409 {
			t.Fatal("invalid confirmation accepted")
		}
	}
	confirm := map[string]any{"id": r.ID, "revision": r.Plan.Revision, "confirm": true}
	if code, _ = call(t, s, "POST", "/simulate", "other", confirm); code != 404 {
		t.Fatal("cross-scope execution")
	}
	code, first := call(t, s, "POST", "/simulate", "w", confirm)
	if code != 200 {
		t.Fatal(string(first))
	}
	code, second := call(t, s, "POST", "/simulate", "w", confirm)
	if code != 200 || string(first) != string(second) {
		t.Fatal("not idempotent")
	}
	restarted, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, third := call(t, restarted, "POST", "/simulate", "w", confirm)
	if string(first) != string(third) {
		t.Fatal("restart lost receipt")
	}
	r.Status = "simulating"
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	restarted, err = New(s.cfg)
	if err != nil || restarted.runs[r.ID].Status != "simulation_unknown" {
		t.Fatal(err)
	}
	if code, _ = call(t, restarted, "POST", "/simulate", "w", confirm); code != 409 {
		t.Fatal("interrupted action retried")
	}
}
func TestExpiredPlanAndUnknownFields(t *testing.T) {
	s := fixture(t, nil)
	_, b := call(t, s, "POST", "/plans", "w", map[string]any{"action": "close", "duration_seconds": 0})
	var r Run
	_ = json.Unmarshal(b, &r)
	r.Plan.ExpiresAt = time.Now().Add(-time.Minute)
	s.runs[r.ID] = r
	code, _ := call(t, s, "POST", "/simulate", "w", map[string]any{"id": r.ID, "revision": r.Plan.Revision, "confirm": true})
	if code != 409 {
		t.Fatal("expired plan accepted")
	}
	code, _ = call(t, s, "POST", "/plans", "w", map[string]any{"action": "open", "duration_seconds": 30, "target": "real-valve"})
	if code != 400 {
		t.Fatal("physical target accepted")
	}
}
func TestImageValidationAndFingerprint(t *testing.T) {
	s := fixture(t, nil)
	in := Input{Task: "image", Question: "描述叶片", ImageSource: "user_upload", Image: "bad"}
	if code, _ := call(t, s, "POST", "/runs", "w", in); code != 422 {
		t.Fatal("invalid image accepted")
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 40)))
	in.Image = base64.StdEncoding.EncodeToString(b.Bytes())
	code, response := call(t, s, "POST", "/runs", "w", in)
	var r Run
	_ = json.Unmarshal(response, &r)
	if code != 200 || r.ImageHash == "" || r.Status != "model_unavailable" {
		t.Fatal(string(response))
	}
	if _, err := os.Stat(filepath.Join(s.cfg.StateDir, "assets", r.ImageHash+".png")); err != nil {
		t.Fatal(err)
	}
}
func TestModelHTTPContractAndNoAutomaticPull(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/chat" {
			t.Fatal(r.URL.Path)
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["model"] != DefaultModel || in["think"] != false || in["keep_alive"] != float64(0) {
			t.Fatal(in)
		}
		if in["options"].(map[string]any)["num_batch"] != float64(128) {
			t.Fatal("unbounded prompt batch")
		}
		respond(w, 200, map[string]any{"done": true, "message": map[string]string{"content": `{"approved":true,"issues":[]}`}})
	}))
	defer srv.Close()
	var review Review
	err := (&Ollama{URL: srv.URL}).Generate(context.Background(), []Message{{Role: "user", Content: "test"}}, reviewSchema, &review)
	if err != nil || !review.Approved || calls != 1 {
		t.Fatal(err, review, calls)
	}
}

func TestPlanMutationRequiresNewConfirmation(t *testing.T) {
	s := fixture(t, nil)
	_, b := call(t, s, "POST", "/plans", "w", map[string]any{"action": "open", "duration_seconds": 30})
	var r Run
	_ = json.Unmarshal(b, &r)
	r.Plan.DurationSeconds = 60
	s.runs[r.ID] = r
	code, _ := call(t, s, "POST", "/simulate", "w", map[string]any{"id": r.ID, "revision": r.Plan.Revision, "confirm": true})
	if code != 409 {
		t.Fatal("a modified plan reused an old confirmation")
	}
}
func TestReplayAuthorizationAndLiveUnitMismatch(t *testing.T) {
	s := fixture(t, nil)
	s.cfg.CheckReplay = func(context.Context, string, *Dataset) error { return fmt.Errorf("device access denied") }
	if _, err := s.evidence(context.Background(), "w", "", fixtureQuery(s)); err == nil {
		t.Fatal("local replay bypassed object authorization")
	}
	s.dataset.ChannelID = "c"
	s.dataset.Metrics[0].PublisherID = "d"
	s.cfg.Live = func(context.Context, string, string, Query) ([]Frame, error) {
		return []Frame{{ReceivedAt: s.dataset.Frames[0].ReceivedAt, Values: map[string]float64{"air_temperature": 80}, Units: map[string]string{"air_temperature": "F"}}}, nil
	}
	q := fixtureQuery(s)
	q.Mode = "platform"
	q.ChannelID = "c"
	q.PublisherID = "d"
	e, err := s.evidence(context.Background(), "w", "auth", q)
	if err != nil || len(e.Metrics) != 1 || e.Metrics[0].Admission != "hold_for_validation" || e.Metrics[0].Unit != "F" {
		t.Fatal(e, err)
	}
}

func TestImageClaimsCannotBeLabeledGeneralKnowledge(t *testing.T) {
	r := Run{Task: "image", ImageHash: "image-sha"}
	bad := ModelOutput{Claims: []Claim{{Kind: "common", Text: "图片中没有叶片。", Citations: []string{}}}}
	if validateOutput(bad, r) == nil {
		t.Fatal("uncited image assessment accepted")
	}
	good := ModelOutput{Claims: []Claim{{Kind: "image", Text: "图片中没有叶片。", Citations: []string{"image-sha"}}}}
	if err := validateOutput(good, r); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeClaimsMustBeActualSourceExcerpts(t *testing.T) {
	r := Run{Knowledge: []Chunk{{ID: "k", Text: "气压单位暂定，仍待设备端确认。"}}}
	out := ModelOutput{Claims: []Claim{{Kind: "knowledge", Text: "气压单位已经确认。", Citations: []string{"k"}}}}
	if validateOutput(out, r) == nil {
		t.Fatal("an invented paraphrase passed as cited evidence")
	}
	out.Claims[0].Text = r.Knowledge[0].Text
	if err := validateOutput(out, r); err != nil || !literalOnly(out) {
		t.Fatal(err)
	}
}
