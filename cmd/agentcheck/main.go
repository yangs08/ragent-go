package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ragent-go/internal/agent"
	"ragent-go/internal/config"
	"ragent-go/internal/model"
	"ragent-go/internal/tooling"
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
	settings, err := model.LoadSettings(getenv)
	if err != nil {
		return err
	}
	options, err := agent.LoadOptions(getenv)
	if err != nil {
		return err
	}
	client, err := model.NewClient(ctx, settings)
	if err != nil {
		return err
	}
	// The CLI demonstrates one trusted identity. Production authentication comes later.
	orderAgent, err := agent.NewOrderAgent(client, tooling.DemoOrders(), tooling.Principal{TenantID: "acme", UserID: "alice"}, options)
	if err != nil {
		return err
	}
	question := getenv("RAGENT_AGENT_PROMPT")
	if question == "" {
		question = "查询 A100 订单的状态，请先调用订单工具再回答。"
	}
	return orderAgent.Run(ctx, question, output)
}
