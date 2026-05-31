package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestL2Normalize(t *testing.T) {
	// Unit vector should stay unit
	v1 := []float32{1, 0, 0}
	result := l2Normalize(v1)
	assert.InDelta(t, float32(1.0), result[0], 0.0001)
	assert.InDelta(t, float32(0.0), result[1], 0.0001)

	// Simple vector
	v2 := []float32{3, 4}
	result2 := l2Normalize(v2)
	assert.InDelta(t, float32(0.6), result2[0], 0.0001)
	assert.InDelta(t, float32(0.8), result2[1], 0.0001)

	// Zero vector should not panic
	v3 := []float32{0, 0, 0}
	result3 := l2Normalize(v3)
	assert.Equal(t, []float32{0, 0, 0}, result3)
}

func TestNativeEmbedderDimensions(t *testing.T) {
	e := &NativeEmbedder{dims: 384}
	assert.Equal(t, 384, e.Dimensions())
}
