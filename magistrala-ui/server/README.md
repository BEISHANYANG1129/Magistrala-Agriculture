# 农业端长期会话

农业端从只保存一小时访问令牌，升级为服务端保存旋转刷新令牌的记住登录机制。登录页调用同源 `/api/session/login`；响应仅包含短期访问令牌和用户/会话编号，密码不会持久化，刷新令牌不会发回浏览器。

## 本地运行

`npm run dev` 和 `npm run preview` 都自动装配会话中间件，无需另开进程。开发环境访问 `http://127.0.0.1:5173`。迁移前的旧登录没有保存刷新令牌，需要重新登录一次。`localhost` 与 `127.0.0.1` 的 Cookie 独立，请固定使用一个入口。

默认加密会话目录为 Windows `%LOCALAPPDATA%/Magistrala/agri-sessions`，其他系统为 `~/.local/share/Magistrala/agri-sessions`。首次运行生成独立加密密钥，不依赖官方 UI 的密钥或会话卷。不要删除这个目录或其中的 `encryption.key`，否则现有农业端会话将失效。不要将它加入 Git、报告或日志。

Cookie 是带签名的随机会话标识，使用 HttpOnly、SameSite=Strict，限定 Path=/api/session；有效期 400 天，每次会话检查续延。刷新令牌按 AES-256-GCM 加密保存。短期访问令牌仍与既有农业 API 兼容地存放在浏览器；后端访问令牌仍为一小时，没有通过延长访问令牌有效期实现长期登录。

页面进入、回到前台、网络恢复及前台每分钟检查会话；剩余五分钟内自动轮换访问令牌。已过期的访问令牌同样能通过刷新令牌恢复。断网/超时/5xx 不清除会话；明确撤销才要求重新登录。退出时先持久化本地撤销；Atom 不可用时记录待撤销，恢复后重试。会话代理本身不可达时，退出显示失败并要求重试，不谎报成功。

当前沿用本机 Atom 已启用的刷新令牌期限，不修改其配置。清理 Cookie、长期不使用导致 Cookie 到期、撤销会话/账号或丢失加密会话目录仍会要求重新登录；不能称为无条件永不过期。

## 部署

纯静态文件托管不能独立实现这个会话服务。正式部署可使用 `node server/session-server.mjs`（默认只监听 127.0.0.1:5174），由同源反向代理转发 `/api/session`、`/api/session/login`、`/api/session/logout`，其余 API 继续使用现有网关。保持原始 Host/Origin，HTTPS 部署设置 `AGRI_COOKIE_SECURE=true`。可配置 `AGRI_SESSION_DIR`、`AGRI_ATOM_URL` 和 `AGRI_SESSION_PORT`。

本地磁盘与进程内锁适用于单个会话代理实例。同一个会话目录不能同时供多个活动代理进程使用；多实例部署前需要共享事务存储和分布式续期锁。Vite dev、preview 或独立服务应择一运行，或分别配置不同会话目录。

## 必要验证

- `node --test server/session-server.test.mjs`：持久化 Cookie、加密存储、16 并发单次续期、重启恢复、断网与恢复、撤销、离线退出及跨源/伪造请求拒绝。
- `npx vitest run src/__tests__/session.spec.js`：浏览器请求合并、短期令牌存储、临时失败保留、明确撤销清理、退出与未完成检查的竞态。
- `node server/verify-session-live.mjs`：使用本机已配置的 seeded admin 创建独立测试会话，对真实 Atom 验证续期、重启与退出。只改该脚本自己的临时会话，结束时撤销并清理，不打印凭据，不触碰用户业务记录。

会话管理核心从本机官方 UI 的 remembered-session 补丁复用，转换为 ESM 以适配 Vite 配置加载；官方部署未在本轮重新构建或改动。
