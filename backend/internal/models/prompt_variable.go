package models

import "time"

type PromptVariable struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Key         string    `gorm:"unique" json:"key"`
	Value       *string   `json:"value"`
	Description *string   `json:"description"`
	Type        string    `gorm:"default:system" json:"type"`
	UserID      *int      `json:"userId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
