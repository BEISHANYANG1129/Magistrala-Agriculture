# Magistrala Agriculture 智慧农业平台

基于 Magistrala / Atom 的农业物联网管理平台，包含完整的 `magistrala/` 核心源码、Vue 农业管理界面、区域分析工作台、图片上传、农业控制服务，以及可选的第三方农业数据集成和设备执行服务。

本仓库直接包含两部分源码，不使用 Git 子模块。普通 `git clone` 即可取得项目。默认部署使用固定镜像摘要的 Magistrala 核心，并从本仓库构建农业界面和区域服务。

## 功能

- 中英文界面；账户登录、注册、工作区切换和服务端加密的记住登录会话。
- 设备、通道、连接授权、分组、设备类型、成员、角色和权限管理。
- 消息查询与 SenML 发送、数据图表、消息视图、规则、告警、报表和设备引导。
- 农田地图、区域配置、设备绑定、传感器证据、分析建议及分析历史。
- 区域工作台默认 `reader-demo`：读取真实 Reader 数据，使用模拟分析器，执行结果为模拟回执。默认部署不需要 Ollama 或 GPU，也不会自动控制真实阀门。

功能是否可用取决于账户权限、数据配置和后端版本。规则、仪表盘等功能可能受到上游版本配额限制。第三方农业平台、通知服务、真实设备执行和真实模型推理需要单独配置。

## 快速部署：Docker

建议在 Linux x86_64 或 Windows Docker Desktop（Linux 容器）上部署。需要 Git、Node.js 22.12+（推荐 Node.js 22 LTS 或 24）、Docker Engine 与 Docker Compose v2。初始化优先使用本机 OpenSSL；未安装时会用 Docker 自动生成证书。完整平台包含多种数据库和三节点消息集群，建议至少预留 4 CPU、8 GB 内存和 15 GB 磁盘；这是部署建议，不是已测定的最低配置。

```bash
git clone https://github.com/BEISHANYANG1129/Magistrala-Agriculture.git
cd Magistrala-Agriculture

# 生成当前部署自己的密码、令牌、证书和可选服务配置
node scripts/deploy.mjs init

# 校验 Compose 配置
node scripts/deploy.mjs config

# 构建两个界面镜像，按依赖顺序启动完整平台
node scripts/deploy.mjs up

# 查看服务状态
node scripts/deploy.mjs status
```

以上命令在 PowerShell、Linux 和 macOS 终端中均可执行。首次构建需要联网访问 GitHub / GHCR、Docker Hub、npm 和 Go 模块代理。核心预构建镜像的主要验证平台是 x86_64；ARM 设备可能需要 amd64 模拟运行，或自行确认每个镜像的架构支持。

| 入口 | 默认地址 | 用途 |
| --- | --- | --- |
| 农业管理界面 | http://127.0.0.1:5173 | 推荐使用入口，包含登录会话代理和区域服务 |
| 上游 Magistrala UI | http://localhost:3000 | 上游界面，包含本仓库的会话与 Bootstrap 兼容补丁 |
| Atom UI | http://localhost:3005 | Atom 原生管理界面 |
| Atom API | http://localhost:8080 | 身份与资源 GraphQL API |
| 平台网关 | http://localhost | HTTP 消息和其他网关接口 |
| Timescale Reader | http://localhost:9011 | 消息历史查询 |

首次登录用户名为 `admin`，密码是本机 `magistrala/docker/.env` 的 `ATOM_ADMIN_SECRET`。每次新的部署随机生成，脚本不会把密码写入终端。请在本机编辑器中查看该项，再登录农业管理界面。也可以从登录页注册自己的账户。

`init` 重复运行会保留已有配置、令牌和状态。修改 `.env.example` 不会自动修改已初始化的 `.env`；调整部署参数应编辑自己的 `.env`。`up` 先关闭 Atom 事件发布启动消息基础设施，再启用完整配置，以处理全新环境的启动依赖。

### 常用管理命令

```bash
node scripts/deploy.mjs logs agriculture
node scripts/deploy.mjs logs atom
node scripts/deploy.mjs down
node scripts/deploy.mjs up
```

`down` 停止并移除本部署的容器和网络，保留数据库、区域历史和登录会话的数据卷。不要在需要保留业务数据时执行 `docker compose down -v`。备份应包含数据库数据卷、`agriculture-state`、上游 UI 会话卷，以及本地 `.env`、`.env.tokens`、`atom-bootstrap.yaml`、证书和 broker secrets；恢复时使用同一套密钥与令牌。

## 首次使用

1. 打开农业界面，登录后创建或选择自己的工作区。
2. 创建设备和通道，为设备授予对应通道的发布／订阅权限。消息发送时使用你自己设备的密钥。
3. 在“消息与视图”发送或查询数据。消息接收与持久化是不同环节，Reader 数据还需要正确的存储规则。农业界面不会自动创建生产存储规则。
4. 在“农田地图”设置位置和分区，在“区域与 AI”创建区域、绑定当前工作区的传感器与通道、配置阈值。
5. 有足够新鲜的传感器数据后发起分析。缺少数据或数据过期时，服务会拒绝分析。执行按钮返回模拟结果；不会驱动实际农业设备。

新部署从空区域配置和空设备映射开始，不携带维护者的工作区、真实设备密钥、历史记录或登录状态。完整功能说明和历次验收边界见 [农业界面说明](docs/AGRI-UI-UPGRADE.md) 与 [区域服务说明](llm/agentv2/README.md)。历史验收记录中的本机路径和地址是当时的测试环境，不是新部署的配置要求。

## 源码目录

```text
Magistrala-Agriculture/
├── magistrala/          # 完整 Magistrala 核心源码与 Docker 部署配置
├── magistrala-ui/       # Vue 3 / Vite 农业界面、API 契约、登录会话服务
├── llm/agentv2/         # 区域分析工作台和 Go 服务
├── llm/                 # 旧版 Ollama / RAG / 任务规划服务
├── agriControlService/  # 农业任务规划和控制 API
├── agriDataIntegration/ # 可选第三方农业数据集成服务
├── agriDeviceExecutor/  # 可选真实设备执行服务
├── image_upload/        # 图片上传 Go 服务
├── liar/                # 历史消息发送和查询示例
├── data/                # 共享配置模板
├── deploy/              # 农业界面及区域服务的多阶段 Dockerfile
├── scripts/             # 初始化、部署、跨平台启动和构建产物服务
└── docs/                # 设计与功能说明
```

`magistrala/docker/compose.pinned.json` 固定核心服务镜像摘要；`compose.agriculture.yaml` 增加农业应用容器。上游 UI 补丁从公开基础镜像构建，不依赖维护者电脑里的私有镜像。核心源码包含上游当时使用的 Go 1.27 要求；常规 Docker 部署直接使用固定的核心镜像，不要求本机安装 Go 1.27。农业 Go 模块使用 Go 1.24.4+，Docker 构建使用 Go 1.24.13。

## 本地开发

先运行 `node scripts/deploy.mjs init`。仅开发前端时，可以让 Docker 栈保持运行，停止该栈的农业容器以释放 5173，然后使用以下命令；完整 Compose 参数写法见 [部署说明](docs/DEPLOYMENT.md)。

```bash
cd magistrala-ui
npm ci
npm run dev
```

在另一个终端，从仓库根目录启动区域服务（需要 Go 1.24.4+）：

```bash
node scripts/run-local.mjs agent-ui
```

开发前端默认连接本机 Atom / Reader 等服务。配置与生产容器中的内网地址不同；不要在已有完整 Docker 部署占用端口时重复启动相同服务。

其他模块可独立运行，每个命令占用一个终端：

```bash
node scripts/run-local.mjs image
node scripts/run-local.mjs control
# 以下为可选功能，启动前完成自己的配置
node scripts/run-local.mjs llm
node scripts/run-local.mjs integration
node scripts/run-local.mjs executor
```

Windows 还可使用 `powershell -ExecutionPolicy Bypass -File .\run-local.ps1 ui` 等兼容入口。启动脚本使用 PATH 中的 Node / Go，不依赖固定的 Windows 磁盘目录。

构建前端并使用 Node 提供构建后的页面、API 转发和会话服务：

```bash
cd magistrala-ui
npm ci
npm run build
cd ..
node scripts/serve.mjs
```

上述 Node 命令默认只监听 `127.0.0.1:5173`；区域服务另行运行。不能只把 `dist/` 上传到静态文件服务器后就期望记住登录和 API 代理正常工作。

### 可选第三方集成和真实模型

`init` 从 `*.example.json` 复制本地配置。第三方平台账号、密码、令牌和工作区／通道 ID 需要自行填入以下文件：

- `data/magistrala.json`：旧模块使用的基础地址和用户令牌。
- `agriDataIntegration/config.json` 与 `sensor_mapping.json`：第三方账号和传感器映射。
- `agriDeviceExecutor/internal/config/config.json` 与 `internal/data/executor_mapping.json`：第三方执行器配置。
- `data/device_registry.json` 和 `llm/config/config.json`：旧控制／LLM 流程的设备映射和模型参数。

这些本地文件已被 Git 忽略。旧第三方集成仍使用历史 REST API 和字段，需针对你部署的 Atom 版本及供应商接口自行适配；它们不属于默认容器栈已经验收的自动集成。历史 `liar` 示例使用 `MG_DEVICE_SECRET`、`MG_WORKSPACE_ID`、`MG_CHANNEL_ID` 环境变量。

需要真实模型时，请按 [agentv2 README](llm/agentv2/README.md) 配置 Ollama 与模型。主机开发模式可设置 `AGRI_UI_MODEL=gemma3:12b` 再启动 `agent-ui`。当前 Docker 快速部署保持 `reader-demo`，没有 GPU 检查器或模型；修改真实模型部署需要额外配置 Python、GPU 检查和 Ollama 地址。旧版 LLM 模块还使用 `nomic-embed-text`。模型不会自动下载。

## 服务器部署与常见问题

农业容器默认只把 5173 暴露到本机。远程访问可使用 HTTPS 反向代理；设置 `AGRI_COOKIE_SECURE=true`，保留原始 Host/Origin，并在 Atom CORS 配置中加入真实访问来源。具体示例、端口冲突、网络代理、版本配额和登录故障处理见 [部署说明](docs/DEPLOYMENT.md)。

核心 Compose 沿用上游的固定容器名、多个主机端口和 `172.30.0.0/24` 消息集群网段。默认脚本适用于同一 Docker 主机上的一个实例；已有 Magistrala 实例或同网段网络时，应先调整完整配置，不能只改 Compose project name。对外部署时应通过防火墙限制数据库、broker 管理和后端端口。

## 验证与维护

```bash
cd magistrala-ui
npm run build
npx vitest run
node --test server/session-server.test.mjs
cd ../llm/agentv2
go test -p 1 ./...
go vet ./...
```

仓库提供 GitHub Actions，检查农业前端构建、单元测试、登录会话、区域 Go 服务和部署初始化／Compose 配置。提交前的完整镜像与隔离部署验证结果记录在 [部署验证](docs/DEPLOYMENT-VALIDATION.md)。

## 来源与许可

农业应用基于 [liar557/Magistrala](https://github.com/liar557/Magistrala) 的项目，并包含当前工作区中的农业管理界面及区域服务修改。`magistrala/` 基于 [absmach/magistrala](https://github.com/absmach/magistrala)，保留原始版权、SPDX 标识及 [Apache-2.0 许可](magistrala/LICENSE)。整合来源提交见 [SOURCE-PROVENANCE.md](docs/SOURCE-PROVENANCE.md)。农业项目和外部镜像的使用许可应以各自原始声明为准，核心许可不自动代表所有农业扩展和外部组件的许可。
