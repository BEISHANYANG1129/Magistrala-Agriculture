package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// 微调模型客户端结构体
type FinetuneClient struct {
	Endpoint string // 微调模型API地址
}

// 微调模型请求结构体（假设为 prompt 字段）
type FinetuneRequest struct {
	Prompt string `json:"prompt"`
}

// 微调模型响应结构体（假设为 result 字段）
type FinetuneResponse struct {
	Result string `json:"result"`
}

// 微调模型推理请求
func (c *FinetuneClient) Infer(prompt string) (string, error) {
	reqBody := FinetuneRequest{
		Prompt: prompt,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := http.Post(c.Endpoint, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result FinetuneResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Result, nil
}
