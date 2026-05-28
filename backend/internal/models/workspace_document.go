package models

import "time"

type WorkspaceDocument struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	DocId         string    `gorm:"unique" json:"docId"`
	Filename      string    `json:"filename"`
	Docpath       string    `json:"docpath"`
	WorkspaceID   int       `json:"workspaceId"`
	Metadata      *string   `json:"metadata"`
	Pinned        *bool     `gorm:"default:false" json:"pinned"`
	Watched       *bool     `gorm:"default:false" json:"watched"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
