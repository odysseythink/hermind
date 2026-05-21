package memprovider_test

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStorageWithEvents embeds fakeStorage and captures AppendMemoryEvent calls
// so tests can assert on events written during consolidation.
type fakeStorageWithEvents struct {
	fakeStorage
	events []struct {
		kind string
		data []byte
	}
}

func (f *fakeStorageWithEvents) AppendMemoryEvent(_ context.Context, _ time.Time, kind string, data []byte) error {
	f.events = append(f.events, struct {
		kind string
		data []byte
	}{kind, data})
	return nil
}

func TestConsolidateWritesEvent(t *testing.T) {
	store := &fakeStorageWithEvents{}
	now := time.Now().UTC()
	require.NoError(t, store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "hello", MemType: "episodic", CreatedAt: now, UpdatedAt: now,
	}))

	_, err := memprovider.Consolidate(context.Background(), store, nil)
	require.NoError(t, err)
	require.NotEmpty(t, store.events)
	assert.Equal(t, "memory.consolidated", store.events[0].kind)
}

func TestConsolidate_ArchivesExpiredForesights(t *testing.T) {
	store := &fakeStorageWithEvents{}
	ctx := context.Background()
	now := time.Now().UTC()

	// Expired foresight.
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "f1", Content: "report due monday", MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(-time.Hour), CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour),
	}))
	// Future foresight.
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "f2", Content: "demo next friday", MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(72 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}))

	rep, err := memprovider.Consolidate(ctx, store, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Archived)

	// Verify states.
	all, _ := store.ListMemoriesByType(ctx, "foresight", 100)
	var archived, active int
	for _, m := range all {
		if m.Status == storage.MemoryStatusArchived {
			archived++
		}
		if m.Status == storage.MemoryStatusActive {
			active++
		}
	}
	assert.Equal(t, 1, archived)
	assert.Equal(t, 1, active)

	// Verify event.
	var found bool
	for _, e := range store.events {
		if e.kind == "memory.foresight_archived" {
			found = true
			assert.Contains(t, string(e.data), `"archived":1`)
		}
	}
	assert.True(t, found, "expected memory.foresight_archived event")
}
