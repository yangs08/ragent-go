package dao

import (
	"context"

	"gorm.io/gorm"

	"ragent-go/internal/bootstrap"
)

// table 只复用连接选择与事务入口，字段由 T 定义，CRUD 继续使用原生 GORM。
// 表入口完全无状态，不持有事务、context 或查询条件。
type table[T any] struct{}
type transactionKey struct{}

// WithTx 将当前事务绑定到局部 context；仅在 Transaction 回调内使用返回值。
// 不修改原 context 或共享 DAO，nil 事务禁止静默回退默认连接。
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	if tx == nil {
		panic("dao.WithTx: nil transaction")
	}
	return context.WithValue(ctx, transactionKey{}, tx)
}

// Conn 每次创建独立模型与查询；事务 context 优先，否则使用进程默认连接。
func (table[T]) Conn(ctx context.Context) *gorm.DB {
	db, ok := ctx.Value(transactionKey{}).(*gorm.DB)
	if !ok {
		db = bootstrap.DB
	}
	return db.Session(&gorm.Session{NewDB: true, Context: ctx}).Model(new(T))
}

// Transaction 统一绑定事务 context；提交、错误回滚和 panic 回滚交给 GORM。
// 回调内所有表必须使用 txCtx，不能将它保存到事务生命周期之外。
func (t table[T]) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return t.Conn(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(WithTx(ctx, tx))
	})
}
