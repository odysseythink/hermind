package models

import "time"

type WorkspaceChat struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID   int       `json:"workspaceId"`
	Prompt        string    `json:"prompt"`
	Response      string    `json:"response"`
	Include       bool      `gorm:"default:true" json:"include"`
	UserID        *int      `json:"userId"`
	ThreadID      *int      `json:"threadId"`
	APISessionID  *string   `json:"apiSessionId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	FeedbackScore *bool     `json:"feedbackScore"`
}
