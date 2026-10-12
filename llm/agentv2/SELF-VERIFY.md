# 自行验收指南

本指南验证独立 Go 模块、新界面、区域隔离、平台 API、规则存储、历史恢复和旧控制协议。默认不调用 Ollama、不发送物理设备命令、不重启共享平台。真实模型能力另行验收。

## 1. 先查看现有结果（不修改配置）

打开 http://127.0.0.1:55512/ 。页面应显示“平台数据 / 模拟分析器”。北区历史中应有 12% 的证据与 simulation-only-valve-north 的模拟回执；南区应有 35% 数据且无任务；东区无数据。

GET http://127.0.0.1:55512/health 应返回 status=ok、mode=reader-demo、execution=simulation_only、history=persistent。

历史是之前的记录，不能证明当前消息仍新鲜。默认数据有效期为 15 分钟，过期后新分析返回 insufficient_data 是正确行为。

## 2. 新建自己的验收实例

打开 PowerShell，执行下面的初始化。保持此窗口，稍后恢复实例时会用到这些变量。

```powershell
Set-Location 'D:/personal/magistrala/main/Magistrala/llm/agentv2'
$GoTool = 'D:/personal/magistrala/.tools/go/bin/go.exe'
$PyTool = 'C:/Users/sysu/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe'
$PlatformEnv = 'D:/personal/magistrala/magistrala/docker/.env'
$env:GOMAXPROCS = '2'
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'off'
# 这些设置只作用于此 PowerShell 窗口，确保普通测试不触发已有的 live-test 环境配置。
$env:AGRI_LIVE_REGISTRY = ''
$env:AGRI_LIVE_MODEL = ''
$env:AGRI_GPU_IDLE_CONFIRMED = ''
$AcceptanceDir = Join-Path (Get-Location) ('runtime/self-check-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $AcceptanceDir | Out-Null
Copy-Item -LiteralPath 'runtime/dual-region-registry.json' -Destination (Join-Path $AcceptanceDir 'registry.json')
& $GoTool build -p 1 -o (Join-Path $AcceptanceDir 'agent.exe') ./cmd/agent
if ($LASTEXITCODE -ne 0) { throw 'Build failed; do not continue' }
Write-Output ('验收目录：' + $AcceptanceDir)
```

保留输出的验收目录。它包含你的配置和后续记录，与现有预览使用的文件不同。

### 更新两条合成样本

下面这一步**会向已授权的 test3 写入两条合成消息**：北区 12%、南区 35%。它不会新增规则、修改当前区域配置或调用模型。不要反复运行来制造大量样本。

```powershell
& $PyTool scripts/dual_region_smoke.py --env-file $PlatformEnv
if ($LASTEXITCODE -ne 0) { throw 'Platform sample verification failed' }
```

预期：north、south 两项的 publish_status 和 reader_status 都是 200，返回 publisher 不同、value 分别为 12 和 35。脚本已在内部精确匹配设备、指标、数值和时间；退出码 0 才算通过。

### 启动验收实例

令牌在 Python 进程内部取得并交给子进程，不在终端打印，也不写入验收文件。

```powershell
$LaunchCode = @'
import os, pathlib, subprocess, sys
sys.path.insert(0, 'scripts')
from platform_inspect import login
run_dir = pathlib.Path(sys.argv[1]).resolve()
env = os.environ.copy()
env['GOMAXPROCS'] = '2'
env['AGRI_READER_AUTH'] = 'Bearer ' + login(sys.argv[2])
command = [str(run_dir/'agent.exe'), '-listen', '127.0.0.1:0',
           '-mode', 'reader-demo', '-reader', 'http://localhost:9011',
           '-time-unit', 'nanoseconds', '-registry', str(run_dir/'registry.json'),
           '-state-dir', str(run_dir/'history'), '-edit-regions']
raise SystemExit(subprocess.call(command, env=env))
'@
& $PyTool -c $LaunchCode $AcceptanceDir $PlatformEnv
```

终端打印 `agent ready http://127.0.0.1:某端口 mode=reader-demo execution=simulation_only`。打开这次打印的地址，不能继续使用 55512 来做修改测试。端口由操作系统分配，避免占用其他服务的端口。保持这个终端运行。

## 3. 按顺序验收界面

先完成前三项基准测试，再修改配置。修改操作都在你自己的验收实例上进行。

| 操作 | 通过标准 |
| --- | --- |
| 选择北区 → 分析当前区域 | 依据为 north 设备的 soil_moisture=12%；有一项建议；没有 south 发布者证据 |
| 点击模拟执行建议 → 展开完整任务与回执 | 状态 simulated_completed；control_plans 中 task.target 为 simulation-only-valve-north；duration_min 为 5；不是 region ID，也不是南区设备 |
| 选择南区 → 分析 | 依据为另一个发布者的 35%；状态 analyzed；tasks 为空；模拟按钮不可用 |
| 选择东区 → 分析 | insufficient_data；证据为空；无任务；无专家调用轨迹 |
| 返回北区，查看分析历史 | 能打开刚才的结果及回执；查看历史不新增分析记录，不重新执行 |
| 编辑北区，名称加“自行验收”，保存 | 名称和配置版本变化；刷新后仍然存在 |
| 新增区域：ID=manual-empty，名字=手动无数据验收；工作区不变；传感器填 `3e692d54-5ccc-4a83-9cfd-ac0080263327, manual-missing-sensor`；执行器、阈值、时长留空 | 保存成功；能切换到新区域；分析为 insufficient_data，无任务 |

示例阈值 20%、时长 5 分钟只是软件测试参数，不是现场农艺建议。

## 4. 验收拒绝条件

| 操作 | 通过标准 |
| --- | --- |
| 编辑南区，把执行设备改成北区的 simulation-only-valve-north，保存 | 被拒绝，提示执行器重复/归属歧义；刷新后南区原绑定未变 |
| 编辑北区，清空模拟灌溉时长，保存，再分析北区并模拟 | 分析仍可提出建议，但模拟返回 explicit region irrigation duration required，不报告模拟成功 |
| 将北区时长恢复为 5，再分析；暂不模拟；随后修改北区名称并保存 | 旧分析不得作为当前配置执行，API 返回 409；用下面的 API 步骤可精确验证 |
| 浏览器两个标签页打开同一个验收实例，均点击编辑；A 保存，再在未刷新的 B 保存不同名字 | B 被拒绝，提示 configuration changed；不能覆盖 A 的修改 |

同一个区域的传感器允许被另一区域显式共享；执行器在同一工作区内不允许同时绑定多个区域。这是当前配置规则。

## 5. 使用 API 核对幂等、版本和范围

打开第二个 PowerShell 窗口。把 Base 换成你的验收实例端口。以下操作仅请求独立 Go 服务；分析会只读平台，模拟不发送设备命令。测试开始前将北区时长恢复为 5，并确保样本未过期。

```powershell
$Base = 'http://127.0.0.1:替换为实际端口'
$Workspace = 'cccd00c1-a12c-4d6e-9aa9-d2c52906f2e5'
$Config = Invoke-RestMethod "$Base/v1/regions"
$AnalyzeBody = @{
    workspace_id = $Workspace
    region_id = 'north'
    mapping_version = $Config.version
} | ConvertTo-Json
$Analysis = Invoke-RestMethod "$Base/v1/analyze" -Method Post -ContentType 'application/json' -Body $AnalyzeBody
$Analysis | ConvertTo-Json -Depth 12
$SimBody = @{ analysis_id = $Analysis.analysis_id } | ConvertTo-Json
$First = Invoke-RestMethod "$Base/v1/simulate" -Method Post -ContentType 'application/json' -Body $SimBody
$Second = Invoke-RestMethod "$Base/v1/simulate" -Method Post -ContentType 'application/json' -Body $SimBody
($First | ConvertTo-Json -Depth 20 -Compress) -eq ($Second | ConvertTo-Json -Depth 20 -Compress)
```

最后应为 True。两次回执的设备任务 ID、完成时间和内容应一致，不只是状态字符串相同。

验证旧分析失效：重新运行上面生成 Analysis 的几行，**不要模拟**；去 UI 保存一次区域名称修改，然后使用这个新 Analysis 的 ID 请求 /v1/simulate。应得到 HTTP 409 / mapping changed。已模拟完成的历史请求仍返回旧回执，这是幂等回放，不属于重新执行。

验证不存在的区域：将 AnalyzeBody 的 region_id 改为 `does-not-exist`，保留当前配置版本，POST /v1/analyze。应为 HTTP 422 / region does not exist，不生成任务。

读取分区历史：

```powershell
Invoke-RestMethod "$Base/v1/history?workspace_id=$Workspace&region_id=north" | ConvertTo-Json -Depth 20
Invoke-RestMethod "$Base/v1/history?workspace_id=$Workspace&region_id=south" | ConvertTo-Json -Depth 20
```

结果的 result.region_id 和证据发布者应与所查区域对应。

## 6. 验收真实重启恢复

1. 记下北区一条已模拟完成记录的 analysis_id 和 completed_at。
2. 回到你启动验收实例的第一个 PowerShell，按 Ctrl+C。只停止自己刚启动的实例，不执行任何 docker stop、compose down 或批量进程结束命令。
3. 在同一个窗口重新运行 `& $PyTool -c $LaunchCode $AcceptanceDir $PlatformEnv`。必须复用相同 AcceptanceDir，不能再次生成新 GUID。
4. 打开新打印的端口。名称、区域绑定和分析历史应恢复。
5. 将第二个窗口的 Base 改为新端口，用原 analysis_id 再 POST /v1/simulate。应返回原 completed_at 和原回执。

如果窗口已关闭，可重新初始化 GoTool/PyTool/PlatformEnv/LaunchCode，手工把 AcceptanceDir 设为之前记录的完整路径；不要再次覆盖 registry.json。

## 7. 跑自动化检查

在不运行服务的第二个终端执行。清空 live-test 开关后，普通测试只使用临时文件和本机模拟 HTTP，不访问 Ollama。

```powershell
Set-Location 'D:/personal/magistrala/main/Magistrala/llm/agentv2'
$GoTool = 'D:/personal/magistrala/.tools/go/bin/go.exe'
$PyTool = 'C:/Users/sysu/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe'
$env:GOMAXPROCS = '2'
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'off'
$env:AGRI_LIVE_REGISTRY = ''
$env:AGRI_LIVE_MODEL = ''
$env:AGRI_GPU_IDLE_CONFIRMED = ''
& $GoTool test -p 1 -count=1 -v ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
node --check web/app.js
& $PyTool scripts/verify_legacy_contract.py --go $GoTool
if ($LASTEXITCODE -ne 0) { throw 'Legacy contract test failed' }
```

预期：Go 输出 PASS/ok，TestLiveWorkflow 为 SKIP（本次是有意跳过），无 FAIL。当前源码对应 20 个顶层测试通过，含子测试共 36 个 pass。Node 语法检查成功时无输出。旧控制隔离测试输出 PASS 和 Original source unchanged: True。

旧控制测试会产生第二组相同任务的开关日志，这是故意验证“旧模块不防重复提交”的现有限制，不是本次测试失败。脚本只操作 runtime 下的新源码副本，模拟执行器只打印日志；不连接已运行的控制服务。

普通测试还覆盖模型虚构区域/证据/知识引用、过期和未来时间、Reader 分页与零值、LangChainGo 到模拟 Ollama 的请求、推理期间映射改变、错误回执及历史文件损坏。它们验证程序约束，不证明真实模型的判断质量。

如果要复核之前报告的其他四个模块“能编译”，可运行以下命令。只构建、不启动服务；失败时保留输出，不自动改依赖或共享配置。

```powershell
foreach ($ModuleName in @('agriControlService','agriDataIntegration','agriDeviceExecutor','image_upload')) {
    Push-Location -LiteralPath (Join-Path 'D:/personal/magistrala/main/Magistrala' $ModuleName)
    try {
        Write-Output ('检查模块：' + $ModuleName)
        & $GoTool build -mod=readonly -p 1 ./...
        if ($LASTEXITCODE -ne 0) { throw ('Build failed: ' + $ModuleName) }
    } finally { Pop-Location }
}
```

审查实现时，可按这张表定位：

| 功能 | 主要文件 |
| --- | --- |
| 区域配置及校验 | types.go、registry.go |
| 平台消息读取与单位转换 | reader.go |
| 专家路由、证据校验、建议生成 | engine.go |
| LangChainGo/Ollama 适配、模拟分析器 | model.go |
| 区域到执行器协议转换 | control.go |
| HTTP 接口、保存配置、模拟幂等 | server.go |
| 历史持久保存与恢复 | storage.go |
| 新增界面 | web/index.html、web/app.js |
| 自动化检查 | agent_test.go、workflow_test.go、live_test.go |

## 8. 核对平台实际改动

打开 http://localhost:3000 ，进入 test 工作区：

- test3 → Connections：原 test2、test4 应仍在；新增 llm-go-test3-sensor-20261008 和 llm-go-test3-south-20261008，均仅有 Publish。
- test5：仍启用，输入为 test3，输出为 Internal DB / save_senml。Lua 前面是对两个测试 publisher 的匹配，后面保留原 `message.payload[1].v == 20` 筛选。
- API 真实上传与读取证据查看 runtime/dual-region-publish.json。上传 t 是秒，Reader time 是纳秒；不能直接用整数原值比较。

官网消息页缺少 ui-backend 的部署问题尚未修复。该页面报 fetch failed 不能单独判定入库失败，应以脚本精确读回结果为准；也不能据此声称官方 UI 全流程已经通过。

## 9. 回退和验收边界

自己的验收实例：按 Ctrl+C 停止即可；保留独立验收目录用于查看证据，不会改变正在运行的原工作台。

test5 回退不是普通验收步骤。只有你决定移除两个测试设备的存储分支时，才执行：

```powershell
& $PyTool scripts/test3_rule.py --env-file 'D:/personal/magistrala/magistrala/docker/.env' --restore
```

它会保留原 v==20 逻辑，不删除历史消息或设备，但之后 12/35 两个合成值不再按测试分支入库。若规则已被其他人更改，脚本会拒绝覆盖；此时不要强行编辑快照绕过检查。

2026-10-08 晚间已完成 gemma3:12b 小规模真实推理验证，原始输入、输出、耗时和失败案例见 PHASE2-RESULTS.md 末尾。这不等于农业判断准确率评估；物理设备执行、地图边界、正式多用户权限、生产级持久队列或向量知识检索仍未完成。GPU 空闲前不要使用 -mode ollama 或设置 AGRI_LIVE_MODEL；普通 CLI 的 yes 开关不会自动检查显卡。新增 scripts/evaluate_model.py 会在每次真实推理前检查 GPU 和 GPU 容器，并拒绝与已有 Ollama 模型共用。

如需复现本轮模型验证，先按前文准备 Go/Python 路径，并在已授权范围内更新两条合成样本，再执行以下命令。它会消耗 GPU，不能当成普通无模型验收反复运行。输出保存在新的 runtime/model-eval-时间戳目录；不会修改现有工作台配置、启动常驻模型界面或向物理设备派发任务。

```powershell
& $GoTool test -c -o runtime/model-eval.test.exe .
if ($LASTEXITCODE -ne 0) { throw 'Evaluation build failed' }
& $PyTool scripts/evaluate_model.py --env-file $PlatformEnv
if ($LASTEXITCODE -ne 0) { throw 'Evaluation stopped; inspect printed report directory and GPU state' }
```

脚本检测到占用时最多短暂等待 20 秒，然后停止，不会停止其他进程或卸载其他任务的模型。通过 --cases south 等参数可仅重测一个场景。直接运行 Go 的 opt-in 入口仍需要操作者负责 GPU 检查，优先使用此脚本。

不要分享 runtime/*session.json 或平台 .env，其中包含测试设备密钥或登录凭据。
