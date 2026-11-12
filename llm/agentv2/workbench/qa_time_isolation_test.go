package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimeRecognizerFailureIsLimitedToQA(t *testing.T) {
	modelCalls := 0
	s := fixture(t, roleModel(func([]Message, any) error {
		modelCalls++
		return errors.New("analysis model unavailable")
	}))
	s.cfg.ConfirmationSubject = func(context.Context) string { return "alice" }
	s.cfg.QueryTools = &QueryTools{}
	s.cfg.TimeRecognizer = &TimeRecognizer{Node: filepath.Join(t.TempDir(), "missing-node.exe"), Script: "missing.cjs"}
	if _, err := s.cfg.TimeRecognizer.Recognize(context.Background(), "今天", time.Now(), time.Now()); !errors.Is(err, ErrTimeRecognizerUnavailable) {
		t.Fatalf("expected a typed component failure, got %v", err)
	}
	for _, endpoint := range []string{"/runs", "/chat"} {
		code, body := call(t, s, "POST", endpoint, "w", Input{Task: "qa", Question: "昨天最高温度是多少？"})
		if code != 200 {
			t.Fatalf("%s returned %d: %s", endpoint, code, body)
		}
		var run Run
		if endpoint == "/chat" {
			decoder := json.NewDecoder(bytes.NewReader(body))
			for {
				var event chatEvent
				err := decoder.Decode(&event)
				if err == io.EOF {
					break
				}
				if err != nil || event.Type == "error" {
					t.Fatalf("unexpected stream error %+v, %v", event, err)
				}
				if event.Run != nil {
					run = *event.Run
				}
			}
		} else if err := json.Unmarshal(body, &run); err != nil {
			t.Fatal(err)
		}
		if run.Status != "time_recognizer_unavailable" || len(run.Claims) != 0 || len(run.QueryResults) != 0 ||
			!strings.Contains(strings.Join(run.Limitations, " "), "时间解析组件不可用") || modelCalls != 0 || len(s.cfg.Gate) != 0 {
			t.Fatalf("component failure continued QA or retained its gate %+v calls=%d", run, modelCalls)
		}
	}
	code, body := call(t, s, "GET", "/status", "w", nil)
	var status map[string]any
	if code != 200 || json.Unmarshal(body, &status) != nil || status["time_recognizer_status"] != "unavailable" {
		t.Fatalf("missing component status: %s", body)
	}
	q := fixtureQuery(s)
	code, body = call(t, s, "POST", "/runs", "w", Input{Task: "environment", Question: "概述这段时间的环境", Query: &q})
	var environment Run
	if code != 200 || json.Unmarshal(body, &environment) != nil || environment.Workflow == nil ||
		environment.Evidence == nil || environment.Evidence.Rows != 2 || len(environment.Claims) == 0 {
		t.Fatalf("time failure blocked environment analysis: %d %s", code, body)
	}
	s.cfg.VisualTool = func(context.Context, VisualInput) (VisionFinding, error) {
		return VisionFinding{Version: "fixture", Subject: "non_leaf", Quality: "usable", Visible: []string{"图中是室内场景。"}}, nil
	}
	code, body = call(t, s, "POST", "/runs", "w", Input{Task: "image", Question: "分析这张图片", Image: automationPhoto(t, 1), ImageSource: "user_upload"})
	var image Run
	if code != 200 || json.Unmarshal(body, &image) != nil || image.Workflow == nil || image.Status != "needs_input" || image.Workflow.Vision == nil {
		t.Fatalf("time failure blocked image workflow: %d %s", code, body)
	}
	installGoalPlanner(s, analysisGoal(true, false))
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "概述环境", Query: &q}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if len(done.Reports) != 1 || done.Reports[0].Evidence == nil || done.Reports[0].Evidence.Rows != 2 || len(s.cfg.Gate) != 0 {
		t.Fatalf("time failure blocked automation %+v", done)
	}
	if s.cfg.TimeRecognizer.Status() != "unavailable" {
		t.Fatal("unrelated workflows unexpectedly invoked the time recognizer")
	}
}

func TestCanceledTimeRecognitionDoesNotMarkComponentUnavailable(t *testing.T) {
	recognizer := &TimeRecognizer{Node: filepath.Join(t.TempDir(), "missing-node.exe"), Script: "missing.cjs"}
	recognizer.status.Store(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := recognizer.Recognize(ctx, "今天", time.Now(), time.Now()); !errors.Is(err, context.Canceled) || recognizer.Status() != "ready" {
		t.Fatalf("cancellation changed component readiness: %v, %s", err, recognizer.Status())
	}
}
