package workbench

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

type roleModel func([]Message, any) error

func (f roleModel) Generate(_ context.Context, msg []Message, _ any, dst any) error {
	return f(msg, dst)
}

func TestEnvironmentHandsOffAdmittedEvidenceAndKeepsVerifiedFacts(t *testing.T) {
	s := fixture(t, nil)
	s.dataset.Metrics = append(s.dataset.Metrics, Metric{Key: "air_humidity", Label: "空气湿度", Unit: "%", Admission: "include"})
	for i := range s.dataset.Frames {
		s.dataset.Frames[i].Values["air_humidity"] = 50
	}
	var primary Summary
	calls := 0
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		calls++
		var input struct {
			Windows []*Evidence      `json:"windows"`
			Quality []QualityFinding `json:"quality"`
		}
		if err := json.Unmarshal([]byte(messages[1].Content), &input); err != nil {
			t.Fatal(err)
		}
		if len(input.Windows) != 2 || len(input.Quality) == 0 {
			t.Fatal("quality result or comparison evidence missing")
		}
		for _, m := range input.Windows[0].Metrics {
			if m.Key == "pm25" {
				t.Fatal("excluded numeric evidence reached a downstream model")
			}
			if m.Key == "air_temperature" {
				primary = m
			}
		}
		if out, ok := dst.(*ModelOutput); ok {
			out.Claims = []Claim{
				{Text: "时段 A 的气温为 30 °C。", Kind: "observation", Citations: []string{primary.ID}},
				{Text: "最高气温 99 °C。", Kind: "observation", Citations: []string{primary.ID}},
				{Text: "时段 B 气温 30 °C，空气湿度 30%。", Kind: "observation", Citations: []string{primary.ID}},
			}
			return nil
		}
		if strings.Contains(messages[1].Content, "99 °C") {
			t.Fatal("hard-rejected claim reached semantic review")
		}
		return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"与时段 A 观测一致"}]}`), dst)
	})
	q := fixtureQuery(s)
	end := q.End
	q.End = q.Start.Add(30 * time.Second)
	in := Input{Task: "environment", Question: "比较两个时段的气温和数据质量", Query: &q,
		Compare: &Window{Start: q.End, End: end}}
	r, err := s.run(context.Background(), "w", "", in)
	if err != nil || calls != 2 || r.Workflow == nil || r.Workflow.Snapshot == "" ||
		r.Status != "evidence_summary" || len(r.Claims) != 2 || r.Comparison == nil {
		t.Fatalf("unexpected report: %+v calls=%d err=%v", r, calls, err)
	}
	if r.Claims[0].Citations[0] != primary.ID || !strings.Contains(r.Claims[1].Text, "时段 B") ||
		r.Workflow.Verification[1].Verdict != "rejected" || r.Workflow.Verification[2].Verdict != "rejected" {
		t.Fatalf("failed to preserve verified claims and fill missing window: %+v", r.Claims)
	}
}

func TestImageRolesUseToolFindingsAndStopOnNonLeaf(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 180, 180))); err != nil {
		t.Fatal(err)
	}
	in := Input{Task: "image", Question: "描述叶片异常", Crop: "coffee", ImageSource: "public_dataset", Image: base64.StdEncoding.EncodeToString(b.Bytes())}
	s := fixture(t, nil)
	subject, asset, calls := "leaf", "", 0
	s.cfg.VisualTool = func(_ context.Context, input VisualInput) (VisionFinding, error) {
		asset = input.AssetID
		if input.Image != in.Image || input.Width != 180 {
			t.Fatal("visual tool did not receive the original")
		}
		return VisionFinding{Version: "fixture-tool", Subject: subject, Quality: "usable", Visible: []string{"叶缘有褐色区域。"}}, nil
	}
	s.cfg.Model = roleModel(func(msg []Message, dst any) error {
		calls++
		if len(msg[1].Images) != 0 || !strings.Contains(msg[1].Content, "叶缘有褐色区域") {
			t.Fatal("downstream role did not receive the tool findings alone")
		}
		if out, ok := dst.(*ModelOutput); ok {
			out.ImageObservations = []Claim{{Text: "叶缘有褐色区域。", Kind: "image", Citations: []string{asset}}}
			return nil
		}
		return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"有视觉依据"}]}`), dst)
	})
	r, err := s.run(context.Background(), "w", "", in)
	if err != nil || r.Status != "completed" || len(r.Claims) != 1 || r.Workflow.Vision.Version != "fixture-tool" || calls != 2 {
		t.Fatalf("image handoff failed: %+v, %v", r, err)
	}
	subject = "non_leaf"
	r, err = s.run(context.Background(), "w", "", in)
	if err != nil || r.Status != "needs_input" || calls != 2 || len(r.Claims) != 0 {
		t.Fatalf("non-leaf should stop before interpretation: %+v, %v", r, err)
	}
}

type missingReadback struct{ localSimulation }

func TestImageDeduplicatesSharedOutputClaimsBeforeReview(t *testing.T) {
	s := fixture(t, nil)
	asset := ""
	s.cfg.VisualTool = func(_ context.Context, in VisualInput) (VisionFinding, error) {
		asset = in.AssetID
		return VisionFinding{Version: "fixture", Subject: "non_leaf", Quality: "usable", Visible: []string{"可见白色键盘。"}}, nil
	}
	s.cfg.Model = roleModel(func(messages []Message, dst any) error {
		if out, ok := dst.(*ModelOutput); ok {
			claim := Claim{Text: "可见白色键盘。", Kind: "image", Citations: []string{asset}}
			out.Claims = []Claim{claim}
			out.ImageObservations = []Claim{claim}
			return nil
		}
		var review struct {
			Candidates []json.RawMessage `json:"candidates"`
		}
		_ = json.Unmarshal([]byte(messages[1].Content), &review)
		if len(review.Candidates) != 1 {
			t.Errorf("same image fact submitted for review %d times", len(review.Candidates))
		}
		return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"有视觉依据"}]}`), dst)
	})
	r, err := s.run(context.Background(), "w", "", Input{Task: "image", Question: "概述画面", Image: automationPhoto(t, 42), ImageSource: "platform_photo", platformPhoto: true, sceneImage: true})
	if err != nil || r.Status != "completed" || len(r.Claims) != 1 {
		t.Fatalf("duplicate report: %+v %v", r, err)
	}
}

func TestSceneDescribesVisibleObjectsWithLimitedQualityWithoutWeakeningLeafChecks(t *testing.T) {
	for _, tc := range []struct {
		name, subject string
		scene         bool
		want          string
		calls         int
	}{
		{"scene_partial_blur", "non_leaf", true, "completed", 2},
		{"leaf_partial_blur", "leaf", false, "needs_input", 0},
		{"leaf_wrong_subject", "non_leaf", false, "needs_input", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t, nil)
			asset, calls := "", 0
			s.cfg.VisualTool = func(_ context.Context, in VisualInput) (VisionFinding, error) {
				asset = in.AssetID
				return VisionFinding{Version: "fixture", Subject: tc.subject, Quality: "limited", Visible: []string{"画面中可辨认白色键盘，部分细节模糊。"}}, nil
			}
			s.cfg.Model = roleModel(func(_ []Message, dst any) error {
				calls++
				if out, ok := dst.(*ModelOutput); ok {
					out.ImageObservations = []Claim{{Text: "画面中可辨认白色键盘，部分细节模糊。", Kind: "image", Citations: []string{asset}}}
					return nil
				}
				return json.Unmarshal([]byte(`{"checks":[{"index":0,"verdict":"approved","reason":"描述受可见发现支持，并保留模糊限制"}]}`), dst)
			})
			result, err := s.run(context.Background(), "w", "", Input{Task: "image", Question: "概述画面中可辨认的内容", Image: automationPhoto(t, 45), ImageSource: "platform_photo", platformPhoto: true, sceneImage: tc.scene})
			if err != nil || result.Status != tc.want || calls != tc.calls {
				t.Fatalf("wrong quality handling: status=%s calls=%d err=%v limits=%v", result.Status, calls, err, result.Limitations)
			}
			if tc.scene && (len(result.Claims) != 1 || strings.Contains(strings.Join(result.Limitations, ""), "叶片")) {
				t.Fatalf("scene was treated as a leaf request: %+v", result)
			}
		})
	}
}

func (*missingReadback) readBack(context.Context, string) (ExecutionFeedback, error) {
	return ExecutionFeedback{}, errors.New("missing simulation feedback")
}

func TestExecutionUsesAnalysisReferenceAndRequiresReadback(t *testing.T) {
	s := fixture(t, nil)
	q := fixtureQuery(s)
	basis, err := s.run(context.Background(), "w", "", Input{Task: "environment", Question: "简要说明这段时间", Query: &q})
	if err != nil || basis.Status != "evidence_summary" {
		t.Fatalf("missing program brief: %+v, %v", basis, err)
	}
	args := map[string]any{"action": "open", "duration_seconds": 5, "basis_run_id": basis.ID}
	if code, _ := call(t, s, "POST", "/plans", "other", args); code != 422 {
		t.Fatal("cross-workspace analysis reference accepted")
	}
	if err := s.store(Run{ID: "public-image", WorkspaceID: "w", Task: "image", Status: "completed", Workflow: basis.Workflow}); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(t, s, "POST", "/plans", "w", map[string]any{"action": "close", "basis_run_id": "public-image"}); code != 422 {
		t.Fatal("unlinked image was accepted as a valve reference")
	}
	for _, missing := range []bool{false, true} {
		code, body := call(t, s, "POST", "/plans", "w", args)
		var plan Run
		if code != 200 || json.Unmarshal(body, &plan) != nil || plan.Plan.BasisSnapshot != basis.Workflow.Snapshot || plan.Feedback != nil {
			t.Fatalf("wrong plan: %d %s", code, body)
		}
		if missing {
			s.simulation = &missingReadback{}
		}
		confirm := map[string]any{"id": plan.ID, "revision": plan.Plan.Revision, "confirm": true}
		code, body = call(t, s, "POST", "/simulate", "w", confirm)
		var result Run
		if code != 200 || json.Unmarshal(body, &result) != nil || result.Approval == nil || result.Approval.Revision != plan.Plan.Revision {
			t.Fatalf("wrong execution result: %d %s", code, body)
		}
		if missing {
			if result.Status != "simulation_unknown" || result.Feedback != nil {
				t.Fatal("missing feedback was treated as success")
			}
			if code, _ := call(t, s, "POST", "/simulate", "w", confirm); code != 409 {
				t.Fatal("unknown execution was replayed")
			}
		} else if result.Status != "simulated_completed" || result.Feedback == nil ||
			result.Feedback.State != "closed" || len(result.Feedback.Transitions) != 2 ||
			result.Feedback.Transitions[1].OffsetSeconds != 5 {
			t.Fatalf("wrong simulation feedback: %+v", result.Feedback)
		}
	}
}
