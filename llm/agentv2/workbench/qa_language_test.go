package workbench

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNaturalCalendarWindows(t *testing.T) {
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	for _, test := range []struct{ Question, Start, End string }{
		{"十月二十六日的最高温度是多少", "2026-10-26 00:00", "2026-10-27 00:00"},
		{"10月26日上午9点到下午3点的平均温度是多少", "2026-10-26 09:00", "2026-10-26 15:00"},
		{"10月26日上午九点半到下午三点的平均温度是多少", "2026-10-26 09:30", "2026-10-26 15:00"},
		{"10月26日到27日最高温度多少", "2026-10-26 00:00", "2026-10-28 00:00"},
		{"2026-10-26 09:00~2026-10-26 15:00最高温度多少", "2026-10-26 09:00", "2026-10-26 15:00"},
		{"2026-10-7 9:00~15:00最高温度多少", "2026-10-07 09:00", "2026-10-07 15:00"},
		{"最近两个小时的平均气压", "2026-11-05 10:00", "2026-11-05 12:00"},
		{"昨天下午的平均温度", "2026-11-04 12:00", "2026-11-04 18:00"},
		{"今天的最高气温", "2026-11-05 00:00", "2026-11-05 12:00"},
		{"１０月２６日的最高气温", "2026-10-26 00:00", "2026-10-27 00:00"},
		{"10月26日晚上12点到27日凌晨1点的平均气温", "2026-10-27 00:00", "2026-10-27 01:00"},
	} {
		t.Run(test.Question, func(t *testing.T) {
			expression := timeExpression(test.Question)
			window, err := queryWindow(expression, now)
			if err != nil || window.Start.Format("2006-01-02 15:04") != test.Start || window.End.Format("2006-01-02 15:04") != test.End {
				t.Fatalf("%q => %q => %+v, %v", test.Question, expression, window, err)
			}
		})
	}
	for _, question := range []string{"2月30日", "10月26日下午3点到上午9点", "最近8天", "12月26日"} {
		if w, err := queryWindow(question, now); err == nil {
			t.Fatalf("invalid query accepted %q => %+v", question, w)
		}
	}
	if expression := timeExpression("昨天温度和今天告警"); expression != "" {
		t.Fatalf("different source windows collapsed to %q", expression)
	}
}

func TestLatestReadingClearsHistoricalWindowAndMode(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	previous := &queryPlan{Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "max", Time: "10月26日", Mode: "history", Window: &Window{
		Start: time.Date(2026, 10, 26, 0, 0, 0, 0, zone), End: time.Date(2026, 10, 27, 0, 0, 0, 0, zone),
	}}}}
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "latest", Mode: "history"}}}}
	s := fixture(t, model)
	plan, err := s.planQA(context.Background(), "那现在的温度呢？", chatHistory{query: previous}, nil, now)
	if err != nil || plan.Reads[0].Window != nil || plan.Reads[0].Time != "" || plan.Reads[0].Mode != "platform" {
		t.Fatalf("current reading inherited a historical window %+v, %v", plan, err)
	}
	model.plan.Reads[0].Operation = "mean"
	model.plan.Reads[0].Metric = "air_humidity"
	plan, err = s.planQA(context.Background(), "不是温度，是湿度", chatHistory{query: previous}, nil, now)
	if err != nil || plan.Reads[0].Window == nil || !plan.Reads[0].Window.Start.Equal(previous.Reads[0].Window.Start) || plan.Reads[0].Mode != "history" || plan.Reads[0].Operation != "max" {
		t.Fatalf("metric correction lost the selected day %+v, %v", plan, err)
	}
}

func TestDifferentMetricsKeepTheirOwnStatistics(t *testing.T) {
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{
		{Source: "observations", Metric: "air_temperature", Operation: "max", Time: "10月26日"},
		{Source: "observations", Metric: "air_humidity", Operation: "mean", Time: "10月26日"},
	}}}
	s := fixture(t, model)
	zone, _ := time.LoadLocation("Asia/Shanghai")
	plan, err := s.planQA(context.Background(), "10月26日最高温度和平均湿度是多少", chatHistory{}, nil, time.Date(2026, 11, 5, 12, 0, 0, 0, zone))
	if err != nil || len(plan.Reads) != 2 || plan.Reads[0].Operation != "max" || plan.Reads[1].Operation != "mean" {
		t.Fatalf("statistics crossed metrics %+v, %v", plan, err)
	}
}

func TestSameMetricCanUseTwoRequestedDays(t *testing.T) {
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{
		{Source: "observations", Metric: "air_temperature", Operation: "mean", Time: "10月26日"},
		{Source: "observations", Metric: "air_temperature", Operation: "mean", Time: "10月27日"},
	}}}
	s := fixture(t, model)
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	plan, err := s.planQA(context.Background(), "10月26日和10月27日平均温度分别是多少", chatHistory{}, nil, now)
	if err != nil || len(plan.Reads) != 2 || plan.Reads[0].Time != "10月26日" || plan.Reads[1].Time != "10月27日" {
		t.Fatalf("two requested days were collapsed %+v, %v", plan, err)
	}
}

func TestQuerySlotsSurviveCompletedAnswerAndRestart(t *testing.T) {
	model := &automaticQueryModel{}
	s := fixture(t, model)
	s.knowledge = nil
	s.cfg.ConfirmationSubject = func(context.Context) string { return "alice" }
	s.dataset.StationID, s.dataset.ChannelID = "station", "channel"
	s.dataset.Metrics[0].PublisherID = "device"
	s.cfg.QueryTools = &QueryTools{}
	base := time.Now().AddDate(0, -1, 0)
	day := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location())
	model.plan = queryPlan{Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "max", Time: day.Format("2006年1月2日")}}}
	s.cfg.Live = func(_ context.Context, workspace, auth string, q Query) ([]Frame, error) {
		if q.Metric != "air_temperature" || q.ChannelID != "channel" || q.PublisherID != "device" || workspace != "w" {
			t.Fatalf("lost observation scope %+v", q)
		}
		return []Frame{
			{ReceivedAt: q.Start.Add(time.Hour), Values: map[string]float64{"air_temperature": 20}, Units: map[string]string{"air_temperature": "Cel"}},
			{ReceivedAt: q.Start.Add(2 * time.Hour), Values: map[string]float64{"air_temperature": 25}, Units: map[string]string{"air_temperature": "Cel"}},
		}, nil
	}
	first, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: day.Format("2006年1月2日") + "的最高温度是多少"})
	if err != nil || len(first.QueryResults) != 1 || first.query == nil || !strings.Contains(first.QueryResults[0].Text, "最大值 25") {
		t.Fatalf("failed initial query %+v, %v", first, err)
	}
	// Re-open the saved conversation so success cannot depend on a pending slot.
	reopened, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	reopened.dataset = s.dataset
	reopened.knowledge = nil
	parent := first.ID
	for _, test := range []struct {
		Question, Want string
		Day            int
	}{
		{"最低呢？", "最小值 20", 1},
		{"那平均呢？", "平均值 22.50", 1},
		{"换成2号呢？", "平均值 22.50", 2},
		{"那下午呢？", "平均值 22.50", 2},
		{"那9点到15点呢？", "平均值 22.50", 2},
	} {
		result, err := reopened.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", ParentID: parent, Question: test.Question})
		if err != nil || result.Status != "completed" || len(result.QueryResults) != 1 || !strings.Contains(result.QueryResults[0].Text, test.Want) || result.Evidence.Start.Day() != test.Day {
			t.Fatalf("followup %q failed %+v, %v", test.Question, result, err)
		}
		parent = result.ID
	}
	if model.calls != 1 {
		t.Fatalf("short slot edits unexpectedly re-planned %d times", model.calls)
	}
}

func TestMultipleStatisticsShareOneObservationQuery(t *testing.T) {
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{
		{Source: "observations", Metric: "air_temperature", Operation: "max", Time: "10月26日"},
		{Source: "observations", Metric: "air_temperature", Operation: "min", Time: "10月26日"},
	}}}
	s := fixture(t, model)
	zone, _ := time.LoadLocation("Asia/Shanghai")
	plan, err := s.planQA(context.Background(), "10月26日最高和最低温度分别是多少", chatHistory{}, nil, time.Date(2026, 11, 5, 12, 0, 0, 0, zone))
	if err != nil || len(plan.Reads) != 1 || plan.Reads[0].Operation != "summary" || fmt.Sprint(plan.Reads[0].Statistics) != "[max min]" {
		t.Fatalf("multiple statistics were lost %+v, %v", plan, err)
	}
}
