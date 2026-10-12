package agridataintegration

import (
	"encoding/json"
	"os"
)

// Config 集成系统配置
type Config struct {
	// 农业平台配置
	AgriPlatform struct {
		BaseURL  string `json:"baseUrl"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"agriPlatform"`

	// Magistrala 平台配置
	Magistrala struct {
		// baseUrl 和 userToken 从共享配置 data/magistrala.json 读取
		ChannelPort string `json:"channelPort"`
		ClientPort  string `json:"clientPort"`
		MessagePort string `json:"messagePort"`
		DomainID    string `json:"domainId"`
		ChannelID   string `json:"channelId"`
	} `json:"magistrala"`

	// 集成配置
	Integration struct {
		SyncInterval     int    `json:"syncInterval"`     // 数据同步间隔（秒）
		MappingFile      string `json:"mappingFile"`      // 映射表文件路径
		BackgroundImage  string `json:"backgroundImage"`  // 背景图片名称
		DefaultPartition string `json:"defaultPartition"` // 默认分区名称
	} `json:"integration"`

	// 服务器配置
	Server struct {
		Port string `json:"port"` // 服务端口
		Host string `json:"host"` // 服务主机
	} `json:"server"`
}

// LoadConfig 从文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig(configPath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
