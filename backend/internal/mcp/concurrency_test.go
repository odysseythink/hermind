package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcurrencyLimiter_AcquireRelease(t *testing.T) {
	l := newConcurrencyLimiter(4)
	for i := 0; i < 4; i++ {
		assert.True(t, l.TryAcquire("srv"), "acquire %d", i)
	}
	for i := 0; i < 4; i++ {
		l.Release("srv")
	}
	for i := 0; i < 4; i++ {
		assert.True(t, l.TryAcquire("srv"), "re-acquire %d", i)
	}
}

func TestConcurrencyLimiter_FifthBlocks_NonBlockingTry(t *testing.T) {
	l := newConcurrencyLimiter(4)
	for i := 0; i < 4; i++ {
		assert.True(t, l.TryAcquire("srv"))
	}
	assert.False(t, l.TryAcquire("srv"))
}

func TestConcurrencyLimiter_PerServerIsolation(t *testing.T) {
	l := newConcurrencyLimiter(4)
	for i := 0; i < 4; i++ {
		assert.True(t, l.TryAcquire("A"))
	}
	for i := 0; i < 4; i++ {
		assert.True(t, l.TryAcquire("B"))
	}
	assert.False(t, l.TryAcquire("A"))
	assert.False(t, l.TryAcquire("B"))
}

func TestConcurrencyLimiter_PerServerOverride(t *testing.T) {
	l := newConcurrencyLimiter(4)
	l.SetOverride("heavy", 1)

	assert.True(t, l.TryAcquire("heavy"))
	assert.False(t, l.TryAcquire("heavy"))

	// "light" still allows 4
	for i := 0; i < 4; i++ {
		assert.True(t, l.TryAcquire("light"))
	}
	assert.False(t, l.TryAcquire("light"))
}

func TestConcurrencyLimiter_ReleaseUnknown_Noop(t *testing.T) {
	l := newConcurrencyLimiter(4)
	// Should not panic
	l.Release("never-acquired")
	assert.True(t, l.TryAcquire("never-acquired"))
}
