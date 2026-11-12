package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatFlushesThinkingAndVerifiedQuoteBeforeOllamaFinishes(t *testing.T) {
	answer := make(chan struct{})
	finish := make(chan struct{})
	quote := "气压单位暂定。"
	thought := "先查阅相关资料。"
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream bool `json:"stream"`
			Think  bool `json:"think"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || !request.Stream || !request.Think {
			http.Error(w, "expected streaming with thinking enabled", 400)
			return
		}
		enc := json.NewEncoder(w)
		_ = enc.Encode(map[string]any{"message": map[string]string{"thinking": thought}, "done": false})
		w.(http.Flusher).Flush()
		select {
		case <-answer:
		case <-r.Context().Done():
			return
		}
		first := `{"claims":[{"kind":"knowledge","text":"` + quote + `","citations":["source"]}`
		_ = enc.Encode(map[string]any{"message": Message{Content: first}, "done": false})
		w.(http.Flusher).Flush()
		select {
		case <-finish:
		case <-r.Context().Done():
			return
		}
		_ = enc.Encode(map[string]any{"message": Message{Content: `],"limitations":[]}`}, "done": true})
	}))
	defer model.Close()
	s := &Service{cfg: Config{Model: &Ollama{URL: model.URL}, Gate: make(chan struct{}, 1)}, runs: map[string]Run{}, knowledge: []Chunk{{ID: "source", Title: "气压单位", Text: quote, Status: "reviewed"}}}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.Handle(w, r, "w", "") }))
	defer api.Close()
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Post(api.URL+"/v2/workbench/chat", "application/json", strings.NewReader(`{"task":"qa","question":"气压单位"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	d := json.NewDecoder(resp.Body)
	thoughtReceived := false
	for {
		var event chatEvent
		if err := d.Decode(&event); err != nil {
			t.Fatalf("thinking or quote was buffered until model completion: %v", err)
		}
		if event.Type == "thinking" {
			if thoughtReceived || event.Delta != thought {
				t.Fatalf("unexpected thinking: %+v", event)
			}
			thoughtReceived = true
			close(answer)
			continue
		}
		if event.Type == "claim" {
			if !thoughtReceived || event.Claim.Text != quote {
				t.Fatalf("unexpected quote: %+v", event.Claim)
			}
			close(finish)
			break
		}
		if event.Type != "stage" {
			t.Fatalf("unexpected event: %+v", event)
		}
	}
	for {
		var event chatEvent
		if err := d.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "result" {
			if event.Run.Status != "completed" || len(event.Run.Claims) != 1 {
				t.Fatalf("unexpected result: %+v", event.Run)
			}
			break
		}
		if event.Type != "stage" {
			t.Fatalf("unexpected event: %+v", event)
		}
	}
}

func TestConversationUsesOnlyWorkspaceQuestionHistory(t *testing.T) {
	s := &Service{runs: map[string]Run{
		"first":  {Task: "qa", WorkspaceID: "w", Question: "气压单位"},
		"second": {Task: "qa", WorkspaceID: "w", ParentID: "first", Question: "需要核实什么"},
	}}
	in := Input{Task: "qa", ParentID: "second"}
	history, err := s.conversation("w", in)
	if err != nil || len(history.turns) != 2 || history.turns[0].Question != "气压单位" {
		t.Fatalf("conversation: %+v, %v", history, err)
	}
	if _, err := s.conversation("other", in); err == nil {
		t.Fatal("accepted another workspace's conversation")
	}
}
