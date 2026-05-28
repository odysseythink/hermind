package mcp

import "sync"

// concurrencyLimiter is a per-MCP-server in-flight call limiter. Each server
// has its own semaphore sized to either its config override or the global
// default. Acquire is non-blocking (TryAcquire): if the slot is taken,
// callers should fail fast with CONCURRENCY_LIMIT rather than queueing.
type concurrencyLimiter struct {
	mu           sync.Mutex
	defaultLimit int
	overrides    map[string]int // serverName → limit
	inFlight     map[string]int // serverName → current count
}

func newConcurrencyLimiter(defaultLimit int) *concurrencyLimiter {
	if defaultLimit < 1 {
		defaultLimit = 1
	}
	return &concurrencyLimiter{
		defaultLimit: defaultLimit,
		overrides:    make(map[string]int),
		inFlight:     make(map[string]int),
	}
}

func (l *concurrencyLimiter) SetOverride(name string, limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit < 1 {
		delete(l.overrides, name)
		return
	}
	l.overrides[name] = limit
}

func (l *concurrencyLimiter) ClearOverride(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.overrides, name)
	delete(l.inFlight, name)
}

func (l *concurrencyLimiter) TryAcquire(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	limit, ok := l.overrides[name]
	if !ok {
		limit = l.defaultLimit
	}
	if l.inFlight[name] >= limit {
		return false
	}
	l.inFlight[name]++
	return true
}

func (l *concurrencyLimiter) Release(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFlight[name] > 0 {
		l.inFlight[name]--
	}
}
