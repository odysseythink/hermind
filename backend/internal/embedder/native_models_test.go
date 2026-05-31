package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailableModels(t *testing.T) {
	models := AvailableModels()
	require.Len(t, models, 3, "expected 3 native models")

	ids := make(map[string]bool)
	for _, m := range models {
		id, ok := m["id"].(string)
		require.True(t, ok, "model id should be string")
		assert.NotEmpty(t, id, "model id should not be empty")
		assert.NotContains(t, ids, id, "duplicate model id: %s", id)
		ids[id] = true

		assert.NotEmpty(t, m["name"], "model %s name should not be empty", id)
		assert.NotEmpty(t, m["description"], "model %s description should not be empty", id)
		assert.NotEmpty(t, m["lang"], "model %s lang should not be empty", id)
		assert.NotEmpty(t, m["size"], "model %s size should not be empty", id)
		assert.NotEmpty(t, m["modelCard"], "model %s modelCard should not be empty", id)
	}

	assert.True(t, ids["sentence-transformers/all-MiniLM-L6-v2"], "default model should be present")
}

func TestGetNativeModelInfo(t *testing.T) {
	// Valid model
	info, ok := getNativeModelInfo("sentence-transformers/all-MiniLM-L6-v2")
	require.True(t, ok)
	assert.Equal(t, 384, info.Dimensions)
	assert.Equal(t, "sentence-transformers/all-MiniLM-L6-v2", info.HFRepo)
	assert.Greater(t, info.MaxConcurrentChunks, 0)
	assert.Greater(t, info.EmbeddingMaxChunkLength, 0)

	// Invalid model falls back to default
	info, ok = getNativeModelInfo("nonexistent-model")
	require.True(t, ok, "should fall back to default model")
	assert.Equal(t, "sentence-transformers/all-MiniLM-L6-v2", info.ID)
}
