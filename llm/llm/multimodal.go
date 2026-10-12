package llm

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BuildOllamaMessagesWithPrompt 唯一入口：传入提示词与消息，拼成多模态输入。
// 提示词需外部从 MD/TXT 读取。
func BuildOllamaMessagesWithPrompt(messages []map[string]interface{}, prompt string) ([]map[string]interface{}, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt 为空，请先从 MD/TXT 读取提示词后再调用")
	}
	return BuildMultimodalContent(prompt, messages)
}

// BuildMultimodalContent 将提示词与传感器/媒体消息拼接成 Ollama 多模态输入。
func BuildMultimodalContent(prompt string, messages []map[string]interface{}) ([]map[string]interface{}, error) {
	var content []map[string]interface{}
	if prompt != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": prompt,
		})
	}

	for _, msg := range messages {
		content = appendSensorContent(content, msg)
	}

	return content, nil
}

// appendSensorContent adds sensor readings and multimodal payloads.
func appendSensorContent(content []map[string]interface{}, msg map[string]interface{}) []map[string]interface{} {
	if pid, ok := msg["partition_id"]; ok && pid != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("分区ID：%v", pid),
		})
	}
	if pname, ok := msg["partition_name"]; ok && pname != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("分区名称：%v", pname),
		})
	}
	if name, ok := msg["name"]; ok && name != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("名称：%v", name),
		})
	}
	if v, ok := msg["value"]; ok {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("数值：%v", v),
		})
	}
	if unit, ok := msg["unit"]; ok && unit != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("单位：%v", unit),
		})
	}
	// 保留时间戳，便于近时段判断
	if t, ok := msg["time"]; ok && t != nil && fmt.Sprint(t) != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("时间：%v", t),
		})
	}

	if sv, ok := msg["string_value"].(string); ok && sv != "" {
		switch {
		case isImageURL(sv):
			b64, mimeType, err := urlToBase64(sv)
			if err != nil {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": fmt.Sprintf("图片获取失败：%v", err),
				})
			} else {
				content = append(content, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": fmt.Sprintf("data:%s;base64,%s", mimeType, b64),
					},
				})
			}
		case isVideoURL(sv):
			b64, mimeType, err := urlToBase64(sv)
			if err != nil {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": fmt.Sprintf("视频获取失败：%v", err),
				})
			} else {
				content = append(content, map[string]interface{}{
					"type": "video_url",
					"video_url": map[string]interface{}{
						"url": fmt.Sprintf("data:%s;base64,%s", mimeType, b64),
					},
				})
			}
		default:
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("文本：%v", sv),
			})
		}
	}

	return content
}

// 判断是否为图片URL
func isImageURL(url string) bool {
	lower := strings.ToLower(url)
	return (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) &&
		(strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp"))
}

// 判断是否为视频URL
func isVideoURL(url string) bool {
	lower := strings.ToLower(url)
	return (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) &&
		(strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".mov") || strings.HasSuffix(lower, ".avi") || strings.HasSuffix(lower, ".mkv"))
}

// 下载文件并转base64
func urlToBase64(url string) (string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	mimeType := http.DetectContentType(data)
	b64 := base64.StdEncoding.EncodeToString(data)
	return b64, mimeType, nil
}
