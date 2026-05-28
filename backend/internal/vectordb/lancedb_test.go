//go:build !windows && !nolancedb
// +build !windows,!nolancedb

package vectordb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLanceDB_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	db := NewLanceDB(tmpDir)
	ctx := context.Background()

	err := db.Connect(ctx)
	require.NoError(t, err)

	// Add vectors
	chunks := []VectorChunk{
		{ID: "v1", Vector: []float32{0.1, 0.2, 0.3}, Metadata: map[string]any{"docId": "d1", "text": "hello"}},
		{ID: "v2", Vector: []float32{0.4, 0.5, 0.6}, Metadata: map[string]any{"docId": "d1", "text": "world"}},
	}
	err = db.AddVectors(ctx, "test-ns", chunks)
	require.NoError(t, err)

	// Search
	results, err := db.SimilaritySearch(ctx, "test-ns", []float32{0.1, 0.2, 0.3}, SearchOptions{TopN: 2, SimilarityThreshold: 0.0})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Delete vectors
	err = db.DeleteVectors(ctx, "test-ns", []string{"v1"})
	require.NoError(t, err)

	// Verify deletion
	results, err = db.SimilaritySearch(ctx, "test-ns", []float32{0.1, 0.2, 0.3}, SearchOptions{TopN: 2, SimilarityThreshold: 0.0})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Tables
	tables, err := db.Tables(ctx)
	require.NoError(t, err)
	require.Contains(t, tables, "test-ns")

	// Total vectors
	count, err := db.TotalVectors(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	// Delete namespace
	err = db.DeleteNamespace(ctx, "test-ns")
	require.NoError(t, err)
}
