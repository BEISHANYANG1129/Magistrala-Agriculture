package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func LoadDataset(path string) (*Dataset, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Dataset
	if err = json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if d.Version == "" || d.WorkspaceID == "" || d.TimeKind != "receiver_time_not_sensor_sampling_time" {
		return nil, fmt.Errorf("invalid historical dataset manifest")
	}
	// Previously imported manifests marked the already-scaled kPa channel as
	// pending. Keep the recorded values and update only this unit metadata.
	updated := false
	for i := range d.Metrics {
		m := &d.Metrics[i]
		if m.Key == "air_pressure" && m.Unit == "kPa" && m.UnitConfirmation == "pending" {
			m.UnitConfirmation = "confirmed"
			updated = true
		}
	}
	if updated {
		d.Version = fingerprint([]string{d.Version, "air-pressure-kpa-confirmed"})
	}
	return &d, nil
}

func summarize(q Query, frames []Frame, metrics []Metric, version, timeKind string, sources map[string]string) Evidence {
	e := Evidence{Mode: q.Mode, DatasetVersion: version, TimeKind: timeKind, Start: q.Start, End: q.End, Metrics: []Summary{}, Warnings: []string{}, Sources: sources}
	rows := []Frame{}
	for _, f := range frames {
		if !f.ReceivedAt.Before(q.Start) && f.ReceivedAt.Before(q.End) {
			if _, found := f.Values[q.Metric]; q.Metric != "" && !found {
				continue
			}
			rows = append(rows, f)
		}
	}
	e.Rows = len(rows)
	if e.Rows == 0 {
		e.Warnings = append(e.Warnings, "所选窗口没有可用记录，不能据此确定设备在线状态或农田环境。")
	}
	if q.Mode == "history" {
		e.Warnings = append(e.Warnings, "本结果为历史接收日志回放；接收时间不等于传感器采样时间，也不代表当前现场状态。")
	}
	for _, m := range metrics {
		if q.Metric != "" && m.Key != q.Metric {
			continue
		}
		s := Summary{Metric: m, Warnings: []string{}}
		seen := map[float64]bool{}
		var sum float64
		for _, f := range rows {
			v, ok := f.Values[m.Key]
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			if s.Count == 0 {
				s.Min = v
				s.Max = v
				s.FirstAt = f.ReceivedAt
				s.LastAt = f.ReceivedAt
			}
			s.Min = math.Min(s.Min, v)
			s.Max = math.Max(s.Max, v)
			s.Count++
			sum += v
			seen[v] = true
			if f.ReceivedAt.Before(s.FirstAt) {
				s.FirstAt = f.ReceivedAt
			}
			if f.ReceivedAt.After(s.LastAt) {
				s.LastAt = f.ReceivedAt
			}
			if m.Key == "co2" && v < 400 {
				s.BelowRange++
			}
		}
		if s.Count == 0 {
			continue
		}
		s.Distinct = len(seen)
		if m.Admission == "hold_for_validation" {
			s.Warnings = append(s.Warnings, "暂缓准入：仅作数据质量核查，不用于环境或农艺判断。")
		}
		if m.UnitConfirmation == "pending" {
			s.Warnings = append(s.Warnings, "单位暂定，待设备端确认；不用于依赖确定单位的推导。")
		}
		if s.Distinct == 1 {
			s.Warnings = append(s.Warnings, "窗口内为恒值；不能单凭恒值断定设备故障。")
		}
		if s.BelowRange > 0 {
			s.Warnings = append(s.Warnings, "存在低于当前配置量程下限400 ppm的记录；没有进行独立校准。")
		}
		if m.Key != "wind_direction" && m.Key != "wind_direction_code" && m.Key != "wind_force" && m.Admission != "hold_for_validation" && m.UnitConfirmation != "pending" {
			avg := sum / float64(s.Count)
			s.Mean = &avg
		}
		s.ID = "obs-" + fingerprint(struct {
			Q       Query
			Version string
			M       Summary
		}{q, version, s})[:24]
		e.Metrics = append(e.Metrics, s)
	}
	sort.Slice(e.Metrics, func(i, j int) bool { return e.Metrics[i].Key < e.Metrics[j].Key })
	return e
}

func validateQuery(q Query) error {
	if q.Mode != "history" && q.Mode != "platform" {
		return fmt.Errorf("select history or platform data explicitly")
	}
	if q.Start.IsZero() || q.End.IsZero() || !q.Start.Before(q.End) || q.End.Sub(q.Start) > 7*24*time.Hour {
		return fmt.Errorf("explicit time window required, maximum seven days")
	}
	if q.End.After(time.Now().Add(5 * time.Minute)) {
		return fmt.Errorf("future observation windows are not supported")
	}
	return nil
}
