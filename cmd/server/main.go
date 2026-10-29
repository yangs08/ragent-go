package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/config"
	httptransport "ragent-go/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Process-level framework setup; constructors do not mutate Gin global mode.
	gin.SetMode(gin.ReleaseMode)
	getenv, err := config.Environment(".env", os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runServer(ctx, getenv, os.Stdout)
}

func runServer(ctx context.Context, getenv func(string) string, output io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: cfg.Server.LogLevel}))
	listener, err := net.Listen("tcp", cfg.Server.HTTPAddr)
	if err != nil {
		return err
	}
	logger.Info("server started", "address", listener.Addr().String())
	err = bootstrap.Serve(ctx, listener, httptransport.NewHandler(logger), cfg.Server.ShutdownTimeout)
	if err == nil {
		logger.Info("server stopped gracefully")
	}
	return err
}
