package compression

import (
	"context"
	"strings"

	pcompression "github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
)

const summaryPrefix = "[Compressed summary of earlier conversation]\n"

// SaveFunc persists an extracted summary. Workspace and thread IDs are captured
// by the closure at the call site (handler.go / runtime.go).
type SaveFunc func(summary string) error

// Observer wraps a ContextEngine to intercept compression results and extract
// the summary for persistence. It is a typed shim for the missing
// PreviousSummary()/SetPreviousSummary() accessors in current Pantheon.
type Observer struct {
	inner ContextEngine
	save  SaveFunc
}

// NewObserver wraps the given engine with summary-extraction persistence.
func NewObserver(inner ContextEngine, save SaveFunc) *Observer {
	return &Observer{inner: inner, save: save}
}

// Compress delegates to the inner engine, then extracts and saves any summary
// found in the returned messages.
func (o *Observer) Compress(ctx context.Context, history []core.Message) ([]core.Message, error) {
	out, err := o.inner.Compress(ctx, history)
	if err != nil {
		return nil, err
	}
	if summary := extractSummary(out); summary != "" && o.save != nil {
		_ = o.save(summary)
	}
	return out, nil
}

// Inner returns the underlying Pantheon *Compressor if the inner engine is
// one (or wraps one). This is needed to pass the concrete type to Pantheon's
// WithCompressor option.
func (o *Observer) Inner() *pcompression.Compressor {
	if pc, ok := o.inner.(*pcompression.Compressor); ok {
		return pc
	}
	return nil
}

func extractSummary(msgs []core.Message) string {
	for _, m := range msgs {
		if m.Role != core.MESSAGE_ROLE_ASSISTANT {
			continue
		}
		text := m.Text()
		if strings.HasPrefix(text, summaryPrefix) {
			return strings.TrimPrefix(text, summaryPrefix)
		}
	}
	return ""
}
