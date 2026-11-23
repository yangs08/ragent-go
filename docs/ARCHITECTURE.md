# 当前架构与设计边界

本文件描述当前实现，是目录、装配和资源边界的统一说明。lessons 和旧实施计划保留学习与阶段验证记录，其中旧代码片段不作为当前目录规范。

## 统一封装规则

先用文件组织职责，再按确有必要的边界拆包。公共函数直接使用；有状态、资源生命周期或真实替换需求时才使用对象与接口。借鉴 action-go 的职责划分，保留本项目 cmd/internal 布局，不引入新的框架层。

| 位置 | 负责 | 不负责 |
|---|---|---|
| cmd | 各命令参数校验、启动、关闭和装配 | 通用 CLI 框架、业务规则 |
| bootstrap | 配置、DB 连接、Logger、HTTP 生命周期 | 业务服务注册、请求状态 |
| common | 响应、用户 context、token、安全日志字段 | 业务编排、数据库初始化 |
| resource | 模型等外部客户端与协议适配 | 认证、聊天等业务流程 |
| controller / middleware | HTTP 输入输出、认证与请求链 | 直接编写数据库查询 |
| service | 登录、账号等业务流程 | SDK 初始化、HTTP context |
| dao | 表结构、表名、具体查询和事务连接选择 | 共享查询条件、通用 CRUD 框架 |
| define | 配置、公共契约、常量、错误码 | 资源初始化；包内专用类型仍就近放置 |
| agent | Eino 执行与同步事件 | HTTP SSE、CLI JSONL 编码 |
| example | 学习工具与 demo.go 演示数据 | 正式订单系统或生产知识库 |

common 按 response.go、context.go、token.go、log.go 分文件，不再建立 response/identity 小包或 Responder 对象。当前接受 common 包同时包含 HTTP 工具；没有独立发布 Agent SDK 的需求，不为间接导入 Gin 提前拆包。

bootstrap/database.go 管理数据库初始化；resource/model.go 管理模型客户端。Agent 同包内用 agent.go、message.go、budget.go 区分执行、流/历史处理和每轮预算，不引入新接口层。示例工具与固定数据也只分文件。

## HTTP 调用链

Router → Middleware → Controller → Service → DAO；表结构直接定义在对应 DAO 文件中。模型接入另由 resource 提供，Agent 直接使用 Eino 组件接口。

cmd/server 在启动请求处理前调用 service.Auth.Init(ctx)，随后创建 Gin 并安装公共中间件；internal/router.go 的 RegisterRoutes(r) 只注册路由。controller.Auth/Health 与 service.Auth/User 是共享入口，Controller 与中间件直接使用 service.Auth，无配置字段与服务传递。Service 方法接收标准 context.Context，不使用 Gin context。

业务响应统一 common.OutputSuccess/OutputError，包装为 define.Result。健康检查及将来的 SSE 保持自身协议；已提交的响应不追加 JSON。认证提供 /auth/login、/auth/logout、/user/me；健康检查使用 /healthz。原始 Authorization token 经认证后，可信用户存入请求 context；密码使用 bcrypt，数据库只保存 token 摘要。

### 参数绑定与超时

HTTP JSON 输入直接使用 Gin 的 c.ShouldBindJSON(&req)，不再自行封装绑定器。登录使用 http.MaxBytesReader 限制读取至 4096 字节；Gin 默认只解码首个 JSON 值，不强制校验 Content-Type、不拒绝未知字段，也不验证尾随内容。Controller 将绑定错误转换为安全提示“请求参数不合法”，不向外输出原始解析错误。Recovery 直接使用 Gin 输出 HTTP 500 和统一错误结构。

账号业务入参使用 define.LoginRequest / CreateUserRequest，必填、长度等通用规则集中在 binding 标签。HTTP 使用 ShouldBindJSON 自动校验，CLI 在调用 Service 前直接调用 binding.Validator.ValidateStruct，不输出原始校验错误。Service 接收已校验的入参，处理账号查询、密码匹配等业务判断；新增调用入口必须先校验。bootstrap/validator.go 在包初始化时仅注册 minbytes、maxbytes、notblank 等通用标签，不增加校验包装函数。用户名格式直接在登录 Controller 和创建用户 CLI 入口判断，直接共用 define.UsernamePattern 已编译正则，不在入口重复定义变量；Service 不做格式校验，不新增校验函数或对象。密码上限按 bcrypt 的字节限制计数，登录与创建保留不同要求。

HTTP 请求时限集中到 Server.RequestTimeout，由 RequestTimeout 中间件设置一次 context deadline，覆盖认证中间件、Controller、Service 和 DAO。Service 不再读取 Auth.RequestTimeout 或重建超时；CLI 自己控制命令总时限，Auth.Init 使用启动入口提供的数据库检查时限。此机制依赖下游响应 ctx 取消，不强制打断 handler，不保证到点立即写出响应。模型、Agent 执行和停机超时独立保留。

## 数据库与公共资源

bootstrap.Boot(ctx) 加载配置并初始化 DB、Config、Logger；只在启动和关闭时写入。配置优先级为系统环境、.env、YAML、默认值，通过结构体标签装载，不逐业务维护转换表。基础资源在请求与后台工作退出后释放。

进程级 Logger 直接使用 bootstrap.Logger；日志中间件在初始化时获取 Logger，不通过路由、Controller、响应器层层传递。Service 与中间件直接读取 bootstrap.Config.Auth；不另存 cfg，也不经路由传递。登录限流实现位于 middleware/login_limit.go：LoginLimit 负责读取客户端 IP 和输出拒绝响应，私有 allow(ip) 只管理令牌桶与清理状态。每个实例最多保存 1024 个 IP，每个补充周期最多清理一次，空闲超过两个周期的条目被回收；容量满时拒绝新 IP，已有 IP 继续按自身额度判断。各实例、各进程预算独立；运行期间配置不可修改。

dao/user.go 和 session.go 同时定义完整表字段、显式列标签、TableName 和具体查询，不再另设 model 目录。普通调用通过 dao.User/dao.Session 使用 bootstrap.DB；Conn(ctx) 创建干净的 GORM 查询。事务内通过 dao.WithTx(ctx, tx) 生成局部 context 并传给各表操作；表入口无状态，不修改共享入口，不允许 nil 事务回退默认连接，连接与事务绑定统一复用 table[T]，不扩展通用 CRUD 或泛型仓储。每个 DAO 只检查自己负责的表。会话创建和过期判断统一使用数据库时钟。

依赖方向：define 为基础类型及只读定义（包括预编译的用户名正则）；bootstrap 只依赖 define 和第三方基础库；common 可依赖 bootstrap/define；业务可使用 common/resource/dao。common/bootstrap 不反向依赖业务，子包不导入 internal 根包。

## Agent 与后续范围

模型与工具集中装配；用户身份在本轮 context 中传递，不写入共享 Agent 或工具对象。工具在执行时检查用户和数据归属。输出回调同步且可失败，取消和期限传到底层。

HTTP 聊天、会话历史、停止接口和 SSE 尚未实现。第一版规划参考 Ragent：GET /agent/v1/chat、POST /agent/v1/stop；SSE 断开取消执行。后续再扩展恢复与跨实例治理。

## 协作与验证

不预建无需求的仓储接口和 CRUD 框架。测试聚焦协议、身份隔离、取消与预算。提交与推送均需用户明确授权。

响应统一 OutputSuccess / OutputError；业务错误构造位于 define/error.go，内部原因保留但不直接打印。失败日志只使用允许的诊断字段，恢复逻辑在 middleware/recovery.go。Session 实体对应 auth_sessions，过期时间采用数据库时钟。

账号创建由 service.User.Create 提供；service.Auth 负责登录认证。Auth 只保留启动时生成、运行时只读的 dummyHash，用户身份与 token 不存在共享对象中。公共响应保持函数形式。

部署配置直接映射到 SDK、数据库驱动及执行器，业务代码不重复进行字段预检；配置文件类型解析、真实调用失败、外部输入与授权检查继续保留。模型 URL 由部署者决定，数据库 TLS 策略保留。

## Agent 执行与输出的边界

cmd/agentcheck 直接装配模型、示例工具和服务端 Definition；internal/agent 不编码 CLI JSONL 或 HTTP SSE。EventSink 同步接收类型化事件，返回错误即结束本轮；Run 返回后不再调用回调。同步回调必须及时返回，context 不能强制终止阻塞的回调，后续 SSE 适配需要写出期限。

当前每轮创建 ADK 运行对象、工具预算和调用关联状态；注册的工具可复用，调用时从 context 读取当前用户。Java 参考项目的 UserContext 职责在 Go 中用 context 传递，不用全局用户变量或模拟线程局部状态。Tool Schema 不接受 user_id，数据归属在工具执行时验证。

Run 保留非流式单轮行为；RunMessages 使用 Runner.Run 接收完整历史并启用流式，发出带 message_id 的 text_delta 和最终 answer 快照。消息流只读一次，同时合并完整消息；失败返回部分正文但不交出可追加历史。展示工具结果由 Definition 控制，完整工具结果仍保留在可信历史中。内部事件不是 HTTP SSE 帧或持久事件总线，模型答案忠实性仍需独立评估。

模型请求期限、整轮期限、决策轮数和工具调用次数分别约束不同边界。预算属于每轮状态，不存进共享工具或当前用户配置。CLI 的 -interactive 将历史保存在当前用户的局部会话中，成功后追加，/exit 或 EOF 退出；Ctrl+C 通过主循环监听 context 取消退出 stdin 等待。阻塞输入协程只属于 CLI 进程生命周期，不能用于 HTTP 请求。尚未接入数据库聊天历史、摘要、重试和 ADK 中间件。详见 [本轮实践](lessons/A01-agent-conversation-and-streaming.md)。

## 配置与阅读入口

配置映射和密钥来源见 [配置说明](../configs/README.md)。联调问题通过 modelcheck / agentcheck 的 -prompt 参数输入；不读取 RAGENT_MODEL_PROMPT 或 RAGENT_AGENT_PROMPT。必要的复杂逻辑注释解释设计原因、状态边界与失败处理，普通赋值和 CRUD 不逐行复述。

当前优先学习 Agent / Eino 的机制，通过参考 Ragent 和项目实验检验理解。通用后台开发按需补充；不以代码已实现或测试通过代替学员能解释、修改和复现。

## 知识工具

CLI 同时注册 get_order 与 search_knowledge。知识工具依赖 Eino Retriever，只返回来源 ID、标题、版本、正文和截断标识，不嵌套模型生成。固定关键词 Retriever 用于机制演示，未接向量库或入库管线；真实后端负责按 context 身份过滤知识范围，本地演示语料对演示用户共享。

工具区分 found、no_match、invalid_arguments、unavailable；取消/超时与身份错误终止执行。输出最多三条、正文每条 1200 字符；字段白名单不代表已实现完整 token 预算或答案忠实性校验。订单状态不能判断退款资格，当前无写操作。详见 [知识工具实践](lessons/A02-knowledge-tool-and-agentic-rag.md)。

## DAO 使用示例

```go
user, err := dao.User.FindByUsername(ctx, username)
err = dao.User.Create(ctx, &dao.UserStruct{ID: id, Username: username, PasswordHash: hash})

// 常规 GORM 能力直接使用 Conn，不另造 CRUD API。
var users []dao.UserStruct
err = dao.User.Conn(ctx).Where("username LIKE ?", prefix+"%").Find(&users).Error

err = dao.User.Transaction(ctx, func(txCtx context.Context) error {
    return dao.User.Create(txCtx, &dao.UserStruct{ID: id, Username: username, PasswordHash: hash})
})
```

WithTx 返回事务 context，仅在事务回调内使用，不能在事务结束后继续使用或交给脱离事务生命周期的后台任务。它保留父 context 的用户身份、取消和期限，不改变父 context；表入口不存状态。每次 Conn 都新建查询，绑定 context；表入口不缓存 GORM 查询链。用户表时间字段只读映射，插入继续采用数据库默认时间；数据库结构仍由 migrations 管理。

每张表只需嵌入 `table[对应Struct]`，自动获得 `Conn(ctx)` 和 `Transaction(ctx, callback)`，无需重复编写连接处理。公共实现位于 `dao/table.go`，每次创建独立模型值和查询。`dao.WithTx(ctx, tx)` 为包级公共函数，所有表自动通过 Conn 识别事务，无逐表包装。

事务优先使用表入口的 Transaction，回调传入的 txCtx 可供同一数据库下的其他表共同使用。已有外部 GORM 事务时才用 dao.WithTx(ctx, tx) 接入；事务生命周期仍由创建方管理。公共实现委托 GORM 提交和回滚，保留 callback error 和 panic，不自行重写事务控制。

固定单表模板：完整 XxxStruct 和 TableName、var Xxx = xxxDao{}、type xxxDao struct{ table[XxxStruct] }，然后只写表专属查询。该简短声明用于组织专属方法，公共能力不再逐表实现，也不引入生成器。

## 公共依赖与实例参数

普通业务 Service、Controller 直接使用进程配置和共享服务；不再持有 AuthConfig，不为无状态用户服务创建构造函数。路由签名只有 RegisterRoutes(r)，中间件只有 RequireLogin()/LoginLimit()。

Agent 的模型、工具、Definition 和执行配置仍属于实例，允许不同实例的策略不同；预算和历史属于每轮或调用方。resource.NewModelClient 的配置实际决定具体 SDK 客户端，不属于业务层的重复缓存。bootstrap.LoadConfig/openDatabase 处理尚未发布的配置；Serve 接收具体 listener/handler 和关闭期限。这些真实参数继续保留。

Boot 将同一 Logger 发布为 bootstrap.Logger 和 slog.Default；cleanup 仅释放 DB 并清空 Config，保留日志配置供最终退出信息使用。模型 resource 与 Agent 一样，用安全业务错误保存供应商 cause，支持 errors.Is/As 检查而不将正文直接输出。

## 验证范围

全量竞态测试、静态检查、命令构建与本地模拟供应商冒烟验证覆盖 HTTP 生命周期、DAO 事务路由、Agent 流式历史和知识工具联合路径。真实 PostgreSQL 认证集成需使用专用数据库另行运行 `make auth-smoke`；历史课文的通过记录不代表本次重构已完成数据库复验。真实模型联合问答、生产检索质量与聊天 HTTP/SSE 均不在已验证范围内。
