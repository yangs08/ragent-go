package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

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
	stream := flag.Bool("stream", false, "emit assistant text deltas and final answer")
	interactive := flag.Bool("interactive", false, "keep in-memory conversation; read follow-ups from stdin")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var input io.Reader
	if *interactive {
		input = os.Stdin
	}
	err := runOrderSession(ctx, *prompt, input, os.Stdout, *stream || *interactive)
	if err != nil {
		operation := "run agentcheck"
		var business *define.BusinessError
		if errors.As(err, &business) && business.Operation != "" {
			operation = business.Operation
		}
		attrs := common.FailureAttrs(operation, err)
		attrs = append(attrs, "error", err)
		var apiError *openai.APIError
		if errors.As(err, &apiError) {
			attrs = append(attrs, "http_status", apiError.HTTPStatusCode)
		}
		if errors.Is(err, agent.ErrRunFailed) {
			// 这里只提供失败后的配置来源提示，不增加一套启动字段校验或输出凭据/响应正文。
			attrs = append(attrs, "hint", "检查 configs/model.yaml 与 .env 或终端环境中的 RAGENT_MODEL_BASE_URL、RAGENT_MODEL_NAME、RAGENT_MODEL_API_KEY；首次配置参考 .env.example")
		}
		slog.Error("order agent check failed", attrs...)
		os.Exit(1)
	}
}

func runOrderCheck(ctx context.Context, question string, output io.Writer) error {
	return runOrderSession(ctx, question, nil, output, false)
}

func runOrderSession(ctx context.Context, question string, input io.Reader, output io.Writer, streaming bool) error {
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
	emit := func(event agent.Event) error { return encoder.Encode(event) }
	if !streaming && input == nil {
		return assistant.Run(ctx, question, emit)
	}

	// 历史只属于本 CLI 的 alice 会话，不存进共享 Agent，也不从展示事件反向构建。
	var history []*schema.Message
	runTurn := func(question string) error {
		messages := append(history, schema.UserMessage(question))
		result, err := assistant.RunMessages(ctx, messages, emit)
		if err != nil {
			return err
		}
		// 失败轮次不追加历史；成功消息包含完整调用参数和工具结果，不能原样展示或打印。
		history = append(messages, result.Messages...)
		return nil
	}
	if err := runTurn(question); err != nil {
		return err
	}
	if input == nil {
		return nil
	}
	// 普通 Reader 的 Read 不一定响应 context，os.Stdin.Close 也不保证打断阻塞读取。
	// CLI 只启动一个输入协程；主循环通过 select 响应信号，退出时由进程回收 stdin。
	// 读取协程不接触模型、历史或数据库，不可把这种生命周期直接照搬到 HTTP 请求中。
	type inputLine struct {
		text string
		err  error
		done bool
	}
	lines := make(chan inputLine)
	inputCtx, stopInput := context.WithCancel(ctx)
	defer stopInput()
	go func() {
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 4096), 64*1024)
		for scanner.Scan() {
			select {
			case lines <- inputLine{text: scanner.Text()}:
			case <-inputCtx.Done():
				return
			}
			if inputCtx.Err() != nil {
				return
			}
		}
		select {
		case lines <- inputLine{err: scanner.Err(), done: true}:
		case <-inputCtx.Done():
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line := <-lines:
			if err := ctx.Err(); err != nil {
				return err
			}
			if line.done {
				return line.err
			}
			next := strings.TrimSpace(line.text)
			if next == "/exit" {
				return nil
			}
			if next == "" {
				continue
			}
			if err := runTurn(next); err != nil {
				return err
			}
		}
	}
}
