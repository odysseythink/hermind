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

// Memory lifecycle statuses. Queries default to MemoryStatusActive.
const (
	MemoryStatusActive     = "active"
	MemoryStatusSuperseded = "superseded"
	MemoryStatusArchived   = "archived"
)

// Memory type tags. Persisted in the mem_type column of the memories table.
const (
	MemTypeEpisodic              = "episodic"
	MemTypeSemantic              = "semantic"
	MemTypePreference            = "preference"
	MemTypeProjectState          = "project_state"
	MemTypeProceduralObservation = "procedural_obs"
	MemTypeWorkingSummary        = "working_summary"
)

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
	// MemType classifies the memory: "episodic", "semantic", "preference".
	// Empty string means unclassified (legacy holographic records).
	MemType string `json:"mem_type,omitempty"`
	// Vector is a gob-encoded []float32 embedding. Nil for unembedded memories.
	Vector []byte `json:"vector,omitempty"`
	// Status is the lifecycle state: active, superseded, archived.
	// Empty string is treated as active (legacy rows).
	Status string `json:"status,omitempty"`
	// SupersededBy, when non-empty, is the ID of the memory that replaced
	// this one. Only meaningful when Status == superseded.
	SupersededBy string `json:"superseded_by,omitempty"`
	// ReinforcementCount is incremented each time this memory influenced an assistant reply.
	ReinforcementCount int `json:"reinforcement_count,omitempty"`
	// NeglectCount is incremented each time this memory was injected but did not influence the reply.
	NeglectCount int `json:"neglect_count,omitempty"`
	// LastUsedAt is the UTC time of the most recent reinforcement. Zero when never used.
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	// ReinforcedAtSeq is the skills_generation seq at the time this memory was
	// last reinforced. Used to decay reinforcement signals from stale skill generations.
	ReinforcedAtSeq int64 `json:"reinforced_at_seq,omitempty"`
}

// MemorySearchOptions controls MemorySearch behavior.
type MemorySearchOptions struct {
	UserID      string
	Tags        []string
	Limit       int
	QueryVector []float32
	// IncludeAll disables the default active-only filter so callers can
	// see superseded/archived memories (e.g., for maintenance tooling).
	IncludeAll bool
	// CurrentSkillsSeq is the current skills_generation seq. When non-zero
	// (and GenerationHalfLife > 0), the hybrid ranker decays reinforcement
	// signals on each candidate by 0.5^((CurrentSkillsSeq - reinforced_at_seq) / GenerationHalfLife).
	CurrentSkillsSeq int64
	// GenerationHalfLife is the per-generation half-life used in the decay
	// above. 0 disables the decay.
	GenerationHalfLife int
}

// MemoryEvent is a structured log row surfaced by /api/memory/report.
type MemoryEvent struct {
	ID   int64     `json:"id"`
	TS   time.Time `json:"ts"`
	Kind string    `json:"kind"`
	Data []byte    `json:"data"` // raw JSON bytes
}

// SkillsGeneration is the singleton tracker of the skills-library content
// hash plus a monotonic sequence that bumps each time the hash changes.
// Used by the memory ranker to decay stale reinforcement signals.
type SkillsGeneration struct {
	Hash      string
	Seq       int64
	UpdatedAt time.Time
}

// MemoryStats summarizes the memories table for /api/memory/stats.
type MemoryStats struct {
	Total          int            `json:"total"`
	ByType         map[string]int `json:"by_type"`
	ByStatus       map[string]int `json:"by_status"`
	VectorCoverage float64        `json:"vector_coverage"`
	Reinforcement  struct {
		NeverUsed     int `json:"never_used"`
		UsedOneToFew  int `json:"used_1_to_3"`
		UsedMany      int `json:"used_4_plus"`
		NeglectedOnly int `json:"neglected_only"`
	} `json:"reinforcement_histogram"`
	StorageBytes int64 `json:"storage_bytes"`
}

// MemoryHealth reports schema and FTS health for /api/memory/health.
type MemoryHealth struct {
	SchemaVersion           int    `json:"schema_version"`
	MigrationsPending       bool   `json:"migrations_pending"`
	FTSIntegrity            string `json:"fts_integrity"`
	OrphanMemories          int    `json:"orphan_memories"`
	ConsolidatorLastRunUnix *int64 `json:"consolidator_last_run_unix,omitempty"`
	ConsolidatorLastReport  *ConsolidateReportView `json:"consolidator_last_report,omitempty"`
	CurrentSkillsGeneration *CurrentSkillsGen `json:"current_skills_generation,omitempty"`
	Presence                *PresenceView `json:"presence,omitempty"`
}

// CurrentSkillsGen is the JSON shape for the current skills generation in MemoryHealth.
type CurrentSkillsGen struct {
	Hash      string `json:"hash"`
	Seq       int64  `json:"seq"`
	UpdatedAt int64  `json:"updated_at"`
}

// PresenceView is the JSON shape surfaced via MemoryHealth.
type PresenceView struct {
	Available bool                 `json:"available"`
	Sources   []PresenceSourceView `json:"sources"`
}

// PresenceSourceView is one row inside MemoryHealth.Presence.Sources.
type PresenceSourceView struct {
	Name string `json:"name"`
	Vote string `json:"vote"` // "Unknown" | "Absent" | "Present"
}

// ConsolidateReportView is the JSON shape surfaced via MemoryHealth.
type ConsolidateReportView struct {
	Scanned    int `json:"scanned"`
	Superseded int `json:"superseded"`
	Archived   int `json:"archived"`
}

// SkillsStats summarizes the skills directory for /api/skills/stats.
type SkillsStats struct {
	Total       int            `json:"total"`
	ByCategory  map[string]int `json:"by_category"`
	Recent      []SkillSummary `json:"recent"`
	UnusedCount int            `json:"unused_count"`
}

// SkillSummary is a minimal view of a discovered skill file.
type SkillSummary struct {
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}
