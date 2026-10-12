package api

import (
	"agriDeviceExecutor/internal/api/handlers"
	"net/http"
)

// SetupMux 初始化 HTTP 路由。
// 说明：
// 1. 内部服务路径不再使用 /api/v2.0 前缀；仅第三方平台仍用其原始前缀（在 service 层构造）。
// 2. 除登录外所有接口均经过 RequireAuth 中间件（严格读取本地 token 与基础地址）。
// 3. handlers.*Handler 只负责参数提取与调用 service，统一返回 JSON。
func SetupMux() *http.ServeMux {
	mux := http.NewServeMux()

	// 登录接口（不需要鉴权中间件；内部固定读取账号密码发起登录写入 token）
	mux.HandleFunc("/entrance/user/userLogin",
		handlers.OnlyPost(handlers.UserLoginHandler))

	// 获取当前登录用户信息（需已登录）
	mux.HandleFunc("/entrance/user/getUser",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.GetUserHandler(w, r, token, baseURL)
		}))

	// 获取用户设备列表（支持 groupId / deviceType 过滤）
	mux.HandleFunc("/entrance/device/getSysUserDevice",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.GetSysUserDeviceHandler(w, r, token, baseURL)
		}))

	// 8.1 批量获取设备详情（devAddr 可传多个，英文逗号分隔，最多 5 个）
	mux.HandleFunc("/irrigation/node/getDeviceIii",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.GetIrrigationDeviceDetailsHandler(w, r, token, baseURL)
		}))

	// 8.2 更新单个设备信息（名称等）
	mux.HandleFunc("/irrigation/device/updateDevInfo",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.UpdateIrrigationDeviceInfoHandler(w, r, token, baseURL)
		}))

	// 8.3 获取设备节点列表（实际可控制的阀门节点集合）
	mux.HandleFunc("/irrigation/node/getDeviceNodeList",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.GetDeviceNodeListHandler(w, r, token, baseURL)
		}))

	// 8.4 更新单个节点信息（如节点名称）
	mux.HandleFunc("/irrigation/node/updateDeviceNode",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.UpdateDeviceNodeHandler(w, r, token, baseURL)
		}))

	// 8.5 批量节点使能/禁用（enable=1 开启 0 关闭，可按 factorType 分类）
	mux.HandleFunc("/irrigation/node/batchNodeEnable",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.BatchNodeEnableHandler(w, r, token, baseURL)
		}))

	// 8.6 获取节点遥调配置（阀门调节档位等）
	mux.HandleFunc("/irrigation/factor/getIrrigationFactorRegulating",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.GetIrrigationFactorRegulatingHandler(w, r, token, baseURL)
		}))

	// 8.7 替换节点遥调配置（整表提交）
	mux.HandleFunc("/irrigation/factor/replaceTbIrrigationFactorRegulating",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.ReplaceTbIrrigationFactorRegulatingHandler(w, r, token, baseURL)
		}))

	// 8.8 历史数据查询（支持时间范围分页、节点筛选）
	mux.HandleFunc("/irrigation/node/getHistoryDataList",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.GetHistoryDataListHandler(w, r, token, baseURL)
		}))

	// 8.9 修改阀门工作模式（mode=1 手动 2 自动）
	mux.HandleFunc("/irrigation/factor/updateFactorMode",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.UpdateFactorModeHandler(w, r, token, baseURL)
		}))

	// 8.10 手动开关阀门（mode=1 开 0 关，需传 factorId）
	mux.HandleFunc("/irrigation/node/manualControlValve",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.ManualControlValveHandler(w, r, token, baseURL)
		}))

	// -------- Executor 精简执行端点（保留旧灌溉接口以便回退） --------

	// 列出所有已映射节点（后端映射加载需在启动时完成）
	mux.HandleFunc("/executor/nodes",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.ExecutorListNodesHandler(w, r)
		}))

	// 触发全量刷新（重新同步设备+节点并生成缺失映射）
	mux.HandleFunc("/executor/nodes/refresh",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.ExecutorRefreshHandler(w, r, token, baseURL)
		}))

	// 阀门开关控制（POST: clientId + action=open|close）
	mux.HandleFunc("/executor/valveControl",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.ExecutorValveControlHandler(w, r, token, baseURL)
		}))

	// 阀门模式更新（POST: clientId + mode=1|2）
	mux.HandleFunc("/executor/modeUpdate",
		handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request, token, baseURL string) {
			handlers.ExecutorModeUpdateHandler(w, r, token, baseURL)
		}))

	return mux
}
