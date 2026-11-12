package workbench

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

func automationPhoto(t *testing.T, red uint8) string {
	t.Helper()
	var buffer bytes.Buffer
	pic := image.NewRGBA(image.Rect(0, 0, 180, 180))
	pic.SetRGBA(0, 0, color.RGBA{R: red, A: 255})
	if err := png.Encode(&buffer, pic); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func automationRequest(t *testing.T, s *Service, method, path string, input any, want int) automationDetail {
	t.Helper()
	code, body := call(t, s, method, path, "w", input)
	var detail automationDetail
	if code != want || json.Unmarshal(body, &detail) != nil {
		t.Fatalf("%s %s returned %d: %s", method, path, code, body)
	}
	return detail
}

func waitAutomation(t *testing.T, s *Service, id string) automationDetail {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		detail := automationRequest(t, s, "GET", "/automation/"+id, nil, 200)
		if detail.Task.Status != "running" && detail.Task.Status != "camera_running" && len(s.cfg.Gate) == 0 {
			return detail
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("automation did not settle")
	return automationDetail{}
}

// Supply the semantic planner's contract explicitly. Natural-language behavior
// is also exercised against the real local model by the browser smoke checks.
func installGoalPlanner(s *Service, plan GoalPlan) {
	analysis := s.cfg.Model
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		if fixturePlanPart(dst, plan) {
			return nil
		}
		switch out := dst.(type) {
		case *speechAct:
			out.Intent = "只读任务"
			if plan.Execution != nil {
				out.Intent = "执行操作"
			}
			return nil
		case *ExecutionMatch:
			if plan.Execution != nil && plan.Execution.DeviceType == "flow_control" && (plan.Execution.Action == "open" || plan.Execution.Action == "close") {
				out.CapabilityID = simulationVersion
			} else {
				out.Reason = "当前尚无该功能的执行适配器。"
			}
			return nil
		case *deviceIntentReview:
			out.Approved = true
			return nil
		default:
			if analysis != nil {
				return analysis.Generate(context.Background(), messages, nil, dst)
			}
			return errors.New("analysis model unavailable")
		}
	})
}

func fixturePlanPart(dst any, plan GoalPlan) bool {
	var part any
	switch dst.(type) {
	case *goalOutline:
		intent := "none"
		if plan.Execution != nil || plan.Camera != nil {
			intent = "operate"
		}
		part = goalOutline{Summary: plan.Summary, Camera: plan.Camera != nil, Environment: plan.Environment != nil, Image: plan.Image != nil, DeviceIntent: intent, UnrepresentedActions: plan.Clarification != ""}
	case *CameraIntent:
		intent := CameraIntent{Mode: "real", Quote: plan.Camera.Quote, Analyze: plan.Camera.Analyze, AnalysisKind: plan.Camera.AnalysisKind, Instruction: plan.Camera.Instruction, Timing: "immediate", DependsOn: plan.Camera.DependsOn}
		for _, op := range plan.Camera.Operations {
			view := op.View
			step := CameraIntentStep{Action: op.Action, Direction: op.Direction, Unit: "unspecified", View: &view}
			if op.Action == "move" {
				value := float64(op.DurationMs)
				step.Amount = &value
				step.Unit = "milliseconds"
			}
			if op.Action == "wait" {
				value := float64(op.WaitSeconds)
				step.Amount = &value
				step.Unit = "seconds"
			}
			intent.Steps = append(intent.Steps, step)
		}
		part = intent
	case *DeviceRequest:
		part = plan.Execution
	case *ObservationRequest:
		part = plan.Environment
	case *PhotoRequest:
		part = plan.Image
	default:
		return false
	}
	b, _ := json.Marshal(part)
	_ = json.Unmarshal(b, dst)
	return true
}

func analysisGoal(environment, image bool) GoalPlan {
	p := GoalPlan{Summary: "分析所需资料"}
	if environment {
		p.Environment = &ObservationRequest{Instruction: "概述环境观测", Source: "auto", TimeScope: "unspecified"}
	}
	if image {
		p.Image = &PhotoRequest{Instruction: "检查叶片", Source: "attachment"}
	}
	return p
}

func fixtureDevice(target, mode, quote string, seconds int) *DeviceRequest {
	return &DeviceRequest{Target: target, DeviceType: "flow_control", Action: "open", Mode: mode, DurationSeconds: &seconds, Timing: "immediate", Quote: quote}
}

func TestAutomationSingleSubmitRunsBothBranchesAndWaitsForConfirmation(t *testing.T) {
	s := fixture(t, nil)
	var assetID string
	s.cfg.VisualTool = func(_ context.Context, input VisualInput) (VisionFinding, error) {
		assetID = input.AssetID
		return VisionFinding{Version: "fixture", Subject: "leaf", Quality: "usable", Visible: []string{"叶片边缘有褐色区域。"}}, nil
	}
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		if strings.Contains(messages[0].Content, "环境分析角色") {
			return errors.New("use deterministic statistics")
		}
		if out, ok := dst.(*ModelOutput); ok {
			out.ImageObservations = []Claim{{Kind: "image", Text: "叶片边缘有褐色区域。", Citations: []string{assetID}}}
			return nil
		}
		return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"来自视觉工具"}]}`), dst)
	})
	q := fixtureQuery(s)
	in := automationInput{Goal: "概述环境并检查这张叶片，然后模拟开启阀门 5 秒", Query: &q, Image: automationPhoto(t, 1), ImageSource: "user_upload"}
	p := analysisGoal(true, true)
	p.Execution = fixtureDevice("阀门", "simulation", "模拟开启阀门 5 秒", 5)
	installGoalPlanner(s, p)
	start := automationRequest(t, s, "POST", "/automation", in, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "waiting_confirmation" || len(done.Reports) != 2 || done.Task.Plan == nil ||
		done.Task.Plan.DurationSeconds != 5 || done.Task.Plan.Basis != done.Reports[0].ID || done.Task.Feedback != nil ||
		done.Task.Approval != nil || len(s.history("w", "automation")) != 1 {
		t.Fatalf("expected one joined task and an unconfirmed draft: %+v", done.Task)
	}
	if !strings.Contains(strings.Join(done.Task.Limitations, " "), "分别核查") {
		t.Fatal("combined result failed to preserve source boundaries")
	}
	if code, _ := call(t, s, "GET", "/automation/"+start.Task.ID, "other", nil); code != 404 {
		t.Fatal("automation leaked across workspaces")
	}
	code, raw := call(t, s, "POST", "/simulate", "w", map[string]any{"id": start.Task.ID, "revision": done.Task.Plan.Revision, "confirm": true})
	var simulated Run
	if code != 200 || json.Unmarshal(raw, &simulated) != nil || simulated.Status != "simulated_completed" || simulated.Feedback.State != "closed" ||
		simulated.Automation.step("simulation").Status != "completed" || len(s.history("w", "automation")) != 1 {
		t.Fatalf("simulation did not complete in the same task: %d %s", code, raw)
	}
}

func TestAutomationResumesAfterMissingInputAndRestoresSavedImage(t *testing.T) {
	s := fixture(t, nil)
	in := automationInput{Goal: "概述环境并检查叶片", Image: automationPhoto(t, 2), ImageSource: "user_upload"}
	installGoalPlanner(s, analysisGoal(true, true))
	start := automationRequest(t, s, "POST", "/automation", in, 202)
	paused := waitAutomation(t, s, start.Task.ID)
	if paused.Task.Status != "needs_input" || strings.Join(paused.Task.Automation.Needs, ",") != "window" {
		t.Fatalf("wrong missing inputs: %+v", paused.Task.Automation)
	}
	reloaded, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.dataset = s.dataset
	q := fixtureQuery(s)
	automationRequest(t, reloaded, "POST", "/automation/"+start.Task.ID+"/resume", automationInput{Query: &q}, 202)
	done := waitAutomation(t, reloaded, start.Task.ID)
	if done.Task.Status != "partial" || len(done.Reports) != 2 || done.Reports[1].ImageHash != paused.Task.Automation.Image.ID ||
		len(reloaded.history("w", "automation")) != 1 {
		t.Fatalf("resume lost saved inputs or created another task: %+v", done.Task)
	}
}

func TestAutomationNeverCreatesDraftFromNegatedIntent(t *testing.T) {
	s := fixture(t, nil)
	q := fixtureQuery(s)
	for _, goal := range []string{"概述环境，不要模拟开启阀门 5 秒", "请实际开启阀门 5 秒"} {
		p := analysisGoal(true, false)
		if goal == "请实际开启阀门 5 秒" {
			p.Environment = nil
			p.Execution = fixtureDevice("阀门", "real", goal, 5)
		}
		installGoalPlanner(s, p)
		start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: goal, Query: &q}, 202)
		done := waitAutomation(t, s, start.Task.ID)
		if done.Task.Plan != nil || done.Task.Status == "waiting_confirmation" {
			t.Fatalf("unsafe draft from %q", goal)
		}
	}
}

func TestAutomationFindsWeatherAndRunsBeforeMissingPhoto(t *testing.T) {
	s := fixture(t, nil)
	s.dataset.StationID = "40366226"
	p := analysisGoal(true, true)
	p.Environment.Source, p.Environment.TimeScope = "imported", "all_available"
	installGoalPlanner(s, p)
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "概述已导入的气象日志，并检查这张叶片。"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	a := done.Task.Automation
	if done.Task.Status != "needs_input" || strings.Join(a.Needs, ",") != "image" || a.Observation == nil ||
		a.Query.Mode != "history" || !a.Query.Start.Equal(s.dataset.Frames[0].ReceivedAt) ||
		len(done.Reports) != 1 || done.Reports[0].Task != "environment" || len(done.Reports[0].Claims) == 0 {
		t.Fatalf("automatic weather lookup did not finish independently: %+v", done)
	}
	p.Image = nil
	p.Environment.Site = "东区"
	installGoalPlanner(s, p)
	other := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "概述东区已导入的气象日志"}, 202)
	unbound := waitAutomation(t, s, other.Task.ID)
	if unbound.Task.Automation.Query != nil || len(unbound.Reports) != 0 {
		t.Fatal("unmapped area was assigned the default station")
	}
}

func TestAutomationReadsRegisteredCameraAndChecksImage(t *testing.T) {
	s := fixture(t, nil)
	s.cfg.PhotoSources = []PhotoSource{{WorkspaceID: "w", PublisherID: "camera", ChannelID: "channel", Name: "Go1 左侧摄像头", Aliases: []string{"Go1"}}}
	raw := automationPhoto(t, 3)
	s.cfg.ReadPhoto = func(_ context.Context, workspace, auth string, source PhotoSource, _ *Window) (PhotoMessage, error) {
		if workspace != "w" || auth != "Bearer test" || source.PublisherID != "camera" {
			return PhotoMessage{}, errors.New("wrong photo scope")
		}
		return PhotoMessage{Encoded: raw, Time: time.Now().Add(-time.Hour)}, nil
	}
	s.cfg.VisualTool = func(_ context.Context, input VisualInput) (VisionFinding, error) {
		if input.Image != raw || input.Source != "platform_photo" {
			return VisionFinding{}, errors.New("wrong photo input")
		}
		return VisionFinding{Version: "fixture", Subject: "non_leaf", Quality: "usable", Visible: []string{"画面不是叶片。"}}, nil
	}
	s.cfg.Model = roleModel(func([]Message, any) error { return errors.New("no interpretation needed") })
	p := analysisGoal(false, true)
	p.Image.Source, p.Image.SourceID = "platform", "camera"
	installGoalPlanner(s, p)
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "检查 Go1 左侧摄像头最新一张照片"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "needs_input" || done.Task.Automation.Photo == nil || done.Task.Automation.Photo.Origin != "platform" ||
		len(done.Reports) != 1 || done.Reports[0].Workflow.Vision.Subject != "non_leaf" {
		t.Fatalf("camera lookup did not lead to image quality check: %+v", done)
	}
	if len(done.Task.Automation.InputNeeds) != 1 || !strings.Contains(done.Task.Automation.InputNeeds[0].Message, "不是叶片") {
		t.Fatal("missing-photo prompt retained a running-stage message")
	}
}

func TestTaskTimezoneValidation(t *testing.T) {
	loc, err := localTimeZone("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if loc.String() != "Asia/Shanghai" {
		t.Fatal("wrong task timezone")
	}
	if _, err := localTimeZone("Local"); err == nil {
		t.Fatal("host timezone must not affect tasks")
	}
}
