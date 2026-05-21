package memorylayer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/odysseythink/hermind/agent/memorylayer"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	calls int
}

func (s *stubLLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	s.calls++
	if s.calls == 1 {
		// taxonomy extraction
		return &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `[{"type":"fact","content":"Go modules are used","confidence":0.9}]`},
		}}}, nil
	}
	// rerank
	return &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `[]`},
	}}}, nil
}

func (s *stubLLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubLLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubLLM) Provider() string { return "stub" }
func (s *stubLLM) Model() string    { return "stub-model" }

type stubEmbedder struct{}

func (s *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{1, 0, 0}, nil
}

func TestIntegration_HybridRecall_AfterBoundary(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	llm := &stubLLM{}
	emb := &stubEmbedder{}

	layer := memorylayer.New(store, emb, nil, llm, memorylayer.Config{
		Hybrid:   memorylayer.HybridConfig{},
		Reranker: memorylayer.RerankerConfig{Enabled: true, BatchSize: 20, Timeout: time.Second},
		Boundary: memorylayer.BoundaryConfig{HardTurnLimit: 5, EnableTopicShift: false},
		Taxonomy: memorylayer.TaxonomyConfig{Enabled: true, MaxOutputs: 4, Timeout: time.Second},
		RecallLimit: 3,
	})

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		layer.ObserveTurn(ctx, memorylayer.Turn{
			ID:        int64(i),
			UserMsg:   fmt.Sprintf("question %d about Go modules", i),
			Assistant: "answer",
			Tokens:    50,
			Timestamp: time.Now(),
		})
	}
	// Boundary fires on the 5th turn; extractor runs in goroutine.
	// Give it a moment to drain (test stub LLM is synchronous).
	time.Sleep(50 * time.Millisecond)

	// Assertions:
	mems, _ := store.SearchMemories(ctx, "Go modules", &storage.MemorySearchOptions{Limit: 10})
	if len(mems) == 0 {
		t.Fatal("expected extracted memories")
	}
	for _, m := range mems {
		if m.ParentTurnID == 0 {
			t.Errorf("memory %q missing ParentTurnID", m.ID)
		}
	}

	out, _ := layer.Recall(ctx, "Go modules", 3)
	if len(out) == 0 {
		t.Fatal("expected recall hits")
	}

	events, _ := store.ListMemoryEvents(ctx, 10, 0, []string{"boundary.detected"})
	if len(events) == 0 {
		t.Error("expected boundary.detected event")
	}
}

// multiCallLLM returns a different response on each call based on a counter.
type multiCallLLM struct {
	calls int
}

func (m *multiCallLLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	m.calls++
	switch m.calls {
	case 1:
		// taxonomy extraction
		return &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `[]`},
		}}}, nil
	case 2:
		// profile update
		return &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `{"adds":[{"kind":"explicit","key":"diet.restrictions","value":"peanuts","evidence":"I'm allergic to peanuts","source_turns":[1],"confidence":0.9}],"updates":[],"deletes":[]}`},
		}}}, nil
	default:
		// reranker / anything else
		return &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `[]`},
		}}}, nil
	}
}
func (m *multiCallLLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) { return nil, fmt.Errorf("not implemented") }
func (m *multiCallLLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *multiCallLLM) Provider() string { return "stub" }
func (m *multiCallLLM) Model() string    { return "stub-model" }

func TestIntegration_ProfileRoundtrip(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()

	llm := &multiCallLLM{}
	layer := memorylayer.New(store, &stubEmbedder{}, nil, llm, memorylayer.Config{
		Boundary: memorylayer.BoundaryConfig{HardTurnLimit: 3, EnableTopicShift: false},
		Taxonomy: memorylayer.TaxonomyConfig{Enabled: true, Timeout: time.Second},
		Profile:  memorylayer.ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"},
		Lifecycle: memorylayer.LifecycleConfig{
			InjectProfileOnStart: true, ProfileMaxTokens: 400, ProfileUserID: "default",
		},
		RecallLimit: 3,
	})
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		layer.ObserveTurn(ctx, memorylayer.Turn{
			ID: int64(i), UserMsg: "I'm allergic to peanuts",
			Assistant: "Noted.", Timestamp: time.Now(),
		})
	}
	// Poll for async profile write (≤ 2s).
	var p *storage.Profile
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got, err := store.GetProfile(ctx, "default"); err == nil && len(got.Sections) > 0 {
			p = got
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NotNil(t, p)
	require.Len(t, p.Sections, 1)
	assert.Equal(t, "explicit", p.Sections[0].Kind)

	pinned, err := layer.LoadPinned(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, pinned)
	assert.Contains(t, pinned[0].Content, "## User Profile")
	assert.Contains(t, pinned[0].Content, "peanuts")
}

func TestIntegration_ForesightArchival(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()
	now := time.Now().UTC()

	// Two foresights: one expired, one fresh.
	_ = store.SaveMemory(ctx, &storage.Memory{
		ID: "f1", Content: "report due monday", MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(-time.Hour), CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour),
	})
	_ = store.SaveMemory(ctx, &storage.Memory{
		ID: "f2", Content: "demo next friday", MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(72 * time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	rep, err := memprovider.Consolidate(ctx, store, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Archived)

	// Active foresight should still be queryable.
	all, _ := store.ListMemoriesByType(ctx, "foresight", 100)
	require.Len(t, all, 1)
	assert.Equal(t, "f2", all[0].ID)
	assert.Equal(t, storage.MemoryStatusActive, all[0].Status)
}

func TestIntegration_SkillCandidateEmit(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	ctx := context.Background()

	llm := &stubLLM{}
	layer := memorylayer.New(store, &stubEmbedder{}, nil, llm, memorylayer.Config{
		Boundary:     memorylayer.BoundaryConfig{HardTurnLimit: 2, EnableTopicShift: false},
		Taxonomy:     memorylayer.TaxonomyConfig{Enabled: true, Timeout: time.Second},
		SkillEmitter: memorylayer.SkillEmitterConfig{Enabled: true, MaxTurns: 8},
		RecallLimit:  3,
	})

	var received *memorylayer.SkillCandidate
	layer.SetSkillCandidateSink(func(cand memorylayer.SkillCandidate) {
		received = &cand
	})

	for i := 1; i <= 2; i++ {
		layer.ObserveTurn(ctx, memorylayer.Turn{
			ID: int64(i), UserMsg: fmt.Sprintf("turn %d", i),
			Assistant: "ok", Timestamp: time.Now(),
		})
	}

	// Poll for async emit (≤ 2s).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if received != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NotNil(t, received)
	require.NotEmpty(t, received.Turns)
	assert.Equal(t, "turn 1", received.Turns[0].UserMsg)
}
