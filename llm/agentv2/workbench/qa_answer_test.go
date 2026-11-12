package workbench

import (
	"strings"
	"testing"
	"time"
)

func TestObservationAnswersAreReadableAndKeepTheirScope(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	window := Window{Start: time.Date(2026, 10, 27, 0, 0, 0, 0, zone), End: time.Date(2026, 10, 28, 0, 0, 0, 0, zone)}
	mean := 29.72
	summary := Summary{Metric: Metric{Key: "air_temperature", Label: "空气温度", Unit: "Cel"}, Count: 2881, Min: 27.1, Max: 36.7, Mean: &mean}
	for _, test := range []struct{ Operation, Want string }{
		{"max", "10月27日的最高气温为 36.7°C。"},
		{"min", "10月27日的最低气温为 27.1°C。"},
		{"mean", "10月27日的平均气温为 29.72°C。"},
		{"count", "10月27日查到 2881 条有效的气温记录。"},
	} {
		got := observationAnswer(queryIntent{Operation: test.Operation}, summary, window, now)
		if got != test.Want || strings.Contains(got, "Cel") || strings.Contains(got, "+08:00") {
			t.Fatalf("%s answer: %q", test.Operation, got)
		}
	}
	got := observationAnswer(queryIntent{Operation: "summary", Statistics: []string{"max", "min"}}, summary, window, now)
	if got != "10月27日的气温，最高为 36.7°C，最低为 27.1°C。" {
		t.Fatal(got)
	}
	window.Start = window.Start.AddDate(0, 0, -1)
	if got := observationAnswer(queryIntent{Operation: "max"}, summary, window, now); got != "10月26日至10月27日的最高气温为 36.7°C。" {
		t.Fatal(got)
	}
	summary.UnitConfirmation = "pending"
	if got := observationAnswer(queryIntent{Operation: "max"}, summary, window, now); !strings.Contains(got, "单位尚待确认") {
		t.Fatal(got)
	}
	latest := Frame{ReceivedAt: time.Date(2026, 10, 27, 22, 8, 34, 0, zone), Values: map[string]float64{"air_humidity": 68.4}}
	got = latestObservationAnswer(Metric{Key: "air_humidity", Unit: "%"}, latest, now)
	if got != "最新可查的湿度记录为 68.4%，记录时间是10月27日 22:08。" {
		t.Fatal(got)
	}
}

func TestOlderQueryAnswersAreRestoredFromEvidence(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Shanghai")
	run := Run{
		CreatedAt: time.Date(2026, 11, 5, 12, 0, 0, 0, zone), Question: "10月27日最高气温是多少？",
		Evidence: &Evidence{Mode: "platform", Start: time.Date(2026, 10, 27, 0, 0, 0, 0, zone), End: time.Date(2026, 10, 28, 0, 0, 0, 0, zone), Metrics: []Summary{{
			Metric: Metric{Key: "air_temperature", Label: "空气温度", Unit: "Cel"}, Count: 2881, Max: 36.7,
		}}},
		QueryResults: []QueryResult{{ID: "existing", Title: "空气温度 · 40366226", Source: "平台 Reader", Text: "2026-10-27 00:00:00 +08:00 至 2026-10-28 00:00:00 +08:00，有效样本数 2881，最大值 36.7 Cel。"}},
	}
	original := run.QueryResults[0].Text
	restoreQueryAnswers(&run)
	if run.QueryResults[0].Answer != "10月27日的最高气温为 36.7°C。" || run.QueryResults[0].Text != original || run.QueryResults[0].ID != "existing" {
		t.Fatalf("older evidence was not presented correctly %+v", run.QueryResults)
	}
	previous := QueryResult{Title: "空气温度 · 40366226", Text: "2026-10-26 00:00:00 +08:00 至 2026-10-27 00:00:00 +08:00，有效样本数 622，最大值 29.3 Cel。"}
	run.QueryResults = []QueryResult{previous}
	restoreQueryAnswers(&run)
	if strings.Contains(run.QueryResults[0].Answer, "36.7") || !strings.Contains(run.QueryResults[0].Answer, "29.3°C") {
		t.Fatalf("a previous window was overwritten by the latest evidence: %+v", run.QueryResults)
	}
}
