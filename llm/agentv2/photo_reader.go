package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"magistrala-agri-agent/workbench"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (r *Reader) latestPhoto(ctx context.Context, workspace string, source workbench.PhotoSource, window *workbench.Window) (workbench.PhotoMessage, error) {
	if workspace != source.WorkspaceID || (r.TimeUnit != "seconds" && r.TimeUnit != "nanoseconds") {
		return workbench.PhotoMessage{}, fmt.Errorf("invalid photo source scope")
	}
	u, err := r.messageURL(workspace, source.ChannelID)
	if err != nil {
		return workbench.PhotoMessage{}, err
	}
	q := u.Query()
	q.Set("publisher", source.PublisherID)
	q.Set("name", source.Metric)
	q.Set("order", "time")
	q.Set("dir", "desc")
	q.Set("limit", "2")
	q.Set("format", "messages")
	if window != nil {
		encodeTime := func(t time.Time) string {
			if r.TimeUnit == "nanoseconds" {
				return strconv.FormatInt(t.UnixNano(), 10)
			}
			return strconv.FormatFloat(float64(t.UnixNano())/1e9, 'f', -1, 64)
		}
		q.Set("from", encodeTime(window.Start))
		q.Set("to", encodeTime(window.End))
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return workbench.PhotoMessage{}, err
	}
	req.Header.Set("Authorization", r.Authorization)
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return workbench.PhotoMessage{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return workbench.PhotoMessage{}, fmt.Errorf("photo reader returned HTTP %d", res.StatusCode)
	}
	var page struct {
		Messages []struct {
			readerMessage
			DataValue string `json:"data_value"`
			VD        string `json:"vd"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&page); err != nil {
		return workbench.PhotoMessage{}, err
	}
	var result workbench.PhotoMessage
	for _, row := range page.Messages {
		if row.Publisher != source.PublisherID || (row.Channel != "" && row.Channel != source.ChannelID) || row.Name != source.Metric {
			continue
		}
		at, err := parseTimestamp(row.Time, r.TimeUnit)
		if err != nil || (window != nil && (at.Before(window.Start) || !at.Before(window.End))) {
			continue
		}
		data := row.DataValue
		if data == "" {
			data = row.VD
		}
		data = strings.NewReplacer("-", "+", "_", "/").Replace(strings.TrimSpace(data))
		data = strings.TrimRight(data, "=")
		if data == "" || base64.RawStdEncoding.DecodedLen(len(data)) > 2<<20 {
			continue
		}
		b, err := base64.RawStdEncoding.DecodeString(data)
		if err != nil {
			continue
		}
		candidate := base64.StdEncoding.EncodeToString(b)
		if at.Equal(result.Time) && candidate != result.Encoded {
			return workbench.PhotoMessage{}, fmt.Errorf("ambiguous photo timestamp")
		}
		if at.After(result.Time) {
			result = workbench.PhotoMessage{Encoded: candidate, Time: at}
		}
	}
	if result.Encoded == "" {
		return result, fmt.Errorf("no matching photo")
	}
	return result, nil
}
