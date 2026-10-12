package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// 判断是否为图片URL
func IsImageURL(url string) bool {
	return strings.HasPrefix(url, "http") &&
		(strings.HasSuffix(url, ".png") || strings.HasSuffix(url, ".jpg") ||
			strings.HasSuffix(url, ".jpeg") || strings.HasSuffix(url, ".bmp") || strings.HasSuffix(url, ".gif"))
}

// RemoveThinkSection 用于剔除字符串中所有 <think> ... </think> 部分。
// 如果模型输出中包含 <think> 标签包裹的内容（如 Ollama/Qwen 等大模型常见），
// 调用本函数可自动去除这些无关内容，便于后续 JSON 解析或展示。
func RemoveThinkSection(s string) string {
	for {
		start := strings.Index(s, "<think>")
		end := strings.Index(s, "</think>")
		// 如果没有找到成对的 <think>...</think>，则退出循环
		if start == -1 || end == -1 || end < start {
			break
		}
		// 剔除 <think>...</think> 之间的内容
		s = s[:start] + s[end+len("</think>"):]
	}
	return s
}

// TrimSpaceAll 用于剔除字符串前后的所有空白字符（包括空格、换行、制表符等）。
// 常用于模型输出的清洗，保证结果内容干净。
func TrimSpaceAll(s string) string {
	return strings.TrimSpace(s)
}

// AnalysisResult 用于接收大模型输出的结构化内容
type AnalysisResult struct {
	Summary     string   `json:"summary"`
	Diagnosis   string   `json:"diagnosis"`
	Risks       []string `json:"risks"`
	Suggestions []string `json:"suggestions"`
	RawAnalysis string   `json:"raw_analysis"`
}

// 提取 JSON 子串（用于去除 markdown 代码块包裹）
func ExtractJSON(s string) string {
	re := regexp.MustCompile(`\{[\s\S]*\}`)
	return re.FindString(s)
}

// JSONToStruct 尝试将字符串解析为 AnalysisResult 结构体。
// 如果解析失败，返回 error。
func JSONToStruct(s string) (AnalysisResult, error) {
	var res AnalysisResult
	err := json.Unmarshal([]byte(s), &res)
	if err != nil {
		fmt.Println("原始模型输出：", s)
		fmt.Println("解析错误：", err)
	}
	return res, err
}
