package tools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCitation_JSONShape(t *testing.T) {
	c := Citation{
		ID:          "https://example.com",
		Title:       "Example",
		Text:        "A snippet",
		ChunkSource: "link://https://example.com",
	}
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "https://example.com", got["id"])
	require.Equal(t, "Example", got["title"])
	require.Equal(t, "A snippet", got["text"])
	require.Equal(t, "link://https://example.com", got["chunkSource"])
	_, hasScore := got["score"]
	require.False(t, hasScore, "score is omitempty so nil score should not appear")
}
