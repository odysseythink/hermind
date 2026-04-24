package memprovider_test

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// fakeStorage is a minimal in-memory storage for testing.
type fakeStorage struct {
	memories []*storage.Memory
}

func (f *fakeStorage) AppendMessage(_ context.Context, _ *storage.StoredMessage) error {
	return nil
}

func (f *fakeStorage) GetHistory(_ context.Context, _, _ int) ([]*storage.StoredMessage, error) {
	return nil, nil
}

func (f *fakeStorage) SearchMessages(_ context.Context, _ string, _ *storage.SearchOptions) ([]*storage.SearchResult, error) {
	return nil, nil
}

func (f *fakeStorage) UpdateSystemPromptCache(_ context.Context, _ string) error {
	return nil
}

func (f *fakeStorage) UpdateUsage(_ context.Context, _ *storage.UsageUpdate) error {
	return nil
}

func (f *fakeStorage) SaveMemory(_ context.Context, m *storage.Memory) error {
	f.memories = append(f.memories, m)
	return nil
}

func (f *fakeStorage) GetMemory(_ context.Context, id string) (*storage.Memory, error) {
	for _, m := range f.memories {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (f *fakeStorage) SearchMemories(_ context.Context, query string, opts *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	limit := 5
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	var out []*storage.Memory
	for _, m := range f.memories {
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStorage) DeleteMemory(_ context.Context, id string) error {
	return nil
}

func (f *fakeStorage) ListMemoriesByType(_ context.Context, memType string, limit int) ([]*storage.Memory, error) {
	var out []*storage.Memory
	for _, m := range f.memories {
		if m.MemType == memType {
			out = append(out, m)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStorage) WithTx(_ context.Context, fn func(storage.Tx) error) error {
	return fn(&fakeTx{})
}

func (f *fakeStorage) Close() error {
	return nil
}

func (f *fakeStorage) Migrate() error {
	return nil
}

func (f *fakeStorage) MarkMemorySuperseded(_ context.Context, oldID, newID string) error {
	for _, m := range f.memories {
		if m.ID == oldID {
			m.Status = storage.MemoryStatusSuperseded
			m.SupersededBy = newID
			return nil
		}
	}
	return storage.ErrNotFound
}

func (f *fakeStorage) BumpMemoryUsage(_ context.Context, id string, used bool) error {
	for _, m := range f.memories {
		if m.ID == id {
			if used {
				m.ReinforcementCount++
				m.LastUsedAt = time.Now().UTC()
			} else {
				m.NeglectCount++
			}
			return nil
		}
	}
	return storage.ErrNotFound
}

// fakeTx implements the Tx interface for testing.
type fakeTx struct{}

func (ft *fakeTx) AppendMessage(_ context.Context, _ *storage.StoredMessage) error {
	return nil
}

func (ft *fakeTx) UpdateSystemPromptCache(_ context.Context, _ string) error {
	return nil
}

func (ft *fakeTx) UpdateUsage(_ context.Context, _ *storage.UsageUpdate) error {
	return nil
}

func TestMetaClawName(t *testing.T) {
	p := memprovider.NewMetaClaw(&fakeStorage{}, nil, nil)
	if p.Name() != "metaclaw" {
		t.Errorf("Name: want metaclaw, got %q", p.Name())
	}
}

func TestMetaClawSyncTurnNoLLM(t *testing.T) {
	store := &fakeStorage{}
	p := memprovider.NewMetaClaw(store, nil, nil)
	_ = p.Initialize(context.Background(), "sess1")
	err := p.SyncTurn(context.Background(), "hello", "world")
	if err != nil {
		t.Fatalf("SyncTurn without LLM: %v", err)
	}
	if len(store.memories) != 0 {
		t.Errorf("expected no memories written without LLM, got %d", len(store.memories))
	}
}
