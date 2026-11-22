package define

import "regexp"

// UsernamePattern 供 HTTP/CLI 入口检查用户名格式，编译一次后只读使用；必填和长度由 binding 校验。
var UsernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// User contains only safe identity fields, supplied after authentication.
type User struct {
	ID       string `json:"userId"`
	Username string `json:"username"`
}
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,maxbytes=72"`
}
type LoginResult struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}

// CreateUserRequest 是创建账号的业务入参，HTTP 和 CLI 共用相同规则。
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,minbytes=12,maxbytes=72,notblank"`
}
