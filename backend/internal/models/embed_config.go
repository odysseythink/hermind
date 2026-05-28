package models

import "time"

type EmbedConfig struct {
	ID                       int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UUID                     string    `gorm:"uniqueIndex;not null" json:"uuid"`
	Enabled                  bool      `gorm:"default:false" json:"enabled"`
	ChatMode                 string    `gorm:"default:query" json:"chatMode"`
	AllowlistDomains         *string   `json:"allowlistDomains"`
	AllowModelOverride       bool      `gorm:"default:false" json:"allowModelOverride"`
	AllowTemperatureOverride bool      `gorm:"default:false" json:"allowTemperatureOverride"`
	AllowPromptOverride      bool      `gorm:"default:false" json:"allowPromptOverride"`
	MaxChatsPerDay           *int      `json:"maxChatsPerDay"`
	MaxChatsPerSession       *int      `json:"maxChatsPerSession"`
	MessageLimit             *int      `gorm:"default:20" json:"messageLimit"`
	WorkspaceID              int       `json:"workspaceId"`
	Workspace                Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	CreatedBy                *int      `json:"createdBy"`
	CreatedAt                time.Time `json:"createdAt"`
	LastUpdatedAt            time.Time `json:"lastUpdatedAt"`
}
