package presence

import (
	"sync/atomic"
	"time"
)

// HTTPIdle reports the user as Absent when no HTTP request has been
// observed for `absentAfter`. It never votes Present — HTTP traffic
// does not prove the user is at the keyboard, especially when hermind
// is exposed as a /v1/messages proxy that may be driven by automated
// clients.
type HTTPIdle struct {
	absentAfter time.Duration
	lastRequest atomic.Int64 // unix nanos
}

// NewHTTPIdle constructs a source. absentAfter <= 0 disables the
// signal (Vote always returns Unknown).
//
// lastRequest is initialized to "now" so the first tick after startup
// doesn't see "elapsed since 1970" and trigger immediately before any
// HTTP activity has been observed.
func NewHTTPIdle(absentAfter time.Duration) *HTTPIdle {
	h := &HTTPIdle{absentAfter: absentAfter}
	h.lastRequest.Store(time.Now().UnixNano())
	return h
}

// Name implements Source.
func (h *HTTPIdle) Name() string { return "http_idle" }

// NoteActivity records a request at the wall-clock now.
// Production HTTP middleware calls this on every request.
func (h *HTTPIdle) NoteActivity() { h.NoteActivityAt(time.Now()) }

// NoteActivityAt is the deterministic-time variant for tests.
func (h *HTTPIdle) NoteActivityAt(t time.Time) {
	h.lastRequest.Store(t.UnixNano())
}

// Vote implements Source. Returns Absent once `absentAfter` has elapsed
// since the most recent NoteActivity, Unknown otherwise. Never Present.
func (h *HTTPIdle) Vote(now time.Time) Vote {
	if h.absentAfter <= 0 {
		return Unknown
	}
	elapsed := now.Sub(time.Unix(0, h.lastRequest.Load()))
	if elapsed < 0 {
		return Unknown // clock skew defense
	}
	if elapsed >= h.absentAfter {
		return Absent
	}
	return Unknown
}
