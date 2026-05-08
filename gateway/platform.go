// Package gateway is the multi-platform messaging front end.
// It receives messages from one or more Platform adapters, routes each
// message to the shared single-conversation engine, and sends replies back.
package gateway

import (
	"context"
	"log/slog"
)

// IncomingMessage is the platform-agnostic shape every adapter emits.
type IncomingMessage struct {
	Platform  string // "telegram", "feishu", etc.
	UserID    string
	ChatID    string
	Text      string
	MessageID string // platform-native id, used for dedup
}

// OutgoingMessage is what the Pump hands back to the adapter.
type OutgoingMessage struct {
	UserID string
	ChatID string
	Text   string
}

// MessageHandler is the callback adapters invoke for each new message.
type MessageHandler func(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error)

// Platform is the interface every adapter implements.
type Platform interface {
	// Name returns the canonical platform name used in logs and dedup keys.
	Name() string

	// Run starts the adapter loop. Must block until ctx is cancelled.
	// Incoming messages are delivered via handler.
	Run(ctx context.Context, handler MessageHandler) error

	// SendReply pushes a message back to the platform.
	SendReply(ctx context.Context, out OutgoingMessage) error
}

// DispatchAndReply calls handler and then SendReply on the platform.
// Errors are logged; the adapter event loop continues regardless.
func DispatchAndReply(ctx context.Context, p Platform, handler MessageHandler, in IncomingMessage) {
	out, err := handler(ctx, in)
	if err != nil {
		slog.ErrorContext(ctx, "gateway: handler error", "platform", p.Name(), "err", err)
		return
	}
	if out == nil {
		return
	}
	if err := p.SendReply(ctx, *out); err != nil {
		slog.ErrorContext(ctx, "gateway: send reply error", "platform", p.Name(), "err", err)
	}
}
