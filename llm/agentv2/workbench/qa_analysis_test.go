package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

type analysisQueryModel struct {
	plan     queryPlan
	more     bool
	reject   bool
	analyses int
	catalog  dataCatalog
}

func (m *analysisQueryModel) Generate(_ context.Context, messages []Message, _ any, dst any) error {
	data := []byte(messages[len(messages)-1].Content)
	switch out := dst.(type) {
	case *queryPlan:
		var input struct {
			Catalog dataCatalog `json:"data_catalog"`
		}
		_ = json.Unmarshal(data, &input)
		m.catalog = input.Catalog
		*out = m.plan
	case *qaAnalysis:
		m.analyses++
		var input struct {
			Results []QueryResult `json:"query_results"`
		}
		_ = json.Unmarshal(data, &input)
		if m.more && m.analyses == 1 {
			out.Additional = []additionalRead{
				{Metric: "air_humidity", WindowID: input.Results[0].ID, Operation: "summary"},
				{Metric: "air_temperature", WindowID: input.Results[0].ID, Operation: "mean"}, // same evidence, no reread
				{Metric: "pm25", WindowID: input.Results[0].ID, Operation: "summary"},         // held
				{Metric: "rain", WindowID: input.Results[0].ID, Operation: "summary"},         // unregistered
				{Metric: "air_pressure", WindowID: "invented-window", Operation: "summary"},
			}
			return nil
		}
		for _, result := range input.Results {
			fact := result.Facts[0]
			for _, f := range result.Facts {
				if strings.HasSuffix(f.ID, "_range") {
					fact = f
					break
				}
			}
			out.Claims = append(out.Claims, Claim{Kind: "observation", Text: "{{" + fact.ID + "}}", Citations: []string{result.ID}})
		}
	default:
		// The analysis reviewer returns independent verdicts for each candidate.
		var input struct {
			Candidates []Claim `json:"candidates"`
		}
		_ = json.Unmarshal(data, &input)
		checks := []ClaimVerification{}
		for i := range input.Candidates {
			verdict := "approved"
			if m.reject {
				verdict = "rejected"
			}
			checks = append(checks, ClaimVerification{Index: i, Verdict: verdict, Reason: "核对指标、时间、数值和限制。"})
		}
		encoded, _ := json.Marshal(map[string]any{"checks": checks})
		return json.Unmarshal(encoded, dst)
	}
	return nil
}

func analysisFixture(t *testing.T, m *analysisQueryModel) (*Service, *int) {
	t.Helper()
	s := fixture(t, m)
	s.knowledge = nil
	s.cfg.ConfirmationSubject = func(context.Context) string { return "alice" }
	s.dataset.StationID, s.dataset.ChannelID = "station", "channel"
	s.dataset.Metrics = []Metric{
		{Key: "air_temperature", Label: "气温", Unit: "Cel", Admission: "include", PublisherID: "device"},
		{Key: "air_humidity", Label: "湿度", Unit: "%", Admission: "include", PublisherID: "device"},
		{Key: "air_pressure", Label: "气压", Unit: "kPa", Admission: "include", UnitConfirmation: "confirmed", PublisherID: "device"},
		{Key: "wind_speed", Label: "风速", Unit: "m/s", Admission: "include", PublisherID: "device"},
		{Key: "illuminance", Label: "光照", Unit: "lx", Admission: "include", PublisherID: "device"},
		{Key: "pm25", Label: "PM2.5", Unit: "ug/m3", Admission: "hold_for_validation", PublisherID: "device"},
	}
	s.cfg.QueryTools = &QueryTools{Devices: func(context.Context, string) ([]QueryDevice, error) {
		return []QueryDevice{{ID: "device", Name: "A站"}}, nil
	}}
	reads := new(int)
	s.cfg.Live = func(_ context.Context, workspace, auth string, q Query) ([]Frame, error) {
		*reads++
		if workspace != "w" || auth != "Bearer caller" || q.ChannelID != "channel" || q.PublisherID != "device" {
			t.Fatalf("unscoped read %+v", q)
		}
		metric, ok := observationMetric(s.dataset.Metrics, q.Metric)
		if !ok || metric.Admission == "hold_for_validation" {
			t.Fatalf("invalid metric reached adapter: %+v", q)
		}
		v := float64(q.Start.Day())
		return []Frame{
			{ReceivedAt: q.Start.Add(time.Hour), Values: map[string]float64{q.Metric: v}, Units: map[string]string{q.Metric: metric.Unit}},
			{ReceivedAt: q.Start.Add(2 * time.Hour), Values: map[string]float64{q.Metric: v + 2}, Units: map[string]string{q.Metric: metric.Unit}},
		}, nil
	}
	return s, reads
}

func TestWeatherOverviewUsesSemanticCatalogAndGroundedSynthesis(t *testing.T) {
	m := &analysisQueryModel{plan: queryPlan{Analyze: true, Reads: []queryIntent{{Source: "observations", Topic: "weather", Operation: "summary", Time: "10月27日"}}}}
	s, reads := analysisFixture(t, m)
	r, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "10月27日的整体天气状况怎么样？"})
	if err != nil || r.Clarification != "" || *reads != 5 || len(r.QueryResults) != 5 || !r.Synthesized || len(r.Claims) != 5 {
		t.Fatalf("overview failed %+v reads=%d err=%v", r, *reads, err)
	}
	if len(m.catalog.Metrics) != 6 || m.catalog.Station != "station" || !strings.Contains(strings.Join(r.AnswerNotes, " "), "晴雨") {
		t.Fatalf("missing catalog or weather limits: %+v %+v", m.catalog, r.AnswerNotes)
	}
	for _, q := range r.QueryResults {
		if q.Summary == nil || q.Window == nil || q.Read == nil || len(q.Facts) == 0 {
			t.Fatal("lost per-query evidence", q)
		}
		if q.Read.Mode != "platform" || q.Summary.Key == "pm25" {
			t.Fatal("invalid query source", q)
		}
	}
	for _, c := range r.Claims {
		if strings.Contains(c.Text, "{{") || strings.Contains(c.Text, "Cel") {
			t.Fatal("unrendered facts", c)
		}
	}
	// The complete per-window snapshots survive restart.
	reopened, err := New(s.cfg)
	if err != nil || len(reopened.runs[r.ID].QueryResults) != 5 || reopened.runs[r.ID].QueryResults[0].Summary == nil {
		t.Fatal("snapshots were not persisted", err)
	}
}

func TestAnalysisCanRequestOneScopedSupplementalRound(t *testing.T) {
	m := &analysisQueryModel{more: true, plan: queryPlan{Analyze: true, Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "summary", Time: "10月27日"}}}}
	s, reads := analysisFixture(t, m)
	r, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "分析10月27日的温度，结合相关指标说明"})
	if err != nil || *reads != 2 || m.analyses != 2 || len(r.query.Reads) != 2 || !r.Synthesized {
		t.Fatalf("invalid supplemental reads %+v reads=%d rounds=%d %v", r, *reads, m.analyses, err)
	}
	if r.QueryResults[1].Summary.Key != "air_humidity" || !r.QueryResults[1].Window.Start.Equal(r.QueryResults[0].Window.Start) {
		t.Fatal("supplemental scope changed", r.QueryResults)
	}
}

func TestRejectedSynthesisKeepsVerifiedResults(t *testing.T) {
	m := &analysisQueryModel{reject: true, plan: queryPlan{Analyze: true, Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "summary", Time: "10月27日"}}}}
	s, _ := analysisFixture(t, m)
	r, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "分析10月27日气温"})
	if err != nil || r.Synthesized || len(r.Claims) != 0 || len(r.QueryResults) != 1 || !strings.Contains(r.QueryResults[0].Answer, "28°C") {
		t.Fatalf("verified fallback lost %+v %v", r, err)
	}
}

func TestFactTokensRejectInventedNumbersAndCrossSourceReferences(t *testing.T) {
	r := Run{QueryResults: []QueryResult{{ID: "temperature", Summary: &Summary{Metric: Metric{Key: "air_temperature", Label: "温度"}}, Facts: []queryFact{{ID: "f1_max", Text: "36.7°C", Source: "temperature"}}}, {ID: "humidity", Summary: &Summary{Metric: Metric{Key: "air_humidity", Label: "湿度"}}, Facts: []queryFact{{ID: "f2_mean", Text: "56.37%", Source: "humidity"}}}}}
	for _, c := range []Claim{
		{Text: "最高温度37°C", Kind: "observation", Citations: []string{"temperature"}},
		{Text: "最高温度{{f2_mean}}", Kind: "observation", Citations: []string{"temperature"}},
		{Text: "最高温度{{invented}}", Kind: "observation", Citations: []string{"temperature"}},
	} {
		if _, err := renderAnalysisClaim(c, r); err == nil {
			t.Fatal("unsupported claim accepted", c)
		}
	}
	c, err := renderAnalysisClaim(Claim{Text: "最高气温为{{f1_max}}。", Kind: "observation", Citations: []string{"temperature"}}, r)
	if err != nil || c.Text != "最高气温为36.7°C。" {
		t.Fatal(c, err)
	}
	c, err = renderAnalysisClaim(Claim{Text: "最高气温为{{f1_max}}。", Kind: "observation", Citations: []string{"humidity"}}, r)
	if err != nil || !slices.Equal(c.Citations, []string{"temperature"}) {
		t.Fatal("fact provenance was not bound by program", c, err)
	}
}

func TestComparisonsComputeDifferencesAndKeepPercentUnits(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	makeResult := func(day int, value float64, unit string) QueryResult {
		window := Window{Start: time.Date(2026, 10, day, 0, 0, 0, 0, zone), End: time.Date(2026, 10, day+1, 0, 0, 0, 0, zone)}
		return QueryResult{ID: fmt.Sprint(day), Window: &window, Read: &queryIntent{Mode: "platform"}, Summary: &Summary{Metric: Metric{Key: "air_humidity", Unit: unit, PublisherID: "device", Admission: "include"}, Min: value, Max: value, Mean: &value}}
	}
	r := Run{QueryResults: []QueryResult{makeResult(26, 66.72, "%"), makeResult(27, 56.37, "%")}}
	appendQueryComparisons(&r, now)
	if len(r.QueryResults) != 3 || !strings.Contains(r.QueryResults[2].Answer, "降低 10.35 个百分点") {
		t.Fatal(r.QueryResults)
	}
	appendQueryComparisons(&r, now)
	if len(r.QueryResults) != 3 {
		t.Fatal("duplicate comparison")
	}
	r = Run{QueryResults: []QueryResult{makeResult(26, 101, "kPa"), makeResult(27, 102, "hPa")}}
	appendQueryComparisons(&r, now)
	if len(r.QueryResults) != 2 {
		t.Fatal("compared incompatible units")
	}
}

func TestCatalogDiscoveryIsCallerAndWorkspaceScoped(t *testing.T) {
	m := &analysisQueryModel{}
	s, _ := analysisFixture(t, m)
	s.dataset.Metrics[0].PublisherID = "hidden-device"
	c := s.qaCatalog(context.Background(), "w")
	if slices.ContainsFunc(c.Metrics, func(m catalogMetric) bool { return m.Key == "air_temperature" }) {
		t.Fatal("hidden device exposed", c)
	}
	if len(s.qaCatalog(context.Background(), "other-workspace").Metrics) != 0 {
		t.Fatal("cross-workspace metric discovery")
	}
	s.cfg.QueryTools.Devices = func(context.Context, string) ([]QueryDevice, error) { return nil, ErrQueryDenied }
	if c = s.qaCatalog(context.Background(), "w"); len(c.Metrics) != 0 || c.Error == "" {
		t.Fatal("denied catalog exposed", c)
	}
}

func TestExplicitCatalogQuestionDoesNotInheritObservationRequirements(t *testing.T) {
	m := &analysisQueryModel{plan: queryPlan{Analyze: true, Reads: []queryIntent{{Source: "observations", Operation: "summary"}}}}
	s, _ := analysisFixture(t, m)
	prior := &queryPlan{Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "summary", Time: "10月25日"}}}
	for _, question := range []string{"当前有哪些可查询的观测指标？", "能查哪些数据？", "有哪些观测指标？"} {
		plan, err := s.planQA(context.Background(), question, chatHistory{query: prior}, nil, time.Now())
		if err != nil || len(plan.Reads) != 1 || plan.Reads[0].Source != "catalog" || plan.Analyze {
			t.Fatal("catalog question became a measurement query", plan, err)
		}
	}
	for _, question := range []string{"10月27日有哪些指标异常？", "今天能查哪些温度数据？", "有哪些指标的平均值超过阈值？"} {
		if asksForCatalog(question) {
			t.Fatal("measurement question became discovery", question)
		}
	}
}

func TestCompositeBoundsAndContextualComparisonFollowup(t *testing.T) {
	m := &analysisQueryModel{}
	s, _ := analysisFixture(t, m)
	plan := queryPlan{Reads: make([]queryIntent, 8)}
	for i := range plan.Reads {
		plan.Reads[i] = queryIntent{Source: "observations", Topic: "weather", Time: fmt.Sprint(i)}
	}
	if s.expandQAReads(&plan, "w") == nil {
		t.Fatal("unbounded expansion")
	}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	window := Window{Start: time.Date(2026, 10, 27, 0, 0, 0, 0, zone), End: time.Date(2026, 10, 28, 0, 0, 0, 0, zone)}
	previous := &queryPlan{Question: "10月27日整体天气", Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "summary", Window: &window}, {Source: "observations", Metric: "air_humidity", Operation: "summary", Window: &window}}}
	got, ok := comparisonFollowup("那和前一天比呢？", chatHistory{query: previous}, nil, now)
	if !ok || !got.Analyze || len(got.Reads) != 4 || got.Reads[1].Window.Start.Day() != 26 || got.Reads[0].Window.Start.Day() != 27 {
		t.Fatal(got, ok)
	}
	filtered, ok := metricFollowup("只看湿度呢？", chatHistory{query: &got}, s.dataset.Metrics, now)
	if !ok || len(filtered.Reads) != 2 || filtered.Reads[0].Metric != "air_humidity" || filtered.Reads[1].Metric != "air_humidity" || filtered.Reads[1].Window.Start.Day() != 26 {
		t.Fatal("metric followup lost comparison windows", filtered, ok)
	}
	misrouted := queryPlan{Question: "10月27日整体天气", Reads: []queryIntent{{Source: "catalog", Topic: "weather", Operation: "summary", Time: "10月27日"}}}
	if err := s.expandQAReads(&misrouted, "w"); err != nil || len(misrouted.Reads) != 5 || misrouted.Reads[0].Source != "observations" {
		t.Fatal("catalog misrouting not normalized", misrouted, err)
	}
}

func TestAnalysisCannotHideAnswerWithOnlyCoverageOrOmitComparison(t *testing.T) {
	r := Run{Synthesized: true, Claims: []Claim{{Text: "记录覆盖日期", Kind: "observation", Citations: []string{"raw"}, facts: []string{"f1_coverage"}}}}
	ensureQAAnswerCoverage(&r)
	if r.Synthesized {
		t.Fatal("coverage-only narrative hid numeric answers")
	}
	r.QueryResults = []QueryResult{{ID: "comparison", Source: "程序计算 · raw-a · raw-b", Answer: "最高气温升高7.4°C。"}}
	ensureQAAnswerCoverage(&r)
	if !r.Synthesized || len(r.Claims) != 2 || r.Claims[1].Text != "最高气温升高7.4°C。" {
		t.Fatal("computed comparison missing", r)
	}
}

func TestOverviewClarifiesMissingTimeAndExplainsMissingData(t *testing.T) {
	m := &analysisQueryModel{plan: queryPlan{Analyze: true, Reads: []queryIntent{{Source: "observations", Topic: "weather", Operation: "summary"}}}}
	s, reads := analysisFixture(t, m)
	r, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "整体天气状况怎么样？"})
	if err != nil || r.Status != "needs_clarification" || !strings.Contains(r.Clarification, "时间") || strings.Contains(r.Clarification, "指标") || *reads != 0 {
		t.Fatal("overview asked for a metric instead of time", r, err)
	}
	m.plan.Reads[0].Time = "10月25日"
	s.cfg.Live = func(context.Context, string, string, Query) ([]Frame, error) { return nil, nil }
	r, err = s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "10月25日整体天气怎么样？"})
	if err != nil || r.Clarification != "" || len(r.QueryResults) != 5 {
		t.Fatal("missing date became slot failure", r, err)
	}
	for _, result := range r.QueryResults {
		if result.Summary != nil || !strings.Contains(result.Answer, "没有查到") {
			t.Fatal("invented weather on an empty day", result)
		}
	}
	if len(r.Claims) != 1 || m.analyses != 0 || strings.Count(r.Claims[0].Text, "没有查到") > 1 {
		t.Fatal("empty overview should be concise and require no analysis", r, m.analyses)
	}
}
