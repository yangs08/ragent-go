package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ragent-go/internal/config"
	"ragent-go/internal/model"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	getenv, err := config.Environment(".env", os.LookupEnv)
	if err == nil {
		err = runCheck(ctx, getenv, os.Stdout)
	}
	if err != nil {
		slog.Error("model check failed", "error", err)
		os.Exit(1)
	}
}

func runCheck(ctx context.Context, getenv func(string) string, output io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	prompt := getenv("RAGENT_MODEL_PROMPT")
	if prompt == "" {
		prompt = "请用一句话解释什么是 Agent。"
	}
	return model.StreamText(ctx, cfg.Model, prompt, output)
}
