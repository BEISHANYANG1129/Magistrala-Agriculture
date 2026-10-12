package agridataintegration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// PlatformService 表示云平台 API 访问对象
type PlatformService struct {
	BaseURL string       // 云平台基础URL，例如 http://www.0531yun.com
	Token   string       // 鉴权token
	Client  *http.Client // http客户端，可自定义
}

// NewPlatformService 创建一个新的云平台 API 访问对象
func NewPlatformService(baseURL string) *PlatformService {
	return &PlatformService{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

// setAuthHeader 在请求头里加上 authorization token
func (s *PlatformService) setAuthHeader(req *http.Request) {
	if s.Token != "" {
		req.Header.Set("authorization", s.Token)
	}
}

// Result 是云平台统一的返回结构
// 注意：具体Data字段根据不同接口变化，在调用方定义Result[具体类型]即可
type Result[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// Login 根据用户名密码获取token
// loginName: 云平台用户名
// password: 云平台密码
// 返回值：token字符串
func (s *PlatformService) Login(loginName, password string) (string, error) {
	api := "/api/getToken"
	params := url.Values{}
	params.Set("loginName", loginName)
	params.Set("password", password)
	u := fmt.Sprintf("%s%s?%s", s.BaseURL, api, params.Encode())

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 返回的Data里有expiration和token两个字段
	var result Result[struct {
		Expiration int64  `json:"expiration"`
		Token      string `json:"token"`
	}]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 1000 {
		return "", fmt.Errorf("login failed: %s", result.Message)
	}

	// 保存token到客户端
	s.Token = result.Data.Token
	return s.Token, nil
}
