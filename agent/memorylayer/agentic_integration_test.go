package memorylayer_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/odysseythink/hermind/agent/memorylayer"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/core"
)

// scriptedLLMIntegration returns pre-programmed responses on each call.
type scriptedLLMIntegration struct {
	calls     int
	responses []*core.Response
}

func (s *scriptedLLMIntegration) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if s.calls >= len(s.responses) {
		return nil, fmt.Errorf("no more scripted responses (calls=%d)", s.calls)
	}
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}

func (s *scriptedLLMIntegration) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *scriptedLLMIntegration) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *scriptedLLMIntegration) Provider() string { return "scripted" }
func (s *scriptedLLMIntegration) Model() string    { return "scripted-model" }

type stubEmbedderIntegration struct{}

func (s *stubEmbedderIntegration) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{1, 0, 0}, nil
}

func TestIntegration_AgenticMultiRound(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	// Seed: 5 facts.
	seed := []struct{ id, content string }{
		{"m1", "user prefers TypeScript"},
		{"m2", "project uses pnpm"},
		{"m3", "the build is via vite"},
		{"m4", "user dislikes Tailwind"},
		{"m5", "the API key is in .env"},
	}
	for _, s := range seed {
		_ = store.SaveMemory(context.Background(), &storage.Memory{
			ID: s.id, Content: s.content, MemType: "fact", Status: "active",
		})
	}

	llm := &scriptedLLMIntegration{responses: []*core.Response{
		// sufficiency: insufficient
		{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `{"sufficient":false,"missing":"build tools"}`},
		}}},
		// expansion: 2 sub-queries
		{Message: core.Message{Content: []core.ContentParter{
			core.TextPart{Text: `["build system","package manager"]`},
		}}},
	}}

	layer := memorylayer.New(store, &stubEmbedderIntegration{}, nil, llm, memorylayer.Config{
		Hybrid:   memorylayer.HybridConfig{},
		Reranker: memorylayer.RerankerConfig{Enabled: false}, // disable to keep LLM calls predictable
		Agentic: memorylayer.AgenticConfig{
			Enabled: true, MaxExtraRounds: 1, ExpansionQueries: 2,
			ShortcutThreshold: 1.0, // force critic to run
			PerTurnTokenCap:    2000,
			PerSessionTokenCap: 20000,
			Timeout:            3 * time.Second,
		},
		RecallLimit: 3,
	})

	out, err := layer.RecallWithAgentic(context.Background(), "what tools and conventions does the project use", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("expected hits")
	}

	if llm.calls != 2 {
		t.Errorf("expected 2 LLM calls (sufficiency + expansion), got %d", llm.calls)
	}
}

func TestIntegration_PinnedRoundtrip(t *testing.T) {
	store, _ := sqlite.Open(":memory:")
	defer store.Close()
	_ = store.Migrate()
	now := time.Now().UTC()
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c1", Content: "I am allergic to peanuts", MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f1", Content: "Project report due Wed",
		MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(48 * time.Hour),
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f2", Content: "Far-future plan",
		MemType: "foresight", Status: "active",
		ExpiresAt: now.AddDate(0, 0, 30),
	})

	layer := memorylayer.New(store, &stubEmbedderIntegration{}, nil, &scriptedLLMIntegration{},
		memorylayer.Config{
			Lifecycle: memorylayer.LifecycleConfig{
				InjectCoreOnStart: true, InjectForesightOnStart: true,
				CoreMaxCount: 10, CoreMaxTokens: 600,
				ForesightMaxCount: 3, ForesightDaysAhead: 7,
			},
			RecallLimit: 3,
		})

	pinned, err := layer.LoadPinned(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pinned) != 2 {
		t.Fatalf("expected 2 pinned (core + 1 near foresight), got %d", len(pinned))
	}
	if !strings.Contains(pinned[0].Content, "peanuts") {
		t.Errorf("core must lead, got %q", pinned[0].Content)
	}
}
