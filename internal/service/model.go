package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"ragent-go/internal/define"
)

var ErrModelProvider = errors.New("model provider request failed")
var ErrEmptyModelResponse = errors.New("model returned no text")

// NewModelClient builds the shared Eino model adapter for chat and Agent use.
func NewModelClient(ctx context.Context, cfg define.ModelConfig) (*openai.ChatModel, error) {
	maxTokens := cfg.MaxCompletionTokens
	var tokenLimit *int
	if maxTokens > 0 {
		tokenLimit = &maxTokens
	}
	client, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey, Timeout: cfg.Timeout, MaxCompletionTokens: tokenLimit})
	if err != nil {
		return nil, fmt.Errorf("create model: %w", ErrModelProvider)
	}
	return client, nil
}

// StreamModelText consumes a configured model and emits text synchronously.
// Callers own overall deadlines and output protocols; provider errors are redacted.
func StreamModelText(ctx context.Context, client einomodel.BaseChatModel, prompt string, emit func(string) error) error {
	if strings.TrimSpace(prompt) == "" || client == nil || emit == nil {
		return errors.New("model, prompt and text consumer are required")
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
		if err := emit(part.Content); err != nil {
			return err
		}
	}
	if !receivedText {
		return ErrEmptyModelResponse
	}
	return nil
}

func streamError(ctx context.Context, stage string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	// Provider bodies can contain private request details; never expose them by default.
	return fmt.Errorf("%s: %w", stage, ErrModelProvider)
}
