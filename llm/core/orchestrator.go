package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// InferFunc 是可插拔的大模型调用函数。
// 输入为 BuildOllamaMessages 所需的 messages（任意结构数组），输出应为 JSON 字符串的指令数组。
type InferFunc func(messages []map[string]interface{}) (string, error)

// ExecCommand 统一的执行指令结构。
type ExecCommand struct {
	ClientId string `json:"clientId"`
	Action   string `json:"action"` // "open"|"close"
	Reason   string `json:"reason,omitempty"`
}

// 区域级命令（LLM输出）
// 这里的“区域”指分区（partition）；channel 是域下的小区域。
type RegionCommand struct {
	PartitionID   string `json:"partition_id,omitempty"`
	PartitionName string `json:"partition_name,omitempty"`
	Action        string `json:"action"`
	Reason        string `json:"reason,omitempty"`
}

// Registry 基本数据结构：域 -> 通道 -> 分区
type PartitionEntry struct {
	PartitionID   string   `json:"partitionId"`
	PartitionName string   `json:"partitionName"`
	Sensors       []string `json:"sensors,omitempty"`
	Executors     []string `json:"executors,omitempty"`
}

type ChannelEntry struct {
	ChannelID  string           `json:"channelId"`
	Partitions []PartitionEntry `json:"partitions"`
}

type DomainEntry struct {
	DomainID string         `json:"domainId"`
	Channels []ChannelEntry `json:"channels"`
}

// Orchestrator 负责：从 Magistrala 拉取 → 组装 → 调 LLM → 解析指令 → 下发到执行模块。
type Orchestrator struct {
	BaseURL     string // 不带端口，例如 http://localhost
	MessagePort int    // 消息查询服务端口
	DomainID    string
	ChannelID   string
	Token       string // Bearer 用户令牌

	ExecutorBase string // 执行模块地址，例如 http://127.0.0.1:8090（控制服务模式下可为空）
	// 映射文件路径（用于区域→clientId 或分区补全的映射）。必须配置。
	MappingPath string

	Infer InferFunc
}

// RunOnce 拉取最近 N 条消息，推理并返回指令集合（不做下发）。
func (o *Orchestrator) RunOnce(limit int) ([]ExecCommand, error) {
	log.Printf("[RunOnce] start limit=%d", limit)
	resp, err := FetchChannelMessages(o.BaseURL, o.MessagePort, o.DomainID, o.ChannelID, o.Token, 0, limit)
	if err != nil {
		log.Printf("[RunOnce] FetchChannelMessages error: %v", err)
		return nil, err
	}
	log.Printf("[RunOnce] fetched messages=%d", len(resp.Messages))

	msgs := ToLLMMessages(resp, o.DomainID, o.ChannelID, o.MappingPath)
	log.Printf("[RunOnce] llm input messages=%d", len(msgs))
	if len(msgs) > 0 {
		b, _ := json.Marshal(msgs[0])
		log.Printf("[RunOnce] sample msg[0]=%s", trunc(string(b), 300))
	}

	raw, err := o.Infer(msgs)
	if err != nil {
		log.Printf("[RunOnce] Infer error: %v", err)
		return nil, err
	}
	log.Printf("[RunOnce] llm raw output=%s", trunc(raw, 800))

	// 先解析为区域级命令
	var regionCmds []RegionCommand
	if err := json.Unmarshal([]byte(raw), &regionCmds); err != nil {
		// 尝试 {commands:[...]}
		var wrap struct {
			Commands []RegionCommand `json:"commands"`
		}
		if err2 := json.Unmarshal([]byte(raw), &wrap); err2 == nil {
			regionCmds = wrap.Commands
		} else {
			// 兼容“单对象”
			var single RegionCommand
			if err3 := json.Unmarshal([]byte(raw), &single); err3 == nil {
				regionCmds = []RegionCommand{single}
			} else {
				return nil, fmt.Errorf("解析区域命令失败: %v; raw=%s", err, trunc(raw, 200))
			}
		}
	}
	log.Printf("[RunOnce] region commands parsed=%d", len(regionCmds))
	if len(regionCmds) == 0 {
		log.Printf("[RunOnce] no region commands, return empty")
		return nil, nil
	}

	// 区域→具体 clientId 映射
	cmds, err := o.ResolveRegionCommands(regionCmds)
	if err != nil {
		log.Printf("[RunOnce] ResolveRegionCommands error: %v", err)
		return nil, err
	}
	log.Printf("[RunOnce] mapped exec commands=%d", len(cmds))
	return cmds, nil
}

// SendToExecutor 下发到执行模块。
func (o *Orchestrator) SendToExecutor(cmd ExecCommand) error {
	if o.ExecutorBase == "" || cmd.ClientId == "" || cmd.Action == "" {
		return fmt.Errorf("参数不完整")
	}
	payload := map[string]string{
		"clientId": cmd.ClientId,
		"action":   cmd.Action,
	}
	b, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/executor/valveControl", o.ExecutorBase)
	log.Printf("[Executor] POST %s payload=%s", url, string(b))
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		log.Printf("[Executor] request error: %v", err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("[Executor] response status=%d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("执行失败 http=%d", resp.StatusCode)
	}
	return nil
}

// ResolveRegionCommands 读取映射文件 executors，将区域命令映射成具体 clientId 指令
func (o *Orchestrator) ResolveRegionCommands(rcs []RegionCommand) ([]ExecCommand, error) {
	if o.MappingPath == "" {
		return nil, fmt.Errorf("mapping path is empty")
	}
	path := filepath.Clean(o.MappingPath)
	log.Printf("[Resolve] using mapping path=%s", path)
	b, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[Resolve] read mapping error: %v", err)
		return nil, fmt.Errorf("读取执行映射失败: %w", err)
	}

	// 按动漫， -> 通道 -> 分区 -> executors 结构解析
	var registry struct {
		Domains []DomainEntry `json:"domains"`
	}
	if err := json.Unmarshal(b, &registry); err != nil {
		log.Printf("[Resolve] parse executors error: %v", err)
		return nil, fmt.Errorf("解析执行映射失败: %w", err)
	}

	partitions := findPartitionsForChannel(registry.Domains, o.DomainID, o.ChannelID)
	if len(partitions) == 0 {
		return nil, fmt.Errorf("未找到匹配的 domain/channel")
	}
	log.Printf("[Resolve] partitions=%d, regionCmds=%d", len(partitions), len(rcs))

	var out []ExecCommand
	for _, rc := range rcs {
		log.Printf("[Resolve] region cmd pid=%s pname=%s action=%s", rc.PartitionID, rc.PartitionName, rc.Action)
		for _, p := range partitions {
			idMatch := rc.PartitionID != "" && p.PartitionID == rc.PartitionID
			nameMatch := rc.PartitionName != "" && p.PartitionName == rc.PartitionName
			if idMatch || nameMatch {
				for _, cid := range p.Executors {
					out = append(out, ExecCommand{ClientId: cid, Action: rc.Action, Reason: rc.Reason})
					log.Printf("[Resolve] matched clientId=%s", cid)
				}
				break
			}
		}
	}
	log.Printf("[Resolve] output exec commands=%d", len(out))
	return out, nil
}

// 调试辅助：安全截断日志输出
func trunc(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
