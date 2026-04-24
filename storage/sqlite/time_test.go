package sqlite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEpochRoundtrip_Zero(t *testing.T) {
	assert.Equal(t, 0.0, toEpoch(time.Time{}))
	assert.True(t, fromEpoch(0.0).IsZero())
}

func TestEpochRoundtrip_NonZero(t *testing.T) {
	ts := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	got := fromEpoch(toEpoch(ts))
	// Round-trip allows for nanosecond quantization jitter; 1µs is plenty.
	assert.WithinDuration(t, ts, got, time.Microsecond)
}
