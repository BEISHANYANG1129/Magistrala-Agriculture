package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// 本文件统一从 JSON 配置读取执行所需参数（模仿 agriDataIntegration/config.json 结构）。
// - 路径：优先环境变量 CONFIG_PATH，其次 CREDENTIALS_PATH（兼容旧版本），否则 ./internal/config/config.json
// - 结构：
//   {
//     "agriPlatform": {"baseUrl":"...","username":"...","password":"...","userToken":"..."},
//     "magistrala":   {"baseUrl":"...","userToken":"...","domainId":"...","channelId":"..."}
//   }

type AppConfig struct {
	AgriPlatform struct {
		BaseURL   string `json:"baseUrl"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		UserToken string `json:"userToken,omitempty"`
	} `json:"agriPlatform"`
	Magistrala struct {
		BaseURL   string `json:"baseUrl,omitempty"`
		UserToken string `json:"userToken,omitempty"`
		DomainID  string `json:"domainId"`
		ChannelID string `json:"channelId"`
	} `json:"magistrala"`
}

// CredentialsPath 返回配置文件路径（兼容旧变量名）。
func CredentialsPath() string {
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		return env
	}
	if env := os.Getenv("CREDENTIALS_PATH"); env != "" { // 兼容
		return env
	}
	return "./internal/config/config.json"
}

// loadCredentials 读取并解析凭据文件；不存在或解析失败直接报错
func loadCredentials() (*AppConfig, error) {
	path := CredentialsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取凭据文件失败: %w", err)
	}
	var c AppConfig
	if err = json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("解析凭据文件失败: %w", err)
	}
	return &c, nil
}

// writeCredentials 将 Credentials 原子化写回 JSON 文件（0600 权限）。
func writeCredentials(c AppConfig) error {
	path := CredentialsPath()

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 序列化（缩进便于人工查看）
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化凭据失败: %w", err)
	}

	// 原子写入：先写临时文件再替换
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, fs.FileMode(0o600)); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换凭据文件失败: %w", err)
	}
	return nil
}

// GetLoginCredentials 获取农业平台账号与密码。
// 返回：username, password, error
func GetLoginCredentials() (string, string, error) {
	c, err := loadCredentials()
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(c.AgriPlatform.Username) == "" || strings.TrimSpace(c.AgriPlatform.Password) == "" {
		return "", "", errors.New("配置缺少 agriPlatform.username 或 password")
	}
	return c.AgriPlatform.Username, c.AgriPlatform.Password, nil
}

// ErrTokenMissing 严格模式下本地未找到已登录 token 时返回该错误。
var ErrTokenMissing = errors.New("未登录")

// GetUserToken 严格读取 userToken：为空返回 ErrTokenMissing。
func GetUserToken() (string, error) {
	c, err := loadCredentials()
	if err != nil {
		return "", err
	}
	tk := strings.TrimSpace(c.AgriPlatform.UserToken)
	if tk == "" {
		return "", ErrTokenMissing
	}
	return tk, nil
}

// SetUserToken 写入/更新 userToken。
// 成功后落盘到凭据 JSON（权限 0600）。
func SetUserToken(token string) error {
	c, err := loadCredentials()
	if err != nil {
		return err
	}
	c.AgriPlatform.UserToken = token
	return writeCredentials(*c) // 解引用
}

// 默认第三方平台 API 基础地址（当 JSON 未设置时回退）。
const defaultAPIBaseURL = "http://api.farm.0531yun.cn"

// GetAPIBaseURL 读取第三方平台 API 基础地址。
// 优先从凭据 JSON 的 apiBaseURL 字段读取；若为空则回退到内置默认值。
func GetAPIBaseURL() (string, error) {
	c, err := loadCredentials()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(c.AgriPlatform.BaseURL)
	if v == "" {
		return defaultAPIBaseURL, nil
	}
	return v, nil
}

// GetNormalizedAPIBaseURL 返回去除尾部斜杠的基础地址。
func GetNormalizedAPIBaseURL() (string, error) {
	base, err := GetAPIBaseURL()
	if err != nil {
		return "", fmt.Errorf("读取 API 基础地址失败: %w", err)
	}
	return strings.TrimRight(base, "/"), nil
}

// LoadCredentials 提供一个对外导出的完整凭据读取方法，供 service 层一次性获得所有字段。
// 注意：若仅需账号或 token，优先使用专门的 GetLoginCredentials / GetUserToken 以减少不必要的解析。
func LoadCredentials() (AppConfig, error) {
	c, err := loadCredentials()
	if err != nil {
		return AppConfig{}, err
	}
	return *c, nil // 解引用后返回
}
