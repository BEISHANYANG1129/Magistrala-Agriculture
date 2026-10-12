package llm

// 大模型统一接口
type LLMClient interface {
	Infer(prompt string) (string, error)
	InferMultimodal(content []map[string]interface{}) (string, error)
}
