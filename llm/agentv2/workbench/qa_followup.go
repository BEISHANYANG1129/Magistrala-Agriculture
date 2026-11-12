package workbench

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

var currentReading = regexp.MustCompile(`现在|当前|最新|最近一条`)
var queryCorrection = regexp.MustCompile(`改成|换成|那天|当天|同一天|同一个|不是.+是`)
var comparisonQuestion = regexp.MustCompile(`相比|比较|对比|[和跟与].{0,20}比`)

func metricFollowup(question string, history chatHistory, metrics []Metric, now time.Time) (queryPlan, bool) {
	previous := history.pending
	if previous == nil {
		previous = history.query
	}
	if previous == nil {
		return queryPlan{}, false
	}
	text := strings.Trim(question, " ，,。？！?!啊呀呢吧")
	for _, prefix := range []string{"那么", "那", "只看", "换成", "改成", "查询", "查"} {
		text = strings.TrimPrefix(text, prefix)
	}
	metric, ok := observationMetric(metrics, text)
	if !ok {
		return queryPlan{}, false
	}
	matched := slices.ContainsFunc(previous.Reads, func(r queryIntent) bool {
		m, ok := observationMetric(metrics, r.Metric)
		return r.Source == "observations" && ok && m.Key == metric.Key
	})
	plan := cloneQuery(previous)
	plan.At, plan.Question = now, question
	plan.Reads = nil
	seen := map[string]bool{}
	for _, read := range previous.Reads {
		if read.Source != "observations" {
			continue
		}
		m, _ := observationMetric(metrics, read.Metric)
		if matched && m.Key != metric.Key {
			continue
		}
		read.Metric = metric.Key
		read.Metrics, read.Topic = nil, ""
		key := fingerprint(read)
		if !seen[key] {
			plan.Reads = append(plan.Reads, read)
			seen[key] = true
		}
	}
	return plan, len(plan.Reads) > 0
}

// Explicit comparison follow-ups add a window instead of overwriting the
// selected date. Contextual "前一天" means the day before that selection.
func comparisonFollowup(question string, history chatHistory, times []recognizedTime, now time.Time) (queryPlan, bool) {
	previous := history.query
	if previous == nil || !comparisonQuestion.MatchString(question) {
		return queryPlan{}, false
	}
	text := strings.TrimSpace(question)
	text = strings.TrimPrefix(strings.TrimPrefix(text, "那么"), "那")
	if !strings.HasPrefix(text, "和") && !strings.HasPrefix(text, "与") && !strings.HasPrefix(text, "跟") && !strings.HasPrefix(text, "对比") {
		return queryPlan{}, false
	}
	var base *Window
	for _, read := range previous.Reads {
		if read.Source != "observations" {
			continue
		}
		if read.Window == nil {
			return queryPlan{}, false
		}
		if base != nil && (!base.Start.Equal(read.Window.Start) || !base.End.Equal(read.Window.End)) {
			return queryPlan{}, false
		}
		base = read.Window
	}
	if base == nil {
		return queryPlan{}, false
	}
	var next *Window
	expression := ""
	if strings.Contains(text, "前一天") || strings.Contains(text, "前一日") {
		window := Window{Start: base.Start.AddDate(0, 0, -1), End: base.End.AddDate(0, 0, -1)}
		next, expression = &window, "前一天"
	} else if len(times) == 1 && times[0].Window != nil && times[0].Error == "" {
		next, expression = times[0].Window, times[0].Text
	} else if expression = timeExpression(question); expression != "" {
		if window, err := queryWindow(expression, now, base.Start); err == nil {
			next = &window
		}
	}
	if next == nil || next.Start.Equal(base.Start) || len(previous.Reads)*2 > maxQAReads {
		return queryPlan{}, false
	}
	plan := cloneQuery(previous)
	plan.Analyze, plan.At, plan.Question = true, now, previous.Question+"；"+question
	plan.Reads = nil
	for _, read := range previous.Reads {
		if read.Source != "observations" {
			continue
		}
		plan.Reads = append(plan.Reads, read)
		read.Time, read.Window, read.TimeError = expression, next, ""
		plan.Reads = append(plan.Reads, read)
	}
	return plan, true
}

func queryFollowup(question string) bool {
	return sourceFollowup.MatchString(question) || queryCorrection.MatchString(question)
}

func previousObservation(plan *queryPlan, metric string) *queryIntent {
	if plan == nil {
		return nil
	}
	var only *queryIntent
	count := 0
	for i := range plan.Reads {
		read := &plan.Reads[i]
		if read.Source != "observations" {
			continue
		}
		if read.Metric == metric {
			return read
		}
		only = read
		count++
	}
	if count == 1 {
		return only
	}
	// A new metric in a single-window overview inherits the shared operation.
	if count > 1 && only != nil {
		for _, read := range plan.Reads {
			if read.Source == "observations" && (read.Operation != only.Operation || read.Mode != only.Mode || read.Target != only.Target || fingerprint(read.Window) != fingerprint(only.Window)) {
				return nil
			}
		}
		return only
	}
	return nil
}

func queryContext(run Run) *queryPlan {
	if run.query != nil {
		return run.query
	}
	if run.pending != nil {
		return run.pending
	}
	// Recover already-saved single-metric answers from before query contexts
	// were persisted. Never infer a device or metric from free model prose.
	if run.Evidence == nil || len(run.Evidence.Metrics) != 1 || len(run.QueryResults) == 0 {
		return nil
	}
	e := run.Evidence
	operation := ""
	for _, result := range run.QueryResults {
		switch {
		case strings.Contains(result.Text, "平均值"):
			operation = "mean"
		case strings.Contains(result.Text, "最大值"):
			operation = "max"
		case strings.Contains(result.Text, "最小值"):
			operation = "min"
		}
	}
	if operation == "" {
		return nil
	}
	return &queryPlan{At: run.CreatedAt, Question: run.Question, Reads: []queryIntent{{
		Source: "observations", Metric: e.Metrics[0].Key, Operation: operation, Mode: e.Mode,
		Target: e.Metrics[0].PublisherID, Window: &Window{Start: e.Start, End: e.End},
	}}}
}

func queryReference(plan *queryPlan, now time.Time) time.Time {
	if plan != nil {
		for _, read := range plan.Reads {
			if read.Window != nil {
				return read.Window.Start.In(now.Location())
			}
			if read.Time != "" {
				at := plan.At
				if at.IsZero() {
					at = now
				}
				if window, err := queryWindow(read.Time, at.In(now.Location())); err == nil {
					return window.Start
				}
			}
		}
		if expression := timeExpression(plan.Question); expression != "" {
			if window, err := queryWindow(expression, now); err == nil {
				return window.Start
			}
		}
	}
	return now
}

func cloneQuery(plan *queryPlan) queryPlan {
	copy := *plan
	copy.Reads = slices.Clone(plan.Reads)
	return copy
}

// Clear, short follow-ups edit the existing query slots. New full questions
// still go through the model, with the saved slots available as context.
func followupQuery(question string, history chatHistory, now time.Time) (queryPlan, bool) {
	previous := history.pending
	if previous == nil {
		previous = history.query
	}
	if previous == nil {
		return queryPlan{}, false
	}
	if expression := timeClarification(question); expression != "" {
		if window, err := queryWindow(expression, now, queryReference(previous, now)); err == nil {
			plan := cloneQuery(previous)
			plan.At = now
			for i := range plan.Reads {
				if plan.Reads[i].Source == "device" {
					continue
				}
				plan.Reads[i].Time = expression
				plan.Reads[i].Window = &window
			}
			return plan, true
		}
	}
	text := strings.Trim(normalizeQueryTime(question), "，,。？！?!啊呀呢吧")
	text = strings.TrimPrefix(strings.TrimPrefix(text, "那么"), "那")
	operation := map[string]string{
		"最低": "min", "最低温度": "min", "最小值": "min",
		"最高": "max", "最高温度": "max", "最大值": "max",
		"平均": "mean", "平均温度": "mean", "平均值": "mean", "均值": "mean",
		"多少条": "count", "样本数": "count",
	}[text]
	if operation == "" {
		return queryPlan{}, false
	}
	plan := cloneQuery(previous)
	plan.At = now
	plan.Reads = nil
	for _, read := range previous.Reads {
		if read.Source == "observations" {
			read.Operation = operation
			read.Statistics = nil
			plan.Reads = append(plan.Reads, read)
		}
	}
	return plan, len(plan.Reads) > 0
}

func requestedStatistics(question string) []string {
	var operations []string
	for _, item := range []struct {
		Words     []string
		Operation string
	}{
		{[]string{"最高", "最大"}, "max"},
		{[]string{"最低", "最小"}, "min"},
		{[]string{"平均", "均值"}, "mean"},
	} {
		for _, word := range item.Words {
			if strings.Contains(question, word) {
				operations = append(operations, item.Operation)
				break
			}
		}
	}
	return operations
}
