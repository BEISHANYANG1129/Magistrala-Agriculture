package handlers

import (
	"agriDeviceExecutor/internal/models"
	"agriDeviceExecutor/internal/service"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ManualControlValveHandler 处理 GET /api/v2.0/irrigation/node/manualControlValve
//
// Query 参数：
//   - deviceAddr: string 必填
//   - factorId:   string 必填（节点 id）
//   - mode:       string 必填，0 关闭 1 开启
//
// 鉴权：token 从本地凭据读取（service 层处理），此处不强制 header。
func ManualControlValveHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	q := r.URL.Query()
	deviceAddr := q.Get("deviceAddr")
	factorId := q.Get("factorId")
	mode := q.Get("mode")

	if deviceAddr == "" || factorId == "" || (mode != "0" && mode != "1") {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "参数非法: 需要 deviceAddr、factorId、mode(0|1)"})
		return
	}

	if err := service.ManualControlValve(token, baseURL, deviceAddr, factorId, mode); err != nil {
		// 按业务常见语义返回 200 + 业务码或 500。
		// 这里保持与其它接口一致：失败时返回 500，message 透出。
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: fmt.Sprintf("%v", err)})
		return
	}

	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success", Data: nil})
}

// GetIrrigationDeviceDetailsHandler 处理批量获取灌溉设备详情（8.1）
// GET /api/v2.0/irrigation/node/getDeviceIii?devAddr=a,b,c （最多 5 个）
// 返回 code=1000 data=[...] 或错误
func GetIrrigationDeviceDetailsHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	devAddr := r.URL.Query().Get("devAddr")
	if devAddr == "" {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "缺少 devAddr"})
		return
	}
	parts := strings.Split(devAddr, ",")
	if len(parts) > 5 {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "最多支持 5 个设备"})
		return
	}
	data, err := service.GetIrrigationDeviceDetails(token, baseURL, devAddr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success", Data: data})
}

// UpdateIrrigationDeviceInfoHandler 修改设备信息（8.2）
// POST /api/v2.0/irrigation/device/updateDevInfo
func UpdateIrrigationDeviceInfoHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	var payload map[string]any
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "请求体无效"})
		return
	}
	if err := service.UpdateIrrigationDeviceInfo(token, baseURL, payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success"})
}

// GetDeviceNodeListHandler 获取节点列表（8.3）
// GET /api/v2.0/irrigation/node/getDeviceNodeList?devAddr=...
func GetDeviceNodeListHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	devAddr := r.URL.Query().Get("devAddr")
	if strings.TrimSpace(devAddr) == "" {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "缺少 devAddr"})
		return
	}
	data, err := service.GetDeviceNodeList(token, baseURL, devAddr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success", Data: data})
}

// UpdateDeviceNodeHandler 修改节点信息（8.4）
// POST /api/v2.0/irrigation/node/updateDeviceNode
func UpdateDeviceNodeHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	var payload map[string]any
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "请求体无效"})
		return
	}
	if err := service.UpdateDeviceNode(token, baseURL, payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success"})
}

// BatchNodeEnableHandler 批量开关节点（8.5）
// POST /api/v2.0/irrigation/node/batchNodeEnable
func BatchNodeEnableHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	var payload struct {
		DevAddr    string `json:"devAddr"`
		Enable     string `json:"enable"`
		FactorType string `json:"factorType"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "请求体无效"})
		return
	}
	if err := service.BatchNodeEnable(token, baseURL, payload.DevAddr, payload.Enable, payload.FactorType); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success"})
}

// GetIrrigationFactorRegulatingHandler 获取节点遥调信息（8.6）
// GET /api/v2.0/irrigation/factor/getIrrigationFactorRegulating?factorId=...
func GetIrrigationFactorRegulatingHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	factorId := r.URL.Query().Get("factorId")
	if strings.TrimSpace(factorId) == "" {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "缺少 factorId"})
		return
	}
	data, err := service.GetIrrigationFactorRegulating(token, baseURL, factorId)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success", Data: data})
}

// ReplaceTbIrrigationFactorRegulatingHandler 更新节点遥调信息（8.7）
// POST /api/v2.0/irrigation/factor/replaceTbIrrigationFactorRegulating
func ReplaceTbIrrigationFactorRegulatingHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	var body struct {
		List []service.RegulatingItem `json:"listTbIrrigationFactorRegulating"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "请求体无效"})
		return
	}
	if err := service.ReplaceTbIrrigationFactorRegulating(token, baseURL, body.List); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success"})
}

// GetHistoryDataListHandler 历史记录（8.8）
// GET /api/v2.0/irrigation/node/getHistoryDataList
func GetHistoryDataListHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	q := r.URL.Query()
	deviceAddr := q.Get("deviceAddr")
	startTime := q.Get("startTime")
	endTime := q.Get("endTime")
	pagesStr := q.Get("pages")
	limitStr := q.Get("limit")
	nodeId := q.Get("nodeId")
	pages, _ := strconv.Atoi(pagesStr)
	limit, _ := strconv.Atoi(limitStr)
	data, err := service.GetHistoryDataList(token, baseURL, deviceAddr, startTime, endTime, pages, limit, nodeId)
	if err != nil {
		// 对必填校验失败返回 400
		if strings.Contains(err.Error(), "必填") || strings.Contains(err.Error(), "不能为空") {
			writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success", Data: data})
}

// UpdateFactorModeHandler 修改阀门工作模式（8.9）
// POST /api/v2.0/irrigation/factor/updateFactorMode
func UpdateFactorModeHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	var payload struct{ FactorId, Mode string }
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "请求体无效"})
		return
	}
	if err := service.UpdateFactorMode(token, baseURL, payload.FactorId, payload.Mode); err != nil {
		// 对参数错误 400，其它 500
		if strings.Contains(err.Error(), "不能为空") || strings.Contains(err.Error(), "取值") {
			writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "success"})
}
