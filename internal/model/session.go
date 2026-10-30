package model

import "time"

// Session stores only a digest of the bearer token.
type Session struct {
	Base[Session] `gorm:"-" json:"-"`
	TokenHash     string `gorm:"primaryKey" json:"-"`
	UserID        string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

func (Session) TableName() string { return "auth_sessions" }
