package workbench

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const calendarDatePattern = `(?:[0-9]{4}年)?[0-9]{1,2}月[0-9]{1,2}[日号]|[0-9]{4}[-/][0-9]{1,2}[-/][0-9]{1,2}|[0-9]{1,2}[日号]|今天|昨天|前天`
const dayPeriodPattern = `凌晨|早上|上午|中午|下午|晚上`
const clockPattern = `(?:(?:` + dayPeriodPattern + `))?[0-9]{1,2}(?::[0-9]{2}|[点时](?:半|[0-9]{1,2}分?)?)`
const dateEndpointPattern = `(?:` + calendarDatePattern + `)(?:的|T)?(?:` + clockPattern + `|` + dayPeriodPattern + `)?`

var numberWords = regexp.MustCompile(`[零〇一二两三四五六七八九十]+(?:年|月|日|号|点|时|分|个|天|周|小时)`)
var isoClockSeparator = regexp.MustCompile(`([0-9]{4}[-/][0-9]{1,2}[-/][0-9]{1,2})(?:\s+|T)([0-9]{1,2}:[0-9]{2})`)
var datePhrase = regexp.MustCompile(dateEndpointPattern + `(?:(?:到|至|~)(?:` + dateEndpointPattern + `|` + clockPattern + `))?|(?:最近|过去)[0-9]+(?:个)?(?:分钟|小时|天|周)`)
var calendarDate = regexp.MustCompile(`^(?:([0-9]{4})年)?([0-9]{1,2})月([0-9]{1,2})[日号]`)
var isoDate = regexp.MustCompile(`^([0-9]{4})[-/]([0-9]{1,2})[-/]([0-9]{1,2})`)
var bareDay = regexp.MustCompile(`^([0-9]{1,2})[日号]`)
var clockTime = regexp.MustCompile(`^(凌晨|早上|上午|中午|下午|晚上)?([0-9]{1,2})(?::([0-9]{2})|[点时](?:(半)|([0-9]{1,2})分?)?)$`)
var relativeWindow = regexp.MustCompile(`^(?:最近|过去)([0-9]+)(?:个)?(分钟|小时|天|周)$`)
var timeReplySuffix = regexp.MustCompile(`^[啊呀呢吧，,。？！?!]*(?:(?:这一天|这1天|这天|那一天|那1天|那天)?(?:有没有数据|没有数据吗|有数据吗|没数据吗|没有记录吗|有没有记录))?[啊呀呢吧，,。？！?!]*$`)

func relativeCount(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	digit := func(s string) int {
		return map[string]int{"零": 0, "〇": 0, "一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9}[s]
	}
	if tens, ones, ok := strings.Cut(s, "十"); ok {
		if tens == "" {
			tens = "一"
		}
		return digit(tens)*10 + digit(ones)
	}
	n := 0
	for _, r := range s {
		n = n*10 + digit(string(r))
	}
	return n
}

func normalizeQueryTime(text string) string {
	text = strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		return r
	}, text)
	text = strings.NewReplacer("：", ":", "～", "~", "—", "~", "–", "~").Replace(text)
	text = isoClockSeparator.ReplaceAllString(text, "${1}T${2}")
	text = strings.Join(strings.Fields(text), "")
	return numberWords.ReplaceAllStringFunc(text, func(word string) string {
		i := strings.IndexFunc(word, func(r rune) bool { return !strings.ContainsRune("零〇一二两三四五六七八九十", r) })
		return strconv.Itoa(relativeCount(word[:i])) + word[i:]
	})
}

// Only one complete expression can supply a shared window. Distinct time
// expressions in a mixed question remain attached to the model's own reads.
func timeExpression(question string) string {
	question = normalizeQueryTime(question)
	parts := datePhrase.FindAllStringIndex(question, -1)
	if len(parts) == 2 && strings.HasPrefix(question, "不是") && strings.Contains(question[parts[0][1]:parts[1][0]], "是") {
		return question[parts[1][0]:parts[1][1]]
	}
	if len(parts) != 1 {
		return ""
	}
	start, end := parts[0][0], parts[0][1]
	if start > 0 && question[start-1] >= '0' && question[start-1] <= '9' {
		return ""
	}
	if strings.HasPrefix(question[end:], "到") || strings.HasPrefix(question[end:], "至") || strings.HasPrefix(question[end:], "~") {
		return ""
	}
	return question[start:end]
}

func timeClarification(question string) string {
	question = normalizeQueryTime(question)
	if expression := timeExpression(question); expression != "" {
		before, after, _ := strings.Cut(question, expression)
		correction := strings.HasPrefix(question, "不是") && strings.HasSuffix(before, "是")
		if (correction || slices.Contains([]string{"", "是", "就是", "我说的是", "我问的是", "查", "查询", "换成", "改成", "那"}, before)) && timeReplySuffix.MatchString(after) {
			return expression
		}
		return ""
	}
	question = strings.Trim(question, "，,。？！?!啊呀呢吧")
	for _, prefix := range []string{"换成", "改成", "那么", "那"} {
		question = strings.TrimPrefix(question, prefix)
	}
	return question
}

type queryEndpoint struct {
	at      time.Time
	hasDate bool
	hasTime bool
	end     time.Time
}

func parseQueryEndpoint(text string, reference time.Time) (queryEndpoint, error) {
	day := time.Date(reference.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, reference.Location())
	dateText := ""
	year, month, dayNumber := reference.Year(), int(reference.Month()), reference.Day()
	switch {
	case calendarDate.MatchString(text):
		parts := calendarDate.FindStringSubmatch(text)
		dateText = parts[0]
		if parts[1] != "" {
			year, _ = strconv.Atoi(parts[1])
		}
		month, _ = strconv.Atoi(parts[2])
		dayNumber, _ = strconv.Atoi(parts[3])
	case isoDate.MatchString(text):
		parts := isoDate.FindStringSubmatch(text)
		dateText = parts[0]
		year, _ = strconv.Atoi(parts[1])
		month, _ = strconv.Atoi(parts[2])
		dayNumber, _ = strconv.Atoi(parts[3])
	case bareDay.MatchString(text):
		parts := bareDay.FindStringSubmatch(text)
		dateText = parts[0]
		dayNumber, _ = strconv.Atoi(parts[1])
	case strings.HasPrefix(text, "今天"):
		dateText = "今天"
	case strings.HasPrefix(text, "昨天"):
		dateText = "昨天"
		day = day.AddDate(0, 0, -1)
	case strings.HasPrefix(text, "前天"):
		dateText = "前天"
		day = day.AddDate(0, 0, -2)
	}
	if dateText != "" && !slices.Contains([]string{"今天", "昨天", "前天"}, dateText) {
		var err error
		day, err = time.ParseInLocation("2006-1-2", fmt.Sprintf("%d-%d-%d", year, month, dayNumber), reference.Location())
		if err != nil {
			return queryEndpoint{}, fmt.Errorf("日期无效，请核对月份和天数")
		}
	}
	remainder := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(text, dateText), "的"), "T")
	result := queryEndpoint{at: day, end: day.AddDate(0, 0, 1), hasDate: dateText != ""}
	if remainder == "" {
		if !result.hasDate {
			return queryEndpoint{}, fmt.Errorf("请提供查询日期")
		}
		return result, nil
	}
	periods := map[string][2]int{"凌晨": {0, 6}, "早上": {6, 12}, "上午": {0, 12}, "中午": {11, 14}, "下午": {12, 18}, "晚上": {18, 24}}
	if bounds, ok := periods[remainder]; ok {
		result.at, result.end = day.Add(time.Duration(bounds[0])*time.Hour), day.Add(time.Duration(bounds[1])*time.Hour)
		return result, nil
	}
	parts := clockTime.FindStringSubmatch(remainder)
	if parts == nil {
		return queryEndpoint{}, fmt.Errorf("请提供明确的日期或小时区间")
	}
	hour, _ := strconv.Atoi(parts[2])
	minute := 0
	if parts[3] != "" {
		minute, _ = strconv.Atoi(parts[3])
	}
	if parts[4] != "" {
		minute = 30
	}
	if parts[5] != "" {
		minute, _ = strconv.Atoi(parts[5])
	}
	period := parts[1]
	if (period != "" && (hour < 1 || hour > 12)) || hour > 24 || minute > 59 || (hour == 24 && minute != 0) {
		return queryEndpoint{}, fmt.Errorf("小时或分钟无效，请核对查询时间")
	}
	if slices.Contains([]string{"下午", "晚上", "中午"}, period) && hour < 12 {
		hour += 12
	}
	if period == "晚上" && hour == 12 {
		hour = 24
	}
	if period == "凌晨" && hour == 12 {
		hour = 0
	}
	result.at = day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	result.hasTime = true
	return result, nil
}

func queryWindow(expression string, now time.Time, references ...time.Time) (Window, error) {
	expression = normalizeQueryTime(expression)
	if expression == "" || expression == "待确认" {
		return Window{}, fmt.Errorf("请提供查询时间，例如「10月26日」或「最近2小时」（单次最多7天）")
	}
	if parts := relativeWindow.FindStringSubmatch(expression); parts != nil {
		n, err := strconv.Atoi(parts[1])
		unit := map[string]time.Duration{"分钟": time.Minute, "小时": time.Hour, "天": 24 * time.Hour, "周": 7 * 24 * time.Hour}[parts[2]]
		if err != nil || n < 1 || n > int(7*24*time.Hour/unit) {
			return Window{}, fmt.Errorf("单次查询最多7天，请缩小时间范围")
		}
		return Window{Start: now.Add(-time.Duration(n) * unit), End: now}, nil
	}
	reference := now
	if len(references) > 0 {
		reference = references[0].In(now.Location())
	}
	expression = strings.NewReplacer("至", "~", "到", "~").Replace(expression)
	from, to, hasRange := strings.Cut(expression, "~")
	if parts := calendarDate.FindStringSubmatch(from); parts != nil && parts[1] == "" {
		reference = time.Date(now.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, now.Location())
	}
	start, err := parseQueryEndpoint(from, reference)
	if err != nil {
		return Window{}, err
	}
	end := start.end
	if hasRange {
		finish, err := parseQueryEndpoint(to, start.at)
		if err != nil {
			return Window{}, err
		}
		end = finish.end
		if finish.hasTime {
			end = finish.at
		}
	} else if start.hasTime {
		return Window{}, fmt.Errorf("请补充小时区间的结束时间")
	}
	if end.After(now) && start.at.Before(now) && !hasRange {
		end = now
	}
	if !start.at.Before(end) {
		return Window{}, fmt.Errorf("结束时间必须晚于开始时间，请核对查询区间")
	}
	if end.After(now) {
		return Window{}, fmt.Errorf("所选日期尚未到来，请提供已发生的观测时间")
	}
	if end.Sub(start.at) > 7*24*time.Hour {
		return Window{}, fmt.Errorf("单次查询最多7天，请缩小时间范围")
	}
	return Window{Start: start.at, End: end}, nil
}
