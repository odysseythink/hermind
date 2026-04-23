// storage/types.go
package storage

import (
	"encoding/json"
	"time"
)

// StoredMessage is the persistence shape of a single conversation message.
// No session_id — messages belong to the instance.
type StoredMessage struct {
	ID               int64
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

// UsageUpdate holds a usage delta to add to the conversation_state row.
type UsageUpdate struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	CostUSD          float64
}

// SearchOptions controls FTS message search.
type SearchOptions struct {
	Limit int
}

// SearchResult is a single hit from SearchMessages.
type SearchResult struct {
	Message *StoredMessage
	Snippet string
	Rank    float64
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
