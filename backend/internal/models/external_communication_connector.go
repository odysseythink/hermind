package models

import "time"

type ExternalCommunicationConnector struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Type          string    `gorm:"unique" json:"type"`
	Config        string    `json:"config"`
	Active        bool      `gorm:"default:false" json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
