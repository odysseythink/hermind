package dto

import "time"

// Requests

type CreateEmbedConfigRequest struct {
	WorkspaceSlug            string   `json:"workspace_slug" binding:"required"`
	ChatMode                 string   `json:"chat_mode,omitempty"`
	AllowlistDomains         []string `json:"allowlist_domains,omitempty"`
	AllowModelOverride       bool     `json:"allow_model_override"`
	AllowTemperatureOverride bool     `json:"allow_temperature_override"`
	AllowPromptOverride      bool     `json:"allow_prompt_override"`
	MaxChatsPerDay           *int     `json:"max_chats_per_day,omitempty"`
	MaxChatsPerSession       *int     `json:"max_chats_per_session,omitempty"`
	MessageLimit             *int     `json:"message_limit,omitempty"`
}

type UpdateEmbedConfigRequest struct {
	Enabled                  *bool    `json:"enabled,omitempty"`
	ChatMode                 *string  `json:"chat_mode,omitempty"`
	AllowlistDomains         []string `json:"allowlist_domains,omitempty"`
	AllowModelOverride       *bool    `json:"allow_model_override,omitempty"`
	AllowTemperatureOverride *bool    `json:"allow_temperature_override,omitempty"`
	AllowPromptOverride      *bool    `json:"allow_prompt_override,omitempty"`
	MaxChatsPerDay           *int     `json:"max_chats_per_day,omitempty"`
	MaxChatsPerSession       *int     `json:"max_chats_per_session,omitempty"`
	MessageLimit             *int     `json:"message_limit,omitempty"`
	WorkspaceID              *int     `json:"workspace_id,omitempty"`
}

type EmbedStreamChatRequest struct {
	SessionID   string   `json:"sessionId" binding:"required"`
	Message     string   `json:"message" binding:"required"`
	Prompt      *string  `json:"prompt,omitempty"`
	Model       *string  `json:"model,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	Username    *string  `json:"username,omitempty"`
}

type ListEmbedChatsRequest struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// Responses

type EmbedConfigResponse struct {
	ID        int              `json:"id"`
	UUID      string           `json:"uuid"`
	Enabled   bool             `json:"enabled"`
	ChatMode  string           `json:"chatMode"`
	Workspace WorkspaceSummary `json:"workspace"`
	ChatCount int64            `json:"chatCount"`
	CreatedAt time.Time        `json:"createdAt"`
}

type WorkspaceSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type EmbedConfigListResponse struct {
	Embeds []EmbedConfigResponse `json:"embeds"`
}

type EmbedChatHistoryItem struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	SentAt  time.Time `json:"sentAt"`
	Sources []any     `json:"sources,omitempty"`
}

type EmbedHistoryResponse struct {
	History []EmbedChatHistoryItem `json:"history"`
}

type EmbedChatAdminItem struct {
	ID          int              `json:"id"`
	Prompt      string           `json:"prompt"`
	Response    string           `json:"response"`
	SessionID   string           `json:"sessionId"`
	EmbedConfig EmbedConfigShort `json:"embed_config"`
	Workspace   WorkspaceSummary `json:"workspace"`
	CreatedAt   time.Time        `json:"createdAt"`
}

type EmbedConfigShort struct {
	ID   int    `json:"id"`
	UUID string `json:"uuid"`
}

type EmbedChatListResponse struct {
	Chats      []EmbedChatAdminItem `json:"chats"`
	HasPages   bool                 `json:"hasPages"`
	TotalChats int64                `json:"totalChats"`
}

// ConnectionMeta is set by middleware for stream-chat
type ConnectionMeta struct {
	Host     string
	IP       string
	Username *string
}
