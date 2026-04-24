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
