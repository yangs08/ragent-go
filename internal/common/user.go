package common

import (
	"context"
	"errors"
	"strings"
)

// User is populated by trusted entry-point code, not model arguments.
// The development CLI supplies its user explicitly; login is not implemented.
type User struct{ ID string }
type key struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, key{}, user)
}
func RequireUser(ctx context.Context) (User, error) {
	user, ok := ctx.Value(key{}).(User)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return User{}, errors.New("authenticated user is required")
	}
	return user, nil
}
