package models

import "time"

type ThreadCompaction struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID   int       `json:"workspaceId"`
	ThreadID      *int      `json:"threadId"`
	Summary       string    `json:"summary"`
	UpToChatID    int       `json:"upToChatId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
