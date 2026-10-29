package main

import (
	"context"
	"crypto/rand"
	"errors"
	"gorm.io/gorm"
	"log/slog"
	"os"
	"ragent-go/internal/config"
	"ragent-go/internal/database"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("database check failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	getenv, err := config.Environment(".env", os.LookupEnv)
	if err != nil {
		return err
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("database pool unavailable")
	}
	defer pool.Close()
	marker := rand.Text()
	rollback := errors.New("intentional rollback")
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)", marker, "dbcheck_"+marker, "not-a-login-credential").Error; err != nil {
			return errors.New("users insert failed; apply migrations first")
		}
		var count int64
		if err := tx.Table("users").Where("id = ?", marker).Count(&count).Error; err != nil || count != 1 {
			return errors.New("transaction read failed")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		return errors.New("transaction check failed")
	}
	var count int64
	if err := db.WithContext(ctx).Table("users").Where("id = ?", marker).Count(&count).Error; err != nil || count != 0 {
		return errors.New("transaction rollback verification failed")
	}
	slog.Info("database check passed", "checks", "connect, transaction read, rollback")
	return nil
}
