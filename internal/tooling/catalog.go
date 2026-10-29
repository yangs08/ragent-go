package tooling

import (
	"context"
	"errors"
	"github.com/cloudwego/eino/components/tool"
	"ragent-go/internal/identity"
	"ragent-go/internal/order"
)

// Catalog assembles registered capabilities. Each Resolve binds a fresh trusted
// identity. A catalog never stores the current user or mutable execution state.
type Catalog struct{ orders *order.Service }

func NewCatalog(orders *order.Service) (*Catalog, error) {
	if orders == nil {
		return nil, errors.New("order service is required")
	}
	return &Catalog{orders: orders}, nil
}
func (c *Catalog) Resolve(ctx context.Context, p identity.Principal) ([]tool.InvokableTool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	t, err := NewOrderTool(c.orders, p)
	if err != nil {
		return nil, err
	}
	return []tool.InvokableTool{t}, nil
}
