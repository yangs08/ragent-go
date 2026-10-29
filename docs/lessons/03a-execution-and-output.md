# 第 03 课补充实践：拆开 Agent 执行与输出协议

## 本课目标

执行器提供可复用的执行能力，由调用方决定将事件输出到终端、HTTP 或测试收集器。本次只完成 ARCHITECTURE.md 的第一项整改，不代表完整企业架构已经完成。

## 为什么要改

此前 OrderAgent.Run 接收 io.Writer 并在内部编码 JSONL。这样 HTTP 接入很可能需要解析 JSONL，再重新编码 SSE，或者在执行器中增加 HTTP 分支。输出格式因此影响业务执行。

现在调用关系为：

```text
agentcheck → OrderAgent.Run(ctx, question, EventSink)
                         ↓
                 同步发出类型化 Event
                         ↓
            CLI 回调 → json.Encoder → stdout
```

执行器仍使用标准 JSON 解析工具结果，这是工具数据解析；移出的只是面向调用者的 JSONL 编码。Event 中暂时保留 JSON 标签以保持 CLI 输出字段，本次尚未拆订单专属事件与通用事件。

## 阅读代码

1. internal/agent/order_agent.go：EventSink 定义、Run 的事件发出位置及错误返回。
2. cmd/agentcheck/main.go：在入口创建 json.Encoder，通过回调消费事件。
3. internal/agent/order_agent_test.go：实际 ADK 与本地模型协议服务测试，不调用付费模型。

EventSink 是 func(Event) error；同步调用，事件按执行器消费顺序到达。返回错误时 Run 返回该错误，defer cancel 取消本轮 context；Run 返回后不再调用回调。回调应及时返回，Go 的 context 无法强制终止一个阻塞的回调。未来 SSE 写入必须设置写期限，不能只依赖 context。

不要把此回调称为事件总线：它没有后台订阅、持久化或重放。正常执行仍输出 tool_call、tool_result、answer，这属于现有 CLI 的事件表达；未来 HTTP 层需要按 Ragent 的 meta/message/block 等协议转换。

## 如何验证

```sh
make test
make vet
make smoke
```

新增验证：消费回调报错后不会继续启动下一次模型请求；多个并发 Run 分别具有完整有序事件和独立的一次工具预算。已有的越权、模型错误脱敏、工具预算、期限与取消测试继续保留。race 检查用于发现共享状态访问问题，不等于证明所有外部组件天然支持并发。

make smoke 使用本地协议服务验证 CLI 输出，make agentcheck 会调用配置中的真实模型，可能产生用量。

## 本次仍存在的边界问题

OrderAgent 仍是订单示例，工具名、结果结构和证据策略尚未泛化；Principal 仍在构造时绑定，只能在当前固定身份示例下使用。没有新增认证、ChatService、数据库或 HTTP 聊天接口。后续先分离订单示例职责和按请求身份，再接应用服务，不能把当前实例直接作为跨用户的全局服务。

## 练习与理解检查

尝试传入一个只收集 Event 的回调，不进行 JSON 编码。解释：为什么执行器不需要知道 stdout 或 SSE？为什么回调失败要停止本轮？为什么同步回调仍然需要输出超时？为什么这一步还没有解决多用户授权对象的生命周期？
