package idle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/agent/presence"
)

// fakeClock is an atomic, thread-safe wall-clock substitute.
type fakeClock struct {
	now atomic.Pointer[time.Time]
}

func newFakeClock(t time.Time) *fakeClock {
	c := &fakeClock{}
	c.now.Store(&t)
	return c
}

func (c *fakeClock) Now() time.Time         { return *c.now.Load() }
func (c *fakeClock) Set(t time.Time)        { c.now.Store(&t) }
func (c *fakeClock) Advance(d time.Duration) {
	next := c.Now().Add(d)
	c.now.Store(&next)
}

// startConsolidator spins up a consolidator + Composite and runs the
// runLoop in a goroutine. Returns the runFn counter, the ticks
// channel, the cancel func, and a `done` channel that closes when the
// loop returns.
func startConsolidator(t *testing.T, clk *fakeClock, comp presence.Provider) (
	runs *atomic.Int64,
	ticks chan<- time.Time,
	cancel context.CancelFunc,
	done <-chan struct{},
) {
	t.Helper()

	c := New(nil, time.Hour, comp, nil)
	r := atomic.Int64{}
	c.runFn = func(_ context.Context) { r.Add(1) }

	ctx, cancelFn := context.WithCancel(context.Background())
	tch := make(chan time.Time, 16)
	doneCh := make(chan struct{})

	go func() {
		c.runLoop(ctx, tch, clk.Now)
		close(doneCh)
	}()

	return &r, tch, cancelFn, doneCh
}

// waitForRuns asserts that the runs counter reaches `want` within 200 ms.
func waitForRuns(t *testing.T, runs *atomic.Int64, want int64) {
	t.Helper()
	require.Eventually(t, func() bool { return runs.Load() == want },
		200*time.Millisecond, time.Millisecond,
		"runs=%d want=%d", runs.Load(), want)
}

func TestLoopE2E_HTTPIdleOnly(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))
	httpIdle := presence.NewHTTPIdle(5 * time.Minute)
	httpIdle.NoteActivityAt(clk.Now())
	comp := presence.NewComposite(httpIdle)

	runs, ticks, cancel, done := startConsolidator(t, clk, comp)
	defer cancel()

	// Just-noted activity → tick → no run (HTTP Unknown, all-Unknown fails closed).
	ticks <- clk.Now()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int64(0), runs.Load(), "just-noted activity must not fire")

	// Advance 5 min → tick → run (HTTP Absent).
	clk.Advance(5 * time.Minute)
	ticks <- clk.Now()
	waitForRuns(t, runs, 1)

	// Activity again → tick → no run.
	httpIdle.NoteActivityAt(clk.Now())
	ticks <- clk.Now()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int64(1), runs.Load())

	cancel()
	<-done
}

func TestLoopE2E_SleepWindowAllowsDespiteHTTPActivity(t *testing.T) {
	// Pacific: 23:30 local, well inside 23:00–07:00 sleep window.
	clk := newFakeClock(time.Date(2026, 1, 2, 7, 30, 0, 0, time.UTC)) // = 23:30 PT (winter)

	httpIdle := presence.NewHTTPIdle(5 * time.Minute)
	httpIdle.NoteActivityAt(clk.Now()) // active right now
	sw, err := presence.NewSleepWindow(presence.SleepWindowConfig{
		Enabled:  true,
		Start:    "23:00",
		End:      "07:00",
		Timezone: "America/Los_Angeles",
	})
	require.NoError(t, err)
	comp := presence.NewComposite(httpIdle, sw)

	runs, ticks, cancel, done := startConsolidator(t, clk, comp)
	defer cancel()

	// HTTP Unknown + Sleep Absent ⇒ Available; runs.
	ticks <- clk.Now()
	waitForRuns(t, runs, 1)

	// Burst: 5 ticks at the same instant — each runs (no dedup).
	for i := 0; i < 5; i++ {
		ticks <- clk.Now()
	}
	waitForRuns(t, runs, 6)

	cancel()
	<-done
}

func TestLoopE2E_OutsideWindowRequiresHTTPQuiet(t *testing.T) {
	// Pacific: 14:30 local, outside the sleep window.
	clk := newFakeClock(time.Date(2026, 1, 2, 22, 30, 0, 0, time.UTC)) // = 14:30 PT (winter)

	httpIdle := presence.NewHTTPIdle(5 * time.Minute)
	httpIdle.NoteActivityAt(clk.Now())
	sw, err := presence.NewSleepWindow(presence.SleepWindowConfig{
		Enabled:  true,
		Start:    "23:00",
		End:      "07:00",
		Timezone: "America/Los_Angeles",
	})
	require.NoError(t, err)
	comp := presence.NewComposite(httpIdle, sw)

	runs, ticks, cancel, done := startConsolidator(t, clk, comp)
	defer cancel()

	// All Unknown (just-noted HTTP, outside sleep) → no run.
	ticks <- clk.Now()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int64(0), runs.Load())

	// Advance HTTP past threshold → Absent → run.
	clk.Advance(5 * time.Minute)
	ticks <- clk.Now()
	waitForRuns(t, runs, 1)

	cancel()
	<-done
}

func TestLoopE2E_HTTPThresholdFlip(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))
	httpIdle := presence.NewHTTPIdle(5 * time.Minute)
	httpIdle.NoteActivityAt(clk.Now())
	comp := presence.NewComposite(httpIdle)

	runs, ticks, cancel, done := startConsolidator(t, clk, comp)
	defer cancel()

	// Tick at 5m - 1ns → still Unknown.
	clk.Advance(5*time.Minute - time.Nanosecond)
	ticks <- clk.Now()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int64(0), runs.Load())

	// Advance 2ns → Absent → run.
	clk.Advance(2 * time.Nanosecond)
	ticks <- clk.Now()
	waitForRuns(t, runs, 1)

	cancel()
	<-done
}

func TestLoopE2E_PingPongActivityNeverFires(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))
	httpIdle := presence.NewHTTPIdle(5 * time.Minute)
	httpIdle.NoteActivityAt(clk.Now())
	comp := presence.NewComposite(httpIdle)

	runs, ticks, cancel, done := startConsolidator(t, clk, comp)
	defer cancel()

	// (Activity → +1m → tick) × 6: each tick is at 4m offset, never reaches 5m.
	for i := 0; i < 6; i++ {
		clk.Advance(time.Minute)
		httpIdle.NoteActivityAt(clk.Now())
		ticks <- clk.Now()
	}
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int64(0), runs.Load(), "pingpong activity must keep HTTP idle Unknown")

	cancel()
	<-done
}

func TestLoopE2E_CtxCancelExitsCleanly(t *testing.T) {
	clk := newFakeClock(time.Now())
	comp := presence.NewComposite(presence.NewHTTPIdle(time.Hour))
	_, _, cancel, done := startConsolidator(t, clk, comp)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runLoop did not exit within 1s of ctx cancellation")
	}
}

func TestLoopE2E_GoldenSnapshotMatchesBSpec(t *testing.T) {
	// With sleep window disabled and HTTP idle absentAfter=300s, the
	// composite reduces to "HTTP idle only" — bit-identical to B-spec.
	clk := newFakeClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))
	httpIdle := presence.NewHTTPIdle(5 * time.Minute)
	httpIdle.NoteActivityAt(clk.Now())
	comp := presence.NewComposite(httpIdle)

	runs, ticks, cancel, done := startConsolidator(t, clk, comp)
	defer cancel()

	// Replay the B-spec scenario: tick before threshold → no fire;
	// tick after threshold → fire.
	ticks <- clk.Now() // t=0, just noted → no
	time.Sleep(20 * time.Millisecond)
	clk.Advance(5 * time.Minute)
	ticks <- clk.Now() // t=5m → fires
	waitForRuns(t, runs, 1)
	clk.Advance(time.Minute)
	ticks <- clk.Now() // t=6m → fires again
	waitForRuns(t, runs, 2)

	cancel()
	<-done
}
