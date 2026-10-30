package database

import (
	"context"
	"errors"
	"math"
	"net"
	"net/url"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ragent-go/internal/define"
)

// Open creates one shared SQL pool, validates connectivity within a deadline,
// and returns a GORM handle. The process owner must close its underlying sql.DB.
func Open(ctx context.Context, cfg define.DatabaseConfig) (*gorm.DB, error) {
	if !cfg.Enabled {
		return nil, errors.New("database is disabled; set RAGENT_DB_ENABLED=true")
	}
	ip := net.ParseIP(cfg.Host)
	local := cfg.Host == "localhost" || (ip != nil && ip.IsLoopback())
	if cfg.SSLMode != "verify-full" && !(local && cfg.SSLMode == "disable") {
		return nil, errors.New("database requires verify-full except loopback may disable TLS")
	}
	endpoint := url.URL{Scheme: "postgres", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), Path: "/" + cfg.Name, User: url.UserPassword(cfg.User, cfg.Password)}
	q := endpoint.Query()
	q.Set("sslmode", cfg.SSLMode)
	q.Set("connect_timeout", strconv.Itoa(int(math.Ceil(cfg.ConnectTimeout.Seconds()))))
	endpoint.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(endpoint.String()), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("initialize database connection failed")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, errors.New("initialize database pool failed")
	}
	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetMaxIdleConns(cfg.MaxIdleConns)
	pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.PingContext(pingCtx); err != nil {
		_ = pool.Close()
		if pingCtx.Err() != nil {
			return nil, pingCtx.Err()
		}
		return nil, errors.New("database connection failed; verify address and credentials")
	}
	return db, nil
}
