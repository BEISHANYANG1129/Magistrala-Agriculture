package executor

import (
	"fmt"
	"time"

	"agri-control-service/internal/logstore"
	"agri-control-service/internal/model"
)

// Executor 负责把规划好的 DeviceCommand 下发到设备层；支持可选日志落盘。
type Executor struct {
	store *logstore.LogStore
}

func NewExecutor(store *logstore.LogStore) *Executor {
	return &Executor{store: store}
}

// Execute 在当前实现中仅打印命令并写日志，预留对接真实设备。
func (e *Executor) Execute(cmd model.DeviceCommand) error {
	start := time.Now()
	fmt.Printf("[EXECUTE] trace=%s task=%s device=%s command=%s params=%v\n",
		cmd.TraceID, cmd.TaskID, cmd.DeviceID, cmd.Command, cmd.Params)

	status := "ok"
	var errMsg string

	// TODO: 在此对接真实设备调用，并设置 status/errMsg

	elapsed := time.Since(start).Milliseconds()

	if e.store != nil {
		_ = e.store.Append(logstore.LogEntry{
			TaskID:    cmd.TaskID,
			TraceID:   cmd.TraceID,
			DeviceID:  cmd.DeviceID,
			Command:   cmd.Command,
			Params:    cmd.Params,
			Status:    status,
			Error:     errMsg,
			ElapsedMs: elapsed,
		})
	}

	return nil
}

// WaitDuration 从参数中解析常见的延迟字段（毫秒/秒/分钟）。
func WaitDuration(params map[string]interface{}) time.Duration {
	if params == nil {
		return 0
	}
	if v, ok := params["duration_ms"]; ok {
		switch x := v.(type) {
		case float64:
			return time.Duration(x) * time.Millisecond
		case int:
			return time.Duration(x) * time.Millisecond
		}
	}
	if v, ok := params["duration_sec"]; ok {
		switch x := v.(type) {
		case float64:
			return time.Duration(x) * time.Second
		case int:
			return time.Duration(x) * time.Second
		}
	}
	if v, ok := params["duration_min"]; ok {
		switch x := v.(type) {
		case float64:
			return time.Duration(x * float64(time.Minute))
		case int:
			return time.Duration(x) * time.Minute
		}
	}
	return 0
}
