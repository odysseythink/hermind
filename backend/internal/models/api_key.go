package models

import "time"

type APIKey struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          *string   `json:"name"`
	Secret        *string   `gorm:"unique" json:"secret"`
	CreatedBy     *int      `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
