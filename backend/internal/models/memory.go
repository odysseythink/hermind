package models

import "time"

// Memory scopes.
const (
	MemoryScopeWorkspace = "workspace"
	MemoryScopeGlobal    = "global"
)

// Limits — match anything-llm exactly.
const (
	GlobalMemoryLimit         = 5
	WorkspaceMemoryLimit      = 20
	MaxInjectedWorkspaceLimit = 5
)

type Memory struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      *int       `gorm:"index:idx_mem_user_ws;index:idx_mem_user_scope" json:"userId"`
	WorkspaceID *int       `gorm:"index:idx_mem_user_ws" json:"workspaceId"`
	Scope       string     `gorm:"not null;default:workspace;index:idx_mem_user_scope" json:"scope"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (Memory) TableName() string { return "memories" }
