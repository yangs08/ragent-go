package bootstrap

import (
	"context"
	"ragent-go/internal/agent"
	"ragent-go/internal/config"
	"ragent-go/internal/devdata"
	"ragent-go/internal/model"
	"ragent-go/internal/order"
	"ragent-go/internal/tooling"
)

// NewDevelopmentAgent explicitly wires the local order adapter. It must not be
// used as a production business integration or authentication mechanism.
func NewDevelopmentAgent(ctx context.Context, cfg config.Config) (*agent.Executor, error) {
	client, err := model.NewClient(ctx, cfg.Model)
	if err != nil {
		return nil, err
	}
	service, err := order.NewService(devdata.DemoOrders())
	if err != nil {
		return nil, err
	}
	catalog, err := tooling.NewCatalog(service)
	if err != nil {
		return nil, err
	}
	definition := agent.Definition{Name: "order_assistant", Description: "只读订单状态查询助手", Instruction: "你是只读订单助手。回答具体订单状态前必须调用 get_order，只依据工具结果回答。订单不明时先追问。查询失败时明确说明，禁止声称已执行写操作。", ExposeToolResults: true}
	return agent.NewExecutor(client, catalog, definition, cfg.Agent)
}
