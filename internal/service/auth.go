package service

import (
	"context"
	"crypto/rand"
	"errors"
	"regexp"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/common"
	"ragent-go/internal/define"
	"ragent-go/internal/model"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{2,63}$`)

// Auth is initialized before serving and cleared after all requests finish.
// Request-specific state must never be stored on this shared instance.
var Auth *AuthService

type AuthService struct{ dummyHash []byte }

func NewAuthService(ctx context.Context) (*AuthService, error) {
	db, cfg := bootstrap.DB, bootstrap.Config.Auth
	if db == nil {
		return nil, define.ServerError("认证服务暂不可用", "initialize auth", errors.New("database unavailable"))
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()
	for _, query := range []string{"SELECT id, username, password_hash FROM users LIMIT 0", "SELECT token_hash, user_id, expires_at FROM auth_sessions LIMIT 0"} {
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return nil, define.ServerError("auth schema unavailable; run make migrate", "check auth schema", err)
		}
	}
	// 复用一个占位密码散列：账号不存在时仍执行 bcrypt，缩小与密码错误路径的耗时差异。
	// 这只能降低基于耗时枚举账号的风险，不保证整个登录过程恒定耗时。
	dummy, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	if err != nil {
		return nil, define.ServerError("认证服务暂不可用", "prepare password verification", err)
	}
	return &AuthService{dummyHash: dummy}, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (define.LoginResult, error) {
	if !usernamePattern.MatchString(username) || len(password) == 0 || len(password) > 72 {
		return define.LoginResult{}, define.ClientError("用户名或密码错误")
	}
	ctx, cancel := context.WithTimeout(ctx, bootstrap.Config.Auth.RequestTimeout)
	defer cancel()
	var user model.User
	err := user.Model(ctx).Where("username = ?", username).Take(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return define.LoginResult{}, define.ServerError("认证服务暂不可用", "find login user", err)
	}
	hash := []byte(user.PasswordHash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash = s.dummyHash
	}
	mismatch := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if errors.Is(err, gorm.ErrRecordNotFound) || mismatch != nil {
		return define.LoginResult{}, define.ClientError("用户名或密码错误")
	}
	if err := ctx.Err(); err != nil {
		return define.LoginResult{}, define.ServerError("认证服务暂不可用", "check login deadline", err)
	}
	token, err := common.NewSessionToken()
	if err != nil {
		return define.LoginResult{}, define.ServerError("认证服务暂不可用", "generate session token", err)
	}

	// 有效期的写入和认证判断都使用数据库时钟，避免不同应用节点的时钟偏差。
	// 仅保存随机 token 的摘要；数据库泄漏不会直接暴露可用于登录的原始 token。
	session := model.Session{}
	result := session.Model(ctx).Create(map[string]any{
		"token_hash": common.TokenHash(token), "user_id": user.ID,
		"expires_at": gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 second')", bootstrap.Config.Auth.SessionTTL.Seconds()),
	})
	if result.Error != nil {
		return define.LoginResult{}, define.ServerError("认证服务暂不可用", "create session", result.Error)
	}

	// 每次最多清理 100 条过期会话，限制登录请求附带的清理工作量；认证仍独立检查有效期。
	// 此时新会话已经入库，清理失败只记日志，避免把成功登录报告为失败并诱发重复登录。
	if err := bootstrap.DB.WithContext(ctx).Exec("DELETE FROM auth_sessions WHERE token_hash IN (SELECT token_hash FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP LIMIT 100)").Error; err != nil {
		bootstrap.Logger.WarnContext(ctx, "session cleanup failed", common.FailureAttrs("cleanup expired sessions", err)...)
	}
	return define.LoginResult{UserID: user.ID, Token: token}, nil
}
func (s *AuthService) Authenticate(ctx context.Context, token string) (define.User, error) {
	if !common.ValidToken(token) {
		return define.User{}, define.ClientError("未登录或登录已过期")
	}
	ctx, cancel := context.WithTimeout(ctx, bootstrap.Config.Auth.RequestTimeout)
	defer cancel()
	var row define.User
	result := bootstrap.DB.WithContext(ctx).Table("auth_sessions AS s").Select("u.id, u.username").Joins("JOIN users AS u ON u.id = s.user_id").Where("s.token_hash = ? AND s.expires_at > CURRENT_TIMESTAMP", common.TokenHash(token)).Scan(&row)
	if result.Error == nil && result.RowsAffected == 0 {
		return define.User{}, define.ClientError("未登录或登录已过期")
	}
	if result.Error != nil {
		return define.User{}, define.ServerError("认证服务暂不可用", "authenticate session", result.Error)
	}
	return define.User{ID: row.ID, Username: row.Username}, nil
}
func (s *AuthService) Logout(ctx context.Context, token string) error {
	user, err := common.RequireUser(ctx)
	if err != nil || !common.ValidToken(token) {
		return define.ClientError("未登录或登录已过期")
	}
	ctx, cancel := context.WithTimeout(ctx, bootstrap.Config.Auth.RequestTimeout)
	defer cancel()
	session := model.Session{}
	if err := session.Model(ctx).Where("token_hash = ? AND user_id = ?", common.TokenHash(token), user.ID).Delete(&session).Error; err != nil {
		return define.ServerError("认证服务暂不可用", "delete session", err)
	}
	return nil
}
