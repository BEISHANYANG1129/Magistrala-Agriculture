package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func analyzed(t *testing.T, s *Server) Result {
	t.Helper()
	w := post(t, s.Handler(), "/v1/analyze", `{"workspace_id":"demo-farm","region_id":"north","mapping_version":"demo-v1"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var r Result
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func simulateID(t *testing.T, s *Server, id string) *httptest.ResponseRecorder {
	t.Helper()
	return post(t, s.Handler(), "/v1/simulate", `{"analysis_id":"`+id+`"}`)
}
func TestControlMappingRejectsWrongDeviceAndUsesPolicy(t *testing.T) {
	e := testEngine(t)
	result, err := e.Analyze(context.Background(), northRequest())
	if err != nil {
		t.Fatal(err)
	}
	reg, _, _ := LoadRegistry(e.RegistryPath)
	result.Tasks[0].Params["duration_min"] = 10000
	plans, err := BuildControlPlans(reg, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Task.Target != "valve-north" || plans[0].Task.Params["duration_min"] != 5 {
		t.Fatal(plans)
	}
	again, _ := BuildControlPlans(reg, result)
	if again[0].Task.TaskID != plans[0].Task.TaskID {
		t.Fatal("unstable idempotency key")
	}
	result.Tasks[0].ExecutorIDs = []string{"valve-south"}
	if _, err = BuildControlPlans(reg, result); err == nil {
		t.Fatal("cross-region executor accepted")
	}
	result.Tasks[0].ExecutorIDs = []string{"valve-north"}
	reg.Regions[0].IrrigationDurationMin = nil
	if _, err = BuildControlPlans(reg, result); err == nil {
		t.Fatal("missing policy accepted")
	}
}
func TestRegistryCASAndOldAnalysisInvalidation(t *testing.T) {
	e := testEngine(t)
	s := NewServer(e)
	s.EditableRegions = true
	result := analyzed(t, s)
	reg, hash, _ := LoadRegistry(e.RegistryPath)
	reg.Regions[0].Name = "updated north"
	raw, _ := json.Marshal(map[string]any{"expected_revision": hash, "registry": reg})
	first := post(t, s.Handler(), "/v1/regions", string(raw))
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body.String())
	}
	second := post(t, s.Handler(), "/v1/regions", string(raw))
	if second.Code != 409 {
		t.Fatal("lost-update guard failed", second.Code)
	}
	updated, _, _ := LoadRegistry(e.RegistryPath)
	if updated.Version == reg.Version || updated.Regions[0].Name != "updated north" {
		t.Fatal(updated)
	}
	if _, err := os.Stat(e.RegistryPath + ".backup.json"); err != nil {
		t.Fatal(err)
	}
	if got := simulateID(t, s, result.AnalysisID); got.Code != 409 {
		t.Fatal("old result executable after edit")
	}
}
func TestInvalidRegistrySaveIsNonDestructive(t *testing.T) {
	e := testEngine(t)
	s := NewServer(e)
	s.EditableRegions = true
	reg, hash, _ := LoadRegistry(e.RegistryPath)
	reg.Regions[1].Executors = reg.Regions[0].Executors
	raw, _ := json.Marshal(map[string]any{"expected_revision": hash, "registry": reg})
	if w := post(t, s.Handler(), "/v1/regions", string(raw)); w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	_, after, _ := LoadRegistry(e.RegistryPath)
	if after != hash {
		t.Fatal("invalid config changed file")
	}
	s.EditableRegions = false
	if w := post(t, s.Handler(), "/v1/regions", string(raw)); w.Code != 403 {
		t.Fatal("read-only config changed")
	}
}

type controlFunc func(context.Context, ControlPlan) (ControlReceipt, error)

func (f controlFunc) Simulate(c context.Context, p ControlPlan) (ControlReceipt, error) {
	return f(c, p)
}
func TestPersistentSimulationDoesNotReplayAfterRestart(t *testing.T) {
	e := testEngine(t)
	dir := t.TempDir()
	s, err := NewPersistentServer(e, dir)
	if err != nil {
		t.Fatal(err)
	}
	r := analyzed(t, s)
	first := simulateID(t, s, r.AnalysisID)
	if first.Code != 200 {
		t.Fatal(first.Body.String())
	}
	restored, err := NewPersistentServer(e, dir)
	if err != nil {
		t.Fatal(err)
	}
	restored.Control = controlFunc(func(context.Context, ControlPlan) (ControlReceipt, error) {
		t.Fatal("replayed completed task")
		return ControlReceipt{}, nil
	})
	second := simulateID(t, restored, r.AnalysisID)
	if second.Body.String() != first.Body.String() {
		t.Fatal("changed receipt after restart")
	}
	if len(restored.history("other-farm", "")) != 0 || len(restored.history("demo-farm", "south")) != 0 || len(restored.history("demo-farm", "north")) != 1 {
		t.Fatal("history scope leak")
	}
}
func TestFailedInterruptedAndInvalidReceiptsNeverClaimSuccess(t *testing.T) {
	for _, mode := range []string{"timeout", "wrong_device", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			e := testEngine(t)
			dir := t.TempDir()
			s, err := NewPersistentServer(e, dir)
			if err != nil {
				t.Fatal(err)
			}
			r := analyzed(t, s)
			calls := 0
			s.Control = controlFunc(func(c context.Context, p ControlPlan) (ControlReceipt, error) {
				calls++
				if mode == "timeout" {
					return ControlReceipt{}, context.DeadlineExceeded
				}
				return ControlReceipt{TaskID: p.Task.TaskID, DeviceID: "valve-south", Status: "simulated_completed"}, nil
			})
			if mode == "interrupted" {
				saved := s.results[r.AnalysisID]
				sim := Simulation{AnalysisID: r.AnalysisID, Status: "simulating"}
				if err = s.persist(saved, &sim); err != nil {
					t.Fatal(err)
				}
			} else {
				w := simulateID(t, s, r.AnalysisID)
				if w.Code != 200 || strings.Contains(w.Body.String(), `"status":"simulated_completed"`) {
					t.Fatal(w.Body.String())
				}
			}
			restored, err := NewPersistentServer(e, dir)
			if err != nil {
				t.Fatal(err)
			}
			restored.Control = controlFunc(func(context.Context, ControlPlan) (ControlReceipt, error) {
				t.Fatal("automatic retry")
				return ControlReceipt{}, nil
			})
			w := simulateID(t, restored, r.AnalysisID)
			if w.Code != 200 || strings.Contains(w.Body.String(), `"status":"simulated_completed"`) {
				t.Fatal(w.Body.String())
			}
			if mode != "interrupted" && calls != 1 {
				t.Fatal(calls)
			}
		})
	}
}
func TestConfigurationCannotChangeDuringAnalysis(t *testing.T) {
	e := testEngine(t)
	s := NewServer(e)
	s.EditableRegions = true
	reg, hash, _ := LoadRegistry(e.RegistryPath)
	e.Analyst = analystFunc(func(c context.Context, role string, r Region, v string, rows []Evidence, docs []Knowledge) (Assessment, error) {
		raw, _ := json.Marshal(map[string]any{"expected_revision": hash, "registry": reg})
		if w := post(t, s.Handler(), "/v1/regions", string(raw)); w.Code != 409 {
			t.Fatal("edited during analysis")
		}
		return (DemoAnalyst{}).Analyze(c, role, r, v, rows, docs)
	})
	analyzed(t, s)
}
func TestCorruptHistoryFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, strings.Repeat("a", 32)+".json"), []byte(`{"result":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPersistentServer(testEngine(t), dir); err == nil {
		t.Fatal("silently ignored corrupt history")
	}
}
func TestAnalysisErrorDoesNotCreateHistory(t *testing.T) {
	e := testEngine(t)
	e.Analyst = analystFunc(func(context.Context, string, Region, string, []Evidence, []Knowledge) (Assessment, error) {
		return Assessment{}, errors.New("model timed out")
	})
	s := NewServer(e)
	w := post(t, s.Handler(), "/v1/analyze", `{"workspace_id":"demo-farm","region_id":"north","mapping_version":"demo-v1"}`)
	if w.Code != 422 || len(s.results) != 0 || len(s.simulations) != 0 {
		t.Fatal("failed inference yielded runnable result")
	}
}
