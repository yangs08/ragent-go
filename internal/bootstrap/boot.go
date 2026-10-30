package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"gorm.io/gorm"

	"ragent-go/internal/database"
	"ragent-go/internal/define"
)

// Startup resources are written by Boot and read-only while the process serves.
var Config define.Config
var DB *gorm.DB
var Logger = slog.Default()

var bootMu sync.Mutex
var booted bool

// Boot initializes config, logging and the optional database before publishing
// global resources from YAML/.env/system environment. Return cleanup
// must run only after all requests and background work have stopped.
func Boot(ctx context.Context) (func(), error) {
	bootMu.Lock()
	defer bootMu.Unlock()
	if booted {
		return nil, errors.New("application already booted")
	}
	cfg, err := LoadConfig("configs", ".env", os.LookupEnv)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.Server.LogLevel}))
	var db *gorm.DB
	if cfg.Database.Enabled {
		db, err = database.Open(ctx, cfg.Database)
		if err != nil {
			return nil, err
		}
		logger.Info("database connected")
	}
	Config = cfg
	DB = db
	Logger = logger
	booted = true
	var once sync.Once
	return func() {
		once.Do(func() {
			bootMu.Lock()
			defer bootMu.Unlock()
			if db != nil {
				pool, err := db.DB()
				if err == nil {
					_ = pool.Close()
				}
			}
			DB = nil
			Config = define.Config{}
			Logger = slog.Default()
			booted = false
		})
	}, nil
}
