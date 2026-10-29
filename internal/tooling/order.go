package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"ragent-go/internal/identity"
	"ragent-go/internal/order"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type orderInput struct {
	OrderID string `json:"order_id" jsonschema:"description=订单编号，例如 A100"`
	invalid bool
}

func NewOrderTool(service *order.Service, principal identity.Principal) (tool.InvokableTool, error) {
	if service == nil || principal.Validate() != nil {
		return nil, errors.New("order tool requires a store and trusted tenant/user identity")
	}
	return utils.InferTool("get_order", "按订单编号查询当前用户的订单状态，只读操作。", func(ctx context.Context, input orderInput) (order.Result, error) {
		if err := ctx.Err(); err != nil {
			return order.Result{}, err
		}
		if input.invalid {
			return order.Result{Code: "invalid_arguments", Message: "需要合法的 order_id，例如 A100；不接受其他参数"}, nil
		}
		return service.Get(ctx, principal, input.OrderID)
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
