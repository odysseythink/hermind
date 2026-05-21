package memorylayer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedLLM returns pre-programmed responses on each call.
type scriptedLLM struct {
	calls     int
	responses []scriptedResponse
}

type scriptedResponse struct {
	resp *core.Response
	err  error
	sleep time.Duration
}

func (s *scriptedLLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if s.calls >= len(s.responses) {
		return nil, errors.New("no more scripted responses")
	}
	r := s.responses[s.calls]
	s.calls++
	if r.sleep > 0 {
		select {
		case <-time.After(r.sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	return r.resp, nil
}

func (s *scriptedLLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *scriptedLLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *scriptedLLM) Provider() string { return "scripted" }
func (s *scriptedLLM) Model() string    { return "scripted-model" }

type passThroughReranker struct{}

func (p *passThroughReranker) Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error) {
	return trimCandidates(cands, topN), nil
}

func makeAgentic(t *testing.T, store storage.Storage, llm core.LanguageModel, cfg AgenticConfig) *Agentic {
	t.Helper()
	ml := &MemoryLayer{
		store:    store,
		hybrid:   NewHybridRecaller(store, &stubEmbedder{}, nil, HybridConfig{}),
		reranker: &passThroughReranker{},
		cfg:      Config{RecallLimit: 5},
	}
	return NewAgentic(ml, llm, cfg)
}

func TestAgentic_ShortcutSkipsCritic(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "test content", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{}
	a := makeAgentic(t, store, llm, AgenticConfig{Enabled: true, ShortcutThreshold: 0.0})
	// With threshold=0, any positive score triggers shortcut.
	cands, err := a.Recall(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, "m1", cands[0].ID)
	assert.Equal(t, 0, llm.calls, "critic should NOT be called when shortcut fires")
}

func TestAgentic_CriticSaysSufficient(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "test content", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{responses: []scriptedResponse{
		{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `{"sufficient":true}`},
		}}}},
	}}
	a := makeAgentic(t, store, llm, AgenticConfig{Enabled: true, ShortcutThreshold: 1.0})
	cands, err := a.Recall(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, "m1", cands[0].ID)
	assert.Equal(t, 1, llm.calls, "only sufficiency check should be called")
}

func TestAgentic_CriticSaysInsufficientRunsExpansion(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "original match", MemType: "fact", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m2", Content: "sub query match", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{responses: []scriptedResponse{
		// sufficiency
		{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `{"sufficient":false,"missing":"sub query context"}`},
		}}}},
		// expansion
		{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `["sub query"]`},
		}}}},
	}}
	a := makeAgentic(t, store, llm, AgenticConfig{
		Enabled: true, ShortcutThreshold: 1.0,
		ExpansionQueries: 2,
		Timeout:          5 * time.Second,
	})
	cands, err := a.Recall(context.Background(), "original", 3)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(cands), 1)
	assert.GreaterOrEqual(t, llm.calls, 2, "expected sufficiency + expansion calls")
}

func TestAgentic_LLMFailFallsBackToM1(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "test content", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{responses: []scriptedResponse{
		{err: errors.New("model down")},
	}}
	a := makeAgentic(t, store, llm, AgenticConfig{Enabled: true, ShortcutThreshold: 1.0})
	cands, err := a.Recall(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, "m1", cands[0].ID)
}

func TestAgentic_BadJSONFallsBack(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "test content", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{responses: []scriptedResponse{
		{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `I'm not sure`},
		}}}},
	}}
	a := makeAgentic(t, store, llm, AgenticConfig{Enabled: true, ShortcutThreshold: 1.0})
	cands, err := a.Recall(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, "m1", cands[0].ID)
}

func TestAgentic_TokenCapShortcutsExpansion(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "test content", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{responses: []scriptedResponse{
		// This should never be called because cap blocks the sufficiency check
		{resp: &core.Response{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `{"sufficient":false}`},
		}}}},
	}}
	a := makeAgentic(t, store, llm, AgenticConfig{
		Enabled: true, ShortcutThreshold: 1.0,
		PerTurnTokenCap:    100,
		PerSessionTokenCap: 10000,
	})
	cands, err := a.Recall(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, 0, llm.calls, "sufficiency check should be skipped when cap is exhausted")
}

func TestAgentic_TimeoutFallsBack(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "m1", Content: "test content", MemType: "fact", Status: "active",
	})

	llm := &scriptedLLM{responses: []scriptedResponse{
		{sleep: 200 * time.Millisecond},
	}}
	a := makeAgentic(t, store, llm, AgenticConfig{
		Enabled: true, ShortcutThreshold: 1.0,
		Timeout: 50 * time.Millisecond,
	})
	cands, err := a.Recall(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, cands, 1)
	assert.Equal(t, "m1", cands[0].ID)
}

func TestRRFFuseCandidates_OverlappingItems(t *testing.T) {
	list1 := []Candidate{
		{ID: "a", Content: "alpha"},
		{ID: "b", Content: "bravo"},
	}
	list2 := []Candidate{
		{ID: "a", Content: "alpha"},
		{ID: "c", Content: "charlie"},
	}
	fused := rrfFuseCandidates([][]Candidate{list1, list2}, 60)
	require.Len(t, fused, 3)

	// a appears in both lists → score = 1/61 + 1/61 ≈ 0.0328
	// b appears only in first (rank 2) → score = 1/62 ≈ 0.0161
	// c appears only in second (rank 2) → score = 1/62 ≈ 0.0161
	// Expected order: a, b, c (b before c because same score, tiebreaker by ID)
	assert.Equal(t, "a", fused[0].ID)
	assert.Equal(t, "b", fused[1].ID)
	assert.Equal(t, "c", fused[2].ID)
	assert.True(t, fused[0].Score > fused[1].Score)
	assert.Equal(t, fused[1].Score, fused[2].Score)
}

func TestRenderCandidatesForCritic_Truncates(t *testing.T) {
	cs := make([]Candidate, 15)
	for i := range cs {
		cs[i] = Candidate{ID: fmt.Sprintf("m%d", i), Content: strings.Repeat("x", 200)}
	}
	out := renderCandidatesForCritic(cs)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Len(t, lines, 10, "critic should see at most 10 candidates")
}

func TestParseJSONObject_ExtractsFromNoise(t *testing.T) {
	var v struct{ Sufficient bool `json:"sufficient"` }
	ok := parseJSONObject(`Some text before {"sufficient":true} and after`, &v)
	require.True(t, ok)
	assert.True(t, v.Sufficient)
}
