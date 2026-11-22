# 项目协作约定

- 修改代码和文档后，等待用户明确确认才可 git commit；不自动提交或推送。
- 当前结构以 docs/ARCHITECTURE.md 为准。借鉴 action-go 的分层职责，保持 cmd + internal；不照搬框架或预建未来模块。
- Controller 处理 HTTP 输入输出，Service 组织业务，DAO 按表集中定义 XxxStruct、表名和查询。简单能力不强制穿过所有层。
- 先按文件划分职责；只有独立边界、规模或复用需求才新增包。不为少量公共函数建立 response、identity、logger 等小包。
- common 统一放 response.go、context.go、token.go、log.go；普通能力直接调用函数，不增加 Responder、Manager 等无状态包装对象。
- bootstrap 统一配置装载、数据库连接、Logger 与 HTTP 生命周期；不导入 common、resource、service、controller 等上层包。define 不初始化外部资源；固定正则可在 define 编译一次并只读使用。
- configs/*.yaml + .env 分层配置；结构体和标签在 define/config.go，装载逻辑在 bootstrap/config.go。不逐业务写 loadXXX 或维护字段转换表。
- Boot(ctx) 统一初始化 Config、DB、Logger，运行期间只读；所有请求和后台工作退出后 cleanup。不要为测试给 Boot 透传 getenv/output。
- Service 与中间件直接读取 bootstrap.Config 对应业务配置，不缓存 cfg 字段或层层传递；DAO 统一使用 dao.User/dao.Session 表入口，默认连接来自 bootstrap.DB；稳定的公共 Logger 直接使用 bootstrap.Logger，不层层传递日志器。请求用户、历史、预算、事务不得存为全局资源。
- 表结构与查询同文件，不写每表 NewXxx。Conn(ctx) 创建独立 GORM 查询；dao.WithTx(ctx, tx) 返回局部事务 context，回调内传给 DAO，事务绑定后不得回退默认连接。连接与事务绑定统一由 dao/table.go 的 table[T] 实现，各表保留 xxxDao 嵌入声明，直接获得 Conn 与 Transaction；事务优先使用 dao.Xxx.Transaction(ctx, callback)，外部已有事务才用 WithTx 接入；WithTx 是包级公共函数，不在每张表实现；不重复实现 Conn，不建立通用 CRUD/泛型仓储或重写 ORM 语法。
- 业务 Service、Controller 使用包级只读入口，取消仅用于接收公共依赖的构造函数；有实例依赖的 Agent、模型客户端和工具继续使用构造函数。接口只用于真实替换点，不给每个类型机械配接口。
- HTTP 使用 internal/router.go / middleware / controller；子包不反向导入 internal 根包。RegisterRoutes 只注册路由，cmd/server 装配应用。
- service.Auth/User、controller.Auth/Health 为共享入口，不存请求状态。service.Auth.Init(ctx) 仅在启动接收请求前准备 dummyHash；禁止运行期重初始化。RegisterRoutes(r)、RequireLogin()、LoginLimit() 不接收服务或配置；HTTP context 不传入 Service，使用标准 context.Context。
- 统一响应使用 common.OutputSuccess/OutputError 和 define.Result；不在 Handler 定义新包装。健康检查及将来的 SSE 使用自身协议；已提交响应不能追加 JSON。
- 公共业务错误使用 define.BusinessError；日志保留安全诊断字段，不直接输出密码、token、SQL 或供应商响应正文。
- resource 封装外部客户端和协议适配，当前模型接入在 resource/model.go；Service 不承载 SDK 初始化工具函数。不为 SDK 再造一套接口。
- agent 按执行、消息处理、预算分文件，保留 Eino 接口和同步事件回调；不引入事件总线、插件容器或无需求的策略层。
- example 保留学习工具；工具适配与演示数据同包分文件（demo.go），不冒充正式业务。工具与历史中的用户必须来自可信 context。
- 测试关注身份隔离、取消、流式协议、预算、事务和关键失败路径；不为目录移动、字段传递或简单路由写镜像测试。
- 信任部署配置，不重复预检模型名、Key、URL、超时和 token 数；保留解析错误、真实调用错误、外部输入/权限校验和数据库 TLS 策略。
- cmd 只管理各命令生命周期与必要装配；不为了统一几行启动代码抽象 CLI 框架。migrations 独立执行，不随 HTTP 启动自动改表。
- Makefile 的 build 覆盖 ./cmd/...；改动后按影响运行测试、vet、构建和 smoke；外部环境不可用时明确记录未验证项。
- 新代码遵循 gofmt，导入按标准库、第三方、项目包分组；复杂逻辑用中文解释设计原因及失败边界，不逐行注释赋值和 CRUD。
- 教学以 Agent / Eino 为主线；普通 CRUD 和封装按需补充，不以代码完成或测试通过推断学员已掌握。

- bootstrap.Logger 与 slog.Default 使用同一个配置实例，cleanup 释放 DB/Config 后保留 Logger，保证退出错误仍按配置输出。模型错误保留分类与内部 cause，Error() 不暴露供应商正文。
- agent 的模型、工具和执行配置属于实例；resource.NewModelClient 的配置用于创建具体客户端；bootstrap 的配置装载和 openDatabase 接收尚未发布的配置。这些参数有真实边界，不因公共业务 Service 简化而全局化。

- HTTP 必填字段使用 binding:"required" 标签，由 Gin 的 ShouldBind / ShouldBindJSON 自动校验；参数绑定直接使用这两个方法，不在 common 重写绑定器。请求读取限额使用标准库 http.MaxBytesReader；必填、长度等通用字段规则放在 binding 标签，由 HTTP/CLI 入口统一校验；用户名格式直接在 Controller 判断，CLI 在自己的入口判断；Service 只处理账号查询、密码匹配等业务流程，不重复入参校验。不得新增绕过校验的调用入口。bootstrap 只注册通用自定义标签，业务格式判断留在 Controller/CLI 入口；不另写绑定器或 Validate 包装。

- HTTP 总时限由 middleware.RequestTimeout 使用 Server.RequestTimeout 在入口设置一次，Service/DAO 沿用传入 ctx；CLI 和启动检查在各自入口设置 deadline，不为每个 Service 方法单独配置或重复创建超时。模型、Agent、数据库连接和优雅退出时限保留各自职责。

- 配置及其他含义不直观的定义补充中文注释，说明用途、单位、生效范围和容易混淆的差异；注释须与实际实现一致，不仅重复字段名。
