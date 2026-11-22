package service

import (
	"context"
	"crypto/rand"

	"golang.org/x/crypto/bcrypt"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/common"
	"ragent-go/internal/dao"
	"ragent-go/internal/define"
)

var Auth = new(authService)

// 仅在启动接收请求前初始化，运行期间只读；不保存用户或请求状态。
type authService struct{ dummyHash []byte }

func (s *authService) Init(ctx context.Context) error {
	s.dummyHash = nil
	if err := dao.User.CheckSchema(ctx); err != nil {
		return define.ServerError("auth schema unavailable; run make migrate", "check auth schema", err)
	}
	if err := dao.Session.CheckSchema(ctx); err != nil {
		return define.ServerError("auth schema unavailable; run make migrate", "check auth schema", err)
	}
	// 账号不存在时仍执行 bcrypt，缩小密码错误与账号不存在路径的耗时差异。
	dummy, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	if err != nil {
		return define.ServerError("认证服务暂不可用", "prepare password verification", err)
	}
	s.dummyHash = dummy
	return nil
}

// Login 接收已通过入口参数校验的入参，处理账号认证与会话创建。
func (s *authService) Login(ctx context.Context, request define.LoginRequest) (define.LoginResult, error) {
	user, err := dao.User.FindByUsername(ctx, request.Username)
	if err != nil {
		return define.LoginResult{}, define.ServerError("认证服务暂不可用", "find login user", err)
	}
	hash := s.dummyHash
	if user != nil {
		hash = []byte(user.PasswordHash)
	}
	mismatch := bcrypt.CompareHashAndPassword(hash, []byte(request.Password))
	if user == nil || mismatch != nil {
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
	if err := dao.Session.Create(ctx, common.TokenHash(token), user.ID, bootstrap.Config.Auth.SessionTTL); err != nil {
		return define.LoginResult{}, define.ServerError("认证服务暂不可用", "create session", err)
	}

	// 每次最多清理 100 条过期会话，限制登录请求附带的清理工作量；认证仍独立检查有效期。
	// 此时新会话已经入库，清理失败只记日志，避免把成功登录报告为失败并诱发重复登录。
	if err := dao.Session.CleanupExpired(ctx); err != nil {
		bootstrap.Logger.WarnContext(ctx, "session cleanup failed", common.FailureAttrs("cleanup expired sessions", err)...)
	}
	return define.LoginResult{UserID: user.ID, Token: token}, nil
}
func (s *authService) Authenticate(ctx context.Context, token string) (define.User, error) {
	if !common.ValidToken(token) {
		return define.User{}, define.ClientError("未登录或登录已过期")
	}
	user, err := dao.Session.FindUser(ctx, common.TokenHash(token))
	if err != nil {
		return define.User{}, define.ServerError("认证服务暂不可用", "authenticate session", err)
	}
	if user == nil {
		return define.User{}, define.ClientError("未登录或登录已过期")
	}
	return *user, nil
}
func (s *authService) Logout(ctx context.Context, token string) error {
	user, err := common.RequireUser(ctx)
	if err != nil || !common.ValidToken(token) {
		return define.ClientError("未登录或登录已过期")
	}
	if err := dao.Session.Delete(ctx, common.TokenHash(token), user.ID); err != nil {
		return define.ServerError("认证服务暂不可用", "delete session", err)
	}
	return nil
}
