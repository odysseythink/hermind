package memorylayer

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStorage is a minimal in-memory storage.Storage for tests.
type fakeStorage struct {
	mems []*storage.Memory
}

func (f *fakeStorage) AppendMessage(ctx context.Context, m *storage.StoredMessage) error { return nil }
func (f *fakeStorage) GetHistory(ctx context.Context, limit, offset int) ([]*storage.StoredMessage, error) {
	return nil, nil
}
func (f *fakeStorage) UpdateMessage(ctx context.Context, id int64, content string) error { return nil }
func (f *fakeStorage) DeleteMessage(ctx context.Context, id int64) error { return nil }
func (f *fakeStorage) DeleteMessagesAfter(ctx context.Context, ts int64) error { return nil }
func (f *fakeStorage) UpdateSystemPromptCache(ctx context.Context, prompt string) error { return nil }
func (f *fakeStorage) UpdateUsage(ctx context.Context, u *storage.UsageUpdate) error { return nil }
func (f *fakeStorage) WithTx(ctx context.Context, fn func(storage.Tx) error) error { return nil }
func (f *fakeStorage) SaveMemory(ctx context.Context, m *storage.Memory) error { return nil }
func (f *fakeStorage) GetMemory(ctx context.Context, id string) (*storage.Memory, error) { return nil, nil }
func (f *fakeStorage) SearchMemories(ctx context.Context, query string, opts *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	var out []*storage.Memory
	for _, m := range f.mems {
		if opts != nil && opts.RankingMode == "fts_only" {
			// Simple substring match for test purposes
			if query == "" || contains(m.Content, query) {
				out = append(out, m)
			}
		} else if opts != nil && opts.RankingMode == "vector_only" {
			out = append(out, m)
		} else {
			out = append(out, m)
		}
	}
	if opts != nil && opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}
func (f *fakeStorage) DeleteMemory(ctx context.Context, id string) error { return nil }
func (f *fakeStorage) ListMemoriesByType(ctx context.Context, memType string, limit int) ([]*storage.Memory, error) {
	return nil, nil
}
func (f *fakeStorage) MarkMemorySuperseded(ctx context.Context, oldID, newID string) error { return nil }
func (f *fakeStorage) BumpMemoryUsage(ctx context.Context, id string, used bool) error { return nil }
func (f *fakeStorage) MemoryStats(ctx context.Context) (*storage.MemoryStats, error) { return nil, nil }
func (f *fakeStorage) MemoryHealth(ctx context.Context) (*storage.MemoryHealth, error) { return nil, nil }
func (f *fakeStorage) SearchMessages(ctx context.Context, query string, opts *storage.SearchOptions) ([]*storage.SearchResult, error) {
	return nil, nil
}
func (f *fakeStorage) AppendMemoryEvent(ctx context.Context, ts time.Time, kind string, data []byte) error {
	return nil
}
func (f *fakeStorage) ListMemoryEvents(ctx context.Context, limit, offset int, kinds []string) ([]*storage.MemoryEvent, error) {
	return nil, nil
}
func (f *fakeStorage) SetSkillsGeneration(ctx context.Context, hash string) (string, int64, int64, bool, error) {
	return "", 0, 0, false, nil
}
func (f *fakeStorage) GetSkillsGeneration(ctx context.Context) (*storage.SkillsGeneration, error) { return nil, nil }
func (f *fakeStorage) SkillsStats(ctx context.Context, skillsDir string) (*storage.SkillsStats, error) {
	return nil, nil
}
func (f *fakeStorage) DeleteMessageAndAfter(ctx context.Context, id int64) error { return nil }
func (f *fakeStorage) SaveFeedback(ctx context.Context, messageID int64, score int) error { return nil }
func (f *fakeStorage) SaveAttachment(ctx context.Context, msgID int64, name string, mimeType string, url string, size int64) error {
	return nil
}
func (f *fakeStorage) ListAttachments(ctx context.Context, msgID int64) ([]storage.Attachment, error) {
	return nil, nil
}
func (f *fakeStorage) Migrate() error { return nil }
func (f *fakeStorage) Close() error { return nil }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

type fakeEmbedder struct {
	vec []float32
}

func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return f.vec, nil
}

func TestRRFFuse_OverlappingLists(t *testing.T) {
	fts := []*storage.Memory{
		{ID: "A", Content: "a"},
		{ID: "B", Content: "b"},
	}
	vec := []*storage.Memory{
		{ID: "C", Content: "c"},
		{ID: "A", Content: "a"},
	}
	out := rrfFuse(fts, vec, 60)
	require.Len(t, out, 3)

	// A appears in both → highest score
	var scoreA, scoreB, scoreC float64
	for _, c := range out {
		switch c.ID {
		case "A":
			scoreA = c.Score
		case "B":
			scoreB = c.Score
		case "C":
			scoreC = c.Score
		}
	}
	assert.Greater(t, scoreA, scoreC, "A should outrank C")
	assert.Greater(t, scoreA, scoreB, "A should outrank B")
}

func TestHybridRecaller_FallsBackWhenEmbedderMissing(t *testing.T) {
	store := &fakeStorage{
		mems: []*storage.Memory{
			{ID: "m1", Content: "foo bar"},
		},
	}
	h := NewHybridRecaller(store, nil, nil, HybridConfig{})
	cands, err := h.Recall(context.Background(), "foo", 5)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, "m1", cands[0].ID)
}

func TestHybridRecaller_SignalBoost(t *testing.T) {
	fts := []*storage.Memory{
		{ID: "boosted", Content: "x", ReinforcementCount: 10, NeglectCount: 0},
		{ID: "neutral", Content: "y", ReinforcementCount: 0, NeglectCount: 0},
	}
	vec := []*storage.Memory{}
	out := rrfFuse(fts, vec, 60)
	h := NewHybridRecaller(nil, nil, nil, HybridConfig{ReinforcementAlpha: 0.5, NeglectPenalty: 0.5})
	h.applySignalBoost(out)
	require.Len(t, out, 2)
	assert.Greater(t, out[0].Score, out[1].Score, "boosted should have higher score")
	assert.Equal(t, "boosted", out[0].ID)
}

func TestHybridRecaller_ExternalOnlyPassthrough(t *testing.T) {
	base := &fakeRecaller{mems: []memprovider.InjectedMemory{
		{ID: "b1", Content: "c1"},
		{ID: "b2", Content: "c2"},
		{ID: "b3", Content: "c3"},
	}}
	h := NewHybridRecaller(nil, nil, base, HybridConfig{})
	cands, err := h.Recall(context.Background(), "q", 2)
	require.NoError(t, err)
	require.Len(t, cands, 3)
	assert.Equal(t, "base", cands[0].Source)
}

func TestHybridRecaller_BothSourcesFail(t *testing.T) {
	store := &failingStorage{err: assert.AnError}
	h := NewHybridRecaller(store, &fakeEmbedder{vec: []float32{1, 0}}, nil, HybridConfig{})
	cands, err := h.Recall(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Nil(t, cands)
}

type fakeRecaller struct {
	mems []memprovider.InjectedMemory
}

func (f *fakeRecaller) Recall(ctx context.Context, query string, limit int) ([]memprovider.InjectedMemory, error) {
	return f.mems, nil
}

type failingStorage struct {
	err error
}

func (f *failingStorage) AppendMessage(ctx context.Context, m *storage.StoredMessage) error { return f.err }
func (f *failingStorage) GetHistory(ctx context.Context, limit, offset int) ([]*storage.StoredMessage, error) {
	return nil, f.err
}
func (f *failingStorage) UpdateMessage(ctx context.Context, id int64, content string) error { return f.err }
func (f *failingStorage) DeleteMessage(ctx context.Context, id int64) error { return f.err }
func (f *failingStorage) DeleteMessagesAfter(ctx context.Context, ts int64) error { return f.err }
func (f *failingStorage) UpdateSystemPromptCache(ctx context.Context, prompt string) error { return f.err }
func (f *failingStorage) UpdateUsage(ctx context.Context, u *storage.UsageUpdate) error { return f.err }
func (f *failingStorage) WithTx(ctx context.Context, fn func(storage.Tx) error) error { return f.err }
func (f *failingStorage) SaveMemory(ctx context.Context, m *storage.Memory) error { return f.err }
func (f *failingStorage) GetMemory(ctx context.Context, id string) (*storage.Memory, error) {
	return nil, f.err
}
func (f *failingStorage) SearchMemories(ctx context.Context, query string, opts *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	return nil, f.err
}
func (f *failingStorage) DeleteMemory(ctx context.Context, id string) error { return f.err }
func (f *failingStorage) ListMemoriesByType(ctx context.Context, memType string, limit int) ([]*storage.Memory, error) {
	return nil, f.err
}
func (f *failingStorage) MarkMemorySuperseded(ctx context.Context, oldID, newID string) error { return f.err }
func (f *failingStorage) BumpMemoryUsage(ctx context.Context, id string, used bool) error { return f.err }
func (f *failingStorage) MemoryStats(ctx context.Context) (*storage.MemoryStats, error) { return nil, f.err }
func (f *failingStorage) MemoryHealth(ctx context.Context) (*storage.MemoryHealth, error) { return nil, f.err }
func (f *failingStorage) SearchMessages(ctx context.Context, query string, opts *storage.SearchOptions) ([]*storage.SearchResult, error) {
	return nil, f.err
}
func (f *failingStorage) AppendMemoryEvent(ctx context.Context, ts time.Time, kind string, data []byte) error {
	return f.err
}
func (f *failingStorage) ListMemoryEvents(ctx context.Context, limit, offset int, kinds []string) ([]*storage.MemoryEvent, error) {
	return nil, f.err
}
func (f *failingStorage) SetSkillsGeneration(ctx context.Context, hash string) (string, int64, int64, bool, error) {
	return "", 0, 0, false, f.err
}
func (f *failingStorage) GetSkillsGeneration(ctx context.Context) (*storage.SkillsGeneration, error) {
	return nil, f.err
}
func (f *failingStorage) SkillsStats(ctx context.Context, skillsDir string) (*storage.SkillsStats, error) {
	return nil, f.err
}
func (f *failingStorage) DeleteMessageAndAfter(ctx context.Context, id int64) error { return f.err }
func (f *failingStorage) SaveFeedback(ctx context.Context, messageID int64, score int) error { return f.err }
func (f *failingStorage) SaveAttachment(ctx context.Context, msgID int64, name string, mimeType string, url string, size int64) error {
	return f.err
}
func (f *failingStorage) ListAttachments(ctx context.Context, msgID int64) ([]storage.Attachment, error) {
	return nil, f.err
}
func (f *failingStorage) Migrate() error { return f.err }
func (f *failingStorage) Close() error { return nil }
