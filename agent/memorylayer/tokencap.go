package memorylayer

import (
	"sync"
	"sync/atomic"
)

// TokenCap tracks Agentic LLM token spend within a session. The
// per-turn budget is enforced by Reset+Allow checks during a single
// Recall; the per-session budget persists across calls.
type TokenCap struct {
	perTurn    int
	perSession int

	sessionUsed atomic.Int64

	mu       sync.Mutex
	turnUsed int
}

func NewTokenCap(perTurn, perSession int) *TokenCap {
	return &TokenCap{perTurn: perTurn, perSession: perSession}
}

// ResetTurn zeros the per-turn counter. Call at the start of each Recall.
func (t *TokenCap) ResetTurn() {
	t.mu.Lock()
	t.turnUsed = 0
	t.mu.Unlock()
}

// Allow reports whether spending cost more tokens would stay under both
// caps. If yes, the cost is recorded. If no, returns false and leaves
// counters untouched.
func (t *TokenCap) Allow(cost int) bool {
	if cost < 0 {
		cost = 0
	}
	sessTotal := t.sessionUsed.Load() + int64(cost)
	if t.perSession > 0 && sessTotal > int64(t.perSession) {
		return false
	}
	t.mu.Lock()
	if t.perTurn > 0 && t.turnUsed+cost > t.perTurn {
		t.mu.Unlock()
		return false
	}
	t.turnUsed += cost
	t.mu.Unlock()
	t.sessionUsed.Add(int64(cost))
	return true
}

// SessionUsed returns the current session-wide spend (for telemetry).
func (t *TokenCap) SessionUsed() int64 { return t.sessionUsed.Load() }
