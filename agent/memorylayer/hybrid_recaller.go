package memorylayer

import (
	"context"
	"sort"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/embedding"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

type HybridConfig struct {
	RRFConstant             float64 // default 60
	BM25TopNMultiplier      int     // default 3
	VectorTopNMultiplier    int     // default 3
	PreRerankTopKMultiplier int     // default 2
	ReinforcementAlpha      float64 // default 0.15
	NeglectPenalty          float64 // default 0.10
}

func (c *HybridConfig) fill() {
	if c.RRFConstant <= 0 {
		c.RRFConstant = 60
	}
	if c.BM25TopNMultiplier <= 0 {
		c.BM25TopNMultiplier = 3
	}
	if c.VectorTopNMultiplier <= 0 {
		c.VectorTopNMultiplier = 3
	}
	if c.PreRerankTopKMultiplier <= 0 {
		c.PreRerankTopKMultiplier = 2
	}
	if c.ReinforcementAlpha < 0 {
		c.ReinforcementAlpha = 0.15
	}
	if c.NeglectPenalty < 0 {
		c.NeglectPenalty = 0.10
	}
}

// HybridRecaller fuses BM25 + Vector via RRF over the local storage,
// and applies reinforcement signals to the final ordering.
type HybridRecaller struct {
	store    storage.Storage
	embedder embedding.Embedder
	base     memprovider.Recaller // external provider for non-SQLite paths
	cfg      HybridConfig
}

func NewHybridRecaller(store storage.Storage, emb embedding.Embedder, base memprovider.Recaller, cfg HybridConfig) *HybridRecaller {
	cfg.fill()
	return &HybridRecaller{store: store, embedder: emb, base: base, cfg: cfg}
}

// Recall returns the fused top-N candidates. limit is the FINAL top-N
// returned to the caller; internally we fetch limit * PreRerankTopKMultiplier
// to leave headroom for the reranker stage (Task 4).
func (h *HybridRecaller) Recall(ctx context.Context, query string, limit int) ([]Candidate, error) {
	if limit <= 0 {
		limit = 5
	}

	if h.store == nil {
		// External-only mode: pass through base.
		if h.base == nil {
			return nil, nil
		}
		out, err := h.base.Recall(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		cands := make([]Candidate, 0, len(out))
		for i, m := range out {
			cands = append(cands, Candidate{ID: m.ID, Content: m.Content, Source: "base", Rank: i + 1})
		}
		return cands, nil
	}

	perSourceLimit := limit * h.cfg.BM25TopNMultiplier
	if v := limit * h.cfg.VectorTopNMultiplier; v > perSourceLimit {
		perSourceLimit = v
	}

	// Run BM25 + Vector concurrently.
	type result struct {
		mems []*storage.Memory
		err  error
		src  string
	}
	ch := make(chan result, 2)

	go func() {
		opts := &storage.MemorySearchOptions{Limit: perSourceLimit, RankingMode: "fts_only"}
		mems, err := h.store.SearchMemories(ctx, query, opts)
		ch <- result{mems: mems, err: err, src: "fts"}
	}()
	go func() {
		var qv []float32
		if h.embedder != nil {
			if v, err := h.embedder.Embed(ctx, query); err == nil {
				qv = v
			}
		}
		if qv == nil {
			ch <- result{mems: nil, err: nil, src: "vector"}
			return
		}
		opts := &storage.MemorySearchOptions{Limit: perSourceLimit, QueryVector: qv, RankingMode: "vector_only"}
		mems, err := h.store.SearchMemories(ctx, "", opts)
		ch <- result{mems: mems, err: err, src: "vector"}
	}()

	var fts, vec []*storage.Memory
	var firstErr error
	for i := 0; i < 2; i++ {
		r := <-ch
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		switch r.src {
		case "fts":
			fts = r.mems
		case "vector":
			vec = r.mems
		}
	}
	if len(fts) == 0 && len(vec) == 0 {
		return nil, firstErr
	}

	fused := rrfFuse(fts, vec, h.cfg.RRFConstant)
	h.applySignalBoost(fused)
	sort.SliceStable(fused, func(i, j int) bool { return fused[i].Score > fused[j].Score })

	top := limit * h.cfg.PreRerankTopKMultiplier
	if top > len(fused) {
		top = len(fused)
	}
	return fused[:top], nil
}

// rrfFuse merges two ranked lists by Reciprocal Rank Fusion:
//
//	score(d) = Σ_m 1 / (k + rank_m(d))
//
// rank is 1-based.
func rrfFuse(fts, vec []*storage.Memory, k float64) []Candidate {
	byID := make(map[string]*Candidate, len(fts)+len(vec))
	add := func(list []*storage.Memory, src string) {
		for i, m := range list {
			if m == nil {
				continue
			}
			rank := i + 1
			contrib := 1.0 / (k + float64(rank))
			if c, ok := byID[m.ID]; ok {
				c.Score += contrib
				c.Source = "fused"
			} else {
				byID[m.ID] = &Candidate{
					ID:            m.ID,
					Content:       m.Content,
					MemType:       m.MemType,
					Score:         contrib,
					Source:        src,
					Rank:          rank,
					Reinforcement: m.ReinforcementCount,
					Neglect:       m.NeglectCount,
				}
			}
		}
	}
	add(fts, "fts")
	add(vec, "vector")
	out := make([]Candidate, 0, len(byID))
	for _, c := range byID {
		out = append(out, *c)
	}
	return out
}

func (h *HybridRecaller) applySignalBoost(cands []Candidate) {
	for i := range cands {
		r := float64(cands[i].Reinforcement)
		n := float64(cands[i].Neglect)
		denom := r + n
		if denom <= 0 {
			continue
		}
		net := (r - n) / denom // [-1, 1]
		if net > 0 {
			cands[i].Score *= 1.0 + h.cfg.ReinforcementAlpha*net
		} else {
			cands[i].Score *= 1.0 + h.cfg.NeglectPenalty*net // net is negative
		}
	}
}
