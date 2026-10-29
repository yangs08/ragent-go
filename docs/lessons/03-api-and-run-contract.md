# 第 03 课：第一版 Agent HTTP 与运行契约

更新：2026-10-29。状态：接口及 SSE 基准已固化到 api/agent.openapi.json，业务 HTTP 接口尚未实现；第一、二课运行代码不变。

## 1. 本课目标与决定

第一版采用 Ragent 的生命周期方向：一次请求启动 Agent，通过同一连接输出 SSE；连接断开时请求取消任务。先完成可靠的短任务交互，再扩展后台执行和事件重放。

保留 Conversation（会话）、Message（历史消息）、Run（一次执行）、Event（过程事件）四个对象。保留 Run 不等于必须引入后台 Worker。

## 2. 设计依据与修订

第一版以本地 Ragent 的 AgentChatController、AgentConversationController、AgentChatServiceImpl 和 AgentSSEEventType 为接口基准。先理解并复现其产品流程，再讨论有证据的改进；Go + Eino 替换内部实现，不自行重设计对外交互。

之前的 POST /api/v1/conversations/{id}/runs 草案撤销。它要求先建会话，并将执行资源作为前端入口，偏离了本课程参考项目的使用流程。Run 可以保留为内部执行模型，对外使用 Ragent 的 taskId；第一版不提供独立 Run 创建、查询或订阅接口。

Ragent 将 Agent 与固定 RAG 分开：Agent 使用 /agent/v1，固定 RAG 使用 /rag/v3；本课先实现 Agent，后续 RAG 课程再分析其接口，不提前统一成 mode 参数。

## 3. 第一版接口基准

以下路径、方法和字段来自 Ragent；按课程逐步实现，不代表当前已提供。

| 接口 | 入参及行为 | 实现阶段 |
|---|---|---|
| GET /agent/v1/chat | query: question 必填，conversationId 可选；同连接返回 SSE | 首次 HTTP 接入 |
| POST /agent/v1/stop | query: taskId；检查属主并请求取消 | 首次 HTTP 接入 |
| GET /agent/v1/conversations | 当前用户的会话列表 | 会话持久化 |
| GET /agent/v1/conversations/{conversationId}/messages | 当前用户有权访问的历史消息 | 会话持久化 |
| PUT /agent/v1/conversations/{conversationId}/title | JSON: {"title":"新标题"} | 会话持久化 |
| DELETE /agent/v1/conversations/{conversationId} | 删除有权访问的会话 | 会话持久化 |
| POST /agent/v1/conversations/batch-delete | JSON: {"ids":["会话ID"]} | 会话持久化 |
| POST /agent/v1/chat/confirm | JSON: conversationId、messageId、approved；返回新 SSE 流 | 人工审批课程 |

首次提问不传 conversationId，由服务端生成会话；继续聊天时携带原 conversationId。每次执行生成新的 taskId。服务端通过首个 meta 事件回传 conversationId、taskId。前端保存会话标识，点击停止时提交本轮 taskId。

身份来自认证上下文，不接受客户端指定 userId/tenantId。开始 SSE 前完成参数、认证、归属和并发检查。按 Ragent 的 GlobalExceptionHandler，业务异常使用 HTTP 200 + application/json + 非零 Result.code；客户端先判断响应类型。普通结果的 code/message/data/requestId、流内各事件字段和结束顺序见 [接口说明](../../api/README.md)。当前不声称前端可直接兼容。

沿用 GET 是为了对照参考项目，记录其实际取舍：问题进入 URL，访问日志应去除查询参数；GET 发起执行时不能按普通可重试读取处理。前端必须禁用自动重连及自动重试，避免重复执行。后续如确需改为 POST，将单独说明需求、兼容和迁移方案。

目前只有 /healthz 真正可调用。

## 4. 运行与取消

```text
认证/校验/会话占用 → 创建会话/内部执行记录 → meta → ADK 事件输出
                                                   ↓
                            completed / failed / cancelled
```

取消触发：请求 context 结束、响应写入失败、整轮超时、用户主动停止、服务退出。

- context 传给 ADK、模型和工具，执行器只在取消被观察到后结算取消状态。
- 主动取消与完成竞争时，终态通过条件更新只结算一次；晚到回调不能覆盖终态。
- 单次模型超时、整轮期限、每次写出期限分别管理，不让失联客户端无限阻塞写出。
- 首包之后执行长工具时发送有界频率心跳，发现断线依赖网络错误或超时，不承诺立即发现。
- 连接已断，无法保证送达 run_cancelled；状态和部分消息在独立且有期限的清理 context 下保存。
- 会话名额在执行实际结束后释放；取消信号不能被误当成下游已经停止。
- 只允许一个活动 Run；进程内互斥作为初期实现，跨节点时在并发治理阶段扩展。

取消不能撤销已完成的远程操作。第一版仍使用只读工具；写操作上线前必须另做审批、幂等与结果对账。

## 5. SSE 事件基准

沿用 Ragent AgentSSEEventType 的事件名称，不直接把第二课 CLI 的 JSONL 名称当成 HTTP 协议：

| 事件 | Ragent 定义的职责 |
|---|---|
| meta | 会话与任务元信息，payload 为 conversationId、taskId |
| message | answer / reasoning / error 增量消息 |
| block | 文本与工具轨迹块更新 |
| hint | 运行提示，如迭代预算提示 |
| confirm | 等待用户审批，与 finish 互斥 |
| finish | 回复完成 |
| done | 流结束 |
| cancel | 用户取消 |

第一阶段只实现已具备能力对应的事件，不伪造 reasoning 或审批能力。已核对 sender、bridge、payload、前端结果解包和调用路径，字段定义见 [OpenAPI](../../api/agent.openapi.json)，帧示例见 [接口说明](../../api/README.md)。正常 finish(NORMAL)→done；失败 message(error)→finish(INTERRUPTED)→done；取消 cancel→done；审批等待 confirm→done。finish 本身不表示执行成功。

内部执行只有一个终态，但不能据此假定外部只发送一个结束事件；finish 与 done 的职责不同。内部 tool_call/tool_result/answer 经 HTTP 适配层转换到 Ragent 协议。

SSE 断开后不自动重连，不自动重发聊天请求。用户重新发送会产生新 taskId 和潜在模型用量。同会话占用用于阻止并发，不等于请求幂等。首个 meta 丢失时不保证找回任务或安全重试。

慢客户端采用有界输出和写出超时；部分输出记录为中断。原始供应商错误和凭据不直接输出，工具参数与结果只暴露经过授权和脱敏的展示字段。

## 6. 数据与进程边界

第 4–6 课加入数据库、可信身份、会话/消息与运行状态。第一版不要求持久保存全部 token 或事件；保存最终/部分消息与运行终态即可。模型自然语言回答仍需依据工具结果评估。

进程重启后不会自动从中断处继续。持久化阶段要识别遗留 running 记录，标记为失败/中断并记录 worker_lost 等安全原因，不能假装工具已经取消或自动重放未知写入。

知识入库仍需要持久任务和 Worker；本次简化只针对交互式 Agent，不删除可靠入库设计。

## 7. 验收条件

| 场景 | 预期 |
|---|---|
| 正常任务 | 先收到 meta 中的 conversationId/taskId，再看到轨迹、答案和符合协议的结束事件 |
| 关闭连接 | 模型/工具观察取消，执行收尾并释放会话名额 |
| 长工具期间断线 | 在检测/超时后取消，不声称即时发现 |
| 用户主动停止 | 校验属主，取消不导致重复终态 |
| 超时 | 可区分期限耗尽与用户取消，保留部分结果 |
| 并发提交同一会话 | 后发请求 409，不启动第二次模型执行 |
| 模型/工具失败 | SSE 中输出脱敏错误并结算失败 |
| 重启 | 不自动恢复，遗留运行有明确处理规则 |

以上均待后续代码与集成测试验证。本轮文档调整不产生新的运行测试结果。

## 8. 后续扩展，不作为第一版前置条件

后台执行、创建/订阅分离、Idempotency-Key、Last-Event-ID 补发、持久事件日志、Checkpoint 崩溃恢复。到具体需求阶段再引入。

## 9. 复盘与下一步

思考：为什么请求 context 取消后，还需要独立的清理 context？为什么阻止同会话并发不等于幂等？为什么 heartbeats 不能保证瞬间检测断线？

本课接口文件与 SSE 示例已完成；下一课进入 PostgreSQL 工程集成，再接身份和会话数据。HTTP 服务化仍按第 07 课建设。HTTP 接入时用主动关闭连接的测试验证下游取消，不能只看浏览器停止显示。

## 10. 本次交付与复盘

新增 api/agent.openapi.json、api/README.md；明确流前 HTTP 200/JSON 业务错误、500 长度限制、Result、meta/message/block/hint/finish/cancel/confirm/done 字段及结束顺序，并补齐原草案漏掉的 /agent/v1/meta。所有未实现业务接口都标记 planned。

本次仅修改契约与文档，不修改运行代码，不重复执行运行测试。完成 JSON 解析、内部引用、路径参数与 operationId 基础检查；未宣称经过专用 OpenAPI 工具的完整规范校验或真实接口验证。

复盘：为什么 HTTP 200 不一定建流成功？为什么 finish 与 done 要分开？为什么请求 context 取消后，数据库收尾还需要独立且有期限的 context？为什么审批确认后的执行有新的 taskId？
