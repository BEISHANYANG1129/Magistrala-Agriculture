# 官方界面的长期登录

本目录为 `http://localhost:3000` 的 Magistrala 官方社区界面提供持久化登录。农业 Vue 前端的登录实现不在此修改范围内。

## 已部署行为

- 默认记住当前浏览器，关闭浏览器、刷新页面、重新进入平台后恢复登录。
- Atom 保留一小时访问令牌，启用独立的旋转刷新令牌。访问令牌到期后仍然可以恢复，不依赖到期前两分钟的窗口。
- 页面打开、重新获得焦点、从后台恢复或网络恢复时检查会话；前台每分钟检查一次，提前五分钟续期。
- 断网、超时和后端临时错误保留登录，恢复连接后重试。确定已撤销的会话要求重新登录。
- 同一会话的并发续期在 UI 的单个 Node 进程中串行处理。服务端保存最新凭据，旧 cookie、多个标签页和 RSC 请求不会重复使用已消耗的刷新令牌。
- 主动退出会撤销 Atom 会话并清除浏览器 cookie。后端离线时先持久化本地撤销，恢复连接后补交撤销请求。
- 刷新令牌用 AES-256-GCM 加密，保存在 `absmach_magistrala_magistrala-ui-sessions-volume`。不会返回到浏览器的 session JSON，也不保存用户密码。

切换机制前的旧登录没有独立刷新令牌，需要重新登录一次。此后使用新机制。

## 有效期与实际限制

浏览器会话为 400 天，每次访问/检查时续延。Chrome 对 cookie 的最长单次保存时间限制为 400 天，因此连续超过 400 天不使用平台、浏览器自行清理站点数据或使用无痕模式时，仍可能需要重新登录。

后端刷新令牌家族有效期配置为 `3153600000` 秒，约 100 年。这是当前 Atom 有期限会话接口下的长期配置，并非数学意义的永不过期。停用账号/租户、撤销会话、轮换加密密钥或丢失会话数据卷同样会使登录失效。

会话卷必须保留，`NEXTAUTH_SECRET` 与 `ATOM_KEY_ENCRYPTION_KEY` 应各自保持稳定。当前实现支持这个本地部署的单个 UI 实例；增加 UI 副本前，需要换成共享数据库和跨进程的续期锁。

## 构建与部署

官方 UI 源码仓库在本次环境中不可读取，采用当前部署镜像的固定 digest 派生镜像。`patch-ui.cjs` 校验已知构建结构，修改两份服务端认证模块和客户端恢复逻辑；客户端文件使用新内容 hash，并更新所有引用，避免已打开的浏览器继续使用缓存中的旧退出逻辑。

在 `D:\personal\magistrala\magistrala` 构建：

```powershell
docker build --pull=false -t magistrala-ui-remembered:20261011 docker/ui-remembered-login
```

使用现有 WSL Compose 项目和两个环境文件更新这两个服务：

```powershell
wsl -d Ubuntu-20.04 --cd /mnt/d/personal/magistrala/magistrala/docker docker compose -p absmach_magistrala --env-file .env --env-file .env.tokens up -d --no-deps atom ui
```

`.env` 中已设置 `MG_UI_IMAGE=magistrala-ui-remembered:20261011`、`ATOM_REFRESH_TOKENS_ENABLED=true`、一小时访问令牌与约 100 年刷新家族期限。Compose 已声明会话卷和相关环境变量。

升级官方 UI 镜像时，需要先核对新的构建结构并调整补丁，再完成测试；不能直接把本派生镜像替换为未经修改的 `latest`。

## 验证

```powershell
node --test docker/ui-remembered-login/remembered-session.test.cjs
node docker/ui-remembered-login/verify-live.cjs
node docker/ui-remembered-login/verify-browser.cjs
```

9 项单元测试覆盖并发续期、加密持久化、重启恢复、断网重试、撤销、离线退出、工作区元数据与输入约束。真实接口验证会读取当前部署的 seeded admin 凭据，创建自己的测试会话，并验证失效恢复、16 个并发请求只旋转一次、UI 重启、后端撤销和退出撤销；不输出凭据。真实浏览器验证使用本机已有的 Playwright 与 Chromium，检查浏览器重开、页面刷新、工作区切换后的持久化 cookie 和离线/在线恢复。

2026-10-11 已在运行中的服务完成上述验证，测试会话均已退出。失效恢复测试只将自己创建的会话中的访问令牌替换为失效值，保留刷新凭据，验证后端仍能恢复；没有等待一小时，也没有改动用户的已有会话。

接口验证会重启 UI 容器。生产使用时不要在业务繁忙期间运行它。测试只改动自己创建的会话，不缩短其他用户令牌的有效期。

## 回退

把 `MG_UI_IMAGE` 改为原始固定镜像 `ghcr.io/absmach/magistrala/ui-mg@sha256:18aaf940b8da15b1cbfc1efdd726684e78adb00e8896cb248abf42541b8053eb`，将 `ATOM_REFRESH_TOKENS_ENABLED` 改为 `false`，然后运行上面的 Compose 更新命令。回退后需要重新登录。无需删除数据库或会话卷。
