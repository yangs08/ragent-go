package model

// User is the database mapping, never a public response.
type User struct {
	Base[User] `gorm:"-" json:"-"`

	ID           string `gorm:"primaryKey"`
	Username     string
	PasswordHash string `json:"-"`
}

func (User) TableName() string { return "users" }
