package agent

import (
	"context"
	"encoding/json"
	"magistrala-agri-agent/workbench"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLatestPhotoUsesAuthorizedSourceAndTimeWindow(t *testing.T) {
	photo := "iVBORw0KGgo="
	source := workbench.PhotoSource{WorkspaceID: "w", ChannelID: "c", PublisherID: "p", Metric: "camera/image"}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/w/channels/c/messages" || r.Header.Get("Authorization") != "Bearer session" ||
			q.Get("publisher") != "p" || q.Get("name") != source.Metric || q.Get("from") != "10" || q.Get("to") != "20" {
			http.Error(w, "wrong reader request", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]any{
			{"channel": "c", "publisher": "other", "name": source.Metric, "time": 19, "data_value": photo},
			{"channel": "c", "publisher": "p", "name": source.Metric, "time": 20, "data_value": photo},
			{"channel": "c", "publisher": "p", "name": source.Metric, "time": 15, "data_value": photo},
		}})
	}))
	defer upstream.Close()
	reader := Reader{BaseURL: upstream.URL, TimeUnit: "seconds", Authorization: "Bearer session"}
	message, err := reader.latestPhoto(context.Background(), "w", source, &workbench.Window{Start: time.Unix(10, 0), End: time.Unix(20, 0)})
	if err != nil || message.Encoded != photo || !message.Time.Equal(time.Unix(15, 0)) {
		t.Fatalf("wrong photo: %+v, %v", message, err)
	}
	if _, err := reader.latestPhoto(context.Background(), "other", source, nil); err == nil {
		t.Fatal("cross-workspace camera access allowed")
	}
}
