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
	"github.com/cloudwego/eino/schema"
	"ragent-go/internal/config"
)

var ErrRunFailed = errors.New("agent execution failed; check provider tool support and execution limits")
var ErrNoAnswer = errors.New("agent returned no final text answer")
var ErrToolBudget = errors.New("agent tool call budget exhausted")

type Event struct {
	Type    string          `json:"type"`
	Tool    string          `json:"tool,omitempty"`
	CallID  string          `json:"tool_call_id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Content string          `json:"content,omitempty"`
}

// EventSink consumes events synchronously and in order. Returning an error stops
// this execution. Run never invokes the sink after returning.
type EventSink func(Event) error

type Executor struct {
	model      einomodel.BaseChatModel
	catalog    ToolCatalog
	definition Definition
	options    config.AgentConfig
}

func NewExecutor(model einomodel.BaseChatModel, catalog ToolCatalog, definition Definition, options config.AgentConfig) (*Executor, error) {
	if model == nil || catalog == nil {
		return nil, errors.New("agent model and tool catalog are required")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(definition.Name) == "" || strings.TrimSpace(definition.Instruction) == "" {
		return nil, errors.New("agent name and instruction are required")
	}
	return &Executor{model: model, catalog: catalog, definition: definition, options: options}, nil
}

// Run uses ADK for the decision/action loop and emits typed events.
// The constructor is reusable; all mutable execution state is local to this run.
func (a *Executor) Run(ctx context.Context, request Request, emit EventSink) error {
	if strings.TrimSpace(request.Question) == "" || emit == nil {
		return errors.New("agent question and event sink are required")
	}
	if err := request.Principal.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, a.options.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	resolved, err := a.catalog.Resolve(ctx, request.Principal)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrRunFailed
	}
	budget := &toolBudget{limit: int32(a.options.MaxToolCalls)}
	tools := make([]tool.BaseTool, 0, len(resolved))
	names := make(map[string]bool, len(resolved))
	for _, t := range resolved {
		if t == nil {
			return ErrRunFailed
		}
		info, err := t.Info(ctx)
		if err != nil || info == nil || strings.TrimSpace(info.Name) == "" || names[info.Name] {
			return ErrRunFailed
		}
		names[info.Name] = true
		tools = append(tools, &budgetedTool{InvokableTool: t, budget: budget})
	}
	engine, err := buildEngine(ctx, a.model, a.definition, tools, a.options)
	if err != nil {
		return a.safeError(ctx, budget)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: engine, EnableStreaming: false})
	events := runner.Query(ctx, request.Question)
	pending := make(map[string]string)
	answer := ""
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return a.safeError(ctx, budget)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return a.safeError(ctx, budget)
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
			// Tool adapters own result redaction. Do not expose tool contents
			// implicitly: only explicitly approved display results leave runtime.
			var result json.RawMessage
			if a.definition.ExposeToolResults {
				result = []byte(message.Content)
				if !json.Valid(result) {
					result, _ = json.Marshal(message.Content)
				}
			}

			if err := emit(Event{Type: "tool_result", Tool: name, CallID: message.ToolCallID, Result: result}); err != nil {
				return err
			}
			continue
		}
		if message.Role != schema.Assistant {
			continue
		}
		for _, call := range message.ToolCalls {
			if call.ID == "" || !names[call.Function.Name] {
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
	if len(pending) != 0 {
		return ErrRunFailed
	}
	return emit(Event{Type: "answer", Content: answer})
}

func (a *Executor) safeError(ctx context.Context, budget *toolBudget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if budget.exhausted.Load() {
		return ErrToolBudget
	}
	return ErrRunFailed
}

type toolBudget struct {
	limit     int32
	calls     atomic.Int32
	exhausted atomic.Bool
}
type budgetedTool struct {
	tool.InvokableTool
	budget *toolBudget
}

func (t *budgetedTool) InvokableRun(ctx context.Context, args string, options ...tool.Option) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t.budget.calls.Add(1) > t.budget.limit {
		t.budget.exhausted.Store(true)
		return "", ErrToolBudget
	}
	return t.InvokableTool.InvokableRun(ctx, args, options...)
}
