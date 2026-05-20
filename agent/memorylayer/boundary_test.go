package memorylayer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEmbedder struct {
	vecs map[string][]float32
}

func (s *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if v, ok := s.vecs[text]; ok {
		return v, nil
	}
	return []float32{1, 0, 0}, nil
}

func TestBoundary_HardTokenLimit(t *testing.T) {
	d := NewBoundaryDetector(BoundaryConfig{HardTokenLimit: 100}, nil)
	ctx := context.Background()

	b := d.Observe(ctx, Turn{ID: 1, Tokens: 50})
	assert.Nil(t, b)

	b = d.Observe(ctx, Turn{ID: 2, Tokens: 60})
	require.NotNil(t, b)
	assert.Equal(t, "hard_token", b.Reason)
	assert.Len(t, b.Turns, 2)
	assert.Equal(t, 110, b.TokenCount)

	// Buffer is reset
	assert.Nil(t, d.Flush())
}

func TestBoundary_HardTurnLimit(t *testing.T) {
	d := NewBoundaryDetector(BoundaryConfig{HardTurnLimit: 3}, nil)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		b := d.Observe(ctx, Turn{ID: int64(i), Tokens: 1})
		if i < 3 {
			assert.Nil(t, b)
		} else {
			require.NotNil(t, b)
			assert.Equal(t, "hard_turn", b.Reason)
			assert.Len(t, b.Turns, 3)
		}
	}
}

func TestBoundary_IdleGap(t *testing.T) {
	d := NewBoundaryDetector(BoundaryConfig{IdleGap: 10 * time.Minute}, nil)
	ctx := context.Background()

	now := time.Now().UTC()
	b := d.Observe(ctx, Turn{ID: 1, Tokens: 10, Timestamp: now})
	assert.Nil(t, b)

	b = d.Observe(ctx, Turn{ID: 2, Tokens: 10, Timestamp: now.Add(11 * time.Minute)})
	require.NotNil(t, b)
	assert.Equal(t, "idle", b.Reason)
	assert.Len(t, b.Turns, 1)
	assert.Equal(t, int64(1), b.Turns[0].ID)

	// New buffer started with turn 2
	f := d.Flush()
	require.NotNil(t, f)
	assert.Len(t, f.Turns, 1)
	assert.Equal(t, int64(2), f.Turns[0].ID)
}

func TestBoundary_TopicShift(t *testing.T) {
	emb := &stubEmbedder{vecs: map[string][]float32{
		"topic A": {1, 0, 0},
		"topic B": {0, 1, 0},
	}}
	d := NewBoundaryDetector(BoundaryConfig{
		SoftTokenThreshold:        10,
		EnableTopicShift:          true,
		TopicShiftCosineThreshold: 0.55,
	}, emb)
	ctx := context.Background()

	d.Observe(ctx, Turn{ID: 1, UserMsg: "topic A", Tokens: 10, Timestamp: time.Now().UTC()})
	b := d.Observe(ctx, Turn{ID: 2, UserMsg: "topic B", Tokens: 10, Timestamp: time.Now().UTC()})
	require.NotNil(t, b)
	assert.Equal(t, "topic_shift", b.Reason)
}

func TestBoundary_TopicShiftDisabledBelowSoftThreshold(t *testing.T) {
	emb := &stubEmbedder{vecs: map[string][]float32{
		"topic A": {1, 0, 0},
		"topic B": {0, 1, 0},
	}}
	d := NewBoundaryDetector(BoundaryConfig{
		SoftTokenThreshold:        100,
		EnableTopicShift:          true,
		TopicShiftCosineThreshold: 0.55,
	}, emb)
	ctx := context.Background()

	d.Observe(ctx, Turn{ID: 1, UserMsg: "topic A", Tokens: 5, Timestamp: time.Now().UTC()})
	b := d.Observe(ctx, Turn{ID: 2, UserMsg: "topic B", Tokens: 5, Timestamp: time.Now().UTC()})
	assert.Nil(t, b)
}

func TestBoundary_Flush(t *testing.T) {
	d := NewBoundaryDetector(BoundaryConfig{}, nil)
	ctx := context.Background()

	assert.Nil(t, d.Flush())
	d.Observe(ctx, Turn{ID: 1, Tokens: 10})
	f := d.Flush()
	require.NotNil(t, f)
	assert.Equal(t, "flush", f.Reason)
	assert.Len(t, f.Turns, 1)
}

func TestBoundary_NoEmbedderSilentlySkipsShift(t *testing.T) {
	d := NewBoundaryDetector(BoundaryConfig{
		SoftTokenThreshold:        10,
		EnableTopicShift:          true,
		TopicShiftCosineThreshold: 0.55,
	}, nil)
	ctx := context.Background()

	d.Observe(ctx, Turn{ID: 1, UserMsg: "a", Tokens: 10, Timestamp: time.Now().UTC()})
	b := d.Observe(ctx, Turn{ID: 2, UserMsg: "b", Tokens: 10, Timestamp: time.Now().UTC()})
	assert.Nil(t, b)
}
