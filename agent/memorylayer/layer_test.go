package memorylayer

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
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
