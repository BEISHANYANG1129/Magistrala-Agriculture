package workbench

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type automaticQueryModel struct {
	plan  queryPlan
	calls int
}

func (m *automaticQueryModel) Generate(_ context.Context, _ []Message, _ any, dst any) error {
	switch out := dst.(type) {
	case *queryPlan:
		m.calls++
		*out = m.plan
	case *ModelOutput:
		*out = ModelOutput{Claims: []Claim{{Kind: "knowledge", Text: "气压按原始值乘0.1接入，接入后的气压单位为 kPa。", Citations: []string{"project-pressure-co2"}}}}
	}
	return nil
}

func TestAutomaticQueryClarifiesThenCombinesPressureAndKnowledge(t *testing.T) {
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{{Source: "observations", Target: "A 站", Metric: "气压", Operation: "mean", Time: "最近24小时", Mode: "history"}}}}
	s := fixture(t, model)
	s.cfg.ConfirmationSubject = func(context.Context) string { return "alice" }
	s.dataset.StationID, s.dataset.ChannelID = "station", "channel"
	s.dataset.Metrics[3] = Metric{Key: "air_pressure", Label: "气压", Unit: "kPa", UnitConfirmation: "confirmed", Admission: "include", PublisherID: "device"}
	s.cfg.QueryTools = &QueryTools{Devices: func(context.Context, string) ([]QueryDevice, error) {
		return []QueryDevice{{ID: "device", Name: "A 站"}}, nil
	}}
	reads := 0
	s.cfg.Live = func(_ context.Context, workspace, auth string, q Query) ([]Frame, error) {
		reads++
		if workspace != "w" || auth != "Bearer caller" || q.Mode != "platform" || q.PublisherID != "device" ||
			q.ChannelID != "channel" || q.Metric != "air_pressure" || q.Start.Hour() != 12 || q.End.Hour() != 18 || q.Start.Location().String() != "Asia/Shanghai" {
			t.Fatalf("unexpected query %+v", q)
		}
		frame := func(value float64) Frame {
			return Frame{ReceivedAt: q.Start.Add(time.Minute), Values: map[string]float64{"air_pressure": value}, Units: map[string]string{"air_pressure": "kPa"}}
		}
		return []Frame{frame(100), frame(102)}, nil
	}
	first, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "A 站平均气压是多少，单位确定了吗？"})
	if err != nil || first.Status != "needs_clarification" || reads != 0 || first.pending == nil {
		t.Fatalf("missing clarification %+v %v", first, err)
	}
	// Continue after reopening saved state. Other accounts cannot open the turn.
	reopened, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	reopened.dataset = s.dataset
	if _, err := reopened.conversation("w", Input{Task: "qa", ParentID: first.ID}, "bob"); err == nil {
		t.Fatal("another account accessed the pending query")
	}
	result, err := reopened.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", ParentID: first.ID, Question: "昨天下午"})
	if err != nil || result.Status != "completed" || reads != 1 || model.calls != 1 || len(result.QueryResults) != 1 ||
		!strings.Contains(result.QueryResults[0].Text, "101.00 kPa") || len(result.Claims) != 1 {
		t.Fatalf("unexpected completed result %+v %v", result, err)
	}
}

func TestLoadDatasetConfirmsOnlyPressureAlreadyInKPa(t *testing.T) {
	d := Dataset{WorkspaceID: "w", Version: "old", TimeKind: "receiver_time_not_sensor_sampling_time", Metrics: []Metric{
		{Key: "air_pressure", Unit: "kPa", UnitConfirmation: "pending"},
		{Key: "air_pressure", Unit: "hPa", UnitConfirmation: "pending"},
		{Key: "other", Unit: "kPa", UnitConfirmation: "pending"},
	}}
	p := filepath.Join(t.TempDir(), "data.json")
	data, _ := json.Marshal(d)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDataset(p)
	if err != nil || got.Metrics[0].UnitConfirmation != "confirmed" || got.Metrics[1].UnitConfirmation != "pending" || got.Metrics[2].UnitConfirmation != "pending" {
		t.Fatalf("unexpected unit migration %+v %v", got, err)
	}
}

func TestAutomaticAlarmsAndDeviceStatus(t *testing.T) {
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{
		{Source: "device", Target: "误提取的设备名"},
	}}}
	s := fixture(t, model)
	s.cfg.ConfirmationSubject = func(context.Context) string { return "alice" }
	s.cfg.QueryTools = &QueryTools{
		Devices: func(context.Context, string) ([]QueryDevice, error) {
			return []QueryDevice{{ID: "device", Name: "A 站", Status: "enabled"}}, nil
		},
		Channels: func(context.Context, string, string) ([]string, error) {
			return []string{"channel"}, nil
		},
		Latest: func(_ context.Context, workspace, channel, device, metric string, window *Window) (*Frame, error) {
			if workspace != "w" || channel != "channel" || device != "device" || metric != "" || window != nil {
				t.Fatal("latest message query out of scope")
			}
			return &Frame{ReceivedAt: time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC), Source: channel}, nil
		},
		Alarms: func(_ context.Context, workspace string, filter AlarmFilter) ([]QueryAlarm, int, error) {
			if workspace != "w" || filter.DeviceID != "device" || filter.Status != "active" || filter.Window != nil {
				t.Fatal("alarm query out of scope")
			}
			return []QueryAlarm{{ID: "alarm", Name: "水位告警", Status: "active", CreatedAt: time.Now()}}, 1, nil
		},
	}
	r, err := s.run(context.Background(), "w", "Bearer caller", Input{Task: "qa", Question: "A 站在线吗，还有哪些活动告警？"})
	if err != nil || len(r.QueryResults) != 4 || !strings.Contains(r.QueryResults[0].Text, "已启用") ||
		!strings.Contains(r.QueryResults[1].Text, "11:00:00 +08:00") || !strings.Contains(strings.Join(r.Limitations, " "), "无法确认实时在线") {
		t.Fatalf("unexpected platform evidence %+v %v", r, err)
	}
	if !strings.Contains(r.QueryResults[0].Answer, "实时在线状态暂时无法确认") || !strings.Contains(r.QueryResults[3].Answer, "活动中") {
		t.Fatalf("important device limits or readable alarm state missing: %+v", r.QueryResults)
	}
}

func TestQueryWindowChineseMonthDay(t *testing.T) {
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	for _, expression := range []string{"10月26日", "10月26号", "10 月 26 日", "2026年10月26日", "2026-10-26"} {
		t.Run(expression, func(t *testing.T) {
			window, err := queryWindow(expression, now)
			want := time.Date(2026, 10, 26, 0, 0, 0, 0, zone)
			if err != nil || !window.Start.Equal(want) || !window.End.Equal(want.AddDate(0, 0, 1)) || window.Start.Location() != zone {
				t.Fatalf("unexpected whole-day window %+v, %v", window, err)
			}
		})
	}
	for _, invalid := range []string{"2月30日", "13月26日", "12月26日", "10月26日到11月5日"} {
		if window, err := queryWindow(invalid, now); err == nil {
			t.Fatalf("invalid or future date accepted %q: %+v", invalid, window)
		}
	}
}

func TestMonthDayQuestionAndConversationalClarification(t *testing.T) {
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	// Reproduce the model supplying a year that the user did not specify.
	model := &automaticQueryModel{plan: queryPlan{Reads: []queryIntent{{Source: "observations", Metric: "air_temperature", Operation: "max", Time: "2025-10-26"}}}}
	s := fixture(t, model)
	first, err := s.planQA(context.Background(), "10月26日的最高温度是什么", chatHistory{}, nil, now)
	if err != nil || len(first.Reads) != 1 {
		t.Fatalf("unexpected date plan %+v, %v", first, err)
	}
	window, err := queryWindow(first.Reads[0].Time, now)
	if err != nil || window.Start.Year() != 2026 || window.Start.Month() != time.October || window.Start.Day() != 26 {
		t.Fatalf("failed to ground the user's month-day %+v, %v", first, err)
	}
	pending := first
	pending.Reads = slices.Clone(first.Reads)
	pending.Reads[0].Time = "待确认"
	history := chatHistory{pending: &pending}
	for _, reply := range []string{"10月26日", "10月26日啊，这一天没有数据吗？"} {
		plan, err := s.planQA(context.Background(), reply, history, nil, now)
		if err != nil || len(plan.Reads) != 1 || plan.Reads[0].Operation != "max" || plan.Reads[0].Metric != "air_temperature" || model.calls != 1 {
			t.Fatalf("time clarification lost the original query %+v, %v", plan, err)
		}
		window, err := queryWindow(plan.Reads[0].Time, now)
		if err != nil || window.Start.Day() != 26 || window.End.Sub(window.Start) != 24*time.Hour {
			t.Fatalf("unexpected reply window %+v, %v", window, err)
		}
	}
	model.plan.Reads[0].Operation = "min"
	changed, err := s.planQA(context.Background(), "10月26日的最低温度呢？", history, nil, now)
	if err != nil || changed.Reads[0].Operation != "min" || model.calls != 2 {
		t.Fatalf("a new question incorrectly reused the pending maximum %+v, %v", changed, err)
	}
}
