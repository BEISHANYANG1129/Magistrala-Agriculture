package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"
)

var (
	ErrQueryDenied = errors.New("platform read permission denied")
	ErrQueryLimit  = errors.New("platform read limit exceeded")
)

func queryFailure(err error) string {
	switch {
	case errors.Is(err, ErrQueryDenied):
		return "当前账号没有读取权限。"
	case errors.Is(err, ErrQueryLimit):
		return "查询范围超过可完整读取的上限，请缩小范围。"
	default:
		return "平台查询失败，请稍后重试。"
	}
}

type QueryDevice struct {
	ID, Name, Alias, Status string
}

type AlarmFilter struct {
	DeviceID, Status string
	Window           *Window
}

type QueryAlarm struct {
	ID, Name, Status string
	CreatedAt        time.Time
}

// QueryTools only exposes scoped reads. Every adapter must use the caller's authorization.
type QueryTools struct {
	Devices  func(context.Context, string) ([]QueryDevice, error)
	Channels func(context.Context, string, string) ([]string, error)
	Latest   func(context.Context, string, string, string, string, *Window) (*Frame, error)
	Alarms   func(context.Context, string, AlarmFilter) ([]QueryAlarm, int, error)
}

type queryIntent struct {
	Source     string   `json:"source"`
	Target     string   `json:"target"`
	Metric     string   `json:"metric"`
	Metrics    []string `json:"metrics,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	Operation  string   `json:"operation"`
	Statistics []string `json:"statistics,omitempty"`
	Time       string   `json:"time"`
	Mode       string   `json:"mode"`
	Status     string   `json:"status"`
	Window     *Window  `json:"resolved_window,omitempty"`
	TimeError  string   `json:"time_error,omitempty"`
}

type queryPlan struct {
	Analyze  bool          `json:"analyze,omitempty"`
	At       time.Time     `json:"at,omitempty"`
	Question string        `json:"question,omitempty"`
	Search   string        `json:"search"`
	Reads    []queryIntent `json:"reads"`
}

type QueryResult struct {
	ID      string       `json:"id"`
	Title   string       `json:"title"`
	Source  string       `json:"source"`
	Text    string       `json:"text"`
	Answer  string       `json:"answer,omitempty"`
	Summary *Summary     `json:"summary,omitempty"`
	Window  *Window      `json:"window,omitempty"`
	Read    *queryIntent `json:"query,omitempty"`
	Facts   []queryFact  `json:"facts,omitempty"`
}

var queryPlanSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["search","reads","analyze"],"properties":{"analyze":{"type":"boolean"},"search":{"type":"string","maxLength":200},"reads":{"type":"array","maxItems":8,"items":{"type":"object","additionalProperties":false,"required":["source","target","metric","operation","time","mode","status"],"properties":{"source":{"enum":["observations","alarms","device","catalog"]},"target":{"type":"string","maxLength":120},"metric":{"type":"string","maxLength":80},"metrics":{"type":"array","maxItems":14,"items":{"type":"string","maxLength":80}},"topic":{"enum":["","weather","environment"]},"operation":{"enum":["","latest","mean","min","max","summary","count","list"]},"time":{"type":"string","maxLength":120},"mode":{"enum":["","platform","history"]},"status":{"enum":["","active","cleared"]}}}}}}`)

func (s *Service) planQA(ctx context.Context, question string, history chatHistory, manual *Query, now time.Time, scopes ...string) (queryPlan, error) {
	workspace := ""
	if len(scopes) > 0 {
		workspace = scopes[0]
	} else if s.dataset != nil {
		workspace = s.dataset.WorkspaceID
	}
	var times []recognizedTime
	if s.cfg.TimeRecognizer != nil {
		previous := history.pending
		if previous == nil {
			previous = history.query
		}
		var err error
		times, err = s.cfg.TimeRecognizer.Recognize(ctx, question, now, queryReference(previous, now))
		if err != nil {
			return queryPlan{}, err
		}
	}
	if asksForCatalog(question) {
		return queryPlan{At: now, Question: question, Reads: []queryIntent{{Source: "catalog", Operation: "list"}}}, nil
	}
	if plan, ok := comparisonFollowup(question, history, times, now); ok {
		return plan, nil
	}
	if s.dataset != nil && s.dataset.WorkspaceID == workspace {
		if plan, ok := metricFollowup(question, history, s.dataset.Metrics, now); ok {
			return plan, nil
		}
	}
	if plan, ok := followupQuery(question, history, now); ok {
		if timeClarification(question) != "" && len(times) == 1 {
			for i := range plan.Reads {
				if plan.Reads[i].Source != "device" {
					plan.Reads[i].Time, plan.Reads[i].Window, plan.Reads[i].TimeError = times[0].Text, times[0].Window, times[0].Error
				}
			}
		}
		return plan, nil
	}
	recent := []string{}
	for _, turn := range history.turns[max(0, len(history.turns)-3):] {
		recent = append(recent, prefixBytes(turn.Question, 1500))
	}
	data, err := json.Marshal(map[string]any{
		"question": question, "recent_questions": recent, "pending": history.pending,
		"last_query": history.query, "manual_observations": manual != nil, "local_date": now.Format("2006-01-02"),
		"time_entities": times,
		"data_catalog":  s.qaCatalog(ctx, workspace),
	})
	if err != nil {
		return queryPlan{}, err
	}
	var plan queryPlan
	err = s.cfg.Model.Generate(ctx, []Message{
		{Role: "system", Content: qaPlanningPrompt},
		{Role: "user", Content: string(data)},
	}, querySchemaWithTimes(times, history), &plan)
	if err != nil {
		return queryPlan{}, err
	}
	plan.Question, plan.At = question, now
	if err := s.expandQAReads(&plan, workspace); err != nil {
		return queryPlan{}, err
	}
	if len(times) > 1 && len(times) <= 8 && len(plan.Reads) == 1 && plan.Reads[0].Source == "observations" && (strings.Contains(question, "分别") || comparisonQuestion.MatchString(question)) {
		read := plan.Reads[0]
		plan.Reads = nil
		for _, entity := range times {
			read.Time = entity.Text
			plan.Reads = append(plan.Reads, read)
		}
	}
	observationMetrics := map[string]bool{}
	for _, read := range plan.Reads {
		if read.Source == "observations" {
			observationMetrics[read.Metric] = true
		}
	}
	seen := map[string]bool{}
	if len(plan.Reads) > maxQAReads || len([]rune(plan.Search)) > 200 {
		return queryPlan{}, fmt.Errorf("查询计划超出允许范围")
	}
	for i, read := range plan.Reads {
		key := read.Source
		if read.Source == "observations" {
			key += ":" + read.Metric + ":" + read.Operation + ":" + read.Time + ":" + read.Target
		}
		if !slices.Contains([]string{"observations", "alarms", "device", "catalog"}, read.Source) || seen[key] ||
			!slices.Contains([]string{"", "latest", "mean", "min", "max", "summary", "count", "list"}, read.Operation) ||
			!slices.Contains([]string{"", "platform", "history"}, read.Mode) ||
			!slices.Contains([]string{"", "active", "cleared"}, read.Status) ||
			len([]rune(read.Target)) > 120 || len([]rune(read.Metric)) > 80 || len([]rune(read.Time)) > 120 {
			return queryPlan{}, fmt.Errorf("查询计划包含无效参数")
		}
		seen[key] = true
		seen[read.Source] = true
		plan.Reads[i].Window = nil
		plan.Reads[i].TimeError = ""
		if read.Source == "observations" && queryFollowup(question) && !currentReading.MatchString(question) && len(requestedStatistics(question)) == 0 && !strings.Contains(question, "样本") && !strings.Contains(question, "条") {
			if prior := previousObservation(history.query, read.Metric); prior != nil {
				plan.Reads[i].Operation = prior.Operation
				plan.Reads[i].Statistics = slices.Clone(prior.Statistics)
			}
		}
		if read.Source == "observations" {
			plan.Reads[i].Mode = "platform"
			if explicitReplay(question) {
				plan.Reads[i].Mode = "history"
			} else if queryFollowup(question) && !currentReading.MatchString(question) && history.query != nil {
				for _, prior := range history.query.Reads {
					if prior.Source == read.Source {
						plan.Reads[i].Mode = prior.Mode
						break
					}
				}
			}
		}
		if s.cfg.TimeRecognizer != nil && read.Source != "device" && read.Source != "catalog" && !(read.Operation == "latest" && currentReading.MatchString(question) && timeExpression(question) == "") {
			if entity := selectRecognizedTime(times, read.Time); entity != nil {
				plan.Reads[i].Time, plan.Reads[i].Window, plan.Reads[i].TimeError = entity.Text, entity.Window, entity.Error
			} else if len(times) > 1 && needsQueryWindow(plan.Reads[i]) {
				plan.Reads[i].TimeError = "请说明每项查询对应的时间范围"
			}
		}
		read = plan.Reads[i]
		// Bind relative time to text the user actually supplied.
		if read.Operation == "latest" && (slices.Contains([]string{"现在", "当前", "最新", "最近一条"}, read.Time) || currentReading.MatchString(question)) && timeExpression(question) == "" {
			plan.Reads[i].Time = ""
		} else if read.Source != "device" && read.Source != "catalog" {
			if expression := timeExpression(question); expression != "" {
				// Use the user's calendar date even if the model omitted it or
				// supplied a year. The backend resolves an omitted year.
				plan.Reads[i].Time = expression
				if window, err := queryWindow(expression, now, queryReference(history.query, now)); err == nil {
					plan.Reads[i].Window = &window
				}
			} else if queryFollowup(question) && len(datePhrase.FindAllString(normalizeQueryTime(question), -1)) == 0 && history.query != nil {
				for _, prior := range history.query.Reads {
					if prior.Source == read.Source {
						plan.Reads[i].Time, plan.Reads[i].Window = prior.Time, prior.Window
						break
					}
				}
			} else if read.Time != "" && !groundedTime(read.Time, question, history) {
				plan.Reads[i].Time = "待确认"
			}
		}
		if read.Source == "observations" {
			stats := requestedStatistics(question)
			if len(stats) > 1 && len(observationMetrics) == 1 || read.Operation == "summary" {
				plan.Reads[i].Operation = "summary"
				plan.Reads[i].Statistics = stats
			}
		}
	}
	completeExplicitReads(&plan, question, seen)
	unique := plan.Reads[:0]
	seenReads := map[string]bool{}
	for _, read := range plan.Reads {
		key := fingerprint(read)
		if !seenReads[key] {
			unique = append(unique, read)
			seenReads[key] = true
		}
	}
	plan.Reads = unique
	if len(plan.Reads) > maxQAReads {
		return queryPlan{}, fmt.Errorf("查询计划超过32项，请缩小范围")
	}
	return plan, nil
}

var explicitAlarmRead = regexp.MustCompile(`(?:哪些|有没有|有无|多少|几条|列出|查看|查询)[^。？！?！]{0,16}告警|告警[^。？！?！]{0,12}(?:有哪些|有多少|有几条)`)
var alarmTimeHint = regexp.MustCompile(`(?:今天|昨天|前天)(?:上午|下午|晚上)?|(?:最近|过去)\s*[0-9一二两三四五六七八九十]+\s*(?:分钟|小时|天)|上周|本周|上个月|本月|[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}`)
var explicitDeviceRead = regexp.MustCompile(`(?:现在|目前|当前)?在线[吗么]|启用[了]?[吗么]|禁用[了]?[吗么]|最后一次.{0,12}(?:消息|收报|上报)`)

// Preserve explicit requests even when the planner omits a branch of a mixed question.
func completeExplicitReads(plan *queryPlan, question string, seen map[string]bool) {
	var deviceTarget string
	for _, read := range plan.Reads {
		if read.Source == "device" {
			deviceTarget = read.Target
		}
	}
	if !seen["alarms"] {
		for _, clause := range strings.FieldsFunc(question, func(r rune) bool { return strings.ContainsRune("。？！?！；;", r) }) {
			if !explicitAlarmRead.MatchString(clause) {
				continue
			}
			read := queryIntent{Source: "alarms", Target: deviceTarget, Operation: "list"}
			if strings.Contains(clause, "多少") || strings.Contains(clause, "几条") {
				read.Operation = "count"
			}
			if strings.Contains(clause, "活动告警") || strings.Contains(clause, "未处理告警") || strings.Contains(clause, "未清除告警") {
				read.Status = "active"
			} else if strings.Contains(clause, "已清除告警") {
				read.Status = "cleared"
			}
			read.Time = alarmTimeHint.FindString(clause)
			plan.Reads = append(plan.Reads, read)
			break
		}
	}
	if !seen["device"] && explicitDeviceRead.MatchString(question) {
		plan.Reads = append(plan.Reads, queryIntent{Source: "device"})
	}
}

func explicitReplay(question string) bool {
	return strings.Contains(question, "回放") || strings.Contains(question, "导入的接收日志")
}

func needsQueryWindow(read queryIntent) bool {
	return read.Time != "" || (read.Source == "observations" && read.Operation != "latest") ||
		(read.Source == "alarms" && read.Operation == "count" && read.Status == "")
}

func groundedTime(expression, question string, history chatHistory) bool {
	compact := func(s string) string { return strings.Join(strings.Fields(s), "") }
	expression = compact(expression)
	if expression == "" {
		return false
	}
	if strings.Contains(compact(question), expression) {
		return true
	}
	if history.pending != nil {
		for _, read := range history.pending.Reads {
			if expression == compact(read.Time) {
				return true
			}
		}
	}
	if sourceFollowup.MatchString(question) {
		for _, turn := range history.turns[max(0, len(history.turns)-3):] {
			if strings.Contains(compact(turn.Question), expression) {
				return true
			}
		}
	}
	return false
}

func mentionedDevices(devices []QueryDevice, question string) []QueryDevice {
	var mentioned []QueryDevice
	question = strings.ToLower(question)
	for _, device := range devices {
		for _, alias := range []string{device.ID, device.Name, device.Alias} {
			if alias != "" && strings.Contains(question, strings.ToLower(alias)) {
				mentioned = append(mentioned, device)
				break
			}
		}
	}
	return mentioned
}

func findQueryDevice(devices []QueryDevice, target, question string) (QueryDevice, string) {
	target = strings.TrimSpace(target)
	mentioned := mentionedDevices(devices, question)
	if len(mentioned) == 1 {
		return mentioned[0], ""
	}
	if len(mentioned) > 1 {
		devices = mentioned
	}
	var exact, partial []QueryDevice
	for _, device := range devices {
		switch {
		case strings.EqualFold(target, device.ID), strings.EqualFold(target, device.Name), device.Alias != "" && strings.EqualFold(target, device.Alias):
			if target != "" {
				exact = append(exact, device)
			}
		case target == "", strings.Contains(strings.ToLower(device.Name), strings.ToLower(target)),
			device.Alias != "" && strings.Contains(strings.ToLower(device.Alias), strings.ToLower(target)):
			partial = append(partial, device)
		}
	}
	if len(exact) != 0 {
		partial = exact
	}
	if len(partial) == 1 {
		return partial[0], ""
	}
	if len(partial) == 0 {
		return QueryDevice{}, "没有找到可查询的匹配设备，请提供设备名称或 ID。"
	}
	choices := []string{}
	for _, device := range partial[:min(len(partial), 5)] {
		choices = append(choices, fmt.Sprintf("%s（%s）", device.Name, device.ID))
	}
	return QueryDevice{}, "请指定要查询的设备，例如 " + strings.Join(choices, "、") + "。"
}

func observationMetric(metrics []Metric, name string) (Metric, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, m := range metrics {
		if strings.EqualFold(m.Key, name) || strings.EqualFold(m.Label, name) || slices.ContainsFunc(metricAliases[m.Key], func(alias string) bool { return strings.EqualFold(alias, name) }) {
			return m, true
		}
	}
	return Metric{}, false
}

func (r *Run) queryResult(title, source, text string) *QueryResult {
	result := QueryResult{Title: title, Source: source, Text: text}
	result.ID = "data-" + fingerprint(result)[:24]
	result.Answer = readableUnit(text)
	r.QueryResults = append(r.QueryResults, result)
	return &r.QueryResults[len(r.QueryResults)-1]
}

func (s *Service) autoQA(ctx context.Context, authorization string, in *Input, r *Run, plan queryPlan, now time.Time, events *chatStream) {
	tools := s.cfg.QueryTools
	dataset := s.dataset
	if dataset != nil && dataset.WorkspaceID != r.WorkspaceID {
		dataset = nil
	}
	var devices []QueryDevice
	var deviceErr error
	devicesRead := false
	for i, read := range plan.Reads {
		if ctx.Err() != nil {
			return
		}
		if read.Source == "observations" && in.Query != nil {
			continue
		}
		if read.Source == "catalog" {
			s.queryCatalog(ctx, r)
			continue
		}
		if read.TimeError != "" {
			r.Clarification = read.TimeError + "。"
			continue
		}
		var window *Window
		if read.Source != "device" && read.Window != nil {
			if err := validateQuery(Query{Mode: "platform", Start: read.Window.Start, End: read.Window.End}); err != nil {
				r.Clarification = "请提供有效的查询时间，单次最多7天。"
				continue
			}
			window = read.Window
		} else if read.Source != "device" && needsQueryWindow(read) {
			parsed, err := queryWindow(read.Time, now)
			if err != nil {
				r.Clarification = err.Error() + "。"
				continue
			}
			window = &parsed
			plan.Reads[i].Window = window
		}
		if read.Source == "observations" && !slices.Contains([]string{"latest", "mean", "min", "max", "summary", "count"}, read.Operation) {
			r.Clarification = "请说明要查最新值、平均值、最小值、最大值还是样本数。"
			continue
		}
		if read.Source == "observations" && strings.TrimSpace(read.Metric) == "" {
			r.Clarification = "请告诉我想查哪个气象指标，例如气温或气压。"
			continue
		}
		var device QueryDevice
		var metric Metric
		if read.Source == "observations" {
			var ok bool
			if dataset != nil {
				metric, ok = observationMetric(dataset.Metrics, read.Metric)
			}
			if !ok || metric.Admission == "hold_for_validation" {
				r.Limitations = append(r.Limitations, "所请求的气象指标尚未登记为可用观测。")
				continue
			}
		}
		station := read.Source == "observations" && dataset != nil &&
			(read.Target == "" || read.Target == dataset.StationID || slices.Contains([]string{"气象站", "本站", "本气象站"}, read.Target))
		if !station {
			workspaceAlarms := read.Source == "alarms" && read.Target == ""
			questionText := plan.Question + " " + in.Question
			if !devicesRead {
				if tools.Devices != nil {
					devices, deviceErr = tools.Devices(ctx, r.WorkspaceID)
				} else {
					deviceErr = ErrQueryDenied
				}
				devicesRead = true
			}
			if deviceErr != nil {
				r.Limitations = append(r.Limitations, "设备列表："+queryFailure(deviceErr))
				continue
			}
			if !workspaceAlarms || len(mentionedDevices(devices, questionText)) != 0 {
				var question string
				device, question = findQueryDevice(devices, read.Target, questionText)
				if question != "" {
					r.Clarification = question
					continue
				}
			}
		}
		events.stage("platform_query")
		switch read.Source {
		case "observations":
			if !station && device.ID != metric.PublisherID {
				r.Limitations = append(r.Limitations, "所选设备没有登记该气象指标。")
				continue
			}
			s.queryObservation(ctx, authorization, r, read, dataset, metric, window, now)
		case "alarms":
			s.queryAlarms(ctx, r, read, device, window, now)
		case "device":
			if read.Time != "" && !slices.Contains([]string{"现在", "当前", "最新", "最近一条"}, read.Time) {
				r.Limitations = append(r.Limitations, "设备状态查询提供当前启停状态及可查询消息，历史连接状态无法确认。")
			}
			s.queryDeviceStatus(ctx, r, device, dataset, now, plan.Question+" "+in.Question)
		}
	}
	if len(plan.Reads) > 0 {
		r.query = &plan
	}
	if r.Clarification != "" {
		if plan.Search == "" {
			plan.Search = prefixBytes(plan.Question, 600)
		}
		r.pending = &plan
	}
}

func (s *Service) queryObservation(ctx context.Context, authorization string, r *Run, read queryIntent, d *Dataset, metric Metric, window *Window, now time.Time) {
	source := "平台 Reader"
	if read.Mode == "history" {
		source = "历史接收日志回放"
	}
	title := metric.Label
	if title == "" {
		title = metric.Key
	}
	target := d.StationID
	if read.Target != "" {
		target = read.Target
	}
	title += " · " + target
	if read.Operation == "latest" {
		var latest *Frame
		var err error
		if read.Mode == "history" {
			if s.cfg.CheckReplay != nil {
				err = s.cfg.CheckReplay(ctx, r.WorkspaceID, d)
			}
			if err == nil {
				for i := range d.Frames {
					frame := &d.Frames[i]
					if window != nil && (frame.ReceivedAt.Before(window.Start) || !frame.ReceivedAt.Before(window.End)) {
						continue
					}
					if _, ok := frame.Values[metric.Key]; ok && (latest == nil || frame.ReceivedAt.After(latest.ReceivedAt)) {
						latest = frame
					}
				}
			}
		} else {
			latest, err = s.cfg.QueryTools.Latest(ctx, r.WorkspaceID, d.ChannelID, metric.PublisherID, metric.Key, window)
		}
		if err != nil {
			r.Limitations = append(r.Limitations, title+"："+queryFailure(err))
			return
		}
		if latest == nil {
			text := "未查到该指标的可用记录。"
			if window != nil {
				text = displayWindow(*window) + "，" + text
			}
			result := r.queryResult(title, source, text)
			result.Answer = missingObservationAnswer(metric, window, now)
			result.Window, result.Read = window, &read
			return
		}
		if read.Mode != "history" && latest.Units[metric.Key] != metric.Unit {
			r.queryResult(title, source, "最近记录的单位与已登记单位不符，暂不可作为有效读数。").Answer = "查到了最近的" + observationName(metric) + "记录，但单位与登记口径不一致，暂时无法作为有效读数。"
			return
		}
		note := ""
		if metric.UnitConfirmation == "pending" {
			note = "，单位暂定"
		}
		result := r.queryResult(title, source, fmt.Sprintf("最近一条记录为 %g %s，记录时间 %s%s。", latest.Values[metric.Key], metric.Unit, displayTime(latest.ReceivedAt.In(now.Location())), note))
		result.Answer = latestObservationAnswer(metric, *latest, now)
		result.Window, result.Read = window, &read
		return
	}
	mode := "platform"
	if read.Mode == "history" {
		mode = "history"
	}
	e, err := s.evidence(ctx, r.WorkspaceID, authorization, Query{
		Mode: mode, ChannelID: d.ChannelID, PublisherID: metric.PublisherID, Metric: metric.Key,
		Start: window.Start, End: window.End,
	})
	if err != nil {
		r.Limitations = append(r.Limitations, title+"："+queryFailure(err))
		return
	}
	r.Evidence = &e
	for _, summary := range e.Metrics {
		if summary.Key != metric.Key || summary.Admission == "hold_for_validation" {
			continue
		}
		text := fmt.Sprintf("%s，有效样本数 %d", displayWindow(*window), summary.Count)
		switch read.Operation {
		case "mean":
			if summary.Mean == nil {
				text += "，该指标在当前质量规则下未提供均值"
			} else {
				text += fmt.Sprintf("，平均值 %.2f %s", *summary.Mean, summary.Unit)
			}
		case "min":
			text += fmt.Sprintf("，最小值 %g %s", summary.Min, summary.Unit)
		case "max":
			text += fmt.Sprintf("，最大值 %g %s", summary.Max, summary.Unit)
		case "summary":
			stats := read.Statistics
			if len(stats) == 0 {
				stats = []string{"max", "min", "mean"}
			}
			if slices.Contains(stats, "max") {
				text += fmt.Sprintf("，最大值 %g %s", summary.Max, summary.Unit)
			}
			if slices.Contains(stats, "min") {
				text += fmt.Sprintf("，最小值 %g %s", summary.Min, summary.Unit)
			}
			if slices.Contains(stats, "mean") && summary.Mean != nil {
				text += fmt.Sprintf("，平均值 %.2f %s", *summary.Mean, summary.Unit)
			}
		}
		if summary.UnitConfirmation == "pending" {
			text += "，单位暂定"
		}
		result := r.queryResult(title, source, text+"。")
		result.Answer = observationAnswer(read, summary, *window, now)
		result.Summary, result.Window, result.Read = &summary, window, &read
		if len(summary.Warnings) > 0 {
			result.Text += strings.Join(summary.Warnings, " ")
		}
		return
	}
	result := r.queryResult(title, source, displayWindow(*window)+"，没有符合指标准入条件的记录。")
	result.Answer = missingObservationAnswer(metric, window, now)
	result.Window, result.Read = window, &read
}

func displayTime(t time.Time) string { return t.Format("2006-01-02 15:04:05 -07:00") }

func displayWindow(w Window) string {
	return displayTime(w.Start) + " 至 " + displayTime(w.End)
}

func (s *Service) queryAlarms(ctx context.Context, r *Run, read queryIntent, device QueryDevice, window *Window, now time.Time) {
	rows, total, err := s.cfg.QueryTools.Alarms(ctx, r.WorkspaceID, AlarmFilter{DeviceID: device.ID, Status: read.Status, Window: window})
	if err != nil {
		r.Limitations = append(r.Limitations, "告警："+queryFailure(err))
		return
	}
	scope := "当前工作区"
	if device.ID != "" {
		scope = device.Name
	}
	answerScope := scope
	if window != nil {
		scope += "，" + displayWindow(*window)
		answerScope = observationPeriod(*window, now) + "，" + answerScope
	}
	category := "告警"
	switch read.Status {
	case "active":
		scope += "，活动告警"
		category = "活动告警"
	case "cleared":
		scope += "，已清除告警"
		category = "已清除的告警"
	}
	r.queryResult("告警查询", "平台告警", fmt.Sprintf("%s，共 %d 条记录。", scope, total)).Answer = fmt.Sprintf("%s有 %d 条%s。", answerScope, total, category)
	if read.Operation == "count" {
		return
	}
	for _, row := range rows {
		state := row.Status
		if state == "active" {
			state = "活动中"
		}
		if state == "cleared" {
			state = "已清除"
		}
		r.queryResult(row.Name, "平台告警 · "+row.ID, fmt.Sprintf("状态 %s，创建时间 %s。", row.Status, displayTime(row.CreatedAt.In(now.Location())))).Answer = row.Name + "，目前" + state + "。"
	}
	if total > len(rows) {
		r.Limitations = append(r.Limitations, "告警明细仅展示最新一页，数量来自平台返回的总数。")
	}
}

func (s *Service) queryDeviceStatus(ctx context.Context, r *Run, device QueryDevice, dataset *Dataset, now time.Time, question string) {
	state := "平台未提供明确的启停状态"
	switch device.Status {
	case "enabled", "active":
		state = "已启用"
	case "disabled", "inactive":
		state = "已禁用"
	}
	answer := device.Name + "目前" + state + "。"
	if strings.Contains(question, "在线") || strings.Contains(question, "离线") {
		answer += "实时在线状态暂时无法确认。"
	}
	r.queryResult(device.Name, "平台设备 · "+device.ID, "管理状态："+state+"。").Answer = answer
	channels, err := s.cfg.QueryTools.Channels(ctx, r.WorkspaceID, device.ID)
	if err != nil {
		r.Limitations = append(r.Limitations, "发布通道："+queryFailure(err))
	}
	if dataset != nil {
		for _, m := range dataset.Metrics {
			if m.PublisherID == device.ID && !slices.Contains(channels, dataset.ChannelID) {
				channels = append(channels, dataset.ChannelID)
			}
		}
	}
	var latest *Frame
	incomplete := err != nil
	for _, channel := range channels {
		row, err := s.cfg.QueryTools.Latest(ctx, r.WorkspaceID, channel, device.ID, "", nil)
		if err != nil {
			incomplete = true
			continue
		}
		if row != nil && (latest == nil || row.ReceivedAt.After(latest.ReceivedAt)) {
			latest = row
		}
	}
	if latest != nil {
		text := "最近可查消息时间 " + displayTime(latest.ReceivedAt.In(now.Location())) + "。"
		if incomplete {
			text += "部分通道查询未完成。"
		}
		at := latest.ReceivedAt.In(now.Location())
		answer := device.Name + "最后一条可查消息记录于" + answerDate(at, now) + " " + at.Format("15:04") + "。"
		if incomplete {
			answer += "部分通道未完成查询，结果可能不完整。"
		}
		r.queryResult(device.Name, "平台 Reader · "+latest.Source, text).Answer = answer
	} else if incomplete {
		r.Limitations = append(r.Limitations, "最近消息查询未完成。")
	} else {
		r.queryResult(device.Name, "平台 Reader", "在可查询通道中没有找到设备消息。").Answer = "没有查到" + device.Name + "的设备消息。"
	}
	r.Limitations = append(r.Limitations, "设备启停与最近可查消息仅说明管理状态和消息时间。当前无法确认实时在线。")
}
