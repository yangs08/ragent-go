package service

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
	"testing"
	"time"

	"ragent-go/internal/define"
)

func TestStreamModelTextUsesRealEinoAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authentication")
		}
		var body struct {
			Model    string
			Stream   bool
			Messages []struct{ Role, Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if !body.Stream || body.Model != "test-model" || len(body.Messages) != 1 || body.Messages[0].Content != "你好" {
			t.Errorf("body=%+v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, `data: {"id":"one","choices":[{"index":0,"delta":{"role":"assistant","content":"你"}}]}

data: {"id":"one","choices":[{"index":0,"delta":{"content":"好"}}]}

data: [DONE]

`)
	}))
	defer server.Close()
	var output bytes.Buffer
	err := streamConfigured(context.Background(), define.ModelConfig{BaseURL: server.URL + "/v1", Model: "test-model", APIKey: "test-key", Timeout: time.Second}, "你好", &output)
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != "你好" {
		t.Fatalf("output=%q", output.String())
	}
}

func TestStreamModelTextFailureAndCancellation(t *testing.T) {
	for _, kind := range []string{"provider error", "midstream error", "deadline", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "provider error":
					w.WriteHeader(500)
					_, _ = io.WriteString(w, `{"error":{"message":"private-value","type":"server_error"}}`)
				case "midstream error":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"partial"}}]}

data: {"error":{"message":"private-value","type":"server_error"}}

`)
				case "deadline":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"partial"}}]}

`)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				default:
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			cfg := define.ModelConfig{BaseURL: server.URL + "/v1", Model: "test-model", APIKey: "test-key", Timeout: 100 * time.Millisecond}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancelled" {
				cancel()
			}
			var output bytes.Buffer
			err := streamConfigured(ctx, cfg, "hello", &output)
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "private-value") {
				t.Fatal("provider payload leaked")
			}
			switch kind {
			case "deadline":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("error=%v", err)
				}
			case "cancelled":
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error=%v", err)
				}
			default:
				if !errors.Is(err, ErrModelProvider) {
					t.Fatalf("error=%v", err)
				}
			}
			if kind == "midstream error" && output.String() != "partial" {
				t.Fatalf("partial=%q", output.String())
			}
		})
	}
}

func TestStreamModelTextRejectsEmptyInput(t *testing.T) {
	if err := streamConfigured(context.Background(), define.ModelConfig{}, " ", io.Discard); err == nil {
		t.Fatal("expected error")
	}
	cfg := define.ModelConfig{BaseURL: "https://model.example/v1", Model: "x", APIKey: "x", Timeout: time.Second}
	cfg.Timeout = 0
	if err := streamConfigured(context.Background(), cfg, "hello", io.Discard); err == nil {
		t.Fatal("expected settings error")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 0, nil }

func TestStreamModelTextEmptyAndOutputFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		writer  io.Writer
		want    error
		content string
	}{
		{"empty", io.Discard, ErrEmptyModelResponse, ""},
		{"writer error", brokenWriter{}, io.ErrClosedPipe, "hello"},
		{"short write", shortWriter{}, io.ErrShortWrite, "hello"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				chunk := map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": tt.content}}}}
				data, _ := json.Marshal(chunk)
				_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
			}))
			defer server.Close()
			cfg := define.ModelConfig{BaseURL: server.URL + "/v1", Model: "test", APIKey: "test", Timeout: time.Second}
			err := streamConfigured(context.Background(), cfg, "hello", tt.writer)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewModelClientHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer server.Close()
	client, err := NewModelClient(context.Background(), define.ModelConfig{BaseURL: server.URL + "/v1", Model: "test", APIKey: "test", Timeout: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = client.Generate(ctx, nil)
	if err == nil || ctx.Err() != nil {
		t.Fatalf("configured client timeout not honored before outer deadline: %v", err)
	}
}

func streamConfigured(ctx context.Context, cfg define.ModelConfig, prompt string, output io.Writer) error {
	client, err := NewModelClient(ctx, cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	return StreamModelText(ctx, client, prompt, func(text string) error {
		n, err := io.WriteString(output, text)
		if err != nil {
			return err
		}
		if n != len(text) {
			return io.ErrShortWrite
		}
		return nil
	})
}
