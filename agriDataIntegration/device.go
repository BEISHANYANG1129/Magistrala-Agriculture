package agridataintegration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ========== 设备信息结构体 ==========

// Factor 表示设备的监测因子
type Factor struct {
	FactorId           string  `json:"factorId"`
	DeviceAddr         int     `json:"deviceAddr"`
	NodeId             int     `json:"nodeId"`
	RegisterId         int     `json:"registerId"`
	FactorName         string  `json:"factorName"`
	Coefficient        float64 `json:"coefficient"`
	Offset             float64 `json:"offset"`
	AlarmDelay         int     `json:"alarmDelay"`
	AlarmRate          int     `json:"alarmRate"`
	BackToNormalDelay  int     `json:"backToNormalDelay"`
	Digits             int     `json:"digits"`
	Unit               string  `json:"unit"`
	Enabled            bool    `json:"enabled"`
	MaxVoiceAlarmTimes int     `json:"maxVoiceAlarmTimes"`
	MaxSmsAlarmTimes   int     `json:"maxSmsAlarmTimes"`
}

// Device 表示设备信息
type Device struct {
	DeviceAddr       int      `json:"deviceAddr"`
	GroupId          string   `json:"groupId"`
	DeviceName       string   `json:"deviceName"`
	OfflineInterval  int      `json:"offlineinterval"`
	SaveDataInterval int      `json:"savedatainterval"`
	AlarmSwitch      int      `json:"alarmSwitch"`
	AlarmRecord      int      `json:"alarmRecord"`
	Lng              float64  `json:"lng"`
	Lat              float64  `json:"lat"`
	UseMarkLocation  bool     `json:"useMarkLocation"`
	Sort             int      `json:"sort"`
	DeviceCode       string   `json:"deviceCode"`
	Factors          []Factor `json:"factors"`
}

// GetDeviceList 查询设备列表（groupId可选）
// groupId 为空表示获取全部设备
func (s *PlatformService) GetDeviceList(groupId string) ([]Device, error) {
	api := "/api/device/getDeviceList"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	if groupId != "" {
		q.Set("groupId", groupId)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]Device]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("GetDeviceList failed: %s", result.Message)
	}
	return result.Data, nil
}

// GetDevice 根据设备地址查询设备信息
func (s *PlatformService) GetDevice(deviceAddr int) (*Device, error) {
	api := "/api/device/getDevice"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	q.Set("deviceAddr", fmt.Sprintf("%d", deviceAddr))
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[Device]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("GetDevice failed: %s", result.Message)
	}
	return &result.Data, nil
}

// ========== 继电器接口结构体 ==========

// Relay 表示继电器信息
type Relay struct {
	DeviceAddr  int    `json:"deviceAddr"`
	DeviceName  string `json:"deviceName"`
	Enabled     bool   `json:"enabled"`
	RelayName   string `json:"relayName"`
	RelayNo     int    `json:"relayNo"`
	RelayStatus int    `json:"relayStatus"`
}

// GetRelayList 根据设备地址获取设备继电器列表
func (s *PlatformService) GetRelayList(deviceAddr int) ([]Relay, error) {
	api := "/api/device/getRelayList"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	q.Set("deviceAddr", fmt.Sprintf("%d", deviceAddr))
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]Relay]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("GetRelayList failed: %s", result.Message)
	}
	return result.Data, nil
}

// SetRelay 继电器操作
// opt: 0=闭合 1=断开
func (s *PlatformService) SetRelay(deviceAddr int, relayNo int, opt int) (bool, error) {
	api := "/api/device/setRelay"
	u := s.BaseURL + api

	form := url.Values{}
	form.Set("deviceAddr", fmt.Sprintf("%d", deviceAddr))
	form.Set("relayNo", fmt.Sprintf("%d", relayNo))
	form.Set("opt", fmt.Sprintf("%d", opt))

	req, err := http.NewRequest("POST", u, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return false, err
	}
	s.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result Result[bool]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	if result.Code != 1000 {
		return false, fmt.Errorf("SetRelay failed: %s", result.Message)
	}
	return result.Data, nil
}
