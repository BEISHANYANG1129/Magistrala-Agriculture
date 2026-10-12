package agridataintegration

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ClientStatus 客户端状态枚举
type ClientStatus string

const (
	StatusNotCreated ClientStatus = "not_created" // 未创建
	StatusCreated    ClientStatus = "created"     // 已创建但未连接
	StatusConnected  ClientStatus = "connected"   // 已创建且已连接
	StatusError      ClientStatus = "error"       // 创建或连接时出错
	StatusUnknown    ClientStatus = "unknown"     // 未知状态，需要检查
)

// SensorMapping 传感器映射条目
type SensorMapping struct {
	// 农业平台传感器信息
	DeviceAddr   int    `json:"deviceAddr"`   // 设备地址
	DeviceName   string `json:"deviceName"`   // 设备名称
	NodeID       int    `json:"nodeId"`       // 节点ID
	RegisterID   int    `json:"registerId"`   // 寄存器ID
	RegisterName string `json:"registerName"` // 寄存器名称
	FactorName   string `json:"factorName"`   // 因子名称
	Unit         string `json:"unit"`         // 单位

	// Magistrala 平台客户端信息
	ClientID     string `json:"clientId"`     // Magistrala 客户端ID
	ClientName   string `json:"clientName"`   // Magistrala 客户端名称
	ClientSecret string `json:"clientSecret"` // Magistrala 客户端密钥

	// 位置和分区信息
	Position struct {
		X float64 `json:"x"` // X坐标（百分比）
		Y float64 `json:"y"` // Y坐标（百分比）
	} `json:"position"`
	Partition string `json:"partition"` // 分区名称

	// 状态信息
	LastSync   int64  `json:"lastSync"`   // 最后同步时间
	IsActive   bool   `json:"isActive"`   // 是否激活
	LastValue  string `json:"lastValue"`  // 最后的值
	LastUpdate int64  `json:"lastUpdate"` // 最后更新时间

	// 新增状态字段
	Status        ClientStatus `json:"status"`        // 客户端状态
	StatusUpdated int64        `json:"statusUpdated"` // 状态更新时间
	ErrorMessage  string       `json:"errorMessage"`  // 错误信息
	RetryCount    int          `json:"retryCount"`    // 重试次数

	// 设备级别状态跟踪
	DeviceStatus   string `json:"deviceStatus"`   // normal, offline, unknown
	LastOnlineTime int64  `json:"lastOnlineTime"` // 最后在线时间
	OfflineCount   int    `json:"offlineCount"`   // 离线次数统计

	// 数据质量跟踪
	DataQuality  string `json:"dataQuality"`  // good, poor, no_data
	LastDataTime int64  `json:"lastDataTime"` // 最后收到数据时间
}

// MappingManager 映射管理器
type MappingManager struct {
	mappings map[string]*SensorMapping // key: DeviceAddr_NodeID_RegisterID
	mu       sync.RWMutex
	filePath string
}

// NewMappingManager 创建映射管理器
func NewMappingManager(filePath string) *MappingManager {
	mm := &MappingManager{
		mappings: make(map[string]*SensorMapping),
		filePath: filePath,
	}

	// 尝试加载现有映射
	mm.LoadFromFile()
	return mm
}

// generateKey 生成映射键
func generateKey(deviceAddr, nodeID, registerID int) string {
	return fmt.Sprintf("%d_%d_%d", deviceAddr, nodeID, registerID)
}

// AddMapping 添加映射
func (mm *MappingManager) AddMapping(mapping *SensorMapping) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	key := generateKey(mapping.DeviceAddr, mapping.NodeID, mapping.RegisterID)

	// 如果是新映射，初始化状态
	if mapping.Status == "" {
		if mapping.ClientID == "" {
			mapping.Status = StatusNotCreated
		} else {
			mapping.Status = StatusUnknown // 需要检查连接状态
		}
		mapping.StatusUpdated = time.Now().Unix()
	}

	mm.mappings[key] = mapping
}

// GetMapping 获取映射
func (mm *MappingManager) GetMapping(deviceAddr, nodeID, registerID int) (*SensorMapping, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	key := generateKey(deviceAddr, nodeID, registerID)
	mapping, exists := mm.mappings[key]
	return mapping, exists
}

// GetMappingByClientID 根据客户端ID获取映射
func (mm *MappingManager) GetMappingByClientID(clientID string) (*SensorMapping, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, mapping := range mm.mappings {
		if mapping.ClientID == clientID {
			return mapping, true
		}
	}
	return nil, false
}

// GetAllMappings 获取所有映射
func (mm *MappingManager) GetAllMappings() []*SensorMapping {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	mappings := make([]*SensorMapping, 0, len(mm.mappings))
	for _, mapping := range mm.mappings {
		mappings = append(mappings, mapping)
	}
	return mappings
}

// GetActiveMappings 获取所有激活的映射
func (mm *MappingManager) GetActiveMappings() []*SensorMapping {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var activeMappings []*SensorMapping
	for _, mapping := range mm.mappings {
		if mapping.IsActive {
			activeMappings = append(activeMappings, mapping)
		}
	}
	return activeMappings
}

// UpdateMapping 更新映射
func (mm *MappingManager) UpdateMapping(deviceAddr, nodeID, registerID int, updates func(*SensorMapping)) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	key := generateKey(deviceAddr, nodeID, registerID)
	if mapping, exists := mm.mappings[key]; exists {
		updates(mapping)
		return true
	}
	return false
}

// RemoveMapping 移除映射
func (mm *MappingManager) RemoveMapping(deviceAddr, nodeID, registerID int) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	key := generateKey(deviceAddr, nodeID, registerID)
	if _, exists := mm.mappings[key]; exists {
		delete(mm.mappings, key)
		return true
	}
	return false
}

// SaveToFile 保存映射到文件
func (mm *MappingManager) SaveToFile() error {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	data, err := json.MarshalIndent(mm.mappings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mm.filePath, data, 0644)
}

// LoadFromFile 从文件加载映射
func (mm *MappingManager) LoadFromFile() error {
	data, err := os.ReadFile(mm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建空映射文件
			return mm.SaveToFile()
		}
		return err
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	return json.Unmarshal(data, &mm.mappings)
}

// Count 获取映射数量
func (mm *MappingManager) Count() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return len(mm.mappings)
}

// GetMappingsByDevice 获取某个设备的所有映射
func (mm *MappingManager) GetMappingsByDevice(deviceAddr int) []*SensorMapping {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var deviceMappings []*SensorMapping
	for _, mapping := range mm.mappings {
		if mapping.DeviceAddr == deviceAddr {
			deviceMappings = append(deviceMappings, mapping)
		}
	}
	return deviceMappings
}

// GetPartitions 获取所有分区名称
func (mm *MappingManager) GetPartitions() []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	partitionSet := make(map[string]bool)
	for _, mapping := range mm.mappings {
		if mapping.Partition != "" {
			partitionSet[mapping.Partition] = true
		}
	}

	partitions := make([]string, 0, len(partitionSet))
	for partition := range partitionSet {
		partitions = append(partitions, partition)
	}
	return partitions
}

// UpdateMappingStatus 更新映射状态
func (mm *MappingManager) UpdateMappingStatus(deviceAddr, nodeID, registerID int, status ClientStatus, errorMsg string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	key := generateKey(deviceAddr, nodeID, registerID)
	if mapping, exists := mm.mappings[key]; exists {
		mapping.Status = status
		mapping.StatusUpdated = time.Now().Unix()
		mapping.ErrorMessage = errorMsg

		// 根据状态更新 IsActive
		mapping.IsActive = (status == StatusConnected)

		// 如果状态改为正常，清零重试次数
		if status == StatusConnected {
			mapping.RetryCount = 0
			mapping.ErrorMessage = ""
		}
	}
}

// IncrementRetryCount 增加重试次数
func (mm *MappingManager) IncrementRetryCount(deviceAddr, nodeID, registerID int) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	key := generateKey(deviceAddr, nodeID, registerID)
	if mapping, exists := mm.mappings[key]; exists {
		mapping.RetryCount++
	}
}

// GetMappingsByStatus 根据状态获取映射
func (mm *MappingManager) GetMappingsByStatus(statuses ...ClientStatus) []*SensorMapping {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var result []*SensorMapping
	statusMap := make(map[ClientStatus]bool)
	for _, status := range statuses {
		statusMap[status] = true
	}

	for _, mapping := range mm.mappings {
		if statusMap[mapping.Status] {
			result = append(result, mapping)
		}
	}

	return result
}

// GetStatusSummary 获取状态统计
func (mm *MappingManager) GetStatusSummary() map[ClientStatus]int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	summary := make(map[ClientStatus]int)
	for _, mapping := range mm.mappings {
		summary[mapping.Status]++
	}

	return summary
}

// MarkAsConnected 标记为已连接
func (mm *MappingManager) MarkAsConnected(mapping *SensorMapping) {
	mapping.Status = StatusConnected
	mapping.StatusUpdated = time.Now().Unix()
	mapping.IsActive = true
	mapping.LastSync = time.Now().Unix()
	mapping.ErrorMessage = ""
	mapping.RetryCount = 0
}

// MarkAsError 标记为错误状态
func (mm *MappingManager) MarkAsError(mapping *SensorMapping, errorMsg string) {
	mapping.Status = StatusError
	mapping.StatusUpdated = time.Now().Unix()
	mapping.IsActive = false
	mapping.ErrorMessage = errorMsg
	mapping.RetryCount++
}

// UpdateDeviceStatus 更新设备状态
func (mm *MappingManager) UpdateDeviceStatus(deviceAddr int, status string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, mapping := range mm.mappings {
		if mapping.DeviceAddr == deviceAddr {
			oldStatus := mapping.DeviceStatus
			mapping.DeviceStatus = status

			if status == "normal" && oldStatus != "normal" {
				// 设备上线
				mapping.LastOnlineTime = time.Now().Unix()
			} else if status != "normal" && oldStatus == "normal" {
				// 设备离线
				mapping.OfflineCount++
			}
		}
	}
}

// UpdateDataQuality 更新数据质量
func (mm *MappingManager) UpdateDataQuality(mapping *SensorMapping, quality string) {
	mapping.DataQuality = quality
	mapping.LastDataTime = time.Now().Unix()
}
