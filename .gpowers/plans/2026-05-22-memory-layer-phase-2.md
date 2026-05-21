# Memory Layer Phase 2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Source Design:** `.gpowers/designs/2026-05-20-memory-layer-design.md` (v2, 2026-05-21).
**Predecessor Plan:** `.gpowers/plans/2026-05-21-memory-layer-phase-1.md` (shipped through commit `91aac49`).

**Scope (P2 only):**
- §5 Agentic multi-round (sufficiency shortcut + 1 extra round + 2 sub-queries + per-turn / per-session token caps + timeout fallback)
- §10 Lifecycle hooks — **2 hooks only**: `OnSessionStart` and a thickened `OnTurnComplete`
- §10.1 Pinned memories: core + foresight (≤ 7 days ahead) preloaded once per session, rendered as a dedicated prompt section that bypasses synergy budget
- Wire Agentic + Lifecycle into `MemoryLayer.Recall` and `api/server.go` / `agent/engine.go`

**Out of scope (deferred to Phase 3 plan):**
- Living Profile object + `ProfileUpdater`
- Foresight expiry archival in `Consolidate` (the `ExpiresAt` field is **read** by lifecycle preload in this phase; consolidate write-back stays for P3)
- Skill candidate emitter → `skills.Evolver` integration
- `working_summary` generalization (MetaClaw's existing implementation keeps running; the lifecycle hook will *not* replace it in P2)
- Clustering (P4)

**Goal:** Make `MemoryLayer.Recall` smart enough to do a second LLM-guided pass on hard queries, and inject the user's pinned context (core + near-term foresight) at every turn without paying the synergy-budget tax.

**Architecture:** `agent/memorylayer` gains two new files — `agentic.go` (wraps the existing `Recall`) and `lifecycle.go` (drives `OnSessionStart`). Engine gets a `pinnedMemories` field and a render path in `prompt.go`. No changes to `Provider` / `Recaller` / `tool.Registry` interfaces.

**Tech Stack:** Go 1.22+, existing `pantheon/core.LanguageModel`, `storage.Storage`, `testify`. No new go.mod deps.

---

## File Structure

```
agent/memorylayer/                                (existing package)
├── agentic.go                                    (new — multi-round wrapper)
├── agentic_test.go                               (new)
├── lifecycle.go                                  (new — OnSessionStart + helpers)
├── lifecycle_test.go                             (new)
├── tokencap.go                                   (new — per-session token tracker)
├── tokencap_test.go                              (new)
├── layer.go                                      (modify — compose Agentic, expose lifecycle entry)
├── layer_test.go                                 (modify — agentic + lifecycle integration)
└── prompts/
    ├── sufficiency_check.txt                     (new)
    └── query_expansion.txt                       (new)

agent/engine.go                                   (modify — pinnedMemories field + getter/setter)
agent/prompt.go                                   (modify — PromptOptions.PinnedMemories + renderPinned)
agent/prompt_test.go                              (modify — pinned ordering + budget bypass)
agent/conversation.go                             (modify — Build prompt with PinnedMemories from engine)

api/server.go                                     (modify — OnSessionStart on run start; PinnedMemories provider hookup)
cli/engine_deps.go                                (modify — pass AgenticConfig + LifecycleConfig)

config/config.go                                  (modify — AgenticConfig + LifecycleConfig sub-structs)
config/defaults.go                                (modify — P2 defaults)
```

---

## Dependencies

All new code depends on what Phase 1 shipped plus:
- `agent.Engine` (gets a new `pinnedMemories []memprovider.InjectedMemory` field + setter)
- `agent.PromptBuilder` (renders pinned section above active memories)
- `storage.Storage.SearchMemories` (Phase 1 `MemTypes` + `IncludeExpired` filters)

No new go.mod entries.

---

## Task 1: Per-session token cap tracker

**Files:**
- **Create:** `agent/memorylayer/tokencap.go`
- **Create:** `agent/memorylayer/tokencap_test.go`

**Context:** Agentic adds LLM calls for sufficiency-check + query-expansion. Caps must be enforced both per-turn (single Recall must not blow up) and per-session (a chatty user shouldn't burn unbounded tokens across the day). Tracker is a tiny stateful object instantiated once per session.

- [ ] **Step 1: Tracker struct**

```go
package memorylayer

import (
    "sync"
    "sync/atomic"
)

// TokenCap tracks Agentic LLM token spend within a session. The
// per-turn budget is enforced by Reset+Allow checks during a single
// Recall; the per-session budget persists across calls.
type TokenCap struct {
    perTurn    int
    perSession int

    sessionUsed atomic.Int64

    mu       sync.Mutex
    turnUsed int
}

func NewTokenCap(perTurn, perSession int) *TokenCap {
    return &TokenCap{perTurn: perTurn, perSession: perSession}
}

// ResetTurn zeros the per-turn counter. Call at the start of each Recall.
func (t *TokenCap) ResetTurn() {
    t.mu.Lock()
    t.turnUsed = 0
    t.mu.Unlock()
}

// Allow reports whether spending `cost` more tokens would stay under both
// caps. If yes, the cost is recorded. If no, returns false and leaves
// counters untouched.
func (t *TokenCap) Allow(cost int) bool {
    if cost < 0 { cost = 0 }
    sessTotal := t.sessionUsed.Load() + int64(cost)
    if t.perSession > 0 && sessTotal > int64(t.perSession) {
        return false
    }
    t.mu.Lock()
    if t.perTurn > 0 && t.turnUsed+cost > t.perTurn {
        t.mu.Unlock()
        return false
    }
    t.turnUsed += cost
    t.mu.Unlock()
    t.sessionUsed.Add(int64(cost))
    return true
}

// SessionUsed returns the current session-wide spend (for telemetry).
func (t *TokenCap) SessionUsed() int64 { return t.sessionUsed.Load() }
```

- [ ] **Step 2: Tests**

```go
func TestTokenCap_PerTurnBlocks(t *testing.T) {
    // perTurn=100, perSession=10000
    // Allow(60) ok; Allow(60) blocked (would be 120 > 100)
}

func TestTokenCap_PerSessionBlocksAcrossTurns(t *testing.T) {
    // perTurn=100, perSession=150
    // Turn 1: ResetTurn + Allow(80) ok
    // Turn 2: ResetTurn + Allow(80) blocked (session total would be 160 > 150)
}

func TestTokenCap_ResetTurnDoesNotResetSession(t *testing.T) {
    // After several turns, SessionUsed accumulates monotonically
}

func TestTokenCap_ZeroCapsMeansUnlimited(t *testing.T) {
    // perTurn=0, perSession=0 → every Allow returns true
}

func TestTokenCap_ConcurrentAllow(t *testing.T) {
    // 100 goroutines calling Allow(10) with perSession=500
    // → exactly 50 succeed
}
```

- [ ] **Step 3: Commit**

```bash
git add agent/memorylayer/tokencap.go agent/memorylayer/tokencap_test.go
git commit -m "feat(memorylayer): per-turn/per-session token cap tracker"
```

---

## Task 2: Sufficiency check + query expansion prompts

**Files:**
- **Create:** `agent/memorylayer/prompts/sufficiency_check.txt`
- **Create:** `agent/memorylayer/prompts/query_expansion.txt`

**Context:** Two short LLM prompts. Both return strict JSON for cheap parsing.

- [ ] **Step 1: `sufficiency_check.txt`**

```
You are a memory-retrieval critic. Given a user query and a list of
memories already retrieved, decide whether those memories are sufficient
to answer or fulfill the query.

Reply ONLY with a JSON object on a single line:
  {"sufficient": true}   — when the memories cover the query
  {"sufficient": false, "missing": "<one short phrase describing what's missing>"}

Be strict: if the query asks about something not directly covered, return false.

User query:
{{QUERY}}

Retrieved memories ({{COUNT}}):
{{MEMORIES}}
```

- [ ] **Step 2: `query_expansion.txt`**

```
You generate complementary retrieval queries. The user asked the
question below, and the first retrieval pass missed key context (the
critic's note is included).

Produce exactly {{N}} additional short queries that probe DIFFERENT
angles than the original. Avoid paraphrases. Each query should be a
single line, no numbering.

Reply ONLY with a JSON array of strings, e.g. ["...", "..."].

Original user query:
{{QUERY}}

Critic's note on what was missing:
{{MISSING}}

Already-retrieved memory snippets (so you don't re-query the same thing):
{{MEMORIES}}
```

- [ ] **Step 3: Embed both via `go:embed` in `agentic.go`** (no commit yet — files are committed alongside Task 3.)

---

## Task 3: Agentic multi-round wrapper

**Files:**
- **Create:** `agent/memorylayer/agentic.go`
- **Create:** `agent/memorylayer/agentic_test.go`

**Context:** Wraps `MemoryLayer.Recall` (the Phase 1 surface). On every call:
1. Run base `Recall`. If empty → return.
2. **Shortcut:** if top candidate's RRF+rerank score is already above `ShortcutThreshold` (default 0.85), skip the LLM critic.
3. Otherwise ask the critic. `SUFFICIENT` → return.
4. `INSUFFICIENT` → ask LLM for `ExpansionQueries` extra queries. Run them in parallel through base `Recall`. RRF-fuse across query result lists, then push through the existing reranker (already inside `Recall`'s pipeline — call it again with the fused candidates).
5. Any failure / timeout / cap hit → return the original M1 (silent degradation, logged at WARN with `metadata` markers).

Agentic operates on `Candidate` lists, not `InjectedMemory` — so it needs access to the un-collapsed candidates. Add a small extension to `MemoryLayer` so Agentic can request "give me reranked Candidates" rather than `[]InjectedMemory`.

- [ ] **Step 1: Expose a Candidate-returning recall on `MemoryLayer`**

Modify `agent/memorylayer/layer.go`. Add a new method **next to** `Recall`:

```go
// RecallCandidates runs Hybrid + Reranker and returns the internal
// Candidate slice (not flattened to InjectedMemory). Used by Agentic
// to compose multi-round fusion before final emit.
func (l *MemoryLayer) RecallCandidates(ctx context.Context, query string, limit int) ([]Candidate, error) {
    if limit <= 0 { limit = l.cfg.RecallLimit }
    cands, err := l.hybrid.Recall(ctx, query, limit)
    if err != nil { return nil, err }
    if len(cands) == 0 { return nil, nil }
    ranked, _ := l.reranker.Rerank(ctx, query, cands, limit)
    return ranked, nil
}
```

Keep `Recall` unchanged. (`RecallCandidates` is internal-only — exported because tests live in the same package; we tag the doc-comment with "internal" so callers know not to depend on it.)

- [ ] **Step 2: Agentic config + struct**

`agent/memorylayer/agentic.go`:

```go
package memorylayer

import (
    "context"
    _ "embed"
    "encoding/json"
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/odysseythink/mlog"
    "github.com/odysseythink/pantheon/core"
)

//go:embed prompts/sufficiency_check.txt
var sufficiencyPromptTemplate string

//go:embed prompts/query_expansion.txt
var queryExpansionPromptTemplate string

type AgenticConfig struct {
    Enabled            bool
    MaxExtraRounds     int     // default 1
    ExpansionQueries   int     // default 2
    ShortcutThreshold  float64 // default 0.85
    PerTurnTokenCap    int     // default 2000
    PerSessionTokenCap int     // default 20000
    Timeout            time.Duration // default 8s
}

func (c *AgenticConfig) fill() {
    if c.MaxExtraRounds <= 0 { c.MaxExtraRounds = 1 }
    if c.ExpansionQueries <= 0 { c.ExpansionQueries = 2 }
    if c.ShortcutThreshold <= 0 { c.ShortcutThreshold = 0.85 }
    if c.PerTurnTokenCap <= 0 { c.PerTurnTokenCap = 2000 }
    if c.PerSessionTokenCap <= 0 { c.PerSessionTokenCap = 20000 }
    if c.Timeout <= 0 { c.Timeout = 8 * time.Second }
}

// Agentic adds a critic-driven extra retrieval round on top of MemoryLayer.
type Agentic struct {
    base      *MemoryLayer
    llm       core.LanguageModel
    cfg       AgenticConfig
    tokens    *TokenCap
}

func NewAgentic(base *MemoryLayer, llm core.LanguageModel, cfg AgenticConfig) *Agentic {
    cfg.fill()
    return &Agentic{
        base:   base,
        llm:    llm,
        cfg:    cfg,
        tokens: NewTokenCap(cfg.PerTurnTokenCap, cfg.PerSessionTokenCap),
    }
}
```

- [ ] **Step 3: Public Recall entry**

```go
// Recall runs Hybrid+Rerank once; if insufficient and budget allows,
// runs one extra round of LLM-driven sub-queries and re-ranks the union.
//
// Failures degrade silently to the first-round result. The returned
// slice has at most `limit` items.
func (a *Agentic) Recall(ctx context.Context, query string, limit int) ([]Candidate, error) {
    if a == nil || !a.cfg.Enabled || a.base == nil {
        return nil, fmt.Errorf("agentic: not configured")
    }
    a.tokens.ResetTurn()

    callCtx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
    defer cancel()

    m1, err := a.base.RecallCandidates(callCtx, query, limit)
    if err != nil { return nil, err }
    if len(m1) == 0 { return nil, nil }

    // Shortcut: best candidate already strong.
    if m1[0].Score >= a.cfg.ShortcutThreshold {
        return m1, nil
    }

    if a.llm == nil { return m1, nil }

    sufficient, missing := a.checkSufficiency(callCtx, query, m1)
    if sufficient {
        return m1, nil
    }

    subqueries := a.expandQueries(callCtx, query, missing, m1)
    if len(subqueries) == 0 {
        return m1, nil // degrade
    }

    extras := a.runSubqueries(callCtx, subqueries, limit)
    fused := rrfFuseCandidates(append([][]Candidate{m1}, extras...), 60)

    // Re-rank the union through the same reranker the base uses.
    final, _ := a.base.reranker.Rerank(callCtx, query, fused, limit)
    return final, nil
}
```

- [ ] **Step 4: Sufficiency + expansion helpers**

```go
func (a *Agentic) checkSufficiency(ctx context.Context, query string, cands []Candidate) (bool, string) {
    // Token estimate: prompt + response are both small (under ~400 tokens
    // total). Charge a flat cost and bail on cap.
    if !a.tokens.Allow(400) {
        mlog.Warning("agentic: sufficiency check skipped — cap reached")
        return true, "" // treat as sufficient to skip extra round
    }
    prompt := strings.ReplaceAll(sufficiencyPromptTemplate, "{{QUERY}}", query)
    prompt = strings.ReplaceAll(prompt, "{{COUNT}}", fmt.Sprintf("%d", len(cands)))
    prompt = strings.ReplaceAll(prompt, "{{MEMORIES}}", renderCandidatesForCritic(cands))

    resp, err := a.llm.Generate(ctx, &core.Request{
        SystemPrompt: "You are a memory-retrieval critic.",
        Messages: []core.Message{{
            Role:    core.MESSAGE_ROLE_USER,
            Content: []core.ContentParter{core.TextPart{Text: prompt}},
        }},
    })
    if err != nil { return true, "" } // degrade
    var out struct {
        Sufficient bool   `json:"sufficient"`
        Missing    string `json:"missing"`
    }
    if !parseJSONObject(extractText(resp), &out) {
        return true, ""
    }
    return out.Sufficient, out.Missing
}

func (a *Agentic) expandQueries(ctx context.Context, query, missing string, m1 []Candidate) []string {
    if !a.tokens.Allow(600) {
        mlog.Warning("agentic: expansion skipped — cap reached")
        return nil
    }
    prompt := strings.ReplaceAll(queryExpansionPromptTemplate, "{{QUERY}}", query)
    prompt = strings.ReplaceAll(prompt, "{{MISSING}}", missing)
    prompt = strings.ReplaceAll(prompt, "{{N}}", fmt.Sprintf("%d", a.cfg.ExpansionQueries))
    prompt = strings.ReplaceAll(prompt, "{{MEMORIES}}", renderCandidatesForCritic(m1))

    resp, err := a.llm.Generate(ctx, &core.Request{
        SystemPrompt: "You generate complementary retrieval queries.",
        Messages: []core.Message{{
            Role:    core.MESSAGE_ROLE_USER,
            Content: []core.ContentParter{core.TextPart{Text: prompt}},
        }},
    })
    if err != nil { return nil }
    text := extractText(resp)
    if i := strings.Index(text, "["); i >= 0 { text = text[i:] }
    if j := strings.LastIndex(text, "]"); j >= 0 { text = text[:j+1] }
    var qs []string
    if err := json.Unmarshal([]byte(text), &qs); err != nil { return nil }
    if len(qs) > a.cfg.ExpansionQueries { qs = qs[:a.cfg.ExpansionQueries] }
    return qs
}

func (a *Agentic) runSubqueries(ctx context.Context, qs []string, limit int) [][]Candidate {
    // Each sub-recall is a hybrid hit on the local store — no LLM call,
    // no cap charge.
    out := make([][]Candidate, len(qs))
    var wg sync.WaitGroup
    for i, q := range qs {
        wg.Add(1)
        go func(i int, q string) {
            defer wg.Done()
            cands, err := a.base.RecallCandidates(ctx, q, limit)
            if err != nil { return }
            out[i] = cands
        }(i, q)
    }
    wg.Wait()
    return out
}

func renderCandidatesForCritic(cs []Candidate) string {
    var b strings.Builder
    for i, c := range cs {
        if i >= 10 { break } // critic sees at most 10
        fmt.Fprintf(&b, "%d. %s\n", i+1, truncate(c.Content, 160))
    }
    return b.String()
}

func parseJSONObject(s string, v any) bool {
    s = strings.TrimSpace(s)
    if i := strings.Index(s, "{"); i >= 0 { s = s[i:] }
    if j := strings.LastIndex(s, "}"); j >= 0 { s = s[:j+1] }
    return json.Unmarshal([]byte(s), v) == nil
}
```

- [ ] **Step 5: Cross-query RRF fusion**

```go
// rrfFuseCandidates fuses N ranked candidate lists by RRF. Candidates
// keyed by ID; score is the sum of 1/(k+rank) contributions across lists
// that contain that candidate. Source is normalized to "agentic_fused".
func rrfFuseCandidates(lists [][]Candidate, k float64) []Candidate {
    byID := make(map[string]*Candidate)
    for _, list := range lists {
        for i, c := range list {
            rank := i + 1
            contrib := 1.0 / (k + float64(rank))
            if existing, ok := byID[c.ID]; ok {
                existing.Score += contrib
            } else {
                cc := c
                cc.Score = contrib
                cc.Source = "agentic_fused"
                byID[c.ID] = &cc
            }
        }
    }
    out := make([]Candidate, 0, len(byID))
    for _, c := range byID { out = append(out, *c) }
    // Sort by score desc; reranker will rewrite ordering anyway, but
    // a deterministic input keeps reranker fallback well-behaved.
    sortCandidatesByScoreDesc(out)
    return out
}

func sortCandidatesByScoreDesc(cs []Candidate) {
    // Use the same sort.SliceStable shape as in hybrid_recaller.go.
    sort.SliceStable(cs, func(i, j int) bool { return cs[i].Score > cs[j].Score })
}
```

Add `import "sort"` if not already present.

- [ ] **Step 6: Tests (table-driven)**

`agentic_test.go`:

```go
func TestAgentic_ShortcutSkipsCritic(t *testing.T) {
    // base returns 1 candidate with Score=0.9 → critic NOT called, return as-is
}

func TestAgentic_CriticSaysSufficient(t *testing.T) {
    // stubLLM returns {"sufficient":true} → no expansion, return m1
}

func TestAgentic_CriticSaysInsufficientRunsExpansion(t *testing.T) {
    // m1 has Score=0.6 (below shortcut)
    // stubLLM first call: {"sufficient":false,"missing":"X"}
    //          second call: ["sub1","sub2"]
    // base receives 2 sub-queries; final result is union after rerank
    // (mock reranker returns its input in order)
}

func TestAgentic_LLMFailFallsBackToM1(t *testing.T) {
    // stubLLM returns error → returns m1 unchanged
}

func TestAgentic_BadJSONFallsBack(t *testing.T) {
    // stubLLM returns "I'm not sure" → degrade to m1
}

func TestAgentic_TokenCapShortcutsExpansion(t *testing.T) {
    // cfg.PerTurnTokenCap=100 → first Allow(400) fails → critic treated as sufficient
}

func TestAgentic_TimeoutFallsBack(t *testing.T) {
    // stubLLM sleeps > Timeout → ctx.Err triggers, return m1
}

func TestRRFFuseCandidates_OverlappingItems(t *testing.T) {
    // ID "a" appears in both lists → score = 1/(k+1)+1/(k+2)
    // ID "b" only in first → score = 1/(k+2)
    // ID "c" only in second → score = 1/(k+1)
    // expected order: a, c, b (with k=60)
}
```

Use the existing `stubLLM` / `stubEmbedder` patterns from Phase 1 integration test as starting points.

- [ ] **Step 7: Commit**

```bash
git add agent/memorylayer/agentic.go agent/memorylayer/agentic_test.go \
        agent/memorylayer/prompts/sufficiency_check.txt agent/memorylayer/prompts/query_expansion.txt \
        agent/memorylayer/layer.go
git commit -m "feat(memorylayer): Agentic multi-round wrapper with sufficiency + expansion"
```

---

## Task 4: Lifecycle hooks — OnSessionStart

**Files:**
- **Create:** `agent/memorylayer/lifecycle.go`
- **Create:** `agent/memorylayer/lifecycle_test.go`

**Context:** Single explicit hook per design §10.1. Pulls `core` + non-expired `foresight` (≤ N days ahead) out of storage and hands them back as a `[]memprovider.InjectedMemory`. The engine integration (Task 6) decides where to put them.

`OnTurnComplete` is **not** a new method — Phase 1's `MemoryLayer.ObserveTurn` already covers that responsibility. The design's "thickened OnTurnComplete" content (working_summary, AppendMemoryEvent) stays as-is in MetaClaw and the existing Phase 1 boundary handler.

- [ ] **Step 1: Config**

In `agent/memorylayer/lifecycle.go`:

```go
package memorylayer

import (
    "context"
    "time"

    "github.com/odysseythink/hermind/storage"
    "github.com/odysseythink/hermind/tool/memory/memprovider"
)

type LifecycleConfig struct {
    InjectCoreOnStart       bool
    CoreMaxCount            int // hard cap on rows; default 10
    CoreMaxTokens           int // character-based proxy; default 600
    InjectForesightOnStart  bool
    ForesightMaxCount       int // default 3
    ForesightDaysAhead      int // default 7 — only inject foresights expiring within this window
}

func (c *LifecycleConfig) fill() {
    if c.CoreMaxCount <= 0       { c.CoreMaxCount = 10 }
    if c.CoreMaxTokens <= 0      { c.CoreMaxTokens = 600 }
    if c.ForesightMaxCount <= 0  { c.ForesightMaxCount = 3 }
    if c.ForesightDaysAhead <= 0 { c.ForesightDaysAhead = 7 }
}

// Lifecycle drives the OnSessionStart hook. It is intentionally narrow
// in P2 — the design's other hook (OnTurnComplete) is already handled
// by MemoryLayer.ObserveTurn / Flush.
type Lifecycle struct {
    store storage.Storage
    cfg   LifecycleConfig
}

func NewLifecycle(store storage.Storage, cfg LifecycleConfig) *Lifecycle {
    cfg.fill()
    return &Lifecycle{store: store, cfg: cfg}
}
```

- [ ] **Step 2: OnSessionStart**

```go
// OnSessionStart loads pinned context from storage and returns it as
// InjectedMemory entries. The caller (engine wiring) decides how to
// merge them into the prompt.
//
// Ordering: core memories come first (most recent first), then any
// foresights whose ExpiresAt is within ForesightDaysAhead. Total
// content length is capped by CoreMaxTokens for core; foresights
// are bounded only by ForesightMaxCount.
func (l *Lifecycle) OnSessionStart(ctx context.Context) ([]memprovider.InjectedMemory, error) {
    out := []memprovider.InjectedMemory{}

    if l.cfg.InjectCoreOnStart {
        core, err := l.store.SearchMemories(ctx, "", &storage.MemorySearchOptions{
            MemTypes: []string{"core"},
            Limit:    l.cfg.CoreMaxCount,
        })
        if err == nil {
            tokens := 0
            for _, m := range core {
                if tokens+len(m.Content) > l.cfg.CoreMaxTokens { break }
                out = append(out, memprovider.InjectedMemory{ID: m.ID, Content: m.Content})
                tokens += len(m.Content)
            }
        }
    }

    if l.cfg.InjectForesightOnStart {
        cutoff := time.Now().UTC().AddDate(0, 0, l.cfg.ForesightDaysAhead)
        fs, err := l.store.SearchMemories(ctx, "", &storage.MemorySearchOptions{
            MemTypes:       []string{"foresight"},
            Limit:          l.cfg.ForesightMaxCount * 4, // overfetch, then filter
            IncludeExpired: false,
        })
        if err == nil {
            picked := 0
            for _, m := range fs {
                if picked >= l.cfg.ForesightMaxCount { break }
                if !m.ExpiresAt.IsZero() && m.ExpiresAt.After(cutoff) {
                    continue // outside the lookahead window
                }
                out = append(out, memprovider.InjectedMemory{ID: m.ID, Content: m.Content})
                picked++
            }
        }
    }

    return out, nil
}
```

- [ ] **Step 3: Tests**

`lifecycle_test.go`:

```go
func TestLifecycle_LoadsCoreThenForesight(t *testing.T) {
    // seed: 2 core, 2 foresight (one 3d ahead, one 30d ahead)
    // ForesightDaysAhead=7 → only the 3d foresight returned
    // result order: core[0], core[1], foresight(3d)
}

func TestLifecycle_CoreMaxTokensTrims(t *testing.T) {
    // seed: 3 core entries each 300 chars; CoreMaxTokens=500
    // → only 1 entry returned (300 ≤ 500, next would push to 600 > 500)
}

func TestLifecycle_NoCoreNoForesight(t *testing.T) {
    // empty store → empty slice, no error
}

func TestLifecycle_ExpiredForesightExcluded(t *testing.T) {
    // foresight with ExpiresAt=now-1h → not returned
    // (depends on storage.IncludeExpired=false; covered by Phase 1 test, but pin here)
}

func TestLifecycle_DisabledTogglesAreNoOp(t *testing.T) {
    // InjectCoreOnStart=false → no core in output
    // InjectForesightOnStart=false → no foresight in output
}
```

Tests use the same in-memory SQLite pattern as the Phase 1 integration test.

- [ ] **Step 4: Commit**

```bash
git add agent/memorylayer/lifecycle.go agent/memorylayer/lifecycle_test.go
git commit -m "feat(memorylayer): Lifecycle.OnSessionStart — core + near-term foresight"
```

---

## Task 5: Engine — pinned memories field

**Files:**
- **Modify:** `agent/engine.go`
- **Modify:** `agent/conversation.go` (find the prompt-build site)
- **Modify:** `agent/prompt.go`
- **Modify:** `agent/prompt_test.go`

**Context:** Pinned memories live on the Engine for the duration of a session. They are rendered **before** `ActiveMemories` and are **not** subject to `SynergyBudget` (which caps active-skill + active-memory length). Cap is enforced upstream by `LifecycleConfig.CoreMaxTokens` / `ForesightMaxCount`.

- [ ] **Step 1: Engine field + accessors**

In `agent/engine.go`, after the existing `memoryLayer` field:

```go
// pinnedMemories is the result of MemoryLayer Lifecycle.OnSessionStart.
// Rendered in every prompt above ActiveMemories, bypassing SynergyBudget.
pinnedMemories []memprovider.InjectedMemory
```

Add setter / getter near the other `Set*` methods:

```go
// SetPinnedMemories replaces the session-pinned memories. Call once on
// session start (after MemoryLayer Lifecycle.OnSessionStart) and again
// only if the underlying store is mutated externally.
func (e *Engine) SetPinnedMemories(mems []memprovider.InjectedMemory) {
    e.pinnedMemories = mems
}

// PinnedMemories returns the current pinned set (defensive copy).
func (e *Engine) PinnedMemories() []memprovider.InjectedMemory {
    if len(e.pinnedMemories) == 0 { return nil }
    out := make([]memprovider.InjectedMemory, len(e.pinnedMemories))
    copy(out, e.pinnedMemories)
    return out
}
```

- [ ] **Step 2: Extend PromptOptions**

In `agent/prompt.go`:

```go
// PromptOptions parameterize prompt generation.
type PromptOptions struct {
    Model           string
    SkipContext     bool
    ActiveSkills    []ActiveSkill   // prepended under a stable header
    PinnedMemories  []string         // ALWAYS injected; bypasses synergy budget
    ActiveMemories  []string         // recalled memory snippets, subject to synergy budget
    ObsidianCtx     *ObsidianContext
}
```

Update `Build` to render pinned **before** active. Pinned uses its own header so caching keys split cleanly:

```go
func (pb *PromptBuilder) Build(opts *PromptOptions) string {
    var parts []string
    parts = append(parts, defaultIdentity)
    if strings.TrimSpace(pb.defaultSystemPrompt) != "" {
        parts = append(parts, pb.defaultSystemPrompt)
    }
    if opts != nil && len(opts.ActiveSkills) > 0 {
        parts = append(parts, renderActiveSkills(opts.ActiveSkills))
    }
    if opts != nil && len(opts.PinnedMemories) > 0 {
        parts = append(parts, renderPinnedMemories(opts.PinnedMemories))
    }
    if opts != nil && len(opts.ActiveMemories) > 0 {
        parts = append(parts, renderActiveMemories(opts.ActiveMemories))
    }
    if opts != nil && opts.ObsidianCtx != nil {
        parts = append(parts, renderObsidianContext(opts.ObsidianCtx))
    }
    return strings.Join(parts, "\n\n")
}

func renderPinnedMemories(mems []string) string {
    var b strings.Builder
    b.WriteString("# Pinned context\n\n")
    b.WriteString("Always-on facts you previously asked me to remember and short-term plans. ")
    b.WriteString("Treat these as ground truth unless contradicted in this turn.\n")
    for _, m := range mems {
        trimmed := strings.TrimSpace(m)
        if trimmed == "" { continue }
        b.WriteString("\n- ")
        b.WriteString(trimmed)
    }
    return strings.TrimRight(b.String(), "\n")
}
```

- [ ] **Step 3: Wire pinned through the conversation loop**

In `agent/conversation.go`, find the site that constructs `PromptOptions` (search for `ActiveMemories:`). After the existing assignment, add:

```go
pinned := e.PinnedMemories()
pinnedContents := make([]string, 0, len(pinned))
for _, m := range pinned {
    pinnedContents = append(pinnedContents, m.Content)
}
// in the PromptOptions literal:
//   PinnedMemories: pinnedContents,
```

Make sure `pinned` is NOT routed through `applySynergyBudget`. Apply synergy only to `ActiveMemories` (existing behavior).

- [ ] **Step 4: Prompt tests**

In `agent/prompt_test.go`:

```go
func TestPrompt_RendersPinnedBeforeActive(t *testing.T) {
    out := pb.Build(&PromptOptions{
        PinnedMemories: []string{"i am allergic to peanuts"},
        ActiveMemories: []string{"recalled: discussed jenkins last week"},
    })
    pinnedIdx := strings.Index(out, "Pinned context")
    activeIdx := strings.Index(out, "Relevant memories")
    if pinnedIdx == -1 || activeIdx == -1 || pinnedIdx > activeIdx {
        t.Fatalf("pinned must precede active; got pinned=%d active=%d", pinnedIdx, activeIdx)
    }
}

func TestPrompt_PinnedSectionOmittedWhenEmpty(t *testing.T) {
    out := pb.Build(&PromptOptions{})
    if strings.Contains(out, "Pinned context") {
        t.Fatal("Pinned section must be omitted when no pinned memories")
    }
}

func TestEngine_SetPinnedMemoriesReturnsCopy(t *testing.T) {
    e := NewEngine(...)
    in := []memprovider.InjectedMemory{{ID: "1", Content: "x"}}
    e.SetPinnedMemories(in)
    out := e.PinnedMemories()
    out[0].Content = "tampered"
    if e.PinnedMemories()[0].Content != "x" {
        t.Fatal("PinnedMemories must return a defensive copy")
    }
}
```

- [ ] **Step 5: Commit**

```bash
git add agent/engine.go agent/prompt.go agent/prompt_test.go agent/conversation.go
git commit -m "feat(agent): pinned memories — rendered above active, bypass synergy budget"
```

---

## Task 6: Layer composition update + wiring

**Files:**
- **Modify:** `agent/memorylayer/layer.go`
- **Modify:** `agent/memorylayer/layer_test.go`

**Context:** `MemoryLayer` becomes the canonical entry for Phase 2 features. Construction takes optional `Agentic` + `Lifecycle` collaborators; `Recall` is unchanged; new method `RecallWithAgentic` exposes the multi-round path; new method `LoadPinned` proxies the lifecycle helper.

- [ ] **Step 1: Extend the layer**

```go
type MemoryLayer struct {
    store     storage.Storage
    hybrid    *HybridRecaller
    reranker  Reranker
    boundary  *BoundaryDetector
    extractor *TaxonomyExtractor
    // P2 additions:
    agentic   *Agentic     // optional
    lifecycle *Lifecycle   // optional

    cfg Config
}

type Config struct {
    Hybrid     HybridConfig
    Reranker   RerankerConfig
    Boundary   BoundaryConfig
    Taxonomy   TaxonomyConfig
    Agentic    AgenticConfig    // P2
    Lifecycle  LifecycleConfig  // P2
    RecallLimit int
}
```

In `New`, optionally construct the two new components:

```go
ml := &MemoryLayer{ /* …existing… */ }
if cfg.Agentic.Enabled {
    ml.agentic = NewAgentic(ml, llm, cfg.Agentic)
}
if cfg.Lifecycle.InjectCoreOnStart || cfg.Lifecycle.InjectForesightOnStart {
    ml.lifecycle = NewLifecycle(store, cfg.Lifecycle)
}
return ml
```

- [ ] **Step 2: Expose Agentic + Lifecycle entry points**

```go
// Recall (existing) keeps its semantics: Hybrid + Rerank → InjectedMemory.

// RecallWithAgentic runs the multi-round critic-driven flow when the
// Agentic component is wired; otherwise it falls back to Recall.
func (l *MemoryLayer) RecallWithAgentic(ctx context.Context, query string, limit int) ([]memprovider.InjectedMemory, error) {
    if l == nil { return nil, nil }
    if l.agentic == nil {
        return l.Recall(ctx, query, limit)
    }
    cands, err := l.agentic.Recall(ctx, query, limit)
    if err != nil { return nil, err }
    return candidatesToInjected(cands, limit), nil
}

// LoadPinned runs OnSessionStart and returns the pinned set. Safe to
// call when lifecycle is not wired (returns nil).
func (l *MemoryLayer) LoadPinned(ctx context.Context) ([]memprovider.InjectedMemory, error) {
    if l == nil || l.lifecycle == nil { return nil, nil }
    return l.lifecycle.OnSessionStart(ctx)
}
```

- [ ] **Step 3: Tests**

Update `layer_test.go` (or add `layer_p2_test.go`):

```go
func TestMemoryLayer_RecallWithAgentic_FallsBackWhenAgenticNil(t *testing.T) {
    // cfg.Agentic.Enabled=false → RecallWithAgentic == Recall
}

func TestMemoryLayer_LoadPinned_NoLifecycle(t *testing.T) {
    // no lifecycle config → returns (nil, nil)
}

func TestMemoryLayer_LoadPinned_HappyPath(t *testing.T) {
    // seed 1 core + 1 foresight(3d ahead) in store
    // → LoadPinned returns 2 entries (core first)
}

func TestMemoryLayer_RecallWithAgentic_EndToEnd(t *testing.T) {
    // small seeded store; stubLLM acts as critic + expander + reranker
    // assert: extra round runs when critic says insufficient
}
```

- [ ] **Step 4: Commit**

```bash
git add agent/memorylayer/layer.go agent/memorylayer/layer_test.go
git commit -m "feat(memorylayer): compose Agentic + Lifecycle into MemoryLayer"
```

---

## Task 7: Configuration

**Files:**
- **Modify:** `config/config.go`
- **Modify:** `config/defaults.go`

**Context:** Add `agentic:` and `lifecycle:` subsections to `memory_layer:`. The names of the inner fields here are what live on disk — the in-package `*Config` structs already mirror them.

- [ ] **Step 1: Extend on-disk schema**

```go
type MemoryLayerConfig struct {
    Hybrid     HybridConfig    `yaml:"hybrid"`
    Reranker   RerankerConfig  `yaml:"reranker"`
    Boundary   BoundaryConfig  `yaml:"boundary"`
    Taxonomy   TaxonomyConfig  `yaml:"taxonomy"`
    Agentic    AgenticConfig   `yaml:"agentic"`     // NEW
    Lifecycle  LifecycleConfig `yaml:"lifecycle"`   // NEW
    RecallLimit int            `yaml:"recall_limit"`
}

type AgenticConfig struct {
    Enabled            bool `yaml:"enabled"`
    MaxExtraRounds     int  `yaml:"max_extra_rounds"`
    ExpansionQueries   int  `yaml:"expansion_queries"`
    ShortcutThreshold  float64 `yaml:"shortcut_threshold"`
    PerTurnTokenCap    int  `yaml:"per_turn_token_cap"`
    PerSessionTokenCap int  `yaml:"per_session_token_cap"`
    TimeoutMS          int  `yaml:"timeout_ms"`
}

type LifecycleConfig struct {
    InjectCoreOnStart       bool `yaml:"inject_core_on_start"`
    CoreMaxCount            int  `yaml:"core_max_count"`
    CoreMaxTokens           int  `yaml:"core_max_tokens"`
    InjectForesightOnStart  bool `yaml:"inject_foresight_on_start"`
    ForesightMaxCount       int  `yaml:"foresight_max_count"`
    ForesightDaysAhead      int  `yaml:"foresight_days_ahead"`
}
```

- [ ] **Step 2: Defaults**

In `config/defaults.go`:

```go
MemoryLayer.Agentic = AgenticConfig{
    Enabled: true, MaxExtraRounds: 1, ExpansionQueries: 2,
    ShortcutThreshold: 0.85,
    PerTurnTokenCap: 2000, PerSessionTokenCap: 20000,
    TimeoutMS: 8000,
}
MemoryLayer.Lifecycle = LifecycleConfig{
    InjectCoreOnStart: true,  CoreMaxCount: 10, CoreMaxTokens: 600,
    InjectForesightOnStart: true, ForesightMaxCount: 3, ForesightDaysAhead: 7,
}
```

- [ ] **Step 3: Translation helpers**

In `cli/engine_deps.go`, where the existing `translateMemoryLayerConfig` (or equivalent) lives, extend it to forward the two new subsections into `memorylayer.Config`. Both `AgenticConfig.Timeout` and the in-package `RerankerConfig.Timeout` are `time.Duration` — translate from milliseconds:

```go
out.Agentic = memorylayer.AgenticConfig{
    Enabled:            in.Agentic.Enabled,
    MaxExtraRounds:     in.Agentic.MaxExtraRounds,
    ExpansionQueries:   in.Agentic.ExpansionQueries,
    ShortcutThreshold:  in.Agentic.ShortcutThreshold,
    PerTurnTokenCap:    in.Agentic.PerTurnTokenCap,
    PerSessionTokenCap: in.Agentic.PerSessionTokenCap,
    Timeout:            time.Duration(in.Agentic.TimeoutMS) * time.Millisecond,
}
out.Lifecycle = memorylayer.LifecycleConfig{
    InjectCoreOnStart:      in.Lifecycle.InjectCoreOnStart,
    CoreMaxCount:           in.Lifecycle.CoreMaxCount,
    CoreMaxTokens:          in.Lifecycle.CoreMaxTokens,
    InjectForesightOnStart: in.Lifecycle.InjectForesightOnStart,
    ForesightMaxCount:      in.Lifecycle.ForesightMaxCount,
    ForesightDaysAhead:     in.Lifecycle.ForesightDaysAhead,
}
```

- [ ] **Step 4: Commit**

```bash
git add config/config.go config/defaults.go cli/engine_deps.go
git commit -m "feat(config): agentic + lifecycle subsections for memory_layer"
```

---

## Task 8: API server wiring

**Files:**
- **Modify:** `api/server.go`

**Context:** Two changes:
1. When the agentic component is wired, replace the `MemoryLayer.Recall` call in the active-memories provider with `RecallWithAgentic`.
2. On run start (where `runCtx` is set up — around line 411), call `MemoryLayer.LoadPinned` and feed the result to `eng.SetPinnedMemories`.

- [ ] **Step 1: Use RecallWithAgentic**

Find the existing block (around line 440):

```go
if deps.MemoryLayer != nil {
    out, _ := deps.MemoryLayer.Recall(ctx, userMsg, memK)
    return out
}
```

Change to:

```go
if deps.MemoryLayer != nil {
    out, _ := deps.MemoryLayer.RecallWithAgentic(ctx, userMsg, memK)
    return out
}
```

`RecallWithAgentic` internally falls back to `Recall` when agentic is disabled, so the conditional doesn't need to know.

- [ ] **Step 2: Load pinned on session start**

Right after `eng := agent.NewEngineWithToolsAndAux(...)` and before the recall provider is wired:

```go
if deps.MemoryLayer != nil {
    if pinned, err := deps.MemoryLayer.LoadPinned(runCtx); err == nil && len(pinned) > 0 {
        eng.SetPinnedMemories(pinned)
        mlog.Info("memorylayer: loaded pinned context",
            mlog.Int("count", len(pinned)))
    }
}
```

- [ ] **Step 3: Build + manual smoke**

```bash
go build ./...
go test ./agent/... ./agent/memorylayer/... ./api/...
go run ./cmd/hermind
# in REPL: insert a "core" memory via /memory or direct SQL,
# restart, observe that the prompt now contains a "# Pinned context" section
```

- [ ] **Step 4: Commit**

```bash
git add api/server.go
git commit -m "feat(api): wire RecallWithAgentic + load pinned context on session start"
```

---

## Task 9: Integration test

**Files:**
- **Create:** `agent/memorylayer/agentic_integration_test.go`

**Context:** End-to-end through an in-memory SQLite store. Mock LLM script:
1. First call (rerank for round 1): returns input order.
2. Second call (sufficiency): `{"sufficient":false,"missing":"Y"}`.
3. Third call (expansion): `["secondary","tertiary"]`.
4. Fourth call (rerank of fused union): returns input order.

Assert: 4 LLM calls happened; final returned slice contains memories from at least one of the sub-query lists.

- [ ] **Step 1: Scaffold**

```go
package memorylayer_test

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/odysseythink/hermind/agent/memorylayer"
    "github.com/odysseythink/hermind/storage"
    "github.com/odysseythink/hermind/storage/sqlite"
)

func TestIntegration_AgenticMultiRound(t *testing.T) {
    store, err := sqlite.Open(":memory:")
    if err != nil { t.Fatal(err) }
    defer store.Close()
    if err := store.Migrate(); err != nil { t.Fatal(err) }

    // Seed: 5 facts, only 2 of which match the original query well.
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

    llm := &scriptedLLM{ /* see helpers below */ }
    emb := &stubEmbedder{}

    layer := memorylayer.New(store, emb, nil, llm, memorylayer.Config{
        Hybrid:   memorylayer.HybridConfig{},
        Reranker: memorylayer.RerankerConfig{Enabled: true, BatchSize: 20, Timeout: time.Second},
        Agentic:  memorylayer.AgenticConfig{
            Enabled: true, MaxExtraRounds: 1, ExpansionQueries: 2,
            ShortcutThreshold: 0.85,
            PerTurnTokenCap: 2000, PerSessionTokenCap: 20000,
            Timeout: 3 * time.Second,
        },
        RecallLimit: 3,
    })

    out, err := layer.RecallWithAgentic(context.Background(), "what tools and conventions does the project use", 3)
    if err != nil { t.Fatal(err) }
    if len(out) == 0 { t.Fatal("expected hits") }

    if llm.calls < 3 { // rerank + sufficiency + expansion at minimum
        t.Errorf("expected >= 3 LLM calls, got %d", llm.calls)
    }
}
```

Define `scriptedLLM` and `stubEmbedder` in a `helpers_test.go` file (or reuse existing ones from Phase 1's `integration_test.go`).

- [ ] **Step 2: Lifecycle integration test**

In the same file:

```go
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

    layer := memorylayer.New(store, &stubEmbedder{}, nil, &stubLLM{},
        memorylayer.Config{
            Lifecycle: memorylayer.LifecycleConfig{
                InjectCoreOnStart: true, InjectForesightOnStart: true,
                CoreMaxCount: 10, CoreMaxTokens: 600,
                ForesightMaxCount: 3, ForesightDaysAhead: 7,
            },
            RecallLimit: 3,
        })

    pinned, err := layer.LoadPinned(context.Background())
    if err != nil { t.Fatal(err) }
    if len(pinned) != 2 {
        t.Fatalf("expected 2 pinned (core + 1 near foresight), got %d", len(pinned))
    }
    if !strings.Contains(pinned[0].Content, "peanuts") {
        t.Errorf("core must lead, got %q", pinned[0].Content)
    }
}
```

- [ ] **Step 3: Run + commit**

```bash
go test -run "TestIntegration_AgenticMultiRound|TestIntegration_PinnedRoundtrip" ./agent/memorylayer/...
git add agent/memorylayer/agentic_integration_test.go
git commit -m "test(memorylayer): agentic multi-round + pinned roundtrip integration"
```

---

## Task 10: Docs + status

**Files:**
- **Modify:** `.gpowers/designs/2026-05-20-memory-layer-design.md`
- **Modify:** `CHANGELOG.md`

- [ ] **Step 1: Update design status**

Top of the design doc:

```
> **Status**: Phase 1 shipped 2026-05-21. Phase 2 shipped 2026-05-XX.
>            Phase 3 plan pending.
```

- [ ] **Step 2: Changelog**

```
### Added
- Memory Layer Phase 2: Agentic multi-round retrieval (sufficiency
  check + 1 extra round + 2 sub-queries + token caps + timeout
  fallback). Lifecycle hook OnSessionStart preloads core memories and
  near-term foresights as pinned context, rendered above active
  memories and bypassing the synergy budget.
```

- [ ] **Step 3: Commit**

```bash
git add .gpowers/designs/2026-05-20-memory-layer-design.md CHANGELOG.md
git commit -m "docs(memorylayer): Phase 2 status + changelog"
```

---

## Risk register (Phase 2)

| Risk | Mitigation |
|---|---|
| Sufficiency check adds latency to every Recall | `ShortcutThreshold` skips the critic when round-1 is already strong; default 0.85 catches the bulk of trivial queries. Timeout (8s) bounds worst case. |
| LLM budget runaway across a long session | `PerSessionTokenCap` (default 20K) freezes Agentic to round-1-only after the cap; logs WARN once on first hit. |
| Pinned context bloat in prompt (cache key drift) | `CoreMaxTokens` is character-based hard cap; `ForesightMaxCount` is row-count cap. Both pre-sorted by recency in storage so prompt prefix stays stable across turns when contents don't change. |
| Pinned set goes stale mid-session (user adds a core memory) | P2 loads pinned once on session start. A future refresh hook can be added trivially (`OnMemoryWrite`) but is out of scope here. Document this in the design status. |
| Agentic + reranker double-spend | Round-2 reranker re-runs on the *fused* candidate set (not original m1). Worst case = 2 reranker calls per Recall; reranker has its own 1.5s timeout (Phase 1). |
| `RecallCandidates` is publicly exported but internal-only | Doc-comment marks it internal; not added to `MemoryLayer` interface (there isn't one). Acceptable for in-tree package. |

---

## Done criteria

Phase 2 is complete when:

1. `go test ./...` is green.
2. New unit tests in `agentic_test.go`, `lifecycle_test.go`, `tokencap_test.go`, and the integration tests pass.
3. Manual REPL: with a seeded core memory and a near-term foresight in the DB, the next session's first prompt contains a `# Pinned context` block listing both — visible via the existing `/dump prompt`-style debug path or by checking the LLM request log.
4. With a memory that's known to round-1-miss (use the seeded integration test as a template), running it through `RecallWithAgentic` actually triggers the critic + expansion path (at least 3 LLM calls observed in the trace).
5. `PerSessionTokenCap` enforcement verified — once cumulative cost exceeds the cap, the next Agentic round falls back to round-1-only (asserted in a unit test).
6. Design doc top is updated; changelog entry exists.

---

## Phase 3 — what to plan next

After Phase 2 lands, write a Phase 3 plan covering:

- **Living Profile** — independent `profiles` + `profile_sections` tables, `ProfileUpdater` (incremental add/update/delete via LLM with ID mapping), `OnSessionStart` extension to inject profile.
- **Foresight expiry behavior** — extend `tool/memory/memprovider/consolidate.go` to archive rows where `ExpiresAt < now`; optional `memory_event "foresight.due"` shortly before expiry.
- **Skill candidate emitter** — when the taxonomy extractor produces something that looks skill-like, emit `skills.OnSkillCandidate(content, sourceTurnID)` so `skills.Evolver` can pick it up. Removes the duplicated-extraction risk the v1 design flagged.

Phase 3 depends on `MemoryLayer.LoadPinned` / Lifecycle interfaces shipped here. Defer until P2 is stable.

---

*Drafted 2026-05-22 against memory layer design v2 + Phase 1 shipped artifacts.*
