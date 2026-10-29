package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"ragent-go/internal/config"
	"ragent-go/internal/devdata"
	"ragent-go/internal/identity"
	"ragent-go/internal/order"
	"ragent-go/internal/tooling"
)

type providerRequest struct {
	Model    string            `json:"model"`
	Stream   bool              `json:"stream"`
	Tools    []json.RawMessage `json:"tools"`
	Messages []struct {
		Role, Content string
		ToolCallID    string `json:"tool_call_id"`
	} `json:"messages"`
}

func newTestAgent(t *testing.T, handler http.HandlerFunc, options config.AgentConfig) (*Executor, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	model, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{BaseURL: server.URL + "/v1", Model: "test", APIKey: "test"})
	if err != nil {
		t.Fatal(err)
	}
	service, _ := order.NewService(devdata.DemoOrders())
	catalog, _ := tooling.NewCatalog(service)
	agent, err := NewExecutor(model, catalog, Definition{Name: "test", Instruction: "使用工具查询订单", ExposeToolResults: true}, options)
	if err != nil {
		t.Fatal(err)
	}
	return agent, &calls
}
func testOptions() config.AgentConfig {
	return config.AgentConfig{MaxIterations: 4, MaxToolCalls: 3, Timeout: time.Second}
}
func toolResponse(w http.ResponseWriter, args string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": "test-response", "object": "chat.completion", "choices": []any{map[string]any{"index": 0, "finish_reason": "tool_calls", "message": map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "call-order", "type": "function", "function": map[string]any{"name": "get_order", "arguments": args}}}}}}})
}
func answerResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": "test-response", "object": "chat.completion", "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": text}}}})
}

func TestOrderAgentActualADKLoop(t *testing.T) {
	for _, tt := range []struct{ id, code, answer string }{{"A100", "ok", "A100 已发货"}, {"B200", "not_found", "订单不存在或无权访问"}, {"Z999", "not_found", "订单不存在或无权访问"}} {
		t.Run(tt.id, func(t *testing.T) {
			agent, calls := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
				var req providerRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				if req.Stream || len(req.Tools) != 1 || !bytes.Contains(req.Tools[0], []byte("get_order")) {
					t.Errorf("missing tool definition: %+v", req)
				}
				last := req.Messages[len(req.Messages)-1]
				if last.Role == "tool" {
					if last.ToolCallID != "call-order" {
						t.Errorf("call id=%s", last.ToolCallID)
					}
					var result order.Result
					if err := json.Unmarshal([]byte(last.Content), &result); err != nil {
						t.Error(err)
					}
					if result.Code != tt.code {
						t.Errorf("code=%s", result.Code)
					}
					answerResponse(w, tt.answer)
					return
				}
				toolResponse(w, fmt.Sprintf(`{"order_id":%q}`, tt.id))
			}, testOptions())
			var output bytes.Buffer
			if err := agent.Run(context.Background(), testRequest("查询订单 "+tt.id), jsonSink(&output)); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 {
				t.Fatalf("model calls=%d", calls.Load())
			}
			decoder := json.NewDecoder(&output)
			var events []Event
			for {
				var event Event
				err := decoder.Decode(&event)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				events = append(events, event)
			}
			if len(events) != 3 || events[0].Type != "tool_call" || events[1].Type != "tool_result" || events[2].Type != "answer" {
				t.Fatalf("events=%+v", events)
			}
			if events[0].CallID != "call-order" || events[1].CallID != "call-order" || resultCode(events[1].Result) != tt.code || events[2].Content != tt.answer {
				t.Fatalf("events=%+v", events)
			}
		})
	}
}

func TestExecutorAllowsClarificationWithoutTool(t *testing.T) {
	agent, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) { answerResponse(w, "请提供订单编号") }, testOptions())
	var events []Event
	if err := agent.Run(context.Background(), testRequest("帮我查订单"), func(e Event) error { events = append(events, e); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "answer" || events[0].Content != "请提供订单编号" {
		t.Fatalf("events=%+v", events)
	}
}

func TestOrderAgentLimitsAndSafeProviderErrors(t *testing.T) {
	for _, kind := range []string{"iterations", "tool budget", "provider", "empty answer", "unknown tool"} {
		t.Run(kind, func(t *testing.T) {
			opts := testOptions()
			if kind == "iterations" {
				opts.MaxIterations = 2
			}
			if kind == "tool budget" {
				opts.MaxToolCalls = 1
			}
			agent, calls := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
				var req providerRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				switch kind {
				case "provider":
					w.WriteHeader(500)
					_, _ = io.WriteString(w, `{"error":{"message":"private-provider-detail"}}`)
				case "empty answer":
					answerResponse(w, "")
				case "unknown tool":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"x","type":"function","function":{"name":"delete_order","arguments":"{}"}}]}}]}`)
				default:
					toolResponse(w, `{"order_id":"A100"}`)
				}
			}, opts)
			err := agent.Run(context.Background(), testRequest("查订单"), jsonSink(io.Discard))
			if err == nil || strings.Contains(err.Error(), "private-provider-detail") {
				t.Fatalf("error=%v", err)
			}
			if kind == "tool budget" && !errors.Is(err, ErrToolBudget) {
				t.Fatalf("error=%v", err)
			}
			if kind == "empty answer" && !errors.Is(err, ErrNoAnswer) {
				t.Fatalf("error=%v", err)
			}
			if kind == "iterations" && calls.Load() > 2 {
				t.Fatalf("model calls=%d", calls.Load())
			}
		})
	}
}

func TestOrderAgentCancellationAndDeadline(t *testing.T) {
	for _, kind := range []string{"cancel", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			opts := testOptions()
			opts.Timeout = 100 * time.Millisecond
			agent, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }, opts)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			err := agent.Run(ctx, testRequest("查订单"), jsonSink(io.Discard))
			want := context.DeadlineExceeded
			if kind == "cancel" {
				want = context.Canceled
			}
			if !errors.Is(err, want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

type failingOutput struct{}

func (failingOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestOrderAgentInputAndOutputErrors(t *testing.T) {
	agent, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) { toolResponse(w, `{"order_id":"A100"}`) }, testOptions())
	if err := agent.Run(context.Background(), testRequest(" "), jsonSink(io.Discard)); err == nil {
		t.Fatal("empty question accepted")
	}
	if err := agent.Run(context.Background(), testRequest("查订单"), nil); err == nil {
		t.Fatal("nil output accepted")
	}
	if err := agent.Run(context.Background(), testRequest("查订单"), jsonSink(failingOutput{})); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error=%v", err)
	}
	if _, err := NewExecutor(nil, agent.catalog, agent.definition, testOptions()); err == nil {
		t.Fatal("nil model accepted")
	}
	for _, opts := range []config.AgentConfig{{}, {MaxIterations: 21, MaxToolCalls: 3, Timeout: time.Second}, {MaxIterations: 4, MaxToolCalls: 21, Timeout: time.Second}, {MaxIterations: 4, MaxToolCalls: 3, Timeout: 6 * time.Minute}} {
		if _, err := NewExecutor(agent.model, agent.catalog, agent.definition, opts); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
	if _, err := NewExecutor(agent.model, nil, agent.definition, testOptions()); err == nil {
		t.Fatal("invalid tool dependencies accepted")
	}
}

// Serialization belongs to the consumer, including consumers used in integration tests.
func jsonSink(output io.Writer) EventSink {
	encoder := json.NewEncoder(output)
	return func(event Event) error { return encoder.Encode(event) }
}

func TestOrderAgentSinkFailureStopsExecution(t *testing.T) {
	agent, calls := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		toolResponse(w, `{"order_id":"A100"}`)
	}, testOptions())
	sentinel := errors.New("consumer stopped")
	count := 0
	err := agent.Run(context.Background(), testRequest("查 A100"), func(event Event) error {
		count++
		if event.Type != "tool_call" {
			t.Errorf("first event=%+v", event)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) || count != 1 || calls.Load() != 1 {
		t.Fatalf("error=%v events=%d model calls=%d", err, count, calls.Load())
	}
}

func TestOrderAgentConcurrentRunsHaveIndependentBudgets(t *testing.T) {
	opts := testOptions()
	opts.MaxToolCalls = 1
	agent, calls := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		var req providerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			answerResponse(w, "已查询")
			return
		}
		toolResponse(w, `{"order_id":"A100"}`)
	}, opts)
	const runs = 4
	results := make(chan error, runs)
	for i := 0; i < runs; i++ {
		go func() {
			var types []string
			err := agent.Run(context.Background(), testRequest("查 A100"), func(event Event) error {
				types = append(types, event.Type)
				return nil
			})
			if err == nil && strings.Join(types, ",") != "tool_call,tool_result,answer" {
				err = fmt.Errorf("events=%v", types)
			}
			results <- err
		}()
	}
	for i := 0; i < runs; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != runs*2 {
		t.Fatalf("model calls=%d", calls.Load())
	}
}

func testRequest(question string) Request {
	return Request{Question: question, Principal: identity.Principal{TenantID: "acme", UserID: "alice"}}
}
func resultCode(data json.RawMessage) string {
	var result order.Result
	_ = json.Unmarshal(data, &result)
	return result.Code
}

func TestExecutorBindsIdentityPerRun(t *testing.T) {
	executor, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		var req providerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			answerResponse(w, "查询完成")
			return
		}
		toolResponse(w, `{"order_id":"A100"}`)
	}, testOptions())
	for _, tt := range []struct{ user, code string }{{"alice", "ok"}, {"bob", "not_found"}, {"alice", "ok"}} {
		req := testRequest("查询 A100")
		req.Principal.UserID = tt.user
		code := ""
		err := executor.Run(context.Background(), req, func(e Event) error {
			if e.Type == "tool_result" {
				code = resultCode(e.Result)
			}
			return nil
		})
		if err != nil || code != tt.code {
			t.Fatalf("user=%s code=%s error=%v", tt.user, code, err)
		}
	}
	req := testRequest("查询 A100")
	req.Principal = identity.Principal{}
	if err := executor.Run(context.Background(), req, func(Event) error { return nil }); err == nil {
		t.Fatal("missing identity accepted")
	}
}

type resolveTools func(context.Context, identity.Principal) ([]tool.InvokableTool, error)

func (f resolveTools) Resolve(ctx context.Context, p identity.Principal) ([]tool.InvokableTool, error) {
	return f(ctx, p)
}
func TestExecutorSupportsOtherToolsWithoutExposingResultsByDefault(t *testing.T) {
	executor, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		var req providerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			answerResponse(w, "晴")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"weather-call","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]}}]}`)
	}, testOptions())
	weather, err := utils.InferTool("lookup_weather", "查询天气", func(context.Context, struct{}) (string, error) { return "晴", nil })
	if err != nil {
		t.Fatal(err)
	}
	executor.catalog = resolveTools(func(context.Context, identity.Principal) ([]tool.InvokableTool, error) {
		return []tool.InvokableTool{weather}, nil
	})
	executor.definition.ExposeToolResults = false
	var events []Event
	if err := executor.Run(context.Background(), testRequest("天气"), func(e Event) error { events = append(events, e); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Tool != "lookup_weather" || len(events[1].Result) != 0 || events[2].Content != "晴" {
		t.Fatalf("events=%+v", events)
	}
}
