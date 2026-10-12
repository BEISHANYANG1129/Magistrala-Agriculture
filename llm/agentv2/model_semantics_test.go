package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// Regression from the real gemma3:12b response that called 35% below 20%.
// Valid JSON and real evidence IDs must not make a contradictory diagnosis valid.
func TestModelThresholdContradictionsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name, region, kind string
		removeThreshold    bool
	}{
		{"wet_called_dry", "south", "soil_dry", false},
		{"dry_called_normal", "north", "normal", false},
		{"missing_threshold_called_dry", "north", "soil_dry", true},
		{"missing_threshold_called_normal", "north", "normal", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := testEngine(t)
			if tc.removeThreshold {
				reg, _, err := LoadRegistry(e.RegistryPath)
				if err != nil {
					t.Fatal(err)
				}
				for i := range reg.Regions {
					reg.Regions[i].SoilMoistureMin = nil
				}
				raw, err := json.Marshal(reg)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(e.RegistryPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			e.Analyst = analystFunc(func(_ context.Context, _ string, r Region, version string, rows []Evidence, _ []Knowledge) (Assessment, error) {
				assessment := Assessment{RegionID: r.ID, MappingVersion: version, Findings: []Finding{{Kind: tc.kind, Explanation: "Contradictory model response", EvidenceIDs: []string{rows[0].ID}}}}
				return assessment, nil
			})
			req := northRequest()
			req.RegionID = tc.region
			result, err := e.Analyze(context.Background(), req)
			if err == nil || len(result.Tasks) != 0 || len(result.Assessments) != 0 {
				t.Fatal("contradictory model diagnosis reached a successful result")
			}
		})
	}
}
