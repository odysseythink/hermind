// Package logging wraps log/slog with a JSON handler and a small
// per-request context helper. All gateway, cron, and platform code
// should use slog.InfoContext/ErrorContext with a context that has
// been enriched via WithRequestID so request IDs appear in logs.
package logging

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type ctxKey int

const requestIDKey ctxKey = 1

// Setup installs a JSON slog handler as the default logger.
// level is one of "debug", "info", "warn", "error" (default info).
func Setup(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(&contextHandler{inner: h}))
}

// WithRequestID attaches a request ID to ctx. If id is empty a new
// UUID is generated.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request ID stored in ctx, or "" if none.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// contextHandler is a slog.Handler that injects the request ID from
// the context into every log record.
type contextHandler struct {
	inner slog.Handler
}

func (c *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.inner.Enabled(ctx, level)
}

func (c *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return c.inner.Handle(ctx, r)
}

func (c *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{inner: c.inner.WithAttrs(attrs)}
}

func (c *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{inner: c.inner.WithGroup(name)}
}
