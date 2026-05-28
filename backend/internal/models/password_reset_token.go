package models

import "time"

type PasswordResetToken struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int       `json:"userId"`
	Token     string    `gorm:"unique" json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}
