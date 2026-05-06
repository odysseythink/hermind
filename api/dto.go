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
	InstanceRoot  string `json:"instance_root"`
	CurrentModel  string `json:"current_model"`
}

// ModelInfoResponse is the payload for GET /api/model/info.
type ModelInfoResponse struct {
	Model           string `json:"model"`
	ContextLength   int    `json:"context_length"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	SupportsTools   bool   `json:"supports_tools"`
	SupportsVision  bool   `json:"supports_vision"`
}

// StoredMessageDTO is one message in the history endpoint.
type StoredMessageDTO struct {
	ID           int64   `json:"id"`
	Role         string  `json:"role"`
	Content      string  `json:"content"`
	ToolCallID   string  `json:"tool_call_id,omitempty"`
	ToolName     string  `json:"tool_name,omitempty"`
	Timestamp    float64 `json:"timestamp"`
	FinishReason string  `json:"finish_reason,omitempty"`
	Reasoning    string  `json:"reasoning,omitempty"`
}

// ConversationHistoryResponse is the payload for GET /api/conversation.
type ConversationHistoryResponse struct {
	Messages []StoredMessageDTO `json:"messages"`
}

// ConversationPostRequest is the body of POST /api/conversation/messages.
type ConversationPostRequest struct {
	UserMessage string           `json:"user_message"`
	Model       string           `json:"model,omitempty"`
	ObsidianCtx *ObsidianContext `json:"obsidian_context,omitempty"`
}

// ConversationPostResponse is returned on 202.
type ConversationPostResponse struct {
	Accepted bool `json:"accepted"`
}

// ObsidianContext carries the active vault/note/selection context from the
// Obsidian plugin so the agent can reason about the user's current workspace.
type ObsidianContext struct {
	VaultPath    string `json:"vault_path"`
	CurrentNote  string `json:"current_note,omitempty"`
	SelectedText string `json:"selected_text,omitempty"`
	CursorLine   int    `json:"cursor_line,omitempty"`
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

// ErrorResponse is the generic error payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// PredicateDTO is the JSON shape of descriptor.Predicate.
type PredicateDTO struct {
	Field  string `json:"field"`
	Equals any    `json:"equals,omitempty"`
	In     []any  `json:"in,omitempty"`
}

// DatalistSourceDTO is the JSON shape of descriptor.DatalistSource.
type DatalistSourceDTO struct {
	Section string `json:"section"`
	Field   string `json:"field"`
}

// ConfigFieldDTO is one field of a ConfigSectionDTO.
type ConfigFieldDTO struct {
	Name           string             `json:"name"`
	Label          string             `json:"label"`
	Help           string             `json:"help,omitempty"`
	Kind           string             `json:"kind"`
	Required       bool               `json:"required,omitempty"`
	Default        any                `json:"default,omitempty"`
	Enum           []string           `json:"enum,omitempty"`
	VisibleWhen    *PredicateDTO      `json:"visible_when,omitempty"`
	DatalistSource *DatalistSourceDTO `json:"datalist_source,omitempty"`
}

// ConfigSectionDTO is one section in the config schema response.
type ConfigSectionDTO struct {
	Key             string           `json:"key"`
	Label           string           `json:"label"`
	Summary         string           `json:"summary,omitempty"`
	GroupID         string           `json:"group_id"`
	Shape           string           `json:"shape,omitempty"` // "scalar" for scalar sections; omitted (= "map") for map sections
	Subkey          string           `json:"subkey,omitempty"`
	NoDiscriminator bool             `json:"no_discriminator,omitempty"`
	Fields          []ConfigFieldDTO `json:"fields"`
}

// ConfigSchemaResponse is the payload for GET /api/config/schema.
type ConfigSchemaResponse struct {
	Sections []ConfigSectionDTO `json:"sections"`
}

// EditMessageRequest is the body of PUT /api/conversation/messages/{id}.
type EditMessageRequest struct {
	Content string `json:"content"`
}

// RegenerateResponse is returned on POST /api/conversation/messages/{id}/regenerate.
type RegenerateResponse struct {
	Accepted bool `json:"accepted"`
}
