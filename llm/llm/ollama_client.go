package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaClient 封装了 Ollama LLM 的基本信息和推理接口
type OllamaClient struct {
	Endpoint string // Ollama 服务端地址，例如 "http://localhost:11434"
	Model    string // 使用的模型名称，例如 "gemma3:12b"
}

// OllamaRequest 表示文本推理请求体结构
type OllamaRequest struct {
	Model  string `json:"model"`  // 模型名称
	Prompt string `json:"prompt"` // 纯文本 prompt
}

// OllamaResponse 表示文本推理响应结构
type OllamaResponse struct {
	Response string `json:"response"` // LLM 返回的文本内容
}

// Infer 实现文本推理接口，适用于纯文本模型
func (c *OllamaClient) Infer(prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model:  c.Model,
		Prompt: prompt,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := http.Post(fmt.Sprintf("%s/api/generate", c.Endpoint), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result OllamaResponse
	var fullResponse string
	decoder := json.NewDecoder(resp.Body)
	// Ollama 文本接口可能返回流式响应，这里循环读取所有响应片段
	for decoder.More() {
		if err := decoder.Decode(&result); err != nil {
			break
		}
		fullResponse += result.Response
	}
	return fullResponse, nil
}

// OllamaMultimodalRequest 表示多模态推理请求体结构
type OllamaMultimodalRequest struct {
	Model    string                   `json:"model"`    // 模型名称
	Messages []map[string]interface{} `json:"messages"` // 多模态消息体，包含文本、图片等
}

// OllamaMultimodalResponse 适配新版 Ollama 多模态接口响应结构
// choices[0].message.content 为大模型输出内容
type OllamaMultimodalResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"` // LLM 返回的内容（可能带 markdown 代码块）
		} `json:"message"`
	} `json:"choices"`
}

// InferMultimodal 实现多模态推理接口，适用于支持图片/视频输入的模型
func (c *OllamaClient) InferMultimodal(content []map[string]interface{}) (string, error) {
	// 构造多模态请求体
	reqBody := OllamaMultimodalRequest{
		Model: c.Model,
		Messages: []map[string]interface{}{
			{
				"role":    "user",
				"content": content, // 由 BuildOllamaMessages 构建的多模态消息体
			},
		},
	}
	data, _ := json.Marshal(reqBody)
	// 发送 POST 请求到 Ollama 多模态接口
	resp, err := http.Post(fmt.Sprintf("%s/v1/chat/completions", c.Endpoint), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码，非 200 直接返回错误内容
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama 返回错误: %s", string(b))
	}

	// 读取完整响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析 JSON 响应
	var result OllamaMultimodalResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}
	// 检查 choices 是否有内容
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("ollama 返回内容为空")
	}
	// 返回大模型输出内容（通常为 markdown 代码块包裹的 JSON）
	return result.Choices[0].Message.Content, nil
}
