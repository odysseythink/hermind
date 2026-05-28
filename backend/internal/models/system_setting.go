package models

import "time"

type SystemSetting struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Key           string    `gorm:"unique" json:"label"`
	Value         *string   `json:"value"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
