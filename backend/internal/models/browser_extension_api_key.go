package models

import "time"

type BrowserExtensionApiKey struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Key           string    `gorm:"unique" json:"key"`
	UserID        *int      `json:"userId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
