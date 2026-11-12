package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestConversationIncludesAllSavedTurnsWhenTheyFit(t *testing.T) {
	s := &Service{runs: map[string]Run{}}
	parent := ""
	longAnswer := strings.Repeat("内容", 2600)
	for i := 0; i < 12; i++ {
		id := fmt.Sprint(i)
		s.runs[id] = Run{ID: id, WorkspaceID: "w", Task: "qa", ParentID: parent, Question: id,
			Claims: []Claim{{Text: longAnswer}}, Knowledge: []Chunk{{Text: "never include raw retrieved evidence"}},
			Trace: []Trace{{Detail: "never include internal details"}},
		}
		parent = id
	}
	history, err := s.conversation("w", Input{Task: "qa", ParentID: parent})
	if err != nil {
		t.Fatal(err)
	}
	turns, summary, _, err := s.fitConversation(context.Background(), history, modelContextTokens, nil)
	if err != nil || len(turns) != 12 || summary != "" || turns[0].Question != "0" || turns[0].Answer != longAnswer {
		t.Fatalf("expected all turns uncut, got %d with summary %q: %v", len(turns), summary, err)
	}
	raw, _ := json.Marshal(turns)
	if strings.Contains(string(raw), "never include") {
		t.Fatal("history includes internal details or raw RAG")
	}
}

type summaryModel struct {
	calls int
	input string
}

func (m *summaryModel) Generate(_ context.Context, messages []Message, _ any, dst any) error {
	m.calls++
	m.input = messages[1].Content
	return json.Unmarshal([]byte(`{"summary":"用户关注早期水泵状态，型号仍待核实。"}`), dst)
}

func TestCompactionKeepsRecentTurnsAndReusesSummary(t *testing.T) {
	model := &summaryModel{}
	s := &Service{cfg: Config{Model: model}}
	history := chatHistory{}
	for i := 0; i < 24; i++ {
		id := fmt.Sprint(i)
		history.turns = append(history.turns, chatTurn{id: id, Question: "q" + id, Answer: strings.Repeat("早期内容", 80)})
	}
	const budget = 12000
	recent, summary, memory, err := s.fitConversation(context.Background(), history, budget, nil)
	if err != nil || memory == nil || summary == "" || len(recent) == 0 || len(recent) >= 24 {
		t.Fatalf("expected a compacted prefix and recent original turns: %d %v %v", len(recent), memory, err)
	}
	if last := recent[len(recent)-1]; last.Question != history.turns[23].Question || last.Answer != history.turns[23].Answer {
		t.Fatal("latest turn changed")
	}
	if historySize(recent, summary) > budget || !strings.Contains(model.input, `"question":"q0"`) ||
		strings.Contains(model.input, `"question":"q23"`) {
		t.Fatal("compaction did not split oldest turns from the recent suffix")
	}
	history.compaction = memory
	_, next, updated, err := s.fitConversation(context.Background(), history, budget, nil)
	if err != nil || next != summary || updated != nil || model.calls != 1 {
		t.Fatalf("summary was not reused: %d %q %v", model.calls, next, err)
	}
}

func TestFollowupRehydratesOnlyApplicableCitedVersion(t *testing.T) {
	source := Chunk{ID: "source", Version: "v1", Status: "reviewed", WorkspaceID: "w", Crop: "coffee", Text: "气压单位暂定。"}
	run := Run{ID: "run", Question: "气压", Knowledge: []Chunk{source, {ID: "unused", Version: "v1"}},
		Claims: []Claim{{Kind: "knowledge", Text: source.Text, Citations: []string{source.ID}}}}
	turn := turnFor(run)
	if len(turn.sources) != 1 {
		t.Fatalf("expected only actually cited source, got %v", turn.sources)
	}
	history := chatHistory{archive: extractMemory(nil, []chatTurn{turn})}
	s := &Service{knowledge: []Chunk{source}}
	if got := s.chatKnowledge("刚才那份呢？", "w", "coffee", history); len(got) != 1 || got[0].ID != source.ID {
		t.Fatalf("prior source not reloaded: %+v", got)
	}
	if got := s.chatKnowledge("刚才那份呢？", "other", "coffee", history); len(got) != 0 {
		t.Fatal("source leaked across workspaces")
	}
	if got := s.chatKnowledge("刚才那份呢？", "w", "cotton", history); len(got) != 0 {
		t.Fatal("source leaked across crops")
	}
	s.knowledge[0].Version = "v2"
	if got := s.chatKnowledge("刚才那份呢？", "w", "coffee", history); len(got) != 0 {
		t.Fatal("reattached an unverified source version")
	}
}

func TestConversationsPreserveAllTurnsAndSeparateTaskHistory(t *testing.T) {
	s := fixture(t, nil)
	now := time.Now().Add(-time.Hour)
	parent := ""
	for i := 0; i < 202; i++ {
		run := Run{ID: fmt.Sprint(i), Task: "qa", WorkspaceID: "w", ParentID: parent,
			Question: fmt.Sprintf("question %d", i), CreatedAt: now.Add(time.Duration(i) * time.Second)}
		if err := s.store(run); err != nil {
			t.Fatal(err)
		}
		parent = run.ID
	}
	for _, run := range []Run{
		{ID: "second", Task: "qa", WorkspaceID: "w", Question: "another chat", CreatedAt: now.Add(4 * time.Minute)},
		{ID: "execution", Task: "execution", WorkspaceID: "w", Question: "a simulation", CreatedAt: now.Add(5 * time.Minute)},
		{ID: "private", Task: "qa", WorkspaceID: "other", Question: "other workspace", CreatedAt: now},
	} {
		if err := s.store(run); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := New(s.cfg)
	if err != nil || len(reloaded.runs) != 205 {
		t.Fatalf("saved records were not retained: %v", err)
	}
	code, body := call(t, reloaded, http.MethodGet, "/conversations", "w", nil)
	var entries []conversationEntry
	if code != 200 || json.Unmarshal(body, &entries) != nil || len(entries) != 2 ||
		entries[0].ID != "second" || entries[1].ID != "0" || entries[1].Title != "question 0" {
		t.Fatalf("wrong conversation list: %d %s", code, body)
	}
	code, body = call(t, reloaded, http.MethodGet, "/conversations/0", "w", nil)
	var turns []Run
	if code != 200 || json.Unmarshal(body, &turns) != nil || len(turns) != 202 ||
		turns[0].ID != "0" || turns[201].ID != parent {
		t.Fatalf("wrong conversation detail: %d turns, status %d", len(turns), code)
	}
	if code, _ := call(t, reloaded, http.MethodGet, "/conversations/0", "other", nil); code != 404 {
		t.Fatal("conversation leaked across workspaces")
	}
	code, body = call(t, reloaded, http.MethodGet, "/history?task=execution", "w", nil)
	var results []Run
	if code != 200 || json.Unmarshal(body, &results) != nil || len(results) != 1 || results[0].ID != "execution" {
		t.Fatalf("wrong task history: %d %s", code, body)
	}
}

func TestConversationListRecoversBranchesWithPreviouslyEvictedRoot(t *testing.T) {
	s := fixture(t, nil)
	old := &chatMemory{Through: "evicted", Text: "青禾一号"}
	for i, id := range []string{"a", "b"} {
		run := Run{ID: id, Task: "qa", WorkspaceID: "w", ParentID: old.Through, archive: old,
			Question: "branch " + id, CreatedAt: time.Unix(int64(i+1), 0)}
		if err := s.store(run); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	groups := reloaded.conversationGroups("w")
	if len(groups) != 1 || len(groups[old.Through]) != 2 {
		t.Fatalf("branches split after reload: %+v", groups)
	}
	history, err := reloaded.conversation("w", Input{Task: "qa", ParentID: "b"})
	if err != nil || memoryText(history.archive) != old.Text {
		t.Fatalf("archive unavailable when continuing: %+v %v", history, err)
	}
	code, body := call(t, reloaded, http.MethodGet, "/conversations/evicted", "w", nil)
	if code != 200 || strings.Contains(string(body), old.Text) || strings.Contains(string(body), "archived_conversation") {
		t.Fatal("archive exposed as public chat content")
	}
}
