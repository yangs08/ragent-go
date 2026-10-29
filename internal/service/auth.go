package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"ragent-go/internal/common"
	"ragent-go/internal/config"
)

var ErrCredentials = errors.New("用户名或密码错误")
var ErrUnauthenticated = errors.New("未登录或登录已过期")
var ErrAuthUnavailable = errors.New("认证服务暂不可用")
var ErrUserInput = errors.New("用户名须为 3..64 位字母数字及 _.-；密码须为 12..72 字节且非全空白")
var ErrUsernameExists = errors.New("用户名已存在")
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{2,63}$`)

// Records stay private to this service until another real use requires sharing.
type userRecord struct{ ID, Username, PasswordHash string }

func (userRecord) TableName() string { return "users" }

type LoginResult struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}
type AuthService struct {
	db        *gorm.DB
	cfg       config.AuthConfig
	dummyHash []byte
}

func NewAuthService(ctx context.Context, db *gorm.DB, cfg config.AuthConfig) (*AuthService, error) {
	if db == nil {
		return nil, ErrAuthUnavailable
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()
	for _, query := range []string{"SELECT id, username, password_hash FROM users LIMIT 0", "SELECT token_hash, user_id, expires_at FROM auth_sessions LIMIT 0"} {
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return nil, errors.New("auth schema unavailable; run make migrate")
		}
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	return &AuthService{db: db, cfg: cfg, dummyHash: dummy}, nil
}

// CreateUser is an operator capability, not a public registration endpoint.
func (s *AuthService) CreateUser(ctx context.Context, username, password string) (string, error) {
	if !usernamePattern.MatchString(username) || len(password) < 12 || len(password) > 72 || strings.TrimSpace(password) == "" {
		return "", ErrUserInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", ErrAuthUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	user := userRecord{ID: rand.Text(), Username: username, PasswordHash: string(hash)}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return "", ErrUsernameExists
		}
		return "", ErrAuthUnavailable
	}
	return user.ID, nil
}
func (s *AuthService) Login(ctx context.Context, username, password string) (LoginResult, error) {
	if !usernamePattern.MatchString(username) || len(password) == 0 || len(password) > 72 {
		return LoginResult{}, ErrCredentials
	}
	ctx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	var user userRecord
	err := s.db.WithContext(ctx).Where("username = ?", username).Take(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return LoginResult{}, ErrAuthUnavailable
	}
	hash := []byte(user.PasswordHash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash = s.dummyHash
	}
	mismatch := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if errors.Is(err, gorm.ErrRecordNotFound) || mismatch != nil {
		return LoginResult{}, ErrCredentials
	}
	if err := ctx.Err(); err != nil {
		return LoginResult{}, ErrAuthUnavailable
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return LoginResult{}, ErrAuthUnavailable
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	// Database time is authoritative for session lifetime across application nodes.
	result := s.db.WithContext(ctx).Exec("INSERT INTO auth_sessions (token_hash, user_id, expires_at) VALUES (?, ?, CURRENT_TIMESTAMP + (? * INTERVAL '1 second'))", tokenHash(token), user.ID, s.cfg.SessionTTL.Seconds())
	if result.Error != nil {
		return LoginResult{}, ErrAuthUnavailable
	}
	// Bounded cleanup; expiry checks do not depend on cleanup having run.
	// Failure here must not turn a persisted login into a client-visible retry.
	_ = s.db.WithContext(ctx).Exec("DELETE FROM auth_sessions WHERE token_hash IN (SELECT token_hash FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP LIMIT 100)").Error
	return LoginResult{UserID: user.ID, Token: token}, nil
}
func (s *AuthService) Authenticate(ctx context.Context, token string) (common.User, error) {
	if !validToken(token) {
		return common.User{}, ErrUnauthenticated
	}
	ctx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	var row struct{ ID, Username string }
	result := s.db.WithContext(ctx).Table("auth_sessions AS s").Select("u.id, u.username").Joins("JOIN users AS u ON u.id = s.user_id").Where("s.token_hash = ? AND s.expires_at > CURRENT_TIMESTAMP", tokenHash(token)).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return common.User{}, ErrUnauthenticated
	}
	if result.Error != nil {
		return common.User{}, ErrAuthUnavailable
	}
	return common.User{ID: row.ID, Username: row.Username}, nil
}
func (s *AuthService) Logout(ctx context.Context, token string) error {
	user, err := common.RequireUser(ctx)
	if err != nil || !validToken(token) {
		return ErrUnauthenticated
	}
	ctx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	if err := s.db.WithContext(ctx).Exec("DELETE FROM auth_sessions WHERE token_hash = ? AND user_id = ?", tokenHash(token), user.ID).Error; err != nil {
		return ErrAuthUnavailable
	}
	return nil
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func validToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(raw) == 32
}
