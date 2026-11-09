package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"ragent-go/internal/agent"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunOrderCheckConfiguration(t *testing.T) {
	setupEnvironment(t, nil)
	if err := runOrderCheck(context.Background(), "", io.Discard); err == nil {
		t.Fatal("missing config accepted")
	}
	env := map[string]string{"RAGENT_MODEL_BASE_URL": "http://127.0.0.1:12345/v1", "RAGENT_MODEL_NAME": "test", "RAGENT_MODEL_API_KEY": "test", "RAGENT_AGENT_MAX_ITERATIONS": "0"}
	setupEnvironment(t, env)
	if err := runOrderCheck(context.Background(), "", io.Discard); err == nil {
		t.Fatal("invalid agent limits accepted")
	}
}
func TestRunOrderCheckAllowsClarification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": "请提供订单编号"}}}})
	}))
	defer server.Close()
	env := map[string]string{"RAGENT_MODEL_BASE_URL": server.URL + "/v1", "RAGENT_MODEL_NAME": "test", "RAGENT_MODEL_API_KEY": "test"}
	setupEnvironment(t, env)
	for _, prompt := range []string{"", "查 A101"} {
		var out strings.Builder
		err := runOrderCheck(context.Background(), prompt, &out)
		if err != nil || !strings.Contains(out.String(), "请提供订单编号") {
			t.Fatalf("error=%v output=%s", err, out.String())
		}
	}
}

func setupEnvironment(t *testing.T, values map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(key, "RAGENT_") {
			t.Setenv(key, "")
		}
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}

func TestInteractiveAgentUsesFullHistoryAndStreams(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role, Content string
				ToolCallID    string `json:"tool_call_id"`
				ToolCalls     []struct {
					ID       string
					Function struct{ Name, Arguments string }
				} `json:"tool_calls"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if !request.Stream {
			t.Error("interactive request was not streaming")
		}
		call := calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(delta any) {
			data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta}}})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
		if call == 1 {
			send(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "cli-order", "type": "function", "function": map[string]string{"name": "get_order", "arguments": `{"order_`}}}})
			send(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]string{"arguments": `id":"A100"}`}}}})
		} else {
			last := request.Messages[len(request.Messages)-1]
			if call == 2 {
				if last.Role != "tool" || last.ToolCallID != "cli-order" || !strings.Contains(last.Content, "shipped") {
					t.Error("first turn did not execute order tool")
				}
				send(map[string]string{"role": "assistant", "content": "A100"})
				send(map[string]string{"content": " 已发货"})
			} else {
				paired := false
				for _, m := range request.Messages {
					if m.Role == "tool" && m.ToolCallID == "cli-order" && strings.Contains(m.Content, "shipped") {
						paired = true
					}
				}
				if !paired || last.Content != "它发货了吗？" || len(request.Messages) != 6 {
					t.Errorf("follow-up lost history: %+v", request.Messages)
				}
				send(map[string]string{"role": "assistant", "content": "它已发货"})
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	setupEnvironment(t, map[string]string{"RAGENT_MODEL_BASE_URL": server.URL + "/v1", "RAGENT_MODEL_NAME": "test", "RAGENT_MODEL_API_KEY": "test"})
	var out strings.Builder
	err := runOrderSession(context.Background(), "查询 A100", strings.NewReader("\n它发货了吗？\n/exit\n不会执行\n"), &out, true)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("model calls=%d", calls.Load())
	}
	decoder := json.NewDecoder(strings.NewReader(out.String()))
	answers, deltas := 0, 0
	for {
		var event agent.Event
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if event.Type == "answer" {
			answers++
		}
		if event.Type == "text_delta" {
			deltas++
		}
	}
	if answers != 2 || deltas != 3 {
		t.Fatalf("answers=%d deltas=%d output=%s", answers, deltas, out.String())
	}
}
