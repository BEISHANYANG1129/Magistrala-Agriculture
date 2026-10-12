// Package llm 提供与 Magistrala 消息检索与映射相关的实用函数，
// 用于从渠道拉取数据、转换为 LLM 可读格式，并基于本地映射文件补充分区信息。
package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// MessagesResponse 定义 Magistrala /channels/{id}/messages 返回结构（精简版）。
type MessagesResponse struct {
	Offset   int          `json:"offset"`
	Limit    int          `json:"limit"`
	Order    string       `json:"order"`
	Dir      string       `json:"dir"`
	Format   string       `json:"format"`
	Total    int          `json:"total"`
	Messages []MagMessage `json:"messages"`
}

// MagMessage 是单条消息（通常为 SenML 扁平化后的字段）。
type MagMessage struct {
	Channel   string `json:"channel"`
	Subtopic  string `json:"subtopic"`
	Publisher string `json:"publisher"`
	Protocol  string `json:"protocol"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Time      int64  `json:"time"`
	ValueAny  any    `json:"value"` // value 可能是数字或字符串
}

// FetchChannelMessages 从 Magistrala 拉取指定频道的消息列表，并返回结构化的响应体。
// 参数说明：
//   - baseURL: 不含端口的主机地址（例 http://localhost）。
//   - messagePort: 消息服务端口（常见为 9011 或控制台暴露端口）。
//   - domainId/channelId: 目标域和频道标识。
//   - token: Bearer 访问令牌。
//   - offset/limit: 翻页与窗口大小，limit<=0 时默认 10。
//
// 行为：构造 GET /{domain}/channels/{channel}/messages?offset&limit&order=time&dir=desc&format=messages
// 并在 10 秒超时内完成请求；非 200 会返回错误并附带响应体。
// 返回值：解析后的 MessagesResponse 或错误。
func FetchChannelMessages(baseURL string, messagePort int, domainId, channelId, token string, offset, limit int) (*MessagesResponse, error) {
	if baseURL == "" || domainId == "" || channelId == "" || token == "" {
		return nil, fmt.Errorf("缺少必要参数: baseURL/domainId/channelId/token")
	}
	if limit <= 0 {
		limit = 10
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析 baseURL 失败: %w", err)
	}
	// 拼接端口
	u.Host = fmt.Sprintf("%s:%d", u.Hostname(), messagePort)
	// 构造路径 /{domain}/channels/{channel}/messages
	u.Path = fmt.Sprintf("/%s/channels/%s/messages", domainId, channelId)
	// 查询参数
	q := url.Values{}
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("order", "time")
	q.Set("dir", "desc")
	q.Set("format", "messages")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败 http=%d body=%s", resp.StatusCode, string(body))
	}
	var out MessagesResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &out, nil
}

// ToLLMMessages 将 Magistrala 消息转换为 LLM 侧消费的通用 map 列表。
// 先展开原始消息并补充分区信息，再构造精简后的消息体：
//   - 保留：partition_id/partition_name、name(或 subtopic 兜底)、value、unit、string_value、time
//   - 去掉：channel、protocol、publisher、clientId 等与推理无关字段
func ToLLMMessages(resp *MessagesResponse, domainID, channelID, mappingPath string) []map[string]interface{} {
	var out []map[string]interface{}
	if resp == nil {
		log.Printf("[ToLLM] resp is nil")
		return out
	}
	log.Printf("[ToLLM] converting messages=%d", len(resp.Messages))

	// 1) 先展开为 map，便于分区补全
	rawMsgs := make([]map[string]interface{}, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		rawMsgs = append(rawMsgs, map[string]interface{}{
			"channel":   m.Channel,
			"subtopic":  m.Subtopic,
			"publisher": m.Publisher,
			"protocol":  m.Protocol,
			"time":      m.Time,
			"name":      m.Name,
			"unit":      m.Unit,
			"value":     m.ValueAny,
			// 预留分区字段
			"partition_id":   "",
			"partition_name": "",
		})
	}

	// 2) 分区补全（基于 mappingPath）
	EnrichPartitionsFromRegistry(rawMsgs, domainID, channelID, mappingPath)
	log.Printf("[ToLLM] after enrich partitions, messages=%d", len(rawMsgs))

	// 3) 瘦身：仅保留分区+传感器必要字段+时间
	for _, msg := range rawMsgs {
		slim := map[string]interface{}{}
		if v := fmt.Sprint(msg["partition_id"]); v != "" {
			slim["partition_id"] = v
		}
		if v := fmt.Sprint(msg["partition_name"]); v != "" {
			slim["partition_name"] = v
		}
		// 传感器标识：优先 name，无则用 subtopic
		if v := fmt.Sprint(msg["name"]); v != "" {
			slim["name"] = v
		} else if v := fmt.Sprint(msg["subtopic"]); v != "" {
			slim["name"] = v
		}
		if v, ok := msg["value"]; ok {
			slim["value"] = v
		}
		if v := fmt.Sprint(msg["unit"]); v != "" {
			slim["unit"] = v
		}
		if v := fmt.Sprint(msg["string_value"]); v != "" {
			slim["string_value"] = v
		}
		// 保留时间戳用于近时段判断
		if v, ok := msg["time"]; ok {
			slim["time"] = v
		}

		out = append(out, slim)
	}

	return out
}

// EnrichPartitionsFromRegistry 使用 publisher 作为匹配键，为消息写入 partition_id/partition_name。
// 依赖本地 mappingPath（设备注册表文件）来解析域/通道下的分区信息，缺失路径会直接中止。
// 不会覆盖已有的分区字段；未匹配到分区的消息保持原样。
func EnrichPartitionsFromRegistry(msgs []map[string]interface{}, domainID, channelID, mappingPath string) {
	if len(msgs) == 0 {
		log.Printf("[Registry] no messages to enrich")
		return
	}
	if mappingPath == "" {
		log.Fatalf("mapping path is empty; please set mapping.path in config")
	}
	registryPath := filepath.Clean(mappingPath)
	log.Printf("[Registry] loading %s", registryPath)
	b, err := os.ReadFile(registryPath)
	if err != nil {
		log.Printf("[Registry] read failed: %v", err)
		return
	}
	var registry struct {
		Domains []DomainEntry `json:"domains"`
	}
	if err := json.Unmarshal(b, &registry); err != nil {
		log.Printf("[Registry] parse failed: %v", err)
		return
	}

	partitions := findPartitionsForChannel(registry.Domains, domainID, channelID)
	if len(partitions) == 0 {
		log.Printf("[Registry] no partitions found for domain/channel")
		return
	}

	// 构建 publisher → 分区索引（仅当前域/通道）
	idx := make(map[string]struct{ id, name string })
	for _, p := range partitions {
		for _, pub := range p.Sensors {
			if pub == "" {
				continue
			}
			idx[pub] = struct{ id, name string }{id: p.PartitionID, name: p.PartitionName}
		}
	}
	log.Printf("[Registry] partitions=%d, index size=%d", len(partitions), len(idx))

	matched := 0
	for i := range msgs {
		m := msgs[i]
		// 已有分区不覆盖
		if fmt.Sprint(m["partition_id"]) != "" || fmt.Sprint(m["partition_name"]) != "" {
			continue
		}
		pub := fmt.Sprint(m["publisher"])
		if pub == "" {
			continue
		}
		if p, ok := idx[pub]; ok {
			if p.id != "" {
				m["partition_id"] = p.id
			}
			if p.name != "" {
				m["partition_name"] = p.name
			}
			matched++
		}
	}
	log.Printf("[Registry] enriched matched=%d / total=%d", matched, len(msgs))
}

// findPartitionsForChannel 在给定域/通道下定位分区切片；若未找到匹配则返回 nil。
func findPartitionsForChannel(domains []DomainEntry, domainID, channelID string) []PartitionEntry {
	for _, d := range domains {
		if domainID != "" && d.DomainID != domainID {
			continue
		}
		for _, c := range d.Channels {
			if channelID != "" && c.ChannelID != channelID {
				continue
			}
			return c.Partitions
		}
	}
	return nil
}
