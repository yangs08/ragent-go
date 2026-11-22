package resource

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

// NewModelClient 将项目配置映射到 Eino 的 OpenAI 兼容客户端配置。
// modelcheck 与 agentcheck 共用此入口，返回原生 SDK 客户端，不再包装模型接口。
func NewModelClient(ctx context.Context, cfg define.ModelConfig) (*openai.ChatModel, error) {
	maxTokens := cfg.MaxCompletionTokens
	var tokenLimit *int
	if maxTokens > 0 {
		tokenLimit = &maxTokens
	}
	client, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey, Timeout: cfg.Timeout, MaxCompletionTokens: tokenLimit})
	if err != nil {
		return nil, streamError(ctx, "create model", err)
	}
	return client, nil
}

// StreamModelText 供 modelcheck 消费单次纯文本流，不负责 Agent 的工具调用循环。
// 同步调用 emit 输出文本，输出失败立即结束；函数退出时关闭流。
// 总时限由调用方控制，供应商原始错误只保留在错误链中。
func StreamModelText(ctx context.Context, client einomodel.BaseChatModel, prompt string, emit func(string) error) error {
	if strings.TrimSpace(prompt) == "" || client == nil || emit == nil {
		return errors.New("model, prompt and text consumer are required")
	}
	stream, err := client.Stream(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return streamError(ctx, "start model stream", err)
	}
	defer stream.Close()
	receivedText := false
	for {
		part, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return streamError(ctx, "read model stream", err)
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

// streamError 保留取消语义与供应商错误原因，对外仅返回安全提示。
func streamError(ctx context.Context, stage string, cause error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	// Provider bodies can contain private request details; never expose them by default.
	return define.ServerError(ErrModelProvider.Error(), stage, errors.Join(ErrModelProvider, cause))
}
