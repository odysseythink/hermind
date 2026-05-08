package idle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/odysseythink/hermind/agent/presence"
)

// TestNewConstructs verifies New() creates a consolidator with the
// given presence provider.
func TestNewConstructs(t *testing.T) {
	p := alwaysPresent{}
	c := New(nil, time.Second, p, nil)
	assert.NotNil(t, c)
	assert.Equal(t, time.Second, c.interval)
}

func TestIdleConsolidator_RespectsDisabled(t *testing.T) {
	c := &IdleConsolidator{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Start(ctx) // must not block, must not call anything
}

func TestIdleConsolidator_AvailableUserTriggersAtInterval(t *testing.T) {
	var called atomic.Int32
	c := New(nil, 50*time.Millisecond, alwaysAbsent{}, nil)
	c.runFn = func(context.Context) { called.Add(1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Start(ctx)

	// alwaysAbsent means user is available; consolidation should fire
	// at each tick.
	time.Sleep(150 * time.Millisecond)
	assert.GreaterOrEqual(t, called.Load(), int32(1))
}

func TestIdleConsolidator_PresentUserSuppressesRun(t *testing.T) {
	var called atomic.Int32
	c := New(nil, 50*time.Millisecond, alwaysPresent{}, nil)
	c.runFn = func(context.Context) { called.Add(1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Start(ctx)

	// alwaysPresent means user is here; consolidation should never fire.
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, int32(0), called.Load())
}

// alwaysAbsent is a stub presence.Provider that always answers Available.
type alwaysAbsent struct{}

func (alwaysAbsent) Available(_ time.Time) bool { return true }
func (alwaysAbsent) Sources(_ time.Time) []presence.SourceVote {
	return []presence.SourceVote{{Name: "stub", Vote: presence.Absent}}
}

// alwaysPresent says "user is here" — consolidator must never fire.
type alwaysPresent struct{}

func (alwaysPresent) Available(_ time.Time) bool { return false }
func (alwaysPresent) Sources(_ time.Time) []presence.SourceVote {
	return []presence.SourceVote{{Name: "stub", Vote: presence.Present}}
}

func TestIdleConsolidator_runLoopFiresWhenAvailable(t *testing.T) {
	c := New(nil, time.Hour, alwaysAbsent{}, nil)
	ran := atomic.Int64{}
	c.runFn = func(_ context.Context) { ran.Add(1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan time.Time, 3)
	done := make(chan struct{})
	go func() {
		c.runLoop(ctx, ticks, time.Now)
		close(done)
	}()

	// Drive 3 ticks; each should run runFn.
	now := time.Now()
	ticks <- now
	ticks <- now
	ticks <- now

	require.Eventually(t, func() bool { return ran.Load() == 3 }, time.Second, 5*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runLoop did not exit on ctx.Done within 1s")
	}
}

func TestIdleConsolidator_runLoopSkipsWhenNotAvailable(t *testing.T) {
	c := New(nil, time.Hour, alwaysPresent{}, nil)
	ran := atomic.Int64{}
	c.runFn = func(_ context.Context) { ran.Add(1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan time.Time, 3)
	go c.runLoop(ctx, ticks, time.Now)

	now := time.Now()
	for i := 0; i < 3; i++ {
		ticks <- now
	}
	// Give the loop a moment to drain.
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int64(0), ran.Load(), "Present vote must skip every tick")
}
