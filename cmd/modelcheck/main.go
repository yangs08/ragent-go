package main

import (
	"context"
	"errors"
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
	if output == nil {
		return errors.New("model output is required")
	}
	client, err := model.NewClient(ctx, cfg.Model)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Model.Timeout)
	defer cancel()
	return model.StreamText(ctx, client, prompt, func(text string) error {
		n, err := io.WriteString(output, text)
		if err != nil {
			return err
		}
		if n != len(text) {
			return io.ErrShortWrite
		}
		return nil
	})
}
