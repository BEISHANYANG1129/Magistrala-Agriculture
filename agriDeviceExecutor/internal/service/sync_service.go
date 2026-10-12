package service

import (
	"agriDeviceExecutor/internal/data"
	"fmt"
)

// SyncAll 遍历用户所有设备与其节点，确保映射存在。
// 规则：
// - device 列表来源：GetSysUserDevice
// - node 列表来源：GetDeviceNodeList
// - 节点级唯一，不再区分寄存器（registerId 移除）
// - 调用 data.EnsureEntry(deviceAddr,nodeId) 保障映射与注册
// 审计：新增映射 syncAdd；已有映射 syncSkip；错误 syncError。
func SyncAll(token, baseURL string) error {
	devices, err := GetSysUserDevice(token, baseURL, "", "")
	if err != nil {
		return fmt.Errorf("获取设备失败: %w", err)
	}
	for _, d := range devices {
		devAddr := fmt.Sprint(d["deviceAddr"])
		if devAddr == "" {
			continue
		}
		nodes, err := GetDeviceNodeList(token, baseURL, devAddr)
		if err != nil {
			_ = data.AppendAudit(data.AuditRecord{Action: "syncError", DeviceAddr: devAddr, Detail: err.Error(), Success: false})
			continue
		}
		for _, n := range nodes {
			nodeIdAny := n["nodeId"]
			if nodeIdAny == nil {
				continue
			}
			// nodeId 可能是数值型
			nodeIdInt := 0
			switch v := nodeIdAny.(type) {
			case float64:
				nodeIdInt = int(v)
			case int:
				nodeIdInt = v
			case string:
				// 若是字符串尝试转换
				fmt.Sscanf(v, "%d", &nodeIdInt)
			}
			if nodeIdInt <= 0 {
				continue
			}
			entry, err := data.EnsureEntry(devAddr, nodeIdInt)
			if err != nil {
				_ = data.AppendAudit(data.AuditRecord{Action: "syncError", DeviceAddr: devAddr, NodeId: nodeIdInt, Detail: err.Error(), Success: false})
				continue
			}
			_ = data.AppendAudit(data.AuditRecord{Action: "syncAdd", DeviceAddr: devAddr, NodeId: nodeIdInt, ClientId: entry.ClientId, Success: true})
		}
	}
	return nil
}
