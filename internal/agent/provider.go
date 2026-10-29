package agent

import (
	"context"
	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"ragent-go/internal/config"
	"ragent-go/internal/identity"
)

// Definition is a server-controlled agent profile, never model or HTTP input.
type Definition struct {
	Name, Description, Instruction string
	ExposeToolResults              bool
}
type Request struct {
	Principal identity.Principal
	Question  string
}

// ToolCatalog is the execution-side boundary for tools bound to this run's identity.
type ToolCatalog interface {
	Resolve(context.Context, identity.Principal) ([]tool.InvokableTool, error)
}

func buildEngine(ctx context.Context, model einomodel.BaseChatModel, d Definition, tools []tool.BaseTool, cfg config.AgentConfig) (*adk.ChatModelAgent, error) {
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{Name: d.Name, Description: d.Description, Instruction: d.Instruction, Model: model, MaxIterations: cfg.MaxIterations, ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}}})
}
