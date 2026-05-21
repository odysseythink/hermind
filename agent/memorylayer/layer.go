package memorylayer

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/embedding"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

type Config struct {
	Hybrid      HybridConfig
	Reranker    RerankerConfig
	Boundary    BoundaryConfig
	Taxonomy    TaxonomyConfig
	RecallLimit int // final top-N returned from Recall; default 5
}

type MemoryLayer struct {
	store     storage.Storage
	hybrid    *HybridRecaller
	reranker  Reranker
	boundary  *BoundaryDetector
	extractor *TaxonomyExtractor
	cfg       Config
}

func New(
	store storage.Storage,
	emb embedding.Embedder,
	base memprovider.Recaller,
	llm core.LanguageModel,
	cfg Config,
) *MemoryLayer {
	if cfg.RecallLimit <= 0 {
		cfg.RecallLimit = 5
	}
	return &MemoryLayer{
		store:     store,
		hybrid:    NewHybridRecaller(store, emb, base, cfg.Hybrid),
		reranker:  NewLLMReranker(llm, cfg.Reranker),
		boundary:  NewBoundaryDetector(cfg.Boundary, emb),
		extractor: NewTaxonomyExtractor(llm, cfg.Taxonomy),
		cfg:       cfg,
	}
}

// RecallCandidates runs Hybrid + Reranker and returns the internal
// Candidate slice (not flattened to InjectedMemory). Used by Agentic
// to compose multi-round fusion before final emit.
func (l *MemoryLayer) RecallCandidates(ctx context.Context, query string, limit int) ([]Candidate, error) {
	if limit <= 0 {
		limit = l.cfg.RecallLimit
	}
	cands, err := l.hybrid.Recall(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		return nil, nil
	}
	ranked, _ := l.reranker.Rerank(ctx, query, cands, limit)
	return ranked, nil
}

// Recall is the single retrieval entry point. limit overrides the
// configured RecallLimit when > 0.
func (l *MemoryLayer) Recall(ctx context.Context, query string, limit int) ([]memprovider.InjectedMemory, error) {
	if limit <= 0 {
		limit = l.cfg.RecallLimit
	}
	cands, err := l.hybrid.Recall(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		return nil, nil
	}
	ranked, _ := l.reranker.Rerank(ctx, query, cands, limit)
	return candidatesToInjected(ranked, limit), nil
}

// ObserveTurn feeds the boundary detector. If a boundary fires, the
// extractor runs synchronously in a detached goroutine (caller's ctx
// is not held). Errors are logged but never returned — observation
// must not slow the turn.
func (l *MemoryLayer) ObserveTurn(ctx context.Context, t Turn) {
	b := l.boundary.Observe(ctx, t)
	if b == nil {
		return
	}
	go l.handleBoundary(b)
}

// Flush runs Boundary.Flush at shutdown and processes any tail buffer.
func (l *MemoryLayer) Flush(ctx context.Context) {
	if b := l.boundary.Flush(); b != nil {
		l.handleBoundary(b)
	}
}

func (l *MemoryLayer) handleBoundary(b *Boundary) {
	ctx := context.Background()
	mems, err := l.extractor.Extract(ctx, b)
	if err != nil {
		mlog.Warning("memorylayer: extractor failed", mlog.String("err", err.Error()), mlog.String("reason", b.Reason))
		return
	}
	for _, m := range mems {
		if err := l.store.SaveMemory(ctx, m); err != nil {
			mlog.Warning("memorylayer: SaveMemory failed", mlog.String("err", err.Error()))
			continue
		}
	}
	_ = l.store.AppendMemoryEvent(ctx, b.Turns[len(b.Turns)-1].Timestamp, "boundary.detected", []byte(`{"reason":"`+b.Reason+`","extracted":`+itoa(len(mems))+`}`))
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
