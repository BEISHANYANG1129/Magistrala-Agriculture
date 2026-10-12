package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"agri-control-service/internal/executor"
	"agri-control-service/internal/logstore"
	"agri-control-service/internal/model"
	"agri-control-service/internal/planner"
	"agri-control-service/internal/policy"
)

// ControlService 负责控制主流程：入队、调度、策略校验、规划动作、串行执行。
// - 队列削峰：HandleTask 将任务放入内存队列，worker 异步处理
// - 调度：支持 schedule_at 定时启动，以及 wait 动作的非阻塞延时
// - 规划：调用 planner 依据 TaskType 生成动作序列
// - 策略：调用 policy 在执行前做参数校验/修正
// - 执行：调用 executor 下发设备命令，附带日志
type ControlService struct {
	executor *executor.Executor // 执行设备命令的执行器
	queue    chan *model.Task   // 任务队列，负责削峰和异步处理
}

const defaultWorkers = 4

// NewControlService 构造控制服务，初始化执行器与队列，并启动指定数量的 worker。
func NewControlService(store *logstore.LogStore, workers int) *ControlService {
	if workers <= 0 {
		workers = defaultWorkers
	}
	s := &ControlService{
		executor: executor.NewExecutor(store),
		queue:    make(chan *model.Task, workers*4), // 简单按 worker 数量放大队列容量
	}
	s.startWorkers(workers)
	return s
}

// HandleTask 校验/补全标识并尝试入队，队列满时返回错误。
func (s *ControlService) HandleTask(task *model.Task) error {
	ensureIdentifiers(task)

	select {
	case s.queue <- task:
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// startWorkers 启动 n 个后台 worker，从队列中取任务执行。
func (s *ControlService) startWorkers(n int) {
	for i := 0; i < n; i++ {
		go s.worker()
	}
}

// worker 从队列消费任务，串行执行单个任务的动作链。
func (s *ControlService) worker() {
	for task := range s.queue {
		s.processTask(task)
	}
}

// processTask 处理调度时间：若 schedule_at 在未来则设定定时器到点再执行。
func (s *ControlService) processTask(task *model.Task) {
	if task.ScheduleAt != "" {
		t, err := time.Parse(time.RFC3339, task.ScheduleAt)
		if err != nil {
			log.Printf("[trace=%s task=%s] invalid schedule_at: %v", task.TraceID, task.TaskID, err)
		} else {
			now := time.Now()
			if t.After(now) {
				d := time.Until(t)
				time.AfterFunc(d, func() {
					s.processPlannedTask(task)
				})
				return
			}
		}
	}
	s.processPlannedTask(task)
}

// processPlannedTask 在通过策略校验后生成动作并启动执行。
func (s *ControlService) processPlannedTask(task *model.Task) {
	if err := policy.ValidateTask(*task); err != nil {
		log.Printf("[trace=%s task=%s] policy reject: %v", task.TraceID, task.TaskID, err)
		return
	}

	actions, err := planner.PlanActions(*task)
	if err != nil {
		log.Printf("[trace=%s task=%s] plan failed: %v", task.TraceID, task.TaskID, err)
		return
	}

	s.runActions(task, actions, 0)
}

// runActions 顺序执行动作；wait 动作用定时器延迟，不阻塞 worker。
func (s *ControlService) runActions(task *model.Task, actions []model.Action, idx int) {
	if idx >= len(actions) {
		return
	}
	action := actions[idx]

	// wait 动作：用定时器延后执行后续动作，当前 worker 立即返回
	if action.ActionType == "wait" {
		d := executor.WaitDuration(action.Params)
		if d > 0 {
			time.AfterFunc(d, func() {
				s.runActions(task, actions, idx+1)
			})
			return
		}
		// 无有效等待时间则跳过
		s.runActions(task, actions, idx+1)
		return
	}

	// 非 wait 动作：立即执行设备命令
	cmd := model.DeviceCommand{
		DeviceID: task.Target,
		Command:  action.ActionType,
		Params:   action.Params,
		TaskID:   task.TaskID,
		TraceID:  task.TraceID,
	}
	if err := s.executor.Execute(cmd); err != nil {
		log.Printf("[trace=%s task=%s] execute failed: %v", task.TraceID, task.TaskID, err)
		return
	}

	s.runActions(task, actions, idx+1)
}

// ensureIdentifiers 保证任务/链路标识存在，便于追踪与日志关联。
func ensureIdentifiers(task *model.Task) {
	if task.TaskID == "" {
		task.TaskID = generateID()
	}
	if task.TraceID == "" {
		task.TraceID = task.TaskID
	}
}

// generateID 生成随机 task/trace ID（hex 编码）。
func generateID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Printf("generate id failed: %v", err)
		return "fallback-id"
	}
	return hex.EncodeToString(buf)
}
