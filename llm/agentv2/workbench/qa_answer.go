package workbench

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func readableUnit(text string) string {
	return strings.NewReplacer(" Cel", "°C", "Cel", "°C", " %", "%").Replace(text)
}

func observationName(metric Metric) string {
	names := map[string]string{"air_temperature": "气温", "air_humidity": "湿度", "air_pressure": "气压", "wind_speed": "风速", "illuminance": "光照强度", "co2": "二氧化碳浓度"}
	if name := names[metric.Key]; name != "" {
		return name
	}
	if metric.Label != "" {
		return metric.Label
	}
	return metric.Key
}

func observationValue(value float64, unit string, mean bool) string {
	number := strconv.FormatFloat(value, 'f', -1, 64)
	if mean {
		number = strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
	}
	switch unit {
	case "Cel", "°C":
		return number + "°C"
	case "%", "deg":
		return number + map[string]string{"%": "%", "deg": "°"}[unit]
	case "":
		return number
	default:
		return number + " " + unit
	}
}

func answerDate(t, now time.Time) string {
	if t.Year() == now.Year() {
		return t.Format("1月2日")
	}
	return t.Format("2006年1月2日")
}

func observationPeriod(window Window, now time.Time) string {
	start, end := window.Start.In(now.Location()), window.End.In(now.Location())
	last := end.Add(-time.Nanosecond)
	midnight := func(t time.Time) bool {
		return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
	}
	if midnight(start) && midnight(end) {
		if start.Format("2006-01-02") == last.Format("2006-01-02") {
			return answerDate(start, now)
		}
		return answerDate(start, now) + "至" + answerDate(last, now)
	}
	if start.Format("2006-01-02") == end.Format("2006-01-02") {
		return answerDate(start, now) + " " + start.Format("15:04") + "至" + end.Format("15:04")
	}
	return answerDate(start, now) + " " + start.Format("15:04") + "至" + answerDate(end, now) + " " + end.Format("15:04")
}

func observationAnswer(read queryIntent, summary Summary, window Window, now time.Time) string {
	period, metric := observationPeriod(window, now), observationName(summary.Metric)
	answer := ""
	switch read.Operation {
	case "max":
		answer = period + "的最高" + metric + "为 " + observationValue(summary.Max, summary.Unit, false) + "。"
	case "min":
		answer = period + "的最低" + metric + "为 " + observationValue(summary.Min, summary.Unit, false) + "。"
	case "mean":
		if summary.Mean == nil {
			answer = period + "暂时无法给出有效的平均" + metric + "。"
		} else {
			answer = period + "的平均" + metric + "为 " + observationValue(*summary.Mean, summary.Unit, true) + "。"
		}
	case "count":
		answer = fmt.Sprintf("%s查到 %d 条有效的%s记录。", period, summary.Count, metric)
	case "summary":
		statistics := read.Statistics
		if len(statistics) == 0 {
			statistics = []string{"max", "min", "mean"}
		}
		var parts []string
		for _, statistic := range statistics {
			switch statistic {
			case "max":
				parts = append(parts, "最高为 "+observationValue(summary.Max, summary.Unit, false))
			case "min":
				parts = append(parts, "最低为 "+observationValue(summary.Min, summary.Unit, false))
			case "mean":
				if summary.Mean != nil {
					parts = append(parts, "平均为 "+observationValue(*summary.Mean, summary.Unit, true))
				}
			}
		}
		answer = period + "的" + metric + "，" + strings.Join(parts, "，") + "。"
	}
	if summary.UnitConfirmation == "pending" {
		answer += "该指标的单位尚待确认。"
	}
	return answer
}

func missingObservationAnswer(metric Metric, window *Window, now time.Time) string {
	scope := ""
	if window != nil {
		scope = observationPeriod(*window, now) + "的"
	}
	return "没有查到" + scope + observationName(metric) + "记录，暂时无法给出对应数值。"
}

func latestObservationAnswer(metric Metric, frame Frame, now time.Time) string {
	at := frame.ReceivedAt.In(now.Location())
	answer := "最新可查的" + observationName(metric) + "记录为 " + observationValue(frame.Values[metric.Key], metric.Unit, false) +
		"，记录时间是" + answerDate(at, now) + " " + at.Format("15:04") + "。"
	if metric.UnitConfirmation == "pending" {
		answer += "该指标的单位尚待确认。"
	}
	return answer
}

// Reconstruct presentation for older, single-metric saved answers from their
// evidence snapshot. The numeric evidence and stored query are retained.
func restoreQueryAnswers(run *Run) {
	context := queryContext(*run)
	for i := range run.QueryResults {
		result := &run.QueryResults[i]
		if result.Answer != "" {
			continue
		}
		result.Answer = readableUnit(result.Text)
		if context == nil || run.Evidence == nil || len(run.Evidence.Metrics) != 1 {
			continue
		}
		window := Window{Start: run.Evidence.Start, End: run.Evidence.End}
		if !strings.HasPrefix(result.Text, displayWindow(window)+"，") {
			continue
		}
		metric := run.Evidence.Metrics[0]
		for _, read := range context.Reads {
			_, matched := observationMetric([]Metric{metric.Metric}, read.Metric)
			if read.Source == "observations" && matched && (strings.HasPrefix(result.Title, metric.Label+" ·") || strings.HasPrefix(result.Title, metric.Key+" ·")) {
				zone, _ := time.LoadLocation("Asia/Shanghai")
				if answer := observationAnswer(read, metric, window, run.CreatedAt.In(zone)); answer != "" {
					result.Answer = answer
				}
				break
			}
		}
	}
}
