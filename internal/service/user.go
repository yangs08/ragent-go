package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
	"ragent-go/internal/model"
)

type UserService struct{}

// Create is an operator capability, not a public registration endpoint.
func (UserService) Create(ctx context.Context, username, password string) (string, error) {
	if bootstrap.DB == nil {
		return "", define.ServerError("账号服务暂不可用", "create user", errors.New("database unavailable"))
	}
	if !usernamePattern.MatchString(username) || len(password) < 12 || len(password) > 72 || strings.TrimSpace(password) == "" {
		return "", define.ClientError("用户名须为 3..64 位字母数字及 _.-；密码须为 12..72 字节且非全空白")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", define.ServerError("账号服务暂不可用", "hash password", err)
	}
	ctx, cancel := context.WithTimeout(ctx, bootstrap.Config.Auth.RequestTimeout)
	defer cancel()
	user := model.User{ID: rand.Text(), Username: username, PasswordHash: string(hash)}
	if err := user.Model(ctx).Create(&user).Error; err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return "", define.ClientError("用户名已存在")
		}
		return "", define.ServerError("账号服务暂不可用", "create user", err)
	}
	return user.ID, nil
}
