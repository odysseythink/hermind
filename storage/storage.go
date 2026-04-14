// storage/storage.go
package storage

import (
	"context"
	"errors"
)

// Sentinel errors returned by storage implementations.
var (
	// ErrNotFound is returned when a session or message does not exist.
	ErrNotFound = errors.New("storage: not found")
)

// Storage is the root storage interface. Implementations must be safe for
// concurrent use.
type Storage interface {
	// Session operations
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, id string, updates *SessionUpdate) error
	ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, error)

	// Message operations
	AddMessage(ctx context.Context, sessionID string, msg *StoredMessage) error
	GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error)
	SearchMessages(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)

	// System prompt cache (for Anthropic prefix caching)
	UpdateSystemPrompt(ctx context.Context, sessionID string, prompt string) error

	// Usage accounting
	UpdateUsage(ctx context.Context, sessionID string, usage *UsageUpdate) error

	// Memory operations
	SaveMemory(ctx context.Context, memory *Memory) error
	GetMemory(ctx context.Context, id string) (*Memory, error)
	SearchMemories(ctx context.Context, query string, opts *MemorySearchOptions) ([]*Memory, error)
	DeleteMemory(ctx context.Context, id string) error

	// Transactions — group multiple operations atomically.
	// The function is called with a Tx scoped to a single SQL transaction.
	// Return an error to roll back. Return nil to commit.
	WithTx(ctx context.Context, fn func(tx Tx) error) error

	// Lifecycle
	Close() error
	Migrate() error
}

// Tx is the transaction-scoped interface. Operations are atomic: either
// all commit or all roll back. Do not retain a Tx reference after the
// callback returns.
type Tx interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, id string, updates *SessionUpdate) error
	AddMessage(ctx context.Context, sessionID string, msg *StoredMessage) error
	UpdateSystemPrompt(ctx context.Context, sessionID string, prompt string) error
	UpdateUsage(ctx context.Context, sessionID string, usage *UsageUpdate) error
}
