package policy

import "agri-control-service/internal/model"

// policy 包：放置任务级策略校验/修正逻辑，用于在执行前约束或调整参数。

// ValidateTask: 对任务参数做策略检查/修正。
func ValidateTask(task model.Task) error {
	// 示例：灌溉时长限制，若超过 60 分钟则截断到 60。
	if task.TaskType == "irrigation" {
		if v, ok := task.Params["duration_min"].(float64); ok {
			if v > 60 {
				task.Params["duration_min"] = 60
			}
		}
	}
	return nil
}
