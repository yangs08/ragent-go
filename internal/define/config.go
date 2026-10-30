package define

import (
	"log/slog"
	"time"
)

// Configuration tags declare YAML keys, environment overrides and code defaults.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Model    ModelConfig    `yaml:"model"`
	Agent    AgentConfig    `yaml:"agent"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	HTTPAddr        string        `yaml:"http_addr" env:"RAGENT_HTTP_ADDR" envDefault:"127.0.0.1:8080"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"RAGENT_SHUTDOWN_TIMEOUT" envDefault:"5s"`
	LogLevel        slog.Level    `yaml:"log_level" env:"RAGENT_LOG_LEVEL" envDefault:"INFO"`
}

type AgentConfig struct {
	MaxIterations int           `yaml:"max_iterations" env:"RAGENT_AGENT_MAX_ITERATIONS" envDefault:"4"`
	MaxToolCalls  int           `yaml:"max_tool_calls" env:"RAGENT_AGENT_MAX_TOOL_CALLS" envDefault:"3"`
	Timeout       time.Duration `yaml:"timeout" env:"RAGENT_AGENT_TIMEOUT" envDefault:"30s"`
}

type AuthConfig struct {
	SessionTTL     time.Duration `yaml:"session_ttl" env:"RAGENT_AUTH_SESSION_TTL" envDefault:"24h"`
	RequestTimeout time.Duration `yaml:"request_timeout" env:"RAGENT_AUTH_REQUEST_TIMEOUT" envDefault:"5s"`
	LoginWindow    time.Duration `yaml:"login_window" env:"RAGENT_AUTH_LOGIN_WINDOW" envDefault:"1m"`
	LoginAttempts  int           `yaml:"login_attempts" env:"RAGENT_AUTH_LOGIN_ATTEMPTS" envDefault:"10"`
}

type DatabaseConfig struct {
	Enabled         bool          `yaml:"enabled" env:"RAGENT_DB_ENABLED" envDefault:"false"`
	Host            string        `yaml:"host" env:"RAGENT_DB_HOST" envDefault:"127.0.0.1"`
	Port            int           `yaml:"port" env:"RAGENT_DB_PORT" envDefault:"15432"`
	Name            string        `yaml:"name" env:"RAGENT_DB_NAME" envDefault:"ragent_go"`
	User            string        `yaml:"user" env:"RAGENT_DB_USER" envDefault:"ragent"`
	Password        string        `yaml:"-" env:"RAGENT_DB_PASSWORD"`
	SSLMode         string        `yaml:"sslmode" env:"RAGENT_DB_SSLMODE" envDefault:"disable"`
	MaxOpenConns    int           `yaml:"max_open_conns" env:"RAGENT_DB_MAX_OPEN_CONNS" envDefault:"10"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"RAGENT_DB_MAX_IDLE_CONNS" envDefault:"5"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout" env:"RAGENT_DB_CONNECT_TIMEOUT" envDefault:"5s"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"RAGENT_DB_CONN_MAX_LIFETIME" envDefault:"30m"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"RAGENT_DB_CONN_MAX_IDLE_TIME" envDefault:"5m"`
}

type ModelConfig struct {
	BaseURL             string        `yaml:"base_url" env:"RAGENT_MODEL_BASE_URL"`
	Model               string        `yaml:"name" env:"RAGENT_MODEL_NAME"`
	APIKey              string        `yaml:"-" env:"RAGENT_MODEL_API_KEY"`
	Timeout             time.Duration `yaml:"timeout" env:"RAGENT_MODEL_TIMEOUT" envDefault:"20s"`
	MaxCompletionTokens int           `yaml:"max_completion_tokens" env:"RAGENT_MODEL_MAX_COMPLETION_TOKENS" envDefault:"256"`
}
