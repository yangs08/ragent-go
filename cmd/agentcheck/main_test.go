package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunOrderCheckConfiguration(t *testing.T) {
	if err := runOrderCheck(context.Background(), func(string) string { return "" }, io.Discard); err == nil {
		t.Fatal("missing config accepted")
	}
	env := map[string]string{"RAGENT_MODEL_BASE_URL": "http://127.0.0.1:12345/v1", "RAGENT_MODEL_NAME": "test", "RAGENT_MODEL_API_KEY": "test", "RAGENT_AGENT_MAX_ITERATIONS": "0"}
	if err := runOrderCheck(context.Background(), func(k string) string { return env[k] }, io.Discard); err == nil {
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
	for _, prompt := range []string{"", "查 A101"} {
		env["RAGENT_AGENT_PROMPT"] = prompt
		var out strings.Builder
		err := runOrderCheck(context.Background(), func(k string) string { return env[k] }, &out)
		if err != nil || !strings.Contains(out.String(), "请提供订单编号") {
			t.Fatalf("error=%v output=%s", err, out.String())
		}
	}
}
