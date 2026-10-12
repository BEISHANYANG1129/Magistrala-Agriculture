package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Reader consumes the expanded SenML reader response, not the publish envelope.
type Reader struct {
	BaseURL       string
	Authorization string
	TimeUnit      string
	Client        *http.Client
	PageSize      int
	MaxPages      int
}
type readerMessage struct {
	Channel   string      `json:"channel"`
	Publisher string      `json:"publisher"`
	Name      string      `json:"name"`
	Unit      string      `json:"unit"`
	Time      json.Number `json:"time"`
	Value     *float64    `json:"value"`
}
type readerPage struct {
	Total    *uint64         `json:"total"`
	Messages []readerMessage `json:"messages"`
}

func (r *Reader) Read(ctx context.Context, workspace string, binding Binding, since time.Time) ([]Evidence, error) {
	if r.TimeUnit != "seconds" && r.TimeUnit != "nanoseconds" {
		return nil, errors.New("reader time_unit must be seconds or nanoseconds")
	}
	u, err := url.Parse(r.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid reader base URL")
	}
	// Build escaped segments explicitly; identifiers cannot inject query/path components.
	basePath := u.EscapedPath()
	u.Path = strings.TrimRight(u.Path, "/") + "/" + workspace + "/channels/" + binding.ChannelID + "/messages"
	u.RawPath = strings.TrimRight(basePath, "/") + "/" + url.PathEscape(workspace) + "/channels/" + url.PathEscape(binding.ChannelID) + "/messages"
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	pageSize := r.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	maxPages := r.MaxPages
	if maxPages <= 0 {
		maxPages = 20
	}
	out := []Evidence{}
	for page := 0; page < maxPages; page++ {
		q := u.Query()
		q.Set("offset", strconv.Itoa(page*pageSize))
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("publisher", binding.PublisherID)
		q.Set("order", "time")
		q.Set("dir", "desc")
		q.Set("format", "messages")
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", r.Authorization)
		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("reader request: %w", err)
		}
		b, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("reader returned HTTP %d", res.StatusCode)
		}
		var p readerPage
		if err = json.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("invalid reader response: %w", err)
		}
		if p.Messages == nil {
			return nil, errors.New("reader response is missing messages array")
		}
		for _, m := range p.Messages {
			// Never rely solely on remote filters for workspace/region isolation.
			if m.Publisher != binding.PublisherID || (m.Channel != "" && m.Channel != binding.ChannelID) {
				continue
			}
			if m.Value == nil || math.IsNaN(*m.Value) || math.IsInf(*m.Value, 0) {
				continue
			}
			observed, err := parseTimestamp(m.Time, r.TimeUnit)
			if err != nil {
				return nil, err
			}
			if observed.Before(since) {
				continue
			}
			identity, _ := json.Marshal([]string{workspace, binding.ChannelID, m.Publisher, m.Name, m.Time.String()})
			id := sha256.Sum256(identity)
			out = append(out, Evidence{ID: hex.EncodeToString(id[:]), ChannelID: binding.ChannelID, PublisherID: m.Publisher, Metric: m.Name, Value: *m.Value, Unit: m.Unit, ObservedAt: observed})
		}
		if len(p.Messages) < pageSize || (p.Total != nil && uint64((page+1)*pageSize) >= *p.Total) {
			return out, nil
		}
	}
	return nil, errors.New("reader pagination limit exceeded; refusing incomplete evidence")
}
func parseTimestamp(n json.Number, unit string) (time.Time, error) {
	if unit == "nanoseconds" {
		v, err := n.Int64()
		if err != nil || v <= 0 {
			return time.Time{}, errors.New("invalid nanosecond timestamp")
		}
		return time.Unix(0, v).UTC(), nil
	}
	v, err := n.Float64()
	if err != nil || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) || v > 32503680000 {
		return time.Time{}, errors.New("invalid timestamp; check configured time unit")
	}
	seconds, fraction := math.Modf(v)
	return time.Unix(int64(seconds), int64(fraction*1e9)).UTC(), nil
}
