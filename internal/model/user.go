package model

import "time"

type User struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Email     string    `gorm:"type:varchar(128);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:varchar(128);not null" json:"-"`
	Nickname  string    `gorm:"type:varchar(64);not null;default:''" json:"nickname"`
	AvatarURL string    `gorm:"type:varchar(255);not null;default:''" json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }
