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

	"github.com/cloudwego/eino/components/tool"

	"ragent-go/internal/agent"
	"ragent-go/internal/config"
	"ragent-go/internal/example"
	"ragent-go/internal/model"
	"ragent-go/internal/usercontext"
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
	client, err := model.NewClient(ctx, cfg.Model)
	if err != nil {
		return err
	}
	orderTool, err := example.NewOrderTool(example.DemoOrders())
	if err != nil {
		return err
	}
	definition := agent.Definition{Name: "order_assistant", Description: "只读订单状态查询助手", Instruction: "你是只读订单助手。回答具体订单状态前必须调用 get_order，只依据工具结果回答。订单不明时先追问，禁止声称执行写操作。", ExposeToolResults: true}
	assistant, err := agent.New(client, []tool.InvokableTool{orderTool}, definition, cfg.Agent)
	if err != nil {
		return err
	}
	question := getenv("RAGENT_AGENT_PROMPT")
	if question == "" {
		question = "查询 A100 订单的状态，请先调用订单工具再回答。"
	}
	encoder := json.NewEncoder(output)
	ctx = usercontext.WithUser(ctx, usercontext.User{ID: "alice"})
	return assistant.Run(ctx, question, func(event agent.Event) error {
		return encoder.Encode(event)
	})
}
