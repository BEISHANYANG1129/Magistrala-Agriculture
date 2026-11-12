package workbench

import (
	"testing"
)

func TestCameraIntentCompilerPreservesUnitsAndNeverSubstitutesTargets(t *testing.T) {
	source := CameraSource{WorkspaceID: "w", DeviceID: "camera", Name: "乔安 G4S4 摄像头", Aliases: []string{"乔安", "G4S4"}}
	value := func(n float64) *float64 { return &n }
	for _, tc := range []struct {
		name, target, unit string
		amount             *float64
		wantMs             int
		unsupported        bool
	}{
		{"milliseconds", "乔安", "milliseconds", value(100), 100, false},
		{"fractional_seconds", "乔安", "seconds", value(.1), 100, false},
		{"direction_default", "G4S4", "unspecified", nil, 200, false},
		{"angle_is_not_time", "乔安", "degrees", value(90), 0, true},
		{"turns_are_not_time", "乔安", "turns", value(1), 0, true},
		{"long_move_not_truncated", "乔安", "seconds", value(5), 0, true},
		{"different_named_camera", "Go1", "milliseconds", value(100), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := Run{Question: "测试摄像头原始请求", Automation: &Automation{}}
			p, err := compileCameraIntent(&run, CameraIntent{Mode: "real", Target: tc.target, TargetSpecified: true, Quote: run.Question, Timing: "immediate", AnalysisKind: "scene", Steps: []CameraIntentStep{{Action: "move", Direction: "left", Unit: tc.unit, Amount: tc.amount}}}, []CameraSource{source})
			if err != nil {
				t.Fatal(err)
			}
			if tc.unsupported {
				if p.Unsupported == "" || len(p.Operations) != 0 {
					t.Fatalf("unsupported request became an executable approximation: %+v", p)
				}
			} else {
				if p.Unsupported != "" || len(p.Operations) != 1 || p.Operations[0].DurationMs != tc.wantMs || p.DeviceID != source.DeviceID {
					t.Fatalf("wrong compiled movement: %+v", p)
				}
			}
			if p.Intent.Steps[0].Unit != tc.unit {
				t.Fatal("source unit was rewritten")
			}
		})
	}
}

func TestCameraIntentCompilerDoesNotRunPartialPlansOrTurnSimulationIntoRealActions(t *testing.T) {
	source := CameraSource{WorkspaceID: "w", DeviceID: "camera", Name: "乔安"}
	amount, angle, view := 100.0, 90.0, 9
	move := CameraIntentStep{Action: "move", Direction: "left", Amount: &amount, Unit: "milliseconds"}
	for _, tc := range []struct {
		mode  string
		steps []CameraIntentStep
	}{
		{"real", []CameraIntentStep{move, {Action: "move", Direction: "right", Amount: &angle, Unit: "degrees"}}},
		{"real", []CameraIntentStep{move, {Action: "snapshot", Unit: "unspecified", View: &view}}},
		{"simulation", []CameraIntentStep{move}},
	} {
		run := Run{Question: "测试原始需求", Automation: &Automation{}}
		p, err := compileCameraIntent(&run, CameraIntent{Mode: tc.mode, Quote: run.Question, Timing: "immediate", AnalysisKind: "scene", Steps: tc.steps}, []CameraSource{source})
		if err != nil || p.Unsupported == "" || len(p.Operations) != 0 {
			t.Fatalf("unsafe partial plan: %+v %v", p, err)
		}
	}
}

func TestCameraIntentSnapshotCountIsIndependentOfViewAndDuration(t *testing.T) {
	for _, tc := range []struct {
		unit   string
		amount float64
		want   int
	}{
		{"unspecified", 1, 1}, {"count", 1, 1}, {"count", 2, 2}, {"count", 9, 0}, {"degrees", 1, 0}, {"milliseconds", 100, 0},
	} {
		run := Run{Question: "截图请求", Automation: &Automation{}}
		view := 1
		p, err := compileCameraIntent(&run, CameraIntent{Mode: "real", Quote: run.Question, Timing: "immediate", AnalysisKind: "scene", Steps: []CameraIntentStep{{Action: "snapshot", Unit: tc.unit, Amount: &tc.amount, View: &view}}}, []CameraSource{{Name: "乔安", DeviceID: "camera"}})
		if err != nil || len(p.Operations) != tc.want || (tc.want == 0 && p.Unsupported == "") {
			t.Fatalf("wrong snapshot quantity: %+v %v", p, err)
		}
		for _, op := range p.Operations {
			if op.Action != "snapshot" || op.View != 1 || op.DurationMs != 0 {
				t.Fatalf("quantity changed the camera action: %+v", op)
			}
		}
	}
}
