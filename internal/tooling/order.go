package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// Principal is bound by trusted application code, never taken from model arguments.
// Lesson two uses a fixed demo identity; this is not production authentication.
type Principal struct{ TenantID, UserID string }

type Order struct{ ID, TenantID, UserID, Status string }
type OrderStore interface {
	Find(context.Context, string, string) (Order, bool, error)
}
type OrderView struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}
type OrderResult struct {
	Code    string     `json:"code"`
	Message string     `json:"message,omitempty"`
	Order   *OrderView `json:"order,omitempty"`
}
type orderInput struct {
	OrderID string `json:"order_id" jsonschema:"description=订单编号，例如 A100"`
	invalid bool
}

var orderIDPattern = regexp.MustCompile(`^[A-Z][0-9]{3}$`)

func NewOrderTool(store OrderStore, principal Principal) (tool.InvokableTool, error) {
	if store == nil || strings.TrimSpace(principal.TenantID) == "" || strings.TrimSpace(principal.UserID) == "" {
		return nil, errors.New("order tool requires a store and trusted tenant/user identity")
	}
	return utils.InferTool("get_order", "按订单编号查询当前用户的订单状态，只读操作。", func(ctx context.Context, input orderInput) (OrderResult, error) {
		if err := ctx.Err(); err != nil {
			return OrderResult{}, err
		}
		if input.invalid || !orderIDPattern.MatchString(input.OrderID) {
			return OrderResult{Code: "invalid_arguments", Message: "需要合法的 order_id，例如 A100；不接受其他参数"}, nil
		}
		order, found, err := store.Find(ctx, principal.TenantID, input.OrderID)
		if ctx.Err() != nil {
			return OrderResult{}, ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return OrderResult{}, err
		}
		if err != nil {
			return OrderResult{Code: "unavailable", Message: "订单查询暂时不可用"}, nil
		}
		// Missing and unauthorized resources share one public result to avoid enumeration.
		if !found || order.TenantID != principal.TenantID || order.UserID != principal.UserID {
			return OrderResult{Code: "not_found", Message: "订单不存在或无权访问"}, nil
		}
		return OrderResult{Code: "ok", Order: &OrderView{OrderID: order.ID, Status: order.Status}}, nil
	}, utils.WithUnmarshalArguments(decodeOrderInput), utils.WithMarshalOutput(func(_ context.Context, output any) (string, error) {
		data, err := json.Marshal(output)
		return string(data), err
	}))
}

func decodeOrderInput(_ context.Context, arguments string) (any, error) {
	invalid := orderInput{invalid: true}
	if len(arguments) > 4096 {
		return invalid, nil
	}
	decoder := json.NewDecoder(strings.NewReader(arguments))
	decoder.DisallowUnknownFields()
	var input orderInput
	if err := decoder.Decode(&input); err != nil {
		return invalid, nil
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return invalid, nil
	}
	return input, nil
}

type orderKey struct{ tenant, id string }
type demoOrderStore struct{ orders map[orderKey]Order }

// DemoOrders returns private immutable data for teaching, not a real business API.
func DemoOrders() OrderStore {
	orders := []Order{
		{ID: "A100", TenantID: "acme", UserID: "alice", Status: "shipped"},
		{ID: "A101", TenantID: "acme", UserID: "alice", Status: "delivered"},
		{ID: "B200", TenantID: "acme", UserID: "bob", Status: "pending"},
		{ID: "C300", TenantID: "other", UserID: "alice", Status: "delivered"},
	}
	indexed := make(map[orderKey]Order, len(orders))
	for _, order := range orders {
		indexed[orderKey{order.TenantID, order.ID}] = order
	}
	return &demoOrderStore{orders: indexed}
}

func (s *demoOrderStore) Find(ctx context.Context, tenant, id string) (Order, bool, error) {
	if err := ctx.Err(); err != nil {
		return Order{}, false, err
	}
	order, found := s.orders[orderKey{tenant, id}]
	return order, found, nil
}
