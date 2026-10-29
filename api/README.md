# Agent HTTP 契约

[agent.openapi.json](agent.openapi.json) 是 Go 版的实现基准，可导入支持 OpenAPI 3.0 的接口工具。只有 /healthz 已实现，其余路由用 x-implementation-status=planned 标记，x-delivery-stage 标明课程阶段。定义接口不等于开放接口。

## 已核对的源码

参考本地 Ragent c3b61577：AgentChatController、AgentConversationController、AgentMetaController；AgentChatServiceImpl、AgentStreamEventBridge、AgentSSEEventType；agent/dto 下的消息、工具进度、文本封口和审批载荷；framework 的 Result、Results、GlobalExceptionHandler、ChatQuestion、SseEmitterSender；前端 agentService.ts、api.ts。

保留路径、方法和 camelCase 参数。Agent 与固定 RAG 仍是不同入口，本文件只覆盖 Agent。引擎信息中的 framework 需要如实返回 Eino，不能复制 AgentScope 标识或虚报知识库、MCP 能力。

## 建流前与建流后

Ragent 的异常处理器保留 HTTP 200 + application/json + 非零业务 code。Go 版以此为第一版基准：请求客户端先检查 Content-Type；JSON 时读取 Result.code，不能因为 HTTP 200 就判断聊天成功。成功码为字符串 "0"，通用客户端错误 A000001，系统错误 B000001；细分业务错误随后续实现补充。

普通业务结果使用 code/message/data/requestId；允许可选字段和兼容附加字段。SSE 的每帧 payload 不套 Result。/healthz 保持现有响应，不是业务 Result。

认证暂参考 Ragent 前端 Authorization 直接发送登录 token 的约定；当前 Go 没有登录接口或 token 校验，不能拿任意 token 调用尚未实现的接口。可信 userId 由服务端认证后写入 context，禁止从工具参数或聊天查询参数指定。

## SSE 示例

正常回复（也可以是无工具的澄清）：

```text
event: meta
data: {"conversationId":"c1","taskId":"t1"}

event: message
data: {"type":"answer","delta":"请提供订单编号"}

event: finish
data: {"messageId":"m1","title":"订单查询","messageStatus":"NORMAL","durationMs":120}

event: done
data: [DONE]

```

失败收尾，不把已输出的部分消息伪装成正常回复：

```text
event: message
data: {"type":"error","delta":"本次回复已中断，请稍后重试"}

event: finish
data: {"messageId":"m1","title":"订单查询","messageStatus":"INTERRUPTED","durationMs":230}

event: done
data: [DONE]

```

用户取消（连接仍可用时）：

```text
event: cancel
data: {"messageStatus":"INTERRUPTED","durationMs":150}

event: done
data: [DONE]

```

等待审批（后续课程）：

```text
event: confirm
data: {"messageId":"m2","title":"等待确认","calls":[],"durationMs":170}

event: done
data: [DONE]

```

最后一个示例只说明帧形状；实际待确认事件应有真实待确认调用。confirm 与 finish 互斥；取消恰逢待审批时，Ragent 也可能走 confirm 收尾。断线时无法保证发出 cancel/done。done 的数据是原始 [DONE] 字符串，不是 JSON，客户端不能无条件 JSON.parse 每帧。

## 轨迹与展示

message 表示增量，type=answer/reasoning/error，正文在 delta。block 是工具进度或文本封口：工具进度包含 name、toolCallId、status、结果和服务端耗时；文本封口不重复发送正文，只补起止与耗时。工具 status 的 done 不等于整个 Agent 正常完成。

CLI tool_call/tool_result/answer 是内部步骤事件，HTTP 需要映射到本协议，不直接原样输出。当前没有 Token 增量或 reasoning 能力时不伪造对应过程；完整 answer 可以作为一帧 message 输出。

## 请求与取消约束

question 为非空白，参考 Ragent 上限 500；Java 与 Go 对补充平面 Unicode 的长度计数可能不同，精确兼容须在实现时统一，不能只依赖 OpenAPI maxLength。首次不传 conversationId 时创建会话；后续传原值。每次执行产生新 taskId；停止必须验证任务属主。

GET /agent/v1/chat 会执行任务，禁止缓存、预取、自动重试和自动重连；访问日志不能记录问题查询串。没有幂等找回、Last-Event-ID 重放或断线恢复。停止接口返回成功仅表示请求已处理，取消不能撤销已完成的外部动作。

## 实现时的 service 职责

Handler 解析 HTTP、调用 Service、输出 SSE。AgentChatService 取得可信用户，校验/创建会话、加载历史、管理同会话占用、生成 taskId、组织执行与结算；Agent 只负责 Eino 循环。会话和认证尚未落地，先按第 04–06 课补齐这些真实依赖，第 07 课接通 SSE；本课不增加占位 Service。

OpenAPI 只描述请求与响应，不能约束 SSE 帧的时间顺序；x-sse-events 是说明性扩展，流协议以本文和实际集成验证共同验收。不声称已与 Ragent 前端完全兼容。
