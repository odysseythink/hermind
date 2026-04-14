// storage/types.go
package storage

import (
	"encoding/json"
	"time"
)

// Session represents a conversation session persisted to storage.
// Fields mirror the Python hermes state.db schema for compatibility.
type Session struct {
	ID              string
	Source          string // "cli", "telegram", "discord", ...
	UserID          string
	Model           string
	ModelConfig     json.RawMessage
	SystemPrompt    string
	ParentSessionID string // non-empty for compression chains
	StartedAt       time.Time
	EndedAt         *time.Time
	EndReason       string
	MessageCount    int
	ToolCallCount   int
	Usage           SessionUsage
	BillingProvider string
	BillingBaseURL  string
	EstimatedCost   float64
	ActualCost      float64
	CostStatus      string
	Title           string
}

// SessionUsage tracks aggregate token counts for a session.
type SessionUsage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
}

// StoredMessage is the persistence shape of a single conversation message.
type StoredMessage struct {
	ID               int64
	SessionID        string
	Role             string
	Content          string // JSON-encoded message.Content
	ToolCallID       string
	ToolCalls        json.RawMessage
	ToolName         string
	Timestamp        time.Time
	TokenCount       int
	FinishReason     string
	Reasoning        string
	ReasoningDetails string
}

// SessionUpdate holds partial fields for UpdateSession.
type SessionUpdate struct {
	EndedAt      *time.Time
	EndReason    string
	Title        string
	MessageCount *int
}

// UsageUpdate holds a usage delta to add to a session.
type UsageUpdate struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	CostUSD          float64
}

// ListOptions controls pagination and filtering for ListSessions.
type ListOptions struct {
	Source string
	UserID string
	Limit  int
	Before time.Time
}

// SearchOptions controls FTS message search.
type SearchOptions struct {
	SessionID string
	Limit     int
}

// SearchResult is a single hit from SearchMessages.
type SearchResult struct {
	Message   *StoredMessage
	SessionID string
	Snippet   string
	Rank      float64
}

// Memory is a persisted agent memory entry.
type Memory struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id,omitempty"`
	Content   string          `json:"content"`
	Category  string          `json:"category,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// MemorySearchOptions controls MemorySearch behavior.
type MemorySearchOptions struct {
	UserID string
	Tags   []string
	Limit  int
}
