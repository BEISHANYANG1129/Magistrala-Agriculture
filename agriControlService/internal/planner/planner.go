package planner

import (
	"errors"

	"agri-control-service/internal/model"
	"agri-control-service/internal/registry"
)

// planner 包：根据任务类型从注册表获取动作序列，并把任务参数注入到每个动作。
// 主要职责是“计划怎么做”，不关心设备执行细节。
func PlanActions(task model.Task) ([]model.Action, error) {
	// 按 task_type 从注册表查找对应的动作链
	actions, ok := registry.TaskActionRegistry[task.TaskType]
	if !ok {
		return nil, errors.New("unknown task type")
	}

	// 将 Task 的动态参数透传到每个 Action，便于后续执行使用
	for i := range actions {
		actions[i].Params = task.Params
	}

	return actions, nil
}
