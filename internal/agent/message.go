package agent

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func consumeMessage(ctx context.Context, output *adk.MessageVariant, onText func(string) error) (*schema.Message, error) {
	if !output.IsStreaming {
		message := output.Message
		if message != nil && message.Role == schema.Assistant && message.Content != "" {
			if err := onText(message.Content); err != nil {
				return nil, err
			}
		}
		return message, nil
	}
	reader := output.MessageStream
	if reader == nil {
		return nil, ErrRunFailed
	}
	defer reader.Close()
	var parts []*schema.Message
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if part == nil {
			continue
		}
		parts = append(parts, part)
		if (output.Role == schema.Assistant || part.Role == schema.Assistant) && part.Content != "" {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := onText(part.Content); err != nil {
				return nil, err
			}
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return schema.ConcatMessages(parts)
}

// 历史来自可信存储，仍检查角色和工具配对，避免坏记录进入供应商协议。
// System 指令由服务端 Definition 管理；当前接口用于文本聊天，不接受客户端自选角色。
func validateMessages(messages []*schema.Message) error {
	invalid := errors.New("agent history must contain paired tool messages and end with a nonempty user question")
	if len(messages) == 0 {
		return invalid
	}
	pending := map[string]bool{}
	for _, m := range messages {
		if m == nil {
			return invalid
		}
		if m.Role != schema.Assistant && len(m.ToolCalls) != 0 {
			return invalid
		}
		if m.Role != schema.Tool && m.ToolCallID != "" {
			return invalid
		}
		switch m.Role {
		case schema.User:
			if len(pending) != 0 {
				return invalid
			}
		case schema.Assistant:
			if len(pending) != 0 {
				return invalid
			}
			for _, call := range m.ToolCalls {
				if call.ID == "" || strings.TrimSpace(call.Function.Name) == "" || pending[call.ID] {
					return invalid
				}
				pending[call.ID] = true
			}
		case schema.Tool:
			if !pending[m.ToolCallID] {
				return invalid
			}
			delete(pending, m.ToolCallID)
		default:
			return invalid
		}
	}
	last := messages[len(messages)-1]
	if len(pending) != 0 || last.Role != schema.User || strings.TrimSpace(last.Content) == "" {
		return invalid
	}
	return nil
}
