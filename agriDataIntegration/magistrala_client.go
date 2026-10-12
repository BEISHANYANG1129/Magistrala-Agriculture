package agridataintegration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ================================
// 数据结构定义
// ================================

// MagistralaClient Magistrala 平台客户端
// 用于与 Magistrala IoT 平台进行交互，包括客户端管理、频道连接和消息发送
type MagistralaClient struct {
	BaseURL   string // Magistrala 服务器基础URL（不含端口）
	UserToken string // 用户认证令牌

	// 不同服务的端口配置
	ChannelPort string // 频道服务端口 (默认: 9005)
	ClientPort  string // 客户端服务端口 (默认: 9006)
	MessagePort string // 消息服务端口 (例如: "9011")
}

// ClientRequest 客户端创建请求结构
// 用于向 Magistrala 平台创建新的 IoT 客户端
type ClientRequest struct {
	Name        string                 `json:"name"`        // 客户端名称
	Tags        []string               `json:"tags"`        // 标签列表，用于分类
	Credentials map[string]interface{} `json:"credentials"` // 认证凭据（包含identity和secret）
	Metadata    map[string]interface{} `json:"metadata"`    // 元数据，存储设备相关信息
	Status      string                 `json:"status"`      // 客户端状态 (enabled/disabled)
}

// ClientResponse 客户端响应结构
// Magistrala 平台返回的客户端信息
type ClientResponse struct {
	ID          string                 `json:"id"`          // 客户端唯一标识符
	Name        string                 `json:"name"`        // 客户端名称
	Tags        []string               `json:"tags"`        // 标签列表
	Credentials map[string]interface{} `json:"credentials"` // 认证凭据
	Metadata    map[string]interface{} `json:"metadata"`    // 元数据
	Status      string                 `json:"status"`      // 状态
	CreatedAt   string                 `json:"created_at"`  // 创建时间
	UpdatedAt   string                 `json:"updated_at"`  // 更新时间
}

// MessagePayload 消息载荷结构
// 包含传感器数据的完整信息，用于发送到 Magistrala 平台
type MessagePayload struct {
	// 时间戳信息
	Timestamp int64 `json:"timestamp"` // Unix时间戳 (毫秒)

	// 设备基础信息
	DeviceAddr   int    `json:"device_addr"`   // 设备地址
	DeviceName   string `json:"device_name"`   // 设备名称
	NodeID       int    `json:"node_id"`       // 节点ID
	RegisterID   int    `json:"register_id"`   // 寄存器ID
	RegisterName string `json:"register_name"` // 寄存器名称

	// 传感器信息
	FactorName string `json:"factor_name"` // 因子名称 (如"空气温度")
	ClientName string `json:"client_name"` // Magistrala客户端名称

	// 数据值
	Value float64 `json:"value"` // 数值型数据
	Text  string  `json:"text"`  // 文本型数据
	Unit  string  `json:"unit"`  // 单位

	// 报警信息
	AlarmLevel int    `json:"alarm_level"` // 报警级别
	AlarmInfo  string `json:"alarm_info"`  // 报警描述

	// 地理位置
	Latitude  float64 `json:"latitude"`  // 纬度
	Longitude float64 `json:"longitude"` // 经度
}

// ================================
// 客户端初始化
// ================================

// NewMagistralaClient 创建 Magistrala 客户端实例
// 参数:
//   - baseURL: Magistrala 服务器基础URL
//   - userToken: 用户认证令牌
//
// 返回: MagistralaClient 实例指针
func NewMagistralaClient(baseURL, userToken string, channelPort, clientPort string, messagePort string) *MagistralaClient {
	return &MagistralaClient{
		BaseURL:     baseURL,
		UserToken:   userToken,
		ChannelPort: channelPort,
		ClientPort:  clientPort,
		MessagePort: messagePort,
	}
}

// ================================
// 频道管理功能
// ================================

// GetChannelMetadata 获取频道元数据
// 用于获取指定频道的详细信息和配置
// 参数:
//   - domainID: 域ID
//   - channelID: 频道ID
//
// 返回: 频道元数据映射和错误信息
func (c *MagistralaClient) GetChannelMetadata(domainID, channelID string) (map[string]interface{}, error) {
	// 构造请求URL
	url := fmt.Sprintf("%s:%s/%s/channels/%s", c.BaseURL, c.ChannelPort, domainID, channelID)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置认证头
	req.Header.Set("Authorization", "Bearer "+c.UserToken)

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// ================================
// 客户端管理功能
// ================================

// CreateClient 创建新的 Magistrala 客户端
// 在 Magistrala 平台上为传感器创建对应的客户端实例
// 参数:
//   - domainID: 域ID
//   - req: 客户端创建请求结构
//
// 返回: 创建的客户端信息和错误
func (c *MagistralaClient) CreateClient(domainID string, req *ClientRequest) (*ClientResponse, error) {
	// 构造请求URL
	url := fmt.Sprintf("%s:%s/%s/clients", c.BaseURL, c.ClientPort, domainID)

	// 序列化请求体
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.UserToken)

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	// 解析响应
	var result ClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ConnectToChannel 连接客户端到指定频道
// 建立客户端与频道之间的发布/订阅关系
// 参数:
//   - domainID: 域ID
//   - clientID: 客户端ID
//   - channelID: 频道ID
//
// 返回: 错误信息
func (c *MagistralaClient) ConnectToChannel(domainID, clientID, channelID string) error {
	// 构造连接API URL
	url := fmt.Sprintf("%s:%s/%s/channels/connect", c.BaseURL, c.ChannelPort, domainID)

	// 构造连接请求体
	connectReq := map[string]interface{}{
		"channel_ids": []string{channelID},              // 要连接的频道列表
		"client_ids":  []string{clientID},               // 要连接的客户端列表
		"types":       []string{"publish", "subscribe"}, // 连接类型：发布和订阅
	}

	// 序列化请求体
	jsonData, err := json.Marshal(connectReq)
	if err != nil {
		return fmt.Errorf("failed to marshal connect request: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create connect request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.UserToken)

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make connect request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("connect failed with status %d", resp.StatusCode)
	}

	return nil
}

// ================================
// 消息发送功能
// ================================

// SendMessage 发送传感器数据消息到 Magistrala 频道
// 使用 SenML 格式发送传感器数据，支持中文到英文的自动翻译
// 参数:
//   - domainID: 域ID
//   - channelID: 频道ID
//   - clientSecret: 客户端密钥
//   - payload: 消息载荷（包含传感器数据）
//
// 返回: 错误信息
func (c *MagistralaClient) SendMessage(domainID, channelID, clientSecret string, payload *MessagePayload) error {
	// 配置子主题（固定为light用于测试）
	subtopic := "light"
	url := fmt.Sprintf("%s:%s/http/m/%s/c/%s/%s", c.BaseURL, c.MessagePort, domainID, channelID, subtopic)

	// 输出调试信息
	fmt.Printf("🔍 发送消息调试信息:\n")
	fmt.Printf("   URL: %s\n", url)
	fmt.Printf("   Client Secret: %s\n", clientSecret)
	fmt.Printf("   ClientName: %s\n", payload.ClientName)
	fmt.Printf("   传感器: %s (值: %.2f %s)\n", payload.FactorName, payload.Value, payload.Unit)

	// 中文到英文翻译处理
	englishClientName := translateClientNameToEnglish(payload.ClientName)
	englishUnit := translateUnitToEnglish(payload.Unit)

	// 构造 SenML 格式的消息记录
	senmlRecord := map[string]interface{}{
		"bn": englishClientName + ":", // Base Name: 英文客户端名称
		"bu": englishUnit,             // Base Unit: 英文基础单位
		"n":  "value",                 // Name: 固定为"value"避免中文问题
		"u":  englishUnit,             // Unit: 英文单位
		"t":  0,                       // Time: 相对时间偏移为0
	}

	// 智能选择数值字段或字符串字段
	if payload.Text != "" && !isNumericString(payload.Text) {
		// 非数字文本值（如"东南风"）使用字符串字段
		englishText := translateTextToEnglish(payload.Text, payload.FactorName)
		senmlRecord["vs"] = englishText // String Value
	} else {
		// 数值（包括0值）使用数值字段
		senmlRecord["v"] = payload.Value // Value
	}

	// 构造 SenML 数据数组
	senmlData := []map[string]interface{}{senmlRecord}

	// 序列化为JSON
	jsonData, err := json.Marshal(senmlData)
	if err != nil {
		return fmt.Errorf("failed to marshal SenML message: %w", err)
	}

	fmt.Printf("   请求体: %s\n", string(jsonData))

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create message request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/senml+json") // SenML JSON格式
	req.Header.Set("Authorization", "Client "+clientSecret)  // 客户端认证

	fmt.Printf("   Content-Type: %s\n", req.Header.Get("Content-Type"))
	fmt.Printf("   Authorization: %s\n", req.Header.Get("Authorization"))

	// 发送请求
	httpClient := &http.Client{Timeout: 10 * time.Second}
	fmt.Printf("🚀 发送请求...\n")
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("✅ 响应状态: %d %s\n", resp.StatusCode, resp.Status)

	// 检查响应状态
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ 消息发送失败，状态码: %d\n", resp.StatusCode)
		return fmt.Errorf("message send failed with status %d", resp.StatusCode)
	}

	fmt.Printf("🎉 消息发送成功!\n")
	return nil
}

// ================================
// 高级客户端管理功能
// ================================

// AssignPartitionPosition 为传感器分配分区和位置信息
// 根据频道元数据或默认规则为传感器分配逻辑分区和物理位置
// 参数:
//   - domainID: 域ID
//   - channelID: 频道ID
//   - mapping: 传感器映射信息（将被更新）
//
// 返回: 错误信息
func (c *MagistralaClient) AssignPartitionPosition(domainID, channelID string, mapping *SensorMapping) error {
	// 获取频道元数据以确定分区策略
	metadata, err := c.GetChannelMetadata(domainID, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel metadata: %w", err)
	}

	// 处理自定义分区逻辑（如果频道元数据中定义了分区）
	if partitions, exists := metadata["partitions"]; exists {
		// 未来可以根据partitions信息进行智能分区
		_ = partitions
	}

	// 使用默认分区策略
	mapping.Partition = "field_1" // 默认分区名称
	mapping.Position.X = 100.0    // 默认X坐标
	mapping.Position.Y = 100.0    // 默认Y坐标

	return nil
}

// CreateMagistralaClientFromSensor 从传感器映射信息创建 Magistrala 客户端
// 根据传感器的详细信息构造客户端创建请求并执行创建操作
// 参数:
//   - domainID: 域ID
//   - mapping: 传感器映射信息
//
// 返回: 创建的客户端响应和错误信息
func (c *MagistralaClient) CreateMagistralaClientFromSensor(domainID string, mapping *SensorMapping) (*ClientResponse, error) {
	// 构造客户端创建请求
	clientReq := &ClientRequest{
		// 客户端基础信息
		Name:   fmt.Sprintf("sensor-%s-%d-%d", mapping.FactorName, mapping.NodeID, mapping.RegisterID),
		Tags:   []string{"sensor", "agriculture", mapping.FactorName}, // 标签用于分类和搜索
		Status: "enabled",                                             // 默认启用状态

		// 认证凭据
		Credentials: map[string]interface{}{
			"identity": fmt.Sprintf("sensor-%d-%d-%d", mapping.DeviceAddr, mapping.NodeID, mapping.RegisterID),
			"secret":   fmt.Sprintf("secret-%d-%d-%d", mapping.DeviceAddr, mapping.NodeID, mapping.RegisterID),
		},

		// 元数据：存储传感器的详细信息
		Metadata: map[string]interface{}{
			"device_addr":   mapping.DeviceAddr,   // 设备地址
			"device_name":   mapping.DeviceName,   // 设备名称
			"node_id":       mapping.NodeID,       // 节点ID
			"register_id":   mapping.RegisterID,   // 寄存器ID
			"register_name": mapping.RegisterName, // 寄存器名称
			"factor_name":   mapping.FactorName,   // 因子名称
			"unit":          mapping.Unit,         // 单位
			"partition":     mapping.Partition,    // 分区
			"position_x":    mapping.Position.X,   // X坐标
			"position_y":    mapping.Position.Y,   // Y坐标
		},
	}

	// 执行客户端创建
	return c.CreateClient(domainID, clientReq)
}

// EnsureClientConnected 确保客户端创建并连接到频道
// 智能处理客户端的创建和连接状态：
// - 对于未创建的客户端：创建并连接
// - 对于已创建但连接失败的客户端：重新连接
// - 对于已创建并连接成功的客户端：跳过处理
// 参数:
//   - domainID: 域ID
//   - channelID: 频道ID
//   - mapping: 传感器映射信息（将被更新）
//
// 返回: 错误信息
func (c *MagistralaClient) EnsureClientConnected(domainID, channelID string, mapping *SensorMapping) error {
	// 1. 检查客户端是否已存在
	if mapping.ClientID != "" {
		// 客户端已存在，尝试连接到频道
		err := c.ConnectToChannel(domainID, mapping.ClientID, channelID)
		if err != nil {
			// 连接失败，重新尝试连接
			fmt.Printf("重新连接客户端 %s: %v\n", mapping.ClientID, err)
			return c.ConnectToChannel(domainID, mapping.ClientID, channelID)
		}
		// 连接成功，无需进一步处理
		fmt.Printf("客户端 %s 已成功连接\n", mapping.ClientID)
		return nil
	}

	// 2. 客户端不存在，需要创建新客户端
	fmt.Printf("创建新客户端用于传感器: %s-%d-%d\n", mapping.FactorName, mapping.NodeID, mapping.RegisterID)

	// 2.1 分配分区和位置信息
	if err := c.AssignPartitionPosition(domainID, channelID, mapping); err != nil {
		return fmt.Errorf("failed to assign partition: %w", err)
	}

	// 2.2 创建客户端
	client, err := c.CreateMagistralaClientFromSensor(domainID, mapping)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// 2.3 更新映射信息
	mapping.ClientID = client.ID
	mapping.ClientName = client.Name
	if secret, ok := client.Credentials["secret"].(string); ok {
		mapping.ClientSecret = secret
	}
	mapping.IsActive = true
	mapping.LastSync = time.Now().Unix()

	// 2.4 连接客户端到频道
	fmt.Printf("连接客户端 %s 到频道 %s\n", client.ID, channelID)
	if err := c.ConnectToChannel(domainID, client.ID, channelID); err != nil {
		return fmt.Errorf("failed to connect client to channel: %w", err)
	}

	fmt.Printf("成功创建并连接客户端: %s\n", client.ID)
	return nil
}

// ================================
// 翻译和工具函数
// ================================

// translateClientNameToEnglish 将包含中文的客户端名称转换为英文
// 解决 Magistrala 平台对中文字符支持的兼容性问题
// 参数:
//   - clientName: 原始客户端名称（可能包含中文）
//
// 返回: 英文客户端名称
func translateClientNameToEnglish(clientName string) string {
	// 中文传感器名称到英文的映射表
	translations := map[string]string{
		"sensor-风力-":    "sensor-wind_force-",
		"sensor-风速-":    "sensor-wind_speed-",
		"sensor-风向-":    "sensor-wind_direction-",
		"sensor-土壤温度1-": "sensor-soil_temp_1-",
		"sensor-土壤水分1-": "sensor-soil_moisture_1-",
		"sensor-空气温度-":  "sensor-air_temperature-",
		"sensor-空气湿度-":  "sensor-air_humidity-",
		"sensor-CO2-":   "sensor-co2-",
		"sensor-大气压-":   "sensor-air_pressure-",
	}

	result := clientName
	// 逐个替换中文部分
	for chinese, english := range translations {
		result = strings.ReplaceAll(result, chinese, english)
	}
	return result
}

// translateUnitToEnglish 将中文单位转换为英文单位
// 确保 SenML 消消息中的单位字段使用标准英文表示
// 参数:
//   - unit: 原始单位（可能是中文）
//
// 返回: 英文单位
func translateUnitToEnglish(unit string) string {
	// 中文单位到英文单位的映射表
	unitTranslations := map[string]string{
		"级":   "level",     // 风力等级
		"m/s": "m_per_s",   // 米每秒
		"方向":  "direction", // 方向
		"℃":   "celsius",   // 摄氏度
		"%":   "percent",   // 百分比
		"PPM": "ppm",       // 百万分之一
		"Kpa": "kpa",       // 千帕
	}

	if english, exists := unitTranslations[unit]; exists {
		return english
	}
	return unit // 如果没有对应翻译，返回原单位
}

// translateTextToEnglish 将中文文本值转换为英文
// 主要用于翻译风向等描述性文本数据
// 参数:
//   - text: 原始文本值
//   - factorName: 因子名称（用于确定翻译策略）
//
// 返回: 英文文本值
func translateTextToEnglish(text, factorName string) string {
	// 风向描述的中英文映射
	windDirections := map[string]string{
		"东风":  "east",      // 东风
		"东南风": "southeast", // 东南风
		"南风":  "south",     // 南风
		"西南风": "southwest", // 西南风
		"西风":  "west",      // 西风
		"西北风": "northwest", // 西北风
		"北风":  "north",     // 北风
		"东北风": "northeast", // 东北风
	}

	// 如果是风向数据，进行专门的翻译
	if factorName == "风向" {
		if english, exists := windDirections[text]; exists {
			return english
		}
	}

	return text // 其他情况返回原文本
}

// isNumericString 检查字符串是否表示数值
// 用于判断文本数据是否应该作为数值处理
// 参数:
//   - s: 要检查的字符串
//
// 返回: true表示是数值字符串，false表示非数值字符串
func isNumericString(s string) bool {
	if s == "" {
		return false
	}

	// 尝试将字符串解析为浮点数
	_, err := strconv.ParseFloat(s, 64)
	return err == nil // 解析成功则认为是数值字符串
}
