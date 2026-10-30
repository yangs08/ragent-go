package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
