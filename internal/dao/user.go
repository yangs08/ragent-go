package dao

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var ErrConflict = errors.New("record already exists")

// UserStruct 映射 users 表；字段与迁移文件保持一致，不作为接口响应。
type UserStruct struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Username     string    `gorm:"column:username"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"` // 仅保存密码散列
	CreatedAt    time.Time `gorm:"column:created_at;->"`          // 数据库默认时间，只读映射
	UpdatedAt    time.Time `gorm:"column:updated_at;->"`
}

func (UserStruct) TableName() string { return "users" }

// User 是共享表入口，不保存请求 context 或查询条件。
var User = userDao{}

type userDao struct{ table[UserStruct] }

func (d userDao) FindByUsername(ctx context.Context, username string) (*UserStruct, error) {
	var user UserStruct
	err := d.Conn(ctx).Where("username = ?", username).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
func (d userDao) Create(ctx context.Context, user *UserStruct) error {
	err := d.Conn(ctx).Create(user).Error
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrConflict
	}
	return err
}

func (d userDao) CheckSchema(ctx context.Context) error {
	return d.Conn(ctx).Exec("SELECT id, username, password_hash FROM users LIMIT 0").Error
}
