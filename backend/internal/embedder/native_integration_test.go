//go:build integration

package embedder

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeEmbedder_Integration(t *testing.T) {
	cfg := &config.Config{
		StorageDir:           t.TempDir(),
		NativeEmbeddingModel: "sentence-transformers/all-MiniLM-L6-v2",
	}

	emb, err := NewNativeEmbedder(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Test EmbedQuery
	vec, err := emb.EmbedQuery(ctx, "hello world")
	require.NoError(t, err)
	assert.Len(t, vec, 384)

	// Test EmbedTexts
	vecs, err := emb.EmbedTexts(ctx, []string{"first text", "second text"})
	require.NoError(t, err)
	assert.Len(t, vecs, 2)
	assert.Len(t, vecs[0], 384)
	assert.Len(t, vecs[1], 384)

	// Test Dimensions
	assert.Equal(t, 384, emb.Dimensions())
}
