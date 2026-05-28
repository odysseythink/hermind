package models

import "time"

type PromptPreset struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Command       string    `json:"command"`
	Prompt        string    `json:"prompt"`
	Description   string    `json:"description"`
	UID           int       `gorm:"default:0" json:"-"`
	UserID        *int      `json:"userId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
