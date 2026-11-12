package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"time"
	_ "time/tzdata"
)

type PhotoSource struct {
	WorkspaceID string   `json:"workspace_id"`
	PublisherID string   `json:"publisher_id"`
	ChannelID   string   `json:"channel_id"`
	Name        string   `json:"name"`
	Metric      string   `json:"metric"`
	Aliases     []string `json:"aliases"`
}

type PhotoMessage struct {
	Encoded string
	Time    time.Time
}

type ReadPhoto func(context.Context, string, string, PhotoSource, *Window) (PhotoMessage, error)

type MaterialReference struct {
	Origin      string     `json:"origin"`
	Name        string     `json:"name"`
	ChannelID   string     `json:"channel_id,omitempty"`
	PublisherID string     `json:"publisher_id,omitempty"`
	MessageTime *time.Time `json:"message_time,omitempty"`
	RecordID    string     `json:"record_id,omitempty"`
}

func LoadPhotoSources(path string) ([]PhotoSource, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sources []PhotoSource
	if err := json.Unmarshal(data, &sources); err != nil {
		return nil, err
	}
	for _, source := range sources {
		if source.WorkspaceID == "" || source.PublisherID == "" || source.ChannelID == "" || source.Name == "" || source.Metric == "" {
			return nil, fmt.Errorf("invalid workbench photo source")
		}
	}
	return sources, nil
}

func localTimeZone(name string) (*time.Location, error) {
	if name == "" {
		name = "Asia/Shanghai"
	}
	if name == "Local" || len(name) > 64 {
		return nil, fmt.Errorf("请选择有效时区。")
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("请选择有效时区。")
	}
	return location, nil
}

func stationPublishers(d *Dataset) []string {
	var publishers []string
	for _, metric := range d.Metrics {
		if metric.PublisherID != "" && !slices.Contains(publishers, metric.PublisherID) {
			publishers = append(publishers, metric.PublisherID)
		}
	}
	sort.Strings(publishers)
	return publishers
}

// Material resolvers consume the semantic plan, never reclassify user wording.
func (s *Service) autoObservation(run *Run) (*Query, *Window, *MaterialReference, string) {
	p := run.Automation.GoalPlan.Environment
	d := s.dataset
	if d == nil || d.WorkspaceID != run.WorkspaceID || len(d.Frames) == 0 {
		return nil, nil, nil, "当前工作区缺少已登记的气象来源，请指定设备和时段。"
	}
	if p.Site != "" {
		return nil, nil, nil, fmt.Sprintf("尚未核实“%s”与气象设备的映射，请选择对应设备和时段。", p.Site)
	}
	if p.Station != "" && p.Station != d.StationID {
		return nil, nil, nil, "任务指定的气象站与当前已登记气象站不一致。"
	}
	primary, comparison := p.Window, p.Comparison
	first, last := d.Frames[0].ReceivedAt, d.Frames[len(d.Frames)-1].ReceivedAt
	if primary == nil && p.TimeScope == "all_available" && p.Source != "platform" {
		primary = &Window{Start: first, End: last.Add(time.Second)}
	}
	if primary == nil {
		return nil, nil, nil, "已理解观测分析要求，但尚未确定具体时间。请补充日期、相对时段，或选择观测时段。"
	}
	q := &Query{Mode: "history", Start: primary.Start, End: primary.End}
	withinReplay := func(window *Window) bool {
		return window == nil || (window.Start.Before(last.Add(time.Nanosecond)) && window.End.After(first))
	}
	if p.Source == "platform" || (p.Source == "auto" && (!withinReplay(primary) || !withinReplay(comparison))) {
		if s.cfg.Live == nil || len(stationPublishers(d)) == 0 || d.ChannelID == "" || d.StationID == "" {
			return nil, nil, nil, "缺少已登记的平台气象数据来源，请手动选择设备和通道。"
		}
		q.Mode, q.ChannelID, q.StationID = "platform", d.ChannelID, d.StationID
		var matches []string
		for _, key := range p.Metrics {
			found := false
			for _, metric := range d.Metrics {
				if metric.Key == key {
					found = true
					if metric.PublisherID != "" && !slices.Contains(matches, metric.PublisherID) {
						matches = append(matches, metric.PublisherID)
					}
				}
			}
			if !found {
				return nil, nil, nil, "任务要求的指标未登记，请选择可用的观测设备与指标。"
			}
		}
		if len(matches) == 1 {
			q.PublisherID, q.StationID = matches[0], ""
		}
	}
	if err := validateQuery(*q); err != nil {
		return nil, nil, nil, "识别的时间范围无效或超过七天，请补充有效范围。"
	}
	if comparison != nil {
		other := *q
		other.Start, other.End = comparison.Start, comparison.End
		if validateQuery(other) != nil || (other.Start.Equal(q.Start) && other.End.Equal(q.End)) {
			return nil, nil, nil, "识别的对照时段无效，请补充不同且不超过七天的对照范围。"
		}
	}
	name := "已导入气象日志"
	if q.Mode == "platform" {
		name = "原平台气象站消息"
	}
	if d.StationID != "" {
		name += " · " + d.StationID
	}
	return q, comparison, &MaterialReference{Origin: q.Mode, Name: name, ChannelID: d.ChannelID, PublisherID: q.PublisherID}, ""
}

func (s *Service) autoPhoto(ctx context.Context, auth string, run *Run) string {
	request := *run.Automation.GoalPlan.Image
	p := &request
	// A registered publisher reference has platform provenance. Uploads are
	// already attached before this resolver; a label cannot change provenance.
	if p.Source == "attachment" && p.SourceID != "" {
		for _, source := range s.cfg.PhotoSources {
			if source.WorkspaceID == run.WorkspaceID && source.PublisherID == p.SourceID {
				p.Source = "platform"
				break
			}
		}
	}
	if p.Source == "capture" {
		return "已理解需要新拍摄的照片，但当前只能读取已保存的照片消息。请上传本次拍摄的照片，或说明改用历史照片。"
	}
	if p.Site != "" {
		return fmt.Sprintf("尚未核实“%s”与照片来源的映射，请指定来源或上传对应照片。", p.Site)
	}
	if p.Source == "archive" {
		if p.TimeSpecified {
			return "此前上传的照片缺少可核实的拍摄时段，请指定照片或上传对应图片。"
		}
		rows := s.history(run.WorkspaceID, "image")
		sort.Slice(rows, func(i, j int) bool { return olderRun(rows[j], rows[i]) })
		for _, row := range rows {
			if row.ImageHash == "" || row.ImageHash == run.ImageHash || row.ImageSource != "user_upload" ||
				row.Workflow == nil || row.Workflow.Vision == nil || row.Workflow.Vision.Subject != "leaf" || row.Workflow.Vision.Quality != "usable" {
				continue
			}
			for _, format := range []string{"jpeg", "png"} {
				asset := &imageAsset{ID: row.ImageHash, Format: format, Source: "user_upload"}
				if _, err := s.readImageAsset(asset); err == nil {
					run.Automation.Image = asset
					run.Automation.Photo = &MaterialReference{Origin: "archive", Name: "本工作区此前上传的叶片照片", RecordID: row.ID}
					return ""
				}
			}
		}
		return "本工作区未找到此前上传且可辨识的叶片照片，请提供照片。"
	}
	if p.Source == "attachment" {
		return "任务需要检查所附照片，请上传要分析的图片。"
	}
	var sources []PhotoSource
	for _, source := range s.cfg.PhotoSources {
		if source.WorkspaceID == run.WorkspaceID && ((p.SourceID != "" && p.SourceID == source.PublisherID) || (p.SourceID == "" && p.Target == "" && p.Source == "auto")) {
			sources = append(sources, source)
		}
	}
	if len(sources) != 1 || s.cfg.ReadPhoto == nil {
		return "没有唯一可确认的照片来源，请指定当前工作区已登记的摄像头，或上传照片。"
	}
	window := p.Window
	if window == nil && p.TimeSpecified {
		return "照片查询时间仍有歧义，请补充明确时段或提供照片。"
	}
	if window != nil && validateQuery(Query{Mode: "history", Start: window.Start, End: window.End}) != nil {
		return "照片查询时段无效或超过七天，请补充范围。"
	}
	source := sources[0]
	message, err := s.cfg.ReadPhoto(ctx, run.WorkspaceID, auth, source, window)
	if err != nil {
		return "已登记的照片来源暂无可用消息，请补充照片或调整查询范围。"
	}
	if err := s.attachAutomationImage(run, message.Encoded, "platform_photo", ""); err != nil || run.Automation.Image.ID == run.ImageHash {
		run.Automation.Image = nil
		return "已找到的照片无法用于本次分析，请提供清晰叶片照片。"
	}
	run.Automation.Photo = &MaterialReference{Origin: "platform", Name: source.Name, ChannelID: source.ChannelID, PublisherID: source.PublisherID, MessageTime: &message.Time}
	return ""
}
