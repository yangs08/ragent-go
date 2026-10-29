package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"ragent-go/internal/config"
	"ragent-go/internal/tooling"
)

var ErrRunFailed = errors.New("agent execution failed; check provider tool support and execution limits")
var ErrNoToolEvidence = errors.New("agent answered without order query evidence")
var ErrNoAnswer = errors.New("agent returned no final text answer")
var ErrToolBudget = errors.New("agent tool call budget exhausted")

type Event struct {
	Type    string               `json:"type"`
	Tool    string               `json:"tool,omitempty"`
	CallID  string               `json:"tool_call_id,omitempty"`
	Result  *tooling.OrderResult `json:"result,omitempty"`
	Content string               `json:"content,omitempty"`
}

// EventSink consumes events synchronously and in order. Returning an error stops
// this execution. Run never invokes the sink after returning.
type EventSink func(Event) error

type OrderAgent struct {
	model     einomodel.BaseChatModel
	orderTool tool.InvokableTool
	options   config.AgentConfig
}

func NewOrderAgent(model einomodel.BaseChatModel, store tooling.OrderStore, principal tooling.Principal, options config.AgentConfig) (*OrderAgent, error) {
	if model == nil {
		return nil, errors.New("agent model is required")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	orderTool, err := tooling.NewOrderTool(store, principal)
	if err != nil {
		return nil, err
	}
	return &OrderAgent{model: model, orderTool: orderTool, options: options}, nil
}

// Run uses ADK for the decision/action loop and emits typed events.
// The constructor is reusable; all mutable execution state is local to this run.
func (a *OrderAgent) Run(ctx context.Context, question string, emit EventSink) error {
	if strings.TrimSpace(question) == "" || emit == nil {
		return errors.New("agent question and event sink are required")
	}
	ctx, cancel := context.WithTimeout(ctx, a.options.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	guarded := &budgetedTool{InvokableTool: a.orderTool, limit: int32(a.options.MaxToolCalls)}
	engine, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "order_assistant",
		Description:   "只读订单状态查询助手",
		Instruction:   "你是只读订单助手。回答订单状态前必须调用 get_order，只依据本次工具结果回答。订单不明时追问，查询失败时明确说明，禁止声称已执行写操作。订单编号示例为 A100。",
		Model:         a.model,
		MaxIterations: a.options.MaxIterations,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: []tool.BaseTool{guarded}}},
	})
	if err != nil {
		return a.safeError(ctx, guarded)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: engine, EnableStreaming: false})
	events := runner.Query(ctx, question)
	pending := make(map[string]string)
	answer := ""
	results := 0
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return a.safeError(ctx, guarded)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return a.safeError(ctx, guarded)
		}
		if message == nil {
			continue
		}
		if message.Role == schema.Tool {
			name, exists := pending[message.ToolCallID]
			if !exists {
				return ErrRunFailed
			}
			delete(pending, message.ToolCallID)
			var result tooling.OrderResult
			if err := json.Unmarshal([]byte(message.Content), &result); err != nil {
				return ErrRunFailed
			}
			if err := emit(Event{Type: "tool_result", Tool: name, CallID: message.ToolCallID, Result: &result}); err != nil {
				return err
			}
			results++
			continue
		}
		if message.Role != schema.Assistant {
			continue
		}
		for _, call := range message.ToolCalls {
			if call.ID == "" || call.Function.Name != "get_order" {
				return ErrRunFailed
			}
			if _, exists := pending[call.ID]; exists {
				return ErrRunFailed
			}
			pending[call.ID] = call.Function.Name
			// Raw model arguments may contain rejected sensitive fields; do not print them.
			if err := emit(Event{Type: "tool_call", Tool: call.Function.Name, CallID: call.ID}); err != nil {
				return err
			}
		}
		if len(message.ToolCalls) == 0 && strings.TrimSpace(message.Content) != "" {
			answer = message.Content
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if answer == "" {
		return ErrNoAnswer
	}
	if results == 0 || guarded.completed.Load() == 0 {
		return ErrNoToolEvidence
	}
	if len(pending) != 0 {
		return ErrRunFailed
	}
	return emit(Event{Type: "answer", Content: answer})
}

func (a *OrderAgent) safeError(ctx context.Context, tool *budgetedTool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tool.exhausted.Load() {
		return ErrToolBudget
	}
	return ErrRunFailed
}

type budgetedTool struct {
	tool.InvokableTool
	limit     int32
	calls     atomic.Int32
	completed atomic.Int32
	exhausted atomic.Bool
}

func (t *budgetedTool) InvokableRun(ctx context.Context, args string, options ...tool.Option) (string, error) {
	if t.calls.Add(1) > t.limit {
		t.exhausted.Store(true)
		return "", ErrToolBudget
	}
	result, err := t.InvokableTool.InvokableRun(ctx, args, options...)
	if err == nil {
		t.completed.Add(1)
	}
	return result, err
}
