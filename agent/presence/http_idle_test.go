package presence

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPIdle_JustNotedReturnsUnknown(t *testing.T) {
	h := NewHTTPIdle(5 * time.Minute)
	now := time.Now()
	h.NoteActivityAt(now)
	require.Equal(t, Unknown, h.Vote(now))
}

func TestHTTPIdle_PastAbsentAfterReturnsAbsent(t *testing.T) {
	h := NewHTTPIdle(5 * time.Minute)
	noted := time.Now()
	h.NoteActivityAt(noted)
	require.Equal(t, Absent, h.Vote(noted.Add(5*time.Minute)),
		"exactly absentAfter elapsed counts as Absent (>= boundary)")
	require.Equal(t, Absent, h.Vote(noted.Add(10*time.Minute)))
}

func TestHTTPIdle_BelowThresholdReturnsUnknown(t *testing.T) {
	h := NewHTTPIdle(5 * time.Minute)
	noted := time.Now()
	h.NoteActivityAt(noted)
	require.Equal(t, Unknown, h.Vote(noted.Add(4*time.Minute+59*time.Second)))
}

func TestHTTPIdle_DisabledAlwaysUnknown(t *testing.T) {
	h := NewHTTPIdle(0)
	require.Equal(t, Unknown, h.Vote(time.Now()))
	h.NoteActivityAt(time.Now().Add(-time.Hour))
	require.Equal(t, Unknown, h.Vote(time.Now()),
		"absentAfter == 0 is the disabled signal")
}

func TestHTTPIdle_FutureLastRequestReturnsUnknown(t *testing.T) {
	// Defensive against system clock skew.
	h := NewHTTPIdle(5 * time.Minute)
	now := time.Now()
	h.NoteActivityAt(now.Add(time.Hour)) // last request "in the future"
	require.Equal(t, Unknown, h.Vote(now))
}

func TestHTTPIdle_NeverReturnsPresent(t *testing.T) {
	h := NewHTTPIdle(5 * time.Minute)
	now := time.Now()
	// Hammer NoteActivity with the same timestamp; HTTPIdle has no Present
	// vote, regardless of activity volume.
	for i := 0; i < 100; i++ {
		h.NoteActivityAt(now)
	}
	require.NotEqual(t, Present, h.Vote(now),
		"HTTPIdle must never vote Present — HTTP traffic does not prove user is at keyboard")
}

func TestHTTPIdle_NoteActivityAndVoteRaceClean(t *testing.T) {
	h := NewHTTPIdle(5 * time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				h.NoteActivity()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_ = h.Vote(time.Now())
			}
		}()
	}
	wg.Wait()
}

func TestHTTPIdle_Name(t *testing.T) {
	require.Equal(t, "http_idle", NewHTTPIdle(time.Second).Name())
}
