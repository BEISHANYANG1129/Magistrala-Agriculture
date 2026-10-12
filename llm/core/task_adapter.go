package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// TaskPayload: 控制服务消费的任务载荷，符合 /control/task 接口。
type TaskPayload struct {
	TaskType string                 `json:"task_type"`
	Target   string                 `json:"target"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Source   string                 `json:"source,omitempty"`
}

// ControlAdapter: 不改动原 orchestrator 的前提下，负责
// 1) 拉取消息→喂给 LLM→解析区域命令→转换为 TaskPayload
// 2) 将 TaskPayload 发送到控制服务 /control/task
// 依赖已有的 FetchChannelMessages/ToLLMMessages/RegionCommand 等工具函数。
type ControlAdapter struct {
	// 数据来源
	BaseURL     string
	MessagePort int
	DomainID    string
	ChannelID   string
	Token       string
	MappingPath string // 分区映射文件路径（必填）

	// 控制服务入口，例如 http://localhost:8280
	ControlBase string

	// LLM 推理函数（必填）
	Infer InferFunc
}

// RunTasks: 拉取最近 limit 条消息，经 LLM 推理为区域命令，再转换为控制服务任务。
// 不做下发，仅返回任务列表。
func (a *ControlAdapter) RunTasks(limit int) ([]TaskPayload, error) {
	if a.Infer == nil {
		return nil, fmt.Errorf("Infer 为空")
	}
	if limit <= 0 {
		limit = 10
	}

	log.Printf("[Adapter] fetch messages limit=%d", limit)
	resp, err := FetchChannelMessages(a.BaseURL, a.MessagePort, a.DomainID, a.ChannelID, a.Token, 0, limit)
	if err != nil {
		return nil, err
	}

	msgs := ToLLMMessages(resp, a.DomainID, a.ChannelID, a.MappingPath)
	if len(msgs) > 0 {
		b, _ := json.Marshal(msgs[0])
		log.Printf("[Adapter] sample msg[0]=%s", trunc(string(b), 300))
	}

	raw, err := a.Infer(msgs)
	if err != nil {
		return nil, err
	}
	log.Printf("[Adapter] llm raw output=%s", trunc(raw, 800))

	regionCmds, err := parseRegionCommands(raw)
	if err != nil {
		return nil, err
	}
	log.Printf("[Adapter] region commands=%d", len(regionCmds))
	if len(regionCmds) == 0 {
		return nil, nil
	}

	tasks := regionCommandsToTasks(regionCmds)
	log.Printf("[Adapter] tasks=%d", len(tasks))
	return tasks, nil
}

// PostTask: 将单个任务发送到控制服务。
func (a *ControlAdapter) PostTask(task TaskPayload) error {
	if a.ControlBase == "" || task.TaskType == "" || task.Target == "" {
		return fmt.Errorf("control base/task_type/target 缺失")
	}
	body, _ := json.Marshal(task)
	url := fmt.Sprintf("%s/control/task", strings.TrimRight(a.ControlBase, "/"))
	log.Printf("[Adapter] POST %s payload=%s", url, string(body))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("控制服务返回 http=%d", resp.StatusCode)
	}
	return nil
}

// parseRegionCommands: 兼容数组、{commands:[...]}、单对象三种格式。
func parseRegionCommands(raw string) ([]RegionCommand, error) {
	var regionCmds []RegionCommand
	if err := json.Unmarshal([]byte(raw), &regionCmds); err != nil {
		var wrap struct {
			Commands []RegionCommand `json:"commands"`
		}
		if err2 := json.Unmarshal([]byte(raw), &wrap); err2 == nil {
			regionCmds = wrap.Commands
		} else {
			var single RegionCommand
			if err3 := json.Unmarshal([]byte(raw), &single); err3 == nil {
				regionCmds = []RegionCommand{single}
			} else {
				return nil, fmt.Errorf("解析区域命令失败: %v; raw=%s", err, trunc(raw, 200))
			}
		}
	}
	return regionCmds, nil
}

// regionCommandsToTasks: 将区域命令转换为控制服务任务载荷。
// 规则：
// - task_type 使用 action；
// - target 优先 partition_name，否则使用 partition_id；
// - params 附带 reason（若存在）；source 固定为 "llm"。
func regionCommandsToTasks(rcs []RegionCommand) []TaskPayload {
	tasks := make([]TaskPayload, 0, len(rcs))
	for _, rc := range rcs {
		target := rc.PartitionName
		if target == "" {
			target = rc.PartitionID
		}
		params := map[string]interface{}{}
		if rc.Reason != "" {
			params["reason"] = rc.Reason
		}
		tasks = append(tasks, TaskPayload{
			TaskType: rc.Action,
			Target:   target,
			Params:   params,
			Source:   "llm",
		})
	}
	return tasks
}
