package agent

import (
	"context"
	"sync/atomic"

	"github.com/cloudwego/eino/components/tool"
)

type toolBudget struct {
	limit     int32
	calls     atomic.Int32
	exhausted atomic.Bool
}
type budgetedTool struct {
	tool.InvokableTool
	budget *toolBudget
}

func (t *budgetedTool) InvokableRun(ctx context.Context, args string, options ...tool.Option) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t.budget.calls.Add(1) > t.budget.limit {
		t.budget.exhausted.Store(true)
		return "", ErrToolBudget
	}
	return t.InvokableTool.InvokableRun(ctx, args, options...)
}
