package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

var ErrProvider = errors.New("model provider request failed")
var ErrEmptyResponse = errors.New("model returned no text")

type Settings struct {
	BaseURL string
	Model   string
	APIKey  string
	Timeout time.Duration
}

func LoadSettings(getenv func(string) string) (Settings, error) {
	cfg := Settings{BaseURL: strings.TrimSpace(getenv("RAGENT_MODEL_BASE_URL")), Model: strings.TrimSpace(getenv("RAGENT_MODEL_NAME")), APIKey: strings.TrimSpace(getenv("RAGENT_MODEL_API_KEY")), Timeout: 20 * time.Second}
	if value := getenv("RAGENT_MODEL_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return Settings{}, errors.New("RAGENT_MODEL_TIMEOUT must be a duration")
		}
		cfg.Timeout = timeout
	}
	if err := cfg.validate(); err != nil {
		return Settings{}, err
	}
	return cfg, nil
}

func (s Settings) validate() error {
	if s.Model == "" {
		return errors.New("RAGENT_MODEL_NAME is required")
	}
	if s.APIKey == "" {
		return errors.New("RAGENT_MODEL_API_KEY is required")
	}
	if s.Timeout <= 0 || s.Timeout > 5*time.Minute {
		return errors.New("RAGENT_MODEL_TIMEOUT must be greater than zero and at most 5m")
	}
	endpoint, err := url.Parse(s.BaseURL)
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("RAGENT_MODEL_BASE_URL must be a base URL without credentials, query or fragment")
	}
	if endpoint.Scheme != "https" {
		ip := net.ParseIP(endpoint.Hostname())
		loopback := endpoint.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
		if endpoint.Scheme != "http" || !loopback {
			return errors.New("RAGENT_MODEL_BASE_URL requires HTTPS except on loopback")
		}
	}
	return nil
}

// NewClient builds the shared Eino model adapter for chat and Agent use.
func NewClient(ctx context.Context, cfg Settings) (*openai.ChatModel, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	maxTokens := 256
	client, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey, MaxCompletionTokens: &maxTokens})
	if err != nil {
		return nil, fmt.Errorf("create model: %w", ErrProvider)
	}
	return client, nil
}

// StreamText exercises the real Eino adapter without implementing retries or routing.
func StreamText(ctx context.Context, cfg Settings, prompt string, output io.Writer) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("model prompt must not be empty")
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return err
	}
	stream, err := client.Stream(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return streamError(ctx, "start model stream")
	}
	defer stream.Close()
	receivedText := false
	for {
		part, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return streamError(ctx, "read model stream")
		}
		if part == nil {
			continue
		}
		if strings.TrimSpace(part.Content) != "" {
			receivedText = true
		}
		n, err := io.WriteString(output, part.Content)
		if err != nil {
			return fmt.Errorf("write model output: %w", err)
		}
		if n != len(part.Content) {
			return io.ErrShortWrite
		}
	}
	if !receivedText {
		return ErrEmptyResponse
	}
	return nil
}

func streamError(ctx context.Context, stage string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	// Provider bodies can contain private request details; never expose them by default.
	return fmt.Errorf("%s: %w", stage, ErrProvider)
}
