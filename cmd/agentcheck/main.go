package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ragent-go/internal/agent"
	"ragent-go/internal/bootstrap"
	"ragent-go/internal/config"
	"ragent-go/internal/identity"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	getenv, err := config.Environment(".env", os.LookupEnv)
	if err == nil {
		err = runOrderCheck(ctx, getenv, os.Stdout)
	}
	if err != nil {
		slog.Error("order agent check failed", "error", err)
		os.Exit(1)
	}
}

func runOrderCheck(ctx context.Context, getenv func(string) string, output io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	if output == nil {
		return errors.New("agent output is required")
	}
	executor, err := bootstrap.NewDevelopmentAgent(ctx, cfg)
	if err != nil {
		return err
	}
	question := getenv("RAGENT_AGENT_PROMPT")
	if question == "" {
		question = "查询 A100 订单的状态，请先调用订单工具再回答。"
	}
	encoder := json.NewEncoder(output)
	return executor.Run(ctx, agent.Request{Question: question, Principal: identity.Principal{TenantID: "acme", UserID: "alice"}}, func(event agent.Event) error {
		return encoder.Encode(event)
	})
}
