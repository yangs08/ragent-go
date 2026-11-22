package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"ragent-go/internal/define"
)

// SessionStruct 映射 auth_sessions 表；字段与迁移文件保持一致，不作为接口响应。
type SessionStruct struct {
	TokenHash string    `gorm:"column:token_hash;primaryKey" json:"-"` // 仅保存 token 摘要
	UserID    string    `gorm:"column:user_id"`                        // users.id，删除用户时级联删除
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;->"` // 数据库默认时间，只读映射
}

func (SessionStruct) TableName() string { return "auth_sessions" }

// Session 是共享表入口，不保存请求 context 或查询条件。
var Session = sessionDao{}

type sessionDao struct{ table[SessionStruct] }

func (d sessionDao) CheckSchema(ctx context.Context) error {
	return d.Conn(ctx).Exec("SELECT token_hash, user_id, expires_at FROM auth_sessions LIMIT 0").Error
}

func (d sessionDao) Create(ctx context.Context, tokenHash, userID string, ttl time.Duration) error {
	// 写入和认证都使用数据库时钟，避免应用节点时钟偏差。
	return d.Conn(ctx).Create(map[string]any{
		"token_hash": tokenHash, "user_id": userID,
		"expires_at": gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 second')", ttl.Seconds()),
	}).Error
}
func (d sessionDao) FindUser(ctx context.Context, tokenHash string) (*define.User, error) {
	var user define.User
	result := d.Conn(ctx).Table("auth_sessions AS s").Select("u.id, u.username").Joins("JOIN users AS u ON u.id = s.user_id").Where("s.token_hash = ? AND s.expires_at > CURRENT_TIMESTAMP", tokenHash).Scan(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &user, nil
}
func (d sessionDao) Delete(ctx context.Context, tokenHash, userID string) error {
	return d.Conn(ctx).Where("token_hash = ? AND user_id = ?", tokenHash, userID).Delete(&SessionStruct{}).Error
}
func (d sessionDao) CleanupExpired(ctx context.Context) error {
	return d.Conn(ctx).Exec("DELETE FROM auth_sessions WHERE token_hash IN (SELECT token_hash FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP LIMIT 100)").Error
}
