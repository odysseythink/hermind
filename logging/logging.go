// Package logging wraps log/slog with a JSON handler and a small
// per-request context helper. All gateway, cron, and platform code
// should use slog.InfoContext/ErrorContext with a context that has
// been enriched via WithRequestID so request IDs appear in logs.
//
// The underlying sink is github.com/odysseythink/mlog — callers keep
// writing structured log records via slog; this package flattens the
// record's message + attrs into a single string and dispatches it to
// mlog's severity-specific writers.
package logging

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/odysseythink/mlog"
)

type ctxKey int

const requestIDKey ctxKey = 1

// Setup installs an mlog-backed slog handler as the default logger.
// level is one of "debug", "info", "warn", "error" (default info).
// mlog's default is to write log files under $TMPDIR; we flip the
// -logtostderr flag so log output still goes to stderr, matching the
// pre-mlog behavior callers expect.
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
	_ = flag.Set("logtostderr", "true")
	h := &mlogHandler{level: lvl}
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

// mlogHandler is a slog.Handler that forwards records to mlog. Attrs
// are flattened into `key=value` pairs appended to the message, since
// mlog is glog-style and does not take structured fields.
type mlogHandler struct {
	level slog.Level
	attrs []slog.Attr
	group string
}

func (h *mlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *mlogHandler) Handle(ctx context.Context, r slog.Record) error {
	var sb strings.Builder
	sb.WriteString(r.Message)
	for _, a := range h.attrs {
		writeAttr(&sb, h.group, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&sb, h.group, a)
		return true
	})
	msg := sb.String()
	switch {
	case r.Level >= slog.LevelError:
		mlog.ErrorContext(ctx, msg)
	case r.Level >= slog.LevelWarn:
		mlog.WarningContext(ctx, msg)
	case r.Level >= slog.LevelInfo:
		mlog.InfoContext(ctx, msg)
	default:
		mlog.DebugContext(ctx, msg)
	}
	return nil
}

func (h *mlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *mlogHandler) WithGroup(name string) slog.Handler {
	clone := *h
	if h.group != "" {
		clone.group = h.group + "." + name
	} else {
		clone.group = name
	}
	return &clone
}

func writeAttr(sb *strings.Builder, group string, a slog.Attr) {
	sb.WriteByte(' ')
	if group != "" {
		sb.WriteString(group)
		sb.WriteByte('.')
	}
	sb.WriteString(a.Key)
	sb.WriteByte('=')
	fmt.Fprintf(sb, "%v", a.Value.Any())
}
