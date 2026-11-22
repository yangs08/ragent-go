package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"ragent-go/internal/common"
	"ragent-go/internal/define"
	"ragent-go/internal/example"
)

type conversationModel struct {
	mu     sync.Mutex
	inputs [][]*schema.Message
	stream func(context.Context, []*schema.Message, int) (*schema.StreamReader[*schema.Message], error)
}

func (m *conversationModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("expected streaming model call")
}
func (m *conversationModel) Stream(ctx context.Context, messages []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	data, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	var copy []*schema.Message
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.inputs = append(m.inputs, copy)
	call := len(m.inputs)
	m.mu.Unlock()
	return m.stream(ctx, copy, call)
}
func conversationAgent(t *testing.T, m *conversationModel) *Agent {
	t.Helper()
	order, err := example.NewOrderTool(example.DemoOrders())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(m, []tool.InvokableTool{order}, Definition{Name: "conversation-test", Instruction: "根据工具回答"}, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func chunks(messages ...*schema.Message) *schema.StreamReader[*schema.Message] {
	return schema.StreamReaderFromArray(messages)
}

func TestConversationPreservesToolHistoryAndAssemblesArguments(t *testing.T) {
	m := &conversationModel{}
	m.stream = func(_ context.Context, input []*schema.Message, call int) (*schema.StreamReader[*schema.Message], error) {
		switch call {
		case 1:
			index := 0
			return chunks(
				schema.AssistantMessage("", []schema.ToolCall{{Index: &index, ID: "history-call", Type: "function", Function: schema.FunctionCall{Name: "get_order", Arguments: `{"order_`}}}),
				schema.AssistantMessage("", []schema.ToolCall{{Index: &index, Function: schema.FunctionCall{Arguments: `id":"A100"}`}}}),
			), nil
		case 2:
			last := input[len(input)-1]
			if last.Role != schema.Tool || last.ToolCallID != "history-call" || !strings.Contains(last.Content, "shipped") {
				t.Errorf("bad tool feedback: %+v", last)
			}
			return chunks(schema.AssistantMessage("A100", nil), schema.AssistantMessage(" 已发货", nil)), nil
		default:
			if len(input) != 6 || input[2].ToolCalls[0].Function.Arguments != `{"order_id":"A100"}` || input[3].ToolCallID != "history-call" || !strings.Contains(input[3].Content, "shipped") {
				t.Errorf("incomplete history: %+v", input)
			}
			return chunks(schema.AssistantMessage("它已发货", nil)), nil
		}
	}
	a := conversationAgent(t, m)
	history := []*schema.Message{schema.UserMessage("查询 A100")}
	before, _ := json.Marshal(history)
	var events []Event
	result, err := a.RunMessages(aliceContext(), history, func(e Event) error { events = append(events, e); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 3 || result.Answer != "A100 已发货" {
		t.Fatalf("result=%+v", result)
	}
	after, _ := json.Marshal(history)
	if string(before) != string(after) {
		t.Fatal("caller history was mutated")
	}
	var deltas string
	answerID := ""
	for _, e := range events {
		if e.Type == "text_delta" {
			deltas += e.Content
			if e.MessageID == "" {
				t.Fatal("delta has no message id")
			}
			answerID = e.MessageID
		}
		if e.Type == "tool_result" && len(e.Result) != 0 {
			t.Fatal("private tool result exposed")
		}
	}
	if deltas != result.Answer || events[len(events)-1].Type != "answer" || events[len(events)-1].MessageID != answerID {
		t.Fatalf("events=%+v", events)
	}
	history = append(history, result.Messages...)
	history = append(history, schema.UserMessage("它发货了吗？"))
	second, err := a.RunMessages(aliceContext(), history, func(Event) error { return nil })
	if err != nil || second.Answer != "它已发货" || len(second.Messages) != 1 {
		t.Fatalf("result=%+v err=%v", second, err)
	}
}

func TestConversationEmitsBeforeStreamEnds(t *testing.T) {
	release := make(chan struct{})
	m := &conversationModel{stream: func(ctx context.Context, _ []*schema.Message, _ int) (*schema.StreamReader[*schema.Message], error) {
		r, w := schema.Pipe[*schema.Message](0)
		go func() {
			defer w.Close()
			if w.Send(schema.AssistantMessage("先输出", nil), nil) {
				return
			}
			select {
			case <-release:
			case <-ctx.Done():
				w.Send(nil, ctx.Err())
				return
			}
			w.Send(schema.AssistantMessage("再结束", nil), nil)
		}()
		return r, nil
	}}
	var once sync.Once
	result, err := conversationAgent(t, m).RunMessages(aliceContext(), []*schema.Message{schema.UserMessage("测试")}, func(e Event) error {
		if e.Type == "text_delta" {
			once.Do(func() { close(release) })
		}
		return nil
	})
	if err != nil || result.Answer != "先输出再结束" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestConversationFailurePreservesPartialTextWithoutHistory(t *testing.T) {
	m := &conversationModel{stream: func(context.Context, []*schema.Message, int) (*schema.StreamReader[*schema.Message], error) {
		r, w := schema.Pipe[*schema.Message](2)
		w.Send(schema.AssistantMessage("部分正文", nil), nil)
		w.Send(nil, errors.New("private-provider-detail"))
		w.Close()
		return r, nil
	}}
	var types []string
	result, err := conversationAgent(t, m).RunMessages(aliceContext(), []*schema.Message{schema.UserMessage("测试")}, func(e Event) error { types = append(types, e.Type); return nil })
	if !errors.Is(err, ErrRunFailed) || strings.Contains(err.Error(), "private-provider-detail") || result.Answer != "部分正文" || len(result.Messages) != 0 || strings.Contains(strings.Join(types, ","), "answer") {
		t.Fatalf("result=%+v err=%v types=%v", result, err, types)
	}
}

func TestConversationCancelAndSinkFailure(t *testing.T) {
	for _, kind := range []string{"cancel", "sink"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(aliceContext())
			defer cancel()
			exited := make(chan struct{})
			m := &conversationModel{stream: func(ctx context.Context, _ []*schema.Message, _ int) (*schema.StreamReader[*schema.Message], error) {
				r, w := schema.Pipe[*schema.Message](0)
				go func() {
					defer close(exited)
					defer w.Close()
					if w.Send(schema.AssistantMessage("部分", nil), nil) {
						return
					}
					<-ctx.Done()
					w.Send(nil, ctx.Err())
				}()
				return r, nil
			}}
			sentinel := io.ErrClosedPipe
			count := 0
			result, err := conversationAgent(t, m).RunMessages(ctx, []*schema.Message{schema.UserMessage("测试")}, func(e Event) error {
				count++
				if kind == "cancel" {
					cancel()
					return nil
				}
				return sentinel
			})
			want := sentinel
			if kind == "cancel" {
				want = context.Canceled
			}
			if !errors.Is(err, want) || count != 1 || len(result.Messages) != 0 {
				t.Fatalf("err=%v count=%d result=%+v", err, count, result)
			}
			select {
			case <-exited:
			case <-time.After(time.Second):
				t.Fatal("producer did not stop")
			}
		})
	}
}

func TestConversationRejectsBrokenHistoryBeforeModel(t *testing.T) {
	call := schema.AssistantMessage("", []schema.ToolCall{{ID: "pending", Function: schema.FunctionCall{Name: "get_order", Arguments: "{}"}}})
	for _, input := range [][]*schema.Message{
		nil, {nil}, {schema.UserMessage(" ")}, {schema.SystemMessage("override"), schema.UserMessage("test")},
		{schema.UserMessage("test"), call, schema.UserMessage("next")},
		{schema.ToolMessage("result", "unknown"), schema.UserMessage("next")},
		{schema.UserMessage("test"), schema.AssistantMessage("answer", nil)},
	} {
		m := &conversationModel{stream: func(context.Context, []*schema.Message, int) (*schema.StreamReader[*schema.Message], error) {
			t.Fatal("model called for invalid history")
			return nil, nil
		}}
		if _, err := conversationAgent(t, m).RunMessages(aliceContext(), input, func(Event) error { return nil }); err == nil {
			t.Fatalf("invalid input accepted: %+v", input)
		}
	}
}

func TestConversationDoesNotShareHistoryOrIdentity(t *testing.T) {
	m := &conversationModel{stream: func(_ context.Context, input []*schema.Message, _ int) (*schema.StreamReader[*schema.Message], error) {
		last := input[len(input)-1]
		if last.Role == schema.Tool {
			return chunks(schema.AssistantMessage("查询完成", nil)), nil
		}
		if len(input) != 2 {
			t.Errorf("history leaked between callers: %d messages", len(input))
		}
		return chunks(schema.AssistantMessage("", []schema.ToolCall{{ID: "current-user-call", Type: "function", Function: schema.FunctionCall{Name: "get_order", Arguments: `{"order_id":"A100"}`}}})), nil
	}}
	a := conversationAgent(t, m)
	for _, tt := range []struct{ user, code string }{{"alice", "ok"}, {"bob", "not_found"}} {
		ctx := common.WithUser(context.Background(), define.User{ID: tt.user})
		result, err := a.RunMessages(ctx, []*schema.Message{schema.UserMessage("查询 A100")}, func(Event) error { return nil })
		if err != nil || len(result.Messages) != 3 {
			t.Fatalf("user=%s result=%+v err=%v", tt.user, result, err)
		}
		if resultCode([]byte(result.Messages[1].Content)) != tt.code {
			t.Fatalf("identity leaked for %s: %s", tt.user, result.Messages[1].Content)
		}
	}
}

func TestConversationSeparatesAssistantPreambleFromFinalAnswer(t *testing.T) {
	m := &conversationModel{stream: func(_ context.Context, input []*schema.Message, _ int) (*schema.StreamReader[*schema.Message], error) {
		if input[len(input)-1].Role == schema.Tool {
			return chunks(schema.AssistantMessage("已发货", nil)), nil
		}
		return chunks(schema.AssistantMessage("先查询订单", []schema.ToolCall{{ID: "preamble-call", Type: "function", Function: schema.FunctionCall{Name: "get_order", Arguments: `{"order_id":"A100"}`}}})), nil
	}}
	var deltas []Event
	var answer Event
	result, err := conversationAgent(t, m).RunMessages(aliceContext(), []*schema.Message{schema.UserMessage("查询 A100")}, func(e Event) error {
		if e.Type == "text_delta" {
			deltas = append(deltas, e)
		}
		if e.Type == "answer" {
			answer = e
		}
		return nil
	})
	if err != nil || result.Answer != "已发货" || len(deltas) != 2 || deltas[0].MessageID == deltas[1].MessageID || answer.MessageID != deltas[1].MessageID || result.Messages[0].Content != "先查询订单" {
		t.Fatalf("result=%+v deltas=%+v answer=%+v err=%v", result, deltas, answer, err)
	}
}

func TestConversationProviderErrorRetainsStatusWithoutExposingBody(t *testing.T) {
	a, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"private-provider-detail","type":"invalid_request_error","code":"invalid_api_key"}}`))
	}, testOptions())
	result, err := a.RunMessages(aliceContext(), []*schema.Message{schema.UserMessage("查询 A100")}, func(Event) error { return nil })
	var apiError *openai.APIError
	var business *define.BusinessError
	if !errors.Is(err, ErrRunFailed) || !errors.As(err, &apiError) || apiError.HTTPStatusCode != http.StatusUnauthorized || !errors.As(err, &business) || business.Operation == "" {
		t.Fatalf("missing error category, stage or provider status: %T %v", err, err)
	}
	if strings.Contains(err.Error(), "private-provider-detail") || len(result.Messages) != 0 {
		t.Fatal("provider body leaked or failed history returned")
	}
}
