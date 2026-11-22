# 配置与结构体映射

configs/ 保存按模块拆分的可提交 YAML，.env 保存密钥与部署覆盖。字段和默认值声明在 internal/define/config.go，通用加载逻辑在 internal/bootstrap/config.go。

```text
configs/
  server.yaml      HTTP、日志
  database.yaml    数据库普通连接参数与池
  model.yaml       模型普通配置
  agent.yaml       Agent 执行预算
  auth.yaml        登录会话与限流
```

每份文件直接写字段，不再包 server/agent 等根节点。define.Config 的 yaml 标签决定文件名，字段的 yaml 标签决定键；新增模块在 Config 加一个声明并提供同名 YAML，不改加载器。

## 声明即映射

```go
type AgentConfig struct {
    MaxIterations int `yaml:"max_iterations" env:"RAGENT_AGENT_MAX_ITERATIONS" envDefault:"4"`
}
```

YAML 由 yaml.v3 直接解码，环境覆盖和默认值由 github.com/caarlos0/env/v11 映射。只需在结构体增加字段和标签、在对应 YAML 填值，不写 loadAgent 或字段转换表。必要业务检查放在实际使用处，不预设配置 Validate。

优先级：系统环境变量 > .env > YAML > envDefault。现有 .env 未重写，里面原来的普通字段仍会覆盖 YAML。显式空环境变量盖掉低优先级来源，再使用该字段程序默认值或留空，实际使用处处理调用失败；不会复活 .env 中被清空的密钥。

model.yaml 不允许 api_key，database.yaml 不允许 password：密钥字段 yaml:"-"，只从 RAGENT_MODEL_API_KEY / RAGENT_DB_PASSWORD 读取。未知字段、类型错误、多份 YAML 文档会拒绝加载，错误不回显值。文件缺失时允许默认值/环境覆盖，兼容诊断二进制；正式部署应携带受审查的 YAML。

联调问题使用 modelcheck/agentcheck 的 -prompt 参数，不再提供 Diagnostics 配置，也不再读取 RAGENT_MODEL_PROMPT / RAGENT_AGENT_PROMPT。

## Boot 与资源

bootstrap/boot.go 声明 Config、DB、Logger，并由 bootstrap.Boot(ctx) 统一初始化。Service 直接读取 bootstrap.Config 对应配置；DAO 表入口使用 bootstrap.DB，事务优先使用表入口 Transaction(ctx, callback)，外部已有事务用 dao.WithTx(ctx, tx) 接入，公共 Logger 可直接使用，运行期间只读；用户身份与事务仍分别在请求 context 和局部 tx 中。停止请求和后台工作后执行 Boot 返回的 cleanup。

没有 global 目录；define/ 只按业务放共享配置、认证请求/结果与返回结构，不负责打开连接或启动服务。数据库适配器依赖 define.DatabaseConfig，避免 common 和初始化代码循环依赖。

## 本地数据库工具

make db-up / make db-stop 调用 cmd/dbdev，使用 bootstrap.LoadConfig 读取同一配置，再通过子进程环境交给 Compose。此工具只装载配置，不先连接尚未启动的数据库；其他应用命令统一 Boot。

默认数据库127.0.0.1:15432、库ragent_go、用户ragent，database.enabled=false。密码必填才能创建容器，改变配置不会自动更新已有数据卷的角色密码。不要输出 docker compose config，以免打印密码。初始化、迁移与登录参考第四、第五课。

数据库表结构与查询一起放 internal/dao，AI 客户端放 internal/resource/model.go，公共函数按职责放在 internal/common 的 response/context/token/log 文件中。日志统一写 stderr。

## 超时边界

普通 HTTP 请求统一使用 server.yaml 的 request_timeout（默认 5s），环境覆盖为 RAGENT_REQUEST_TIMEOUT。原 auth.yaml 的 request_timeout 和 RAGENT_AUTH_REQUEST_TIMEOUT 已移除，部署覆盖需迁移到新配置。RequestTimeout 中间件设置整条请求链的 context deadline，Service/DAO 不再反复创建超时 context。请求结束会释放 context；上游更短的 deadline 和取消信号仍然有效。

这是协作式取消，不是强制终止 handler 或连接读写超时；下游操作需使用传入 ctx。CLI 在命令入口设置总时限，认证初始化在 server 启动入口使用 database.connect_timeout。model.timeout、agent.timeout、database.connect_timeout、server.shutdown_timeout 分别控制模型调用、Agent 执行、数据库连接与启动检查、优雅退出，保留独立配置。未来流式 HTTP 接口需按其执行时长选择请求预算。
