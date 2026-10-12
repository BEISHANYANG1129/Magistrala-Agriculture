package service

import (
	"agriDeviceExecutor/internal/config"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// UserLogin 用户登录，返回平台 token 及登录元信息。
//
// 合同（Contract）：
// - 输入：忽略入参，账号与密码一律从 credentials.json 读取
// - 成功：返回包含 token、loginSign、currDate、expDate 等信息的 map
// - 失败：返回错误（参数缺失、HTTP 非 200、业务 code != 1000、解析失败等）
//
// 行为说明：
//   - 从 credentials.json 读取 apiBaseURL，通过
//     POST {apiBaseURL}/api/v2.0/entrance/user/userLogin 调用第三方平台登录接口；
//     请求体：{"loginName":"...","loginPwd":"..."}
//   - 响应结构参照文档：code=1000 表示成功，data 中包含 token 等字段；
//   - 成功后会调用 config.SetUserToken() 将 token 写入 credentials.json，供后续接口复用；
//   - HTTP 超时 10s，错误信息尽量保留平台返回便于排查。
//
// 安全提示：避免将明文口令写入日志；生产环境建议使用 HTTPS。
func UserLogin(_ string, _ string) (interface{}, error) {
	// 始终从本地 JSON 读取账号与密码（不依赖客户端传参）
	fileLoginName, fileLoginPwd, err := config.GetLoginCredentials()
	if err != nil {
		return nil, fmt.Errorf("读取本地凭据失败: %w", err)
	}
	loginName := fileLoginName
	loginPwd := fileLoginPwd

	// 读取第三方平台基础地址（公共方法处理去尾部斜杠与错误包装）
	baseURL, err := config.GetNormalizedAPIBaseURL()
	if err != nil {
		return nil, err
	}

	// 构造请求
	loginURL := baseURL + "/api/v2.0/entrance/user/userLogin"
	reqBody := map[string]string{
		"loginName": loginName,
		"loginPwd":  loginPwd,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// 尝试解析业务层 JSON 以获取 code/message
		var w struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBytes, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("登录失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("登录失败 HTTP %d, body=%s", resp.StatusCode, string(respBytes))
	}

	// 解析通用响应
	var wrapper struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("解析登录响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "登录失败"
		}
		return nil, errors.New(wrapper.Message)
	}

	// 解析 data 字段，按文档包含 token、loginSign、currDate、expDate
	var data struct {
		LoginSign string `json:"loginSign"`
		CurrDate  int64  `json:"currDate"`
		ExpDate   int64  `json:"expDate"`
		Token     string `json:"token"`
	}
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &data); err != nil {
			return nil, fmt.Errorf("解析登录data失败: %w", err)
		}
	}
	if data.Token == "" {
		return nil, errors.New("登录成功但未返回 token")
	}

	// 持久化 token 到本地凭据文件
	if err := config.SetUserToken(data.Token); err != nil {
		return nil, fmt.Errorf("保存用户令牌失败: %w", err)
	}

	// 返回给上层
	return map[string]any{
		"token":     data.Token,
		"loginSign": data.LoginSign,
		"currDate":  data.CurrDate,
		"expDate":   data.ExpDate,
		"loginName": loginName,
	}, nil
}

// GetUserInfo 调用第三方接口根据 token 获取登录用户信息（支持自动回退读取本地 token）。
func GetUserInfo(token string, baseURL string) (interface{}, error) {
	url := baseURL + "/api/v2.0/entrance/user/getUser"
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBytes, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("获取用户失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("获取用户失败 HTTP %d, body=%s", resp.StatusCode, string(respBytes))
	}

	var wrapper struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("解析用户响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "获取用户失败"
		}
		return nil, errors.New(wrapper.Message)
	}
	var user map[string]any
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &user); err != nil {
			return nil, fmt.Errorf("解析用户data失败: %w", err)
		}
	}
	if user == nil {
		user = map[string]any{}
	}
	return user, nil
}

// GetSysUserDevice 调用第三方接口获取当前用户的设备列表。
//
// 合同（Contract）：
// - 输入：token（可为空，若为空则自动从 credentials.json 中读取 userToken）；groupID/deviceType 可选过滤。
// - 成功：返回设备数组（[]map[string]any）。
// - 失败：token 缺失、HTTP 非 200、业务 code != 1000、解析失败等。
//
// 行为说明：
//   - 读取 apiBaseURL 构造 GET {apiBaseURL}/api/v2.0/entrance/device/getSysUserDevice
//   - Header: token
//   - Query: groupId, deviceType（若提供）
//   - 响应 code=1000 时解析 data 为设备列表并返回。
func GetSysUserDevice(token, baseURL, groupID, deviceType string) ([]map[string]any, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析基础地址失败: %w", err)
	}
	u.Path = "/api/v2.0/entrance/device/getsysUserDevice"
	q := u.Query()
	if groupID != "" {
		q.Set("groupId", groupID)
	}
	if deviceType != "" {
		q.Set("deviceType", deviceType)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("token", token)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)

	// 调试输出：云平台原始返回（不包含 token）
	fmt.Printf("[DEBUG] getSysUserDevice cloud resp http=%d raw=%s\n", resp.StatusCode, string(respBytes))

	if resp.StatusCode != http.StatusOK {
		var w struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBytes, &w)
		if w.Code != 0 || w.Message != "" {
			return nil, fmt.Errorf("获取设备失败 HTTP %d, 业务code=%d, message=%s", resp.StatusCode, w.Code, w.Message)
		}
		return nil, fmt.Errorf("获取设备失败 HTTP %d, body=%s", resp.StatusCode, string(respBytes))
	}

	var wrapper struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("解析设备响应失败: %w", err)
	}
	if wrapper.Code != 1000 {
		if wrapper.Message == "" {
			wrapper.Message = "获取设备失败"
		}
		return nil, errors.New(wrapper.Message)
	}

	var devices []map[string]any
	if len(wrapper.Data) > 0 {
		if err := json.Unmarshal(wrapper.Data, &devices); err != nil {
			return nil, fmt.Errorf("解析设备列表失败: %w", err)
		}
	}
	if devices == nil {
		devices = []map[string]any{}
	}
	return devices, nil
}
