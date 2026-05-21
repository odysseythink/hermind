package memorylayer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/odysseythink/hermind/agent/memorylayer"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/core"
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
