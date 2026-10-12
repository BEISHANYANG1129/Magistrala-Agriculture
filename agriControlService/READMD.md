# Agri Control Service (MVP)

> 一个 **只负责接收 LLM 决策结果并安全执行的智慧农业控制服务**。
>
> 本服务的核心目标是：
> **让 LLM 只负责“想做什么”，而不是“怎么做”，也永远不直接接触设备。**

---

## 一、项目背景与目标

在智慧农业系统中，通常会引入大模型（LLM）用于：
- 环境分析
- 风险判断
- 农业决策建议

但 **LLM 天生不适合直接控制真实设备**，原因包括：
- 输出不稳定
- 无法保证安全边界
- 难以审计与回放

因此，本项目的目标是：

> 构建一个 **位于 LLM 与真实设备之间的“控制服务”**，
> 通过严格的结构设计，确保系统 **安全、可扩展、可维护**。

---

## 二、核心设计原则（非常重要）

### 1️⃣ LLM 永远不直接控制设备

- LLM **只能输出抽象的 Task（任务）**
- 不知道：
  - 设备 ID
  - 控制协议
  - HTTP / MQTT / Modbus

### 2️⃣ Executor 永远稳定

- Executor 只关心：
  - 设备
  - 动作
  - 参数
- **新增农业场景 ≠ 修改执行代码**

### 3️⃣ 农业语义与工程实现彻底解耦

```
农业场景 / 决策     ← 可频繁变化
──────────────
控制服务（本项目）   ← 稳定核心
──────────────
设备执行层          ← 极少变化
```

---

## 三、整体架构概览

```
        LLM / Rule Engine
               │
               ▼
        Task (JSON)
               │
               ▼
    ┌──────────────────────────┐
    │   Agri Control Service   │
    │                          │
    │  Policy → Planner → Exec │
    └──────────────────────────┘
               │
               ▼
        Real Devices / API
```

本项目 **只实现中间这一层**。

---

## 四、目录结构说明

```
agri-control-service/
├── cmd
│   ├── server                # 服务入口
│   │   └── main.go        
│   ├── replay                # 日志回放工具
│   │   └── main.go        
├── internal/
│   ├── api/                  # HTTP API
│   │   └── handler.go
│   ├── model/                # 核心数据结构
│   │   └── types.go
│   ├── registry/             # Task → Action 注册表（核心扩展点）
│   │   └── registry.go
│   ├── planner/              # 行为规划器
│   │   └── planner.go
│   ├── policy/               # 控制策略与安全约束
│   │   └── policy.go
│   ├── executor/             # 设备执行器（永远不改）
│   │   └── executor.go
│   ├── logstore/             # 执行日志存储（JSONL + 加锁写入）
│   │   └── logstore.go
│   └── service/              # 控制服务主流程
│       └── control.go
├── configs/
│   └── scenarios.yaml        # 示例场景配置
├── data/                     # 执行日志输出目录（运行时生成）
├── go.mod
└── README.md
```

---

## 五、核心数据模型说明

### Task（LLM 输出）

```json
{
  "task_id": "a9c9...",
  "trace_id": "a9c9...",
  "task_type": "irrigation",
  "target": "field-A",
  "params": {
    "duration_min": 30
  },
  "source": "llm"
}
```

含义：
- **task_id**：服务生成的唯一任务 ID（若未提供自动生成）
- **trace_id**：可透传链路 ID（默认与 task_id 相同）
- **task_type**：农业任务类型（语义层）
- **target**：抽象目标（地块 / 区域 / 设备组）
- **params**：任务参数
- **source**：任务来源（llm / rule / human）

> 任务与动作执行链路会在日志中携带 `trace_id`/`task_id`，便于排障与审计。

---

## 六、服务内部执行流程（重点）

### 1️⃣ API 接收 Task

- HTTP 接口：`POST /control/task`
- 只做 JSON 解析，不做决策

### 2️⃣ Policy（安全与约束）

- 校验参数是否合法
- 强制上限 / 下限
- 防止危险操作

> ⚠️ 即使 LLM 出错，这一层也能兜底

### 3️⃣ Planner（Task → Action）

- 根据 `TaskType` 查找注册表
- 将抽象任务拆解为 **固定动作序列**

示例：
```
irrigation
  ↓
open_valve → wait → close_valve
```

### 4️⃣ Executor（真正执行）

- 将 Action 转换为设备命令
- 调用真实设备 API（当前为打印示例）

---

## 七、如何新增农业场景（核心问题）

### ✅ 场景新增（推荐方式）

- 在 LLM / 规则系统中新增场景分析
- 输出已有 TaskType

> 不需要修改本项目代码

---

### ✅ 新增任务类型（仍然安全）

优先修改配置：

```
configs/scenarios.yaml
```

示例（actions 段）：
```yaml
actions:
  irrigation:
    - action_type: open_valve
      device_type: irrigation
    - action_type: wait
      device_type: system
    - action_type: close_valve
      device_type: irrigation
```

示例（scenarios 段，供上游参考）：
```yaml
scenarios:
  drought:          { task_type: irrigation,   params: { duration_min: 30 } }
  heat_wave:        { task_type: irrigation,   params: { duration_min: 20 } }
```

> 配置加载失败时，会自动退回内置默认表（registry.go）。

❗ 不需要改：
- executor
- service 主流程
- API

---

## 八、为什么这种设计可以长期扩展？

### 1️⃣ 扩展点全部集中在“上层”

- 新农业知识 → LLM / Rule
- 新任务 → Registry

### 2️⃣ 下层稳定

- Executor 只和设备打交道
- 不感知任何农业语义

### 3️⃣ 便于演进为：

- YAML / JSON 驱动的 Registry（已上线基础版）
- 多设备协议支持
- 任务调度 / 回放 / 审计

---

## 九、未来可扩展方向（不破坏现有结构）

- [已完成] Registry 支持从 `configs/scenarios.yaml` / JSON 加载，失败回退内置默认表
- [已完成] Task 全链路标识：自动生成 `task_id` / `trace_id`，入口透传，日志携带
- [已完成] 执行日志与回放：JSONL 持久化，支持按 task/trace 重放
- [已完成] 并发与调度：worker 池并发执行，支持 `schedule_at` 延时
- Executor 对接真实灌溉 / 施肥 API

**已完成项实现要点（摘要）**
- Registry：启动时从 `configs/scenarios.yaml` 读取 `actions` 映射，解析失败自动退回内置默认表。
- Task ID：API 入口自动生成 `task_id` / `trace_id`，全链路透传到日志与执行器。
- 执行日志：`logstore` 单文件单编码器 + 互斥锁串行写 JSONL，字段含 ts/task_id/trace_id/device/command/params/status/elapsed_ms；`cmd/replay` 可按 task/trace 回放。
- 并发与调度：队列 + worker 池；`schedule_at` 未来时间用定时器到点再执行；`wait` 动作非阻塞，定时器触发后续动作，避免占用 worker。

---

## 十、执行日志与回放（新增）

- 日志落盘：`data/execution.log`（JSONL），字段包含 ts/task_id/trace_id/device/command/params/status/elapsed_ms
- 启动日志：服务启动自动创建日志文件（不可写则仅打印不落盘）
- 并发安全：日志写入采用单文件单编码器加锁串行写，避免并发写冲突
- 回放工具：

```bash
go run ./cmd/replay -log data/execution.log -task <task_id>
# 可选：-trace <trace_id> 过滤，-limit 100 限制条数
```

> 回放当前为“重放命令”模式：读取历史命令并按同顺序调用 Executor（示例打印）。若后续接入真实设备，请确认回放环境的安全性和幂等性。

## 十一、并发与调度（新增）

- 并发：启动时通过参数 `-workers` 指定 worker 数，默认 4；内部队列自动限流，队列满返回 500。
- 调度：Task 可选字段 `schedule_at`（RFC3339 时间），到点后再执行规划/下发，时间早于当前则立即执行。
- 非阻塞等待：Action 中的 `wait` 不再占用 worker，服务使用定时器到点继续后续动作；长等待不影响其它任务并行。
- 入口行为：API 仍为 POST `/control/task`，成功表示“已入队/排期”；执行结果通过日志观测。

## 十二、启动与测试

- 启动服务（默认端口 8280）：
```bash
mkdir -p data
go run ./cmd/server -registry configs/scenarios.yaml -workers 4
```

- 发起示例任务（task_id/trace_id 可缺省）：
```bash
curl -X POST http://localhost:8280/control/task \
  -H "Content-Type: application/json" \
  -d '{"task_type":"irrigation","target":"field-A","params":{"duration_min":30},"source":"llm"}'
```

- 查看执行日志：`data/execution.log`（JSONL），或直接观察服务终端输出。

## 十三、可进一步改进

- 日志轮转与分片：按大小/日期切分，回放支持多文件输入。
- 定时器管理：集中管理 wait/schedule 的定时器，停服时优雅关闭或忽略回调。
- 并发/速率控制：为定时触发的后续动作增加全局并发/速率限制，防止瞬时洪峰。
- 幂等与重试：Executor 接入真实设备时设计幂等键、重试与退避策略，记录失败原因。
- 配置校验：对 registry 做 schema 校验与启动前预检，给出明确告警。
- 回放安全：回放提供 dry-run / 模拟模式，避免在生产设备上触发真实操作。
- 观测性：增加 action 序号、总步数等结构化字段，补充 metrics（队列长度、定时器数、执行耗时分布）。
