package define

import (
	"log/slog"
	"time"
)

// Config 汇总应用配置；yaml 标签对应 configs 下的模块文件与字段，env 标签声明环境覆盖。
// 优先级为系统环境变量 > .env > YAML > envDefault；密钥字段 yaml:"-" 禁止从 YAML 读取。
// time.Duration 字段使用带单位的字符串，如 500ms、5s、30m、24h。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Model    ModelConfig    `yaml:"model"`
	Agent    AgentConfig    `yaml:"agent"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
}

// ServerConfig 控制 HTTP 服务、请求总时限和日志。
type ServerConfig struct {
	// 整个 HTTP 请求链的总时限，包含认证和业务处理；通过 context 取消，不强制中断 handler。
	RequestTimeout time.Duration `yaml:"request_timeout" env:"RAGENT_REQUEST_TIMEOUT" envDefault:"5s"`
	// HTTP 监听地址，格式为 host:port；127.0.0.1 仅允许本机访问。
	HTTPAddr string `yaml:"http_addr" env:"RAGENT_HTTP_ADDR" envDefault:"127.0.0.1:8080"`
	// 收到退出信号后等待在途请求结束的最长时间，超时后关闭连接。
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"RAGENT_SHUTDOWN_TIMEOUT" envDefault:"5s"`
	// 日志最低输出级别，使用 DEBUG、INFO、WARN、ERROR。
	LogLevel slog.Level `yaml:"log_level" env:"RAGENT_LOG_LEVEL" envDefault:"INFO"`
}

// AgentConfig 限制每次 Agent 执行的工作量；多轮对话每次调用重新计数。
type AgentConfig struct {
	// Eino ADK 的最大迭代次数，限制模型与工具循环；不是历史对话条数。
	MaxIterations int `yaml:"max_iterations" env:"RAGENT_AGENT_MAX_ITERATIONS" envDefault:"4"`
	// 一次 Agent 执行允许的工具调用总次数，所有工具共享预算。
	MaxToolCalls int `yaml:"max_tool_calls" env:"RAGENT_AGENT_MAX_TOOL_CALLS" envDefault:"3"`
	// 一次 Agent 执行的总时限，覆盖模型调用与工具执行；上游更短的 deadline 优先。
	Timeout time.Duration `yaml:"timeout" env:"RAGENT_AGENT_TIMEOUT" envDefault:"30s"`
}

// AuthConfig 控制登录会话和按客户端 IP 生效的登录限流。
type AuthConfig struct {
	// 登录成功后会话的有效期，按数据库时间计算，认证时检查是否过期。
	SessionTTL time.Duration `yaml:"session_ttl" env:"RAGENT_AUTH_SESSION_TTL" envDefault:"24h"`
	// 登录限流的补充周期；令牌按 LoginAttempts / LoginWindow 的速率持续补充。
	LoginWindow time.Duration `yaml:"login_window" env:"RAGENT_AUTH_LOGIN_WINDOW" envDefault:"1m"`
	// 每个 IP 的登录令牌桶容量，同时决定每个 LoginWindow 补充的令牌数；每次尝试消耗一个。
	LoginAttempts int `yaml:"login_attempts" env:"RAGENT_AUTH_LOGIN_ATTEMPTS" envDefault:"10"`
}

// DatabaseConfig 配置 PostgreSQL 连接及进程内共享连接池。
type DatabaseConfig struct {
	// 是否在 Boot 时连接数据库；关闭时 HTTP 服务仅注册健康检查路由。
	Enabled bool `yaml:"enabled" env:"RAGENT_DB_ENABLED" envDefault:"false"`
	// PostgreSQL 主机名或 IP，不包含协议和端口。
	Host string `yaml:"host" env:"RAGENT_DB_HOST" envDefault:"127.0.0.1"`
	// PostgreSQL 端口；本地示例使用宿主机映射端口 15432。
	Port int `yaml:"port" env:"RAGENT_DB_PORT" envDefault:"15432"`
	// 连接的数据库名称。
	Name string `yaml:"name" env:"RAGENT_DB_NAME" envDefault:"ragent_go"`
	// 数据库登录用户名。
	User string `yaml:"user" env:"RAGENT_DB_USER" envDefault:"ragent"`
	// 数据库密码，仅从环境变量或 .env 读取，不写入可提交的 YAML。
	Password string `yaml:"-" env:"RAGENT_DB_PASSWORD"`
	// 数据库 TLS 策略：远程连接必须使用 verify-full，仅回环地址允许 disable。
	SSLMode string `yaml:"sslmode" env:"RAGENT_DB_SSLMODE" envDefault:"disable"`
	// 连接池最大打开连接数，包含正在使用和空闲的连接；达到上限后等待可用连接。
	MaxOpenConns int `yaml:"max_open_conns" env:"RAGENT_DB_MAX_OPEN_CONNS" envDefault:"10"`
	// 连接池最多保留的空闲连接数，避免每次查询都重新建连。
	MaxIdleConns int `yaml:"max_idle_conns" env:"RAGENT_DB_MAX_IDLE_CONNS" envDefault:"5"`
	// 建连及启动连通性检查的时限，也用于启动时认证表检查；不是单条 SQL 的超时。
	ConnectTimeout time.Duration `yaml:"connect_timeout" env:"RAGENT_DB_CONNECT_TIMEOUT" envDefault:"5s"`
	// 连接从创建起可复用的最长时间，过期后由连接池回收；不强行打断正在执行的查询。
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"RAGENT_DB_CONN_MAX_LIFETIME" envDefault:"30m"`
	// 连接连续空闲多久后回收；与从创建起计时的 ConnMaxLifetime 不同。
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"RAGENT_DB_CONN_MAX_IDLE_TIME" envDefault:"5m"`
}

// ModelConfig 配置 OpenAI 兼容模型客户端，可供普通模型调用与 Agent 共用。
type ModelConfig struct {
	// 模型 API 基础地址，例如 https://provider.example/v1，不填写具体聊天接口路径。
	BaseURL string `yaml:"base_url" env:"RAGENT_MODEL_BASE_URL"`
	// 供应商提供的模型标识，对应 YAML 的 name 字段。
	Model string `yaml:"name" env:"RAGENT_MODEL_NAME"`
	// 模型服务凭证，仅从环境变量或 .env 读取。
	APIKey string `yaml:"-" env:"RAGENT_MODEL_API_KEY"`
	// 模型客户端单次请求的超时；Agent 总时限可覆盖多次模型与工具调用。
	Timeout time.Duration `yaml:"timeout" env:"RAGENT_MODEL_TIMEOUT" envDefault:"20s"`
	// 单次模型请求的生成 token 上限，不是输入长度或对话总预算；大于 0 才发送给供应商。
	MaxCompletionTokens int `yaml:"max_completion_tokens" env:"RAGENT_MODEL_MAX_COMPLETION_TOKENS" envDefault:"256"`
}
