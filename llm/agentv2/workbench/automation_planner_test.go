package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDeviceRequestPreservesRealIntentAndExplicitlySwitchesSameTask(t *testing.T) {
	s := fixture(t, nil)
	p := GoalPlan{Summary: "开启闸门10秒后关闭", Execution: fixtureDevice("闸门", "real", "开启闸门10秒", 10)}
	installGoalPlanner(s, p)
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "开启闸门10秒"}, 202)
	paused := waitAutomation(t, s, start.Task.ID)
	if paused.Task.Status != "needs_input" || strings.Join(paused.Task.Automation.Needs, ",") != "execution_mode" || paused.Task.Plan != nil || paused.Task.Automation.GoalPlan.Execution.Mode != "real" {
		t.Fatalf("real intent was lost or silently simulated: %+v", paused.Task)
	}
	if !strings.Contains(paused.Task.Automation.InputNeeds[0].Message, "控制和回读接口") {
		t.Fatal("missing specific capability explanation")
	}
	// A reload preserves both the understood request and the pending choice.
	reloaded, err := New(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	automationRequest(t, reloaded, "POST", "/automation/"+start.Task.ID+"/resume", automationInput{ExecutionMode: "simulation"}, 202)
	done := waitAutomation(t, reloaded, start.Task.ID)
	if done.Task.Status != "waiting_confirmation" || done.Task.Plan.DurationSeconds != 10 || done.Task.Plan.Target != simulatedValve || done.Task.Automation.GoalPlan.Execution.Mode != "real" || done.Task.Automation.ExecutionMode != "simulation" || len(reloaded.history("w", "automation")) != 1 || done.Task.Feedback != nil {
		t.Fatalf("switch did not create an unconfirmed simulation in the same task: %+v", done.Task)
	}
}

func TestCapabilityChecksDoNotTurnOtherDevicesOrConditionsIntoValveActions(t *testing.T) {
	for _, tc := range []struct{ name, deviceType, action, timing, condition, need string }{
		{"水泵", "pump", "open", "immediate", "", "execution"},
		{"阀门压力", "flow_control", "调节压力", "immediate", "", "execution"},
		{"明天开启阀门", "flow_control", "open", "scheduled", "明天", "execution"},
		{"温度高了开阀门", "flow_control", "open", "conditional", "温度高", "execution"},
		{"开启十秒", "unspecified", "open", "immediate", "", "clarification"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t, nil)
			p := GoalPlan{Summary: tc.name, Execution: fixtureDevice(tc.name, "simulation", tc.name, 10)}
			p.Execution.DeviceType, p.Execution.Action, p.Execution.Timing, p.Execution.Condition = tc.deviceType, tc.action, tc.timing, tc.condition
			if tc.deviceType == "unspecified" {
				p.Execution.Target = ""
			}
			installGoalPlanner(s, p)
			start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: tc.name}, 202)
			done := waitAutomation(t, s, start.Task.ID)
			if done.Task.Plan != nil || done.Task.Status != "needs_input" || !strings.Contains(strings.Join(done.Task.Automation.Needs, ","), tc.need) {
				t.Fatalf("unsupported request was misrouted: %+v", done.Task)
			}
		})
	}
}

func TestPlannerFailureIsRetryableAndNeverBlamesGoal(t *testing.T) {
	s := fixture(t, roleModel(func([]Message, any) error { return errors.New("model offline") }))
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "打开闸门十秒钟，再关上"}, 202)
	failed := waitAutomation(t, s, start.Task.ID)
	if failed.Task.Status != "planner_failed" || len(failed.Task.Automation.Needs) != 0 || failed.Task.Plan != nil || !strings.Contains(failed.Task.Automation.Steps[0].Detail, "无需改写成固定句式") {
		t.Fatalf("wrong failure classification: %+v", failed.Task)
	}
	installGoalPlanner(s, GoalPlan{Summary: "开启闸门十秒后关闭", Execution: fixtureDevice("闸门", "real", "打开闸门十秒钟", 10)})
	automationRequest(t, s, "POST", "/automation/"+start.Task.ID+"/resume", automationInput{}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "needs_input" || done.Task.Automation.GoalPlan.Execution.DurationSeconds == nil {
		t.Fatal("retry failed")
	}
}

func TestPlannerRetriesInvalidPlanWithFeedbackBeforeUsingTools(t *testing.T) {
	s := fixture(t, nil)
	calls, reviewed := 0, 0
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		switch out := dst.(type) {
		case *goalOutline:
			calls++
			*out = goalOutline{Summary: "模拟开启", DeviceIntent: "operate"}
			if calls == 2 {
				if !strings.Contains(messages[1].Content, "validation_feedback") {
					t.Fatal("missing verification feedback")
				}
			}
		case *DeviceRequest:
			*out = *fixtureDevice("阀门", "simulation", "编造的指令", 10)
			if calls == 2 {
				out.Quote = "演练一下阀门通水十秒后关上"
			}
		case *speechAct:
			out.Intent = "执行操作"
		case *deviceIntentReview:
			reviewed++
			out.Approved = true
		case *ExecutionMatch:
			out.CapabilityID = simulationVersion
		}
		return nil
	})
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "演练一下阀门通水十秒后关上"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if calls != 2 || reviewed != 1 || done.Task.Status != "waiting_confirmation" {
		t.Fatalf("invalid plan was not repaired: %+v", done.Task)
	}
}

func TestDeviceSemanticRejectionNeverCreatesDraft(t *testing.T) {
	s := fixture(t, nil)
	s.cfg.Model = roleModel(func(_ []Message, dst any) error {
		switch out := dst.(type) {
		case *goalOutline:
			*out = goalOutline{Summary: "错误地执行否定动作", DeviceIntent: "operate"}
		case *DeviceRequest:
			*out = *fixtureDevice("阀门", "simulation", "不要开启阀门", 10)
		case *deviceIntentReview:
			out.Approved, out.Issue = false, "用户否定了该动作"
		case *speechAct:
			out.Intent = "执行操作"
		}
		return nil
	})
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "不要开启阀门"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "planner_failed" || done.Task.Plan != nil || done.Task.Feedback != nil {
		t.Fatal("rejected intent reached executor")
	}
}

func TestPlannerCatalogIsWorkspaceScopedAndHasNoDeviceTokens(t *testing.T) {
	s := fixture(t, nil)
	s.cfg.PhotoSources = []PhotoSource{{WorkspaceID: "other", PublisherID: "secret-device"}, {WorkspaceID: "w", PublisherID: "ours"}}
	run := Run{Question: "检查最近图片", WorkspaceID: "w", Automation: &Automation{TimeZone: "Asia/Shanghai"}}
	b, _ := json.Marshal(s.plannerInput(&run))
	if strings.Contains(string(b), "secret-device") || !strings.Contains(string(b), "ours") || !strings.Contains(string(b), "Asia/Shanghai") {
		t.Fatalf("wrong planner scope: %s", b)
	}
}

func TestClarificationResolvesWindowAndPreservesParent(t *testing.T) {
	s := fixture(t, nil)
	initial := analysisGoal(true, false)
	installGoalPlanner(s, initial)
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "说说这段时间的环境状况"}, 202)
	paused := waitAutomation(t, s, start.Task.ID)
	if paused.Task.Status != "needs_input" {
		t.Fatal("unanchored time should pause")
	}
	p := analysisGoal(true, false)
	q := fixtureQuery(s)
	p.Environment.TimeScope, p.Environment.Window = "window", &Window{Start: q.Start, End: q.End}
	installGoalPlanner(s, p)
	automationRequest(t, s, "POST", "/automation/"+start.Task.ID+"/resume", automationInput{Clarification: "用刚才那一分钟的记录"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "completed" || len(done.Reports) != 1 || len(done.Task.Automation.Replies) != 1 || len(s.history("w", "automation")) != 1 {
		t.Fatalf("clarification did not resume work: %+v", done.Task)
	}
}

func TestDependencyGraphRunsImageBeforeEnvironmentWhenRequested(t *testing.T) {
	s := fixture(t, nil)
	order := []string{}
	var assetID string
	s.cfg.VisualTool = func(_ context.Context, input VisualInput) (VisionFinding, error) {
		assetID = input.AssetID
		order = append(order, "image")
		return VisionFinding{Version: "fixture", Subject: "leaf", Quality: "usable", Visible: []string{"叶片可见"}}, nil
	}
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		if strings.Contains(messages[0].Content, "环境分析角色") {
			order = append(order, "environment")
			return errors.New("statistics fallback")
		}
		if out, ok := dst.(*ModelOutput); ok {
			out.ImageObservations = []Claim{{Kind: "image", Text: "叶片可见", Citations: []string{assetID}}}
			return nil
		}
		return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"来自视觉工具"}]}`), dst)
	})
	p := analysisGoal(true, true)
	p.Environment.DependsOn = []string{"image"}
	installGoalPlanner(s, p)
	q := fixtureQuery(s)
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "先检查照片，再看环境", Query: &q, Image: automationPhoto(t, 9), ImageSource: "user_upload"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if strings.Join(order, ",") != "image,environment" || len(done.Reports) != 2 {
		t.Fatalf("dependencies were ignored: %v %+v", order, done.Task)
	}
}

func TestGoalPlanRejectsContradictoryTimeAndCyclicDependencies(t *testing.T) {
	window := &Window{Start: time.Now().Add(-time.Hour), End: time.Now()}
	for _, mutate := range []func(*GoalPlan){
		func(p *GoalPlan) { p.Environment.Window = window },
		func(p *GoalPlan) { p.Environment.Comparison = window },
		func(p *GoalPlan) {
			p.Environment.DependsOn = []string{"image"}
			p.Image.DependsOn = []string{"environment"}
		},
		func(p *GoalPlan) { p.Image.Window = window },
	} {
		p := analysisGoal(true, true)
		mutate(&p)
		if validateGoalPlan(&Run{Question: "查看资料", Automation: &Automation{}}, &p) == nil {
			t.Fatal("invalid handoff was admitted")
		}
	}
}

func TestInventedExecutionCapabilityCannotCreateDraft(t *testing.T) {
	s := fixture(t, nil)
	p := GoalPlan{Summary: "模拟开启阀门", Execution: fixtureDevice("阀门", "simulation", "模拟开启阀门十秒", 10)}
	installGoalPlanner(s, p)
	understanding := s.cfg.Model
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		if out, ok := dst.(*ExecutionMatch); ok {
			out.CapabilityID = "invented-real-controller"
			return nil
		}
		return understanding.Generate(context.Background(), messages, nil, dst)
	})
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "模拟开启阀门十秒"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "planner_failed" || done.Task.Plan != nil || done.Task.Feedback != nil {
		t.Fatal("an invented capability reached execution")
	}
}

func TestIndependentSpeechActCheckStopsMethodQuestionBeforeDeviceExtraction(t *testing.T) {
	s := fixture(t, nil)
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		switch out := dst.(type) {
		case *goalOutline:
			*out = goalOutline{Summary: "开启设备", DeviceIntent: "operate"}
		case *speechAct:
			if strings.Contains(messages[1].Content, "previous_attempt") || strings.Contains(messages[1].Content, "capability") {
				t.Fatal("independent intent check received a candidate plan")
			}
			out.Intent = "方法咨询"
		default:
			t.Fatal("method question reached extraction or execution")
		}
		return nil
	})
	start := automationRequest(t, s, "POST", "/automation", automationInput{Goal: "怎样才能开启闸门十秒？"}, 202)
	done := waitAutomation(t, s, start.Task.ID)
	if done.Task.Status != "needs_input" || done.Task.Automation.GoalPlan.Execution != nil || strings.Join(done.Task.Automation.Needs, ",") != "clarification" {
		t.Fatalf("method question became an action: %+v", done.Task)
	}
}

type plannerSchemaModel func([]Message, any, any) error

func (f plannerSchemaModel) Generate(_ context.Context, messages []Message, schema, dst any) error {
	return f(messages, schema, dst)
}

func TestAnalysisSchemasKeepIndependentDependencyContracts(t *testing.T) {
	s := fixture(t, nil)
	seen := map[string]bool{}
	s.cfg.Model = plannerSchemaModel(func(_ []Message, schema, dst any) error {
		switch out := dst.(type) {
		case *goalOutline:
			*out = goalOutline{Summary: "先图后环境", Environment: true, Image: true}
		case *speechAct:
			out.Intent = "只读任务"
		case *ObservationRequest, *PhotoRequest:
			role, expected := "environment", "image"
			if _, ok := out.(*PhotoRequest); ok {
				role, expected = "image", "environment"
			}
			deps := schema.(map[string]any)["properties"].(map[string]any)["depends_on"].(map[string]any)
			values := deps["items"].(map[string]any)["enum"].([]string)
			if deps["maxItems"] != 1 || strings.Join(values, ",") != expected {
				t.Fatalf("%s received the wrong dependency contract: %+v", role, deps)
			}
			seen[role] = true
			if role == "environment" {
				*dst.(*ObservationRequest) = *analysisGoal(true, false).Environment
			} else {
				*dst.(*PhotoRequest) = *analysisGoal(false, true).Image
			}
		}
		return nil
	})
	run := Run{ID: id(), WorkspaceID: "w", Task: "automation", Question: "先检查照片再分析环境", Automation: &Automation{}}
	var plan GoalPlan
	if err := s.extractGoalPlan(context.Background(), &run, s.plannerInput(&run), &plan); err != nil || len(seen) != 2 {
		t.Fatalf("role schemas were not checked: %v %v", seen, err)
	}
}
