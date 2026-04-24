package memprovider_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
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
	// Upsert: replace by ID if exists, otherwise append
	for i, existing := range f.memories {
		if existing.ID == m.ID {
			f.memories[i] = m
			return nil
		}
	}
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

func TestMetaClawRecallReturnsInjectedMemory(t *testing.T) {
	store := &fakeStorage{}
	now := time.Now().UTC()
	require.NoError(t, store.SaveMemory(context.Background(), &storage.Memory{
		ID: "mc_abc", Content: "likes Go", MemType: "preference",
		CreatedAt: now, UpdatedAt: now,
	}))

	mc := memprovider.NewMetaClaw(store, nil, nil)
	got, err := mc.Recall(context.Background(), "go", 5)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "mc_abc", got[0].ID)
	assert.Equal(t, "likes Go", got[0].Content)
}

func TestMetaClawSyncTurnAppendsToRingBuffer(t *testing.T) {
	mc := memprovider.NewMetaClaw(&fakeStorage{}, nil, nil)
	require.NoError(t, mc.Initialize(context.Background(), "sess"))

	for i := 0; i < 25; i++ {
		require.NoError(t, mc.SyncTurn(context.Background(),
			fmt.Sprintf("u%d", i), fmt.Sprintf("a%d", i)))
	}

	buf := mc.RecentBufferSnapshot()
	require.Len(t, buf, 20, "ring buffer should cap at 20")
	// Oldest kept entry should be turn #5 (0..4 pushed out).
	assert.Equal(t, "u5", buf[0].User)
	assert.Equal(t, "a24", buf[len(buf)-1].Assistant)
}

type stubLLM struct {
	lastSystemPrompt string
	reply            string
}

func (s *stubLLM) Name() string { return "stub" }
func (s *stubLLM) Available() bool { return true }
func (s *stubLLM) Complete(_ context.Context, req *provider.Request) (*provider.Response, error) {
	s.lastSystemPrompt = req.SystemPrompt
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(s.reply),
		},
	}, nil
}
func (s *stubLLM) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	panic("stub: Stream not used")
}
func (s *stubLLM) ModelInfo(model string) *provider.ModelInfo {
	return nil
}
func (s *stubLLM) EstimateTokens(model string, text string) (int, error) {
	return 0, nil
}

func TestMetaClawRefreshWorkingSummaryOnThreshold(t *testing.T) {
	store := &fakeStorage{}
	llm := &stubLLM{reply: "Rolling summary: user working on X."}
	mc := memprovider.NewMetaClaw(store, llm, nil)
	mc.SetSummaryEvery(3)
	require.NoError(t, mc.Initialize(context.Background(), "sess"))

	for i := 0; i < 3; i++ {
		require.NoError(t, mc.SyncTurn(context.Background(),
			fmt.Sprintf("u%d", i), fmt.Sprintf("a%d", i)))
	}
	// Give the goroutine a moment to run.
	for i := 0; i < 50; i++ {
		if _, err := store.GetMemory(context.Background(), "working_summary"); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := store.GetMemory(context.Background(), "working_summary")
	require.NoError(t, err)
	assert.Equal(t, "Rolling summary: user working on X.", got.Content)
	assert.Equal(t, storage.MemTypeWorkingSummary, got.MemType)
}
