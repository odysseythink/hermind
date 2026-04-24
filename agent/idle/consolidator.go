// Package idle runs periodic background maintenance (memory consolidation)
// gated by HTTP-level activity, so heavy work happens only during quiet
// windows rather than on the conversation-end critical path.
package idle

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// IdleConsolidator periodically calls memprovider.Consolidate when no
// HTTP activity has been observed for idleAfter.
type IdleConsolidator struct {
	store       storage.Storage
	interval    time.Duration
	idleAfter   time.Duration
	opts        *memprovider.ConsolidateOptions
	lastRequest atomic.Int64 // unix nanos

	// runFn is an override hook for tests. Nil means use the real path.
	runFn func(ctx context.Context)
}

// New constructs a consolidator. interval <= 0 disables it (Start returns
// immediately when called).
func New(store storage.Storage, interval, idleAfter time.Duration, opts *memprovider.ConsolidateOptions) *IdleConsolidator {
	return &IdleConsolidator{
		store:     store,
		interval:  interval,
		idleAfter: idleAfter,
		opts:      opts,
	}
}

// NoteActivity records that an HTTP request just happened. Safe for
// concurrent use.
func (c *IdleConsolidator) NoteActivity() {
	c.lastRequest.Store(time.Now().UnixNano())
}

// Start blocks until ctx is done. Returns immediately when interval is
// non-positive (disabled).
func (c *IdleConsolidator) Start(ctx context.Context) {
	if c.interval <= 0 {
		return
	}
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if time.Since(time.Unix(0, c.lastRequest.Load())) < c.idleAfter {
				continue
			}
			if c.runFn != nil {
				c.runFn(ctx)
				continue
			}
			_, _ = memprovider.Consolidate(ctx, c.store, c.opts)
		}
	}
}
