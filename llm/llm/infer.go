package llm

import (
	"fmt"
	"log"
)

// AnalyzeRegionCommands 让模型直接输出“区域级指令”JSON（字符串返回），供上层编排器解析为 RegionCommand。
func AnalyzeRegionCommands(client LLMClient, messages []map[string]interface{}) (string, error) {
	return AnalyzeRegionCommandsWithPrompt(client, messages, "")
}

// AnalyzeRegionCommandsWithPrompt 允许自定义提示词（非空时覆盖默认提示）。
func AnalyzeRegionCommandsWithPrompt(client LLMClient, messages []map[string]interface{}, prompt string) (string, error) {
	// 1. 构建区域级命令提示（需外部提供 prompt）
	content, err := BuildOllamaMessagesWithPrompt(messages, prompt)
	if err != nil {
		return "", err
	}
	// 2. 模型推理
	result, err := client.InferMultimodal(content)
	if err != nil {
		return "", err
	}
	log.Printf("[Infer] llm raw output=%s", result)
	// 3. 清洗
	cleaned := RemoveThinkSection(result)
	cleaned = TrimSpaceAll(cleaned)
	// 4. 仅返回 JSON 子串
	jsonStr := ExtractJSON(cleaned)
	if jsonStr == "" {
		return "", fmt.Errorf("未检测到有效JSON，请检查大模型输出")
	}
	return jsonStr, nil
}
