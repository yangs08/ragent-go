package devdata

import (
	"context"
	"ragent-go/internal/order"
)

type orderKey struct{ tenant, id string }
type orderRepository struct{ orders map[orderKey]order.Order }

// DemoOrders returns private immutable data for teaching, not a real business API.
func DemoOrders() order.Repository {
	orders := []order.Order{
		{ID: "A100", TenantID: "acme", UserID: "alice", Status: "shipped"},
		{ID: "A101", TenantID: "acme", UserID: "alice", Status: "delivered"},
		{ID: "B200", TenantID: "acme", UserID: "bob", Status: "pending"},
		{ID: "C300", TenantID: "other", UserID: "alice", Status: "delivered"},
	}
	indexed := make(map[orderKey]order.Order, len(orders))
	for _, order := range orders {
		indexed[orderKey{order.TenantID, order.ID}] = order
	}
	return &orderRepository{orders: indexed}
}

func (s *orderRepository) Find(ctx context.Context, tenant, id string) (order.Order, bool, error) {
	if err := ctx.Err(); err != nil {
		return order.Order{}, false, err
	}
	order, found := s.orders[orderKey{tenant, id}]
	return order, found, nil
}
