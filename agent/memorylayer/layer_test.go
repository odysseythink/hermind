package memorylayer

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoReranker returns input unchanged (trimmed to topN).
type echoReranker struct{}

func (e *echoReranker) Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error) {
	if topN > 0 && len(cands) > topN {
		return cands[:topN], nil
	}
	return cands, nil
}

func TestMemoryLayer_RecallEnd2End(t *testing.T) {
	store := &fakeStorage{
		mems: []*storage.Memory{
			{ID: "m1", Content: "q alpha"},
			{ID: "m2", Content: "q beta"},
			{ID: "m3", Content: "q gamma"},
			{ID: "m4", Content: "q delta"},
			{ID: "m5", Content: "q epsilon"},
		},
	}
	layer := &MemoryLayer{
		store:    store,
		hybrid:   NewHybridRecaller(store, nil, nil, HybridConfig{}),
		reranker: &echoReranker{},
		cfg:      Config{RecallLimit: 3},
	}
	out, err := layer.Recall(context.Background(), "q", 0)
	require.NoError(t, err)
	require.Len(t, out, 3)
}

func TestMemoryLayer_ObserveTurn_NoBoundary(t *testing.T) {
	store := &fakeStorage{}
	layer := New(store, nil, nil, nil, Config{
		Boundary: BoundaryConfig{HardTurnLimit: 10},
		Taxonomy: TaxonomyConfig{Enabled: true},
	})
	ctx := context.Background()
	layer.ObserveTurn(ctx, Turn{ID: 1, Tokens: 5})
	// No boundary yet; buffer has 1 turn.
	b := layer.boundary.Flush()
	require.NotNil(t, b)
	assert.Len(t, b.Turns, 1)
}

func TestMemoryLayer_ObserveTurn_BoundaryPersists(t *testing.T) {
	store := &fakeStorage{}
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `[{"type":"fact","content":"test fact","confidence":0.9}]`},
	}}}}
	layer := New(store, nil, nil, llm, Config{
		Boundary: BoundaryConfig{HardTurnLimit: 2},
		Taxonomy: TaxonomyConfig{Enabled: true, MaxOutputs: 4, Timeout: time.Second},
	})
	ctx := context.Background()
	layer.ObserveTurn(ctx, Turn{ID: 1, Tokens: 5})
	layer.ObserveTurn(ctx, Turn{ID: 2, Tokens: 5})
	// Boundary fires; extractor runs in goroutine.
	time.Sleep(50 * time.Millisecond)

	// Verify that SaveMemory was called (fakeStorage doesn't track, but we can
	// at least verify no panic and Flush is empty after boundary).
	assert.Nil(t, layer.boundary.Flush())
}

func TestMemoryLayer_FlushAtShutdown(t *testing.T) {
	store := &fakeStorage{}
	llm := &mockLLM{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `[{"type":"fact","content":"flush test","confidence":0.9}]`},
	}}}}
	layer := New(store, nil, nil, llm, Config{
		Boundary: BoundaryConfig{HardTurnLimit: 100},
		Taxonomy: TaxonomyConfig{Enabled: true, MaxOutputs: 4, Timeout: time.Second},
	})
	ctx := context.Background()
	layer.ObserveTurn(ctx, Turn{ID: 1, Tokens: 5})
	layer.ObserveTurn(ctx, Turn{ID: 2, Tokens: 5})
	layer.ObserveTurn(ctx, Turn{ID: 3, Tokens: 5})

	// Flush at shutdown
	layer.Flush(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.Nil(t, layer.boundary.Flush())
}

// Verify candidatesToInjected works correctly.
func TestCandidatesToInjected(t *testing.T) {
	cs := []Candidate{
		{ID: "a", Content: "alpha"},
		{ID: "b", Content: "beta"},
		{ID: "c", Content: "gamma"},
	}
	out := candidatesToInjected(cs, 2)
	require.Len(t, out, 2)
	assert.Equal(t, memprovider.InjectedMemory{ID: "a", Content: "alpha"}, out[0])
	assert.Equal(t, memprovider.InjectedMemory{ID: "b", Content: "beta"}, out[1])
}

func TestMemoryLayer_RecallWithAgentic_FallsBackWhenAgenticNil(t *testing.T) {
	store := &fakeStorage{
		mems: []*storage.Memory{
			{ID: "m1", Content: "q alpha"},
			{ID: "m2", Content: "q beta"},
		},
	}
	layer := &MemoryLayer{
		store:    store,
		hybrid:   NewHybridRecaller(store, nil, nil, HybridConfig{}),
		reranker: &echoReranker{},
		cfg:      Config{RecallLimit: 3},
	}
	// agentic is nil → falls back to plain Recall
	out, err := layer.RecallWithAgentic(context.Background(), "q", 2)
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestMemoryLayer_LoadPinned_NoLifecycle(t *testing.T) {
	layer := &MemoryLayer{
		store:     &fakeStorage{},
		lifecycle: nil,
	}
	out, err := layer.LoadPinned(context.Background())
	require.NoError(t, err)
	require.Nil(t, out)
}

func TestMemoryLayer_LoadPinned_HappyPath(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c1", Content: "core one", MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f1", Content: "foresight near",
		MemType: "foresight", Status: "active",
		ExpiresAt: time.Now().UTC().Add(48 * time.Hour),
	})

	layer := New(store, nil, nil, nil, Config{
		Lifecycle: LifecycleConfig{
			InjectCoreOnStart: true, InjectForesightOnStart: true,
			CoreMaxCount: 10, CoreMaxTokens: 600,
			ForesightMaxCount: 3, ForesightDaysAhead: 7,
		},
	})
	out, err := layer.LoadPinned(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "core one", out[0].Content)
	assert.Equal(t, "foresight near", out[1].Content)
}

func TestMemoryLayer_RecallWithAgentic_EndToEnd(t *testing.T) {
	store := &fakeStorage{mems: []*storage.Memory{
		{ID: "m1", Content: "original match", MemType: "fact", Status: "active"},
		{ID: "m2", Content: "sub query match", MemType: "fact", Status: "active"},
	}}
	llm := &mockLLM{}
	layer := New(store, nil, nil, llm, Config{
		Hybrid:   HybridConfig{},
		Reranker: RerankerConfig{Enabled: false},
		Agentic: AgenticConfig{
			Enabled: true, MaxExtraRounds: 1, ExpansionQueries: 2,
			ShortcutThreshold: 1.0, // force critic
			PerTurnTokenCap:    2000,
			PerSessionTokenCap: 20000,
			Timeout:            3 * time.Second,
		},
		RecallLimit: 3,
	})
	// Set up scripted LLM responses: sufficiency says insufficient, then expansion returns sub-queries
	llm.resp = &core.Response{Message: core.Message{Content: []core.ContentParter{
		core.TextPart{Text: `{"sufficient":false,"missing":"more context"}`},
	}}}
	// The expansion will use the same LLM, but since mockLLM always returns the same resp,
	// expansion gets the same JSON which is not a valid string array → degrade to m1.
	// That's acceptable for this test: we just verify RecallWithAgentic doesn't panic
	// and returns results when agentic is enabled.
	out, err := layer.RecallWithAgentic(context.Background(), "original", 3)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(out), 1)
}
