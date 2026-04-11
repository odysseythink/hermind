// Package gateway is the multi-platform messaging front end.
// It receives messages from one or more Platform adapters, routes
// each message to a per-user agent.Engine conversation, and sends
// replies back. Adapters are registered in main and driven by a
// shared Gateway instance.
package gateway

import "context"

// IncomingMessage is the platform-agnostic shape every adapter emits.
type IncomingMessage struct {
	Platform string // "telegram", "api_server", etc.
	UserID   string
	ChatID   string
	Text     string
	Extra    map[string]any
}

// OutgoingMessage is what the Gateway hands back to the adapter.
type OutgoingMessage struct {
	UserID string
	ChatID string
	Text   string
}

// MessageHandler is the callback adapters invoke for each new message.
type MessageHandler func(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error)

// Platform is the interface every adapter implements.
type Platform interface {
	// Name returns the canonical platform name.
	Name() string

	// Run starts the adapter loop. It must block until ctx is
	// cancelled. Incoming messages are delivered via handler.
	Run(ctx context.Context, handler MessageHandler) error

	// SendReply pushes a message back to the user via the platform.
	SendReply(ctx context.Context, out OutgoingMessage) error
}
