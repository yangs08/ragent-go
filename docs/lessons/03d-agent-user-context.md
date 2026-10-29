# 第 03 课补充实践：收敛 Agent 与用户上下文

## 为什么再次调整

上一轮把职责独立落实成了过多类型和包。尤其是 Principal、按用户 Resolve 工具目录，以及订单示例的 Service/Repository/devdata 分层，并非当前需求必须。本轮依据 Ragent 源码收敛，保留实质边界。

## Ragent 怎么做

- UserContextInterceptor：从 SaToken 取得登录用户，写入 UserContext，请求结束清理。
- ReActAgentProvider：组合并缓存 Agent，提示词或工具目录变化时重建。
- AgentChatServiceImpl：取得 userId，构建 RuntimeContext，带 userId、sessionId 和 MCP 元数据。
- McpToolProxy：从本轮运行上下文取得元数据，传给远程工具；不是把用户塞进模型参数。

Java 使用 TransmittableThreadLocal；Go 使用 context.Context 随调用传递。这里参考职责与身份边界，不复制线程局部机制或框架实例缓存。

## 当前实现

```go
ctx = common.WithUser(ctx, common.User{ID: "alice"})
assistant, err := agent.New(client, tools, definition, cfg.Agent)
// 检查 err 后执行
err = assistant.Run(ctx, question, emit)
```

工具在装配时创建并注册一次，调用时用 common.RequireUser(ctx) 获取本次用户。模型 Schema 只有 order_id，不能通过参数选择用户。没有用户的数据查询直接拒绝。同一个工具分别在 alice、bob 的上下文中运行，不改变工具对象字段。

User 目前只包含 ID；没有租户需求时不强制增加 TenantID。原 C300 示例改为另一个用户的订单，不再宣称已有多租户隔离。最终项目的租户能力若需要，将在实际授权与数据模型课程明确实现。

订单能力统一放在 internal/example/order.go：它是可运行的工具示例，不是生产订单系统。去掉仅为示例建立的 order、devdata、identity、tooling 目录和开发装配工厂；cmd/agentcheck 的装配代码直接可读。未来真实知识库与 MCP 能力出现后再建立对应模块。

## 需要保留什么

统一配置、通用 Eino 调用、每轮共享预算、类型化事件、调用关联、取消/期限、严格参数校验和安全错误处理继续保留。Agent 不写死订单，也不编码 JSONL。每轮新建 ADK 与预算包装，注册工具可复用，不声明框架原生 Agent 单例的并发与状态语义。

单次问题不再包一层 Request；工具列表直接传给构造器，不再要求一个 ToolCatalog 接口。Definition 是服务端提示词与 Agent 元数据，不属于客户端参数。

## 验证与边界

使用 make test、make vet、make smoke、make verify-modules。迁移已有关键测试，确认共享工具切换用户时不串身份，无用户不能读取订单，越权返回 not_found；命令行为仍保持不变。自动化只用本地模型协议服务。

当前没有 HTTP 认证；CLI 手动注入用户只便于开发，不代表已验证登录。上下文持有可信身份不意味着工具结果一定可信，也不保证模型不会编造。SSE 与会话持久化后续建设。

## 复盘

为什么配置可以共享，当前用户必须在请求上下文中？为什么 Go 不能用进程全局变量模拟 UserContext？为什么工具定义不需要按用户重建？为什么没有必要给一个订单示例建立完整业务模块？

## 目录收敛

HTTP 文件由 internal/transport/http 移到 internal/handler，用户辅助函数由 internal/usercontext 移到 internal/common/user.go；代码调用使用 common.WithUser / common.RequireUser。只调整组织，不改变身份隔离、工具授权或 HTTP 行为。service 随聊天业务实现再创建，本轮不增加占位接口与空服务。
