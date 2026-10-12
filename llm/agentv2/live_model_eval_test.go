package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type evaluatedCall struct {
	Messages  []llms.MessageContent `json:"messages"`
	Response  *llms.ContentResponse `json:"response,omitempty"`
	ElapsedMS int64                 `json:"elapsed_ms"`
	Error     string                `json:"error,omitempty"`
}

type recordingEvalModel struct {
	llms.Model
	Calls []evaluatedCall
}

func (m *recordingEvalModel) GenerateContent(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	if guard := os.Getenv("AGRI_EVAL_GPU_GUARD"); guard != "" {
		if output, err := exec.CommandContext(ctx, os.Getenv("AGRI_EVAL_PYTHON"), guard, "--check-only").CombinedOutput(); err != nil {
			return nil, fmt.Errorf("GPU idle check refused model call: %w: %s", err, output)
		}
	}
	start := time.Now()
	response, err := m.Model.GenerateContent(ctx, messages, opts...)
	call := evaluatedCall{Messages: messages, Response: response, ElapsedMS: time.Since(start).Milliseconds()}
	if err != nil {
		call.Error = err.Error()
	}
	m.Calls = append(m.Calls, call)
	return response, err
}

// Opt-in evaluation, not a statistical model-quality benchmark. The launcher
// must check the shared GPU immediately before each selected subtest.
func TestLiveModelEvaluation(t *testing.T) {
	if os.Getenv("AGRI_MODEL_EVAL") != "yes" {
		t.Skip("real model evaluation not enabled")
	}
	if os.Getenv("AGRI_GPU_IDLE_CONFIRMED") != "yes" {
		t.Fatal("fresh external GPU check required")
	}
	model, auth, output := os.Getenv("AGRI_LIVE_MODEL"), os.Getenv("AGRI_READER_AUTH"), os.Getenv("AGRI_EVAL_OUTPUT")
	if model == "" || auth == "" || output == "" {
		t.Fatal("model, Reader authorization and dedicated output directory required")
	}
	reg, _, err := LoadRegistry(os.Getenv("AGRI_LIVE_REGISTRY"))
	if err != nil {
		t.Fatal(err)
	}
	docBytes, err := os.ReadFile("examples/knowledge.json")
	if err != nil {
		t.Fatal(err)
	}
	var docs []Knowledge
	if err = json.Unmarshal(docBytes, &docs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"north", "south", "no_threshold", "two_experts", "missing", "stale"} {
		t.Run(name, func(t *testing.T) {
			regionID := "north"
			if name == "south" {
				regionID = "south"
			}
			var region Region
			for _, candidate := range reg.Regions {
				if candidate.ID == regionID {
					region = candidate
				}
			}
			if region.ID == "" || len(region.Sensors) != 1 {
				t.Fatal("expected single test sensor binding")
			}
			if name == "no_threshold" {
				region.SoilMoistureMin = nil
			}
			isolated := Registry{Version: "model-eval-" + name, Regions: []Region{region}}
			registryPath := filepath.Join(t.TempDir(), "registry.json")
			raw, err := json.Marshal(isolated)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(registryPath, raw, 0600); err != nil {
				t.Fatal(err)
			}
			analyst, err := NewOllama("http://127.0.0.1:11434", model)
			if err != nil {
				t.Fatal(err)
			}
			recorder := &recordingEvalModel{Model: analyst.Model}
			analyst.Model = recorder
			var source Source = &Reader{BaseURL: "http://localhost:9011", Authorization: auth, TimeUnit: "nanoseconds"}
			dataOrigin := "live Magistrala Reader; authorized synthetic telemetry"
			if name == "two_experts" || name == "missing" || name == "stale" {
				dataOrigin = "isolated fixture; not real field telemetry"
				source = sourceFunc(func(_ context.Context, _ string, binding Binding, _ time.Time) ([]Evidence, error) {
					if name == "missing" {
						return nil, nil
					}
					observed := time.Now().UTC()
					if name == "stale" {
						observed = observed.Add(-time.Hour)
					}
					rows := []Evidence{{ID: "eval-soil", ChannelID: binding.ChannelID, PublisherID: binding.PublisherID, Metric: "soil_moisture", Unit: "%", Value: 12, ObservedAt: observed}}
					if name == "two_experts" {
						rows = append(rows, Evidence{ID: "eval-air", ChannelID: binding.ChannelID, PublisherID: binding.PublisherID, Metric: "air_temperature", Unit: "Cel", Value: 28, ObservedAt: observed})
					}
					return rows, nil
				})
			}
			engine := &Engine{RegistryPath: registryPath, Source: source, Analyst: analyst, Knowledge: docs, MaxAge: 15 * time.Minute, Mode: "live_ollama_evaluation"}
			var result Result
			var plans []ControlPlan
			var receipts []ControlReceipt
			defer func() {
				report := map[string]any{"case": name, "model": model, "created_at": time.Now().UTC(), "passed": !t.Failed(), "data_origin": dataOrigin, "calls": recorder.Calls, "result": result, "plans": plans, "receipts": receipts, "physical_dispatch": false}
				b, marshalErr := json.MarshalIndent(report, "", "  ")
				if marshalErr != nil {
					t.Error(marshalErr)
					return
				}
				if writeErr := os.WriteFile(filepath.Join(output, name+".json"), b, 0600); writeErr != nil {
					t.Error(writeErr)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 190*time.Second)
			defer cancel()
			result, err = engine.Analyze(ctx, Request{WorkspaceID: region.WorkspaceID, RegionID: region.ID, MappingVersion: isolated.Version})
			if err != nil {
				t.Fatal(err)
			}
			if name == "missing" || name == "stale" {
				if result.Status != "insufficient_data" || len(result.Tasks) != 0 || len(recorder.Calls) != 0 {
					t.Fatal("invalid evidence triggered analysis or task")
				}
				return
			}
			wantCalls := 1
			if name == "two_experts" {
				wantCalls = 2
			}
			if len(recorder.Calls) != wantCalls || len(result.Assessments) != wantCalls {
				t.Fatalf("expected %d expert calls", wantCalls)
			}
			for _, row := range result.Evidence {
				if row.PublisherID != region.Sensors[0].PublisherID {
					t.Fatal("cross-region evidence")
				}
			}
			if name == "north" || name == "no_threshold" {
				if len(result.Evidence) != 1 || result.Evidence[0].Value != 12 {
					t.Fatal("north sample must be 12 percent")
				}
			}
			if name == "south" && (len(result.Evidence) != 1 || result.Evidence[0].Value != 35) {
				t.Fatal("south sample must be 35 percent")
			}
			if name == "south" || name == "no_threshold" {
				if len(result.Tasks) != 0 || result.Status != "analyzed" {
					t.Fatal("unexpected proposal without low moisture and configured threshold")
				}
				for _, a := range result.Assessments {
					for _, finding := range a.Findings {
						if finding.Kind == "soil_dry" {
							t.Fatal("unsupported dry diagnosis")
						}
					}
				}
			} else {
				if len(result.Tasks) != 1 || result.Status != "proposal_ready" {
					t.Fatal("expected one validated low-moisture proposal")
				}
				plans, err = BuildControlPlans(isolated, result)
				if err != nil {
					t.Fatal(err)
				}
				for _, plan := range plans {
					if plan.Task.Target != "simulation-only-valve-north" || plan.Task.Params["duration_min"] != 5 {
						t.Fatal("incorrect executor or duration")
					}
					receipt, simErr := (VirtualControl{}).Simulate(ctx, plan)
					if simErr != nil || receipt.Status != "simulated_completed" || receipt.DeviceID != plan.Task.Target {
						t.Fatal("invalid simulated receipt")
					}
					receipts = append(receipts, receipt)
				}
			}
			t.Logf("case=%s calls=%d status=%s evidence=%d tasks=%d", name, len(recorder.Calls), result.Status, len(result.Evidence), len(result.Tasks))
		})
	}
}
