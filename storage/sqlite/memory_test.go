// storage/sqlite/memory_test.go
package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/embedding"
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

func TestSaveAndGetMemoryWithTypeAndVector(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	vec := []float32{0.1, 0.2, 0.3}
	encoded, err := embedding.EncodeVector(vec)
	require.NoError(t, err)

	m := &storage.Memory{
		ID:        "mc_001",
		Content:   "user prefers Go over Python",
		MemType:   "preference",
		Vector:    encoded,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, s.SaveMemory(ctx, m))

	got, err := s.GetMemory(ctx, "mc_001")
	require.NoError(t, err)
	assert.Equal(t, "preference", got.MemType)
	assert.NotEmpty(t, got.Vector)

	// Verify vector can be decoded
	decodedVec, err := embedding.DecodeVector(got.Vector)
	require.NoError(t, err)
	assert.Equal(t, 3, len(decodedVec))
	assert.InDelta(t, 0.1, decodedVec[0], 0.001)
	assert.InDelta(t, 0.2, decodedVec[1], 0.001)
	assert.InDelta(t, 0.3, decodedVec[2], 0.001)
}

func TestListMemoriesByType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i, memType := range []string{"episodic", "semantic", "preference", "episodic"} {
		_ = s.SaveMemory(ctx, &storage.Memory{
			ID:        fmt.Sprintf("m%d", i),
			Content:   fmt.Sprintf("content %d", i),
			MemType:   memType,
			CreatedAt: time.Now().UTC().Add(time.Duration(i) * time.Second),
			UpdatedAt: time.Now().UTC(),
		})
	}

	mems, err := s.ListMemoriesByType(ctx, "episodic", 10)
	require.NoError(t, err)
	require.Len(t, mems, 2)

	// Verify order (newest first)
	assert.Equal(t, "m3", mems[0].ID)
	assert.Equal(t, "m0", mems[1].ID)
}

func TestSaveAndGetMemory_LastUsedAtZeroRoundtrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID:        "mem-zero",
		Content:   "never used",
		CreatedAt: now,
		UpdatedAt: now,
		// LastUsedAt left zero
	}))

	got, err := store.GetMemory(ctx, "mem-zero")
	require.NoError(t, err)
	assert.True(t, got.LastUsedAt.IsZero(), "LastUsedAt should round-trip as zero time")
}
