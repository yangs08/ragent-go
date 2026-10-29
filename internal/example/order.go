package example

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"ragent-go/internal/common"
)

type orderInput struct {
	OrderID string `json:"order_id" jsonschema:"description=订单编号，例如 A100"`
	invalid bool
}

func NewOrderTool(store OrderStore) (tool.InvokableTool, error) {
	if store == nil {
		return nil, errors.New("order tool requires an order store")
	}
	return utils.InferTool("get_order", "按订单编号查询当前用户的订单状态，只读操作。", func(ctx context.Context, input orderInput) (OrderResult, error) {
		if err := ctx.Err(); err != nil {
			return OrderResult{}, err
		}
		if input.invalid {
			return OrderResult{Code: "invalid_arguments", Message: "需要合法的 order_id，例如 A100；不接受其他参数"}, nil
		}
		return queryOrder(ctx, store, input.OrderID)
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

// This order capability is a local example, not a production order module.
type Order struct{ ID, UserID, Status string }
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

var orderIDPattern = regexp.MustCompile(`^[A-Z][0-9]{3}$`)

func queryOrder(ctx context.Context, store OrderStore, id string) (OrderResult, error) {
	if err := ctx.Err(); err != nil {
		return OrderResult{}, err
	}
	user, err := common.RequireUser(ctx)
	if err != nil {
		return OrderResult{}, err
	}
	if !orderIDPattern.MatchString(id) {
		return OrderResult{Code: "invalid_arguments", Message: "需要合法的 order_id，例如 A100；不接受其他参数"}, nil
	}
	value, found, err := store.Find(ctx, user.ID, id)
	if ctx.Err() != nil {
		return OrderResult{}, ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return OrderResult{}, err
	}
	if err != nil {
		return OrderResult{Code: "unavailable", Message: "订单查询暂时不可用"}, nil
	}
	if !found || value.UserID != user.ID {
		return OrderResult{Code: "not_found", Message: "订单不存在或无权访问"}, nil
	}
	return OrderResult{Code: "ok", Order: &OrderView{OrderID: value.ID, Status: value.Status}}, nil
}

type demoOrders map[string]Order

func DemoOrders() OrderStore {
	return demoOrders{
		"A100": {ID: "A100", UserID: "alice", Status: "shipped"},
		"A101": {ID: "A101", UserID: "alice", Status: "delivered"},
		"B200": {ID: "B200", UserID: "bob", Status: "pending"},
		"C300": {ID: "C300", UserID: "carol", Status: "delivered"},
	}
}
func (s demoOrders) Find(ctx context.Context, userID, id string) (Order, bool, error) {
	if err := ctx.Err(); err != nil {
		return Order{}, false, err
	}
	value, found := s[id]
	if !found || value.UserID != userID {
		return Order{}, false, nil
	}
	return value, true, nil
}
