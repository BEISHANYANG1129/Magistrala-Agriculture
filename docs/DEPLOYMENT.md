# 部署与配置

仓库根目录的 `scripts/deploy.mjs` 是默认部署入口，使用完整核心 Compose、固定镜像摘要覆盖文件和农业容器覆盖文件。首次部署步骤见 [README](../README.md)。本文件说明配置、开发模式和故障处理。

## Compose 的完整参数

从仓库根目录执行。例如仅停止农业应用以使用本机 Vite 开发服务器：

```bash
docker compose -f magistrala/docker/docker-compose.yaml -f magistrala/docker/compose.pinned.json -f magistrala/docker/compose.agriculture.yaml --env-file magistrala/docker/.env --env-file magistrala/docker/.env.tokens stop agriculture
```

相同参数后接 `up -d agriculture` 可恢复农业容器。直接运行 Compose 时必须同时加载 `.env` 和 `.env.tokens`；否则核心服务无法用正确令牌连接 Atom。Docker 快速部署不需要从源码编译 Magistrala 核心。

## 配置文件与状态

- `.env.example` 是可提交的模板。`init` 复制为 `.env`，为标记项生成随机值；每个部署的管理员密码、数据库密码、加密密钥和 broker secret 独立生成。
- `atom-bootstrap.template.yaml` 是公开声明模板。初始化生成服务身份凭据到 `atom-bootstrap.yaml`，并把配对令牌写到 `.env.tokens`。两个生成文件须同时备份、同时恢复。
- `docker/ssl/certs/`、`docker/fluxmq/secrets/`、`docker/re/secrets/` 是本次部署的私有材料。开发 Compose 中证书与 broker secret 需要能被非 root 容器用户读取，因此生成文件的 Unix 权限为 0644；正式环境可按实际容器 UID/GID 调整为 0640，宿主机目录和备份仅允许管理人员访问。
- `agriculture-state` 数据卷包含农业登录会话加密密钥、区域配置、分析历史。会话服务和区域状态采用单实例文件存储；不能让多个活动实例共享同一状态目录。
- 公开仓库不会包含真实设备的密钥、工作区配置、用户会话、数据库备份或上传图片。默认区域为空，需要在自己的工作区内配置。

重复初始化不轮换已经存在的凭据。不要单独删除 `.env.tokens` 或 `atom-bootstrap.yaml`，也不要在运行平台时删除证书、broker secret 或会话密钥；这些操作会使现有身份、连接或会话失效。

## 服务器入口

默认农业入口绑定宿主机 `127.0.0.1:5173`。推荐在前面使用自己的 HTTPS 反向代理。例如 Nginx：

```nginx
server {
    listen 443 ssl;
    server_name farm.example.com;
    ssl_certificate /etc/letsencrypt/live/farm.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/farm.example.com/privkey.pem;
    location / {
        proxy_pass http://127.0.0.1:5173;
        proxy_set_header Host $host;
        proxy_set_header Origin $http_origin;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 160s;
    }
}
```

在 `magistrala/docker/.env` 添加 `AGRI_COOKIE_SECURE=true`，并将 `https://farm.example.com` 加到 `ATOM_CORS_ALLOWED_ORIGINS`。再运行 `node scripts/deploy.mjs up` 应用配置。`farm.example.com` 只是示例，需要自己的域名和 TLS 证书。

如需改变农业入口端口，在 `.env` 添加 `AGRI_PUBLIC_PORT=5175`。`AGRI_BIND_HOST=0.0.0.0` 会把农业入口暴露到所有网卡，只有在网络和 HTTPS 入口已经配置好时才应使用。默认核心 Compose 仍发布其他服务端口，需根据访问需求和防火墙限制数据库、broker 管理和后端 API。注册、未验证邮箱登录和速率限制沿用本地开发配置；正式对外环境应配 SMTP、验证邮箱策略和可信代理／限流。

## Docker 外运行农业界面

`scripts/serve.mjs` 提供构建产物、同源会话服务和 API 转发。支持以下环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `AGRI_HOST` / `AGRI_PORT` | `127.0.0.1` / `5173` | Node 页面服务监听地址和端口 |
| `AGRI_ATOM_URL` | `http://127.0.0.1:80` | 登录与 GraphQL 服务 |
| `AGRI_GATEWAY_URL` | `http://127.0.0.1:80` | 消息／证书网关 |
| `AGRI_AGENT_URL` | `http://127.0.0.1:9025` | 区域工作台 |
| `AGRI_READER_URL` | `http://127.0.0.1:9011` | Reader 查询 |
| `AGRI_BOOTSTRAP_URL` | `http://127.0.0.1:9010` | 宿主机 Bootstrap 入口；容器内端口是 9013 |
| `AGRI_RULES_URL` / `AGRI_ALARMS_URL` / `AGRI_REPORTS_URL` | 本机 9008 / 8050 / 9017 | 各功能后端 |
| `AGRI_UI_BACKEND_URL` | `http://127.0.0.1:9097` | 仪表盘／界面存储后端 |
| `AGRI_SESSION_DIR` | 系统用户数据目录 | 加密登录状态存放位置 |
| `AGRI_COOKIE_SECURE` | `false` | HTTPS 环境设置 `true` |

区域服务的默认模式为 `reader-demo`，只读实时数据并返回模拟建议和执行回执。Docker 中的 Node 服务会启动容器内区域二进制，两个进程共享容器但区域端口只监听 loopback；外部请求通过 `/api/agent` 访问，保留 Host/Origin 检查。区域服务退出时整个应用容器退出并由 Compose 重启。

## 常见问题

### 端口或容器名称冲突

默认 Compose 保留上游固定容器名及 `172.30.0.0/24` 网段，同一宿主机默认只运行一个实例。已有 Magistrala 部署时不要直接启动第二套默认栈；需一起修改容器名、网段、FluxMQ node 配置里的集群 IP 和暴露端口。只修改 project name 无法消除全部冲突。

### 构建或拉取失败

检查能否访问 GHCR、Docker Hub、npm 和 Go 模块代理。可在自己的 `.env` 中调整 `DOCKER_HUB_REGISTRY`；核心 GHCR 镜像摘要由 `compose.pinned.json` 指定。不要把摘要替换成不匹配的最新镜像，农业界面的 GraphQL 契约与当前固定 Atom 版本对应。

### 服务暂时不可用 / 502

先执行 `node scripts/deploy.mjs status`，再查看相应服务日志。首次启动数据库和三节点集群需要时间。`up` 已处理 Atom 事件发布对消息集群的冷启动依赖；服务的 restart 策略会重试暂时失败的连接。未部署的通知、图片或第三方集成入口也会显示服务不可用，需要另行启动并配置对应后端。

### 登录失败

确认使用本次初始化的 `ATOM_ADMIN_SECRET`，不是旧电脑的密码；保持固定访问域名，`localhost` 与 `127.0.0.1` 的 Cookie 不通用。代理必须保留正确 Host/Origin。请勿打印密码、JWT、刷新令牌或完整 `.env` 到公开日志。

### 没有消息或分析被拒绝

确认设备与通道授权、发送格式、Reader 存储规则、区域绑定和时间范围。空平台没有自动生成的生产遥测数据。分析拒绝过期或不足的数据是正常行为。真实阀门不会因默认模拟执行而动作。

### 仪表盘／规则配额

`dashboard limit for this edition reached` 等错误来自当前上游后端许可或配额。新界面不解除这些限制。根据所用版本配置和许可处理，不要通过删除已有业务资源来做部署验收。
