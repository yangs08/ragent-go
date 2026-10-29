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

func newTestAgent(t *testing.T, handler http.HandlerFunc, options Options) (*OrderAgent, *atomic.Int32) {
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
	agent, err := NewOrderAgent(model, tooling.DemoOrders(), tooling.Principal{TenantID: "acme", UserID: "alice"}, options)
	if err != nil {
		t.Fatal(err)
	}
	return agent, &calls
}
func testOptions() Options { return Options{MaxIterations: 4, MaxToolCalls: 3, Timeout: time.Second} }
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
					var result tooling.OrderResult
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
			if err := agent.Run(context.Background(), "查询订单 "+tt.id, jsonSink(&output)); err != nil {
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
			if events[0].CallID != "call-order" || events[1].CallID != "call-order" || events[1].Result.Code != tt.code || events[2].Content != tt.answer {
				t.Fatalf("events=%+v", events)
			}
		})
	}
}

func TestOrderAgentRequiresToolEvidence(t *testing.T) {
	agent, _ := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) { answerResponse(w, "我已经查过，订单已完成") }, testOptions())
	var output bytes.Buffer
	err := agent.Run(context.Background(), "查订单 A100", jsonSink(&output))
	if !errors.Is(err, ErrNoToolEvidence) || strings.Contains(output.String(), "我已经查过") {
		t.Fatalf("error=%v output=%s", err, output.String())
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
			err := agent.Run(context.Background(), "查订单", jsonSink(io.Discard))
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
			err := agent.Run(ctx, "查订单", jsonSink(io.Discard))
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
	if err := agent.Run(context.Background(), " ", jsonSink(io.Discard)); err == nil {
		t.Fatal("empty question accepted")
	}
	if err := agent.Run(context.Background(), "查订单", nil); err == nil {
		t.Fatal("nil output accepted")
	}
	if err := agent.Run(context.Background(), "查订单", jsonSink(failingOutput{})); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error=%v", err)
	}
	if _, err := NewOrderAgent(nil, tooling.DemoOrders(), tooling.Principal{TenantID: "acme", UserID: "alice"}, testOptions()); err == nil {
		t.Fatal("nil model accepted")
	}
	for _, opts := range []Options{{}, {MaxIterations: 21, MaxToolCalls: 3, Timeout: time.Second}, {MaxIterations: 4, MaxToolCalls: 21, Timeout: time.Second}, {MaxIterations: 4, MaxToolCalls: 3, Timeout: 6 * time.Minute}} {
		if _, err := NewOrderAgent(agent.model, tooling.DemoOrders(), tooling.Principal{TenantID: "acme", UserID: "alice"}, opts); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
	if _, err := NewOrderAgent(agent.model, nil, tooling.Principal{}, testOptions()); err == nil {
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
	err := agent.Run(context.Background(), "查 A100", func(event Event) error {
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
