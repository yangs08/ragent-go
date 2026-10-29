# 当前架构与设计边界

更新：2026-10-29。本文件描述当前代码，取代前一版按用户生成工具目录和独立订单模块的设计。

## 参考 Ragent 的实际分工

Ragent 使用 ReActAgentProvider 组合并缓存 Agent，AgentToolCatalog 集中注册工具；用户身份由登录拦截器放入 UserContext，AgentChatService 再将 userId/sessionId 写入本轮 RuntimeContext，MCP 调用通过元数据传递用户。身份不保存在共享 Agent 或工具对象字段中。

Go 保留相同原则：模型与工具集中装配，本轮身份放入 context.Context；不照搬 Java 线程上下文，也不在尚无需求时引入租户与权限领域模型。

## 当前代码

```text
cmd/agentcheck/main.go        配置、模型与示例工具装配，注入开发用户，JSONL 输出
internal/agent/agent.go       Agent 定义、Eino 执行、预算与事件
internal/common/user.go  简单 User 与 context 存取函数
internal/example/order.go    本地订单示例、工具定义、数据与查询
internal/config/             Server/Model/Agent 统一配置
internal/model/              模型适配与文本流回调
internal/handler/     Gin 健康检查和中间件
internal/bootstrap/server.go HTTP 生命周期
```

删除多余的 Principal、Request 包装、ToolCatalog 接口、独立 provider.go、开发 Agent bootstrap 工厂，以及单独的 order/devdata 包。当前一个订单示例不需要生产业务模块体系。真正的知识库工具与 MCP 会按对应需求建设。

## 调用与身份

```text
启动：配置 → 模型 + 注册工具列表 → agent.New
执行：ctx（可信用户） + question → Agent.Run → Eino → 工具
                                                       ↓
                                      common.RequireUser(ctx)
                                                       ↓
                                             按用户查询与授权
```

工具只创建一次，不捕获当前用户。每轮 context 自然隔离身份；模型工具参数只有 order_id。当前 CLI 显式注入 alice，HTTP 登录已在第五课实现，不能把 WithUser 当作认证。

Agent 包不强制所有任务必须登录；访问用户数据的工具在调用时要求可信用户。未来 HTTP 登录检查在中间件/应用服务执行，不能允许请求直接指定可信用户。

共享配置、模型与无用户字段的工具；每轮创建 Eino ADK、工具预算包装和调用关联表。这是当前安全隔离策略，不声称 Eino 与 Ragent 的 AgentScope 具有相同的共享状态行为。

## 保留的必要边界

配置只在入口加载，组件接收配置值；执行器不依赖订单，也不编码终端输出；工具校验模型参数和访问归属，内部错误脱敏；输出回调同步、可失败，取消与期限传到底层。工具尝试预算在本轮全部工具之间共享。

通用 Agent 允许澄清，不能用“必须调用工具”拦截全部回答；订单提示词要求具体状态依据查询结果，但仍需未来业务核验与评估。工具结果默认不展示，示例 Profile 显式开启已脱敏结果。

## 当前限制与下一阶段

只有 /healthz HTTP 路由。聊天、会话历史、业务会话表、主动停止、SSE 协议和跨实例治理尚未实现。第一版接口按 Ragent：GET /agent/v1/chat、POST /agent/v1/stop，断线取消，不自动重连。

优先实现真实调用链，再建立有实际用途的抽象；不预建 Service 容器、动态工具目录或租户系统。测试聚焦身份隔离、越权、取消与预算，不为目录与简单传参堆测试。

过程说明见 [本课：Agent 与用户上下文收敛](lessons/03d-agent-user-context.md)。

## 目录约定

HTTP 入口统一使用 handler，不保留 transport/http 两级结构；用户上下文是 common/user.go 中的简单辅助函数，不单列业务模块。common 仅放实际共享的小型基础类型/函数，不集中业务逻辑或全局可变状态。

service 用于后续聊天与会话业务流程；当前尚无这条业务链，暂不建立空目录。Handler 负责协议与输入，Service 协调业务，Agent 负责 Eino 执行；示例工具只依赖 common 中的标准 context 辅助函数，不依赖 Gin。

## 第 04 课数据库接入

新增 internal/database 的 GORM 连接初始化，配置仍在统一 config.Database 中；migrations 保存独立版本 SQL，cmd/migrate 显式执行，不由 HTTP 启动自动改表。数据库已验证，登录 AuthService 已在第五课实现，会话和聊天 Service 尚未实现。详见 [第四课](lessons/04-postgresql-and-migrations.md)。

## 第 05 课认证接入

internal/service/auth.go 直接使用 GORM，Handler 负责登录协议和认证中间件；common.User 增加 Username。启用数据库并迁移后提供 /auth/login、/auth/logout、/user/me，token 在 PostgreSQL 中只存摘要。现有示例工具的开发身份与真实账号 ID 分开，HTTP 聊天尚未接入。

service 现在有真实认证流程，后续会话业务继续沿 Handler → Service → GORM；不新增空的仓储接口。详见 [第五课](lessons/05-login-and-user-context.md)。
