package llm

// 模型加载相关（Ollama本地服务无需显式加载，保留接口便于扩展）
func NewOllamaClient(endpoint, model string) *OllamaClient {
	return &OllamaClient{
		Endpoint: endpoint,
		Model:    model,
	}
}
