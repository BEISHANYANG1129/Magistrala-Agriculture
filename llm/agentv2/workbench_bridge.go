package agent

import (
	"context"
	"fmt"
	"magistrala-agri-agent/workbench"
)

// EnableWorkbench shares the existing request gate, authentication and Reader.
func (s *Server) EnableWorkbench(cfg workbench.Config, readerURL, timeUnit string, alarmsURL ...string) error {
	if s.Platform != nil {
		cfg.CheckCamera = s.Platform.checkWorkbenchCamera
	}
	cfg.Gate = s.gate
	cfg.CheckAutomationManage = func(ctx context.Context, workspace string) error {
		if s.Platform == nil {
			return nil
		}
		auth, _ := ctx.Value(authorizationContext).(string)
		return s.Platform.authorize(ctx, auth, workspace, true)
	}
	cfg.ConfirmationSubject = func(ctx context.Context) string {
		auth, _ := ctx.Value(authorizationContext).(string)
		subject, _ := sessionSubject(auth)
		return subject
	}
	cfg.CheckReplay = func(ctx context.Context, workspace string, d *workbench.Dataset) error {
		if s.Platform == nil {
			return nil
		}
		bindings := []Binding{}
		seen := map[string]bool{}
		for _, m := range d.Metrics {
			if !seen[m.PublisherID] {
				seen[m.PublisherID] = true
				bindings = append(bindings, Binding{ChannelID: d.ChannelID, PublisherID: m.PublisherID})
			}
		}
		return s.Platform.validateBindings(ctx, Registry{Version: "replay-read", Regions: []Region{{ID: "replay", WorkspaceID: workspace, Sensors: bindings}}})
	}
	cfg.Live = func(ctx context.Context, workspace, authorization string, q workbench.Query) ([]workbench.Frame, error) {
		if s.Platform == nil {
			return nil, fmt.Errorf("live platform queries require integrated authentication")
		}
		reg := Registry{Version: "workbench-read", Regions: []Region{{ID: "read", WorkspaceID: workspace, Sensors: []Binding{{ChannelID: q.ChannelID, PublisherID: q.PublisherID}}}}}
		if err := s.Platform.validateBindings(ctx, reg); err != nil {
			return nil, err
		}
		rows, err := (&Reader{BaseURL: readerURL, Authorization: authorization, TimeUnit: timeUnit, MaxPages: 600, Until: q.End, Metric: q.Metric}).Read(ctx, workspace, reg.Regions[0].Sensors[0], q.Start)
		if err != nil {
			return nil, err
		}
		out := []workbench.Frame{}
		for _, row := range rows {
			out = append(out, workbench.Frame{ID: row.ID, ReceivedAt: row.ObservedAt, Source: "platform-reader", Values: map[string]float64{row.Metric: row.Value}, Units: map[string]string{row.Metric: row.Unit}})
		}
		return out, nil
	}
	cfg.ReadPhoto = func(ctx context.Context, workspace, authorization string, source workbench.PhotoSource, window *workbench.Window) (workbench.PhotoMessage, error) {
		if s.Platform == nil || source.WorkspaceID != workspace {
			return workbench.PhotoMessage{}, fmt.Errorf("photo source outside the authorized workspace")
		}
		reg := Registry{Version: "workbench-photo-read", Regions: []Region{{ID: "photo", WorkspaceID: workspace, Sensors: []Binding{{ChannelID: source.ChannelID, PublisherID: source.PublisherID}}}}}
		if err := s.Platform.validateBindings(ctx, reg); err != nil {
			return workbench.PhotoMessage{}, err
		}
		return (&Reader{BaseURL: readerURL, Authorization: authorization, TimeUnit: timeUnit}).latestPhoto(ctx, workspace, source, window)
	}
	if s.Platform != nil && len(alarmsURL) > 0 && alarmsURL[0] != "" {
		cfg.QueryTools = &workbench.QueryTools{
			Devices:  s.Platform.queryDevices,
			Channels: s.Platform.queryPublishingChannels,
			Latest: func(ctx context.Context, workspace, channel, device, metric string, window *workbench.Window) (*workbench.Frame, error) {
				auth, _ := ctx.Value(authorizationContext).(string)
				if auth == "" || requestWorkspace(ctx) != workspace {
					return nil, fmt.Errorf("authenticated workspace required")
				}
				binding := Binding{ChannelID: channel, PublisherID: device}
				reg := Registry{Regions: []Region{{WorkspaceID: workspace, Sensors: []Binding{binding}}}}
				if err := s.Platform.validateBindings(ctx, reg); err != nil {
					return nil, err
				}
				return (&Reader{BaseURL: readerURL, Authorization: auth, TimeUnit: timeUnit}).latestQueryMessage(ctx, workspace, binding, metric, window)
			},
			Alarms: func(ctx context.Context, workspace string, filter workbench.AlarmFilter) ([]workbench.QueryAlarm, int, error) {
				return listQueryAlarms(ctx, alarmsURL[0], workspace, filter)
			},
		}
	}
	w, err := workbench.New(cfg)
	if err != nil {
		return err
	}
	s.Workbench = w
	return nil
}
