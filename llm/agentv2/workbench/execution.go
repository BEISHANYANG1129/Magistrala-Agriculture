package workbench

import (
	"context"
	"fmt"
	"time"
)

const (
	simulatedValve    = "simulation-only-valve"
	simulationVersion = "simulated-valve-v1"
)

type Approval struct {
	Revision    string    `json:"revision"`
	ConfirmedAt time.Time `json:"confirmed_at"`
	Subject     string    `json:"subject,omitempty"`
}

type ExecutionFeedback struct {
	CommandID   string          `json:"command_id"`
	Mode        string          `json:"mode"`
	State       string          `json:"state"`
	CheckedAt   time.Time       `json:"checked_at"`
	Transitions []ValvePosition `json:"transitions"`
}

type ValvePosition struct {
	OffsetSeconds int    `json:"offset_seconds"`
	State         string `json:"state"`
}

type simulationAdapter interface {
	dispatch(context.Context, string, Plan) error
	readBack(context.Context, string) (ExecutionFeedback, error)
}

type localSimulation struct {
	commandID   string
	state       string
	transitions []ValvePosition
}

func (a *localSimulation) dispatch(ctx context.Context, commandID string, plan Plan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkSimulation(plan); err != nil {
		return err
	}
	a.commandID = commandID
	if plan.Action == "open" {
		a.state = "open"
		a.transitions = append(a.transitions, ValvePosition{State: a.state})
	}
	a.state = "closed"
	a.transitions = append(a.transitions, ValvePosition{OffsetSeconds: plan.DurationSeconds, State: a.state})
	return nil
}

func (a *localSimulation) readBack(ctx context.Context, commandID string) (ExecutionFeedback, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionFeedback{}, err
	}
	if a.commandID != commandID || a.state == "" {
		return ExecutionFeedback{}, fmt.Errorf("simulation command has no feedback")
	}
	return ExecutionFeedback{CommandID: commandID, Mode: "simulation_only", State: a.state, CheckedAt: time.Now().UTC(), Transitions: a.transitions}, nil
}

func checkSimulation(plan Plan) error {
	if plan.Target != simulatedValve || plan.Mode != "simulation_only" || plan.Capability != simulationVersion || plan.StopAction != "close" {
		return fmt.Errorf("simulation capability changed or unavailable")
	}
	switch plan.Action {
	case "open":
		if plan.DurationSeconds >= 1 && plan.DurationSeconds <= 300 {
			return nil
		}
	case "close":
		if plan.DurationSeconds == 0 {
			return nil
		}
	}
	return fmt.Errorf("simulation open duration is 1–300 seconds; close duration must be zero")
}

func executionWorkflow() *Workflow {
	return &Workflow{Version: workflowVersion, Steps: []WorkflowStep{
		{ID: "planner", Status: "completed", Detail: "登记用户选定的模拟目标、动作和停止步骤。"},
		{ID: "reviewer", DependsOn: []string{"planner"}, Status: "completed", Detail: "程序核对目标能力、时长和关闭条件。"},
		{ID: "approval", DependsOn: []string{"reviewer"}, Status: "pending"},
		{ID: "executor", DependsOn: []string{"approval"}, Status: "pending"},
		{ID: "feedback", DependsOn: []string{"executor"}, Status: "pending"},
	}}
}

func (w *Workflow) mark(id, status, detail string) {
	if w == nil {
		return
	}
	for i := range w.Steps {
		if w.Steps[i].ID == id {
			w.Steps[i].Status, w.Steps[i].Detail = status, detail
			return
		}
	}
}

func (s *Service) runSimulation(ctx context.Context, run *Run) {
	var adapter simulationAdapter = &localSimulation{}
	if s.simulation != nil {
		adapter = s.simulation
	}
	command := run.ID
	if err := adapter.dispatch(ctx, command, *run.Plan); err != nil {
		run.Status = "simulation_unknown"
		run.Workflow.mark("executor", "failed", "模拟适配器未确认受理。")
		run.Receipt = "模拟动作状态未知，需重新建立草案。"
		return
	}
	run.Workflow.mark("executor", "completed", "模拟适配器已受理。")
	feedback, err := adapter.readBack(ctx, command)
	if err != nil || feedback.CommandID != command || feedback.Mode != "simulation_only" || feedback.State != "closed" {
		run.Status = "simulation_unknown"
		run.Workflow.mark("feedback", "failed", "未读到此模拟指令已关闭的状态。")
		run.Receipt = "模拟动作状态未知，需重新建立草案。"
		return
	}
	run.Feedback = &feedback
	run.Status = "simulated_completed"
	run.Receipt = "已读取模拟阀门状态为关闭。反馈来自模拟适配器，真实设备状态和农业效果尚未验证。"
	run.Workflow.mark("feedback", "completed", "读取模拟状态，确认关闭步骤完成。")
}
