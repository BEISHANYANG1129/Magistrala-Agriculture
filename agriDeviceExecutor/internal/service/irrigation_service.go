package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ControlIrrigationNode 控制指定设备的某个节点。
//
// 合同（Contract）：
// - 输入：deviceAddr（必填）、nodeID（>0）、action ∈ {"open","close"}
// - 成功：返回 nil（表示已接受/执行）
// - 失败：参数非法或后端失败
//
// 集成说明：
// - 真实环境请调用外部“控制”接口（POST）。
// - 考虑幂等性与设备应答的确认机制。
// - 建议为控制操作增加审计日志。
func ControlIrrigationNode(deviceAddr string, nodeID int, action string) error {
	if deviceAddr == "" || nodeID <= 0 || (action != "open" && action != "close") {
		return fmt.Errorf("控制参数非法")
	}
	// 示例：假定控制成功
	return nil
}

// ManualControlValve 对接云平台“手动开启关闭阀门”接口。
// 文档：GET /api/v2.0/irrigation/valveOperatingMode/manualControlValve
// Header: token（从本地凭据读取）
// Query: deviceAddr, factorId, mode(0|1)
func ManualControlValve(token, baseURL, deviceAddr, factorId, mode string) error {
	if deviceAddr == "" || factorId == "" || (mode != "0" && mode != "1") {
		return fmt.Errorf("参数非法: deviceAddr/factorId/mode 必填，mode=0或1")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础地址失败: %w", err)
	}
	// 文档 8.10：/api/v2.0/irrigation/node/manualControlValve（GET）
	u.Path = "/api/v2.0/irrigation/node/manualControlValve"
	q := u.Query()
	q.Set("deviceAddr", deviceAddr)
	q.Set("factorId", factorId)
	q.Set("mode", mode)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &w)
		if w.Code != 0 || w.Message != "" {
			return fmt.Errorf("手动阀门失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return fmt.Errorf("手动阀门失败 HTTP %d, body=%s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return errors.New(wrapper.Message)
	}
	return nil
}

// GetIrrigationDeviceDetails 批量获取灌溉设备详情（最多 5 个）。
// 文档：GET /api/v2.0/irrigation/device/getDeviceIii
// Header: token（内部读取） Query: devAddr（英文逗号分隔，最多 5 个）
// 成功返回：[]map[string]any
func GetIrrigationDeviceDetails(token, baseURL, devAddr string) ([]map[string]any, error) {
	devAddr = strings.TrimSpace(devAddr)
	if devAddr == "" {
		return nil, fmt.Errorf("devAddr 不能为空")
	}
	parts := strings.Split(devAddr, ",")
	if len(parts) > 5 {
		return nil, fmt.Errorf("最多同时查询 5 个设备")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析基础地址失败: %w", err)
	}
	// 文档 8.1：/api/v2.0/irrigation/node/getDeviceIii（GET）
	u.Path = "/api/v2.0/irrigation/node/getDeviceIii"
	q := u.Query()
	q.Set("devAddr", devAddr)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("批量设备详情失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("批量设备详情失败 HTTP %d, body=%s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return nil, errors.New(wrapper.Message)
	}

	var devices []map[string]any
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &devices); err != nil {
			return nil, fmt.Errorf("解析设备详情列表失败: %w", err)
		}
	}
	if devices == nil {
		devices = []map[string]any{}
	}
	return devices, nil
}

// UpdateIrrigationDeviceInfo 修改设备信息（8.2）
// 文档：POST /api/v2.0/irrigation/device/updateDevInfo
// Body: JSON，包含 deviceAddr 及若干可选字段
func UpdateIrrigationDeviceInfo(token, baseURL string, payload map[string]any) error {
	if payload == nil || strings.TrimSpace(fmt.Sprint(payload["deviceAddr"])) == "" {
		return fmt.Errorf("deviceAddr 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/device/updateDevInfo"
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return fmt.Errorf("修改设备失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return fmt.Errorf("修改设备失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
	}
	_ = json.Unmarshal(b, &wrapper)
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return errors.New(wrapper.Message)
	}
	return nil
}

// GetDeviceNodeList 获取节点列表（8.3）
// 文档：GET /api/v2.0/irrigation/node/getDeviceNodeList?devAddr=...
func GetDeviceNodeList(token, baseURL, devAddr string) ([]map[string]any, error) {
	devAddr = strings.TrimSpace(devAddr)
	if devAddr == "" {
		return nil, fmt.Errorf("devAddr 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/node/getDeviceNodeList"
	q := u.Query()
	q.Set("devAddr", devAddr)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("获取节点失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("获取节点失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
		Data    json.RawMessage
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return nil, errors.New(wrapper.Message)
	}
	var nodes []map[string]any
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &nodes); err != nil {
			return nil, fmt.Errorf("解析节点列表失败: %w", err)
		}
	}
	if nodes == nil {
		nodes = []map[string]any{}
	}
	return nodes, nil
}

// UpdateDeviceNode 修改节点信息（8.4）
// 文档：POST /api/v2.0/irrigation/node/updateDeviceNode（Body: JSON）
func UpdateDeviceNode(token, baseURL string, payload map[string]any) error {
	if payload == nil || strings.TrimSpace(fmt.Sprint(payload["deviceAddr"])) == "" || fmt.Sprint(payload["nodeId"]) == "" {
		return fmt.Errorf("deviceAddr 与 nodeId 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/node/updateDeviceNode"
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return fmt.Errorf("修改节点失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return fmt.Errorf("修改节点失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
	}
	_ = json.Unmarshal(b, &wrapper)
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return errors.New(wrapper.Message)
	}
	return nil
}

// BatchNodeEnable 批量开关节点（8.5）
// 文档：POST /api/v2.0/irrigation/node/batchNodeEnable
func BatchNodeEnable(token, baseURL, devAddr, enable, factorType string) error {
	devAddr = strings.TrimSpace(devAddr)
	if devAddr == "" || (enable != "0" && enable != "1") {
		return fmt.Errorf("devAddr 与 enable(0|1) 必填")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/node/batchNodeEnable"
	payload := map[string]any{"devAddr": devAddr, "enable": enable}
	if strings.TrimSpace(factorType) != "" {
		payload["factorType"] = factorType
	}
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return fmt.Errorf("批量使能失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return fmt.Errorf("批量使能失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
	}
	_ = json.Unmarshal(b, &wrapper)
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return errors.New(wrapper.Message)
	}
	return nil
}

// GetIrrigationFactorRegulating 获取节点遥调信息（8.6）
// 文档：GET /api/v2.0/irrigation/factor/getIrrigationFactorRegulating?factorId=...
func GetIrrigationFactorRegulating(token, baseURL, factorId string) ([]map[string]any, error) {
	factorId = strings.TrimSpace(factorId)
	if factorId == "" {
		return nil, fmt.Errorf("factorId 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/factor/getIrrigationFactorRegulating"
	q := u.Query()
	q.Set("factorId", factorId)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("获取遥调失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("获取遥调失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
		Data    json.RawMessage
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return nil, errors.New(wrapper.Message)
	}
	var items []map[string]any
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &items); err != nil {
			return nil, fmt.Errorf("解析遥调列表失败: %w", err)
		}
	}
	if items == nil {
		items = []map[string]any{}
	}
	return items, nil
}

// ReplaceTbIrrigationFactorRegulating 更新节点遥调信息（8.7，删除原有重新添加）
// 文档：POST /api/v2.0/irrigation/factor/replaceTbIrrigationFactorRegulating
type RegulatingItem struct {
	FactorId     string `json:"factorId"`
	RegularValue int    `json:"regularValue"`
	RegularText  string `json:"regularText"`
	AlarmLevel   int    `json:"alarmLevel"`
}

func ReplaceTbIrrigationFactorRegulating(token, baseURL string, items []RegulatingItem) error {
	if len(items) == 0 {
		return fmt.Errorf("listTbIrrigationFactorRegulating 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/factor/replaceTbIrrigationFactorRegulating"
	payload := map[string]any{"listTbIrrigationFactorRegulating": items}
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return fmt.Errorf("更新遥调失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return fmt.Errorf("更新遥调失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
	}
	_ = json.Unmarshal(b, &wrapper)
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return errors.New(wrapper.Message)
	}
	return nil
}

// GetHistoryDataList 历史记录（8.8）
// 文档：GET /api/v2.0/irrigation/node/getHistoryDataList
func GetHistoryDataList(token, baseURL, deviceAddr, startTime, endTime string, pages, limit int, nodeId string) (map[string]any, error) {
	if strings.TrimSpace(deviceAddr) == "" || strings.TrimSpace(startTime) == "" || strings.TrimSpace(endTime) == "" || pages <= 0 || limit <= 0 {
		return nil, fmt.Errorf("deviceAddr/startTime/endTime/pages/limit 必填且有效")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/node/getHistoryDataList"
	q := u.Query()
	q.Set("deviceAddr", deviceAddr)
	q.Set("startTime", startTime)
	q.Set("endTime", endTime)
	q.Set("pages", fmt.Sprint(pages))
	q.Set("limit", fmt.Sprint(limit))
	if strings.TrimSpace(nodeId) != "" {
		q.Set("nodeId", nodeId)
	}
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("获取历史失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("获取历史失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
		Data    json.RawMessage
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return nil, errors.New(wrapper.Message)
	}
	var result map[string]any
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &result); err != nil {
			return nil, fmt.Errorf("解析历史数据失败: %w", err)
		}
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

// UpdateFactorMode 修改阀门工作模式（8.9）
// 文档：POST /api/v2.0/irrigation/factor/updateFactorMode Body: {factorId, mode}
func UpdateFactorMode(token, baseURL, factorId, mode string) error {
	if strings.TrimSpace(factorId) == "" || (mode != "1" && mode != "2") {
		return fmt.Errorf("factorId 不能为空，mode 取值 1(手动)/2(自动)")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/irrigation/factor/updateFactorMode"
	payload := map[string]any{"factorId": factorId, "mode": mode}
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int
			Message string
		}
		_ = json.Unmarshal(b, &w)
		if w.Code != 0 || w.Message != "" {
			return fmt.Errorf("修改模式失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return fmt.Errorf("修改模式失败 HTTP %d, body=%s", resp.StatusCode, string(b))
	}
	var wrapper struct {
		Code    int
		Message string
	}
	_ = json.Unmarshal(b, &wrapper)
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "操作失败"
		}
		return errors.New(wrapper.Message)
	}
	return nil
}
