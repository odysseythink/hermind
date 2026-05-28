package models

import "time"

type WorkspaceAgentInvocation struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"uniqueIndex;not null" json:"uuid"`
	WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
	UserID      *int      `gorm:"index" json:"userId"`
	ThreadID    *int      `gorm:"index" json:"threadId"`
	Prompt      string    `gorm:"type:text;not null" json:"prompt"`
	Closed      bool      `gorm:"index;default:false" json:"closed"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (WorkspaceAgentInvocation) TableName() string { return "workspace_agent_invocations" }
