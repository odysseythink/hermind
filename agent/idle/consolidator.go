// Package idle runs periodic background maintenance (memory consolidation)
// gated by the presence framework, so heavy work happens only during
// quiet windows rather than on the conversation-end critical path.
package idle

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/agent/presence"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// IdleConsolidator periodically calls memprovider.Consolidate when the
// presence Provider says the user is unavailable.
type IdleConsolidator struct {
	store    storage.Storage
	interval time.Duration
	presence presence.Provider
	opts     *memprovider.ConsolidateOptions

	// runFn is an override hook for tests. Nil means use the real path.
	runFn func(ctx context.Context)
}

// New constructs a consolidator. interval <= 0 disables it (Start
// returns immediately when called). p must be non-nil; pass a
// presence.NewComposite() with no sources to get always-fail-closed
// behavior.
func New(store storage.Storage, interval time.Duration, p presence.Provider, opts *memprovider.ConsolidateOptions) *IdleConsolidator {
	return &IdleConsolidator{
		store:    store,
		interval: interval,
		presence: p,
		opts:     opts,
	}
}

// Start blocks until ctx is done. Returns immediately when interval is
// non-positive (disabled).
func (c *IdleConsolidator) Start(ctx context.Context) {
	if c.interval <= 0 {
		return
	}
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	c.runLoop(ctx, ticker.C, time.Now)
}

// runLoop is the testable inner loop. Production callers go through
// Start; tests inject a synthetic ticks channel and now-func to drive
// time deterministically.
func (c *IdleConsolidator) runLoop(ctx context.Context, ticks <-chan time.Time, now func() time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if c.presence == nil || !c.presence.Available(now()) {
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
