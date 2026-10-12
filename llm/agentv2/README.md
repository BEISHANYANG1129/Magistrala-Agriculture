# 农田区域智能体：独立 Go 原型

本目录是独立 Go module，不替换正在运行的旧 LLM 服务。使用社区维护的 `github.com/tmc/langchaingo v0.1.15` 接入 Ollama。当前完成区域证据读取、专家路由、结构化分析、策略校验、建议生成和模拟执行；不是已接管真实设备的生产系统。

## 2026-10-10 农业平台集成更新

农业前端现已原生整合本模块，新增带平台鉴权的集成启动方式：在项目根目录运行 `powershell -ExecutionPolicy Bypass -File .\run-local.ps1 agent-ui`。默认 9025 端口、`reader-demo`，使用 `runtime/agri-ui/` 独立状态目录。完整入口与验证边界见 [农业平台说明](../../docs/AGRI-UI-UPGRADE.md)。下面 10 月 8 日的表格是当时的历史结果；当前 ui-backend 已运行。

新增 `-platform-url` 启用 Atom 工作区权限核验。集成 API 必须带当前用户 Bearer 令牌及 `X-Workspace-ID`；Reader 使用该用户令牌。读取、历史、分析和模拟回执受工作区约束；编辑要求管理权限并核验传感器/通道归属。空区域列表现在是合法配置。区域文件仍只允许一个服务实例使用。

`AGRI_UI_MODEL` 非空时，启动脚本选择真实 Ollama 模式。集成模式每次专家调用前通过 `-gpu-python` 指定的 Python 执行 `-gpu-guard` 检查脚本；显卡忙碌或检查失败则不调用模型，用户稍后重试。默认模式不会触发 GPU。本轮新增鉴权与隔离测试通过，没有重新执行模型推理或真实设备控制。

## 当前验证结果（2026-10-08）

| 范围 | 结果 |
| --- | --- |
| 单元及 HTTP 模拟测试 | 21 个顶层测试通过（含子测试共 41 项 pass），两个真实平台/模型入口默认跳过；新增模型阈值矛盾回归检查 |
| 原型 UI | 区域创建/编辑、绑定修改、分析历史、控制计划和模拟回执已通过浏览器验证 |
| 平台管理 UI | 新建独立工作区、组、设备、通道连接已验证 |
| 平台消息 API | Basic 设备认证上传成功，Reader Bearer 认证读取成功 |
| Rule 存储 | 经用户授权扩展 test5 后，专用测试设备的合成数据进入 test3 并成功读回 |
| Go Reader 真实联调 | 同一通道两个真实测试发布者：北区 12% 只关联北区阀门，南区 35% 无建议；另有无数据区域验证 |
| 旧控制模块协议 | 在隔离源码副本中运行原 HTTP handler、planner、executor，通过真实适配器生成的任务验证；原文件哈希未变 |
| 重启恢复 | 独立预览实例重启后 3 条记录完全一致，同一分析返回原模拟回执，不重新执行 |
| 真实 Ollama 推理 | gemma3:12b 最终 6 个场景通过，含 5 次真实推理；北区、南区和缺失阈值使用真实 Reader 的合成样本，其余为隔离数据；失败案例及修复见 PHASE2-RESULTS.md 末尾 |
| 官网 UI 消息页 | 仍受 ui-backend 未运行/DNS 不可解析阻塞，已保存现场；未改动共享部署 |
| 其他模块 | agriControlService、agriDataIntegration、agriDeviceExecutor、image_upload 编译通过；不等于真实接口兼容性已全部验证 |

平台联调记录见 [PLATFORM-VALIDATION.md](PLATFORM-VALIDATION.md)，本阶段完整结果见 [PHASE2-RESULTS.md](PHASE2-RESULTS.md)。`runtime/` 和 `bin/` 被忽略；其中 session 文件包含测试设备密钥，不要提交或分享。

## 运行

需要 Go >= 1.24.4。本机 Go 位于 `D:/personal/magistrala/.tools/go/bin`。

```powershell
Set-Location D:/personal/magistrala/main/Magistrala/llm/agentv2
$env:GOMAXPROCS = '2'
& D:/personal/magistrala/.tools/go/bin/go.exe test -p 1 ./...
& D:/personal/magistrala/.tools/go/bin/go.exe run -p 1 ./cmd/agent -mode demo
```

程序打印随机空闲的本机端口，打开该地址即可。演示模式不会访问模型或真实设备；示例阈值 20% 仅用于验证软件流程，不是农艺标准。

需要区域编辑和历史恢复时，先将示例配置复制到自己专用的配置文件（不要覆盖现有配置），然后指定 `-registry <专用文件> -edit-regions -state-dir <专用历史目录>`。每个目录只供一个实例使用；多实例共享文件不受支持。独立模式没有平台多用户权限；需要该能力时使用上方集成模式。

等待 GPU 时可使用 `-mode reader-demo -reader http://localhost:9011 -time-unit nanoseconds`，并安全注入 `AGRI_READER_AUTH`。该模式读取真实平台数据，但分析器固定为模拟分析器，不请求 Ollama。

接入平台和 Ollama 前，先检查 `nvidia-smi`、计算进程及 Ollama `/api/ps`。若其他任务使用显卡或无法确认空闲，应继续等待。不要停止其他任务、卸载其模型或重启共享服务。

```powershell
# 用户令牌由运行者安全注入，不要写进代码或命令历史。
# $env:AGRI_READER_AUTH = 'Bearer <user_token>'
& D:/personal/magistrala/.tools/go/bin/go.exe run -p 1 ./cmd/agent `
  -mode ollama -reader http://localhost:9011 -time-unit nanoseconds `
  -registry runtime/test3-registry.live.json -model gemma3:12b
```

该模式使用真实 Reader 和真实模型，但输出仍然只是建议，执行按钮仍然是模拟。每次分析最多顺序调用土壤、环境两个专家，服务器同一时间只接受一个分析请求。服务仅绑定 loopback。

## 区域与工作流

`examples/registry.json` 定义 `workspace_id + region_id`、传感器的 `channel_id + publisher_id`、区域执行器 ID 和显式 `irrigation_duration_min`。界面保存时自动生成新版本；手工编辑时应更新 version。每次分析重新加载配置，并以内容哈希检查推理过程中是否发生变更；因此只修改配置而忘记改版本也会拒绝使用旧结果。

1. 校验工作区、区域和映射版本。
2. 只读取该区域绑定设备的证据，分页读取并在本地再次核验发布者。
3. 排除过期和异常未来时间，校验数值与单位，每个设备指标保留最新值。
4. 按指标路由到土壤或环境专家，检索最多四条人工整理知识。
5. 通过 LangChainGo 调用模型，严格解析 JSON，核验区域、映射、证据及知识引用。
6. 确认映射与证据仍有效，再由确定性策略生成建议。
7. UI 可模拟执行并返回幂等结果；不会调用旧控制服务或发送物理设备命令。

知识检索目前是指标标签匹配，尚未接入向量检索。专家选择目前是确定性规则，不声称已经复现论文中的完整多智能体匹配算法。任何专家返回 uncertain 都会阻止生成灌溉建议。阈值缺失时只允许描述观察结果；控制适配器仅从区域配置读取灌溉时长，拒绝缺失、非正数和超过 60 分钟的配置。示例 5 分钟只用于模拟验证。

真实模型曾把 35% 误判为低于 20%，因此现在由应用计算 threshold_checks，并复核 soil_dry/normal 与引用数值是否相符；冲突会拒绝整个分析，不修饰成成功结果。Ollama 请求使用 JSON Schema 约束字段层级；model_schema.go 在当前 LangChainGo 仅支持字符串 format 的限制下，为此客户端注入 schema。仍保留严格解析和区域/证据校验，不会因为 schema 合法就信任分析含义。上下文设为 8192、keep_alive=0，请求结束后释放本次模型；这些是单次请求配置，不改变共享 Ollama 的全局配置。冷启动仍受 90 秒客户端超时约束，不能把后续缓存条件下的 9–10 秒耗时当作冷启动保证。

区域由服务端配置定义，地图 location/perimeter 可作为未来区域显示元数据，但不能替代区域、设备、执行器归属和访问权限。UI 支持区域创建、编辑、绑定配置、分析记录和模拟状态。独立原型页面不提供地图和平台管理；新版农业界面已提供这些入口，平台鉴权由上方集成模式启用。

## HTTP 接口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | /health | 当前模式和模拟执行标识 |
| GET | /v1/regions | 区域、版本、revision 和当前是否允许编辑 |
| POST | /v1/regions | `expected_revision` + `registry`，冲突返回 409；保存旧配置到 .backup.json |
| POST | /v1/analyze | `workspace_id`、`region_id`、`mapping_version` |
| POST | /v1/simulate | `analysis_id`，只接受当前服务保存的结果 |
| GET | /v1/history | 必须指定 workspace_id，可指定 region_id；按时间倒序 |

写请求要求 JSON、同源 Origin（存在时）和本机 Host。指定 state-dir 后将分析及模拟回执原子保存为 JSON，重启可恢复；模拟之前先保存开始状态，若中断则恢复为 unknown，不自动重试。最多保存 200 条记录，满时拒绝新增，不自动删除。文件未加密，也不是防篡改的生产审计系统。

`BuildControlPlans` 保留旧 `task_id`、`trace_id`、`task_type`、`target`、`params`、`source` 协议，将区域拆成一条设备任务一个目标，并生成稳定的设备任务 ID。平台预览仅调用 VirtualControl；没有到真实设备或共享控制服务的网络传输。旧控制代码的兼容性通过 `scripts/verify_legacy_contract.py` 在独立副本中验证。

旧控制接口的 `200 task executed` 实际代表入队，且同 ID 重复提交会再执行一次。不能将其当成完成回执，也不能去掉新模块的重复提交保护。当前模拟状态包括 completed、failed、unknown；失败或未知结果不会被显示为成功。

## 联调与回退

普通 `go test` 自动跳过 `TestLiveWorkflow`。显式设置 `AGRI_LIVE_REGISTRY` 和 `AGRI_READER_AUTH` 可验证真实 Reader；默认仍使用模拟分析器。只有同时设置 `AGRI_LIVE_MODEL`、`AGRI_GPU_IDLE_CONFIRMED=yes` 才会请求真实模型；该开关不是 GPU 检测工具，运行者必须刚刚完成空闲核验。

`scripts/test3_rule.py --include-south` 只修改已授权 test5 的脚本，为南北两个专用测试设备增加分支；保留原输入通道、主题、输出、名称、状态及元数据。若远端规则发生其他修改，会拒绝覆盖。`scripts/dual_region_smoke.py` 每个测试设备仅发送一条样本，已有区域配置不会被覆盖。回退命令：

```powershell
python scripts/test3_rule.py --env-file D:/personal/magistrala/magistrala/docker/.env --restore
```

回退只移除新增测试分支，不删除任何已入库消息、设备或通道。请先确认没有后续测试依赖该分支。

## 后续工作

- 已完成首轮小规模真实模型验证。后续需扩大真实农业数据、异常输入和重复运行的评估；本轮不代表农艺准确率或生产可靠性认证。
- 完善地图边界、正式权限及农艺知识库，再评估与现有 UI 的合并；当前区域管理独立运行。
- 对真实外部设备接口的端到端验证另行安排；编译和隔离模拟不代表物理设备验证通过。
- 在明确设备能力、控制策略和反馈协议后接入控制服务；真实执行需有幂等键、超时、反馈和区域归属复验。
