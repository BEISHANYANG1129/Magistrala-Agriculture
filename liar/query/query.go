package query

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// 查询配置参数结构体
type QueryConfig struct {
	ServicePort  int
	DomainID     string
	ChannelID    string
	Offset       int
	Limit        int
	ClientSecret string
	Timeout      int
}

// 消息响应结构体
type MessageResponse struct {
	Offset   int           `json:"offset"`
	Limit    int           `json:"limit"`
	Format   string        `json:"format"`
	Total    int           `json:"total"`
	Messages []interface{} `json:"messages"`
	Error    string        `json:"error,omitempty"`
}

// 格式化响应数据
func formatResponse(responseData map[string]interface{}, offset, limit int) MessageResponse {
	total, _ := responseData["total"].(float64)
	messages, _ := responseData["messages"].([]interface{})
	return MessageResponse{
		Offset:   offset,
		Limit:    limit,
		Format:   "messages",
		Total:    int(total),
		Messages: messages,
	}
}

// 获取消息
func FetchMessages(cfg QueryConfig) MessageResponse {
	url := fmt.Sprintf("http://localhost:%d/%s/channels/%s/messages", cfg.ServicePort, cfg.DomainID, cfg.ChannelID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return MessageResponse{Error: err.Error()}
	}

	// 设置请求参数
	q := req.URL.Query()
	q.Add("offset", fmt.Sprintf("%d", cfg.Offset))
	q.Add("limit", fmt.Sprintf("%d", cfg.Limit))
	req.URL.RawQuery = q.Encode()

	// 设置请求头
	req.Header.Set("Authorization", "Client "+cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}

	log.Println("===== 开始获取消息请求 =====")
	// log.Printf("请求URL: %s", req.URL.String())
	// log.Printf("请求头: %v", req.Header)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("获取消息失败: %v", err)
		return MessageResponse{Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// log.Printf("响应状态码: %d", resp.StatusCode)
	// log.Printf("响应内容: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return MessageResponse{Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))}
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		log.Printf("解析响应JSON失败: %v", err)
		return MessageResponse{Error: err.Error()}
	}

	result := formatResponse(responseData, cfg.Offset, cfg.Limit)
	log.Println("消息获取成功")
	return result
}
