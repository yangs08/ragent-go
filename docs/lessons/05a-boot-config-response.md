# 第 05 课补充：简化启动、配置与目录职责

更新：2026-10-30。状态：整改待学员审阅，未提交。

## 当前目录

```text
configs/*.yaml                 按业务拆分的普通配置
.env                           密钥与本机覆盖，不提交
internal/define/config.go      配置结构与映射标签
internal/define/auth.go        登录请求/结果、可信用户视图
internal/define/result.go      公共返回结构
internal/model/user.go         users 表的 GORM 实体
internal/bootstrap/config.go      通用配置加载
internal/bootstrap/boot.go        Boot 和公共 Config/DB/Logger
internal/service/model.go AI 模型客户端与文本流
internal/common/functions.go          真正共享的小函数
internal/controller/              HTTP 协议与中间件
internal/service/              业务流程
```

model 留给数据库实体，AI 客户端移到 common。define 放接口与配置声明。model.User.PasswordHash 不参与 JSON 输出，实体不直接作为 HTTP 响应。functions.go 当前放用户 context 存取，包名 internal，调用处使用 functions 别名；仅依赖 define，不反向依赖业务层。

## Boot 只保留 ctx

getenv/output 原本是测试注入参数，不需要出现在正式启动链路中。现在所有入口统一调用：

```go
cleanup, err := bootstrap.Boot(ctx)
// 检查 err 后 defer cleanup()
```

Boot 内部加载 configs/、.env 和系统环境，初始化日志与可选 DB。日志统一 stderr；诊断 stdout 只输出结果。测试通过临时目录和测试环境变量隔离，不再给 Boot 传测试参数。

公共资源直接声明在 bootstrap/boot.go，没有 global 目录。启动后只读，停止请求/后台工作后再 cleanup。用户身份仍在请求 context，事务仍使用局部 tx，不放进公共变量。

## 自动映射配置

系统环境 > .env > 模块 YAML > envDefault。YAML 直接解码到结构体，环境与默认值使用 caarlos0/env 根据标签映射。Config 成员的 yaml 标签决定文件名，不写 loadXXX 或字段转换表。

```go
type AgentConfig struct {
    MaxIterations int `yaml:"max_iterations" env:"RAGENT_AGENT_MAX_ITERATIONS" envDefault:"4"`
}
```

已移除配置结构的 Validate 方法与预设范围检查。解析层仍检查类型、未知字段和密钥位置；数据库、模型、Agent、认证在真正使用时检查必需条件，如连接信息、正期限和访问授权。业务输入、密码和资源授权检查继续保留。

## 删除 Diagnostics

Diagnostics 只装两个联调命令的默认提问，不是业务配置。字段、结构体和对应 YAML 支持均已删除，改用命令参数：

```sh
go run ./cmd/modelcheck -prompt "请解释什么是 Agent"
go run ./cmd/agentcheck -prompt "查询 A101 的订单状态"
```

make modelcheck / make agentcheck 仍可使用默认问题。原 RAGENT_MODEL_PROMPT / RAGENT_AGENT_PROMPT 不再读取，真实 .env 未被改写。

## 返回结构

define.Result 唯一定义业务包装，common/response.go 统一 JSON、requestId、Cache-Control。业务仍沿用 HTTP 200 + 非零 code；panic HTTP 500 的正文也使用 Result，已提交的流不能替换。健康检查与 SSE 各自使用独立协议。

## 验证与阅读

配置映射、Boot 生命周期、Agent/HTTP 回归已适配新结构。冒烟验证正常进程、无效地址、退出与模型/工具协议；错误文本不再绑定到已删除的配置 Validate。认证用真实 PostgreSQL 验证，随机测试账号在结束后清理。

依次阅读 define/config.go → bootstrap/config.go → bootstrap/boot.go → model/user.go → service/auth.go → functions.go → common/response.go。

解释：为什么测试不应让 Boot 增加参数？数据库实体和接口响应有什么区别？为什么联调问题是命令输入？公共函数为何不能依赖业务 Service？

所有修改等待确认后才可 commit。

## 认证错误、公共函数与实体查询约定

认证服务不再为每条提示声明 ErrCredentials/ErrUserInput 等变量；统一用 define.BusinessError 携带安全的 code/message，由 handler.writeError 写成公共 Result。functions.ClientError / ServerError 创建业务错误，未分类内部错误统一返回安全系统提示。GORM 的 ErrRecordNotFound 仍按 errors.Is 判断，因为它是实际流程分支。

TokenHash、ValidToken、NewSessionToken 移入 internal/common/functions.go。TokenHash 仅处理高熵会话 token，密码继续使用 bcrypt，不混用。AuthService 不再缓存 DB 或 cfg；使用 bootstrap.DB 和 bootstrap.Config.Auth，只保留为防账号枚举而复用的 dummyHash。

用户查询写法：

```go
var user model.User
err := model.UserQuery(ctx).
    Where("username = ?", username).
    Take(&user).Error
```

UserQuery 只返回原生 *gorm.DB，不另外实现 Where/First/Save。每次调用创建带本轮 context 的查询；不要共享已附加 Where 条件的查询链。事务内用 tx.Model(&model.User{})，不能调用全局 UserQuery 后意外离开事务。

Go 没有 PHP 的 User:: 静态调用语法，也没有类继承。嵌入 gorm.Model 只是组合 uint 主键、时间戳和 DeletedAt，不会赋予模型静态查询方法。当前 users 使用字符串主键且没有 deleted_at，所以不直接嵌入 gorm.Model，避免改变现有表语义。参见 [GORM 实体说明](https://gorm.io/docs/models.html)。

本次实际验证：现有 race 回归、vet、离线冒烟、模块校验和真实 PostgreSQL 认证验证均通过，测试账号已清理。代码继续等待确认，未提交。


### 模型统一的查询入口：Base[T]

数据库模型嵌入 `Base[User]`，通过 `user.Model(ctx)` 获取绑定当前模型的 GORM 查询对象。Base 只提供入口，Where、Take、Count、Updates 等继续使用 GORM 原生方法，不逐个模型编写 UserQuery，也不重复包装 CRUD。

```go
type User struct {
    Base[User] `gorm:"-" json:"-"`
    ID string `gorm:"primaryKey"`
    Username string
}

var user model.User
err := user.Model(ctx).Where("username = ?", username).Take(&user).Error
```

`gorm:"-"` 和 `json:"-"` 表明 Base 不参与表字段和 JSON 输出。这个 Base 不包含公共数据库字段，不改变现有表结构。Go 的嵌入方法不能自动识别外层类型，因此显式使用泛型参数 User 确定查询模型。

事务内仍然使用 `tx.Model(&model.User{})`，不能调用会重新使用全局 DB 的 `user.Model(ctx)`。本节替代前文的 UserQuery 示例。


### 模型调用归属 Service

AI 模型客户端创建与文本流调用统一放在 `internal/service/model.go`，供 Agent 和联调入口复用。common 维护启动资源和配置，model 维护数据库实体；Agent 编排保持在 agent 包中。原模型适配器测试同步迁移，功能不变。


### HTTP 分层收敛

原 handler 包已移除：router 统一注册路由；controller 解析请求、调用 Service；middleware 管理认证、限流和日志。接口契约移到 docs/api。统一响应函数放 common/response.go，供三个 HTTP 包共享，避免 middleware 依赖 controller；Service 不依赖 Gin。本节替代前面的旧 handler 路径。


### 统一响应和错误处理

Controller 和中间件使用 common.Success(c, data)、Fail(c, message)、NoAuth(c)、Error(c, err)。失败入口统一 Abort，但调用处仍需 return；Abort 不会终止当前 Go 函数。底层响应写出不导出，普通业务保持 HTTP 200 和既有 Result 字段。恢复中间件使用 InternalError，在已开始输出时不覆盖响应。

业务错误类型及构造集中于 define/error.go；Service 不依赖 Gin，ServerError 保留操作阶段与内部 cause，支持 errors.Is/As，Error() 仅返回安全提示。HTTP 日志记录 requestId、操作、错误类型、SQLSTATE 或取消原因，不打印原始错误文本。CLI 默认输出仍只显示安全提示。

model.Session 映射认证会话，创建与删除走 GORM；有效期由数据库时间计算，关联认证查询和有界过期清理保留 SQL 表达式。恢复逻辑已移到 middleware/recovery.go。以上替代前文 WriteResult/WriteError 和 functions.ClientError 示例。


### 启动资源统一到 bootstrap

配置装载、Boot 与 DB/Config/Logger 从 common 移到 bootstrap，HTTP 生命周期也由该包负责。bootstrap 不导入业务 Service、Controller 或根包。cmd/server 调用 Boot 后创建服务与 Gin、挂公共中间件，再调用 internal.RegisterRoutes；最后调用 bootstrap.Serve，并在关闭后清理资源。

路由集中于 internal/router.go，不再单设 router 包。健康检查处理在 controller/health.go。共享用户 context 与 token 函数移到 common/functions.go：根包现在导入 Controller 和 Service，子包若再导入根包就会循环依赖。common 保留响应和诊断能力，可依赖 bootstrap 读取日志资源，bootstrap 不反向导入 common。旧代码片段按此节路径理解。


### 认证服务使用共享实例

HTTP 启动入口在 bootstrap.Boot 成功后创建 service.Auth，成功后再创建 Gin 和注册路由。RegisterRoutes 只接收引擎；AuthController 不存 Service 字段，RequireLogin 和 LoginLimit 不再透传实例或配置。Controller 和认证中间件统一调用 service.Auth，限流初始化读取 bootstrap.Config.Auth。数据库关闭时不注册认证路由。

service.Auth 在开始接收请求前发布，HTTP 请求全部结束后清空，再关闭数据库。运行期间不得替换共享实例或写入用户、context、token 等请求数据；不支持同一进程运行多套独立配置的应用。bootstrap 不反向依赖 service。usercreate 是独立命令，仍可创建局部 AuthService，无需发布 HTTP 共享实例。


登录限流器只持有互斥锁、IP 限流状态和清理时间，不保存 AuthConfig 副本；处理请求时从 bootstrap.Config.Auth 读取启动后保持不变的配置。


### 整体封装复查

请求日志和恢复中间件统一在安装时读取 bootstrap.Logger，不再传递日志参数。限流器只持有运行状态，不保存配置副本。

账号创建归 UserService.Create，独立命令只初始化基础资源，不再为创建账号生成登录 dummyHash 或查询会话表；AuthService 保留登录、认证与退出。Service 直接使用 GORM，不新增仓储转发或 BaseService。

公共响应写出在响应已经提交时不再追加 JSON；错误入口仍中断后续处理。保留严格 JSON 输入、IP 限流、会话数据库时钟与有界清理，不改变接口协议。清理任务独立调度属于后续维护能力，本轮没有添加后台任务。

Agent 的配置属于可独立创建的执行器实例，模型适配器构造参数用于明确选择供应商实例；它们与 HTTP 中间件重复缓存全局配置不是同一情况，不为统一形式盲目删除。


模型客户端直接使用配置字段，不再重复预检空模型名、API Key、超时及 token 数量。保留 SDK 错误处理、端点安全限制及请求参数和权限校验。


### 部署配置预检整体收敛

按配置可信原则，移除模型 URL 解析与自设协议限制、认证参数正数检查、数据库字段/端口/连接池参数预检、dbdev 的重复字段检查、Agent 预算配置正数预检。模型 URL 直接交给 SDK；数据库参数交给驱动和连接池；Compose 保留其自身必填声明。配置装载仍处理 YAML/env 类型解析错误，实际连接、SDK 调用与迁移表检查仍处理失败。

保留请求参数、用户名密码、会话 token、身份归属、执行中预算和取消检查；保留必要资源存在性与数据库 TLS 策略，不把配置可信等同于外部请求可信。删除针对已移除配置预检的测试，不增加替代校验框架。本节替代此前模型 URL 必须由应用限制的说明。
