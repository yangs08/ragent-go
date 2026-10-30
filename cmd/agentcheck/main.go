package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/eino/components/tool"

	"ragent-go/internal/agent"
	"ragent-go/internal/bootstrap"
	"ragent-go/internal/common"
	"ragent-go/internal/define"
	"ragent-go/internal/example"
	"ragent-go/internal/service"
)

const defaultPrompt = "查询 A100 订单的状态，请先调用订单工具再回答。"

func main() {
	prompt := flag.String("prompt", defaultPrompt, "question for this command")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := runOrderCheck(ctx, *prompt, os.Stdout)
	if err != nil {
		slog.Error("order agent check failed", "error", err)
		os.Exit(1)
	}
}

func runOrderCheck(ctx context.Context, question string, output io.Writer) error {
	cleanup, err := bootstrap.Boot(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	if output == nil {
		return errors.New("agent output is required")
	}
	client, err := service.NewModelClient(ctx, bootstrap.Config.Model)
	if err != nil {
		return err
	}
	orderTool, err := example.NewOrderTool(example.DemoOrders())
	if err != nil {
		return err
	}
	definition := agent.Definition{Name: "order_assistant", Description: "只读订单状态查询助手", Instruction: "你是只读订单助手。回答具体订单状态前必须调用 get_order，只依据工具结果回答。订单不明时先追问，禁止声称执行写操作。", ExposeToolResults: true}
	assistant, err := agent.New(client, []tool.InvokableTool{orderTool}, definition, bootstrap.Config.Agent)
	if err != nil {
		return err
	}
	if question == "" {
		question = defaultPrompt
	}

	encoder := json.NewEncoder(output)
	ctx = common.WithUser(ctx, define.User{ID: "alice"})
	return assistant.Run(ctx, question, func(event agent.Event) error {
		return encoder.Encode(event)
	})
}
