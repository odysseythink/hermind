// Package browser provides browser automation providers. Plan 6d ships
// only the Browserbase cloud backend. The tool surface is intentionally
// small: create, close, and fetch live debug URLs. Full page-level
// automation is expected to happen through MCP/CDP tooling once the
// session URL is known.
package browser

import "context"

// Session describes a created browser session.
type Session struct {
	ID         string `json:"id"`
	ConnectURL string `json:"connect_url"` // CDP WebSocket URL
	LiveURL    string `json:"live_url"`    // Debug / watch URL (may be empty)
	Provider   string `json:"provider"`
}

// Provider is the minimal browser backend interface. Implementations
// must be safe for concurrent use.
type Provider interface {
	Name() string
	IsConfigured() bool
	CreateSession(ctx context.Context) (*Session, error)
	CloseSession(ctx context.Context, id string) error
	LiveURL(ctx context.Context, id string) (string, error)
}
