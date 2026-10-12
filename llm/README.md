# LLM 模块 (Magistrala LLM)

> 一个 **只做“区域级分析与决策”，不直接控制设备** 的上游推理模块。  
> 核心目标：让 LLM 只输出“要对哪个分区做什么”，安全下发交给控制服务。

---

## 1. 背景与目标
- 从 Magistrala 拉取传感/多模态消息，补充分区信息，送入大模型。
- LLM 产出 **区域级指令 JSON**（不含具体设备 ID）。
- 将区域指令转为控制服务可执行的任务，通过 Control Service 下发，隔离设备细节。
- 可选 RAG：基于本地 Ollama Embedding 对农业知识库检索，给 LLM 追加参考上下文。（当前为简易实现，后续可扩展向量缓存/更优近邻搜索）

---

## 2. 关键原则
- LLM 只看分区与传感信息，不直连设备、不生成设备 ID。
- 消息先“分区补全 + 字段瘦身”再送入模型，减少噪声与带宽。
- 多模态仅在遇到图片/视频 URL 时下载转 base64，其余文本直接透传。
- 提示词外置（MD/TXT），按段选择（如 `region_commands`）。
- RAG 可开关，未启用时流程不变；启用后在推理前拼接检索段落（当前实现简单，需后续迭代）。

---

## 3. 目录结构
```
llm/
├── config/
│   ├── config.json              # 运行配置
│   ├── knowledge/
│   │   └── agri_knowledge.txt   # 知识库文本（空行分段）
│   └── prompts/
│       └── llm_prompts.md       # 多段提示词（如 region_commands）
├── core/                        # 编排/下发
│   ├── magistrala_reader.go     # 拉取/分区补全/瘦身
│   ├── orchestrator.go          # 推理编排
│   ├── task_adapter.go          # 转控制服务任务
├── llm/                         # LLM 客户端与推理辅助
│   ├── client.go
│   ├── ollama_client.go
│   ├── finetune_client.go
│   ├── infer.go                 # 调模型 + 清洗输出
│   ├── multimodal.go            # 构建多模态输入（含下载）
│   ├── rag.go                   # RAG（Ollama embedding + 余弦 TopK，简易实现）
│   └── utils.go
└── llm_api/
    ├── handler.go               # HTTP 接口
    └── main.go
```

---

## 4. 整体架构概览
```
              Magistrala
                    │ 拉取消息
                    ▼
      分区补全 + 字段精简 (core.ToLLMMessages)
                    │
                    ▼
        RAG 检索 (Ollama Embedding, 可选)
                    │
                    ▼
        Prompt 选择 (MD 段 region_commands)
                    │
                    ▼
        多模态拼装 (llm/multimodal.go)
                    │
                    ▼
        LLM 推理 (InferMultimodal)
                    │
                    ▼
      区域级指令 JSON (AnalyzeRegionCommands)
                    │
                    ▼
          Control Service /control/task
                    │
                    ▼
            Real Devices / API
```

---

## 5. 数据流（端到端）
1) 拉取消息：`core.FetchChannelMessages` 用 magistrala 配置获取最新消息。  
2) 分区补全 + 瘦身：`core.ToLLMMessages` 补全 `partition_id/name`，只保留 `partition_id/partition_name`、`name/subtopic`、`value`、`unit`、`string_value`（可含媒体 URL）、`time`，去掉 `channel/protocol/publisher/clientId`。  
3) 加载提示词：handler 读取 `config/prompts/llm_prompts.md`，按段名（如 `region_commands`）截取。  
4) （可选）RAG：`rag.Retrieve` 用精简消息构造查询，调用本地 Ollama Embedding 对知识库分段向量化匹配，取 TopK 段落；将这些段落拼到 prompt 作为“参考资料”。（当前为简易实现，建议后续加入持久化向量缓存/更优近邻搜索）  
5) 构建 LLM 输入：`BuildOllamaMessagesWithPrompt` 先放 prompt(+参考资料)，再逐条拼接传感字段；仅在 `string_value` 为图片/视频 URL 时下载转 base64 生成 `image_url/video_url`，其他文本不重复拼接。  
6) 推理：`AnalyzeRegionCommandsWithPrompt` 调 `LLMClient.InferMultimodal`，清洗输出并提取 JSON。  
7) 解析与下发：解析区域指令 → 转任务 (`task_type/target/params`) → `ControlAdapter.PostTasks` 发送到 Control Service `/control/task`。  

---

## 6. 配置要点（config/config.json）
- magistrala: `baseUrl`、`userToken`、`messagePort`
- `mapping.path`: 分区/设备映射表
- `controlService.baseUrl`: 控制服务地址
- llm: `endpoint`、`model`
- `prompt.path`: 提示词文件（如 `config/prompts/llm_prompts.md`）
- rag（可选，当前简易版）:
  - `enabled`: 是否开启
  - `storePath`: 知识库文本路径（空行分段）
  - `topK`: 返回段数（默认 3）
  - `ollamaEndpoint`: Ollama 服务地址（默认 `http://localhost:11434`）
  - `ollamaModel`: Embedding 模型名（默认 `nomic-embed-text`）

---

## 7. 提示词（MD 示例）
```
## region_commands
你是智慧农业专家...（仅输出区域级指令 JSON，action 仅 open/close，缺分区跳过）
```
handler 通过 `extractPromptSection(md, "region_commands")` 选取该段传入推理。

---

## 8. 多模态处理规则
- 仅当 `string_value` 是图片/视频 URL：
  - 下载并转 base64，生成 `image_url` 或 `video_url`。
- 其他文本不重复拼接。
- 传感字段始终包含：名称、数值、单位、时间；分区信息默认追加。

---

## 9. 启动与调用
```bash
cd llm
go run ./llm_api/main.go

curl -X POST http://localhost:9000/llm/plan-and-send \
  -H "Content-Type: application/json" \
  -d '{"limit":10,"domainId":"dom1","channelId":"ch1"}'
```

---

## 10. 测试
- 单测示例（控制任务下发）：`go test ./core -run TestPostTask`（可用 httptest 模拟控制服务）。
- 端到端：确保 Control Service 可用，调用 `/llm/plan-and-send`，观察控制服务收到 `/control/task`。

---

## 11. 扩展与注意
- 多场景可在 MD 中增段并在 handler 选择对应段名。
- 若需时间窗口/趋势分析，可在瘦身前按时间筛选或聚合；`time` 字段已随消息传给 LLM，可直接用于近时段判断。
- RAG 语料更新后可重启以重新 embedding；语料大时建议做向量缓存持久化，并可替换为更高效的向量近邻库。
- 保持 LLM 仅产出区域级目标，设备映射与安全控制交由下游控制服务处理。