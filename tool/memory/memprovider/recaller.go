package memprovider

import "context"

// Recaller is an optional capability implemented by memory providers
// that can surface relevant past memories for a given query. The engine
// type-asserts to this interface before calling Recall during a turn,
// so providers without recall (Honcho, Mem0, etc.) are simply skipped.
type Recaller interface {
	Recall(ctx context.Context, query string, limit int) ([]string, error)
}
