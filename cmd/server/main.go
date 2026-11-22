package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	app "ragent-go/internal"
	"ragent-go/internal/bootstrap"
	"ragent-go/internal/middleware"
	"ragent-go/internal/service"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runServer(ctx)
}

func runServer(ctx context.Context) error {
	cleanup, err := bootstrap.Boot(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	if bootstrap.DB != nil {
		initCtx, cancel := context.WithTimeout(ctx, bootstrap.Config.Database.ConnectTimeout)
		err := service.Auth.Init(initCtx)
		cancel()
		if err != nil {
			return err
		}
	}
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.RedirectTrailingSlash = false
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.RequestLog(), middleware.Recovery(), middleware.RequestTimeout())
	app.RegisterRoutes(r)
	listener, err := net.Listen("tcp", bootstrap.Config.Server.HTTPAddr)
	if err != nil {
		return err
	}
	bootstrap.Logger.Info("server started", "address", listener.Addr().String())
	err = bootstrap.Serve(ctx, listener, r, bootstrap.Config.Server.ShutdownTimeout)
	if err == nil {
		bootstrap.Logger.Info("server stopped gracefully")
	}
	return err
}
