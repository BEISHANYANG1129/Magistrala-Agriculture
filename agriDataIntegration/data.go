package agridataintegration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ========== 实时数据 ==========

// RegisterItem 表示某个寄存器数据
type RegisterItem struct {
	RegisterId   int     `json:"registerId"`
	RegisterName string  `json:"registerName"`
	Data         string  `json:"data"`
	Value        float64 `json:"value"`
	AlarmLevel   int     `json:"alarmLevel"`
	AlarmInfo    string  `json:"alarmInfo"`
	Unit         string  `json:"unit"`
}

// DataItem 表示节点数据
type DataItem struct {
	NodeId       int            `json:"nodeId"`
	RegisterItem []RegisterItem `json:"registerItem"`
}

// RealTimeData 表示实时数据
type RealTimeData struct {
	SystemCode   string     `json:"systemCode"`
	DeviceAddr   int        `json:"deviceAddr"`
	DeviceName   string     `json:"deviceName"`
	Lat          float64    `json:"lat"`
	Lng          float64    `json:"lng"`
	DeviceStatus string     `json:"deviceStatus"`
	RelayStatus  string     `json:"relayStatus"` // 继电器状态Json字符串
	DataItem     []DataItem `json:"dataItem"`
	TimeStamp    int64      `json:"timeStamp"`
}

// GetRealTimeData 查询实时数据（groupId可选）
func (s *PlatformService) GetRealTimeData(groupId string) ([]RealTimeData, error) {
	api := "/api/data/getRealTimeData"
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

	var result Result[[]RealTimeData]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("GetRealTimeData failed: %s", result.Message)
	}
	return result.Data, nil
}

// GetRealTimeDataByDeviceAddr 根据设备地址查询实时数据（多个设备用英文逗号分隔）
func (s *PlatformService) GetRealTimeDataByDeviceAddr(deviceAddrs string) ([]RealTimeData, error) {
	api := "/api/data/getRealTimeDataByDeviceAddr"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	q.Set("deviceAddrs", deviceAddrs)
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]RealTimeData]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("GetRealTimeDataByDeviceAddr failed: %s", result.Message)
	}
	return result.Data, nil
}

// ========== 历史数据 ==========

// HistoryData 历史数据记录
type HistoryData struct {
	DeviceAddr    int     `json:"deviceAddr"`
	NodeId        int     `json:"nodeId"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	RecordTime    int64   `json:"recordTime"`
	RecordId      string  `json:"recordId"`
	RecordTimeStr string  `json:"recordTimeStr"`
	Data          []struct {
		RegisterId   int     `json:"registerId"`
		RegisterName string  `json:"registerName"`
		Value        float64 `json:"value"`
		Text         string  `json:"text"`
		AlarmLevel   int     `json:"alarmLevel"`
	} `json:"data"`
}

// HistoryList 获取历史数据列表
// nodeId=-1 表示查询所有节点
// 时间格式："YYYY-MM-dd HH:mm:ss"
func (s *PlatformService) HistoryList(deviceAddr, nodeId int, startTime, endTime string) ([]HistoryData, error) {
	api := "/api/data/historyList"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	q.Set("deviceAddr", fmt.Sprintf("%d", deviceAddr))
	q.Set("nodeId", fmt.Sprintf("%d", nodeId))
	q.Set("startTime", startTime)
	q.Set("endTime", endTime)
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]HistoryData]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("HistoryList failed: %s", result.Message)
	}
	return result.Data, nil
}

// DelHistory 删除历史数据
func (s *PlatformService) DelHistory(id string) (bool, error) {
	api := "/api/data/delHistory"
	u := s.BaseURL + api

	form := url.Values{}
	form.Set("id", id)

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
		return false, fmt.Errorf("DelHistory failed: %s", result.Message)
	}
	return result.Data, nil
}

// ========== 继电器操作记录 ==========

// RelayOptRecord 表示继电器操作记录
type RelayOptRecord struct {
	RecordId     string `json:"recordId"`
	DeviceAdd    int    `json:"deviceAdd"`
	RelayNo      int    `json:"relayNo"`
	RelayName    string `json:"relayName"`
	CreateTime   int64  `json:"createTime"`
	Opt          int    `json:"opt"`
	OptUserId    string `json:"optUserId"`
	OptLoginName string `json:"optLoginName"`
}

// GetRelayOptRecord 查询继电器操作记录
// 时间戳为毫秒值
func (s *PlatformService) GetRelayOptRecord(deviceAddr int, beginTime, endTime int64) ([]RelayOptRecord, error) {
	api := "/api/data/getRelayOptRecord"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	q.Set("deviceAddr", fmt.Sprintf("%d", deviceAddr))
	q.Set("beginTime", fmt.Sprintf("%d", beginTime))
	q.Set("endTime", fmt.Sprintf("%d", endTime))
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]RelayOptRecord]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("GetRelayOptRecord failed: %s", result.Message)
	}
	return result.Data, nil
}

// ========== 报警数据 ==========

// AlarmRecord 表示报警数据
type AlarmRecord struct {
	DeviceAddr int     `json:"deviceAddr"`
	NodeId     int     `json:"nodeId"`
	FactorId   string  `json:"factorId"`
	FactorName string  `json:"factorName"`
	AlarmLevel int     `json:"alarmLevel"`
	DataValue  float64 `json:"dataValue"`
	DataText   string  `json:"dataText"`
	AlarmRange string  `json:"alarmRange"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	RecordTime int64   `json:"recordTime"`
	Handled    bool    `json:"handled"`
	HandleMsg  string  `json:"handleMsg"`
	HandleUser string  `json:"handleUser"`
	HandleTime int64   `json:"handleTime"`
	RecordId   string  `json:"recordId"`
}

// AlarmRecordList 获取报警数据列表
// nodeId=-1 表示查询所有节点
func (s *PlatformService) AlarmRecordList(deviceAddr, nodeId int, startTime, endTime string) ([]AlarmRecord, error) {
	api := "/api/data/alarmRecordList"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	q := req.URL.Query()
	q.Set("deviceAddr", fmt.Sprintf("%d", deviceAddr))
	q.Set("nodeId", fmt.Sprintf("%d", nodeId))
	q.Set("startTime", startTime)
	q.Set("endTime", endTime)
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]AlarmRecord]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("AlarmRecordList failed: %s", result.Message)
	}
	return result.Data, nil
}
