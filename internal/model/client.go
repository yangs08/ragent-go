package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"ragent-go/internal/config"
)

var ErrProvider = errors.New("model provider request failed")
var ErrEmptyResponse = errors.New("model returned no text")

// NewClient builds the shared Eino model adapter for chat and Agent use.
func NewClient(ctx context.Context, cfg config.ModelConfig) (*openai.ChatModel, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	maxTokens := 256
	client, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey, Timeout: cfg.Timeout, MaxCompletionTokens: &maxTokens})
	if err != nil {
		return nil, fmt.Errorf("create model: %w", ErrProvider)
	}
	return client, nil
}

// StreamText exercises the real Eino adapter without implementing retries or routing.
func StreamText(ctx context.Context, cfg config.ModelConfig, prompt string, output io.Writer) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("model prompt must not be empty")
	}
	if err := cfg.Validate(); err != nil {
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
