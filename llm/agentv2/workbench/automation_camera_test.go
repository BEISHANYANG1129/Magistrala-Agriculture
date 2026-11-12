package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func cameraFixture(t *testing.T, ops []CameraOperation) (*Service, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	s := fixture(t, nil)
	moves, captures := &atomic.Int32{}, &atomic.Int32{}
	source := CameraSource{WorkspaceID: "w", DeviceID: "b6d1c497-3e75-43af-8ab6-6ba723290e2d", Name: "乔安 G4S4", BindingID: "jooan-g4s4"}
	s.cfg.CameraSources = []CameraSource{source}
	s.cfg.CheckCamera = func(context.Context, string, CameraSource, bool) error { return nil }
	photo := automationPhoto(t, 65)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			respond(w, 200, map[string]any{"state": "ready", "enabled": true, "generation": "g-" + strings.Repeat("a", 32)})
		case strings.HasSuffix(r.URL.Path, "/capabilities"):
			respond(w, 200, map[string]bool{"serverSnapshot": true, "autonomousModelControl": true})
		case strings.HasSuffix(r.URL.Path, "/ptz/move"):
			moves.Add(1)
			respond(w, 200, map[string]any{"startResult": 0, "stopResult": 0, "positionVerified": false})
		case strings.HasSuffix(r.URL.Path, "/ptz/stop"):
			respond(w, 200, map[string]any{"startResult": 0, "stopResult": 0, "positionVerified": false})
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			captures.Add(1)
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			respond(w, 200, map[string]any{"image": photo, "generation": input["generation"], "view": input["view"], "receivedAt": time.Now().UTC(), "segment": 123})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	s.cfg.CameraGatewayURL = server.URL
	p := GoalPlan{Summary: "摄像头任务", Camera: &CameraRequest{DeviceID: source.DeviceID, Target: source.Name, Quote: "摄像头任务", Operations: ops, AnalysisKind: "scene"}}
	installGoalPlanner(s, p)
	return s, moves, captures
}
func startCameraFixture(t *testing.T, s *Service) automationDetail {
	created := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "摄像头任务"}, 202)
	return waitAutomation(t, s, created.Task.ID)
}
func confirmCameraFixture(t *testing.T, s *Service, d automationDetail) automationDetail {
	return automationRequest(t, s, "POST", "/automation/"+d.Task.ID+"/camera/confirm", map[string]any{"revision": d.Task.Automation.Camera.Revision, "confirmed": true}, 202)
}

func TestCameraWholePlanConfirmationNoPreApprovalActuationAndNoReplay(t *testing.T) {
	s, moves, captures := cameraFixture(t, []CameraOperation{{Action: "move", Direction: "left", DurationMs: 200, View: 1}, {Action: "snapshot", View: 1}})
	d := startCameraFixture(t, s)
	if d.Task.Automation.step("planner").Status != "completed" {
		t.Fatal("planning still shown as running while waiting for camera confirmation")
	}
	if d.Task.Status != "waiting_camera_confirmation" || moves.Load() != 0 || captures.Load() != 0 {
		t.Fatalf("actuation before confirmation: %+v", d.Task)
	}
	path := "/automation/" + d.Task.ID + "/camera/confirm"
	if code, _ := call(t, s, "POST", path, "w", map[string]any{"revision": "old", "confirmed": true}); code != 409 {
		t.Fatal("stale plan accepted")
	}
	confirmCameraFixture(t, s, d)
	done := waitAutomation(t, s, d.Task.ID)
	if done.Task.Status != "completed" || moves.Load() != 1 || captures.Load() != 1 || len(done.Task.Automation.Camera.Captures) != 1 || len(done.Reports) != 0 {
		t.Fatalf("unexpected result: %+v", done)
	}
	if code, _ := call(t, s, "POST", path, "w", map[string]any{"revision": d.Task.Automation.Camera.Revision, "confirmed": true}); code != 200 {
		t.Fatal(code)
	}
	if moves.Load() != 1 || captures.Load() != 1 {
		t.Fatal("confirmation replayed")
	}
	capture := done.Task.Automation.Camera.Captures[0]
	if code, b := call(t, s, "GET", "/automation/"+d.Task.ID+"/camera/captures/"+capture.ID, "w", nil); code != 200 || !strings.Contains(string(b), "image/png") {
		t.Fatalf("capture not readable: %d %s", code, b)
	}
	s.cfg.CheckCamera = func(context.Context, string, CameraSource, bool) error { return fmt.Errorf("revoked") }
	if code, _ := call(t, s, "GET", "/automation/"+d.Task.ID+"/camera/captures/"+capture.ID, "w", nil); code != 403 {
		t.Fatal("capture permission not rechecked")
	}
}

func TestCameraStopBypassesGateAndCancelsRemainingSteps(t *testing.T) {
	s, _, captures := cameraFixture(t, []CameraOperation{{Action: "wait", WaitSeconds: 10, View: 1}, {Action: "snapshot", View: 1}})
	d := startCameraFixture(t, s)
	confirmCameraFixture(t, s, d)
	if code, b := call(t, s, "POST", "/automation/"+d.Task.ID+"/camera/stop", "w", map[string]any{}); code != 200 {
		t.Fatalf("stop blocked: %d %s", code, b)
	}
	done := waitAutomation(t, s, d.Task.ID)
	if done.Task.Status != "camera_stopped" || captures.Load() != 0 {
		t.Fatalf("continued after stop: %+v", done.Task)
	}
}

func TestCameraPermissionExpiryAndGenerationChangesNeverMove(t *testing.T) {
	for _, kind := range []string{"permission", "expiry", "generation"} {
		t.Run(kind, func(t *testing.T) {
			s, moves, _ := cameraFixture(t, []CameraOperation{{Action: "move", Direction: "up", DurationMs: 200, View: 1}})
			d := startCameraFixture(t, s)
			if kind == "permission" {
				s.cfg.CheckCamera = func(context.Context, string, CameraSource, bool) error { return fmt.Errorf("revoked") }
				if code, _ := call(t, s, "POST", "/automation/"+d.Task.ID+"/camera/confirm", "w", map[string]any{"revision": d.Task.Automation.Camera.Revision, "confirmed": true}); code != 403 {
					t.Fatal(code)
				}
			} else {
				run, _ := s.automationRun("w", d.Task.ID)
				run.Automation = cloneAutomation(run.Automation)
				if kind == "expiry" {
					run.Automation.Camera.ExpiresAt = time.Now().Add(-time.Second)
				} else {
					run.Automation.Camera.Generation = "g-" + strings.Repeat("b", 32)
				}
				_ = s.saveAutomation(&run)
				if kind == "expiry" {
					if code, _ := call(t, s, "POST", "/automation/"+d.Task.ID+"/camera/confirm", "w", map[string]any{"revision": d.Task.Automation.Camera.Revision, "confirmed": true}); code != 409 {
						t.Fatal(code)
					}
				} else {
					confirmCameraFixture(t, s, d)
					done := waitAutomation(t, s, d.Task.ID)
					if done.Task.Status != "needs_input" || done.Task.Automation.Camera.StartedAt != nil {
						t.Fatal(done.Task.Status)
					}
					// Reprepare the same goal without replaying a movement. It must
					// return to a new whole-plan confirmation first.
					automationRequest(t, s, "POST", "/automation/"+d.Task.ID+"/resume", automationInput{}, 202)
					ready := waitAutomation(t, s, d.Task.ID)
					if ready.Task.Status != "waiting_camera_confirmation" || ready.Task.ID != d.Task.ID || ready.Task.Automation.Camera.Revision == d.Task.Automation.Camera.Revision {
						t.Fatalf("did not prepare a new confirmation: %+v", ready.Task)
					}
				}
			}
			if moves.Load() != 0 {
				t.Fatal("unsafe move")
			}
		})
	}
}

func TestCameraInterruptedDispatchRecoveredWithoutReplay(t *testing.T) {
	s, _, _ := cameraFixture(t, []CameraOperation{{Action: "snapshot", View: 1}})
	d := startCameraFixture(t, s)
	run := d.Task
	now := time.Now()
	run.Status = "camera_running"
	run.Automation.Camera.Status = "running"
	run.Automation.Camera.StartedAt = &now
	run.Automation.Camera.Receipts = []CameraReceipt{{Index: 0, RequestID: id(), Status: "dispatching"}}
	_ = s.saveAutomation(&run)
	restored, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := restored.automationRun("w", run.ID)
	if saved.Status != "camera_unknown" || saved.Automation.Camera.Status != "unknown" {
		t.Fatalf("unsafe restart state: %+v", saved)
	}
}

func TestCameraFailedDispatchedMoveCannotBeRepreparedOrReplayed(t *testing.T) {
	s, _, _ := cameraFixture(t, []CameraOperation{{Action: "move", Direction: "left", DurationMs: 100, View: 1}})
	var sends atomic.Int32
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			respond(w, 200, map[string]any{"state": "ready", "enabled": true, "generation": "g-" + strings.Repeat("a", 32)})
		case strings.HasSuffix(r.URL.Path, "/capabilities"):
			respond(w, 200, map[string]bool{"serverSnapshot": true, "autonomousModelControl": true})
		case strings.HasSuffix(r.URL.Path, "/ptz/move"):
			sends.Add(1)
			respond(w, 502, map[string]string{"message": "receipt unavailable"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer bridge.Close()
	s.cfg.CameraGatewayURL = bridge.URL
	d := startCameraFixture(t, s)
	confirmCameraFixture(t, s, d)
	done := waitAutomation(t, s, d.Task.ID)
	if done.Task.Status != "camera_unknown" || len(done.Task.Automation.Camera.Receipts) != 1 || sends.Load() != 1 {
		t.Fatalf("wrong dispatched failure: %+v", done.Task)
	}
	if code, _ := call(t, s, "POST", "/automation/"+d.Task.ID+"/resume", "w", automationInput{}); code != 409 {
		t.Fatal("dispatched movement was allowed to replan")
	}
	if code, _ := call(t, s, "POST", "/automation/"+d.Task.ID+"/camera/confirm", "w", map[string]any{"revision": d.Task.Automation.Camera.Revision, "confirmed": true}); code != 200 || sends.Load() != 1 {
		t.Fatal("dispatched movement was replayed")
	}
}

func TestUnregisteredCameraIsACapabilityGapAfterSourceSemanticReview(t *testing.T) {
	s := fixture(t, nil)
	s.cfg.CameraSources = []CameraSource{{WorkspaceID: "w", DeviceID: "jooan", Name: "乔安 G4S4", BindingID: "jooan-g4s4"}}
	goal := "请用Go1拍摄新照片，不要用乔安替代。"
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		switch out := dst.(type) {
		case *goalOutline:
			*out = goalOutline{Summary: "使用Go1拍照", Camera: true, DeviceIntent: "none"}
		case *speechAct:
			out.Intent = "只读任务"
		case *CameraIntent:
			*out = CameraIntent{Mode: "real", Target: "Go1", TargetSpecified: true, Quote: goal, Timing: "immediate", AnalysisKind: "scene", Steps: []CameraIntentStep{{Action: "snapshot", Unit: "unspecified"}}}
		case *deviceIntentReview:
			var payload map[string]any
			_ = json.Unmarshal([]byte(messages[1].Content), &payload)
			camera := payload["plan"].(map[string]any)["camera"].(map[string]any)
			_, compiled := camera["device_id"]
			_, catalog := payload["registered_cameras"]
			out.Approved = !compiled && !catalog && camera["target"] == "Go1"
			out.Issue = "capability data contaminated source review"
		default:
			return fmt.Errorf("unexpected model role %T", dst)
		}
		return nil
	})
	created := automationRequest(t, s, "POST", "/automation", automationInput{Goal: goal}, 202)
	done := waitAutomation(t, s, created.Task.ID)
	if done.Task.Status != "needs_input" || done.Task.Automation.GoalPlan.Camera.Unsupported == "" || done.Task.Automation.Camera != nil {
		t.Fatalf("unknown device treated as failed understanding: %+v", done.Task)
	}
}

func TestCameraRejectsUnboundedOrUngroundedPlans(t *testing.T) {
	run := Run{Question: "向左微调摄像头", Automation: &Automation{}}
	base := CameraRequest{Quote: run.Question, AnalysisKind: "scene", Operations: []CameraOperation{{Action: "move", Direction: "left", DurationMs: 200, View: 1}}}
	for _, alter := range []func(*CameraRequest){func(p *CameraRequest) { p.Quote = "打开阀门" }, func(p *CameraRequest) { p.Operations[0].View = 2 }, func(p *CameraRequest) { p.Operations[0].DurationMs = 2000 }, func(p *CameraRequest) { p.Analyze = true }, func(p *CameraRequest) { p.Operations = make([]CameraOperation, 9) }} {
		p := base
		p.Operations = append([]CameraOperation(nil), base.Operations...)
		alter(&p)
		if validateCameraRequest(&run, &p) == nil {
			t.Fatalf("invalid plan accepted: %+v", p)
		}
	}
}

func TestCameraQuoteRestoresOriginalSpacingButRejectsChangedMeaning(t *testing.T) {
	source := "请用乔安 G4S4 摄像头截取画面1。不要转动摄像头。"
	for _, quote := range []string{"请用乔安 G4S4 摄像头截取画面 1。不要转动摄像头。", "截取画面 1"} {
		original, ok := alignCameraQuote(source, quote)
		if !ok || !strings.Contains(source, original) || strings.Contains(original, "画面 1") {
			t.Fatalf("did not preserve original evidence: %q %v", original, ok)
		}
	}
	for _, quote := range []string{"请用乔安 G4S4 摄像头截取画面2。不要转动摄像头。", "请用乔安 G4S4 摄像头截取画面1。转动摄像头。", "请用Go1摄像头截取画面1。不要转动摄像头。", " \t\n"} {
		if _, ok := alignCameraQuote(source, quote); ok {
			t.Fatalf("changed evidence accepted: %q", quote)
		}
	}
	run := Run{Question: source, Automation: &Automation{}}
	request := CameraRequest{Quote: "请用乔安 G4S4 摄像头截取画面 1。不要转动摄像头。", AnalysisKind: "scene", Operations: []CameraOperation{{Action: "snapshot", View: 1}}}
	if err := validateCameraRequest(&run, &request); err != nil || request.Quote != source {
		t.Fatalf("valid request rejected: %v %+v", err, request)
	}
}

func TestCameraSceneAnalysisAcceptsNonLeafAndKeepsBothCaptureReports(t *testing.T) {
	s, _, captures := cameraFixture(t, []CameraOperation{{Action: "snapshot", View: 1}, {Action: "snapshot", View: 2}})
	plan := GoalPlan{Summary: "摄像头任务", Camera: &CameraRequest{DeviceID: s.cfg.CameraSources[0].DeviceID, Target: "乔安", Quote: "摄像头任务", AnalysisKind: "scene", Analyze: true, Instruction: "说明画面中的对象", Operations: []CameraOperation{{Action: "snapshot", View: 1}, {Action: "snapshot", View: 2}}}}
	s.cfg.VisualTool = func(_ context.Context, in VisualInput) (VisionFinding, error) {
		return VisionFinding{Version: "fixture", AssetID: in.AssetID, Subject: "non_leaf", Quality: "usable", Visible: []string{"画面中可见墙面。"}}, nil
	}
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		if out, ok := dst.(*ModelOutput); ok {
			var input struct {
				Image imageAsset `json:"image"`
			}
			_ = json.Unmarshal([]byte(messages[1].Content), &input)
			out.ImageObservations = []Claim{{Text: "画面中可见墙面。", Kind: "image", Citations: []string{input.Image.ID}}}
			return nil
		}
		return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"可见发现支持"}]}`), dst)
	})
	installGoalPlanner(s, plan)
	d := startCameraFixture(t, s)
	confirmCameraFixture(t, s, d)
	done := waitAutomation(t, s, d.Task.ID)
	if captures.Load() != 2 || done.Task.Status != "completed" || len(done.Reports) != 2 {
		t.Fatalf("non-leaf scene failed or reports lost: %+v", done)
	}
	for _, report := range done.Reports {
		if report.Status != "completed" || report.Workflow.Vision.Subject != "non_leaf" || len(report.Claims) != 1 || report.CameraSource == nil {
			t.Fatalf("unexpected scene report: %+v", report)
		}
	}
	s.cfg.CheckCamera = func(context.Context, string, CameraSource, bool) error { return fmt.Errorf("revoked") }
	if code, body := call(t, s, "GET", "/history?task=image", "w", nil); code != 200 || string(body) != "[]\n" {
		t.Fatalf("camera report leaked in history: %d %s", code, body)
	}
}
