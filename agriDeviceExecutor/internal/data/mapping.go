package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agriDeviceExecutor/internal/config"
)

// 默认映射文件路径（相对执行目录）。
const defaultMappingPath = "internal/data/executor_mapping.json"

// ExecutorMappingEntry 表示设备+节点+寄存器 与 Magistrala clientId 的映射关系。
// LastValue 与 Status 预留给后续采集/执行状态更新；UpdatedAt 标记最近更新时间。
type ExecutorMappingEntry struct {
	DeviceAddr   string      `json:"deviceAddr"`
	NodeId       int         `json:"nodeId"`
	ClientId     string      `json:"clientId"`
	ClientSecret string      `json:"clientSecret"`
	Status       string      `json:"status"`
	LastValue    interface{} `json:"lastValue"`
	UpdatedAt    int64       `json:"updatedAt"`
}

// 内存存储结构。
type mappingStore struct {
	mu      sync.RWMutex
	entries map[string]ExecutorMappingEntry // key = deviceAddr|nodeId
	path    string
}

var globalStore = &mappingStore{entries: map[string]ExecutorMappingEntry{}, path: defaultMappingPath}

// key 生成。
func makeKey(deviceAddr string, nodeId int) string {
	return fmt.Sprintf("%s|%d", deviceAddr, nodeId)
}

// LoadMapping 从文件加载映射（若文件不存在则创建空文件）。
func LoadMapping(path string) error {
	if path == "" {
		path = defaultMappingPath
	}
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	globalStore.path = path

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// 若新文件不存在，尝试从旧文件迁移
		legacy := "internal/data/Executor_mapping.json"
		if _, lerr := os.Stat(legacy); lerr == nil {
			if content, rerr := os.ReadFile(legacy); rerr == nil {
				// 尝试直接写入到新文件
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
					if len(content) == 0 {
						content = []byte("[]")
					}
					_ = os.WriteFile(path, content, 0o644)
				}
			}
		}
		// 如果仍不存在，初始化空文件
		if _, ck := os.Stat(path); errors.Is(ck, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("创建映射目录失败: %w", err)
			}
			empty := []ExecutorMappingEntry{}
			b, _ := json.MarshalIndent(empty, "", "  ")
			if err := os.WriteFile(path, b, 0o644); err != nil {
				return fmt.Errorf("创建映射文件失败: %w", err)
			}
			return nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取映射文件失败: %w", err)
	}
	var list []ExecutorMappingEntry
	if len(content) > 0 {
		if err := json.Unmarshal(content, &list); err != nil {
			return fmt.Errorf("解析映射文件失败: %w", err)
		}
	}
	globalStore.entries = map[string]ExecutorMappingEntry{}
	// 迁移：旧文件可能包含 registerId；节点级唯一，仅保留首个出现的 client
	for _, e := range list {
		k := makeKey(e.DeviceAddr, e.NodeId)
		if _, exists := globalStore.entries[k]; exists {
			// 已存在则跳过重复节点
			continue
		}
		// 去掉 RegisterId（旧数据忽略）
		globalStore.entries[k] = ExecutorMappingEntry{
			DeviceAddr:   e.DeviceAddr,
			NodeId:       e.NodeId,
			ClientId:     e.ClientId,
			ClientSecret: e.ClientSecret,
			Status:       e.Status,
			LastValue:    e.LastValue,
			UpdatedAt:    e.UpdatedAt,
		}
	}
	return nil
}

// UpdateEntryValue 更新某映射的运行状态与最新值。
func UpdateEntryValue(deviceAddr string, nodeId int, status string, value interface{}) error {
	key := makeKey(deviceAddr, nodeId)

	// 写锁阶段：更新内存
	globalStore.mu.Lock()
	e, ok := globalStore.entries[key]
	if !ok {
		globalStore.mu.Unlock()
		return errors.New("映射不存在")
	}
	e.Status = status
	e.LastValue = value
	e.UpdatedAt = time.Now().Unix()
	globalStore.entries[key] = e
	globalStore.mu.Unlock() // 释放写锁，避免与 SaveMapping 的 RLock 互阻

	// 持久化阶段（单独加读锁快照）
	return SaveMapping()
}

// SaveMapping 将内存映射写回文件。
func SaveMapping() error {
	globalStore.mu.RLock()
	list := make([]ExecutorMappingEntry, 0, len(globalStore.entries))
	for _, e := range globalStore.entries {
		list = append(list, e)
	}
	globalStore.mu.RUnlock() // 尽早释放读锁，减少持有时间

	b, _ := json.MarshalIndent(list, "", "  ")
	if err := os.WriteFile(globalStore.path, b, 0o644); err != nil {
		return fmt.Errorf("保存映射失败: %w", err)
	}
	return nil
}

// GetEntry 查询单条映射。
func GetEntry(deviceAddr string, nodeId int) (ExecutorMappingEntry, bool) {
	globalStore.mu.RLock()
	defer globalStore.mu.RUnlock()
	e, ok := globalStore.entries[makeKey(deviceAddr, nodeId)]
	return e, ok
}

// GetAllEntries 返回全部映射列表。
func GetAllEntries() []ExecutorMappingEntry {
	globalStore.mu.RLock()
	defer globalStore.mu.RUnlock()
	list := make([]ExecutorMappingEntry, 0, len(globalStore.entries))
	for _, e := range globalStore.entries {
		list = append(list, e)
	}
	return list
}

// GetEntryByClientId 根据 clientId 反向查询映射。常用于执行端点根据 clientId 控制。
func GetEntryByClientId(clientId string) (ExecutorMappingEntry, bool) {
	if clientId == "" {
		return ExecutorMappingEntry{}, false
	}
	globalStore.mu.RLock()
	defer globalStore.mu.RUnlock()
	for _, e := range globalStore.entries {
		if e.ClientId == clientId {
			return e, true
		}
	}
	return ExecutorMappingEntry{}, false
}

// EnsureEntry 确保映射存在；若不存在则向 Magistrala 注册并创建。
func EnsureEntry(deviceAddr string, nodeId int) (ExecutorMappingEntry, error) {
	if deviceAddr == "" || nodeId <= 0 {
		return ExecutorMappingEntry{}, errors.New("非法参数: deviceAddr/nodeId 必填")
	}
	key := makeKey(deviceAddr, nodeId)
	globalStore.mu.RLock()
	existing, ok := globalStore.entries[key]
	globalStore.mu.RUnlock()
	if ok {
		return existing, nil
	}
	clientId, clientSecret, err := RegisterMagistralaClient(deviceAddr, nodeId)
	if err != nil {
		return ExecutorMappingEntry{}, fmt.Errorf("注册 Magistrala client 失败: %w", err)
	}
	entry := ExecutorMappingEntry{
		DeviceAddr:   deviceAddr,
		NodeId:       nodeId,
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Status:       "new",
		UpdatedAt:    time.Now().Unix(),
	}
	globalStore.mu.Lock()
	globalStore.entries[key] = entry
	globalStore.mu.Unlock()
	if err := SaveMapping(); err != nil {
		return entry, fmt.Errorf("保存映射文件失败: %w", err)
	}
	return entry, nil
}

// RegisterMagistralaClient 注册设备节点到 Magistrala，并连接到指定频道，返回 clientId 与 clientSecret。
func RegisterMagistralaClient(deviceAddr string, nodeId int) (string, string, error) {
	if strings.TrimSpace(deviceAddr) == "" || nodeId <= 0 {
		return "", "", errors.New("注册参数非法: 需要 deviceAddr、nodeId")
	}

	// 读取配置（均来自 config.json）
	baseURL, err := config.GetMagistralaBaseURL()
	if err != nil {
		return "", "", err
	}
	magToken, err := config.GetMagistralaToken()
	if err != nil {
		return "", "", err
	}
	domainID, err := config.GetMagistralaDomainID()
	if err != nil {
		return "", "", err
	}
	channelID, err := config.GetMagistralaChannelID()
	if err != nil {
		return "", "", err
	}

	// 在此处拼接端口：client 用 9006，channel 用 9005
	clientBase := strings.TrimRight(baseURL, "/") + ":9006"
	channelBase := strings.TrimRight(baseURL, "/") + ":9005"

	// 构造客户端创建请求体（executor）
	// 为避免重复信息，name 与 identity 不再包含 registerId
	name := fmt.Sprintf("executor-%s-%d", deviceAddr, nodeId)
	identity := fmt.Sprintf("executor-%s-%d", deviceAddr, nodeId)
	secret := fmt.Sprintf("secret-%s-%d-%d", deviceAddr, nodeId, time.Now().UnixNano())

	payload := map[string]any{
		"name":   name,
		"tags":   []string{"agri", "executor"},
		"status": "enabled",
		"credentials": map[string]any{
			"identity": identity,
			"secret":   secret,
		},
		"metadata": map[string]any{
			"device_addr":  deviceAddr,
			"node_id":      nodeId,
			"created_from": "executor-auto-sync",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("序列化创建请求失败: %w", err)
	}

	// 1) 创建客户端（9006）
	createURL := fmt.Sprintf("%s/%s/clients", clientBase, domainID)
	req, err := http.NewRequest(http.MethodPost, createURL, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+magToken)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取创建响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("创建客户端失败 http=%d body=%s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		ID          string         `json:"id"`
		Credentials map[string]any `json:"credentials"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", "", fmt.Errorf("解析创建响应失败: %w", err)
	}
	clientId := result.ID
	clientSecret := secret
	if s, ok := result.Credentials["secret"].(string); ok && s != "" {
		clientSecret = s
	}
	if clientId == "" {
		return "", "", errors.New("创建成功但未返回 id")
	}

	// 2) 连接到频道（9005）
	connectURL := fmt.Sprintf("%s/%s/channels/connect", channelBase, domainID)
	connectReq := map[string]any{
		"channel_ids": []string{channelID},
		"client_ids":  []string{clientId},
		"types":       []string{"publish", "subscribe"},
	}
	connectBody, err := json.Marshal(connectReq)
	if err != nil {
		return "", "", fmt.Errorf("序列化连接请求失败: %w", err)
	}
	cReq, err := http.NewRequest(http.MethodPost, connectURL, bytes.NewReader(connectBody))
	if err != nil {
		return "", "", fmt.Errorf("创建连接请求失败: %w", err)
	}
	cReq.Header.Set("Authorization", "Bearer "+magToken)
	cReq.Header.Set("Content-Type", "application/json")

	cResp, err := httpClient.Do(cReq)
	if err != nil {
		return "", "", fmt.Errorf("连接频道请求失败: %w", err)
	}
	defer cResp.Body.Close()
	cBytes, _ := io.ReadAll(cResp.Body)
	if cResp.StatusCode != http.StatusOK && cResp.StatusCode != http.StatusCreated && cResp.StatusCode != http.StatusNoContent {
		return "", "", fmt.Errorf("连接频道失败 http=%d body=%s", cResp.StatusCode, string(cBytes))
	}

	return clientId, clientSecret, nil
}
