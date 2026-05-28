package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizer_Count_ShortInput(t *testing.T) {
	tok, err := NewTokenizer()
	require.NoError(t, err)

	count := tok.Count("hello world")
	assert.Greater(t, count, 0)
}

func TestTokenizer_Count_LongInput(t *testing.T) {
	tok, err := NewTokenizer()
	require.NoError(t, err)

	longInput := strings.Repeat("a", 20*1024)
	count := tok.Count(longInput)
	// For large inputs we fall back to heuristic: (len + 7) / 8
	expected := (len(longInput) + 7) / 8
	assert.Equal(t, expected, count)
}
