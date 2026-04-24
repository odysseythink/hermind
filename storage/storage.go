// storage/storage.go
package storage

import (
	"context"
	"errors"
)

// Sentinel errors returned by storage implementations.
var (
	// ErrNotFound is returned when a record does not exist.
	ErrNotFound = errors.New("storage: not found")
)

// Storage is the root storage interface. Implementations must be safe
// for concurrent use. Messages are instance-scoped — there is a single
// implicit conversation per hermind instance.
type Storage interface {
	// Conversation log.
	AppendMessage(ctx context.Context, msg *StoredMessage) error
	GetHistory(ctx context.Context, limit, offset int) ([]*StoredMessage, error)
	SearchMessages(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)

	// Conversation state (singleton row id=1).
	UpdateSystemPromptCache(ctx context.Context, prompt string) error
	UpdateUsage(ctx context.Context, usage *UsageUpdate) error

	// Memory — unchanged semantics.
	SaveMemory(ctx context.Context, memory *Memory) error
	GetMemory(ctx context.Context, id string) (*Memory, error)
	SearchMemories(ctx context.Context, query string, opts *MemorySearchOptions) ([]*Memory, error)
	DeleteMemory(ctx context.Context, id string) error
	// ListMemoriesByType returns memories filtered by MemType, newest first.
	ListMemoriesByType(ctx context.Context, memType string, limit int) ([]*Memory, error)
	// MarkMemorySuperseded transitions oldID → superseded by newID.
	MarkMemorySuperseded(ctx context.Context, oldID, newID string) error
	// BumpMemoryUsage bumps reinforcement_count or neglect_count on a memory.
	// used=true increments reinforcement_count and sets last_used_at=now;
	// used=false increments neglect_count and leaves last_used_at unchanged.
	BumpMemoryUsage(ctx context.Context, id string, used bool) error

	// Transactions — group multiple operations atomically.
	WithTx(ctx context.Context, fn func(tx Tx) error) error

	// Lifecycle.
	Close() error
	Migrate() error
}

// Tx is the transaction-scoped interface.
type Tx interface {
	AppendMessage(ctx context.Context, msg *StoredMessage) error
	UpdateSystemPromptCache(ctx context.Context, prompt string) error
	UpdateUsage(ctx context.Context, usage *UsageUpdate) error
}
