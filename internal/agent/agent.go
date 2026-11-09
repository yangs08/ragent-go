package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ragent-go/internal/define"
)

var ErrRunFailed = errors.New("agent execution failed; check provider tool support and execution limits")
var ErrNoAnswer = errors.New("agent returned no final text answer")
var ErrToolBudget = errors.New("agent tool call budget exhausted")

type Event struct {
	Type      string          `json:"type"`
	MessageID string          `json:"message_id,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	CallID    string          `json:"tool_call_id,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Content   string          `json:"content,omitempty"`
}

// EventSink consumes events synchronously and in order. Returning an error stops
// this execution. Run never invokes the sink after returning.
type EventSink func(Event) error

type Agent struct {
	model      einomodel.BaseChatModel
	tools      []tool.InvokableTool
	definition Definition
	cfg        define.AgentConfig
}

func New(model einomodel.BaseChatModel, tools []tool.InvokableTool, definition Definition, cfg define.AgentConfig) (*Agent, error) {
	if model == nil {
		return nil, errors.New("agent model is required")
	}
	if strings.TrimSpace(definition.Name) == "" || strings.TrimSpace(definition.Instruction) == "" {
		return nil, errors.New("agent name and instruction are required")
	}
	return &Agent{model: model, tools: append([]tool.InvokableTool(nil), tools...), definition: definition, cfg: cfg}, nil
}

// Run 保留单轮完整事件协议，供既有 CLI 和调用方继续使用。
func (a *Agent) Run(ctx context.Context, question string, emit EventSink) error {
	_, err := a.run(ctx, []*schema.Message{schema.UserMessage(question)}, emit, false)
	return err
}

// RunResult 的 Messages 只在成功时包含本轮新增的完整消息，不包含输入历史。
// Answer 在失败时可能是最后一条 assistant 的部分正文，不能当作成功答案。
// 完整工具结果仅供可信的会话上下文使用，不应直接作为 HTTP 响应或普通日志输出。
type RunResult struct {
	Messages []*schema.Message
	Answer   string
}

// RunMessages 接受调用方管理的历史，最后一条必须是当前用户问题。
// 历史和身份必须来自同一可信会话；执行器不存共享历史，也不判断历史的用户归属。
// text_delta 是普通 assistant 文本增量（可能含调用工具前的说明）；answer 是最终快照。
func (a *Agent) RunMessages(ctx context.Context, messages []*schema.Message, emit EventSink) (*RunResult, error) {
	return a.run(ctx, messages, emit, true)
}

func (a *Agent) run(ctx context.Context, messages []*schema.Message, emit EventSink, streaming bool) (*RunResult, error) {
	result := &RunResult{}
	if emit == nil {
		return result, errors.New("agent event sink is required")
	}
	if err := validateMessages(messages); err != nil {
		return result, err
	}
	// ADK 会为消息补充元数据；复制完整历史，避免改写调用方保存的消息或并发读取的数据。
	data, err := json.Marshal(messages)
	if err != nil {
		return result, errors.New("agent history cannot be copied")
	}
	var input []*schema.Message
	if err := json.Unmarshal(data, &input); err != nil {
		return result, errors.New("agent history cannot be copied")
	}
	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	budget := &toolBudget{limit: int32(a.cfg.MaxToolCalls)}
	tools := make([]tool.BaseTool, 0, len(a.tools))
	names := make(map[string]bool, len(a.tools))
	for _, t := range a.tools {
		if t == nil {
			return result, ErrRunFailed
		}
		info, err := t.Info(ctx)
		if err != nil || info == nil || strings.TrimSpace(info.Name) == "" || names[info.Name] {
			return result, ErrRunFailed
		}
		names[info.Name] = true
		tools = append(tools, &budgetedTool{InvokableTool: t, budget: budget})
	}
	engine, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: a.definition.Name, Description: a.definition.Description, Instruction: a.definition.Instruction,
		Model: a.model, MaxIterations: a.cfg.MaxIterations,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
	})
	if err != nil {
		return result, a.safeError(ctx, budget, "create agent", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: engine, EnableStreaming: streaming})
	events := runner.Run(ctx, input)
	pending := make(map[string]string)
	var completed []*schema.Message
	answer, answerID := "", ""
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return result, a.safeError(ctx, budget, "run agent", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		messageID := ""
		if streaming {
			messageID = rand.Text()
		}
		if output.Role == schema.Assistant {
			result.Answer = ""
		}
		var sinkErr error
		message, err := consumeMessage(ctx, output, func(text string) error {
			result.Answer += text
			if !streaming {
				return nil
			}
			sinkErr = emit(Event{Type: "text_delta", MessageID: messageID, Content: text})
			return sinkErr
		})
		if err != nil {
			if sinkErr != nil {
				return result, sinkErr
			}
			return result, a.safeError(ctx, budget, "read agent message", err)
		}
		if message == nil {
			continue
		}
		completed = append(completed, message)
		if message.Role == schema.Tool {
			name, exists := pending[message.ToolCallID]
			if !exists {
				return result, ErrRunFailed
			}
			delete(pending, message.ToolCallID)
			// 展示结果可隐藏；完整 tool 消息仍保留在可信历史中，供下一轮模型使用。
			var display json.RawMessage
			if a.definition.ExposeToolResults {
				display = []byte(message.Content)
				if !json.Valid(display) {
					display, _ = json.Marshal(message.Content)
				}
			}
			if err := emit(Event{Type: "tool_result", Tool: name, CallID: message.ToolCallID, Result: display}); err != nil {
				return result, err
			}
			continue
		}
		if message.Role != schema.Assistant {
			continue
		}
		// 工具名和参数可能跨片段到达，必须在完整消息合并后核对，不能逐片自行执行工具。
		for _, call := range message.ToolCalls {
			if call.ID == "" || !names[call.Function.Name] || pending[call.ID] != "" {
				return result, ErrRunFailed
			}
			pending[call.ID] = call.Function.Name
			// 不向展示事件泄露原始工具参数；调用和结果通过 CallID 关联。
			if err := emit(Event{Type: "tool_call", Tool: call.Function.Name, CallID: call.ID}); err != nil {
				return result, err
			}
		}
		if len(message.ToolCalls) == 0 && strings.TrimSpace(message.Content) != "" {
			answer, answerID = message.Content, messageID
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if answer == "" {
		return result, ErrNoAnswer
	}
	if len(pending) != 0 {
		return result, ErrRunFailed
	}
	result.Answer = answer
	if err := emit(Event{Type: "answer", MessageID: answerID, Content: answer}); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// 只有整轮成功且消费者接受最终事件，才交出可追加历史；失败不暴露未配对消息链。
	result.Messages = completed
	return result, nil
}

func consumeMessage(ctx context.Context, output *adk.MessageVariant, onText func(string) error) (*schema.Message, error) {
	if !output.IsStreaming {
		message := output.Message
		if message != nil && message.Role == schema.Assistant && message.Content != "" {
			if err := onText(message.Content); err != nil {
				return nil, err
			}
		}
		return message, nil
	}
	reader := output.MessageStream
	if reader == nil {
		return nil, ErrRunFailed
	}
	defer reader.Close()
	var parts []*schema.Message
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if part == nil {
			continue
		}
		parts = append(parts, part)
		if (output.Role == schema.Assistant || part.Role == schema.Assistant) && part.Content != "" {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := onText(part.Content); err != nil {
				return nil, err
			}
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return schema.ConcatMessages(parts)
}

// 历史来自可信存储，仍检查角色和工具配对，避免坏记录进入供应商协议。
// System 指令由服务端 Definition 管理；当前接口用于文本聊天，不接受客户端自选角色。
func validateMessages(messages []*schema.Message) error {
	invalid := errors.New("agent history must contain paired tool messages and end with a nonempty user question")
	if len(messages) == 0 {
		return invalid
	}
	pending := map[string]bool{}
	for _, m := range messages {
		if m == nil {
			return invalid
		}
		if m.Role != schema.Assistant && len(m.ToolCalls) != 0 {
			return invalid
		}
		if m.Role != schema.Tool && m.ToolCallID != "" {
			return invalid
		}
		switch m.Role {
		case schema.User:
			if len(pending) != 0 {
				return invalid
			}
		case schema.Assistant:
			if len(pending) != 0 {
				return invalid
			}
			for _, call := range m.ToolCalls {
				if call.ID == "" || strings.TrimSpace(call.Function.Name) == "" || pending[call.ID] {
					return invalid
				}
				pending[call.ID] = true
			}
		case schema.Tool:
			if !pending[m.ToolCallID] {
				return invalid
			}
			delete(pending, m.ToolCallID)
		default:
			return invalid
		}
	}
	last := messages[len(messages)-1]
	if len(pending) != 0 || last.Role != schema.User || strings.TrimSpace(last.Content) == "" {
		return invalid
	}
	return nil
}

func (a *Agent) safeError(ctx context.Context, budget *toolBudget, operation string, cause error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if budget.exhausted.Load() {
		return ErrToolBudget
	}
	// 对外仍只返回安全提示；内部保留 SDK 错误供 errors.As 提取状态码，不能丢掉原因。
	// Join 同时保留已有 ErrRunFailed 分类，避免依赖它的调用方和测试失效。
	return define.ServerError(ErrRunFailed.Error(), operation, errors.Join(ErrRunFailed, cause))
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

// Definition is a server-controlled profile, not request or model input.
type Definition struct {
	Name, Description, Instruction string
	ExposeToolResults              bool
}
