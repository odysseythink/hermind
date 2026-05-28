package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokenCount(t *testing.T) {
	assert.Equal(t, 0, EstimateTokenCount(""))
	// ASCII: ~4 chars per token. "Hello world test" = 16 chars → 4 tokens.
	assert.Equal(t, 4, EstimateTokenCount("Hello world test"))
	// CJK: ~1 char per token. "你好世界" = 4 chars → 4 tokens.
	assert.Equal(t, 4, EstimateTokenCount("你好世界"))
	// Mixed
	assert.True(t, EstimateTokenCount("Hello 你好") > 0)
}
