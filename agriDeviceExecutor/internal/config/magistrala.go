package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// 严格从 config.json 读取基础配置（不带端口）
func GetMagistralaBaseURL() (string, error) {
	// 仅从共享配置 data/magistrala.json 读取
	if shared := tryLoadSharedMagBase(); shared != nil {
		v := strings.TrimSpace(shared.BaseURL)
		if v != "" {
			return strings.TrimRight(v, "/"), nil
		}
	}
	return "", errors.New("缺少共享配置 data/magistrala.json 的 baseUrl")
}

func GetMagistralaToken() (string, error) {
	// 仅从共享配置 data/magistrala.json 读取
	if shared := tryLoadSharedMagBase(); shared != nil {
		t := strings.TrimSpace(shared.UserToken)
		if t != "" {
			return t, nil
		}
	}
	return "", errors.New("缺少共享配置 data/magistrala.json 的 userToken")
}

func GetMagistralaDomainID() (string, error) {
	c, err := loadCredentials()
	if err != nil {
		return "", errors.New("读取 config.json 失败")
	}
	d := strings.TrimSpace(c.Magistrala.DomainID)
	if d == "" {
		return "", errors.New("缺少 magistrala.domainId，请在 config.json 配置")
	}
	return d, nil
}

func GetMagistralaChannelID() (string, error) {
	c, err := loadCredentials()
	if err != nil {
		return "", errors.New("读取 config.json 失败")
	}
	id := strings.TrimSpace(c.Magistrala.ChannelID)
	if id == "" {
		return "", errors.New("缺少 magistrala.channelId，请在 config.json 配置")
	}
	return id, nil
}

// tryLoadSharedMagBase 尝试从共享配置文件读取 baseUrl/userToken
// 共享文件路径优先顺序：
//   - ../data/magistrala.json （从 agriDeviceExecutor 运行目录）
//   - ../../data/magistrala.json （从更深子目录执行时）
//   - data/magistrala.json （从仓库根目录执行时）
func tryLoadSharedMagBase() *struct {
	BaseURL   string `json:"baseUrl"`
	UserToken string `json:"userToken"`
} {
	candidates := []string{
		"../data/magistrala.json",
		"../../data/magistrala.json",
		"data/magistrala.json",
	}
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			var out struct {
				BaseURL   string `json:"baseUrl"`
				UserToken string `json:"userToken"`
			}
			if json.Unmarshal(b, &out) == nil {
				if strings.TrimSpace(out.BaseURL) != "" || strings.TrimSpace(out.UserToken) != "" {
					return &out
				}
			}
		}
	}
	return nil
}
