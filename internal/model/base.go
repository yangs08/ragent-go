package model

import (
	"context"

	"gorm.io/gorm"

	"ragent-go/internal/bootstrap"
)

// Base provides a model-specific entry to GORM without duplicating query methods.
type Base[T any] struct{}

// Model 从全局连接池创建绑定当前模型和 context 的新查询，不复用之前的 Where 条件。
// 事务内必须使用 tx.Model(new(T))；调用本方法会回到全局 DB，导致该操作脱离事务。
func (Base[T]) Model(ctx context.Context) *gorm.DB {
	return bootstrap.DB.WithContext(ctx).Model(new(T))
}
