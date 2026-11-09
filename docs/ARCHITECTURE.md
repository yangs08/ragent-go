# 当前架构与设计边界

本文件描述当前实现，是目录、装配和资源边界的统一说明。lessons 保留核心课程的学习与验证记录，其中旧代码片段不作为当前目录规范。

## 目录与职责

- cmd：服务、迁移、账号创建和联调入口。
- configs：按业务拆分 YAML；密钥和部署覆盖使用 .env。
- docs/api：OpenAPI 接口契约，不存放 Controller。
- internal/router.go：给已有 Gin 引擎注册路由。
- internal/middleware：认证、登录限流、请求日志。
- internal/controller：HTTP 参数解析、调用 Service、输出响应。
- internal/service：认证业务与 Eino 模型客户端调用。
- internal/model：数据库实体；Base[T] 提供 Model(ctx) 原生 GORM 查询入口。
- internal/agent：Eino Agent 编排、工具预算、取消与事件。
- internal/example：明确标记为学习示例的订单工具和本地数据。
- internal/common：统一 HTTP 响应、共享函数和安全诊断。
- internal/define：配置、接口结构、Result、BusinessError。
- internal/common/functions.go：用户 context、token 和 FailureAttrs 等共享函数。
- internal/database：PostgreSQL 连接初始化。
- internal/bootstrap：配置装载、Boot、全局 DB/Config/Logger、HTTP 生命周期与优雅关闭。
- migrations：独立 SQL 迁移，不在 HTTP 启动时自动改表。

## HTTP 调用链

Router → Middleware → Controller → Service → Model / Agent。

cmd/server 创建 Gin、装配 Service、安装公共中间件；RegisterRoutes 绑定路径及路由级中间件，Controller 不注册路由。bootstrap 不导入业务包。业务 Service 不依赖 Gin。统一响应函数放 common/response.go，供 Controller、中间件和路由恢复逻辑共享，避免中间件反向依赖 Controller。

认证提供 /auth/login、/auth/logout、/user/me；健康检查使用 /healthz。原始 Authorization token 经认证后，可信用户存入请求 context；密码使用 bcrypt，数据库仅保存会话 token 摘要。

## 数据库与公共资源

bootstrap.Boot(ctx) 加载配置并初始化 DB、Config、Logger；全局资源只在启动和关闭时写入。配置优先级为系统环境、.env、YAML、默认值。使用标签映射到 define 中的结构体，无逐业务 loadXXX。

业务模型嵌入 Base[业务类型]，通过 user.Model(ctx).Where(...).Take(&user) 使用 GORM；事务内必须使用 tx.Model(&model.User{})，不能回到全局数据库。Base 不包含数据库字段，不改变表结构。

## Agent 与后续范围

模型与工具集中装配；用户身份在本轮 context 中传递，不写入共享 Agent 或工具对象。工具在执行时检查用户和数据归属。输出回调同步且可失败，取消和期限传到底层。

HTTP 聊天、会话历史、停止接口和 SSE 尚未实现。第一版规划参考 Ragent：GET /agent/v1/chat、POST /agent/v1/stop；SSE 断开取消执行。后续再扩展恢复与跨实例治理。

## 协作与验证

不预建无需求的仓储接口和 CRUD 框架。测试聚焦协议、身份隔离、取消与预算。代码与文档修改后等待用户确认再 commit，不自动 push。

响应统一 Success / Fail / NoAuth / Error；业务错误构造位于 define/error.go，内部原因保留但不直接打印。失败日志只使用允许的诊断字段，恢复逻辑在 middleware/recovery.go。Session 实体对应 auth_sessions，过期时间采用数据库时钟。

HTTP 认证使用启动时发布的 service.Auth，Router 仅接收 Gin 引擎；Controller 和中间件直接使用共享服务。实例运行期间只读，服务停止且请求处理结束后清空，随后释放基础资源；不是每个请求结束后清空。

账号创建由 UserService.Create 提供；AuthService 负责登录认证。公共日志中间件不透传 Logger，初始化时读取启动资源。统一响应禁止向已提交的响应追加 JSON。

部署配置直接映射到 SDK、数据库驱动及执行器，业务代码不重复进行字段预检；配置文件类型解析、真实调用失败、外部输入与授权检查继续保留。模型 URL 由部署者决定，数据库 TLS 策略保留。

## Agent 执行与输出的边界

cmd/agentcheck 直接装配模型、示例工具和服务端 Definition；internal/agent 不编码 CLI JSONL 或 HTTP SSE。EventSink 同步接收类型化事件，返回错误即结束本轮；Run 返回后不再调用回调。同步回调必须及时返回，context 不能强制终止阻塞的回调，后续 SSE 适配需要写出期限。

当前每轮创建 ADK 运行对象、工具预算和调用关联状态；注册的工具可复用，调用时从 context 读取当前用户。Java 参考项目的 UserContext 职责在 Go 中用 context 传递，不用全局用户变量或模拟线程局部状态。Tool Schema 不接受 user_id，数据归属在工具执行时验证。

Run 保留非流式单轮行为；RunMessages 使用 Runner.Run 接收完整历史并启用流式，发出带 message_id 的 text_delta 和最终 answer 快照。消息流只读一次，同时合并完整消息；失败返回部分正文但不交出可追加历史。展示工具结果由 Definition 控制，完整工具结果仍保留在可信历史中。内部事件不是 HTTP SSE 帧或持久事件总线，模型答案忠实性仍需独立评估。

模型请求期限、整轮期限、决策轮数和工具调用次数分别约束不同边界。预算属于每轮状态，不存进共享工具或当前用户配置。CLI 的 -interactive 将历史保存在当前用户的局部会话中，成功后追加，/exit 或 EOF 退出；Ctrl+C 通过主循环监听 context 取消退出 stdin 等待。阻塞输入协程只属于 CLI 进程生命周期，不能用于 HTTP 请求。尚未接入数据库聊天历史、摘要、重试和 ADK 中间件。详见 [本轮实践](lessons/A01-agent-conversation-and-streaming.md)。

## 配置与阅读入口

配置映射和密钥来源见 [配置说明](../configs/README.md)。联调问题通过 modelcheck / agentcheck 的 -prompt 参数输入；不读取 RAGENT_MODEL_PROMPT 或 RAGENT_AGENT_PROMPT。必要的复杂逻辑注释解释设计原因、状态边界与失败处理，普通赋值和 CRUD 不逐行复述。

当前优先学习 Agent / Eino 的机制，通过参考 Ragent 和项目实验检验理解。通用后台开发按需补充；不以代码已实现或测试通过代替学员能解释、修改和复现。
