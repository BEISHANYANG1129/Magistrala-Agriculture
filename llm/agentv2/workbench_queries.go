package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"magistrala-agri-agent/workbench"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

func (p *PlatformAuth) queryDevices(ctx context.Context, workspace string) ([]workbench.QueryDevice, error) {
	auth, _ := ctx.Value(authorizationContext).(string)
	devices := []workbench.QueryDevice{}
	for offset := 0; offset < 10000; offset += 100 {
		var data struct {
			Entities struct {
				Total int
				Items []struct {
					TenantID, ID, Name, Alias, Status string
					DeletedAt                         *time.Time
				}
			}
		}
		if err := p.query(ctx, auth, `query($workspace:ID!,$offset:Int){entities(tenantId:$workspace,kind:device,limit:100,offset:$offset){total items{id tenantId name alias status deletedAt}}}`, map[string]any{"workspace": workspace, "offset": offset}, &data); err != nil {
			return nil, err
		}
		for _, d := range data.Entities.Items {
			if d.ID != "" && d.TenantID == workspace && (d.DeletedAt == nil || d.DeletedAt.IsZero()) {
				devices = append(devices, workbench.QueryDevice{ID: d.ID, Name: d.Name, Alias: d.Alias, Status: d.Status})
			}
		}
		if len(data.Entities.Items) == 0 || offset+len(data.Entities.Items) >= data.Entities.Total {
			return devices, nil
		}
	}
	return nil, workbench.ErrQueryLimit
}

func (p *PlatformAuth) queryPublishingChannels(ctx context.Context, workspace, device string) ([]string, error) {
	auth, _ := ctx.Value(authorizationContext).(string)
	channels := []string{}
	for offset := 0; offset < 10000; offset += 100 {
		var data struct {
			DirectPolicies struct {
				Total int
				Items []struct {
					SubjectID       string
					PermissionBlock struct {
						ObjectID, ObjectKind, ObjectType, Effect string
						Actions                                  []struct{ Name string }
					}
				}
			}
		}
		if err := p.query(ctx, auth, `query($workspace:ID!,$device:ID!,$offset:Int){directPolicies(tenantId:$workspace,subjectKind:entity,subjectId:$device,limit:100,offset:$offset){total items{subjectId permissionBlock{objectId objectKind objectType effect actions{name}}}}}`, map[string]any{"workspace": workspace, "device": device, "offset": offset}, &data); err != nil {
			return nil, err
		}
		for _, item := range data.DirectPolicies.Items {
			block := item.PermissionBlock
			if item.SubjectID != device || block.ObjectKind != "resource" || block.ObjectType != "resource:channel" || block.Effect != "allow" || block.ObjectID == "" {
				continue
			}
			for _, action := range block.Actions {
				if action.Name == "publish" && !slices.Contains(channels, block.ObjectID) {
					channels = append(channels, block.ObjectID)
				}
			}
		}
		if len(data.DirectPolicies.Items) == 0 || offset+len(data.DirectPolicies.Items) >= data.DirectPolicies.Total {
			return channels, nil
		}
	}
	return nil, workbench.ErrQueryLimit
}

func queryJSON(ctx context.Context, endpoint, authorization string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return workbench.ErrQueryDenied
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("platform query returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(dst)
}

func listQueryAlarms(ctx context.Context, baseURL, workspace string, filter workbench.AlarmFilter) ([]workbench.QueryAlarm, int, error) {
	auth, _ := ctx.Value(authorizationContext).(string)
	if auth == "" || requestWorkspace(ctx) != workspace {
		return nil, 0, fmt.Errorf("authenticated workspace required")
	}
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(workspace) + "/alarms")
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, 0, fmt.Errorf("invalid alarms service URL")
	}
	params := url.Values{"limit": {"10"}, "offset": {"0"}, "dir": {"desc"}, "order": {"created_at"}}
	if filter.DeviceID != "" {
		params.Set("device_id", filter.DeviceID)
	}
	if filter.Status != "" {
		params.Set("status", filter.Status)
	}
	if filter.Window != nil {
		params.Set("created_from", filter.Window.Start.UTC().Format(time.RFC3339Nano))
		params.Set("created_to", filter.Window.End.UTC().Format(time.RFC3339Nano))
	}
	endpoint.RawQuery = params.Encode()
	var response struct {
		Total  *int
		Alarms []struct {
			ID        string
			DeviceID  string `json:"device_id"`
			Name      string
			Status    string
			CreatedAt time.Time `json:"created_at"`
		}
	}
	if err := queryJSON(ctx, endpoint.String(), auth, &response); err != nil {
		return nil, 0, err
	}
	if response.Total == nil || *response.Total < len(response.Alarms) {
		return nil, 0, fmt.Errorf("invalid alarm count")
	}
	rows := []workbench.QueryAlarm{}
	for _, row := range response.Alarms {
		if row.ID == "" || row.CreatedAt.IsZero() || (filter.DeviceID != "" && row.DeviceID != filter.DeviceID) || (filter.Status != "" && row.Status != filter.Status) {
			return nil, 0, fmt.Errorf("alarm response outside requested scope")
		}
		name := row.Name
		if name == "" {
			name = "告警 " + row.ID
		}
		rows = append(rows, workbench.QueryAlarm{ID: row.ID, Name: name, Status: row.Status, CreatedAt: row.CreatedAt})
	}
	return rows, *response.Total, nil
}

func (r *Reader) latestQueryMessage(ctx context.Context, workspace string, binding Binding, metric string, window *workbench.Window) (*workbench.Frame, error) {
	if r.TimeUnit != "seconds" && r.TimeUnit != "nanoseconds" {
		return nil, fmt.Errorf("invalid reader time unit")
	}
	endpoint, err := r.messageURL(workspace, binding.ChannelID)
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("publisher", binding.PublisherID)
	params.Set("order", "time")
	params.Set("dir", "desc")
	params.Set("limit", "1")
	params.Set("format", "messages")
	if metric != "" {
		params.Set("name", metric)
	}
	if window != nil {
		setMessageWindow(params, window.Start, window.End, r.TimeUnit)
	}
	endpoint.RawQuery = params.Encode()
	var page readerPage
	if err = queryJSON(ctx, endpoint.String(), r.Authorization, &page); err != nil {
		return nil, err
	}
	for _, m := range page.Messages {
		if m.Publisher != binding.PublisherID || (m.Channel != "" && m.Channel != binding.ChannelID) || (metric != "" && m.Name != metric) {
			return nil, fmt.Errorf("reader returned a message outside requested scope")
		}
		at, err := parseTimestamp(m.Time, r.TimeUnit)
		if err != nil {
			return nil, err
		}
		if window != nil && (at.Before(window.Start) || !at.Before(window.End)) {
			continue
		}
		frame := &workbench.Frame{ReceivedAt: at, Source: binding.ChannelID}
		if metric != "" {
			if m.Value == nil || math.IsNaN(*m.Value) || math.IsInf(*m.Value, 0) {
				return nil, fmt.Errorf("latest metric is not a numeric observation")
			}
			frame.Values = map[string]float64{metric: *m.Value}
			frame.Units = map[string]string{metric: m.Unit}
		}
		return frame, nil
	}
	return nil, nil
}
