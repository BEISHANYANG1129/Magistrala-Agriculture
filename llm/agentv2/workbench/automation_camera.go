package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// CameraSource contains platform object IDs only. Device credentials stay in the
// existing encrypted bridge profile, outside model inputs and task history.
var cameraUUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var cameraGeneration = regexp.MustCompile(`^g-[a-f0-9]{32}$`)

type CameraSource struct {
	WorkspaceID string   `json:"workspace_id"`
	DeviceID    string   `json:"device_id"`
	Name        string   `json:"name"`
	BindingID   string   `json:"binding_id"`
	Aliases     []string `json:"aliases,omitempty"`
}

func LoadCameraSources(path string) ([]CameraSource, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sources []CameraSource
	if err = json.Unmarshal(b, &sources); err != nil {
		return nil, err
	}
	for _, c := range sources {
		if !cameraUUID.MatchString(c.WorkspaceID) || !cameraUUID.MatchString(c.DeviceID) || c.BindingID != "jooan-g4s4" || c.Name == "" {
			return nil, fmt.Errorf("invalid registered camera")
		}
	}
	return sources, nil
}

type CameraOperation struct {
	Action      string `json:"action"`
	Direction   string `json:"direction"`
	DurationMs  int    `json:"duration_ms"`
	View        int    `json:"view"` // User-facing view numbers 1..3; only view 1 moves.
	WaitSeconds int    `json:"wait_seconds"`
}
type CameraRequest struct {
	Intent       *CameraIntent     `json:"original_intent,omitempty"`
	DeviceID     string            `json:"device_id"`
	Target       string            `json:"target"`
	Quote        string            `json:"quote"`
	Operations   []CameraOperation `json:"operations"`
	Analyze      bool              `json:"analyze"`
	AnalysisKind string            `json:"analysis_kind"`
	Instruction  string            `json:"instruction"`
	Unsupported  string            `json:"unsupported"`
	DependsOn    []string          `json:"depends_on"`
}
type CameraReceipt struct {
	Index            int    `json:"index"`
	RequestID        string `json:"request_id"`
	Status           string `json:"status"`
	Detail           string `json:"detail,omitempty"`
	PositionVerified bool   `json:"position_verified"`
}
type CameraCapture struct {
	ID         string     `json:"id"`
	Asset      imageAsset `json:"asset"`
	View       int        `json:"view"`
	Generation string     `json:"generation"`
	ReceivedAt time.Time  `json:"received_at"`
	Segment    int        `json:"segment"`
	ReportID   string     `json:"report_id,omitempty"`
	raw        string
}
type CameraTask struct {
	Source     CameraSource    `json:"source"`
	Request    CameraRequest   `json:"request"`
	Revision   string          `json:"revision"`
	Generation string          `json:"generation"`
	ExpiresAt  time.Time       `json:"expires_at"`
	Status     string          `json:"status"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	ApprovedBy string          `json:"approved_by,omitempty"`
	Receipts   []CameraReceipt `json:"receipts"`
	Captures   []CameraCapture `json:"captures"`
}

func cloneCameraTask(c *CameraTask) *CameraTask {
	if c == nil {
		return nil
	}
	n := *c
	n.Source.Aliases = slices.Clone(c.Source.Aliases)
	n.Request.Operations = slices.Clone(c.Request.Operations)
	n.Request.DependsOn = slices.Clone(c.Request.DependsOn)
	if c.Request.Intent != nil {
		intent := *c.Request.Intent
		intent.Steps = slices.Clone(intent.Steps)
		intent.DependsOn = slices.Clone(intent.DependsOn)
		for i := range intent.Steps {
			if value := intent.Steps[i].Amount; value != nil {
				copy := *value
				intent.Steps[i].Amount = &copy
			}
			if view := intent.Steps[i].View; view != nil {
				copy := *view
				intent.Steps[i].View = &copy
			}
		}
		n.Request.Intent = &intent
	}
	n.Receipts = slices.Clone(c.Receipts)
	n.Captures = slices.Clone(c.Captures)
	return &n
}
func (s *Service) cameras(workspace string) []CameraSource {
	out := []CameraSource{}
	for _, c := range s.cfg.CameraSources {
		if c.WorkspaceID == workspace {
			out = append(out, c)
		}
	}
	return out
}
func (s *Service) cameraSource(workspace, device string) (CameraSource, error) {
	for _, c := range s.cameras(workspace) {
		if c.DeviceID == device {
			return c, nil
		}
	}
	return CameraSource{}, fmt.Errorf("当前工作区没有与请求匹配的已接入摄像头，请明确设备；本轮仅接入乔安 G4S4")
}
func validateCameraRequest(run *Run, p *CameraRequest) error {
	if p.AnalysisKind != "scene" && p.AnalysisKind != "leaf" {
		return fmt.Errorf("invalid camera analysis kind")
	}
	grounded := false
	for _, text := range append([]string{run.Question}, run.Automation.Replies...) {
		if strings.TrimSpace(p.Quote) != "" && strings.Contains(text, p.Quote) {
			grounded = true
		}
		if !grounded {
			if original, ok := alignCameraQuote(text, p.Quote); ok {
				p.Quote, grounded = original, true
			}
		}
	}
	if !grounded {
		return fmt.Errorf("camera quote must occur in user goal or replies")
	}
	if p.Unsupported != "" {
		return nil
	}
	if len(p.Operations) < 1 || len(p.Operations) > 8 {
		return fmt.Errorf("摄像头计划必须有1至8个有限步骤，不得省略多余动作或循环")
	}
	moves, captures, wait := 0, 0, 0
	for _, op := range p.Operations {
		switch op.Action {
		case "move":
			if !slices.Contains([]string{"left", "right", "up", "down"}, op.Direction) || op.DurationMs < 100 || op.DurationMs > 1000 || op.View != 1 || op.WaitSeconds != 0 {
				return fmt.Errorf("只有画面1可转动，四向单次100至1000毫秒")
			}
			moves += op.DurationMs
		case "snapshot":
			if op.View < 1 || op.View > 3 || op.Direction != "" || op.DurationMs != 0 || op.WaitSeconds != 0 {
				return fmt.Errorf("截图画面必须为1至3")
			}
			captures++
		case "stop":
			if op.Direction != "" || op.DurationMs != 0 || op.WaitSeconds != 0 || op.View != 1 {
				return fmt.Errorf("invalid camera stop")
			}
		case "wait":
			if op.WaitSeconds < 1 || op.WaitSeconds > 10 || op.Direction != "" || op.DurationMs != 0 || op.View != 1 {
				return fmt.Errorf("单次等待必须为1至10秒")
			}
			wait += op.WaitSeconds
		default:
			return fmt.Errorf("unsupported camera action")
		}
	}
	if moves > 3000 || captures > 3 || wait > 30 {
		return fmt.Errorf("本轮总转动最多3000毫秒，截图最多3张，总等待最多30秒；请缩短计划，不得静默截断")
	}
	if p.Analyze && (captures == 0 || strings.TrimSpace(p.Instruction) == "") {
		return fmt.Errorf("camera analysis requires a snapshot and explicit instruction")
	}
	return nil
}

// Restore the exact source span if the model only changed spacing (for example
// 画面1 -> 画面 1). All non-whitespace characters, numbers and negations must
// still match; the persisted quote always comes from the user's original text.
func alignCameraQuote(source, quote string) (string, bool) {
	needle := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, quote)
	if needle == "" {
		return "", false
	}
	var compact strings.Builder
	positions := []int{}
	for at, r := range source {
		if unicode.IsSpace(r) {
			continue
		}
		token := string(r)
		compact.WriteString(token)
		for offset := range len(token) {
			positions = append(positions, at+offset)
		}
	}
	at := strings.Index(compact.String(), needle)
	if at < 0 {
		return "", false
	}
	return source[positions[at] : positions[at+len(needle)-1]+1], true
}
func (s *Service) checkCamera(ctx context.Context, workspace string, source CameraSource, write bool) error {
	if s.cfg.CheckCamera == nil {
		return fmt.Errorf("摄像头平台权限校验尚未接入")
	}
	return s.cfg.CheckCamera(ctx, workspace, source, write)
}

func (s *Service) visibleCameraRuns(ctx context.Context, workspace string, rows []Run) []Run {
	decisions := map[string]bool{}
	return slices.DeleteFunc(rows, func(run Run) bool {
		source := run.CameraSource
		if run.Automation != nil && run.Automation.Camera != nil {
			source = &run.Automation.Camera.Source
		}
		if source == nil {
			return false
		}
		key := source.DeviceID + ":" + source.BindingID
		allowed, checked := decisions[key]
		if !checked {
			allowed = s.checkCamera(ctx, workspace, *source, false) == nil
			decisions[key] = allowed
		}
		return !allowed
	})
}
func (s *Service) cameraCall(ctx context.Context, auth string, c CameraSource, tail string, body any, out any) error {
	base, err := url.Parse(s.cfg.CameraGatewayURL)
	if err != nil || base.Scheme != "http" || (base.Hostname() != "127.0.0.1" && base.Hostname() != "localhost") || base.User != nil || base.Path != "" {
		return fmt.Errorf("camera gateway must be a configured loopback URL")
	}
	method := "GET"
	var raw []byte
	if body != nil {
		method = "POST"
		raw, _ = json.Marshal(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base.String()+"/api/cameras/"+c.WorkspaceID+"/"+c.DeviceID+"/"+tail, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("摄像头接口未返回确定结果：%w", err)
	}
	defer response.Body.Close()
	b, err := io.ReadAll(io.LimitReader(response.Body, (3<<20)+1))
	if err != nil || len(b) > 3<<20 {
		return fmt.Errorf("invalid camera response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var msg struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &msg)
		return fmt.Errorf("摄像头接口拒绝或未完成操作（%d）：%s", response.StatusCode, prefixBytes(msg.Message, 300))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

type cameraStatus struct {
	State      string `json:"state"`
	Enabled    bool   `json:"enabled"`
	Generation string `json:"generation"`
}

func (s *Service) prepareCamera(ctx context.Context, auth string, run *Run) {
	a := run.Automation
	p := a.GoalPlan.Camera
	fail := func(message string) {
		a.need("camera", message)
		a.mark("camera", "needs_input", message)
		run.Status = "needs_input"
		s.checkpoint(run)
	}
	if a.Camera != nil && a.Camera.StartedAt != nil {
		fail("摄像头动作已发出，当前结果为" + a.Camera.Status + "，不会自动重放；请查看已保存回执并新建后续任务。")
		return
	}
	if p.Unsupported != "" {
		fail(p.Unsupported)
		return
	}
	if len(p.DependsOn) > 0 {
		fail("当前摄像头执行计划须能独立确定，暂不根据分析结果自动改变转动方向或触发条件。请明确有限步骤。")
		return
	}
	source, err := s.cameraSource(run.WorkspaceID, p.DeviceID)
	if err != nil {
		fail(err.Error())
		return
	}
	if err = s.checkCamera(ctx, run.WorkspaceID, source, cameraWrites(p)); err != nil {
		fail("当前账号无法访问或控制这台摄像头。")
		return
	}
	var status cameraStatus
	var capabilities struct {
		ServerSnapshot bool `json:"serverSnapshot"`
		Autonomous     bool `json:"autonomousModelControl"`
	}
	if err = s.cameraCall(ctx, auth, source, "status", nil, &status); err != nil {
		fail(err.Error())
		return
	}
	if !status.Enabled || status.State != "ready" || !cameraGeneration.MatchString(status.Generation) {
		fail("摄像头视频尚未就绪。请在原设备视频页确认播放开关和网络连接，再补充说明继续；任务不会擅自打开已停止的播放。")
		return
	}
	if err = s.cameraCall(ctx, auth, source, "capabilities", nil, &capabilities); err != nil || !capabilities.ServerSnapshot || !capabilities.Autonomous {
		fail("当前摄像头桥接版本尚不支持任务执行或服务端截图。")
		return
	}
	c := &CameraTask{Source: source, Request: *p, Generation: status.Generation, ExpiresAt: time.Now().UTC().Add(10 * time.Minute), Status: "waiting_confirmation", Receipts: []CameraReceipt{}, Captures: []CameraCapture{}}
	c.Revision = fingerprint(struct {
		Task, Generation string
		Request          CameraRequest
		Source           CameraSource
		Expires          time.Time
	}{run.ID, c.Generation, c.Request, c.Source, c.ExpiresAt})
	a.Camera = c
	a.mark("camera", "waiting_confirmation", "已生成整条有限步骤计划，等待一次确认。转动为真实设备操作，截图等待新画面。")
	run.Status = "waiting_camera_confirmation"
	s.checkpoint(run)
}
func cameraWrites(p *CameraRequest) bool {
	return slices.ContainsFunc(p.Operations, func(op CameraOperation) bool { return op.Action == "move" || op.Action == "stop" })
}

func cameraUndispatched(c *CameraTask) bool {
	return c != nil && len(c.Receipts) == 0 && len(c.Captures) == 0
}

func (s *Service) handleCameraAutomation(w http.ResponseWriter, r *http.Request, path, workspace, auth string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/automation/"), "/")
	if len(parts) < 3 || parts[1] != "camera" {
		return false
	}
	run, ok := s.automationRun(workspace, parts[0])
	if !ok || run.Automation.Camera == nil {
		problem(w, 404, fmt.Errorf("camera task unavailable"))
		return true
	}
	run.Automation = cloneAutomation(run.Automation)
	c := run.Automation.Camera
	if err := s.checkCamera(r.Context(), workspace, c.Source, r.Method == "POST" && cameraWrites(&c.Request)); err != nil {
		problem(w, 403, fmt.Errorf("当前账号无权访问或控制这台摄像头"))
		return true
	}
	if r.Method == "GET" && len(parts) == 4 && parts[2] == "captures" {
		for _, capture := range c.Captures {
			if capture.ID == parts[3] {
				raw := capture.raw
				var err error
				if raw == "" {
					raw, err = s.readImageAsset(&capture.Asset)
				}
				if err != nil {
					problem(w, 404, fmt.Errorf("截图文件不可读取"))
				} else {
					respond(w, 200, map[string]any{"image": raw, "mime": "image/" + capture.Asset.Format})
				}
				return true
			}
		}
		problem(w, 404, fmt.Errorf("capture unavailable"))
		return true
	}
	if r.Method != "POST" || len(parts) != 3 {
		problem(w, 404, fmt.Errorf("camera task endpoint unavailable"))
		return true
	}
	switch parts[2] {
	case "stop":
		// Permission is checked even for a screenshot-only task: this endpoint can
		// send a real stop. It bypasses the shared inference/task gate.
		if err := s.checkCamera(r.Context(), workspace, c.Source, true); err != nil {
			problem(w, 403, err)
			return true
		}
		s.cameraMu.Lock()
		cancel := s.cameraJobs[run.ID]
		stopTaskID := run.ID
		// A stop from another saved task for this device must also cancel the
		// currently active plan, so it cannot start a later movement.
		for jobID, jobCancel := range s.cameraJobs {
			job, ok := s.automationRun(workspace, jobID)
			if ok && job.Automation.Camera != nil && job.Automation.Camera.Source.DeviceID == c.Source.DeviceID {
				cancel = jobCancel
				stopTaskID = jobID
				break
			}
		}
		if cancel != nil {
			cancel()
		}
		if cancel == nil {
			fresh, _ := s.automationRun(workspace, run.ID)
			if fresh.Status == "waiting_camera_confirmation" {
				fresh.Automation = cloneAutomation(fresh.Automation)
				fresh.Automation.Camera.Status = "stopped"
				fresh.Status = "camera_stopped"
				fresh.Automation.mark("camera", "stopped", "任务已停止，不会执行计划。")
				s.checkpoint(&fresh)
			}
		}
		s.cameraMu.Unlock()
		ctx, done := context.WithTimeout(context.WithoutCancel(r.Context()), 12*time.Second)
		defer done()
		err := s.cameraCall(ctx, auth, c.Source, "ptz/stop", map[string]any{"requestId": id(), "taskId": stopTaskID}, nil)
		if err != nil {
			problem(w, 502, fmt.Errorf("已取消后续步骤，但停止回执未确认：%w", err))
		} else {
			respond(w, 200, map[string]any{"status": "stop_sent", "position_verified": false})
		}
		return true
	case "confirm":
		var input struct {
			Revision  string `json:"revision"`
			Confirmed bool   `json:"confirmed"`
		}
		if err := readJSON(w, r, &input); err != nil {
			problem(w, 400, err)
			return true
		}
		if !input.Confirmed || input.Revision != c.Revision {
			problem(w, 409, fmt.Errorf("请核对并确认当前整条摄像头计划"))
			return true
		}
		if c.StartedAt != nil {
			respond(w, 200, s.automationDetail(run))
			return true
		}
		if run.Status != "waiting_camera_confirmation" || c.Status != "waiting_confirmation" || time.Now().After(c.ExpiresAt) {
			problem(w, 409, fmt.Errorf("此计划已失效，请补充说明重新规划"))
			return true
		}
		if run.Automation.PlanningVersion != goalPlanningVersion || c.Request.Intent == nil {
			message := "摄像头计划的能力校验已更新，旧计划未执行。请继续本任务重新准备并确认计划。"
			c.Status = "invalidated"
			run.Status = "needs_input"
			run.Automation.need("camera", message)
			run.Automation.mark("camera", "needs_input", message)
			if err := s.saveAutomation(&run); err != nil {
				problem(w, 500, err)
			} else {
				problem(w, 409, fmt.Errorf("%s", message))
			}
			return true
		}
		select {
		case s.cfg.Gate <- struct{}{}:
		default:
			problem(w, 429, fmt.Errorf("当前有任务运行，请稍后确认"))
			return true
		}
		s.cameraMu.Lock()
		// Re-read under both locks: stop and duplicate confirmation are serialized.
		fresh, _ := s.automationRun(workspace, run.ID)
		if fresh.Automation.Camera.StartedAt != nil || fresh.Status != "waiting_camera_confirmation" || fresh.Automation.Camera.Revision != c.Revision {
			s.cameraMu.Unlock()
			<-s.cfg.Gate
			respond(w, 200, s.automationDetail(fresh))
			return true
		}
		now := time.Now().UTC()
		c.StartedAt = &now
		c.ApprovedBy = s.subject(r.Context())
		c.Status = "running"
		run.Status = "camera_running"
		run.Automation.mark("camera", "running", "已确认整条计划，开始自动执行。")
		if err := s.saveAutomation(&run); err != nil {
			s.cameraMu.Unlock()
			<-s.cfg.Gate
			problem(w, 500, err)
			return true
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
		s.cameraJobs[run.ID] = cancel
		s.cameraMu.Unlock()
		response := s.automationDetail(run)
		response.Task.Automation = cloneAutomation(run.Automation)
		go func() {
			defer func() { cancel(); s.cameraMu.Lock(); delete(s.cameraJobs, run.ID); s.cameraMu.Unlock(); <-s.cfg.Gate }()
			s.executeCamera(ctx, auth, &run)
		}()
		respond(w, 202, response)
		return true
	default:
		problem(w, 404, fmt.Errorf("camera action unavailable"))
		return true
	}
}

func (s *Service) executeCamera(ctx context.Context, auth string, run *Run) {
	a, c := run.Automation, run.Automation.Camera
	fail := func(err error) {
		c.Status = "unknown"
		run.Status = "camera_unknown"
		if ctx.Err() != nil {
			c.Status = "stopped"
			run.Status = "camera_stopped"
		} else if cameraUndispatched(c) {
			// Every step persists a dispatch intent BEFORE calling a tool. With
			// no receipts, only read-only preflight happened; a fresh confirmed
			// plan is safe, while any dispatched command remains non-retryable.
			c.Status, c.StartedAt = "invalidated", nil
			run.Status = "needs_input"
			message := "尚未发送摄像头操作，原计划已失效。" + prefixBytes(err.Error(), 300) + "。可继续本任务以重新准备计划，核对后再次确认。"
			a.need("camera", message)
			a.mark("camera", "needs_input", message)
			s.checkpoint(run)
			return
		}
		a.mark("camera", c.Status, "已停止后续步骤，不会自动重试动作。"+prefixBytes(err.Error(), 420))
		s.checkpoint(run)
	}
	defer func() {
		if ctx.Err() != nil && run.Status != "camera_stopped" {
			fail(ctx.Err())
		}
	}()
	var status cameraStatus
	if err := s.cameraCall(ctx, auth, c.Source, "status", nil, &status); err != nil {
		fail(err)
		return
	}
	if !status.Enabled || status.State != "ready" || status.Generation != c.Generation {
		fail(fmt.Errorf("视频已停止或重连，原确认计划不再适用"))
		return
	}
	lastMove := ""
	for i, op := range c.Request.Operations {
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		if err := s.checkCamera(ctx, run.WorkspaceID, c.Source, op.Action == "move" || op.Action == "stop"); err != nil {
			fail(fmt.Errorf("摄像头权限或平台绑定已变化"))
			return
		}
		requestID := id()
		c.Receipts = append(c.Receipts, CameraReceipt{Index: i, RequestID: requestID, Status: "dispatching"})
		if !s.checkpoint(run) {
			return
		}
		var err error
		switch op.Action {
		case "move", "stop":
			command := map[string]any{"requestId": requestID, "taskId": run.ID}
			if op.Action == "stop" {
				delete(command, "taskId")
			}
			if op.Action == "move" {
				command["direction"], command["durationMs"] = op.Direction, op.DurationMs
			}
			var result struct {
				Start    *int `json:"startResult"`
				Stop     *int `json:"stopResult"`
				Position bool `json:"positionVerified"`
			}
			err = s.cameraCall(ctx, auth, c.Source, "ptz/"+op.Action, command, &result)
			if err == nil && (result.Start == nil || result.Stop == nil || *result.Start < 0 || *result.Stop < 0) {
				err = fmt.Errorf("SDK未确认操作或停止")
			}
			if err == nil {
				lastMove = requestID
				err = waitCamera(ctx, 1600*time.Millisecond)
			}
		case "wait":
			err = waitCamera(ctx, time.Duration(op.WaitSeconds)*time.Second)
		case "snapshot":
			command := map[string]any{"requestId": requestID, "taskId": run.ID, "generation": c.Generation, "view": op.View - 1}
			if lastMove != "" {
				command["afterRequestId"] = lastMove
			}
			var result struct {
				Image      string    `json:"image"`
				Generation string    `json:"generation"`
				View       int       `json:"view"`
				ReceivedAt time.Time `json:"receivedAt"`
				Segment    int       `json:"segment"`
			}
			err = s.cameraCall(ctx, auth, c.Source, "snapshots", command, &result)
			if err == nil && (result.Generation != c.Generation || result.View != op.View-1 || result.ReceivedAt.IsZero()) {
				err = fmt.Errorf("截图来源元数据不匹配")
			}
			if err == nil {
				asset := analysisTask{service: s, input: Input{Image: result.Image, ImageSource: "platform_photo", platformPhoto: true}}
				_, _, err = asset.loadImage(ctx)
				if err == nil {
					capture := CameraCapture{ID: requestID, Asset: asset.image, View: op.View, Generation: c.Generation, ReceivedAt: result.ReceivedAt, Segment: result.Segment}
					if s.cfg.StateDir == "" {
						capture.raw = result.Image
					}
					c.Captures = append(c.Captures, capture)
				}
			}
		}
		receipt := &c.Receipts[len(c.Receipts)-1]
		if err != nil {
			receipt.Status = "unknown"
			receipt.Detail = prefixBytes(err.Error(), 420)
			fail(err)
			return
		}
		receipt.Status = "completed"
		receipt.Detail = "步骤已返回回执；没有角度或位置反馈。"
		if !s.checkpoint(run) {
			return
		}
	}
	if err := ctx.Err(); err != nil {
		fail(err)
		return
	}
	c.Status = "completed"
	a.mark("camera", "completed", fmt.Sprintf("有限步骤计划已完成，保存%d张新截图；SDK回执不表示已到达某个角度。", len(c.Captures)))
	if !s.checkpoint(run) {
		return
	}
	if c.Request.Analyze {
		for i := range c.Captures {
			if ctx.Err() != nil {
				fail(ctx.Err())
				return
			}
			capture := &c.Captures[i]
			raw := capture.raw
			var err error
			if raw == "" {
				raw, err = s.readImageAsset(&capture.Asset)
			}
			if err != nil {
				fail(err)
				return
			}
			a.Image = &capture.Asset
			a.Photo = &MaterialReference{Origin: "camera_capture", Name: c.Source.Name, PublisherID: c.Source.DeviceID, RecordID: capture.ID}
			run.automationImage = raw
			a.mark("image", "pending", "使用本任务的新截图进行分析。")
			if !s.runAutomationBranch(ctx, auth, run, Input{Task: "image", Question: c.Request.Instruction, Image: raw, ImageSource: "platform_photo", platformPhoto: true, sceneImage: c.Request.AnalysisKind == "scene"}) {
				return
			}
			capture.ReportID = a.step("image").RunID
		}
	}
	// Continue environment analysis / valve simulation preparation under the same
	// parent task. Completed camera steps are never replayed on analysis resume.
	s.runAutomation(ctx, auth, run)
}
func waitCamera(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
