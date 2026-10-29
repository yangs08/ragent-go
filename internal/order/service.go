package order

import (
	"context"
	"errors"
	"ragent-go/internal/identity"
	"regexp"
)

var orderIDPattern = regexp.MustCompile(`^[A-Z][0-9]{3}$`)

type Order struct{ ID, TenantID, UserID, Status string }
type Repository interface {
	Find(context.Context, string, string) (Order, bool, error)
}
type View struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}
type Result struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Order   *View  `json:"order,omitempty"`
}

// Service owns order authorization; all callers use the same rule.
type Service struct{ repository Repository }

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("order repository is required")
	}
	return &Service{repository: repository}, nil
}
func (s *Service) Get(ctx context.Context, principal identity.Principal, id string) (Result, error) {
	if err := principal.Validate(); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !orderIDPattern.MatchString(id) {
		return Result{Code: "invalid_arguments", Message: "需要合法的 order_id，例如 A100；不接受其他参数"}, nil
	}
	value, found, err := s.repository.Find(ctx, principal.TenantID, id)
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Result{}, err
	}
	if err != nil {
		return Result{Code: "unavailable", Message: "订单查询暂时不可用"}, nil
	}
	if !found || value.TenantID != principal.TenantID || value.UserID != principal.UserID {
		return Result{Code: "not_found", Message: "订单不存在或无权访问"}, nil
	}
	return Result{Code: "ok", Order: &View{OrderID: value.ID, Status: value.Status}}, nil
}
