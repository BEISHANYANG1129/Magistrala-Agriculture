# agriDeviceExecutor

面向“智慧农业灌溉”场景的轻量级 HTTP 服务，使用 Go 标准库 net/http 实现，不依赖第三方 Web 框架。提供用户登录/获取用户信息，以及灌溉设备列表与节点控制的接口骨架，方便后续对接外部平台 API。

## 目录结构

- `cmd/main.go`：程序入口，启动 HTTP 服务（默认 :8080）。
- `internal/api/router.go`：路由注册，统一挂载所有接口。
- `internal/api/handlers/`：HTTP 处理器（入参校验、统一响应包装）。
- `internal/service/`：业务实现（当前为桩；替换为真实外部 API 调用）。
- `internal/config/credentials.go`：本地凭据管理（单用户模式）。
- `internal/models/`：公共响应模型（ResultData）。
- `pkg/logger/`：日志占位（可替换为结构化日志）。

## 启动运行

- 直接运行：
  - `go run ./cmd`
- 生产建议：
  - 通过反向代理提供 TLS/CORS/限流等；或在本服务中加中间件。
  - 配置优雅关闭（graceful shutdown）。

## Magistrala 配置来源

- 共享配置文件：从仓库根目录的 `data/magistrala.json` 读取 `baseUrl` 与 `userToken`。
  - 示例：
    ```json
    {
      "baseUrl": "http://localhost",
      "userToken": "<magistrala_user_token>"
    }
    ```
- 本地执行器配置：在 `internal/config/config.json` 中仅保留 `domainId` 与 `channelId`。
  - 示例：
    ```json
    {
      "magistrala": {
        "domainId": "<domain_id>",
        "channelId": "<channel_id>"
      }
    }
    ```
- 端口说明：执行器内目前固定使用 `9006`（客户端）与 `9005`（频道）进行注册与连接。
  - 若后续需要改为可配置，可在 `internal/data/mapping.go` 中拓展端口来源。

## 凭据与 token 管理（单用户模式）

- 凭据文件默认路径：`./internal/config/credentials.json`
- 可通过环境变量覆盖：`CREDENTIALS_PATH=/path/to/credentials.json`
- JSON 结构示例：

```json
{
  "loginName": "admin",
  "loginPwd":  "123456",
  "userToken": "可选：登录后写入",
  "roles": ["admin"],
  "extra": {"nick": "管理员"}
}
```

- 提供的方法（在 `internal/config/credentials.go` 中）：
  - `GetLoginCredentials() (loginName, loginPwd string, err error)`：只读返回账号/密码。
  - `GetUserToken() (string, error)`：读取 userToken（可能为空）。
  - `SetUserToken(token string) error`：写入/更新 userToken，原子化落盘（0600 权限）。

- 默认登录流程（桩实现）：
  - `POST /api/v2.0/entrance/user/userLogin` 成功后，会调用 `SetUserToken()` 将令牌落盘。
  - 后续接口读取并校验 token：`GetUserToken()`。

## API 说明

所有接口均返回统一响应模型：

```json
{
  "code": 1000,
  "message": "成功",
  "data": {}
}
```

- 成功：`code=1000`，HTTP 200
- 失败：`code` 与 `message` 描述错误，HTTP 根据错误类型返回（如 400/401/500）

### 1) 用户登录
- 方法与路径：`POST /api/v2.0/entrance/user/userLogin`
- 请求体：
```json
{
  "loginName": "string",
  "loginPwd":  "string"
}
```
- 响应：
```json
{
  "code": 1000,
  "message": "登录成功",
  "data": {"token": "...", "userId": 1, "loginName": "...", "displayName": "..."}
}
```
- 说明：当前为桩实现，真实环境请在 `internal/service/global_service.go` 对接外部登录 API，获取 token 后调用 `config.SetUserToken()` 落盘。

### 2) 获取用户信息
- 方法与路径：`GET /api/v2.0/entrance/user/getUser`
- 头部：`token: <登录返回的令牌>`
- 响应：用户资料；当 token 缺失或无效时返回 401。

### 3) 获取灌溉设备列表
- 方法与路径：`GET /api/v2.0/irrigation/getDevices`
- 头部：`token: <登录返回的令牌>`
- 响应：设备列表，示例：
```json
[
  {"deviceAddr": "40366226", "name": "Valve-1", "status": "normal"},
  {"deviceAddr": "40366227", "name": "Valve-2", "status": "normal"}
]
```
- 说明：当前为桩实现；在 `internal/service/irrigation_service.go` 对接外部设备清单 API。

### 4) 控制灌溉节点
- 方法与路径：`POST /api/v2.0/irrigation/controlNode`
- 请求体：
```json
{
  "deviceAddr": "string",
  "nodeId": 1,
  "action": "open"  // 或 "close"
}
```
- 响应：`{"code":1000,"message":"控制成功"}`；错误时返回 400/500 等。
- 说明：当前为桩实现；在 `internal/service/irrigation_service.go` 对接外部控制 API。

## 对接指引（Service 层）

- `internal/service/global_service.go`
  - `UserLogin()`：调用外部登录接口，拿到 token 后 `config.SetUserToken(token)`。
  - `GetUserInfo()`：调用外部用户信息/令牌校验接口；或先做本地最小校验再调用下游。

- `internal/service/irrigation_service.go`
  - `GetIrrigationDevices()`：携带 token 调用设备清单 API；按字段归一化。
  - `ControlIrrigationNode()`：调用控制 API；建议增加审计日志。

## Curl 示例（本地快速自测）

以下命令仅为演示用途，可在登录桩实现下通过：

1. 登录获取 token：
```bash
curl -sS -X POST http://127.0.0.1:8080/api/v2.0/entrance/user/userLogin \
  -H 'Content-Type: application/json' \
  -d '{"loginName":"admin","loginPwd":"123456"}'
```

2. 读取用户信息（将 <TOKEN> 替换为上一步返回的 token）：
```bash
curl -sS http://127.0.0.1:8080/api/v2.0/entrance/user/getUser \
  -H 'token: <TOKEN>'
```

3. 获取设备列表：
```bash
curl -sS http://127.0.0.1:8080/api/v2.0/irrigation/getDevices \
  -H 'token: <TOKEN>'
```

4. 控制设备节点：
```bash
curl -sS -X POST http://127.0.0.1:8080/api/v2.0/irrigation/controlNode \
  -H 'Content-Type: application/json' \
  -d '{"deviceAddr":"40366226","nodeId":1,"action":"open"}'
```

## 后续建议

- 将 Service 层桩实现替换为真实外部 API 调用，并处理超时/重试/错误码映射。
- 增加中间件（鉴权、CORS、日志、限流）。
- 引入结构化日志与统一的错误码体系。
- 在响应模型中加入 requestId/traceId，便于排错。
