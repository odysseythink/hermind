package integration

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGVectorAddAndSearch(t *testing.T) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		t.Skip("DATABASE_URL not set, skipping PGVector test")
	}

	pg := vectordb.NewPGVector(connStr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := pg.Connect(ctx)
	require.NoError(t, err)

	ns := "test-workspace-" + uuid.New().String()
	chunks := []vectordb.VectorChunk{
		{ID: uuid.New().String(), Vector: []float32{1, 0, 0}, Metadata: map[string]any{"text": "hello world", "docId": "doc1"}},
		{ID: uuid.New().String(), Vector: []float32{0, 1, 0}, Metadata: map[string]any{"text": "foo bar", "docId": "doc1"}},
		{ID: uuid.New().String(), Vector: []float32{0, 0, 1}, Metadata: map[string]any{"text": "baz qux", "docId": "doc2"}},
	}

	err = pg.AddVectors(ctx, ns, chunks)
	require.NoError(t, err)

	results, err := pg.SimilaritySearch(ctx, ns, []float32{1, 0, 0}, vectordb.SearchOptions{TopN: 2, SimilarityThreshold: 0.0})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "hello world", results[0].Text)

	// Cleanup
	_ = pg.DeleteNamespace(ctx, ns)
}

func TestLanceDBConnect(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("lancedb CGo bindings not available on Windows")
	}
	dir := t.TempDir()
	ldb := vectordb.NewLanceDB(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ldb.Connect(ctx)
	if err != nil && err.Error() == "lancedb: stub build" {
		t.Skip("lancedb not available in this build")
	}
	require.NoError(t, err)

	tables, err := ldb.Tables(ctx)
	require.NoError(t, err)
	assert.Empty(t, tables)
}
