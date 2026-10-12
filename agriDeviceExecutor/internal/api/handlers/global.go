package handlers

import (
	"agriDeviceExecutor/internal/models"
	"agriDeviceExecutor/internal/service"
	"net/http"
)

// UserLoginHandler 处理 POST /api/v2.0/entrance/user/userLogin
//
// 请求体 JSON 示例：
//
//	{
//	  "loginName": "string",   // 必填：登录名
//	  "loginPwd":  "string"    // 必填：登录密码
//	}
//
// 响应（models.ResultData）：
//   - 1000 OK：data 携带令牌和用户信息
//   - 401 Unauthorized：凭证无效
//   - 400 Bad Request：请求 JSON 无效
//
// 说明：
//   - 具体认证逻辑由 service.UserLogin 实现，已对接第三方平台登录接口，
//     成功后会将 token 持久化到 credentials.json 供后续接口复用。
func UserLoginHandler(w http.ResponseWriter, r *http.Request) {
	// 不再从请求体获取账号密码；统一从 credentials.json 读取
	data, err := service.UserLogin("", "")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ResultData{Code: 401, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "登录成功", Data: data})
}

// GetUserHandler 处理 GET /api/v2.0/entrance/user/getUser
//
// 鉴权：
// - 通过请求头 `token` 传入应用令牌。
//
// 响应（models.ResultData）：
//   - 1000 OK：返回用户资料
//   - 401 Unauthorized：缺少/无效 token
//   - 500 Internal Server Error：下游调用失败
//
// 说明：
// - 基于请求头携带 token 的鉴权方式，由 service.GetUserInfo 调用第三方接口校验并获取资料。
func GetUserHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	data, err := service.GetUserInfo(token, baseURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "获取成功", Data: data})
}

// GetSysUserDeviceHandler 处理 GET /api/v2.0/entrance/device/getSysUserDevice
// 功能：获取当前用户设备列表。
// token 获取：优先请求头 token；若缺失则由服务层回退读取 credentials.json 中的 userToken。
// 可选查询参数：groupId、deviceType。
// 响应：code=1000 -> data 为设备数组；401 -> token 问题；500 -> 下游错误。
func GetSysUserDeviceHandler(w http.ResponseWriter, r *http.Request, token string, baseURL string) {
	groupID := r.URL.Query().Get("groupId")
	deviceType := r.URL.Query().Get("deviceType")
	devices, err := service.GetSysUserDevice(token, baseURL, groupID, deviceType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ResultData{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.ResultData{Code: 1000, Message: "获取成功", Data: devices})
}
