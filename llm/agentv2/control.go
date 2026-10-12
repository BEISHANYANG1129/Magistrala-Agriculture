package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ControlTask matches the legacy /control/task payload. Target is a device ID.
type ControlTask struct {
	TaskID   string             `json:"task_id"`
	TraceID  string             `json:"trace_id"`
	TaskType string             `json:"task_type"`
	Target   string             `json:"target"`
	Params   map[string]float64 `json:"params"`
	Source   string             `json:"source"`
}
type ControlPlan struct {
	WorkspaceID    string      `json:"workspace_id"`
	RegionID       string      `json:"region_id"`
	MappingVersion string      `json:"mapping_version"`
	Task           ControlTask `json:"task"`
}
type ControlReceipt struct {
	TaskID      string    `json:"task_id"`
	DeviceID    string    `json:"device_id"`
	Status      string    `json:"status"`
	Steps       []string  `json:"steps"`
	Detail      string    `json:"detail"`
	CompletedAt time.Time `json:"completed_at"`
}

// SimulationControl cannot dispatch real devices. Production transport is deliberately absent.
type SimulationControl interface {
	Simulate(context.Context, ControlPlan) (ControlReceipt, error)
}
type VirtualControl struct{}

func (VirtualControl) Simulate(ctx context.Context, p ControlPlan) (ControlReceipt, error) {
	if err := ctx.Err(); err != nil {
		return ControlReceipt{}, err
	}
	return ControlReceipt{TaskID: p.Task.TaskID, DeviceID: p.Task.Target, Status: "simulated_completed", Steps: []string{"open_valve", "virtual_wait", "close_valve"}, Detail: fmt.Sprintf("virtual duration %.3f min; no device command sent", p.Task.Params["duration_min"]), CompletedAt: time.Now().UTC()}, nil
}

func BuildControlPlans(reg Registry, r Result) ([]ControlPlan, error) {
	if err := reg.Validate(); err != nil {
		return nil, err
	}
	if r.MappingVersion != reg.Version {
		return nil, errors.New("mapping version mismatch")
	}
	region, err := reg.Find(r.WorkspaceID, r.RegionID)
	if err != nil {
		return nil, err
	}
	if region.IrrigationDurationMin == nil {
		return nil, errors.New("explicit region irrigation duration required; model cannot choose it")
	}
	plans := []ControlPlan{}
	seen := map[string]bool{}
	for _, task := range r.Tasks {
		if task.TaskID == "" || task.TraceID != r.AnalysisID || task.TaskType != "irrigation" || task.WorkspaceID != region.WorkspaceID || task.Target != region.ID || task.MappingVersion != reg.Version {
			return nil, errors.New("task scope differs from analysis")
		}
		if len(task.ExecutorIDs) == 0 || len(task.ExecutorIDs) != len(region.Executors) {
			return nil, errors.New("executor mapping differs from current registry")
		}
		allowed := map[string]bool{}
		for _, id := range region.Executors {
			allowed[id] = true
		}
		for _, id := range task.ExecutorIDs {
			if !allowed[id] || seen[id] {
				return nil, errors.New("unbound or duplicate executor")
			}
			seen[id] = true
			raw, _ := json.Marshal([]string{r.WorkspaceID, r.RegionID, task.TaskID, id})
			sum := sha256.Sum256(raw)
			plans = append(plans, ControlPlan{WorkspaceID: r.WorkspaceID, RegionID: r.RegionID, MappingVersion: reg.Version, Task: ControlTask{TaskID: hex.EncodeToString(sum[:]), TraceID: r.AnalysisID, TaskType: "irrigation", Target: id, Params: map[string]float64{"duration_min": *region.IrrigationDurationMin}, Source: "llm_advisory_simulation"}})
		}
	}
	if len(plans) == 0 {
		return nil, errors.New("no eligible task proposal")
	}
	return plans, nil
}
