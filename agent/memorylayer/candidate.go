package memorylayer

import "github.com/odysseythink/hermind/tool/memory/memprovider"

// Candidate is the internal ranked-memory representation used between
// HybridRecaller, Reranker, and (P2) Agentic layers. It carries enough
// metadata to debug rankings and to apply downstream policies.
type Candidate struct {
	ID            string
	Content       string
	MemType       string
	Score         float64 // composite score in the producing stage
	Source        string  // "fts" | "vector" | "fused" | "rerank" | "base"
	Rank          int     // 1-based rank in producing stage; 0 if unknown
	Reinforcement int
	Neglect       int
}

func candidatesToInjected(cs []Candidate, limit int) []memprovider.InjectedMemory {
	if limit > 0 && len(cs) > limit {
		cs = cs[:limit]
	}
	out := make([]memprovider.InjectedMemory, 0, len(cs))
	for _, c := range cs {
		out = append(out, memprovider.InjectedMemory{ID: c.ID, Content: c.Content})
	}
	return out
}
