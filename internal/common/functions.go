package common

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// NewSessionToken returns a 256-bit random bearer secret, never a user-supplied ID.
func NewSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// TokenHash is for high-entropy session tokens, never for hashing passwords.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func ValidToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(raw) == 32
}
