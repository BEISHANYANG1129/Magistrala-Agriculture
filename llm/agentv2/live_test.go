package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Explicit opt-in: ordinary go test never contacts a running service or model.
// Before setting AGRI_LIVE_MODEL, the operator must check that no other task is using the GPU.
func TestLiveWorkflow(t *testing.T) {
	registryPath := os.Getenv("AGRI_LIVE_REGISTRY")
	if registryPath == "" {
		t.Skip("live platform test not enabled")
	}
	auth := os.Getenv("AGRI_READER_AUTH")
	if auth == "" {
		t.Fatal("AGRI_READER_AUTH required")
	}
	reg, _, err := LoadRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	engine := &Engine{RegistryPath: registryPath, MaxAge: 15 * time.Minute, Mode: "live_reader_mock_analyst",
		Source: &Reader{BaseURL: "http://localhost:9011", Authorization: auth, TimeUnit: "nanoseconds"}, Analyst: DemoAnalyst{}}
	if model := os.Getenv("AGRI_LIVE_MODEL"); model != "" {
		if os.Getenv("AGRI_GPU_IDLE_CONFIRMED") != "yes" {
			t.Fatal("fresh GPU occupancy check required")
		}
		engine.Mode = "live_reader_ollama"
		engine.Analyst, err = NewOllama("http://127.0.0.1:11434", model)
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	region := reg.Regions[0]
	result, err := engine.Analyze(ctx, Request{WorkspaceID: region.WorkspaceID, RegionID: region.ID, MappingVersion: reg.Version})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evidence) == 0 || len(result.Assessments) == 0 {
		t.Fatal("missing fresh evidence or validated assessment")
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join("runtime", result.Mode+"-result.json")
	if err = os.WriteFile(destination, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("mode=%s status=%s evidence=%d assessments=%d proposals=%d (no device dispatch); saved %s", result.Mode, result.Status, len(result.Evidence), len(result.Assessments), len(result.Tasks), destination)
}
