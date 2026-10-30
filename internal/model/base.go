package model

import (
	"context"

	"gorm.io/gorm"

	"ragent-go/internal/bootstrap"
)

// Base provides a model-specific entry to GORM without duplicating query methods.
type Base[T any] struct{}

// Model starts a fresh query. Inside a transaction, use tx.Model(new(T))
// instead so every operation uses the same transaction.
func (Base[T]) Model(ctx context.Context) *gorm.DB {
	return bootstrap.DB.WithContext(ctx).Model(new(T))
}
