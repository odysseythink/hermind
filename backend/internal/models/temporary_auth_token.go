package models

import "time"

type TemporaryAuthToken struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Token     string    `gorm:"uniqueIndex" json:"token"`
	UserID    int       `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}
