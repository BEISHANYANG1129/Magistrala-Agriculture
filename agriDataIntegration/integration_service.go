package agridataintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// ================================
// 数据结构定义
// ================================

// IntegrationService 农业数据集成服务
// 核心功能：连接农业平台和Magistrala IoT平台，实现数据的实时同步
// 主要职责：
// 1. 传感器发现和映射管理
// 2. 实时数据同步和智能过滤
// 3. 设备状态监控和管理
// 4. 客户端创建和连接管理
type IntegrationService struct {
	// 核心组件
	config           *Config           // 系统配置信息
	agriClient       *PlatformService  // 农业平台客户端
	magistralaClient *MagistralaClient // Magistrala IoT平台客户端
	mappingManager   *MappingManager   // 传感器映射管理器

	// 运行控制
	ctx       context.Context    // 上下文控制器，用于优雅停止
	cancel    context.CancelFunc // 取消函数
	wg        sync.WaitGroup     // 协程等待组
	isRunning bool               // 服务运行状态标志
	mu        sync.RWMutex       // 读写锁，保护并发访问

	// 统计信息
	stats struct {
		TotalSensors   int    `json:"total_sensors"`        // 总传感器数量
		ActiveMappings int    `json:"active_mappings"`      // 活跃映射数量
		MessagesSent   int64  `json:"messages_sent"`        // 已发送消息总数
		LastSync       int64  `json:"last_sync"`            // 最后同步时间戳
		LastError      string `json:"last_error,omitempty"` // 最后错误信息
		SyncErrors     int64  `json:"sync_errors"`          // 同步错误计数
	}
}

// ================================
// 服务初始化和生命周期管理
// ================================

// NewIntegrationService 创建集成服务实例
// 初始化所有必要的组件并建立连接
// 参数:
//   - config: 系统配置对象，包含农业平台和Magistrala平台的连接信息
//
// 返回: 集成服务实例和可能的错误
func NewIntegrationService(config *Config) (*IntegrationService, error) {
	log.Println("🚀 初始化农业数据集成服务...")

	// 1. 创建农业平台客户端
	agriClient := NewPlatformService(config.AgriPlatform.BaseURL)
	log.Printf("   ✅ 农业平台客户端已创建: %s", config.AgriPlatform.BaseURL)

	// 2. 登录农业平台获取访问令牌
	token, err := agriClient.Login(config.AgriPlatform.Username, config.AgriPlatform.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to login to agri platform: %w", err)
	}
	log.Printf("   ✅ 农业平台登录成功，令牌: %s...", token[:20])

	// 3. 从共享配置读取 baseUrl/userToken，创建 Magistrala IoT 平台客户端
	type sharedMagCfg struct {
		BaseURL   string `json:"baseUrl"`
		UserToken string `json:"userToken"`
	}
	tryLoadSharedMag := func() *sharedMagCfg {
		candidates := []string{
			"data/magistrala.json",
			"../data/magistrala.json",
			"../../data/magistrala.json",
		}
		for _, p := range candidates {
			if b, err := os.ReadFile(p); err == nil {
				var out sharedMagCfg
				if json.Unmarshal(b, &out) == nil {
					return &out
				}
			}
		}
		return nil
	}
	sm := tryLoadSharedMag()
	if sm == nil || sm.BaseURL == "" || sm.UserToken == "" {
		return nil, fmt.Errorf("共享配置缺失 baseUrl/userToken，请编辑 data/magistrala.json")
	}

	magistralaClient := NewMagistralaClient(sm.BaseURL, sm.UserToken, config.Magistrala.ChannelPort, config.Magistrala.ClientPort, config.Magistrala.MessagePort)
	log.Printf("   ✅ Magistrala客户端已创建: %s (messagePort=%s, channelPort=%s, clientPort=%s)", sm.BaseURL, config.Magistrala.MessagePort, config.Magistrala.ChannelPort, config.Magistrala.ClientPort)

	// 4. 创建传感器映射管理器
	mappingManager := NewMappingManager(config.Integration.MappingFile)
	log.Printf("   ✅ 映射管理器已创建，映射文件: %s", config.Integration.MappingFile)

	// 5. 创建上下文控制器用于优雅停止
	ctx, cancel := context.WithCancel(context.Background())

	log.Println("   🎉 所有组件初始化完成")
	return &IntegrationService{
		config:           config,
		agriClient:       agriClient,
		magistralaClient: magistralaClient,
		mappingManager:   mappingManager,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// Start 启动集成服务
// 执行传感器发现、映射创建，并启动数据同步循环
// 特点：
// 1. 线程安全的启动检查
// 2. 传感器预发现策略
// 3. 异步数据同步循环
// 返回: 错误信息
func (is *IntegrationService) Start() error {
	is.mu.Lock()
	defer is.mu.Unlock()

	// 检查服务是否已经在运行
	if is.isRunning {
		return fmt.Errorf("integration service is already running")
	}

	log.Println("🚀 启动农业数据集成服务...")

	// 标记服务为运行状态
	is.isRunning = true

	// 1. 初始化阶段：发现传感器并创建映射关系
	log.Println("   📡 开始传感器发现和映射创建...")
	if err := is.discoverAndMapSensors(); err != nil {
		log.Printf("   ⚠️ 传感器发现警告: %v", err)
		// 不返回错误，允许服务继续运行
	}

	// 2. 启动数据同步循环协程
	log.Println("   🔄 启动数据同步循环...")
	is.wg.Add(1)
	go is.dataSyncLoop()

	log.Println("✅ 农业数据集成服务启动成功")
	return nil
}

// Stop 优雅停止集成服务
// 执行清理工作：
// 1. 停止数据同步循环
// 2. 等待所有协程安全退出
// 3. 保存映射状态到文件
func (is *IntegrationService) Stop() {
	is.mu.Lock()
	defer is.mu.Unlock()

	if !is.isRunning {
		log.Println("Integration service is not running")
		return
	}

	log.Println("🛑 停止农业数据集成服务...")

	// 1. 取消上下文，通知所有协程退出
	is.cancel()

	// 2. 等待所有协程安全退出
	is.wg.Wait()

	// 3. 标记服务为停止状态
	is.isRunning = false

	// 4. 保存映射状态到文件
	if err := is.mappingManager.SaveToFile(); err != nil {
		log.Printf("   ❌ 保存映射失败: %v", err)
	} else {
		log.Println("   ✅ 映射状态已保存")
	}

	log.Println("✅ 农业数据集成服务已停止")
}

// ================================
// 传感器发现和映射管理
// ================================

// discoverAndMapSensors 智能传感器发现和映射创建（优化版本）
// 核心功能：
// 1. 从农业平台发现所有可用传感器
// 2. 为每个传感器创建Magistrala客户端映射
// 3. 智能状态管理和批量操作
// 4. 错误重试和状态恢复
// 特点：
// - 预创建策略：不管设备是否在线都创建映射
// - 批量处理：提高创建和连接效率
// - 智能重试：错误状态自动重试
// 返回: 错误信息
func (is *IntegrationService) discoverAndMapSensors() error {
	log.Println("📡 从农业平台发现传感器...")

	// 1. 获取农业平台设备列表
	devices, err := is.agriClient.GetDeviceList("")
	if err != nil {
		return fmt.Errorf("failed to get device list: %w", err)
	}

	log.Printf("   📱 发现设备数量: %d", len(devices))
	is.stats.TotalSensors = 0

	// 2. 传感器状态分类收集
	var needCreate []*SensorMapping       // 需要创建客户端的传感器
	var needConnect []*SensorMapping      // 需要连接频道的传感器
	var needCheck []*SensorMapping        // 需要重新检查的传感器
	var alreadyConnected []*SensorMapping // 已经连接完成的传感器

	// 3. 遍历所有设备和传感器因子
	for _, device := range devices {
		log.Printf("   🔍 处理设备: %s (地址: %d)", device.DeviceName, device.DeviceAddr)

		for _, factor := range device.Factors {
			// 跳过未启用的因子
			if !factor.Enabled {
				log.Printf("     ⚠️ 跳过未启用因子: %s", factor.FactorName)
				continue
			}

			is.stats.TotalSensors++

			// 4. 检查映射是否已存在
			mapping, exists := is.mappingManager.GetMapping(device.DeviceAddr, factor.NodeId, factor.RegisterId)
			if !exists {
				// 创建新的映射 - 预创建策略：不管设备是否在线都创建
				mapping = &SensorMapping{
					// 基础设备信息
					DeviceAddr:   device.DeviceAddr,
					DeviceName:   device.DeviceName,
					NodeID:       factor.NodeId,
					RegisterID:   factor.RegisterId,
					RegisterName: factor.FactorName,
					FactorName:   factor.FactorName,
					Unit:         factor.Unit,

					// 状态信息
					Status:        StatusNotCreated,
					StatusUpdated: time.Now().Unix(),
					IsActive:      false,

					// 设备状态管理
					DeviceStatus: "unknown", // 初始设备状态未知
					DataQuality:  "no_data", // 初始无数据质量评估
				}

				is.mappingManager.AddMapping(mapping)
				log.Printf("     ✅ 创建映射: %s (设备:%d 节点:%d 寄存器:%d)",
					mapping.FactorName, mapping.DeviceAddr, mapping.NodeID, mapping.RegisterID)
			}

			// 5. 根据当前状态进行智能分类
			switch mapping.Status {
			case StatusNotCreated:
				needCreate = append(needCreate, mapping)
			case StatusCreated:
				needConnect = append(needConnect, mapping)
			case StatusConnected:
				alreadyConnected = append(alreadyConnected, mapping)
			case StatusError, StatusUnknown:
				// 错误状态或未知状态需要重新检查
				if mapping.RetryCount < 3 { // 最多重试3次
					needCheck = append(needCheck, mapping)
					log.Printf("     🔄 安排重试: %s (第%d次)", mapping.FactorName, mapping.RetryCount+1)
				} else {
					log.Printf("     ❌ 传感器 %s 重试次数已达上限，跳过处理", mapping.FactorName)
				}
			}
		}
	}

	// 6. 输出传感器状态分类统计
	log.Printf("📊 传感器状态统计:")
	log.Printf("   • 需要创建客户端: %d 个", len(needCreate))
	log.Printf("   • 需要连接频道: %d 个", len(needConnect))
	log.Printf("   • 需要重新检查: %d 个", len(needCheck))
	log.Printf("   • 已经连接就绪: %d 个", len(alreadyConnected))

	// 7. 批量创建客户端处理
	if len(needCreate) > 0 {
		log.Printf("🔨 开始创建 %d 个新客户端...", len(needCreate))
		for i, mapping := range needCreate {
			log.Printf("   创建进度 %d/%d: %s", i+1, len(needCreate), mapping.FactorName)

			if err := is.createSensorClient(mapping); err != nil {
				// 创建失败，标记错误状态
				is.mappingManager.MarkAsError(mapping, err.Error())
				log.Printf("   ❌ 创建失败: %v", err)
				continue
			}

			// 创建成功，标记为已创建但未连接状态
			mapping.Status = StatusCreated
			mapping.StatusUpdated = time.Now().Unix()
			needConnect = append(needConnect, mapping)
			log.Printf("   ✅ 创建成功: %s", mapping.ClientID)
		}
	}

	// 8. 批量连接客户端到频道处理
	allToConnect := append(needConnect, needCheck...)
	if len(allToConnect) > 0 {
		log.Printf("🔗 开始连接 %d 个客户端到频道...", len(allToConnect))

		if err := is.batchConnectSensors(allToConnect); err != nil {
			log.Printf("   ❌ 批量连接失败: %v", err)
			// 标记所有为错误状态
			for _, mapping := range allToConnect {
				is.mappingManager.MarkAsError(mapping, err.Error())
			}
		} else {
			log.Printf("   ✅ 批量连接成功")
			// 标记所有为已连接状态
			for _, mapping := range allToConnect {
				is.mappingManager.MarkAsConnected(mapping)
			}
		}
	}

	// 9. 保存映射状态到文件
	if err := is.mappingManager.SaveToFile(); err != nil {
		log.Printf("   ⚠️ 保存映射失败: %v", err)
	}

	// 10. 更新统计信息
	summary := is.mappingManager.GetStatusSummary()
	is.stats.ActiveMappings = summary[StatusConnected]

	// 11. 输出最终发现和映射统计
	log.Printf("✅ 传感器发现和映射完成:")
	log.Printf("   • 总传感器数量: %d", is.stats.TotalSensors)
	log.Printf("   • 已连接可用: %d", summary[StatusConnected])
	log.Printf("   • 已创建待连接: %d", summary[StatusCreated])
	log.Printf("   • 错误状态: %d", summary[StatusError])

	return nil
}

// assignPartitionAndPosition 为传感器分配分区和位置信息
// 委托给Magistrala客户端处理具体的分区分配逻辑
// 参数:
//   - mapping: 传感器映射对象（将被更新分区信息）
//
// 返回: 错误信息
func (is *IntegrationService) assignPartitionAndPosition(mapping *SensorMapping) error {
	return is.magistralaClient.AssignPartitionPosition(
		is.config.Magistrala.DomainID,
		is.config.Magistrala.ChannelID,
		mapping)
}

// ================================
// 数据同步功能
// ================================

// dataSyncLoop 数据同步循环协程
// 定时从农业平台获取实时数据并同步到Magistrala平台
// 特点：
// 1. 可配置的同步间隔
// 2. 优雅的停止机制
// 3. 错误统计和恢复
// 4. 协程安全退出
func (is *IntegrationService) dataSyncLoop() {
	defer is.wg.Done()

	// 创建定时器
	ticker := time.NewTicker(time.Duration(is.config.Integration.SyncInterval) * time.Second)
	defer ticker.Stop()

	log.Printf("🔄 数据同步循环已启动，同步间隔: %d 秒", is.config.Integration.SyncInterval)

	for {
		select {
		case <-is.ctx.Done():
			// 收到停止信号，优雅退出
			log.Println("🛑 数据同步循环已停止")
			return

		case <-ticker.C:
			// 定时同步触发
			if err := is.syncData(); err != nil {
				log.Printf("❌ 数据同步错误: %v", err)
				is.stats.SyncErrors++
				is.stats.LastError = err.Error()
			} else {
				// 同步成功，更新统计
				is.stats.LastSync = time.Now().Unix()
				is.stats.LastError = ""
			}
		}
	}
}

// syncData 智能数据同步 - 只同步在线设备的数据
// 核心数据同步逻辑，实现智能过滤和高效同步
// 特点：
// 1. 设备状态感知：只处理在线设备
// 2. 智能过滤：跳过离线设备数据
// 3. 批量处理：高效处理设备数据
// 4. 状态更新：实时更新设备和传感器状态
// 5. 详细统计：提供同步过程的详细信息
// 返回: 错误信息
func (is *IntegrationService) syncData() error {
	// 1. 获取所有设备的实时数据（包含设备状态信息）
	realTimeData, err := is.agriClient.GetRealTimeData("")
	if err != nil {
		return fmt.Errorf("failed to get real time data: %w", err)
	}

	log.Printf("📊 获取到 %d 个设备的实时数据", len(realTimeData))

	// 2. 初始化同步统计计数器
	deviceOnlineCount := 0  // 在线设备计数
	deviceOfflineCount := 0 // 离线设备计数
	messageSentCount := 0   // 成功发送消息计数

	// 3. 处理每个设备的数据
	for _, deviceData := range realTimeData {
		// 4. 智能设备状态检查 - 同步优化的核心逻辑
		if deviceData.DeviceStatus != "normal" {
			// 设备离线，更新设备状态但跳过数据同步
			log.Printf("🔴 设备 %s (地址:%d) 离线 (状态:%s)，跳过数据同步",
				deviceData.DeviceName, deviceData.DeviceAddr, deviceData.DeviceStatus)

			// 更新设备下所有传感器的离线状态
			is.updateDeviceStatus(deviceData.DeviceAddr, deviceData.DeviceStatus)
			deviceOfflineCount++
			continue
		}

		// 5. 设备在线，开始处理数据同步
		log.Printf("🟢 设备 %s (地址:%d) 在线，开始同步数据",
			deviceData.DeviceName, deviceData.DeviceAddr)

		// 更新设备状态为在线
		is.updateDeviceStatus(deviceData.DeviceAddr, "normal")
		deviceOnlineCount++

		// 6. 处理设备的所有数据项
		for _, dataItem := range deviceData.DataItem {
			for _, registerItem := range dataItem.RegisterItem {
				// 7. 查找对应的传感器映射
				mapping, exists := is.mappingManager.GetMapping(
					deviceData.DeviceAddr,
					dataItem.NodeId,
					registerItem.RegisterId)

				if !exists {
					log.Printf("   ⚠️ 未找到映射: 设备%d 节点%d 寄存器%d",
						deviceData.DeviceAddr, dataItem.NodeId, registerItem.RegisterId)
					continue
				}

				// 8. 检查传感器连接状态
				if mapping.Status != StatusConnected {
					log.Printf("   ⚠️ 传感器 %s 未连接 (状态:%s)，跳过数据发送",
						mapping.FactorName, mapping.Status)
					continue
				}

				// 9. 发送传感器数据到Magistrala平台
				if err := is.sendSensorData(mapping, &registerItem, &deviceData); err != nil {
					log.Printf("   ❌ 发送数据失败 - 传感器:%s 错误:%v", mapping.FactorName, err)
					// 标记数据质量为差
					is.mappingManager.UpdateDataQuality(mapping, "poor")
					continue
				}

				// 10. 发送成功，更新映射状态和统计
				messageSentCount++
				is.stats.MessagesSent++

				// 更新传感器的数据状态和时间戳
				is.mappingManager.UpdateMapping(deviceData.DeviceAddr, dataItem.NodeId, registerItem.RegisterId,
					func(m *SensorMapping) {
						m.LastValue = registerItem.Data
						m.LastUpdate = time.Now().Unix()
						m.LastSync = time.Now().Unix()
					})

				// 标记数据质量为良好
				is.mappingManager.UpdateDataQuality(mapping, "good")

				log.Printf("   ✅ 数据已发送 - 传感器:%s 数值:%s %s",
					mapping.FactorName, registerItem.Data, registerItem.Unit)
			}
		}
	}

	// 11. 输出本轮同步的详细统计信息
	log.Printf("📊 数据同步完成:")
	log.Printf("   • 在线设备: %d 个", deviceOnlineCount)
	log.Printf("   • 离线设备: %d 个", deviceOfflineCount)
	log.Printf("   • 发送消息: %d 条", messageSentCount)

	return nil
}

// sendSensorData 发送单个传感器数据到Magistrala平台
// 构造SenML格式的消息载荷并发送到指定频道
// 参数:
//   - mapping: 传感器映射信息（包含客户端认证信息）
//   - register: 寄存器数据项（包含实际传感器数值）
//   - deviceData: 设备实时数据（包含时间戳和位置信息）
//
// 返回: 错误信息
func (is *IntegrationService) sendSensorData(mapping *SensorMapping, register *RegisterItem, deviceData *RealTimeData) error {
	// 1. 构造消息载荷，包含完整的传感器信息
	payload := &MessagePayload{
		// 时间戳信息
		Timestamp: deviceData.TimeStamp, // 使用设备数据的时间戳

		// 设备基础信息
		DeviceAddr:   mapping.DeviceAddr,
		DeviceName:   mapping.DeviceName,
		NodeID:       mapping.NodeID,
		RegisterID:   mapping.RegisterID,
		RegisterName: mapping.RegisterName,

		// 传感器信息
		FactorName: mapping.FactorName,
		ClientName: mapping.ClientName, // 重要：用于SenML格式的bn字段

		// 数据值信息
		Value: register.Value, // 数值型数据
		Text:  register.Data,  // 文本型数据
		Unit:  register.Unit,  // 数据单位

		// 报警信息
		AlarmLevel: register.AlarmLevel, // 报警级别
		AlarmInfo:  register.AlarmInfo,  // 报警描述

		// 地理位置信息
		Latitude:  deviceData.Lat, // 纬度
		Longitude: deviceData.Lng, // 经度
	}

	// 2. 发送消息到Magistrala平台
	if err := is.magistralaClient.SendMessage(
		is.config.Magistrala.DomainID,
		is.config.Magistrala.ChannelID,
		mapping.ClientSecret,
		payload); err != nil {
		return fmt.Errorf("failed to send message to Magistrala: %w", err)
	}

	// 3. 更新映射统计信息
	mapping.LastValue = register.Data
	mapping.LastUpdate = deviceData.TimeStamp
	mapping.LastDataTime = time.Now().Unix()
	mapping.DataQuality = "good"

	// 4. 更新全局统计
	is.stats.MessagesSent++
	return nil
}

// ================================
// 客户端管理功能
// ================================

// createSensorClient 为单个传感器创建Magistrala客户端
// 包括分区分配和客户端创建的完整流程
// 参数:
//   - mapping: 传感器映射对象（将被更新客户端信息）
//
// 返回: 错误信息
func (is *IntegrationService) createSensorClient(mapping *SensorMapping) error {
	// 1. 分配分区和位置信息
	if err := is.assignPartitionAndPosition(mapping); err != nil {
		log.Printf("   ⚠️ 传感器 %s 分区分配失败: %v，使用默认分区", mapping.FactorName, err)
		// 使用默认分区作为后备方案
		mapping.Partition = is.config.Integration.DefaultPartition
	}

	// 2. 创建Magistrala客户端
	client, err := is.magistralaClient.CreateMagistralaClientFromSensor(
		is.config.Magistrala.DomainID, mapping)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// 3. 更新映射中的客户端信息
	mapping.ClientID = client.ID
	mapping.ClientName = client.Name
	if secret, ok := client.Credentials["secret"].(string); ok {
		mapping.ClientSecret = secret
	}

	log.Printf("   ✅ 客户端创建成功: %s -> %s", mapping.FactorName, client.ID)
	return nil
}

// batchConnectSensors 批量连接传感器客户端到频道
// 高效处理多个客户端的频道连接操作
// 参数:
//   - mappings: 需要连接的传感器映射列表
//
// 返回: 错误信息
func (is *IntegrationService) batchConnectSensors(mappings []*SensorMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	// 1. 收集需要连接的有效客户端ID
	var clientIDs []string
	for _, mapping := range mappings {
		if mapping.ClientID != "" {
			clientIDs = append(clientIDs, mapping.ClientID)
		}
	}

	if len(clientIDs) == 0 {
		return fmt.Errorf("no valid client IDs to connect")
	}

	// 2. 逐个连接客户端到频道
	// 注：未来可以优化为真正的批量API调用
	for _, mapping := range mappings {
		if mapping.ClientID != "" {
			if err := is.magistralaClient.ConnectToChannel(
				is.config.Magistrala.DomainID,
				mapping.ClientID,
				is.config.Magistrala.ChannelID); err != nil {
				return fmt.Errorf("failed to connect client %s: %w", mapping.ClientID, err)
			}
			log.Printf("   ✅ 客户端 %s 已连接到频道", mapping.ClientID)
		}
	}

	return nil
}

// ================================
// 设备状态管理
// ================================

// updateDeviceStatus 更新设备及其所有传感器的状态
// 智能处理设备上线/离线事件，更新相关传感器状态
// 参数:
//   - deviceAddr: 设备地址
//   - status: 新的设备状态（"normal"表示在线，其他表示离线）
func (is *IntegrationService) updateDeviceStatus(deviceAddr int, status string) {
	// 1. 获取该设备下的所有传感器映射
	mappings := is.mappingManager.GetMappingsByDevice(deviceAddr)

	// 2. 更新每个传感器映射的设备状态
	for _, mapping := range mappings {
		oldStatus := mapping.DeviceStatus
		mapping.DeviceStatus = status

		// 3. 处理设备状态变化事件
		if status == "normal" && oldStatus != "normal" {
			// 设备上线事件
			mapping.LastOnlineTime = time.Now().Unix()
			log.Printf("   📍 设备 %d 上线，传感器 %s 可以接收数据", deviceAddr, mapping.FactorName)
		} else if status != "normal" && oldStatus == "normal" {
			// 设备离线事件
			mapping.OfflineCount++
			log.Printf("   📍 设备 %d 离线，传感器 %s 暂停数据接收", deviceAddr, mapping.FactorName)
		}
	}

	// 4. 在映射管理器中批量更新设备状态
	is.mappingManager.UpdateDeviceStatus(deviceAddr, status)
}

// ================================
// 查询和管理接口
// ================================

// GetStats 获取集成服务的详细统计信息
// 提供服务运行状态的完整视图，用于监控和管理
// 返回: 包含各项统计指标的映射
func (is *IntegrationService) GetStats() map[string]any {
	is.mu.RLock()
	defer is.mu.RUnlock()

	return map[string]any{
		"total_sensors":   is.stats.TotalSensors,   // 总传感器数量
		"active_mappings": is.stats.ActiveMappings, // 活跃映射数量
		"messages_sent":   is.stats.MessagesSent,   // 已发送消息总数
		"last_sync":       is.stats.LastSync,       // 最后同步时间戳
		"last_error":      is.stats.LastError,      // 最后错误信息
		"sync_errors":     is.stats.SyncErrors,     // 同步错误计数
		"is_running":      is.isRunning,            // 服务运行状态
	}
}

// GetMappings 获取所有传感器映射
// 返回当前系统中所有传感器的映射信息，用于管理界面显示
// 返回: 传感器映射列表
func (is *IntegrationService) GetMappings() []*SensorMapping {
	return is.mappingManager.GetAllMappings()
}

// RefreshSensors 手动触发传感器重新发现
// 提供手动刷新功能，用于管理界面的按需更新
// 返回: 错误信息
func (is *IntegrationService) RefreshSensors() error {
	log.Println("🔄 手动触发传感器重新发现...")
	return is.discoverAndMapSensors()
}
