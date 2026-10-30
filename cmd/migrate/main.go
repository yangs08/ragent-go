package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"ragent-go/internal/bootstrap"
	"ragent-go/migrations"
)

func main() {
	command := flag.String("command", "status", "status or up")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, *command); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, command string) error {
	if command != "status" && command != "up" {
		return errors.New("supported migration commands: status, up")
	}
	cleanup, err := bootstrap.Boot(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	if bootstrap.DB == nil {
		return errors.New("database is disabled; set RAGENT_DB_ENABLED=true")
	}
	db := bootstrap.DB
	pool, err := db.DB()
	if err != nil {
		return errors.New("database pool unavailable")
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return errors.New("initialize migration lock failed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, pool, migrations.Files, goose.WithDisableGlobalRegistry(true), goose.WithSessionLocker(locker))
	if err != nil {
		return errors.New("initialize migrations failed")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if command == "up" {
		applied, err := provider.Up(ctx)
		if err != nil {
			return errors.New("apply migrations failed; inspect database state before retrying")
		}
		slog.Info("database migrations applied", "count", len(applied))
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return errors.New("read migration version failed")
	}
	slog.Info("database migration status", "version", version)
	return nil
}
