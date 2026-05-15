// Package logging wraps mlog with a small per-request context helper.
// All gateway, cron, and platform code should use mlog.InfoContext/ErrorContext
// with a context that has been enriched via WithRequestID so request IDs
// appear in logs.
//
// The package also installs an mlog-backed slog.Handler as the default logger
// so that third-party code using log/slog or the standard "log" package is
// captured and emitted through mlog's structured pipeline.
package logging

import (
	"context"
	"flag"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/mlog"
)

type ctxKey int

const requestIDKey ctxKey = 1

func init() {
	// Default to stderr so that test processes (which may not call Setup)
	// do not trigger mlog's file-sink creation, which can collide when
	// multiple severity levels are initialised within the same second.
	_ = flag.Set("logtostderr", "true")
}

// Setup installs an mlog-backed slog handler as the default logger and
// configures mlog for structured stderr output.
// level is one of "debug", "info", "warn", "error" (default info).
func Setup(level string) {
	mlog.SetLogMode(mlog.LogModeStructured)
	_ = flag.Set("logtostderr", "true")

	var slogLvl slog.Level
	switch level {
	case "debug":
		slogLvl = slog.LevelDebug
	case "warn":
		slogLvl = slog.LevelWarn
	case "error":
		slogLvl = slog.LevelError
	default:
		slogLvl = slog.LevelInfo
	}

	h := &mlogHandler{level: slogLvl}
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

// mlogHandler is a slog.Handler that forwards records to mlog. slog.Attrs
// are converted to mlog.Field values so structured output is preserved.
type mlogHandler struct {
	level slog.Level
	attrs []slog.Attr
	group string
}

func (h *mlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *mlogHandler) Handle(ctx context.Context, r slog.Record) error {
	fields := make([]mlog.Field, 0, len(h.attrs)+r.NumAttrs()+1)
	for _, a := range h.attrs {
		fields = append(fields, attrToField(h.group, a))
	}
	r.Attrs(func(a slog.Attr) bool {
		fields = append(fields, attrToField(h.group, a))
		return true
	})

	args := make([]any, 0, 1+len(fields))
	args = append(args, r.Message)
	for _, f := range fields {
		args = append(args, f)
	}
	switch {
	case r.Level >= slog.LevelError:
		mlog.ErrorContext(ctx, args...)
	case r.Level >= slog.LevelWarn:
		mlog.WarningContext(ctx, args...)
	case r.Level >= slog.LevelInfo:
		mlog.InfoContext(ctx, args...)
	default:
		mlog.DebugContext(ctx, args...)
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

func attrToField(group string, a slog.Attr) mlog.Field {
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	switch v := a.Value.Any().(type) {
	case string:
		return mlog.String(key, v)
	case int:
		return mlog.Int(key, v)
	case int64:
		return mlog.Int64(key, v)
	case float64:
		return mlog.Float64(key, v)
	case bool:
		return mlog.Bool(key, v)
	case time.Duration:
		return mlog.Duration(key, v)
	case error:
		return mlog.String(key, v.Error())
	default:
		return mlog.Any(key, v)
	}
}
