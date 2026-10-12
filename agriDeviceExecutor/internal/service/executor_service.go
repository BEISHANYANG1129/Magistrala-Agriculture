package service

import (
	"fmt"
	"strconv"
	"strings"

	"agriDeviceExecutor/internal/data"
)

// ExecuteValveControl 通过 clientId 控制阀门开关（open=true 开；false 关）
func ExecuteValveControl(clientId string, open bool, token, baseURL string) error {
	// start := time.Now()
	// log.Printf("[debug][exec] start ExecuteValveControl clientId=%s open=%v", clientId, open)

	e, ok := data.GetEntryByClientId(clientId)
	if !ok {
		// log.Printf("[debug][exec] mapping miss clientId=%s cost=%s", clientId, time.Since(start))
		return fmt.Errorf("clientId 未找到映射: %s", clientId)
	}
	devAddr := strings.TrimSpace(e.DeviceAddr)
	factorId := fmt.Sprintf("%s_%d", devAddr, e.NodeId)
	mode := 0
	if open {
		mode = 1
	}
	modeStr := strconv.Itoa(mode)

	// log.Printf("[debug][exec] resolved devAddr=%s nodeId=%d factorId=%s modeStr=%s", devAddr, e.NodeId, factorId, modeStr)

	// callStart := time.Now()
	if err := ManualControlValve(token, baseURL, devAddr, factorId, modeStr); err != nil {
		// log.Printf("[debug][exec] ManualControlValve error=%v cost=%s callCost=%s",
		// 	err, time.Since(start), time.Since(callStart))
		return fmt.Errorf("手动控制失败: %w", err)
	}
	// log.Printf("[debug][exec] ManualControlValve ok callCost=%s totalCost=%s",
	// 	time.Since(callStart), time.Since(start))

	// 执行成功后更新映射状态与最近动作
	status := map[bool]string{true: "on", false: "off"}[open]
	last := map[bool]string{true: "open", false: "close"}[open]
	if err := data.UpdateEntryValue(devAddr, e.NodeId, status, last); err != nil {
		// 可选：warn 日志
		// log.Printf("[warn] UpdateEntryValue failed deviceAddr=%s nodeId=%d err=%v", devAddr, e.NodeId, err)
	}

	// log.Printf("[debug][exec] mapping updated clientId=%s status=%s finalCost=%s",
	// 	clientId, map[bool]string{true: "on", false: "off"}[open], time.Since(start))
	return nil
}

// ExecuteModeUpdate 通过 clientId 修改阀门工作模式（"1" 手动 / "2" 自动）。
func ExecuteModeUpdate(clientId string, mode string, token, baseURL string) error {
	if mode != "1" && mode != "2" {
		return fmt.Errorf("非法模式: %s", mode)
	}
	entry, ok := data.GetEntryByClientId(clientId)
	if !ok {
		return fmt.Errorf("clientId 未找到映射: %s", clientId)
	}
	err := UpdateFactorMode(token, baseURL, fmt.Sprint(entry.NodeId), mode)
	_ = data.AppendAudit(data.AuditRecord{
		Action:     "modeUpdate",
		ClientId:   clientId,
		DeviceAddr: entry.DeviceAddr,
		NodeId:     entry.NodeId,
		Success:    err == nil,
		Detail:     fmt.Sprintf("mode=%s err=%v", mode, err),
	})
	return err
}
