// storage/sqlite/memory_test.go
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndGetMemory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	mem := &storage.Memory{
		ID:        "mem-001",
		UserID:    "user-1",
		Content:   "The user prefers Go over Rust",
		Category:  "preference",
		Tags:      []string{"language", "go"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.SaveMemory(ctx, mem))

	got, err := store.GetMemory(ctx, "mem-001")
	require.NoError(t, err)
	assert.Equal(t, "mem-001", got.ID)
	assert.Equal(t, "The user prefers Go over Rust", got.Content)
	assert.Contains(t, got.Tags, "go")
}

func TestGetMemoryNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	_, err := store.GetMemory(ctx, "nonexistent")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSearchMemoriesFTS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m1", Content: "The quick brown fox", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m2", Content: "Lazy dogs sleep", CreatedAt: now, UpdatedAt: now,
	}))

	results, err := store.SearchMemories(ctx, "fox", &storage.MemorySearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "m1", results[0].ID)
}

func TestSearchMemoriesEmptyQueryListsRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m1", Content: "a", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m2", Content: "b", CreatedAt: now.Add(time.Minute), UpdatedAt: now,
	}))

	results, err := store.SearchMemories(ctx, "", &storage.MemorySearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "m2", results[0].ID) // most recent first
}

func TestDeleteMemory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "delete-me", Content: "bye", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.DeleteMemory(ctx, "delete-me"))

	_, err := store.GetMemory(ctx, "delete-me")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDeleteMemoryNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	err := store.DeleteMemory(ctx, "nonexistent")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
