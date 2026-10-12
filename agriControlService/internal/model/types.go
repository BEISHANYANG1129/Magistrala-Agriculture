package model

// 任务与执行相关的数据结构定义。
type Task struct {
	TaskID     string                 `json:"task_id,omitempty" yaml:"task_id,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
	ScheduleAt string                 `json:"schedule_at,omitempty" yaml:"schedule_at,omitempty"`
	TaskType   string                 `json:"task_type" yaml:"task_type"`
	Target     string                 `json:"target" yaml:"target"`
	Params     map[string]interface{} `json:"params" yaml:"params"`
	Source     string                 `json:"source" yaml:"source"`
}

// Action 描述 planner 规划出的单个动作。
type Action struct {
	ActionType string                 `json:"action_type" yaml:"action_type"`
	DeviceType string                 `json:"device_type" yaml:"device_type"`
	Params     map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

// DeviceCommand 是 executor 可直接下发的设备指令。
type DeviceCommand struct {
	DeviceID string                 `json:"device_id"`
	Command  string                 `json:"command"`
	Params   map[string]interface{} `json:"params"`
	TaskID   string                 `json:"task_id"`
	TraceID  string                 `json:"trace_id"`
}
