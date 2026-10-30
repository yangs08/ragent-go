# 项目协作约定

- 修改代码和文档后，等待用户明确确认，才可以执行 git commit；不得自动提交或推送。
- 采用简单的 Controller / Service 项目结构，按实际需求增加抽象。
- 测试聚焦关键风险，不为简单字段传递和目录变动补测试。
- 公共配置由 configs/*.yaml 和 .env 分层加载；使用 bootstrap.Boot 统一初始化 bootstrap.Config、bootstrap.DB、bootstrap.Logger。
- 业务响应复用 define.Result 与 common/response.go，不在单个 Handler 另行定义返回包装。
- 配置结构在 internal/define/config.go，通用 .env/YAML 装载在 internal/bootstrap/config.go，不再单设配置代码目录。
- 配置与共享数据结构按业务维护在 internal/define；bootstrap/config.go 只实现通用装载，不为每个结构体写 loadXXX 或字段转换表。
- 公共启动资源直接声明在 internal/bootstrap/boot.go，不创建 global 目录。define 不负责资源初始化。
- Boot 只接收 ctx，内部统一配置和日志，不为测试透传 getenv/output。
- internal/model 放数据库实体，AI 客户端在 service/model.go，define 放配置和接口结构。
- 不预设 Diagnostics 和配置 Validate；必要检查在实际使用处。internal/common/functions.go 仅放不反向依赖业务层的共享小函数。
- 认证业务错误统一使用 define.BusinessError；通用 token 函数放 internal/common/functions.go，Service 不重复缓存全局 DB/配置。
- 模型查询直接复用 GORM 链，不模拟 PHP 静态语法或自造 ORM；事务内使用当前 tx。

- HTTP 固定为 internal/router.go / middleware / controller；Service 不依赖 Gin。响应函数位于 common/response.go，接口契约位于 docs/api。

- bootstrap 不导入 common 或业务包；cmd/server 装配业务对象和 Gin，internal.RegisterRoutes 只注册路由。子包不得反向导入 internal 根包。

- HTTP AuthService 通过 service.Auth 共享；cmd/server 在注册路由前初始化，请求结束后清空；禁止存请求状态。RegisterRoutes 只接收 Gin 引擎。

- 简单路由注册不单独编写测试；验证聚焦业务规则、认证隔离和关键失败路径，优先复用集成验证。

- 信任部署配置，不在业务代码重复检查模型名、API Key、超时和 token 数量；保留解析错误、真实调用错误及外部输入与权限校验。模型 URL 与协议由部署配置负责，不额外限制；数据库 TLS 策略保留。
