# Plan 7c: Gateway Session Persistence + Dedup + Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Make the Gateway resilient across restarts and unreliable platforms by adding (1) inbound message deduplication, (2) session history rehydration from SQLite, and (3) retry-with-backoff around the handler body.

**Architecture:**
- `IncomingMessage` grows a `MessageID` field that adapters populate when the source platform has one (Telegram update ID, Twilio SID, etc.).
- A new `gateway.Dedup` struct holds an LRU of recent (platform, message_id) pairs. `handleMessage` consults it before doing real work.
- `SessionStore.GetOrCreate` gains a `LoadHistoryFn` hook. When a session is first seen in memory, the hook rehydrates history from persistent storage via `storage.GetMessages` (returning a slice of `message.Message`). Subsequent lookups hit the cache.
- `Gateway.handleMessage` wraps the inner Engine call in a retry loop with exponential backoff (100ms → 200ms → 400ms, max 3 attempts). Non-retryable errors (ctx canceled, validation) return immediately.

**Tech Stack:** Go 1.25 stdlib, existing `storage`, `message`, `gateway` packages. No new deps.

---

## Task 1: IncomingMessage.MessageID + Dedup

- [ ] Add `MessageID string` to `gateway.IncomingMessage`.
- [ ] Create `gateway/dedup.go`:

```go
package gateway

import "sync"

// Dedup is a small LRU-ish in-memory cache of recently seen message
// IDs. It is intentionally simple: when capacity is reached the
// oldest entries are dropped via a FIFO order slice.
type Dedup struct {
	mu       sync.Mutex
	capacity int
	order    []string
	set      map[string]struct{}
}

func NewDedup(capacity int) *Dedup {
	if capacity <= 0 {
		capacity = 1024
	}
	return &Dedup{
		capacity: capacity,
		set:      make(map[string]struct{}, capacity),
	}
}

// Seen returns true if key was already observed, inserting it otherwise.
func (d *Dedup) Seen(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.set[key]; ok {
		return true
	}
	d.set[key] = struct{}{}
	d.order = append(d.order, key)
	if len(d.order) > d.capacity {
		drop := d.order[0]
		d.order = d.order[1:]
		delete(d.set, drop)
	}
	return false
}
```

- [ ] Create `gateway/dedup_test.go` covering repeat insertion, eviction, and capacity.
- [ ] Commit `feat(gateway): add MessageID dedup LRU`.

---

## Task 2: SessionStore history loader hook

- [ ] Add a new field + setter on `SessionStore`:

```go
// LoadHistoryFn, if non-nil, is called the first time a session key
// is requested so the caller can rehydrate history from persistent
// storage. It returns the initial history (may be nil) and nil
// error on success. Errors are logged but do not block the session
// from being created.
type LoadHistoryFn func(ctx context.Context, platform, userID string) ([]message.Message, error)
```

- [ ] Update `GetOrCreate` to optionally call the loader. Since `GetOrCreate` currently takes no `ctx`, keep backward compat by adding a new method `GetOrCreateWithLoader(ctx, platform, userID) *Session`.
- [ ] Commit `feat(gateway): add SessionStore history-loader hook`.

---

## Task 3: Gateway integration

- [ ] Add a `storage.Storage` dependency (already present) to construct a default history loader that converts `storage.StoredMessage` to `message.Message`.
- [ ] Add a retry helper:

```go
func (g *Gateway) runWithRetry(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	const maxAttempts = 3
	var lastErr error
	backoff := 100 * time.Millisecond
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err := g.runOnce(ctx, in)
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lastErr = err
		slog.WarnContext(ctx, "gateway: retry", "attempt", attempt, "err", err.Error())
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
	}
	return nil, lastErr
}
```

where `runOnce` is the current body of `handleMessage`.

- [ ] Update `handleMessage` to:
  1. Early-return when `in.MessageID != ""` and `g.dedup.Seen(in.Platform + ":" + in.MessageID)` is true.
  2. Call `runWithRetry` instead of executing inline.

- [ ] Wire history loader from storage in `NewGateway`: if `s != nil`, set `g.sessions.LoadHistoryFn` to a function that loads up to 50 recent messages and converts them.

- [ ] Commit `feat(gateway): add retry, dedup, and history rehydration to handleMessage`.

---

## Task 4: Tests

- [ ] Update the existing `TestGatewayRoutesMessageAndReplies` to still pass.
- [ ] Add `TestGatewayDedupSkipsDuplicate`: same MessageID sent twice, fake provider should only be called once.
- [ ] Add `TestGatewayRetryRecovers`: a fake provider that fails once then succeeds should end up sending one reply.
- [ ] Run `go test ./gateway/...` — PASS.
- [ ] Commit `test(gateway): cover dedup and retry paths`.

---

## Verification Checklist

- [ ] `go test ./gateway/...` passes
- [ ] The Gateway still works with `storage=nil` (dedup + retry active, loader skipped)
- [ ] Duplicate inbound messages produce exactly one Engine call
