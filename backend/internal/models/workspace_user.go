package models

import "time"

type WorkspaceUser struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        int       `json:"userId"`
	WorkspaceID   int       `json:"workspaceId"`
	Role          string    `gorm:"default:admin" json:"role"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	Workspace     Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}
