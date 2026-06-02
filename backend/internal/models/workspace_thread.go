package models

import "time"

type WorkspaceThread struct {
	ID             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string     `json:"name"`
	Slug           string     `gorm:"unique" json:"slug"`
	WorkspaceID    int        `json:"workspaceId"`
	UserID         *int       `json:"userId"`
	ParentThreadID *int       `json:"parentThreadId"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastUpdatedAt  time.Time  `json:"lastUpdatedAt"`
	Workspace      *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}
