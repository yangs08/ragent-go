package config

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

type DatabaseConfig struct {
	Enabled                                          bool
	Host                                             string
	Port                                             int
	Name, User, Password, SSLMode                    string
	MaxOpenConns, MaxIdleConns                       int
	ConnectTimeout, ConnMaxLifetime, ConnMaxIdleTime time.Duration
}

func loadDatabase(getenv func(string) string) (DatabaseConfig, error) {
	cfg := DatabaseConfig{Host: "127.0.0.1", Port: 15432, Name: "ragent_go", User: "ragent", SSLMode: "disable", MaxOpenConns: 10, MaxIdleConns: 5, ConnectTimeout: 5 * time.Second, ConnMaxLifetime: 30 * time.Minute, ConnMaxIdleTime: 5 * time.Minute}
	if value := getenv("RAGENT_DB_ENABLED"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return DatabaseConfig{}, errors.New("RAGENT_DB_ENABLED must be a boolean")
		}
		cfg.Enabled = parsed
	}
	for key, target := range map[string]*string{"RAGENT_DB_HOST": &cfg.Host, "RAGENT_DB_NAME": &cfg.Name, "RAGENT_DB_USER": &cfg.User, "RAGENT_DB_SSLMODE": &cfg.SSLMode} {
		if v := getenv(key); v != "" {
			*target = v
		}
	}
	cfg.Password = getenv("RAGENT_DB_PASSWORD")
	for key, target := range map[string]*int{"RAGENT_DB_PORT": &cfg.Port, "RAGENT_DB_MAX_OPEN_CONNS": &cfg.MaxOpenConns, "RAGENT_DB_MAX_IDLE_CONNS": &cfg.MaxIdleConns} {
		if value := getenv(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return DatabaseConfig{}, errors.New(key + " must be an integer")
			}
			*target = n
		}
	}
	for key, target := range map[string]*time.Duration{"RAGENT_DB_CONNECT_TIMEOUT": &cfg.ConnectTimeout, "RAGENT_DB_CONN_MAX_LIFETIME": &cfg.ConnMaxLifetime, "RAGENT_DB_CONN_MAX_IDLE_TIME": &cfg.ConnMaxIdleTime} {
		if value := getenv(key); value != "" {
			d, err := time.ParseDuration(value)
			if err != nil {
				return DatabaseConfig{}, errors.New(key + " must be a duration")
			}
			*target = d
		}
	}
	if cfg.Enabled {
		if err := cfg.Validate(); err != nil {
			return DatabaseConfig{}, err
		}
	}
	return cfg, nil
}
func (c DatabaseConfig) Validate() error {
	if !c.Enabled {
		return errors.New("database is disabled; set RAGENT_DB_ENABLED=true")
	}
	if strings.TrimSpace(c.Host) == "" || strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.User) == "" || c.Password == "" {
		return errors.New("database host, name, user and password are required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("RAGENT_DB_PORT must be between 1 and 65535")
	}
	if c.MaxOpenConns < 1 || c.MaxIdleConns < 0 || c.MaxIdleConns > c.MaxOpenConns {
		return errors.New("database pool requires max open >= 1 and idle between zero and max open")
	}
	if c.ConnectTimeout <= 0 || c.ConnectTimeout > time.Minute || c.ConnMaxLifetime <= 0 || c.ConnMaxIdleTime <= 0 {
		return errors.New("invalid database timeouts; connect timeout must be in (0,1m]")
	}
	ip := net.ParseIP(c.Host)
	local := c.Host == "localhost" || (ip != nil && ip.IsLoopback())
	if c.SSLMode != "verify-full" && !(local && c.SSLMode == "disable") {
		return errors.New("database requires sslmode=verify-full except loopback may use disable")
	}
	return nil
}
