package models

import (
	"time"

	"gorm.io/gorm"
)

// InitFTS5 creates the SQLite FTS5 virtual table for chat history search.
func InitFTS5(db *gorm.DB) error {
	return db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS workspace_chat_fts USING fts5(prompt, response)`).Error
}

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
	FeedbackScore   *bool     `json:"feedbackScore"`
	MemoryProcessed *bool     `gorm:"index" json:"memoryProcessed,omitempty"`
}
