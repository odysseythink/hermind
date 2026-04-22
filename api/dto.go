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
	SystemPrompt string  `json:"system_prompt"`
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

// MessageSubmitRequest is the body of POST /api/sessions/{id}/messages.
type MessageSubmitRequest struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
}

// MessageSubmitResponse is returned on 202.
type MessageSubmitResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
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

// SchemaFieldDTO describes one field of a platform descriptor.
type SchemaFieldDTO struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Kind     string   `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Default  any      `json:"default,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// SchemaDescriptorDTO is one descriptor in the schema response.
type SchemaDescriptorDTO struct {
	Type        string           `json:"type"`
	DisplayName string           `json:"display_name"`
	Summary     string           `json:"summary,omitempty"`
	Fields      []SchemaFieldDTO `json:"fields"`
}

// PlatformsSchemaResponse is the payload for GET /api/platforms/schema.
type PlatformsSchemaResponse struct {
	Descriptors []SchemaDescriptorDTO `json:"descriptors"`
}

// RevealRequest is the body of POST /api/platforms/{key}/reveal.
type RevealRequest struct {
	Field string `json:"field"`
}

// RevealResponse is the success payload for reveal.
type RevealResponse struct {
	Value string `json:"value"`
}

// ErrorResponse is the generic error payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// PlatformTestResponse is the payload for POST /api/platforms/{key}/test.
// ok=true on success; on failure, both ok=false and error are set.
type PlatformTestResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ApplyResult is the payload for POST /api/platforms/apply and also
// the return type of the GatewayController.Apply method. Shared so
// the controller can hand a value straight to the handler without an
// intermediate mapping.
type ApplyResult struct {
	OK        bool              `json:"ok"`
	Restarted []string          `json:"restarted,omitempty"`
	Errors    map[string]string `json:"errors,omitempty"`
	TookMS    int64             `json:"took_ms"`
	Error     string            `json:"error,omitempty"` // only on ok=false
}

// PredicateDTO is the JSON shape of descriptor.Predicate.
type PredicateDTO struct {
	Field  string `json:"field"`
	Equals any    `json:"equals"`
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
