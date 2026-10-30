# 当前架构与设计边界

本文件描述当前实现。历史过程见 lessons；不以历史示例作为当前目录规范。

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
- internal/common/functions.go：用户 context、token 等共享小函数。
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

HTTP 认证使用启动时发布的 service.Auth，Router 仅接收 Gin 引擎；Controller 和中间件直接使用共享服务。实例运行期间只读，请求结束后清空，随后释放基础资源。

账号创建由 UserService.Create 提供；AuthService 负责登录认证。公共日志中间件不透传 Logger，初始化时读取启动资源。统一响应禁止向已提交的响应追加 JSON。

部署配置直接映射到 SDK、数据库驱动及执行器，业务代码不重复进行字段预检；配置文件类型解析、真实调用失败、外部输入与授权检查继续保留。模型 URL 由部署者决定，数据库 TLS 策略保留。
