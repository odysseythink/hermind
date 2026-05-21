package memorylayer

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/mlog"
)

type LifecycleConfig struct {
	InjectCoreOnStart      bool
	CoreMaxCount           int // hard cap on rows; default 10
	CoreMaxTokens          int // character-based proxy; default 600
	InjectForesightOnStart bool
	ForesightMaxCount      int // default 3
	ForesightDaysAhead     int // default 7 — only inject foresights expiring within this window
}

func (c *LifecycleConfig) fill() {
	if c.CoreMaxCount <= 0 {
		c.CoreMaxCount = 10
	}
	if c.CoreMaxTokens <= 0 {
		c.CoreMaxTokens = 600
	}
	if c.ForesightMaxCount <= 0 {
		c.ForesightMaxCount = 3
	}
	if c.ForesightDaysAhead <= 0 {
		c.ForesightDaysAhead = 7
	}
}

// Lifecycle drives the OnSessionStart hook. It is intentionally narrow
// in P2 — the design's other hook (OnTurnComplete) is already handled
// by MemoryLayer.ObserveTurn / Flush.
type Lifecycle struct {
	store storage.Storage
	cfg   LifecycleConfig
}

func NewLifecycle(store storage.Storage, cfg LifecycleConfig) *Lifecycle {
	cfg.fill()
	return &Lifecycle{store: store, cfg: cfg}
}

// OnSessionStart loads pinned context from storage and returns it as
// InjectedMemory entries. The caller (engine wiring) decides how to
// merge them into the prompt.
//
// Ordering: core memories come first (most recent first), then any
// foresights whose ExpiresAt is within ForesightDaysAhead. Total
// content length is capped by CoreMaxTokens for core; foresights
// are bounded only by ForesightMaxCount.
func (l *Lifecycle) OnSessionStart(ctx context.Context) ([]memprovider.InjectedMemory, error) {
	out := []memprovider.InjectedMemory{}

	if l.cfg.InjectCoreOnStart {
		core, err := l.store.SearchMemories(ctx, "", &storage.MemorySearchOptions{
			MemTypes: []string{"core"},
			Limit:    l.cfg.CoreMaxCount,
		})
		if err != nil {
			mlog.Warning("memorylayer: Lifecycle core search failed", mlog.String("err", err.Error()))
		} else {
			tokens := 0
			for _, m := range core {
				if tokens+len(m.Content) > l.cfg.CoreMaxTokens {
					break
				}
				out = append(out, memprovider.InjectedMemory{ID: m.ID, Content: m.Content})
				tokens += len(m.Content)
			}
		}
	}

	if l.cfg.InjectForesightOnStart {
		cutoff := time.Now().UTC().AddDate(0, 0, l.cfg.ForesightDaysAhead)
		fs, err := l.store.SearchMemories(ctx, "", &storage.MemorySearchOptions{
			MemTypes:       []string{"foresight"},
			Limit:          l.cfg.ForesightMaxCount * 4, // overfetch, then filter
			IncludeExpired: false,
		})
		if err != nil {
			mlog.Warning("memorylayer: Lifecycle foresight search failed", mlog.String("err", err.Error()))
		} else {
			picked := 0
			for _, m := range fs {
				if picked >= l.cfg.ForesightMaxCount {
					break
				}
				if !m.ExpiresAt.IsZero() && m.ExpiresAt.After(cutoff) {
					continue // outside the lookahead window
				}
				out = append(out, memprovider.InjectedMemory{ID: m.ID, Content: m.Content})
				picked++
			}
		}
	}

	return out, nil
}
