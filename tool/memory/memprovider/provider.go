// Package memprovider defines pluggable external memory providers.
//
// Each Provider is an adapter to a remote memory service (Honcho, Mem0,
// Supermemory, …). The CLI bootstrap picks at most one active Provider
// based on config.Memory.Provider, calls Initialize once at startup,
// registers its extra tools into the tool.Registry, and drives the
// SyncTurn / Shutdown lifecycle.
//
// This is intentionally a much smaller surface than the Python
// MemoryProvider base class. Advanced hooks (on_session_end,
// on_pre_compress, on_delegation, background prefetch) are deferred
// to a later plan.
package memprovider

import (
	"context"

	"github.com/odysseythink/hermind/tool"
)

// Provider is the minimal interface every external memory provider
// implements. Implementations are expected to be safe to call from
// the CLI goroutine; long-running work should be queued internally.
type Provider interface {
	// Name is a short, lowercase identifier used in logs and the
	// factory ("honcho", "mem0", "supermemory").
	Name() string

	// Initialize performs one-time setup for the given session. It is
	// called exactly once during CLI startup.
	Initialize(ctx context.Context, sessionID string) error

	// RegisterTools registers any provider-specific tools into the
	// given registry. Most providers register 1–2 tools
	// (recall + remember). Called after Initialize.
	RegisterTools(reg *tool.Registry)

	// SyncTurn is called after each completed turn with the user
	// message and assistant reply. Implementations should queue
	// persistence work and return quickly.
	SyncTurn(ctx context.Context, userMsg, assistantMsg string) error

	// Shutdown flushes any queued work and releases resources.
	// Called at CLI exit.
	Shutdown(ctx context.Context) error
}
