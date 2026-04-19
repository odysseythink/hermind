// Package api serves the hermind web UI and REST API. Shapes match the
// existing Python React frontend so it can be pointed at the Go server
// without changes.
//
// Layout is one file per resource:
//
//	server.go           — http.Server wiring + chi router
//	auth.go             — Bearer token middleware
//	dto.go              — request/response DTOs
//	handlers_meta.go    — /api/status, /api/model/info
//	handlers_config.go  — /api/config GET/PUT
//	handlers_sessions.go — /api/sessions, /api/sessions/{id}, /messages
//	handlers_messages.go — message-centric read endpoints
//	handlers_tools.go    — /api/tools (stub)
//	handlers_skills.go   — /api/skills (stub)
//	handlers_providers.go — /api/providers (stub)
//	stream_hook.go       — streaming hook interface for the WebSocket agent
package api

// StatusResponse is the payload for GET /api/status.
type StatusResponse struct {
	Version       string `json:"version"`
	UptimeSec     int64  `json:"uptime_sec"`
	StorageDriver string `json:"storage_driver"`
}

// ModelInfoResponse is the payload for GET /api/model/info.
type ModelInfoResponse struct {
	Model           string `json:"model"`
	ContextLength   int    `json:"context_length"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	SupportsTools   bool   `json:"supports_tools"`
	SupportsVision  bool   `json:"supports_vision"`
}

// SessionDTO is one session row in the list endpoint.
type SessionDTO struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Model        string  `json:"model"`
	StartedAt    float64 `json:"started_at"`
	EndedAt      float64 `json:"ended_at"`
	MessageCount int     `json:"message_count"`
	Title        string  `json:"title"`
}

// SessionListResponse is the payload for GET /api/sessions.
type SessionListResponse struct {
	Sessions []SessionDTO `json:"sessions"`
	Total    int          `json:"total"`
}

// MessageDTO is one message in the messages endpoint.
type MessageDTO struct {
	ID         int64   `json:"id"`
	Role       string  `json:"role"`
	Content    string  `json:"content"`
	ToolCalls  string  `json:"tool_calls,omitempty"` // raw JSON string
	Timestamp  float64 `json:"timestamp"`
	TokenCount int     `json:"token_count,omitempty"`
}

// MessagesResponse is the payload for GET /api/sessions/{id}/messages.
type MessagesResponse struct {
	Messages []MessageDTO `json:"messages"`
	Total    int          `json:"total"`
}

// ConfigResponse is the payload for GET /api/config.
type ConfigResponse struct {
	Config map[string]any `json:"config"`
}

// OKResponse is the success payload used by PUT/DELETE endpoints.
type OKResponse struct {
	OK bool `json:"ok"`
}

// ToolDTO describes a single tool exposed by /api/tools.
type ToolDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ToolsResponse is the payload for GET /api/tools.
type ToolsResponse struct {
	Tools []ToolDTO `json:"tools"`
}

// SkillDTO describes a single skill exposed by /api/skills.
type SkillDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// SkillsResponse is the payload for GET /api/skills.
type SkillsResponse struct {
	Skills []SkillDTO `json:"skills"`
}

// ProviderDTO describes a configured provider.
type ProviderDTO struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}

// ProvidersResponse is the payload for GET /api/providers.
type ProvidersResponse struct {
	Providers []ProviderDTO `json:"providers"`
}
