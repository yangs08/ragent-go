package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCheckMissingConfig(t *testing.T) {
	if err := runCheck(context.Background(), func(string) string { return "" }, io.Discard); err == nil {
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
	var output bytes.Buffer
	if err := runCheck(context.Background(), func(k string) string { return env[k] }, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "local response") {
		t.Fatalf("output=%s", output.String())
	}
	env["RAGENT_MODEL_PROMPT"] = "custom prompt"
	if err := runCheck(context.Background(), func(k string) string { return env[k] }, io.Discard); err != nil {
		t.Fatal(err)
	}
}
