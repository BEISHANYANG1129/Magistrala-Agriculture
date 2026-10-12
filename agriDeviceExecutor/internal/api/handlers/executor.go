package handlers

import (
	"agriDeviceExecutor/internal/data"
	"agriDeviceExecutor/internal/models"
	"agriDeviceExecutor/internal/service"
	"net/http"
	//"time"
)

// 统一执行端点返回结构：models.ResultData
// 成功：code=1000,message="ok"
// 失败：根据错误类型选择 400 或 500。业务错误暂归类 500，可后续细化。

// ExecutorListNodesHandler GET /executor/nodes
// 返回全部映射 entries。
func ExecutorListNodesHandler(w http.ResponseWriter, r *http.Request) {
	entries := data.GetAllEntries()
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "ok", Data: entries})
}

// ExecutorRefreshHandler POST /executor/nodes/refresh
// 触发全量同步。无请求体。
func ExecutorRefreshHandler(w http.ResponseWriter, r *http.Request, token, baseURL string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, models.ResultData{Code: 405, Message: "method not allowed"})
		return
	}
	if err := service.SyncAll(token, baseURL); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "ok"})
}

// ExecutorValveControlHandler POST /executor/valveControl
// Body: {"clientId":"...","action":"open"|"close"}
func ExecutorValveControlHandler(w http.ResponseWriter, r *http.Request, token, baseURL string) {
	//startAll := time.Now()
	// log.Printf("[debug][valve] enter handler method=%s uri=%s", r.Method, r.RequestURI)

	if r.Method != http.MethodPost {
		// log.Printf("[debug][valve] wrong method=%s cost=%s", r.Method, time.Since(startAll))
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, models.ResultData{Code: 405, Message: "method not allowed"})
		return
	}

	var body struct {
		ClientId string `json:"clientId"`
		Action   string `json:"action"`
	}
	if err := decodeJSON(r, &body); err != nil {
		// log.Printf("[debug][valve] decodeJSON error=%v cost=%s", err, time.Since(startAll))
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "invalid json"})
		return
	}
	// log.Printf("[debug][valve] parsed body clientId=%s action=%s", body.ClientId, body.Action)

	if body.ClientId == "" || (body.Action != "open" && body.Action != "close") {
		// log.Printf("[debug][valve] validate fail clientId=%s action=%s cost=%s",
		// 	body.ClientId, body.Action, time.Since(startAll))
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "clientId/action invalid"})
		return
	}

	open := body.Action == "open"
	// log.Printf("[debug][valve] calling ExecuteValveControl clientId=%s open=%v", body.ClientId, open)
	if err := service.ExecuteValveControl(body.ClientId, open, token, baseURL); err != nil {
		// log.Printf("[debug][valve] ExecuteValveControl error=%v cost=%s", err, time.Since(startAll))
		if err.Error() == "clientId 未找到映射: "+body.ClientId {
			writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	// log.Printf("[debug][valve] success clientId=%s totalCost=%s", body.ClientId, time.Since(startAll))
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "ok"})
}

// ExecutorModeUpdateHandler POST /executor/modeUpdate
// Body: {"clientId":"...","mode":"1"|"2"}
func ExecutorModeUpdateHandler(w http.ResponseWriter, r *http.Request, token, baseURL string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, models.ResultData{Code: 405, Message: "method not allowed"})
		return
	}
	var body struct {
		ClientId string `json:"clientId"`
		Mode     string `json:"mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "invalid json"})
		return
	}
	if body.ClientId == "" || (body.Mode != "1" && body.Mode != "2") {
		writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: "clientId/mode invalid"})
		return
	}
	if err := service.ExecuteModeUpdate(body.ClientId, body.Mode, token, baseURL); err != nil {
		if err.Error() == "clientId 未找到映射: "+body.ClientId {
			writeJSON(w, http.StatusBadRequest, models.ResultData{Code: 400, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "ok"})
}
