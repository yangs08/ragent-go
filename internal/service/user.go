package service

import (
	"context"
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"ragent-go/internal/dao"
	"ragent-go/internal/define"
)

var User = new(userService)

type userService struct{}

// Create 接收已通过入口参数校验的入参；仅供管理员创建账号。
func (s *userService) Create(ctx context.Context, request define.CreateUserRequest) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", define.ServerError("账号服务暂不可用", "hash password", err)
	}
	user := dao.UserStruct{ID: rand.Text(), Username: request.Username, PasswordHash: string(hash)}
	if err := dao.User.Create(ctx, &user); err != nil {
		if errors.Is(err, dao.ErrConflict) {
			return "", define.ClientError("用户名已存在")
		}
		return "", define.ServerError("账号服务暂不可用", "create user", err)
	}
	return user.ID, nil
}
