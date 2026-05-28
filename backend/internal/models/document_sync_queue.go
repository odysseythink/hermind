package models

import "time"

type DocumentSyncQueue struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	StaleAfterMs   int       `gorm:"default:604800000" json:"staleAfterMs"`
	NextSyncAt     time.Time `json:"nextSyncAt"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"createdAt"`
	LastSyncedAt   time.Time `json:"lastSyncedAt"`
	WorkspaceDocID int       `gorm:"unique" json:"workspaceDocId"`
}
