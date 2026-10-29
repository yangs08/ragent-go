# 第 03 课补充实践：沿真实职责重整应用架构

> 历史阶段记录：后续已简化 Agent、身份与工具组织，当前代码以 [03d](03d-agent-user-context.md) 和 [架构基线](../ARCHITECTURE.md) 为准。


## 目标

把前两课已验证的能力纳入可演进的应用结构。功能尚未完整，但 Agent、工具、业务和适配器职责从现在开始明确。完整审查表与依赖图见 [架构基线](../ARCHITECTURE.md)。

## 先读 Ragent

参考本地仓库的 AgentEngineConfiguration、ReActAgentProvider、AgentToolCatalog、KnowledgeSearchTool、McpToolProxy。重点观察模型装配、Agent 组合、工具集合、工具适配、业务委托和执行期上下文的分工。

Ragent 会基于提示词和工具目录指纹缓存 Agent，Go 当前每轮创建 ADK 及身份绑定工具，不照搬共享运行实例。我们复用已验证的 Eino v0.9.22 调用方式，没有新增框架依赖。

## 当前运行方式

make agentcheck 仍使用开发订单仓储，输出 tool_call、tool_result、answer 三类 JSONL；make modelcheck 消费文本流；make run 仍只提供健康检查。全部演示数据由 bootstrap.NewDevelopmentAgent 显式接入，不进入通用执行器。

## 顺着一条调用学习

1. cmd/agentcheck/main.go：统一加载配置，调用开发装配函数，每次 Run 传 Principal 和 Question，消费事件。
2. internal/bootstrap/agent.go：组装模型、订单服务、工具目录、Agent Definition、Executor。
3. internal/agent/provider.go：定义可信 Agent Profile 和工具目录边界，组合 Eino 模型与工具。
4. internal/agent/executor.go：每轮解析工具目录、校验名称唯一、共享预算、消费执行事件，不知道订单业务。
5. internal/tooling/catalog.go：按当前主体创建本轮工具，不在共享目录中保存当前用户。
6. internal/tooling/order.go：通过 InferTool 定义名称、Schema 与调用函数，严格解析模型 JSON，再调用业务服务。
7. internal/order/service.go：校验 ID、按租户读取、核对资源归属、返回安全业务结果。
8. internal/devdata/orders.go：本地开发仓储；未来真实业务适配应遵循订单 Repository 或独立业务系统 API 的契约。

单模型、多工具的 ReAct 循环由 Eino 执行。Catalog 的任务是注册可信能力，不是自己根据自然语言路由工具。需要添加新工具时，在工具适配与 Catalog 注册；无需修改 Executor 的工具名或结果类型。

## 事件与回答行为变化

Event.Result 使用通用 JSON 载荷，执行器不解析 OrderResult。默认不向事件消费者展示工具内容；订单开发 Definition 因结果已脱敏而显式开启。原始模型调用参数仍不展示。

不再使用 ErrNoToolEvidence 拒绝全部无工具回答：澄清问题可以直接回答。具体订单状态必须依据查询的规则保留在订单 Profile 的提示词里；提示词不能证明答案忠实性。后续需要业务场景的结果核验与评估，不能把通用循环误当成事实校验器。

模型文本输出也改为回调，客户端构造与整轮期限由调用方负责。模型单次 HTTP 超时仍由客户端配置控制。回答长度通过 RAGENT_MODEL_MAX_COMPLETION_TOKENS 配置，默认 256；手动构造 ModelConfig 时零值表示交给供应商默认值，环境加载要求显式字段为正整数。

## 验证与限制

执行 make test、make vet、make smoke、make verify-modules。关键回归验证授权、JSON 参数、取消、预算、调用关联和进程输出；另验证同一 Executor 按 alice/bob/alice 先后执行不会复用上轮身份，非订单工具可正常运行，默认隐藏工具结果，无工具澄清被允许。

所有自动化使用本地协议服务，不代表真实供应商工具行为已完成联调。当前没有新建聊天 HTTP 路由、认证或数据库。下一课围绕真实 ChatService 与会话数据推进，不用空结构体或返回固定成功的接口代替实现。

## 复盘

解释：谁决定注册哪些工具，谁选择调用哪个工具？为什么权限不能只放在 Tool Schema？为什么 Catalog 共享而工具按轮创建？为什么数据库/业务错误不直接返回模型？为什么取消不能撤销已完成的外部操作？

练习：从 agentcheck 入口画出查询 A100 的依赖关系；分别标注共享对象与每轮对象，找出如果接入第二个只读工具需要修改的位置。
