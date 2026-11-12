package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

type TimeRecognizer struct {
	Node, Script string
	status       atomic.Int32
}

var ErrTimeRecognizerUnavailable = errors.New("时间解析组件不可用")

func (r *TimeRecognizer) Status() string {
	switch r.status.Load() {
	case 1:
		return "ready"
	case -1:
		return "unavailable"
	default:
		return "unchecked"
	}
}

type recognizedTime struct {
	Text   string  `json:"text"`
	Window *Window `json:"window,omitempty"`
	Error  string  `json:"error,omitempty"`
}

func querySchemaWithTimes(times []recognizedTime, history chatHistory) any {
	if len(times) == 0 {
		return queryPlanSchema
	}
	values := []string{""}
	for _, entity := range times {
		if !slices.Contains(values, entity.Text) {
			values = append(values, entity.Text)
		}
	}
	for _, prior := range []*queryPlan{history.pending, history.query} {
		if prior != nil {
			for _, read := range prior.Reads {
				if !slices.Contains(values, read.Time) {
					values = append(values, read.Time)
				}
			}
		}
	}
	var schema map[string]any
	_ = json.Unmarshal(queryPlanSchema, &schema)
	reads := schema["properties"].(map[string]any)["reads"].(map[string]any)
	timeField := reads["items"].(map[string]any)["properties"].(map[string]any)["time"].(map[string]any)
	timeField["enum"] = values
	return schema
}

type timeEntity struct {
	Text       string
	Resolution struct {
		Values []struct {
			Type, Timex, Value, Start, End string
		}
	}
}

var timeRangeConnector = regexp.MustCompile(`到|至|~|～`)

func (r *TimeRecognizer) Recognize(ctx context.Context, question string, now, reference time.Time) ([]recognizedTime, error) {
	requestContext := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	wall := func(t time.Time) string { return t.Format("2006-01-02T15:04:05") }
	payload, _ := json.Marshal(map[string]string{"text": question, "now": wall(now), "reference": wall(reference)})
	command := exec.CommandContext(ctx, r.Node, r.Script)
	command.Stdin = bytes.NewReader(payload)
	output, err := command.Output()
	if err != nil || len(output) > 1<<20 {
		if requestContext.Err() != nil {
			return nil, requestContext.Err()
		}
		r.status.Store(-1)
		return nil, fmt.Errorf("%w，请检查 Node.js 和 time-recognizer 依赖", ErrTimeRecognizerUnavailable)
	}
	var entities []timeEntity
	if err := json.Unmarshal(output, &entities); err != nil {
		r.status.Store(-1)
		return nil, fmt.Errorf("%w，组件返回了无效结果", ErrTimeRecognizerUnavailable)
	}
	r.status.Store(1)
	var results []recognizedTime
	for _, entity := range entities {
		resolution := recognizedTime{Text: entity.Text}
		for _, value := range entity.Resolution.Values {
			var start, end time.Time
			var err error
			parseDate := func(s string) (time.Time, error) {
				return time.ParseInLocation("2006-01-02", s, now.Location())
			}
			parseMoment := func(s string) (time.Time, error) {
				return time.ParseInLocation("2006-01-02 15:04:05", s, now.Location())
			}
			switch value.Type {
			case "date":
				start, err = parseDate(value.Value)
				end = start.AddDate(0, 0, 1)
			case "daterange":
				start, err = parseDate(value.Start)
				if err == nil {
					end, err = parseDate(value.End)
				}
				// Explicit day ranges include the named final day in this platform.
				if timeRangeConnector.MatchString(entity.Text) {
					end = end.AddDate(0, 0, 1)
				}
			case "datetimerange":
				start, err = parseMoment(value.Start)
				if err == nil {
					end, err = parseMoment(value.End)
				}
			case "timerange":
				day := reference.Format("2006-01-02")
				start, err = parseMoment(day + " " + value.Start)
				if err == nil {
					end, err = parseMoment(day + " " + value.End)
				}
			default:
				resolution.Error = "请提供明确的查询时间区间"
				continue
			}
			if err != nil {
				resolution.Error = "日期无效，请核对月份和天数"
				continue
			}
			// Month/day questions use the current year, including when that
			// leads to a future-date clarification instead of another year.
			if strings.Contains(value.Timex, "XXXX") && start.Year() != now.Year() {
				continue
			}
			// The application's agreed day-part boundaries take precedence
			// over a library's generic definition of afternoon or evening.
			if strings.HasSuffix(entity.Text, "下午") || strings.HasSuffix(entity.Text, "上午") || strings.HasSuffix(entity.Text, "晚上") {
				if policy, err := queryWindow(entity.Text, now, reference); err == nil {
					start, end = policy.Start, policy.End
				}
			}
			if end.After(now) && start.Before(now) && !timeRangeConnector.MatchString(entity.Text) {
				end = now
			}
			switch {
			case !start.Before(end):
				resolution.Error = "结束时间必须晚于开始时间，请核对查询区间"
			case end.After(now):
				resolution.Error = "所选日期尚未到来，请提供已发生的观测时间"
			case end.Sub(start) > 7*24*time.Hour:
				resolution.Error = "单次查询最多7天，请缩小时间范围"
			default:
				resolution.Window = &Window{Start: start, End: end}
				resolution.Error = ""
			}
			break
		}
		if resolution.Window == nil && resolution.Error == "" {
			resolution.Error = "请提供明确的查询日期"
		}
		results = append(results, resolution)
	}
	return results, nil
}

func selectRecognizedTime(times []recognizedTime, expression string) *recognizedTime {
	if len(times) == 1 {
		return &times[0]
	}
	normalized := normalizeQueryTime(expression)
	if normalized != "" {
		for i := range times {
			if normalizeQueryTime(times[i].Text) == normalized {
				return &times[i]
			}
		}
	}
	return nil
}
