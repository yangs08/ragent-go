package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRunCheckMissingConfig(t *testing.T) {
	setupEnvironment(t, nil)
	if err := runCheck(context.Background(), "", io.Discard); err == nil {
		t.Fatal("expected config error")
	}
}

func TestRunCheckWithLocalProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"local response"}}]}

data: [DONE]

`)
	}))
	defer server.Close()
	env := map[string]string{"RAGENT_MODEL_BASE_URL": server.URL + "/v1", "RAGENT_MODEL_NAME": "test", "RAGENT_MODEL_API_KEY": "test"}
	setupEnvironment(t, env)
	var output bytes.Buffer
	if err := runCheck(context.Background(), "", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "local response") {
		t.Fatalf("output=%s", output.String())
	}

	if err := runCheck(context.Background(), "custom prompt", io.Discard); err != nil {
		t.Fatal(err)
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
