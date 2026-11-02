package common

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
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

// FailureAttrs 将错误转换为 slog 使用的键值对，只提取允许记录的诊断字段。
// 不记录 err.Error()：原始错误可能包含 SQL、凭据或供应商响应正文。
func FailureAttrs(operation string, err error) []any {
	// BusinessError 对外提供安全提示；日志分类需要查看它包装的内部原因。
	var business *define.BusinessError
	if errors.As(err, &business) && business.Cause != nil {
		err = business.Cause
	}
	attrs := []any{"operation", operation, "error_type", fmt.Sprintf("%T", err)}
	// 通过能力接口提取 SQLSTATE，无需依赖具体数据库驱动；errors.As 可穿透错误包装。
	// SQLSTATE 是数据库错误类别码，不是 SQL 语句或查询参数。
	var sql interface{ SQLState() string }
	if errors.As(err, &sql) {
		attrs = append(attrs, "sqlstate", sql.SQLState())
	}
	// 用 errors.Is 识别包装后的取消/超时，避免依赖可能变化的错误文本。
	if errors.Is(err, context.Canceled) {
		attrs = append(attrs, "reason", "canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		attrs = append(attrs, "reason", "deadline_exceeded")
	}
	return attrs
}
