package gateway

import (
	"context"
	"log/slog"
)

// DispatchAndReply is a convenience adapters call inside their event
// loop: it runs the MessageHandler and calls SendReply if it succeeds.
// Errors are logged.
func DispatchAndReply(ctx context.Context, p Platform, handler MessageHandler, in IncomingMessage) {
	out, err := handler(ctx, in)
	if err != nil {
		slog.ErrorContext(ctx, "gateway: handler error", "platform", p.Name(), "err", err.Error())
		return
	}
	if out == nil {
		return
	}
	if err := p.SendReply(ctx, *out); err != nil {
		slog.ErrorContext(ctx, "gateway: send reply error", "platform", p.Name(), "err", err.Error())
	}
}
