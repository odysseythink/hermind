package models

import "time"

type RecoveryCode struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int       `json:"userId"`
	CodeHash  string    `json:"codeHash"`
	CreatedAt time.Time `json:"createdAt"`
}
