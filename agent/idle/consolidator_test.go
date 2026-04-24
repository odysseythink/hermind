package idle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIdleConsolidator_RespectsDisabled(t *testing.T) {
	c := &IdleConsolidator{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Start(ctx) // must not block, must not call anything
}

func TestIdleConsolidator_NoteActivityTriggersAtInterval(t *testing.T) {
	var called atomic.Int32
	c := &IdleConsolidator{
		interval:  50 * time.Millisecond,
		idleAfter: 10 * time.Millisecond,
		runFn:     func(context.Context) { called.Add(1) },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Start(ctx)

	// Activity at t=0; threshold is met after 10ms, so the next tick
	// at 50ms should fire.
	c.NoteActivity()
	time.Sleep(150 * time.Millisecond)
	assert.GreaterOrEqual(t, called.Load(), int32(1))
}

func TestIdleConsolidator_ActiveTrafficSuppressesRun(t *testing.T) {
	var called atomic.Int32
	c := &IdleConsolidator{
		interval:  50 * time.Millisecond,
		idleAfter: 200 * time.Millisecond,
		runFn:     func(context.Context) { called.Add(1) },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Start(ctx)

	// Constant activity — idleAfter threshold never elapses.
	for i := 0; i < 10; i++ {
		c.NoteActivity()
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, int32(0), called.Load())
}
