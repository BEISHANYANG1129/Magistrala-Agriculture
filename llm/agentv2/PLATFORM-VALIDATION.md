# 本地 Magistrala 联调记录

日期：2026-10-08。仅描述本地实际部署，不将网上示例直接视为当前运行接口。

## Rule 拒绝的原因和处理

创建规则返回 `rule limit for this edition reached`。官方 Consumers 文档明确 Community Edition 每套部署最多一条规则，Enterprise 无规则数量限制：
https://www.absmach.eu/docs/magistrala/dev-guide/services/consumers/

只读 API 核实：独立工作区 `LLM Go Isolation 20261008` 为 0 条；原工作区 `test` 有已启用的 `test5` 一条。因此新建工作区并不获得独立规则额度。

原 test5 输入为 test3，Lua 逻辑仅在 `message.payload[1].v == 20` 时返回 payload，输出是 `save_senml`。原配置已保存到 `runtime/test5-before.json`。

用户明确授权修改 test5 和 test3 后，创建 `llm-go-test3-sensor-20261008`，仅增加 Publish 连接；原 test2/test4 的 Publish/Subscribe 连接保持不变。test5 前置分支仅匹配该专用设备的 `message.publisher`，其他发布者继续执行原脚本。规则其他字段经 GET 回读比对保持不变。备份与修改结果分别为 `runtime/test5-before.json`、`runtime/test5-after.json`。

## 真实 API

| 操作 | 地址/认证 | 已观察结果 |
| --- | --- | --- |
| 管理 API | POST http://localhost:8080/graphql，用户 Bearer | 登录、工作区/设备/通道查询和创建成功 |
| 发布 | POST http://localhost/http/m/{workspace}/c/{channel} | Basic base64(device_id:shared_key)，HTTP 200 `{"status":"ok"}` |
| 读取 | GET http://localhost:9011/{workspace}/channels/{channel}/messages | 用户 Bearer，HTTP 200 |
| 规则 | GET/PATCH http://localhost:9008/{workspace}/rules/{rule} | 用户 Bearer，PATCH 后 GET 校验成功 |

网上部分消息示例使用 `Authorization: Client <secret>`；本地实测该方式为 401，Basic 成功。因此协议适配要以部署版本及实际响应为准。

上传样例（合成数据）：

```json
[{"n":"soil_moisture","u":"%","v":12,"t":1791431829.106}]
```

对应 Reader 中的消息：

```json
{
  "channel": "3e692d54-5ccc-4a83-9cfd-ac0080263327",
  "publisher": "e30a6c90-11bd-4634-9ffd-b650b972198d",
  "protocol": "http",
  "name": "soil_moisture",
  "unit": "%",
  "time": 1791431829106000000,
  "value": 12
}
```

发布时 t 是秒，当前 Reader 的 time 是纳秒。Go Reader 必须选择 `-time-unit nanoseconds`。前一次脚本用秒比较返回时间，产生了假失败；修正单位后重新发送并精确匹配 publisher/name/time，验证通过。完整已脱敏请求响应在 `runtime/test3-platform-smoke.json`。

Reader 顶层还包括 offset、limit、order、dir、publishers、format、total、messages。Go 接入必须保留 channel/publisher 身份，不能只给 LLM 传数值。分页需校验完整性；值为 0 不能误认为缺失。

## 结论及边界

没有匹配存储规则时，独立通道发布 HTTP 200，但 Reader 为 total 0；增加被授权的专用设备规则分支后，test3 的合成数据能读回。由此验证：消息接收成功不等于持久化成功。

真实 Go Reader 已将此数据转换为区域证据并通过流程验证，分析器为模拟分析器。实际 Ollama 推理仍须等显卡空闲。平台和旧服务未重启，原仓库已有未提交修改未被本模块覆盖。

本记录不证明原 test2/test4 的端到端回归已完成；这里只进行了配置保留比对，并仅使用新建测试设备发布合成数据。

## UI 消息页和 GPU 的剩余阻塞

消息页重试后仍显示 `fetch failed`。UI 容器配置 `MG_BACKEND_URL=http://ui-backend:9097`，但 `docker ps` 没有正在运行的 ui-backend 容器；UI 容器内 DNS 查询返回 `ENOTFOUND`。与此同时，UI 容器访问 Reader、Alarms、Reports 的健康检查均为 200。这是已确认的 UI 后端依赖缺失，修复后仍需重测消息页，不能声称已完成 UI 消息展示闭环。

没有启动或重启该共享服务。现场截图在 `runtime/platform-messages-error.png`，新增设备和原连接保留情况在 `runtime/test3-connections.png`。

GPU 检查中利用率曾为 80%、61%、34%，短暂降到 3% 后连续三次又为 31%；另有两个运行中的任务容器均申请了 NVIDIA GPU 0。Ollama `/api/ps` 无已加载模型。按用户要求未发出任何真实推理请求；不会以短暂低利用率认定其他任务已结束。
