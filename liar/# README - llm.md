# 智慧农业 LLM 代码结构与函数功能说明

本项目用于智慧农业场景下的大模型（LLM）推理与分析服务，基于 Ollama 本地大模型（支持微调模型），实现消息数据的智能分析与建议。  
本说明将以**代码文件为单位**，从宏观到微观详细描述每个文件及其主要函数的功能。

---

## 目录结构

```
llm/
├── client.go
├── finetune_client.go
├── infer.go
├── loader.go
├── ollama_client.go
├── prompt.go
├── utils.go
llm_api/
├── handler.go
├── main.go
llm_test/
├── main.go
```

---

## llm 目录

### 1. client.go

**作用：**  
定义大模型统一接口，便于主流程与不同模型后端解耦。

**主要内容：**
- `type LLMClient interface`  
  - 统一大模型推理接口，要求实现 `Infer(prompt string) (string, error)` 方法。

---

### 2. ollama_client.go

**作用：**  
实现 Ollama 本地模型的推理请求，负责与 Ollama API 交互。

**主要函数：**
- `type OllamaClient struct`  
  - 保存 Ollama API 地址和模型名。
- `func (c *OllamaClient) Infer(prompt string) (string, error)`  
  - 发送推理请求到 Ollama，处理流式 JSON 响应，拼接完整推理结果。

---

### 3. finetune_client.go

**作用：**  
实现自定义微调模型的推理请求（如 HTTP API），便于扩展除 Ollama 外的模型后端。

**主要函数：**
- `type FinetuneClient struct`  
  - 保存微调模型 API 地址。
- `func (c *FinetuneClient) Infer(prompt string) (string, error)`  
  - 发送推理请求到微调模型服务，返回推理结果。

---

### 4. loader.go

**作用：**  
客户端初始化工厂方法，便于灵活切换和统一管理模型客户端。

**主要函数：**
- `func NewOllamaClient(endpoint, model string) *OllamaClient`  
  - 创建 Ollama 客户端实例。
- `func NewFinetuneClient(endpoint string) *FinetuneClient`  
  - 创建微调模型客户端实例。

---

### 5. infer.go

**作用：**  
推理主流程，统一完成 prompt 构建、模型推理、清洗和结构化输出。

**主要函数：**
- `func AnalyzeMessages(client LLMClient, messages []map[string]interface{}) (AnalysisResult, error)`  
  - 1. 构建 prompt（调用 BuildPrompt）
  - 2. 调用模型推理（client.Infer）
  - 3. 剔除 `<think>...</think>` 标签内容（RemoveThinkSection）
  - 4. 去除首尾空白（TrimSpaceAll）
  - 5. 将模型输出的 JSON 字符串解析为结构体（JSONToStruct）
  - 6. 返回结构化的分析结果

---

### 6. prompt.go

**作用：**  
Prompt 模板管理与构建，支持多种业务场景。

**主要函数：**
- `func BuildPrompt(messages []map[string]interface{}) string`  
  - 根据传感器数据等输入，构建适合大模型推理的 prompt 文本，强制要求输出标准 JSON。

---

### 7. utils.go

**作用：**  
工具函数集合，提供文本清洗、结构体转换等通用功能。

**主要函数：**
- `func IsImageURL(url string) bool`  
  - 判断字符串是否为图片 URL。
- `func RemoveThinkSection(s string) string`  
  - 剔除字符串中所有 `<think> ... </think>` 部分，避免影响 JSON 解析。
- `func TrimSpaceAll(s string) string`  
  - 去除字符串前后的所有空白字符。
- `type AnalysisResult struct`  
  - 用于承接大模型输出的结构化内容（summary、diagnosis、risks、suggestions、raw_analysis）。
- `func JSONToStruct(s string) (AnalysisResult, error)`  
  - 将 JSON 字符串解析为 AnalysisResult 结构体，便于类型安全地访问各字段。

---

## llm_api 目录

### 1. main.go

**作用：**  
API 服务入口文件，负责初始化模型客户端、注册 HTTP 路由并启动服务。

**主要流程：**
- 创建 OllamaClient 实例。
- 注册分析请求处理路由（调用 handler.go 的 AnalyzeHandler）。
- 启动 HTTP 服务监听端口。

---

### 2. handler.go

**作用：**  
路由处理与业务逻辑分发，负责接收前端/第三方系统的分析请求，调用 LLM 推理主流程，并返回结构化结果。

**主要函数：**
- `func AnalyzeHandler(client *llm.OllamaClient) http.HandlerFunc`  
  - 1. 解析请求体中的 JSON，获取消息列表（messages）。
  - 2. 调用 `llm.AnalyzeMessages`，完成 prompt 构建、模型推理、清洗和结构化。
  - 3. 返回结构化 JSON 结果或错误信息。

---

## llm_test 目录

### 1. main.go

**作用：**  
本地推理测试入口，模拟传感器数据，输出结构化分析结果，便于开发调试。

**主要流程：**
- 初始化 Ollama 客户端。
- 构造测试消息数据。
- 调用 `llm.AnalyzeMessages` 获取结构化分析结果。
- 遍历输出各字段，尤其是数组字段逐项输出，便于阅读。

---

## 总结

- 所有推理、清洗、结构化处理均在 `llm` 目录下完成，`llm_api` 只负责 HTTP 服务和请求分发。
- 结构体方式保证类型安全，便于前端和业务代码直接使用。
- 支持多模型后端（如 Ollama、微调模型等），只需实现统一接口即可切换。
- 可根据业务需求扩展更多 prompt 模板和推理流程。