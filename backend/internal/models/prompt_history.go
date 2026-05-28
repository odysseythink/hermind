package models

import "time"

type PromptHistory struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
	Prompt      string    `gorm:"type:text;not null" json:"prompt"`
	ModifiedBy  *int      `gorm:"index" json:"modifiedBy,omitempty"`
	ModifiedAt  time.Time `gorm:"autoCreateTime" json:"modifiedAt"`
}

// TableName matches the anything-llm prisma model name for parity.
func (PromptHistory) TableName() string { return "prompt_history" }
