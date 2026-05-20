# Memory Layer Phase 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Source Design:** `.gpowers/designs/2026-05-20-memory-layer-design.md` (v2, 2026-05-21).

**Scope (P1 only):**
- §4 Hybrid Retrieval (BM25 + Vector + RRF) + signal weighting
- §4.3 LLM Reranker (with degradation)
- §3.4 Storage extensions (`ParentTurnID`, `ParentMemID`, `ExpiresAt`, `ClusterID`) + migration v10
- §3.5 Taxonomy Extractor (4 types: `core` / `episode` / `fact` / `foresight`) with provenance
- §7 Boundary Detector (MemCell-lite)
- Wire HybridRecaller into `api/server.go` behind config flags

**Out of scope (deferred to Phase 2/3 plans):**
- Agentic multi-round (P2.1)
- Lifecycle hooks beyond minimal turn observation (P2.2)
- Living Profile object (P3.1)
- Foresight expiry/archival on Consolidate (P3.2) — schema ready, behavior deferred
- Skill candidate emitter → Evolver (P3.3)
- Clustering (P4)

P2/P3 plans are blocked on the interfaces this plan ships. Write them after Task 10 here lands.

**Goal:** Make every `Recall` go through `BM25 + Vector + RRF + reinforcement-signal + LLM Reranker`, with every extracted memory carrying back-references to its source turn(s). Lay the groundwork for Agentic multi-round in P2.

**Architecture:** `agent/memorylayer` is a decorator package that wraps the existing `memprovider.Recaller`. Storage gains 4 columns and one new ranking mode. No change to `memprovider.Provider` / `Recaller` / `tool.Registry` interfaces.

**Tech Stack:** Go 1.22+, SQLite (FTS5), existing `embedding.Embedder`, existing `pantheon/core.LanguageModel`, `testify` + table-driven tests.

---

## File Structure

```
agent/memorylayer/                              (new package)
├── doc.go                                       (new — package overview)
├── candidate.go                                 (new — internal scored type)
├── hybrid_recaller.go                           (new — RRF fusion + signal boost)
├── hybrid_recaller_test.go                      (new)
├── reranker.go                                  (new — LLM-as-reranker)
├── reranker_test.go                             (new)
├── boundary.go                                  (new — MemCell-lite detector)
├── boundary_test.go                             (new)
├── extractor.go                                 (new — Taxonomy extractor)
├── extractor_test.go                            (new)
├── layer.go                                     (new — composition root)
├── layer_test.go                                (new)
└── prompts/
    ├── taxonomy_extract.txt                     (new)
    └── rerank.txt                               (new)

storage/types.go                                 (modify — Memory fields + MemorySearchOptions.RankingMode)
storage/sqlite/migrate.go                        (modify — bump to v10, add ALTERs)
storage/sqlite/memory.go                        (modify — RankingMode branching, scan new cols)
storage/sqlite/memory_test.go                   (modify — new field round-trip + RankingMode cases)

api/server.go                                    (modify — wrap Recaller behind config flag)
config/config.go                                 (modify — MemoryLayerConfig)
config/defaults.go (if present)                  (modify — defaults for new config)

cli/engine_deps.go                               (modify — instantiate MemoryLayer when enabled)
```

---

## Dependencies

All net-new packages depend only on existing modules:
- `storage` + `storage/sqlite` (Memory schema, FTS5)
- `tool/memory/memprovider` (`Recaller`, `InjectedMemory`)
- `tool/embedding` (`Embedder`, `DecodeVector`, `pembed.Cosine`)
- `pantheon/core` (`LanguageModel`, `Message`, `Request`)
- `config`
- `mlog` for logging

No new go.mod dependencies.

---

## Task 1: Storage schema extensions (migration v10)

**Files:**
- **Modify:** `storage/types.go`
- **Modify:** `storage/sqlite/migrate.go`
- **Modify:** `storage/sqlite/memory.go`
- **Modify:** `storage/sqlite/memory_test.go`

**Context:** Adds 4 columns to `memories` for provenance and time-bound memories. Pattern mirrors v4–v8 (ALTER TABLE with `duplicate column name` tolerance). Indexes only where queries will need them.

- [ ] **Step 1: Extend `storage.Memory` struct**

In `storage/types.go`, add fields after `ReinforcedAtSeq`:

```go
// ParentTurnID is the source message id this memory was derived from.
// 0 means no recorded source (manual entries, external syncs).
ParentTurnID int64 `json:"parent_turn_id,omitempty"`

// ParentMemID, when set, is the MemCell-equivalent parent memory id
// (i.e., episode → MemCell). Empty for top-level memories.
ParentMemID string `json:"parent_mem_id,omitempty"`

// ExpiresAt is non-zero only for time-bound memories (currently only
// MemType="foresight"). Consolidate uses this to archive expired rows
// (behavior wired in Phase 3; column persisted now).
ExpiresAt time.Time `json:"expires_at,omitempty"`

// ClusterID is the topical cluster id assigned by the clustering pass.
// Empty until P4 ships; column reserved now to avoid a future migration.
ClusterID string `json:"cluster_id,omitempty"`
```

- [ ] **Step 2: Add `RankingMode` to `MemorySearchOptions`**

```go
// RankingMode selects which signals contribute to the result ordering.
//   "" or "hybrid": existing weighted-sum behavior (default, backward compat)
//   "fts_only":     return rows ranked by FTS5 BM25 alone, no cosine/recency/reinforcement
//   "vector_only":  return rows ranked by cosine alone (QueryVector required)
// Used by agent/memorylayer.HybridRecaller to feed independent ranked lists into RRF.
RankingMode string

// MemTypes, when non-empty, restricts results to these MemTypes (OR).
MemTypes []string

// IncludeExpired, when false (default), filters out rows whose ExpiresAt
// is non-zero and in the past.
IncludeExpired bool
```

- [ ] **Step 3: Bump schema version + add v10 migration**

In `storage/sqlite/migrate.go`:

```go
// Change:
const currentSchemaVersion = 10
```

Add a new case in `applyVersion`:

```go
case 10:
    for _, ddl := range []string{
        `ALTER TABLE memories ADD COLUMN parent_turn_id INTEGER NOT NULL DEFAULT 0`,
        `ALTER TABLE memories ADD COLUMN parent_mem_id  TEXT    NOT NULL DEFAULT ''`,
        `ALTER TABLE memories ADD COLUMN expires_at     REAL    NOT NULL DEFAULT 0`,
        `ALTER TABLE memories ADD COLUMN cluster_id     TEXT    NOT NULL DEFAULT ''`,
    } {
        if _, err := tx.Exec(ddl); err != nil {
            if !strings.Contains(err.Error(), "duplicate column name") {
                return fmt.Errorf("v10 alter memories: %w", err)
            }
        }
    }
    for _, ddl := range []string{
        `CREATE INDEX IF NOT EXISTS idx_memories_parent_turn ON memories(parent_turn_id)`,
        `CREATE INDEX IF NOT EXISTS idx_memories_expires_at  ON memories(expires_at)`,
        `CREATE INDEX IF NOT EXISTS idx_memories_cluster_id  ON memories(cluster_id)`,
    } {
        if _, err := tx.Exec(ddl); err != nil {
            return fmt.Errorf("v10 create index: %w", err)
        }
    }
```

Also update the `schemaSQL` constant (top of `migrate.go`) so fresh databases get all v10 columns inline — match the existing pattern used for `mem_type`, `vector`, etc.

- [ ] **Step 4: Update `memory.go` SELECT projections + scans**

In `storage/sqlite/memory.go`:
- Add the 4 new columns to every `SELECT … FROM memories` / projection list (search projection, `GetMemory`, `ListMemoriesByType`, candidate fetch).
- Extend the row scanner (`scanMemoryRow` or inline scans) to populate the new fields. Convert `expires_at` REAL via existing `fromEpoch`.
- Implement `MemTypes` and `IncludeExpired` filters in `fetchMemoryCandidates`:
  - `MemTypes`: `WHERE mem_type IN (?, ?, …)` (use `sqlIn` or build placeholders).
  - `IncludeExpired=false`: `AND (expires_at = 0 OR expires_at > strftime('%s','now'))`.
- Add `RankingMode` branching in `SearchMemories`:
  - `fts_only`: skip cosine/recency/reinforcement scoring, return FTS-ranked candidates directly (still apply Top-K limit).
  - `vector_only`: require `QueryVector`; score by cosine only; return Top-K.
  - `""` / `hybrid`: existing weighted-sum behavior (unchanged).

When persisting (`SaveMemory`), include the 4 new columns in INSERT / UPDATE.

- [ ] **Step 5: Round-trip + filter tests**

In `storage/sqlite/memory_test.go`:

```go
func TestMemory_ParentAndExpiresPersist(t *testing.T) {
    // SaveMemory with ParentTurnID/ParentMemID/ExpiresAt → GetMemory returns identical
}

func TestSearchMemories_FilterMemTypes(t *testing.T) {
    // insert mixed types; query with MemTypes=["fact","foresight"] excludes others
}

func TestSearchMemories_ExcludeExpired(t *testing.T) {
    // insert foresight with ExpiresAt = now-1h; default search excludes;
    // IncludeExpired=true returns it
}

func TestSearchMemories_RankingMode_FTSOnly(t *testing.T) {
    // verify ordering ignores reinforcement when RankingMode="fts_only"
}

func TestSearchMemories_RankingMode_VectorOnly(t *testing.T) {
    // verify ordering follows cosine only; QueryVector required
}
```

- [ ] **Step 6: Commit**

```bash
git add storage/types.go storage/sqlite/migrate.go storage/sqlite/memory.go storage/sqlite/memory_test.go
git commit -m "feat(storage): v10 — parent_turn_id/parent_mem_id/expires_at/cluster_id + RankingMode"
```

---

## Task 2: Internal `Candidate` type

**Files:**
- **Create:** `agent/memorylayer/candidate.go`
- **Create:** `agent/memorylayer/doc.go`

**Context:** `memprovider.InjectedMemory` is `{ID, Content}` only — too thin to carry scores between HybridRecaller → Reranker → final emit. Use an internal `Candidate` type and convert at the public boundary.

- [ ] **Step 1: Package doc**

`agent/memorylayer/doc.go`:

```go
// Package memorylayer wraps the existing memprovider.Recaller with
// Hybrid Retrieval (BM25 + Vector + RRF), an LLM Reranker, and a
// MemCell-lite boundary-triggered taxonomy extractor.
//
// All components are decorators: the underlying Provider / Recaller
// interface stays unchanged, and any external memory provider continues
// to work (downgraded to single-source mode without RRF when it cannot
// expose pure-BM25 / pure-vector ranking).
//
// See .gpowers/designs/2026-05-20-memory-layer-design.md for the
// design rationale and Phase 1 scope.
package memorylayer
```

- [ ] **Step 2: Candidate + conversion helpers**

```go
package memorylayer

import "github.com/odysseythink/hermind/tool/memory/memprovider"

// Candidate is the internal ranked-memory representation used between
// HybridRecaller, Reranker, and (P2) Agentic layers. It carries enough
// metadata to debug rankings and to apply downstream policies.
type Candidate struct {
    ID       string
    Content  string
    MemType  string
    Score    float64 // composite score in the producing stage
    Source   string  // "fts" | "vector" | "fused" | "rerank" | "base"
    Rank     int     // 1-based rank in producing stage; 0 if unknown
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
```

- [ ] **Step 3: Commit**

```bash
git add agent/memorylayer/doc.go agent/memorylayer/candidate.go
git commit -m "feat(memorylayer): package skeleton + internal Candidate type"
```

---

## Task 3: HybridRecaller (RRF + signal weighting)

**Files:**
- **Create:** `agent/memorylayer/hybrid_recaller.go`
- **Create:** `agent/memorylayer/hybrid_recaller_test.go`

**Context:** Runs two independent ranked retrievals (`fts_only`, `vector_only`) against the underlying storage, fuses by Reciprocal Rank Fusion (k=60), and applies a reinforcement boost / neglect penalty in the final score. For external providers without `Storage` access, downgrade to a single-source pass-through.

- [ ] **Step 1: Interface + struct**

```go
package memorylayer

import (
    "context"
    "sort"

    "github.com/odysseythink/hermind/storage"
    "github.com/odysseythink/hermind/tool/embedding"
    "github.com/odysseythink/hermind/tool/memory/memprovider"
)

type HybridConfig struct {
    RRFConstant            float64 // default 60
    BM25TopNMultiplier     int     // default 3
    VectorTopNMultiplier   int     // default 3
    PreRerankTopKMultiplier int    // default 2
    ReinforcementAlpha     float64 // default 0.15
    NeglectPenalty         float64 // default 0.10
}

func (c *HybridConfig) fill() {
    if c.RRFConstant <= 0 { c.RRFConstant = 60 }
    if c.BM25TopNMultiplier <= 0 { c.BM25TopNMultiplier = 3 }
    if c.VectorTopNMultiplier <= 0 { c.VectorTopNMultiplier = 3 }
    if c.PreRerankTopKMultiplier <= 0 { c.PreRerankTopKMultiplier = 2 }
    if c.ReinforcementAlpha < 0 { c.ReinforcementAlpha = 0.15 }
    if c.NeglectPenalty < 0 { c.NeglectPenalty = 0.10 }
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
```

- [ ] **Step 2: Recall implementation**

```go
// Recall returns the fused top-N candidates. limit is the FINAL top-N
// returned to the caller; internally we fetch limit * PreRerankTopKMultiplier
// to leave headroom for the reranker stage (Task 4).
func (h *HybridRecaller) Recall(ctx context.Context, query string, limit int) ([]Candidate, error) {
    if limit <= 0 { limit = 5 }

    if h.store == nil {
        // External-only mode: pass through base.
        if h.base == nil { return nil, nil }
        out, err := h.base.Recall(ctx, query, limit)
        if err != nil { return nil, err }
        cands := make([]Candidate, 0, len(out))
        for i, m := range out {
            cands = append(cands, Candidate{ID: m.ID, Content: m.Content, Source: "base", Rank: i + 1})
        }
        return cands, nil
    }

    perSourceLimit := limit * h.cfg.BM25TopNMultiplier
    if v := limit * h.cfg.VectorTopNMultiplier; v > perSourceLimit { perSourceLimit = v }

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
            if v, err := h.embedder.Embed(ctx, query); err == nil { qv = v }
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
        if r.err != nil && firstErr == nil { firstErr = r.err }
        switch r.src {
        case "fts":    fts = r.mems
        case "vector": vec = r.mems
        }
    }
    if len(fts) == 0 && len(vec) == 0 {
        return nil, firstErr
    }

    fused := rrfFuse(fts, vec, h.cfg.RRFConstant)
    h.applySignalBoost(fused)
    sort.SliceStable(fused, func(i, j int) bool { return fused[i].Score > fused[j].Score })

    top := limit * h.cfg.PreRerankTopKMultiplier
    if top > len(fused) { top = len(fused) }
    return fused[:top], nil
}

// rrfFuse merges two ranked lists by Reciprocal Rank Fusion:
//   score(d) = Σ_m 1 / (k + rank_m(d))
// rank is 1-based.
func rrfFuse(fts, vec []*storage.Memory, k float64) []Candidate {
    byID := make(map[string]*Candidate, len(fts)+len(vec))
    add := func(list []*storage.Memory, src string) {
        for i, m := range list {
            if m == nil { continue }
            rank := i + 1
            contrib := 1.0 / (k + float64(rank))
            if c, ok := byID[m.ID]; ok {
                c.Score += contrib
                c.Source = "fused"
            } else {
                byID[m.ID] = &Candidate{
                    ID: m.ID, Content: m.Content, MemType: m.MemType,
                    Score: contrib, Source: src, Rank: rank,
                    Reinforcement: m.ReinforcementCount, Neglect: m.NeglectCount,
                }
            }
        }
    }
    add(fts, "fts")
    add(vec, "vector")
    out := make([]Candidate, 0, len(byID))
    for _, c := range byID { out = append(out, *c) }
    return out
}

func (h *HybridRecaller) applySignalBoost(cands []Candidate) {
    for i := range cands {
        r := float64(cands[i].Reinforcement)
        n := float64(cands[i].Neglect)
        denom := r + n
        if denom <= 0 { continue }
        net := (r - n) / denom // [-1, 1]
        if net > 0 {
            cands[i].Score *= 1.0 + h.cfg.ReinforcementAlpha*net
        } else {
            cands[i].Score *= 1.0 + h.cfg.NeglectPenalty*net // net is negative
        }
    }
}
```

- [ ] **Step 3: Tests**

`hybrid_recaller_test.go` covers:

```go
func TestRRFFuse_OverlappingLists(t *testing.T) {
    // mem A: fts rank 1, vector rank 2 → highest combined score
    // mem B: fts rank 2 only
    // mem C: vector rank 1 only
    // assert ordering: A > C > B (since A appears in both)
}

func TestHybridRecaller_FallsBackWhenEmbedderMissing(t *testing.T) {
    // embedder nil → vector path returns 0, still gets BM25 results
}

func TestHybridRecaller_SignalBoost(t *testing.T) {
    // two candidates equal RRF, one with Reinforcement=10 / Neglect=0
    // → boosted one ranks first
}

func TestHybridRecaller_ExternalOnlyPassthrough(t *testing.T) {
    // store=nil, base returns 3 memories → 3 candidates, Source="base"
}

func TestHybridRecaller_BothSourcesFail(t *testing.T) {
    // mock storage returns error from both calls → propagates first error
}
```

Use a small fake `storage.Storage` (table-driven memory list) in tests; do not pull in the SQLite implementation. A minimal `embedder` stub returns a fixed vector.

- [ ] **Step 4: Commit**

```bash
git add agent/memorylayer/hybrid_recaller.go agent/memorylayer/hybrid_recaller_test.go
git commit -m "feat(memorylayer): HybridRecaller — BM25 + Vector + RRF + signal boost"
```

---

## Task 4: LLM Reranker

**Files:**
- **Create:** `agent/memorylayer/reranker.go`
- **Create:** `agent/memorylayer/reranker_test.go`
- **Create:** `agent/memorylayer/prompts/rerank.txt`

**Context:** Single-pass LLM reranker. The LLM receives `query` + the candidate list (with IDs) and returns the IDs in descending relevance. Failure or timeout → return input unchanged (recorded in `metadata.reranker_skipped`).

- [ ] **Step 1: Prompt template**

`agent/memorylayer/prompts/rerank.txt`:

```
You are a memory relevance reranker. Given a user query and a numbered
list of candidate memories, return the candidate IDs ordered from MOST
to LEAST relevant for answering the query.

Reply ONLY with a JSON array of IDs, e.g. ["m_42", "m_17", "m_5"].
Include EVERY input ID exactly once. If you cannot decide between two,
keep their relative input order.

User query:
{{QUERY}}

Candidates:
{{CANDIDATES}}
```

Use a `prompts/embed.go` (existing pattern in the repo for embedded files) OR `go:embed` the directory:

```go
package memorylayer

import _ "embed"

//go:embed prompts/rerank.txt
var rerankPromptTemplate string
```

- [ ] **Step 2: Reranker struct + interface**

```go
package memorylayer

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/odysseythink/pantheon/core"
)

type RerankerConfig struct {
    Enabled   bool
    BatchSize int           // cap candidates sent; default 20
    Timeout   time.Duration // default 1500 ms
}

func (c *RerankerConfig) fill() {
    if c.BatchSize <= 0 { c.BatchSize = 20 }
    if c.Timeout <= 0 { c.Timeout = 1500 * time.Millisecond }
}

type Reranker interface {
    Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error)
}

type LLMReranker struct {
    llm core.LanguageModel
    cfg RerankerConfig
}

func NewLLMReranker(llm core.LanguageModel, cfg RerankerConfig) *LLMReranker {
    cfg.fill()
    return &LLMReranker{llm: llm, cfg: cfg}
}

func (r *LLMReranker) Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error) {
    if !r.cfg.Enabled || r.llm == nil || len(cands) <= 1 {
        return trimCandidates(cands, topN), nil
    }
    if len(cands) > r.cfg.BatchSize {
        cands = cands[:r.cfg.BatchSize]
    }

    callCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
    defer cancel()

    prompt := renderRerankPrompt(query, cands)
    req := &core.Request{
        SystemPrompt: "You are a memory relevance reranker.",
        Messages: []core.Message{{
            Role:    core.MESSAGE_ROLE_USER,
            Content: []core.ContentParter{core.TextPart{Text: prompt}},
        }},
    }
    resp, err := r.llm.Generate(callCtx, req)
    if err != nil {
        return trimCandidates(cands, topN), nil // degrade silently
    }
    ids := parseRerankIDs(extractText(resp))
    if len(ids) == 0 {
        return trimCandidates(cands, topN), nil
    }

    byID := make(map[string]Candidate, len(cands))
    for _, c := range cands { byID[c.ID] = c }
    ordered := make([]Candidate, 0, len(cands))
    seen := make(map[string]bool, len(cands))
    for _, id := range ids {
        if c, ok := byID[id]; ok && !seen[id] {
            c.Source = "rerank"
            ordered = append(ordered, c)
            seen[id] = true
        }
    }
    // Append any candidates the LLM dropped, in original order.
    for _, c := range cands {
        if !seen[c.ID] { ordered = append(ordered, c) }
    }
    return trimCandidates(ordered, topN), nil
}

func renderRerankPrompt(query string, cands []Candidate) string {
    var b strings.Builder
    for i, c := range cands {
        fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, c.ID, truncate(c.Content, 200))
    }
    p := strings.ReplaceAll(rerankPromptTemplate, "{{QUERY}}", query)
    return strings.ReplaceAll(p, "{{CANDIDATES}}", b.String())
}

func parseRerankIDs(text string) []string {
    text = strings.TrimSpace(text)
    if i := strings.Index(text, "["); i >= 0 { text = text[i:] }
    if j := strings.LastIndex(text, "]"); j >= 0 { text = text[:j+1] }
    var ids []string
    if err := json.Unmarshal([]byte(text), &ids); err != nil { return nil }
    return ids
}

func trimCandidates(cs []Candidate, n int) []Candidate {
    if n <= 0 || n >= len(cs) { return cs }
    return cs[:n]
}

func truncate(s string, max int) string {
    if len(s) <= max { return s }
    return s[:max] + "…"
}

func extractText(resp *core.Response) string {
    if resp == nil { return "" }
    var b strings.Builder
    for _, part := range resp.Message.Content {
        if p, ok := part.(core.TextPart); ok { b.WriteString(p.Text) }
    }
    return b.String()
}
```

- [ ] **Step 3: Tests**

```go
func TestLLMReranker_Disabled(t *testing.T) {
    // cfg.Enabled=false → returns input trimmed to topN
}

func TestLLMReranker_HappyPath(t *testing.T) {
    // mock llm returns ["m_2","m_3","m_1"] → output ordered accordingly
}

func TestLLMReranker_LLMFails(t *testing.T) {
    // mock llm returns error → returns input unchanged (degraded), no error
}

func TestLLMReranker_LLMDropsCandidates(t *testing.T) {
    // mock llm returns subset → dropped IDs appended at the end
}

func TestLLMReranker_Timeout(t *testing.T) {
    // mock llm sleeps > timeout → returns input unchanged, no error
}

func TestParseRerankIDs_Robust(t *testing.T) {
    // tolerant of code fences and surrounding chatter
}
```

- [ ] **Step 4: Commit**

```bash
git add agent/memorylayer/reranker.go agent/memorylayer/reranker_test.go agent/memorylayer/prompts/rerank.txt
git commit -m "feat(memorylayer): LLM-as-reranker with timeout + silent degradation"
```

---

## Task 5: Boundary Detector (MemCell-lite)

**Files:**
- **Create:** `agent/memorylayer/boundary.go`
- **Create:** `agent/memorylayer/boundary_test.go`

**Context:** In-memory turn buffer. Emits a `Boundary` event when any of: token threshold reached, hard turn count reached, idle gap exceeded, or topic shift detected. Topic shift is a cheap cosine check (no LLM) between buffer head and tail embeddings. Phase 1 only **emits** boundaries — actual extractor dispatch is Task 6's job, lifecycle wiring is P2.

- [ ] **Step 1: Detector struct**

```go
package memorylayer

import (
    "context"
    "sync"
    "time"

    "github.com/odysseythink/hermind/tool/embedding"
    pembed "github.com/odysseythink/pantheon/extensions/embed"
)

type BoundaryConfig struct {
    HardTokenLimit            int           // default 8000
    HardTurnLimit             int           // default 20
    SoftTokenThreshold        int           // default 1500 (below this, topic shift not checked)
    IdleGap                   time.Duration // default 10 * time.Minute
    TopicShiftCosineThreshold float64       // default 0.55 (cosine < threshold = shift)
    EnableTopicShift          bool          // default true
}

func (c *BoundaryConfig) fill() {
    if c.HardTokenLimit <= 0 { c.HardTokenLimit = 8000 }
    if c.HardTurnLimit <= 0 { c.HardTurnLimit = 20 }
    if c.SoftTokenThreshold <= 0 { c.SoftTokenThreshold = 1500 }
    if c.IdleGap <= 0 { c.IdleGap = 10 * time.Minute }
    if c.TopicShiftCosineThreshold <= 0 { c.TopicShiftCosineThreshold = 0.55 }
}

type Turn struct {
    ID        int64
    UserMsg   string
    Assistant string
    Tokens    int
    Timestamp time.Time
    Embedding []float32 // optional; computed lazily for topic shift
}

type Boundary struct {
    Turns      []Turn
    TokenCount int
    Reason     string // "hard_token" | "hard_turn" | "idle" | "topic_shift" | "flush"
}

type BoundaryDetector struct {
    cfg      BoundaryConfig
    embedder embedding.Embedder // optional; required only if EnableTopicShift

    mu     sync.Mutex
    buf    []Turn
    tokens int
}

func NewBoundaryDetector(cfg BoundaryConfig, emb embedding.Embedder) *BoundaryDetector {
    cfg.fill()
    return &BoundaryDetector{cfg: cfg, embedder: emb}
}
```

- [ ] **Step 2: Observe + Flush**

```go
// Observe appends a turn. Returns a non-nil Boundary if a boundary just
// triggered; the buffer is reset in that case.
func (d *BoundaryDetector) Observe(ctx context.Context, t Turn) *Boundary {
    d.mu.Lock()
    defer d.mu.Unlock()

    // Idle gap is computed against the LAST turn in the existing buffer,
    // BEFORE appending the new one.
    if len(d.buf) > 0 && t.Timestamp.Sub(d.buf[len(d.buf)-1].Timestamp) > d.cfg.IdleGap {
        b := d.snapshotAndReset("idle")
        d.buf = append(d.buf, t)
        d.tokens = t.Tokens
        return b
    }

    d.buf = append(d.buf, t)
    d.tokens += t.Tokens

    switch {
    case d.tokens >= d.cfg.HardTokenLimit:
        return d.snapshotAndReset("hard_token")
    case len(d.buf) >= d.cfg.HardTurnLimit:
        return d.snapshotAndReset("hard_turn")
    case d.cfg.EnableTopicShift && d.tokens >= d.cfg.SoftTokenThreshold && d.detectTopicShift(ctx):
        return d.snapshotAndReset("topic_shift")
    }
    return nil
}

// Flush emits whatever is buffered with reason="flush". Used at shutdown.
func (d *BoundaryDetector) Flush() *Boundary {
    d.mu.Lock()
    defer d.mu.Unlock()
    if len(d.buf) == 0 { return nil }
    return d.snapshotAndReset("flush")
}

func (d *BoundaryDetector) snapshotAndReset(reason string) *Boundary {
    b := &Boundary{
        Turns:      append([]Turn(nil), d.buf...),
        TokenCount: d.tokens,
        Reason:     reason,
    }
    d.buf = d.buf[:0]
    d.tokens = 0
    return b
}

// detectTopicShift compares the embedding of buf[0] vs buf[last]. It
// computes embeddings lazily and caches them on the turns in the buffer.
func (d *BoundaryDetector) detectTopicShift(ctx context.Context) bool {
    if d.embedder == nil || len(d.buf) < 2 { return false }
    head := &d.buf[0]
    tail := &d.buf[len(d.buf)-1]
    if head.Embedding == nil {
        if v, err := d.embedder.Embed(ctx, head.UserMsg); err == nil { head.Embedding = v }
    }
    if tail.Embedding == nil {
        if v, err := d.embedder.Embed(ctx, tail.UserMsg); err == nil { tail.Embedding = v }
    }
    if head.Embedding == nil || tail.Embedding == nil { return false }
    cos := pembed.Cosine(head.Embedding, tail.Embedding)
    return float64(cos) < d.cfg.TopicShiftCosineThreshold
}
```

- [ ] **Step 3: Tests**

```go
func TestBoundary_HardTokenLimit(t *testing.T) {
    // accumulate turns until tokens >= 8000 → boundary "hard_token", buffer reset
}

func TestBoundary_HardTurnLimit(t *testing.T) {
    // 20 turns → boundary "hard_turn"
}

func TestBoundary_IdleGap(t *testing.T) {
    // turn A at t=0, turn B at t=11min → first Observe returns nil,
    // second Observe returns boundary "idle" containing only A; new buffer starts with B
}

func TestBoundary_TopicShift(t *testing.T) {
    // stub embedder: head returns [1,0,0,...], tail returns [0,1,0,...] (cosine=0)
    // soft threshold met → boundary "topic_shift"
}

func TestBoundary_TopicShiftDisabledBelowSoftThreshold(t *testing.T) {
    // tokens < 1500 → topic shift not checked even if embeddings divergent
}

func TestBoundary_Flush(t *testing.T) {
    // buffer non-empty → Flush returns it; empty → nil
}

func TestBoundary_NoEmbedderSilentlySkipsShift(t *testing.T) {
    // embedder nil + EnableTopicShift=true → behaves as if disabled
}
```

- [ ] **Step 4: Commit**

```bash
git add agent/memorylayer/boundary.go agent/memorylayer/boundary_test.go
git commit -m "feat(memorylayer): MemCell-lite boundary detector"
```

---

## Task 6: Taxonomy Extractor (with provenance)

**Files:**
- **Create:** `agent/memorylayer/extractor.go`
- **Create:** `agent/memorylayer/extractor_test.go`
- **Create:** `agent/memorylayer/prompts/taxonomy_extract.txt`

**Context:** Replaces MetaClaw's 3-type ad-hoc extraction with a 4-type LLM extraction over a `Boundary` (not a single turn). Every output memory carries `ParentTurnID` (last turn in the boundary) — the new field shipped in Task 1.

- [ ] **Step 1: Prompt template**

`agent/memorylayer/prompts/taxonomy_extract.txt`:

```
You extract durable memories from a multi-turn conversation segment.

For each memory worth keeping, classify into ONE of:
- "core":      user explicitly asked to remember forever (e.g. "I'm allergic to peanuts")
- "episode":   a narrative summary of WHAT happened in this segment
- "fact":      a discrete, atomic fact (one sentence; "the project uses pnpm")
- "foresight": a time-bound plan or prediction; include "expires_at" ISO-8601 if discoverable

Reply ONLY with a JSON array. Each item:
  {"type":"core|episode|fact|foresight","content":"…","confidence":0.0-1.0,"expires_at":"YYYY-MM-DDTHH:MM:SSZ"}
"expires_at" is required for foresight, omitted otherwise.
If nothing is worth remembering, reply [].

Conversation segment ({{TURN_COUNT}} turns):
{{TURNS}}
```

```go
//go:embed prompts/taxonomy_extract.txt
var taxonomyPromptTemplate string
```

- [ ] **Step 2: Extractor**

```go
package memorylayer

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/odysseythink/hermind/storage"
    "github.com/odysseythink/pantheon/core"
)

type TaxonomyConfig struct {
    Enabled       bool
    MaxOutputs    int           // default 8 per boundary
    Timeout       time.Duration // default 6 * time.Second
    AllowedTypes  []string      // default {"core","episode","fact","foresight"}
}

func (c *TaxonomyConfig) fill() {
    if c.MaxOutputs <= 0 { c.MaxOutputs = 8 }
    if c.Timeout <= 0 { c.Timeout = 6 * time.Second }
    if len(c.AllowedTypes) == 0 {
        c.AllowedTypes = []string{"core", "episode", "fact", "foresight"}
    }
}

type TaxonomyExtractor struct {
    llm core.LanguageModel
    cfg TaxonomyConfig
}

func NewTaxonomyExtractor(llm core.LanguageModel, cfg TaxonomyConfig) *TaxonomyExtractor {
    cfg.fill()
    return &TaxonomyExtractor{llm: llm, cfg: cfg}
}

type extractedItem struct {
    Type       string  `json:"type"`
    Content    string  `json:"content"`
    Confidence float64 `json:"confidence"`
    ExpiresAt  string  `json:"expires_at,omitempty"`
}

// Extract runs the LLM extractor over a Boundary and returns Memory rows
// ready for storage. Caller is responsible for SaveMemory.
func (e *TaxonomyExtractor) Extract(ctx context.Context, b *Boundary) ([]*storage.Memory, error) {
    if !e.cfg.Enabled || e.llm == nil || b == nil || len(b.Turns) == 0 {
        return nil, nil
    }
    callCtx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
    defer cancel()

    prompt := renderTaxonomyPrompt(b)
    req := &core.Request{
        SystemPrompt: "You extract durable structured memories from conversations.",
        Messages: []core.Message{{
            Role:    core.MESSAGE_ROLE_USER,
            Content: []core.ContentParter{core.TextPart{Text: prompt}},
        }},
    }
    resp, err := e.llm.Generate(callCtx, req)
    if err != nil { return nil, err }

    items := parseExtracted(extractText(resp))
    if len(items) > e.cfg.MaxOutputs { items = items[:e.cfg.MaxOutputs] }

    allowed := make(map[string]bool, len(e.cfg.AllowedTypes))
    for _, t := range e.cfg.AllowedTypes { allowed[t] = true }

    lastTurn := b.Turns[len(b.Turns)-1]
    now := time.Now().UTC()
    out := make([]*storage.Memory, 0, len(items))
    for _, it := range items {
        if !allowed[it.Type] || strings.TrimSpace(it.Content) == "" { continue }
        m := &storage.Memory{
            Content:      it.Content,
            MemType:      it.Type,
            ParentTurnID: lastTurn.ID,
            CreatedAt:    now,
            UpdatedAt:    now,
            Status:       "active",
        }
        if it.Type == "foresight" {
            if exp, err := time.Parse(time.RFC3339, it.ExpiresAt); err == nil {
                m.ExpiresAt = exp.UTC()
            } else {
                // default: 7 days out, configurable in P3
                m.ExpiresAt = now.AddDate(0, 0, 7)
            }
        }
        out = append(out, m)
    }
    return out, nil
}

func renderTaxonomyPrompt(b *Boundary) string {
    var lines strings.Builder
    for _, t := range b.Turns {
        fmt.Fprintf(&lines, "[turn %d]\nuser: %s\nassistant: %s\n\n", t.ID, t.UserMsg, t.Assistant)
    }
    p := strings.ReplaceAll(taxonomyPromptTemplate, "{{TURN_COUNT}}", fmt.Sprintf("%d", len(b.Turns)))
    return strings.ReplaceAll(p, "{{TURNS}}", lines.String())
}

func parseExtracted(text string) []extractedItem {
    text = strings.TrimSpace(text)
    if i := strings.Index(text, "["); i >= 0 { text = text[i:] }
    if j := strings.LastIndex(text, "]"); j >= 0 { text = text[:j+1] }
    var items []extractedItem
    if err := json.Unmarshal([]byte(text), &items); err != nil { return nil }
    return items
}
```

- [ ] **Step 3: Tests**

```go
func TestTaxonomyExtractor_Disabled(t *testing.T) {
    // returns nil without calling llm
}

func TestTaxonomyExtractor_HappyPath4Types(t *testing.T) {
    // mock llm returns 4 items (one per type) → 4 Memory rows with right MemType
    // foresight row has ExpiresAt set from JSON
    // every row has ParentTurnID == last turn id
}

func TestTaxonomyExtractor_DefaultsForesightExpiry(t *testing.T) {
    // foresight item with empty expires_at → ExpiresAt ≈ now + 7d
}

func TestTaxonomyExtractor_RejectsUnknownType(t *testing.T) {
    // llm returns {"type":"skill",…} → skipped (skill not in AllowedTypes)
}

func TestTaxonomyExtractor_RespectsMaxOutputs(t *testing.T) {
    // llm returns 20 items, MaxOutputs=8 → 8 rows
}

func TestTaxonomyExtractor_BadJSONReturnsEmpty(t *testing.T) {
    // llm returns garbage → no error, no rows
}

func TestTaxonomyExtractor_Timeout(t *testing.T) {
    // llm sleeps > timeout → returns err == context.DeadlineExceeded
}
```

- [ ] **Step 4: Commit**

```bash
git add agent/memorylayer/extractor.go agent/memorylayer/extractor_test.go agent/memorylayer/prompts/taxonomy_extract.txt
git commit -m "feat(memorylayer): 4-type taxonomy extractor with ParentTurnID provenance"
```

---

## Task 7: Layer composition root

**Files:**
- **Create:** `agent/memorylayer/layer.go`
- **Create:** `agent/memorylayer/layer_test.go`

**Context:** A single `MemoryLayer` struct holds the wired components and exposes:
1. `Recall(ctx, query, limit)` → goes through HybridRecaller + Reranker, returns `[]InjectedMemory`.
2. `ObserveTurn(ctx, turn)` → feeds the boundary detector and, on boundary, runs the extractor + persists.

Lifecycle hooks (P2.2) will be a thin wrapper on top of this.

- [ ] **Step 1: Struct + constructor**

```go
package memorylayer

import (
    "context"

    "github.com/odysseythink/hermind/storage"
    "github.com/odysseythink/hermind/tool/embedding"
    "github.com/odysseythink/hermind/tool/memory/memprovider"
    "github.com/odysseythink/mlog"
    "github.com/odysseythink/pantheon/core"
)

type Config struct {
    Hybrid    HybridConfig
    Reranker  RerankerConfig
    Boundary  BoundaryConfig
    Taxonomy  TaxonomyConfig
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
    if cfg.RecallLimit <= 0 { cfg.RecallLimit = 5 }
    return &MemoryLayer{
        store:     store,
        hybrid:    NewHybridRecaller(store, emb, base, cfg.Hybrid),
        reranker:  NewLLMReranker(llm, cfg.Reranker),
        boundary:  NewBoundaryDetector(cfg.Boundary, emb),
        extractor: NewTaxonomyExtractor(llm, cfg.Taxonomy),
        cfg:       cfg,
    }
}
```

- [ ] **Step 2: Recall**

```go
// Recall is the single retrieval entry point. limit overrides the
// configured RecallLimit when > 0.
func (l *MemoryLayer) Recall(ctx context.Context, query string, limit int) ([]memprovider.InjectedMemory, error) {
    if limit <= 0 { limit = l.cfg.RecallLimit }
    cands, err := l.hybrid.Recall(ctx, query, limit)
    if err != nil { return nil, err }
    if len(cands) == 0 { return nil, nil }
    ranked, _ := l.reranker.Rerank(ctx, query, cands, limit)
    return candidatesToInjected(ranked, limit), nil
}
```

- [ ] **Step 3: ObserveTurn**

```go
// ObserveTurn feeds the boundary detector. If a boundary fires, the
// extractor runs synchronously in a detached goroutine (caller's ctx
// is not held). Errors are logged but never returned — observation
// must not slow the turn.
func (l *MemoryLayer) ObserveTurn(ctx context.Context, t Turn) {
    b := l.boundary.Observe(ctx, t)
    if b == nil { return }
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

func itoa(n int) string { return fmt.Sprintf("%d", n) } // keep imports minimal
```

(Adjust imports — `fmt` and `mlog` — to whatever's already idiomatic.)

- [ ] **Step 4: Tests**

```go
func TestMemoryLayer_RecallEnd2End(t *testing.T) {
    // fake storage with 5 memories, mock reranker echoing input
    // Recall returns Top-3 InjectedMemory
}

func TestMemoryLayer_ObserveTurn_NoBoundary(t *testing.T) {
    // single short turn → no extractor calls
}

func TestMemoryLayer_ObserveTurn_BoundaryPersists(t *testing.T) {
    // feed turns until hard_turn → extractor called, SaveMemory called n times,
    // memory_events row appended
}

func TestMemoryLayer_FlushAtShutdown(t *testing.T) {
    // leave 3 turns in buffer → Flush extracts
}
```

- [ ] **Step 5: Commit**

```bash
git add agent/memorylayer/layer.go agent/memorylayer/layer_test.go
git commit -m "feat(memorylayer): composition root with Recall + ObserveTurn"
```

---

## Task 8: Configuration wiring

**Files:**
- **Modify:** `config/config.go` (and `config/defaults.go` if defaults are split)

**Context:** Mirrors `memory_layer:` keys defined in design §11. Phase 1 only ships keys consumed by Phase 1 components.

- [ ] **Step 1: Config structs**

```go
// MemoryLayerConfig is the on-disk shape of the memory layer.
// Subsystems can be independently disabled; there is no top-level enabled flag
// (any subsystem you disable falls back to the prior behavior).
type MemoryLayerConfig struct {
    Hybrid    HybridConfig    `yaml:"hybrid"`
    Reranker  RerankerConfig  `yaml:"reranker"`
    Boundary  BoundaryConfig  `yaml:"boundary"`
    Taxonomy  TaxonomyConfig  `yaml:"taxonomy"`
    RecallLimit int           `yaml:"recall_limit"`
}

type HybridConfig struct {
    Enabled                bool    `yaml:"enabled"`
    RRFConstant            float64 `yaml:"rrf_k"`
    BM25TopNMultiplier     int     `yaml:"bm25_top_n_multiplier"`
    VectorTopNMultiplier   int     `yaml:"vector_top_n_multiplier"`
    PreRerankTopKMultiplier int    `yaml:"pre_rerank_top_k_multiplier"`
    ReinforcementAlpha     float64 `yaml:"reinforcement_alpha"`
    NeglectPenalty         float64 `yaml:"neglect_penalty"`
}

type RerankerConfig struct {
    Enabled   bool          `yaml:"enabled"`
    BatchSize int           `yaml:"batch_size"`
    TimeoutMS int           `yaml:"timeout_ms"`
}

type BoundaryConfig struct {
    HardTokenLimit            int     `yaml:"hard_token_limit"`
    HardTurnLimit             int     `yaml:"hard_turn_limit"`
    SoftTokenThreshold        int     `yaml:"soft_token_threshold"`
    IdleGapMinutes            int     `yaml:"idle_gap_minutes"`
    EnableTopicShift          bool    `yaml:"topic_shift_enabled"`
    TopicShiftCosineThreshold float64 `yaml:"topic_shift_cosine_threshold"`
}

type TaxonomyConfig struct {
    Enabled      bool     `yaml:"enabled"`
    MaxOutputs   int      `yaml:"max_outputs"`
    TimeoutMS    int      `yaml:"timeout_ms"`
    AllowedTypes []string `yaml:"types"`
}
```

Add `MemoryLayer MemoryLayerConfig \`yaml:"memory_layer"\`` to the top-level `AgentConfig` or `Config` (whichever is the conventional spot — check neighboring config additions, e.g. `Memory.MetaClaw`).

- [ ] **Step 2: Defaults**

In `config/defaults.go` (or wherever defaults are seeded), set:

```go
MemoryLayer: MemoryLayerConfig{
    Hybrid:   HybridConfig{Enabled: true, RRFConstant: 60, BM25TopNMultiplier: 3, VectorTopNMultiplier: 3, PreRerankTopKMultiplier: 2, ReinforcementAlpha: 0.15, NeglectPenalty: 0.10},
    Reranker: RerankerConfig{Enabled: true, BatchSize: 20, TimeoutMS: 1500},
    Boundary: BoundaryConfig{HardTokenLimit: 8000, HardTurnLimit: 20, SoftTokenThreshold: 1500, IdleGapMinutes: 10, EnableTopicShift: true, TopicShiftCosineThreshold: 0.55},
    Taxonomy: TaxonomyConfig{Enabled: true, MaxOutputs: 8, TimeoutMS: 6000, AllowedTypes: []string{"core","episode","fact","foresight"}},
    RecallLimit: 5,
},
```

- [ ] **Step 3: Translation helpers**

Tiny adapters from `config.HybridConfig` → `memorylayer.HybridConfig` (and the same for the other three). Put them in `cli/engine_deps.go` next to where the layer is constructed — these aren't worth a dedicated file.

- [ ] **Step 4: Commit**

```bash
git add config/config.go config/defaults.go
git commit -m "feat(config): MemoryLayer configuration (Hybrid/Reranker/Boundary/Taxonomy)"
```

---

## Task 9: Wire MemoryLayer into the API server

**Files:**
- **Modify:** `cli/engine_deps.go`
- **Modify:** `api/server.go`
- **Modify:** `agent/conversation.go` (or whichever file invokes `SyncTurn` per turn — confirm by greping)

**Context:** Where `api/server.go` currently builds a recall callback from `r.Recall(ctx, userMsg, memK)`, route through `MemoryLayer.Recall` instead **when the underlying provider is a Recaller AND `MemoryLayerConfig.Hybrid.Enabled`**. Also tap the per-turn point that already calls `SyncTurn` to additionally call `layer.ObserveTurn`.

- [ ] **Step 1: Construct MemoryLayer in `engine_deps.go`**

After the existing `MemProvider` is built, add:

```go
// MemoryLayer is constructed when hybrid retrieval is enabled and the
// underlying provider implements Recaller. A nil layer is a no-op.
var memLayer *memorylayer.MemoryLayer
if cfg.MemoryLayer.Hybrid.Enabled {
    if r, ok := memProvider.(memprovider.Recaller); ok && storage != nil {
        memLayer = memorylayer.New(
            storage,
            embedder,           // existing embedding.Embedder instance
            r,                  // for external-only fallback path
            llm,                // for reranker + extractor
            translateMemoryLayerConfig(cfg.MemoryLayer),
        )
    } else {
        mlog.Info("memorylayer: skipped (provider has no Recaller or no storage)")
    }
}
```

Export `memLayer` via the existing `EngineDeps` struct (whatever wraps these for `api/server.go` consumption).

- [ ] **Step 2: Use `memLayer.Recall` in `api/server.go`**

Replace the inline lambda around line 434 of `api/server.go`:

```go
if r, ok := deps.MemProvider.(memprovider.Recaller); ok {
    mc := s.opts.Config.Memory.MetaClaw
    memK := mc.InjectCount
    if memK <= 0 { memK = 3 }

    recallFn := func(ctx context.Context, userMsg string) []memprovider.InjectedMemory {
        if deps.MemoryLayer != nil {
            out, _ := deps.MemoryLayer.Recall(ctx, userMsg, memK)
            return out
        }
        out, _ := r.Recall(ctx, userMsg, memK)
        return out
    }
    eng.SetActiveMemoriesProvider(recallFn)
    // ...rest unchanged
}
```

- [ ] **Step 3: Observe each turn**

In the per-turn hook that already calls `Provider.SyncTurn` (likely in `agent/conversation.go` — grep first to confirm), add:

```go
if deps.MemoryLayer != nil {
    deps.MemoryLayer.ObserveTurn(ctx, memorylayer.Turn{
        ID:        turnID,
        UserMsg:   userMsg,
        Assistant: assistantMsg,
        Tokens:    estimateTokens(userMsg) + estimateTokens(assistantMsg),
        Timestamp: time.Now().UTC(),
    })
}
```

Use the existing token estimator (or `len/4` approximation if no better signal exists at that site — leave a TODO).

- [ ] **Step 4: Flush on shutdown**

Wherever `Provider.Shutdown` is called (Engine shutdown path), call `memLayer.Flush(ctx)` first.

- [ ] **Step 5: Build + smoke test**

```bash
go build ./...
go test ./agent/memorylayer/...
go test ./storage/sqlite/...
```

- [ ] **Step 6: Commit**

```bash
git add cli/engine_deps.go api/server.go agent/conversation.go
git commit -m "feat(memorylayer): wire HybridRecaller + boundary into api/server"
```

---

## Task 10: Integration test

**Files:**
- **Create:** `agent/memorylayer/integration_test.go`

**Context:** End-to-end: spin a real `storage/sqlite` (in-memory), insert seed memories, mock the LLM, run `MemoryLayer.Recall` and `MemoryLayer.ObserveTurn` for a 25-turn synthetic conversation. Assert: boundary fires at least once, taxonomy extractor produces ≥ 1 memory per fired boundary, `boundary.detected` event is recorded, `Recall` returns hybrid-ranked results.

- [ ] **Step 1: Test scaffold**

```go
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
)

func TestIntegration_HybridRecall_AfterBoundary(t *testing.T) {
    store, err := sqlite.Open(":memory:")
    if err != nil { t.Fatal(err) }
    defer store.Close()
    if err := store.Migrate(); err != nil { t.Fatal(err) }

    llm := &stubLLM{
        // First call (taxonomy): returns 2 facts
        // Second call (rerank): echoes input order
    }
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
            ID: int64(i),
            UserMsg: fmt.Sprintf("question %d about Go modules", i),
            Assistant: "answer",
            Tokens: 50, Timestamp: time.Now(),
        })
    }
    // Boundary fires on the 5th turn; extractor runs in goroutine.
    // Give it a moment to drain (test stub LLM is synchronous).
    time.Sleep(50 * time.Millisecond)

    // Assertions:
    mems, _ := store.SearchMemories(ctx, "Go modules", &storage.MemorySearchOptions{Limit: 10})
    if len(mems) == 0 { t.Fatal("expected extracted memories") }
    for _, m := range mems {
        if m.ParentTurnID == 0 { t.Errorf("memory %q missing ParentTurnID", m.ID) }
    }

    out, _ := layer.Recall(ctx, "Go modules", 3)
    if len(out) == 0 { t.Fatal("expected recall hits") }

    events, _ := store.ListMemoryEvents(ctx, 10, 0, []string{"boundary.detected"})
    if len(events) == 0 { t.Error("expected boundary.detected event") }
}
```

(Define `stubLLM` and `stubEmbedder` inline; mirror the patterns already used in `agent/*_test.go`.)

- [ ] **Step 2: Run**

```bash
go test -run TestIntegration_HybridRecall_AfterBoundary ./agent/memorylayer/...
```

- [ ] **Step 3: Commit**

```bash
git add agent/memorylayer/integration_test.go
git commit -m "test(memorylayer): integration — boundary → extract → hybrid recall"
```

---

## Task 11: Manual verification + docs

**Files:**
- **Modify:** `.gpowers/designs/2026-05-20-memory-layer-design.md` (mark Phase 1 status)
- **Modify:** `CHANGELOG.md` (or wherever release notes go)

**Context:** No new design content — just close the loop and signal that Phase 1 is shipped.

- [ ] **Step 1: Run the full build + tests**

```bash
go build ./...
go test ./...
```

Fix any unrelated breakages by the simplest change that keeps tests green.

- [ ] **Step 2: Smoke test the binary**

```bash
go run ./cmd/hermind
# In REPL: drive 5+ turns, watch logs for "boundary.detected" event
```

- [ ] **Step 3: Update design doc status**

Edit the top of `.gpowers/designs/2026-05-20-memory-layer-design.md`:

```
> **Status**: Phase 1 shipped 2026-06-XX. Phase 2/3 plans to follow.
```

- [ ] **Step 4: Changelog entry**

In `CHANGELOG.md` under an `Unreleased` section:

```
### Added
- Memory Layer Phase 1: Hybrid Retrieval (BM25+Vector+RRF), LLM Reranker,
  MemCell-lite boundary detection, 4-type Taxonomy Extractor with
  ParentTurnID provenance. Storage schema v10.
```

- [ ] **Step 5: Commit**

```bash
git add .gpowers/designs/2026-05-20-memory-layer-design.md CHANGELOG.md
git commit -m "docs(memorylayer): Phase 1 status + changelog"
```

---

## Risk register (Phase 1)

| Risk | Mitigation |
|---|---|
| Storage v10 migration breaks existing dbs | All `ALTER TABLE` with `duplicate column name` tolerance (matches v4–v9 pattern); each ALTER is idempotent. Add a `TestMigrate_v9_to_v10` in `migrate_test.go` (Task 1, Step 5). |
| RRF makes search slower than current weighted-sum | Reranker stage skips when ≤ 1 candidate; HybridRecaller fetches concurrently; reranker batch ≤ 20. Expected total ≤ 250 ms (design §13.3). |
| Reranker LLM errors degrade UX silently | Reranker returns unchanged input on timeout/error; log line at WARN; design accepts this as the intended behavior. |
| Boundary fires too aggressively in long topical sessions | `HardTurnLimit=20` is conservative; `EnableTopicShift` is the only "smart" trigger and can be turned off. |
| Goroutine in `ObserveTurn` leaks if extractor hangs | Extractor has a hard timeout (default 6s). Worst case: one stale goroutine per stuck call; acceptable. |
| `InjectedMemory` lacking score blocks P2 (Agentic needs scores) | P2 plan will widen `InjectedMemory` OR keep `Candidate` internal and only widen on the Agentic boundary. Not blocking P1. |

---

## Done criteria

Phase 1 is complete when:

1. `go test ./...` is green on a fresh checkout.
2. `migrate_test.go` proves v9→v10 idempotent on existing test DBs.
3. Manual REPL shows: turns flow normally; after ~5 turns or `HardTokenLimit`, a `boundary.detected` event appears in `memory_events`; the next `Recall` shows freshly extracted memories.
4. Search ordering visibly responds to `ReinforcementCount` (artificially bump one, verify it climbs).
5. `Memory.ParentTurnID` is populated on every newly extracted row.
6. Design doc top is updated; changelog entry exists.

---

## Phase 2 / Phase 3 — what to plan next

After this lands, write a Phase 2 plan covering:

- `agent/memorylayer/agentic.go` — multi-round wrapper around `MemoryLayer.Recall`
- Lifecycle hooks (`OnSessionStart` / `OnTurnComplete`) — engine integration, pinned memories rendering
- `applySynergyBudget` integration with pinned memories

And a Phase 3 plan for:

- Living Profile (independent table + `ProfileUpdater`)
- Foresight expiry + Consolidate extension
- Skill candidate emitter → `skills.Evolver` (kill the v1 standalone SkillExtractor)

Both depend on the `MemoryLayer` and `Candidate` types defined here. Do not start them before Phase 1's interfaces are stable.

---

*Drafted 2026-05-21 against memory layer design v2.*
