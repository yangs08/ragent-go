package config

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
	LogLevel        slog.Level
}

// Load validates configuration before the application opens any listener.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{HTTPAddr: "127.0.0.1:8080", ShutdownTimeout: 5 * time.Second, LogLevel: slog.LevelInfo}
	if value := getenv("RAGENT_HTTP_ADDR"); value != "" {
		cfg.HTTPAddr = value
	}
	host, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil || host == "" {
		return Config{}, fmt.Errorf("RAGENT_HTTP_ADDR must include a host and valid port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("RAGENT_HTTP_ADDR port must be between 1 and 65535")
	}
	if value := getenv("RAGENT_SHUTDOWN_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 || timeout > time.Minute {
			return Config{}, fmt.Errorf("RAGENT_SHUTDOWN_TIMEOUT must be greater than zero and at most 1m")
		}
		cfg.ShutdownTimeout = timeout
	}
	if value := getenv("RAGENT_LOG_LEVEL"); value != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(value)); err != nil {
			return Config{}, fmt.Errorf("RAGENT_LOG_LEVEL must be a valid slog level")
		}
	}
	return cfg, nil
}
