// tool/memory/memory_test.go
package memory

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSetup(t *testing.T) (*tool.Registry, *sqlite.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	t.Cleanup(func() { _ = store.Close() })

	reg := tool.NewRegistry()
	RegisterAll(reg, store)
	return reg, store
}

func TestMemorySaveAndSearch(t *testing.T) {
	reg, _ := newTestSetup(t)
	ctx := context.Background()

	// Save
	args := json.RawMessage(`{"content":"the user prefers Go","tags":["preference","lang"]}`)
	out, err := reg.Dispatch(ctx, "memory_save", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"id"`)

	// Search
	searchArgs := json.RawMessage(`{"query":"Go"}`)
	out2, err := reg.Dispatch(ctx, "memory_search", searchArgs)
	require.NoError(t, err)

	var result memorySearchResult
	require.NoError(t, json.Unmarshal([]byte(out2), &result))
	require.Len(t, result.Results, 1)
	assert.Contains(t, result.Results[0].Content, "prefers Go")
}

func TestMemoryDeleteRemovesEntry(t *testing.T) {
	reg, store := newTestSetup(t)
	ctx := context.Background()

	out, err := reg.Dispatch(ctx, "memory_save", json.RawMessage(`{"content":"x"}`))
	require.NoError(t, err)
	var saved memorySaveResult
	require.NoError(t, json.Unmarshal([]byte(out), &saved))

	delArgs := json.RawMessage(`{"id":"` + saved.ID + `"}`)
	_, err = reg.Dispatch(ctx, "memory_delete", delArgs)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.GetMemory(ctx, saved.ID)
	assert.Error(t, err)
}

func TestMemorySaveRequiresContent(t *testing.T) {
	reg, _ := newTestSetup(t)
	out, err := reg.Dispatch(context.Background(), "memory_save", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "content")
}

func TestMemoryDeleteRequiresID(t *testing.T) {
	reg, _ := newTestSetup(t)
	out, err := reg.Dispatch(context.Background(), "memory_delete", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "id")
}
