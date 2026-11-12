package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type chatEvent struct {
	Type  string `json:"type"`
	Stage string `json:"stage,omitempty"`
	Delta string `json:"delta,omitempty"`
	Claim *Claim `json:"claim,omitempty"`
	Run   *Run   `json:"run,omitempty"`
	Error string `json:"error,omitempty"`
}

type chatStream struct {
	encoder *json.Encoder
	flusher http.Flusher
	emitted map[string]bool
}

func (s *chatStream) send(event chatEvent) error {
	if err := s.encoder.Encode(event); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *chatStream) stage(stage string) {
	if s != nil {
		_ = s.send(chatEvent{Type: "stage", Stage: stage})
	}
}

func (s *chatStream) thinking(delta string) error {
	return s.send(chatEvent{Type: "thinking", Delta: delta})
}

func (s *chatStream) claim(claim Claim) error {
	key := fingerprint(claim)
	if s.emitted[key] {
		return nil
	}
	if err := s.send(chatEvent{Type: "claim", Claim: &claim}); err != nil {
		return err
	}
	s.emitted[key] = true
	return nil
}

// A complete source quote can be verified as soon as its JSON object arrives.
// Mixed evidence and free text stay in the normal review pipeline.
func (s *chatStream) preview(content string, run Run) error {
	if run.Evidence != nil || len(run.Knowledge) == 0 {
		return nil
	}
	d := json.NewDecoder(strings.NewReader(content))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return nil
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil
		}
		if key != "claims" {
			var skip json.RawMessage
			if d.Decode(&skip) != nil {
				return nil
			}
			continue
		}
		if token, err := d.Token(); err != nil || token != json.Delim('[') {
			return nil
		}
		for i := 0; i < 8 && d.More(); i++ {
			var claim Claim
			if d.Decode(&claim) != nil {
				return nil
			}
			out := ModelOutput{Claims: []Claim{claim}}
			if !literalOnly(out) || validateOutput(out, run) != nil {
				return nil
			}
			if err := s.claim(claim); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func (s *Service) chat(ctx context.Context, w http.ResponseWriter, r *http.Request, workspace, authorization string) {
	var in Input
	if err := readJSON(w, r, &in); err != nil {
		problem(w, 400, err)
		return
	}
	if in.Task != "qa" {
		problem(w, 422, fmt.Errorf("chat requires a question"))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		problem(w, 500, fmt.Errorf("streaming unavailable"))
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	stream := &chatStream{encoder: json.NewEncoder(w), flusher: flusher, emitted: map[string]bool{}}
	stream.stage("accepted")
	run, err := s.runWithEvents(ctx, workspace, authorization, in, stream)
	if err != nil {
		_ = stream.send(chatEvent{Type: "error", Error: err.Error()})
		return
	}
	for _, claim := range run.Claims {
		if err := stream.claim(claim); err != nil {
			return
		}
	}
	_ = stream.send(chatEvent{Type: "result", Run: &run})
}
