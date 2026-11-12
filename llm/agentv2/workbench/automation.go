package workbench

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

type Automation struct {
	Camera           *CameraTask        `json:"camera,omitempty"`
	Query            *Query             `json:"query,omitempty"`
	Compare          *Window            `json:"compare,omitempty"`
	Image            *imageAsset        `json:"image,omitempty"`
	Crop             string             `json:"crop,omitempty"`
	Duration         *int               `json:"duration_seconds,omitempty"`
	Needs            []string           `json:"needs,omitempty"`
	Steps            []AutomationStep   `json:"steps"`
	UpdatedAt        time.Time          `json:"updated_at"`
	TimeZone         string             `json:"time_zone,omitempty"`
	Observation      *MaterialReference `json:"observation_source,omitempty"`
	Photo            *MaterialReference `json:"photo_source,omitempty"`
	GoalPlan         *GoalPlan          `json:"goal_plan,omitempty"`
	InputNeeds       []InputNeed        `json:"input_needs,omitempty"`
	Replies          []string           `json:"replies,omitempty"`
	PlannedReplies   int                `json:"planned_replies,omitempty"`
	ExecutionMode    string             `json:"execution_mode,omitempty"`
	PlanningAttempts []PlanningAttempt  `json:"planning_attempts,omitempty"`
	ExecutionMatch   *ExecutionMatch    `json:"execution_match,omitempty"`
	MatchRevision    string             `json:"execution_match_revision,omitempty"`
	PlanningVersion  string             `json:"planning_version,omitempty"`
}

type AutomationStep struct {
	Role   string `json:"role"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	RunID  string `json:"run_id,omitempty"`
}

type automationInput struct {
	Goal          string  `json:"goal"`
	Query         *Query  `json:"query,omitempty"`
	Compare       *Window `json:"compare,omitempty"`
	Image         string  `json:"image,omitempty"`
	ImageSource   string  `json:"image_source,omitempty"`
	Crop          string  `json:"crop,omitempty"`
	Duration      *int    `json:"duration_seconds,omitempty"`
	TimeZone      string  `json:"time_zone,omitempty"`
	Clarification string  `json:"clarification,omitempty"`
	ExecutionMode string  `json:"execution_mode,omitempty"`
}

type automationDetail struct {
	Task    Run   `json:"task"`
	Reports []Run `json:"reports"`
}

func (s *Service) automationRun(workspace, id string) (Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	return run, ok && run.Task == "automation" && run.WorkspaceID == workspace && run.Automation != nil
}

func (s *Service) automationDetail(run Run) automationDetail {
	s.mu.Lock()
	defer s.mu.Unlock()
	detail := automationDetail{Task: run, Reports: []Run{}}
	for _, role := range []string{"environment", "image"} {
		for _, step := range run.Automation.Steps {
			if report, ok := s.runs[step.RunID]; step.Role == role && ok && report.WorkspaceID == run.WorkspaceID {
				detail.Reports = append(detail.Reports, report)
			}
		}
	}
	if camera := run.Automation.Camera; camera != nil {
		for _, capture := range camera.Captures {
			if report, ok := s.runs[capture.ReportID]; ok && report.WorkspaceID == run.WorkspaceID && !slices.ContainsFunc(detail.Reports, func(r Run) bool { return r.ID == report.ID }) {
				detail.Reports = append(detail.Reports, report)
			}
		}
	}
	return detail
}

func (s *Service) handleAutomation(w http.ResponseWriter, r *http.Request, path, workspace, authorization string) {
	if s.handleCameraAutomation(w, r, path, workspace, authorization) {
		return
	}
	key := strings.TrimPrefix(path, "/automation")
	if r.Method == "GET" && key == "" {
		type entry struct {
			ID        string    `json:"id"`
			Goal      string    `json:"goal"`
			Status    string    `json:"status"`
			UpdatedAt time.Time `json:"updated_at"`
		}
		list := []entry{}
		for _, run := range s.visibleCameraRuns(r.Context(), workspace, s.history(workspace, "automation")) {
			if run.Automation != nil {
				list = append(list, entry{run.ID, run.Question, run.Status, run.Automation.UpdatedAt})
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
		respond(w, 200, list)
		return
	}
	taskID, resume := strings.CutSuffix(strings.TrimPrefix(key, "/"), "/resume")
	if r.Method == "GET" && !resume && taskID != "" {
		if run, ok := s.automationRun(workspace, taskID); ok {
			if camera := run.Automation.Camera; camera != nil {
				if err := s.checkCamera(r.Context(), workspace, camera.Source, false); err != nil {
					problem(w, 403, fmt.Errorf("当前账号无法访问此摄像头任务"))
					return
				}
			}
			respond(w, 200, s.automationDetail(run))
			return
		}
	}
	if r.Method != "POST" || (key != "" && !resume) {
		problem(w, 404, fmt.Errorf("automation task unavailable"))
		return
	}
	var input automationInput
	if err := readJSON(w, r, &input); err != nil {
		problem(w, 400, err)
		return
	}
	select {
	case s.cfg.Gate <- struct{}{}:
	default:
		problem(w, 429, fmt.Errorf("当前有任务正在运行，请稍后再提交。"))
		return
	}
	started := false
	defer func() {
		if !started {
			<-s.cfg.Gate
		}
	}()
	run := Run{ID: id(), Task: "automation", WorkspaceID: workspace, CreatedAt: time.Now().UTC(), Model: s.cfg.ModelName, Automation: &Automation{}}
	if resume {
		var ok bool
		run, ok = s.automationRun(workspace, taskID)
		if !ok {
			problem(w, 404, fmt.Errorf("automation task unavailable"))
			return
		}
		cameraReplan := run.Status == "camera_unknown" && cameraUndispatched(run.Automation.Camera)
		if run.Status != "needs_input" && run.Status != "interrupted" && run.Status != "planner_failed" && run.Status != "waiting_camera_confirmation" && !cameraReplan {
			problem(w, 409, fmt.Errorf("此任务当前无法继续。"))
			return
		}
		run.Automation = cloneAutomation(run.Automation)
		if cameraReplan {
			run.Automation.Camera.StartedAt = nil
			run.Automation.Camera.Status = "invalidated"
		}
	}
	if err := s.prepareAutomation(&run, input); err != nil {
		problem(w, 422, err)
		return
	}
	run.Status = "running"
	run.Automation.Needs = nil
	run.Automation.InputNeeds = nil
	run.Limitations = nil
	run.Automation.mark("planner", "running", "识别任务目标和所需资料。")
	if err := s.saveAutomation(&run); err != nil {
		problem(w, 500, err)
		return
	}
	response := s.automationDetail(run)
	// Freeze the response before the worker mutates its private copy.
	response.Task.Automation = cloneAutomation(run.Automation)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 15*time.Minute)
	started = true
	go func() {
		defer func() { cancel(); <-s.cfg.Gate }()
		s.runAutomation(ctx, authorization, &run)
	}()
	respond(w, 202, response)
}

func cloneAutomation(state *Automation) *Automation {
	cloned := *state
	cloned.Steps, cloned.Needs = slices.Clone(state.Steps), slices.Clone(state.Needs)
	cloned.InputNeeds, cloned.Replies = slices.Clone(state.InputNeeds), slices.Clone(state.Replies)
	cloned.PlanningAttempts = slices.Clone(state.PlanningAttempts)
	cloned.Camera = cloneCameraTask(state.Camera)
	return &cloned
}

func (s *Service) saveAutomation(run *Run) error {
	run.Automation.UpdatedAt = time.Now().UTC()
	snapshot := *run
	snapshot.Automation = cloneAutomation(run.Automation)
	snapshot.Limitations = slices.Clone(run.Limitations)
	return s.store(snapshot)
}

func (s *Service) checkpoint(run *Run) bool {
	if err := s.saveAutomation(run); err != nil {
		log.Printf("workbench automation %s: persist state: %v", run.ID, err)
		return false
	}
	return true
}

func (a *Automation) step(role string) *AutomationStep {
	for i := range a.Steps {
		if a.Steps[i].Role == role {
			return &a.Steps[i]
		}
	}
	a.Steps = append(a.Steps, AutomationStep{Role: role, Status: "pending"})
	return &a.Steps[len(a.Steps)-1]
}

func (a *Automation) mark(role, status, detail string) {
	step := a.step(role)
	step.Status, step.Detail = status, detail
}

func (s *Service) prepareAutomation(run *Run, input automationInput) error {
	a := run.Automation
	if input.TimeZone != "" {
		if _, err := localTimeZone(input.TimeZone); err != nil {
			return err
		}
		a.TimeZone = input.TimeZone
	}
	goal := strings.TrimSpace(input.Goal)
	if run.Question == "" || (goal != "" && goal != run.Question) {
		if run.Question != "" && !slices.Contains(a.Needs, "goal") {
			return fmt.Errorf("更改任务目标需要新建任务。")
		}
		if n := len([]rune(goal)); n < 1 || n > 2000 {
			return fmt.Errorf("请填写 1–2000 字的任务目标。")
		}
		run.Question = goal
		a.GoalPlan, a.ExecutionMode = nil, ""
	}
	if input.Clarification != "" {
		if len([]rune(input.Clarification)) > 2000 || len(a.Replies) >= 30 {
			return fmt.Errorf("补充说明每次不超过2000字，每条任务最多补充30次。")
		}
		a.Replies = append(a.Replies, strings.TrimSpace(input.Clarification))
	}
	if input.ExecutionMode != "" {
		if input.ExecutionMode != "simulation" || !slices.Contains(a.Needs, "execution_mode") || a.GoalPlan == nil || a.GoalPlan.Execution == nil {
			return fmt.Errorf("当前任务没有可改为模拟的设备请求。")
		}
		a.ExecutionMode = "simulation"
	}
	if input.Query != nil && a.step("environment").Status != "completed" {
		if err := validateQuery(*input.Query); err != nil {
			return err
		}
		if input.Query.Mode == "platform" && (input.Query.ChannelID == "" || (input.Query.PublisherID == "" && input.Query.StationID == "")) {
			return fmt.Errorf("请选择平台设备和通道。")
		}
		if input.Compare != nil {
			q := *input.Query
			q.Start, q.End = input.Compare.Start, input.Compare.End
			if err := validateQuery(q); err != nil {
				return fmt.Errorf("对照时段无效：%w", err)
			}
			if q.Start.Equal(input.Query.Start) && q.End.Equal(input.Query.End) {
				return fmt.Errorf("请选择与主时段不同的对照时段。")
			}
		}
		a.Query, a.Compare = input.Query, input.Compare
		a.Observation = nil
		a.step("environment").Status = "pending"
	}
	if input.Compare != nil && input.Query == nil {
		return fmt.Errorf("对照时段需要主时段。")
	}
	if input.Image != "" && a.step("image").Status != "completed" {
		if input.ImageSource != "user_upload" && input.ImageSource != "public_dataset" {
			return fmt.Errorf("请选择上传图片的实际来源。")
		}
		if err := s.attachAutomationImage(run, input.Image, input.ImageSource, input.Crop); err != nil {
			return err
		}
		a.Photo = nil
	}
	if input.Duration != nil {
		if *input.Duration < 1 || *input.Duration > 300 {
			return fmt.Errorf("模拟开启时长须为 1–300 秒。")
		}
		a.Duration = input.Duration
	}
	// Probing an optional step must not make it appear in the task plan.
	a.Steps = slices.DeleteFunc(a.Steps, func(step AutomationStep) bool {
		return step.Status == "pending" && step.RunID == ""
	})
	return nil
}

func (s *Service) runAutomation(ctx context.Context, auth string, run *Run) {
	a := run.Automation
	if err := s.planAutomation(ctx, run); err != nil {
		run.Status = "planner_failed"
		if ctx.Err() != nil {
			run.Status = "interrupted"
		}
		a.mark("planner", "failed", err.Error())
		s.checkpoint(run)
		return
	}
	plan := a.GoalPlan
	active := map[string]bool{"planner": true, "environment": plan.Environment != nil, "image": plan.Image != nil, "simulation": plan.Execution != nil, "camera": plan.Camera != nil}
	active["materials"], active["review"], active["report"] = active["environment"] || active["image"], active["environment"] || active["image"], active["environment"] || active["image"]
	a.Steps = slices.DeleteFunc(a.Steps, func(step AutomationStep) bool { return !active[step.Role] })
	if plan.Clarification != "" {
		a.need("clarification", plan.Clarification)
	}
	if plan.Camera != nil && (a.Camera == nil || a.Camera.Status != "completed") {
		if len(a.Needs) > 0 {
			run.Status = "needs_input"
			s.checkpoint(run)
			return
		}
		a.mark("planner", "completed", "已确定所需能力及执行顺序。")
		s.prepareCamera(ctx, auth, run)
		return
	}
	// The semantic plan is checked against the registered executors, separately
	// from interpreting the user's words. An unavailable capability is explicit.
	action, duration, err := s.executionRequest(ctx, run)
	if err != nil {
		run.Status = "planner_failed"
		a.mark("planner", "failed", err.Error())
		s.checkpoint(run)
		return
	}
	if plan.Execution != nil {
		if action == "" {
			a.mark("simulation", "needs_input", "设备操作已理解，等待解决执行条件。")
		} else {
			a.mark("simulation", "pending", "准备核对模拟执行条件。")
		}
	}
	routes := struct{ Environment, Image bool }{plan.Environment != nil, plan.Image != nil}
	a.mark("planner", "completed", "已确定所需能力及执行顺序。")
	readyEnvironment, readyImage := routes.Environment, routes.Image
	if routes.Environment || routes.Image {
		a.mark("materials", "running", "按任务目标查找观测和已有照片。")
		if !s.checkpoint(run) {
			return
		}
	}
	if routes.Environment {
		a.step("environment")
		if a.Query == nil {
			query, compare, source, reason := s.autoObservation(run)
			if query != nil {
				a.Query, a.Compare, a.Observation = query, compare, source
				a.mark("environment", "pending", "已找到可用观测来源和时段。")
			} else {
				a.mark("environment", "needs_input", reason)
			}
		}
		if a.Query == nil || a.step("environment").Status == "needs_input" {
			a.need("window", a.step("environment").Detail)
			readyEnvironment = false
		}
		if a.Compare == nil && plan.Environment.ComparisonRequired {
			a.need("comparison", "该任务需要比较两个时段，请补充另一个观测时段。")
			readyEnvironment = false
		}
	}
	if routes.Image {
		a.step("image")
		if a.Image == nil {
			if reason := s.autoPhoto(ctx, auth, run); reason != "" {
				a.mark("image", "needs_input", reason)
			}
		}
		if a.Image == nil || (a.step("image").Status == "needs_input" && a.Image.ID == run.ImageHash) {
			a.need("image", a.step("image").Detail)
			readyImage = false
		}
	}
	if routes.Environment || routes.Image {
		a.mark("materials", "completed", "已核对可用资料，各分支按资料齐备情况运行。")
		if !s.checkpoint(run) {
			return
		}
	}
	// Traverse analysis dependencies selected by the planner. Missing inputs
	// pause only affected branches, leaving independent work able to finish.
	pending := []string{}
	if routes.Environment && readyEnvironment {
		pending = append(pending, "environment")
	}
	if routes.Image && readyImage {
		pending = append(pending, "image")
	}
	for len(pending) > 0 {
		progress := false
		for i, role := range pending {
			if a.step(role).Status == "completed" {
				pending = slices.Delete(pending, i, i+1)
				progress = true
				break
			}
			deps := []string{}
			if role == "environment" {
				deps = plan.Environment.DependsOn
			} else {
				deps = plan.Image.DependsOn
			}
			blocked := false
			for _, dep := range deps {
				if dep == "execution" {
					a.need("clarification", "任务要求设备操作后再分析。当前没有操作后重新采集数据的执行接口，请补充可用方案或调整流程。")
					blocked = true
					break
				}
				if a.step(dep).Status != "completed" {
					blocked = true
				}
			}
			if blocked {
				continue
			}
			input := Input{Task: role}
			if role == "environment" {
				input.Question, input.Query, input.Compare = plan.Environment.Instruction, a.Query, a.Compare
			} else {
				input.Question, input.Crop = plan.Image.Instruction, a.Crop
				raw, err := s.automationImage(run)
				if err != nil {
					a.mark("image", "needs_input", "图片不可读取，请重新上传。")
					a.need("image", "图片不可读取，请重新上传。")
				} else {
					input.Image, input.ImageSource, input.platformPhoto = raw, a.Image.Source, a.Image.Source == "platform_photo"
					input.sceneImage = plan.Camera != nil && plan.Camera.AnalysisKind == "scene"
				}
			}
			if role != "image" || input.Image != "" {
				if !s.runAutomationBranch(ctx, auth, run, input) {
					return
				}
			}
			pending = slices.Delete(pending, i, i+1)
			progress = true
			break
		}
		if !progress {
			for _, role := range pending {
				a.mark(role, "blocked", "等待前置步骤完成。")
			}
			break
		}
	}
	if routes.Environment && routes.Image {
		run.Limitations = append(run.Limitations, "图片与观测时段分别核查。照片的采集时间和地块关联需要单独确认。")
	}
	if routes.Environment || routes.Image {
		checked := 0
		for _, report := range s.automationDetail(*run).Reports {
			if (report.Status == "completed" || report.Status == "evidence_summary") && report.Workflow != nil && report.Workflow.Snapshot != "" {
				checked++
			}
		}
		status := "completed"
		if checked == 0 {
			status = "skipped"
		}
		a.mark("review", status, fmt.Sprintf("汇总范围内有 %d 份已核对的分支报告。", checked))
	}
	if len(a.Needs) > 0 {
		run.Status = "needs_input"
		if routes.Environment || routes.Image {
			a.mark("report", "partial", "保留已完成分支的结果，等待补齐资料。")
		}
		s.checkpoint(run)
		return
	}
	if ctx.Err() != nil {
		run.Status = "interrupted"
		s.checkpoint(run)
		return
	}
	run.Status = "completed"
	if slices.ContainsFunc(a.Steps, func(step AutomationStep) bool { return (step.Status == "failed" || step.Status == "blocked") }) {
		run.Status = "partial"
		if action != "" {
			a.mark("simulation", "skipped", "前置分析未完成，草案未生成。")
		}
	} else if action != "" {
		if s.cfg.CheckAutomationManage != nil {
			if err := s.cfg.CheckAutomationManage(ctx, run.WorkspaceID); err != nil {
				a.mark("simulation", "failed", "当前账号缺少生成模拟草案的工作区管理权限。")
				run.Status = "failed"
				s.checkpoint(run)
				return
			}
		}
		basis := ""
		if routes.Environment && a.step("environment").Status == "completed" {
			basis = a.step("environment").RunID
		}
		plan, err := s.simulationPlan(run.WorkspaceID, action, duration, basis)
		if err != nil {
			run.Status = "failed"
			a.mark("simulation", "failed", "无法生成满足能力条件的模拟草案。")
		} else {
			run.Plan, run.Workflow, run.Status = plan, executionWorkflow(), "waiting_confirmation"
			a.mark("simulation", "waiting_confirmation", "草案已生成，请核对目标、动作、时长与关闭步骤。")
		}
	}
	if routes.Environment || routes.Image {
		a.mark("report", "completed", "汇总各分支已经核对的报告与缺口。")
	}
	s.checkpoint(run)
}

func (s *Service) runAutomationBranch(ctx context.Context, auth string, run *Run, input Input) bool {
	a := run.Automation
	if a.Camera != nil {
		input.cameraSource = &a.Camera.Source
	}
	a.mark(input.Task, "running", "正在运行此分支。")
	if !s.checkpoint(run) {
		return false
	}
	report, err := s.run(ctx, run.WorkspaceID, auth, input)
	if err != nil {
		if ctx.Err() != nil {
			run.Status = "interrupted"
			a.mark(input.Task, "failed", "任务运行中断。")
			s.checkpoint(run)
			return false
		}
		a.mark(input.Task, "failed", prefixBytes(err.Error(), 360))
		return s.checkpoint(run)
	}
	a.step(input.Task).RunID = report.ID
	switch report.Status {
	case "completed", "evidence_summary":
		if report.Workflow == nil || report.Workflow.Snapshot == "" || report.WorkspaceID != run.WorkspaceID {
			a.mark(input.Task, "failed", "分支缺少可核对的证据快照。")
		} else {
			a.mark(input.Task, "completed", "已保存核对后的结果。")
		}
	case "needs_input":
		run.ImageHash = report.ImageHash
		detail := "需要补拍清晰叶片照片。"
		if input.sceneImage {
			detail = "当前画面缺少可辨识的内容，请提供能看清对象的照片。"
		} else if report.Workflow != nil && report.Workflow.Vision != nil && report.Workflow.Vision.Subject == "non_leaf" {
			detail = "已读取照片，但画面不是叶片，请补充要检查的叶片照片。"
		}
		a.mark(input.Task, "needs_input", detail)
		a.need("image", detail)
	case "data_insufficient":
		a.mark(input.Task, "needs_input", "所选时段缺少可用观测。")
		a.need("window", "所选时段缺少可用观测。")
	default:
		a.mark(input.Task, "failed", "分支未生成可发布结论，详见分支记录。")
	}
	return s.checkpoint(run)
}

func (s *Service) automationImage(run *Run) (string, error) {
	if run.automationImage != "" {
		return run.automationImage, nil
	}
	return s.readImageAsset(run.Automation.Image)
}

func (s *Service) attachAutomationImage(run *Run, encoded, source, crop string) error {
	asset := analysisTask{service: s, input: Input{Image: encoded, ImageSource: source, platformPhoto: source == "platform_photo"}}
	if _, _, err := asset.loadImage(context.Background()); err != nil {
		return err
	}
	run.Automation.Image, run.Automation.Crop = &asset.image, crop
	if s.cfg.StateDir == "" {
		run.automationImage = encoded
	}
	return nil
}

func (s *Service) readImageAsset(asset *imageAsset) (string, error) {
	hash, err := hex.DecodeString(asset.ID)
	if err != nil || len(hash) != 32 || (asset.Format != "png" && asset.Format != "jpeg") || s.cfg.StateDir == "" {
		return "", fmt.Errorf("invalid image reference")
	}
	b, err := os.ReadFile(filepath.Join(s.cfg.StateDir, "assets", asset.ID+"."+asset.Format))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
