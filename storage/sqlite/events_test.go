package sqlite

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendAndListMemoryEvents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	data1, _ := json.Marshal(map[string]int{"scanned": 10})
	data2, _ := json.Marshal(map[string]string{"outcome": "success"})
	require.NoError(t, store.AppendMemoryEvent(ctx, now, "memory.consolidated", data1))
	require.NoError(t, store.AppendMemoryEvent(ctx, now.Add(time.Second), "conversation.judged", data2))
	require.NoError(t, store.AppendMemoryEvent(ctx, now.Add(2*time.Second), "memory.consolidated", data1))

	all, err := store.ListMemoryEvents(ctx, 10, 0, nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	// ts DESC: newest first
	assert.Equal(t, "memory.consolidated", all[0].Kind)
	// Verify type
	assert.IsType(t, (*storage.MemoryEvent)(nil), all[0])

	filtered, err := store.ListMemoryEvents(ctx, 10, 0, []string{"conversation.judged"})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "conversation.judged", filtered[0].Kind)

	paged, err := store.ListMemoryEvents(ctx, 1, 1, nil)
	require.NoError(t, err)
	require.Len(t, paged, 1)
}

func TestMemoryStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m1", Content: "c1", MemType: storage.MemTypeEpisodic, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m2", Content: "c2", MemType: storage.MemTypeSemantic, CreatedAt: now, UpdatedAt: now,
	}))

	s, err := store.MemoryStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, s.Total)
	assert.Equal(t, 1, s.ByType[storage.MemTypeEpisodic])
	assert.Equal(t, 1, s.ByType[storage.MemTypeSemantic])
	assert.Equal(t, 2, s.ByStatus[storage.MemoryStatusActive])
}

func TestMemoryHealth(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	h, err := store.MemoryHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, 9, h.SchemaVersion)
	assert.False(t, h.MigrationsPending)
	assert.Equal(t, "ok", h.FTSIntegrity)
}
