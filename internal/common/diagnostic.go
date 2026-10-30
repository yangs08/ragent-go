package common

import (
	"context"
	"errors"
	"fmt"

	"ragent-go/internal/define"
)

// FailureAttrs records allowlisted diagnostics, never raw SQL, tokens or provider bodies.
func FailureAttrs(operation string, err error) []any {
	var business *define.BusinessError
	if errors.As(err, &business) && business.Cause != nil {
		err = business.Cause
	}
	attrs := []any{"operation", operation, "error_type", fmt.Sprintf("%T", err)}
	var sql interface{ SQLState() string }
	if errors.As(err, &sql) {
		attrs = append(attrs, "sqlstate", sql.SQLState())
	}
	if errors.Is(err, context.Canceled) {
		attrs = append(attrs, "reason", "canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		attrs = append(attrs, "reason", "deadline_exceeded")
	}
	return attrs
}
