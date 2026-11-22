package common

import (
	"context"
	"strings"

	"ragent-go/internal/define"
)

type userContextKey struct{}

func WithUser(ctx context.Context, user define.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}
func RequireUser(ctx context.Context) (define.User, error) {
	user, ok := ctx.Value(userContextKey{}).(define.User)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return define.User{}, define.ClientError("未登录或登录已过期")
	}
	return user, nil
}
