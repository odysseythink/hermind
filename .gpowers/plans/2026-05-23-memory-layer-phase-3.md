# Memory Layer Phase 3 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Source Design:** `.gpowers/designs/2026-05-20-memory-layer-design.md` (v2, 2026-05-21).
**Predecessor Plans:**
- `.gpowers/plans/2026-05-21-memory-layer-phase-1.md` (shipped — Hybrid+RRF, Reranker, Boundary, Provenance)
- `.gpowers/plans/2026-05-22-memory-layer-phase-2.md` (shipped — Agentic, Lifecycle.OnSessionStart, Pinned)

**Scope (P3 only):**
- **P3.1 Living Profile (§6)** — independent `profiles` + `profile_sections` tables; `ProfileUpdater` applies LLM-emitted incremental deltas (add / update / delete) with optimistic-locking versions; profile is rendered as a dedicated `## User Profile` block at `OnSessionStart`, *above* core/foresight.
- **P3.2 Foresight expiry archival (§8.2)** — extend `memprovider/consolidate.go` so expired `foresight` rows transition to `archived`; surface a `memory.foresight_archived` event.
- **P3.3 Skill candidate emitter (§3.3, §7.3)** — `MemoryLayer.handleBoundary` dispatches `SkillCandidate` events to a new hook on `skills.Evolver`; the Evolver remains the single writer of skill files.

**Out of scope (deferred / dropped):**
- Clustering / ClusterID rebuilds (P4; column already reserved).
- `working_summary` migration out of `MetaClaw` (no intelligence gain, keep as-is — see P2 plan).
- Foresight "due-soon" reminder events (design §8.2 v2.1; no UI consumer yet).
- Profile rollback CLI / API (design §14; surfaced via raw row inspection until a real UX use-case appears).
- `profile.redact_sections` (§14, v2.1).

**Goal:** Close the design's remaining intelligence gaps — give the engine a coherent, incrementally-edited user picture; stop foresights from accumulating as expired noise; let the boundary detector contribute to the skill pipeline without becoming a parallel writer.

**Architecture:** No new infrastructure layers. Three additive surfaces:
1. `agent/memorylayer/profile.go` + storage `profile_*` tables fed by a new boundary-driven updater.
2. A 3-line addition to `Consolidate` that runs after the dedup pass.
3. A `SkillCandidate` channel on the existing `skills.Evolver` (Hermind wrapper), invoked from `handleBoundary`.

**Tech Stack:** Go 1.22+, existing `pantheon/core.LanguageModel`, `storage.Storage`, `testify`. No new go.mod deps.

---

## File Structure

```
agent/memorylayer/                                (existing package)
├── profile.go                                    (new — ProfileUpdater + delta apply)
├── profile_test.go                               (new)
├── skill_emitter.go                              (new — SkillCandidate dispatcher)
├── skill_emitter_test.go                         (new)
├── layer.go                                      (modify — compose profile updater + skill emitter)
├── lifecycle.go                                  (modify — emit profile block into pinned output)
├── lifecycle_test.go                             (modify)
├── integration_test.go                           (modify — profile round-trip + foresight archive + skill emit)
└── prompts/
    └── profile_update.txt                        (new — LLM delta extraction prompt)

storage/types.go                                  (modify — Profile + ProfileSection structs)
storage/storage.go                                (modify — GetProfile / SaveProfileDelta / ListProfileSections)
storage/sqlite/profile.go                         (new — CRUD + sqlite tx)
storage/sqlite/profile_test.go                    (new)
storage/sqlite/migrate.go                         (modify — v11 schema bump)

tool/memory/memprovider/consolidate.go            (modify — expired foresight archival)
tool/memory/memprovider/consolidate_test.go       (modify)

skills/evolver.go                                 (modify — OnSkillCandidate hook + handler)
skills/evolver_test.go                            (modify)

config/config.go                                  (modify — ProfileConfigML + ForesightConfigML)
config/defaults.go                                (modify — P3 defaults)
config/defaults/agent.yaml                        (modify — example block, if present)

api/server.go                                     (modify — wire profile updater into MemoryLayer deps; wire Evolver OnSkillCandidate)

docs/memory-layer.md                              (modify — P3 status, profile section, foresight archival)
CHANGELOG.md                                      (modify — P3 entry)
```

---

## Dependencies

All new code depends on what Phase 1 + Phase 2 shipped plus:
- Schema v11: `profiles` + `profile_sections` tables (new migration block, follows the existing `case 10` pattern).
- A new entry on `storage.Storage`: `GetProfile`, `SaveProfileDelta` (transactional), `ListProfileSections`. No existing call-sites broken — pure addition.
- `skills.Evolver.OnSkillCandidate func(SkillCandidate)`: new field, default nil. No interface changes.

No new go.mod entries.

---

## Task 1: Storage schema — profiles + profile_sections

**Files:**
- **Modify:** `storage/sqlite/migrate.go`
- **Modify:** `storage/types.go`
- **Modify:** `storage/storage.go`
- **Create:** `storage/sqlite/profile.go`
- **Create:** `storage/sqlite/profile_test.go`

**Context:** Phase 1 reserved `ParentTurnID`/`ParentMemID`/`ExpiresAt`/`ClusterID` on `memories`. Phase 3 introduces a **separate** object (per design §6.1 — profile is **not** a MemType). One row per `user_id` in `profiles`; many child rows in `profile_sections` keyed by `(user_id, kind, key)`. The whole thing is updated atomically via `SaveProfileDelta` so a partial LLM diff never leaves the profile in a half-applied state.

- [ ] **Step 1: Bump `currentSchemaVersion` to 11; add `case 11` migration**

```go
// storage/sqlite/migrate.go
// v11 adds profiles + profile_sections tables for Living Profile (Phase 3).
const currentSchemaVersion = 11

// ... inside the switch in stepMigrate:
case 11:
    if _, err := tx.Exec(`
        CREATE TABLE IF NOT EXISTS profiles (
            user_id     TEXT PRIMARY KEY,
            version     INTEGER NOT NULL DEFAULT 0,
            updated_at  REAL    NOT NULL DEFAULT 0
        )`); err != nil {
        return fmt.Errorf("v11 create profiles: %w", err)
    }
    if _, err := tx.Exec(`
        CREATE TABLE IF NOT EXISTS profile_sections (
            id           INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id      TEXT    NOT NULL,
            kind         TEXT    NOT NULL,
            key          TEXT    NOT NULL,
            value        TEXT    NOT NULL DEFAULT '',
            evidence     TEXT    NOT NULL DEFAULT '',
            source_turns TEXT    NOT NULL DEFAULT '[]',
            confidence   REAL    NOT NULL DEFAULT 0,
            updated_at   REAL    NOT NULL DEFAULT 0,
            UNIQUE(user_id, kind, key)
        )`); err != nil {
        return fmt.Errorf("v11 create profile_sections: %w", err)
    }
    if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_profile_sections_user ON profile_sections(user_id)`); err != nil {
        return fmt.Errorf("v11 idx profile_sections user: %w", err)
    }
```

- [ ] **Step 2: Add `Profile` / `ProfileSection` / `ProfileDelta` types in `storage/types.go`**

```go
// storage/types.go (additive)

type Profile struct {
    UserID    string
    Version   int64
    UpdatedAt time.Time
    Sections  []ProfileSection
}

type ProfileSection struct {
    ID          int64
    UserID      string
    Kind        string  // "explicit" | "implicit"
    Key         string  // e.g. "diet.restrictions", "style.communication"
    Value       string
    Evidence    string
    SourceTurns []int64
    Confidence  float64
    UpdatedAt   time.Time
}

// ProfileDelta is the atomic write unit emitted by ProfileUpdater.
// Empty Adds/Updates/Deletes are all valid (e.g., a no-op LLM result).
type ProfileDelta struct {
    UserID  string
    Adds    []ProfileSection // ID ignored; assigned on insert
    Updates []ProfileSection // looked up by (UserID, Kind, Key)
    Deletes []ProfileSectionRef
}

type ProfileSectionRef struct {
    UserID string
    Kind   string
    Key    string
}
```

- [ ] **Step 3: Extend `storage.Storage` interface**

```go
// storage/storage.go (added inside interface block)

// GetProfile returns the user's profile or storage.ErrNotFound when no
// rows exist for userID. Sections are ordered by (kind, key) for stable
// rendering.
GetProfile(ctx context.Context, userID string) (*Profile, error)

// SaveProfileDelta applies an additive/updating/deleting batch atomically.
// Bumps Profile.Version by 1 on success. Returns the new version.
SaveProfileDelta(ctx context.Context, delta *ProfileDelta) (int64, error)
```

- [ ] **Step 4: Implement in `storage/sqlite/profile.go`**

```go
package sqlite

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"

    "github.com/odysseythink/hermind/storage"
)

func (s *Store) GetProfile(ctx context.Context, userID string) (*storage.Profile, error) {
    row := s.db.QueryRowContext(ctx,
        `SELECT version, updated_at FROM profiles WHERE user_id = ?`, userID)
    var (
        version int64
        updated float64
    )
    if err := row.Scan(&version, &updated); err != nil {
        if errorsIsNoRows(err) {
            return nil, storage.ErrNotFound
        }
        return nil, fmt.Errorf("sqlite: get profile %s: %w", userID, err)
    }
    rows, err := s.db.QueryContext(ctx,
        `SELECT id, kind, key, value, evidence, source_turns, confidence, updated_at
         FROM profile_sections WHERE user_id = ? ORDER BY kind, key`, userID)
    if err != nil {
        return nil, fmt.Errorf("sqlite: list profile sections: %w", err)
    }
    defer rows.Close()

    p := &storage.Profile{UserID: userID, Version: version, UpdatedAt: fromEpoch(updated)}
    for rows.Next() {
        var (
            sec     storage.ProfileSection
            srcJSON string
            sectionUpdated float64
        )
        if err := rows.Scan(&sec.ID, &sec.Kind, &sec.Key, &sec.Value,
            &sec.Evidence, &srcJSON, &sec.Confidence, &sectionUpdated); err != nil {
            return nil, fmt.Errorf("sqlite: scan profile section: %w", err)
        }
        sec.UserID = userID
        sec.UpdatedAt = fromEpoch(sectionUpdated)
        _ = json.Unmarshal([]byte(srcJSON), &sec.SourceTurns)
        p.Sections = append(p.Sections, sec)
    }
    return p, rows.Err()
}

func (s *Store) SaveProfileDelta(ctx context.Context, d *storage.ProfileDelta) (int64, error) {
    if d == nil || d.UserID == "" {
        return 0, fmt.Errorf("sqlite: SaveProfileDelta requires UserID")
    }
    var newVersion int64
    err := s.WithTx(ctx, func(tx storage.Tx) error {
        sqlTx := tx.(*sqliteTx).sql  // see step 5
        now := toEpoch(time.Now().UTC())

        // Upsert profile row + bump version.
        if _, err := sqlTx.ExecContext(ctx, `
            INSERT INTO profiles (user_id, version, updated_at)
            VALUES (?, 1, ?)
            ON CONFLICT(user_id) DO UPDATE SET
                version = version + 1,
                updated_at = excluded.updated_at`,
            d.UserID, now); err != nil {
            return fmt.Errorf("upsert profile: %w", err)
        }
        if err := sqlTx.QueryRowContext(ctx,
            `SELECT version FROM profiles WHERE user_id = ?`, d.UserID).
            Scan(&newVersion); err != nil {
            return fmt.Errorf("read version: %w", err)
        }

        for _, del := range d.Deletes {
            if _, err := sqlTx.ExecContext(ctx,
                `DELETE FROM profile_sections WHERE user_id = ? AND kind = ? AND key = ?`,
                del.UserID, del.Kind, del.Key); err != nil {
                return fmt.Errorf("delete %s/%s: %w", del.Kind, del.Key, err)
            }
        }
        for _, sec := range append(d.Adds, d.Updates...) {
            srcJSON, _ := json.Marshal(sec.SourceTurns)
            if _, err := sqlTx.ExecContext(ctx, `
                INSERT INTO profile_sections (user_id, kind, key, value, evidence, source_turns, confidence, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(user_id, kind, key) DO UPDATE SET
                    value = excluded.value,
                    evidence = excluded.evidence,
                    source_turns = excluded.source_turns,
                    confidence = excluded.confidence,
                    updated_at = excluded.updated_at`,
                d.UserID, sec.Kind, sec.Key, sec.Value, sec.Evidence,
                string(srcJSON), sec.Confidence, now); err != nil {
                return fmt.Errorf("upsert %s/%s: %w", sec.Kind, sec.Key, err)
            }
        }
        return nil
    })
    if err != nil {
        return 0, err
    }
    return newVersion, nil
}

func errorsIsNoRows(err error) bool { return err == sql.ErrNoRows }
```

- [ ] **Step 5: Make sqlite Tx expose its `*sql.Tx`** (only if not already exposed)

Check `storage/sqlite/tx.go`. If the concrete `sqliteTx` type doesn't carry the `*sql.Tx`, add an unexported accessor (`sql` field) so profile upsert can run inside the same transaction. Reuse the existing pattern from `AppendMessage` if it uses tx.

- [ ] **Step 6: Tests in `storage/sqlite/profile_test.go`**

Cover:
- Get on empty user → `storage.ErrNotFound`.
- Save adds → Get returns sections in `(kind, key)` order.
- Save updates → row in place; Confidence/Value/Evidence reflect update; Version bumps to 2.
- Save deletes → rows removed.
- Mixed delta (1 add + 1 update + 1 delete) in one call → version bumps by 1, all four end-states observable.
- Concurrent SaveProfileDelta on the same user serializes through tx (best-effort smoke test using two goroutines).

**Commit message:** `feat(storage): v11 profiles + profile_sections schema + Profile API`

---

## Task 2: Profile update prompt + parser

**Files:**
- **Create:** `agent/memorylayer/prompts/profile_update.txt`

**Context:** The LLM sees the current profile (with stable short IDs `s1`, `s2`, ...) and the new boundary's turns, then emits a JSON delta. Short IDs eliminate hallucinated keys/values — the LLM only references what's there.

- [ ] **Step 1: Author `profile_update.txt`**

```
You maintain a structured user profile from conversation evidence.

CURRENT PROFILE (each line is a section the user has already accrued):
{{CURRENT_SECTIONS}}

NEW EVIDENCE (recent conversation turns):
{{TURNS}}

Emit a JSON object with three optional arrays — adds, updates, deletes —
describing the minimal change to keep the profile accurate. Use the short
IDs (s1, s2, ...) shown above in updates and deletes.

ALLOWED KINDS: explicit | implicit
KEY GUIDANCE: dot-separated, lowercase, plural-when-natural (e.g.
  "diet.restrictions", "style.communication", "work.role").
EVIDENCE: a brief verbatim or paraphrased quote from the turns.
SOURCE_TURNS: array of integer turn IDs you drew from.
CONFIDENCE: float 0–1.

Do NOT propose updates that merely rephrase existing values.
Do NOT propose deletes unless the new evidence contradicts the prior.
If the evidence adds nothing new, emit {"adds":[],"updates":[],"deletes":[]}.

Output ONLY the JSON object:
{
  "adds":    [{"kind":"...","key":"...","value":"...","evidence":"...","source_turns":[123],"confidence":0.9}],
  "updates": [{"id":"s2","kind":"explicit","key":"diet.restrictions","value":"...","evidence":"...","source_turns":[124],"confidence":0.95}],
  "deletes": [{"id":"s5"}]
}
```

**Commit message:** `feat(memorylayer): profile update prompt template`

---

## Task 3: ProfileUpdater — boundary-driven incremental editor

**Files:**
- **Create:** `agent/memorylayer/profile.go`
- **Create:** `agent/memorylayer/profile_test.go`

**Context:** Runs in the same `handleBoundary` goroutine as the taxonomy extractor. Failures are warning-logged but never bubble (consistent with extractor behavior).

- [ ] **Step 1: ProfileConfig + ProfileUpdater struct**

```go
package memorylayer

import (
    "context"
    _ "embed"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/odysseythink/hermind/storage"
    "github.com/odysseythink/mlog"
    "github.com/odysseythink/pantheon/core"
)

//go:embed prompts/profile_update.txt
var profileUpdatePromptTemplate string

type ProfileConfig struct {
    Enabled       bool
    Timeout       time.Duration // default 6s
    MaxSections   int           // max sections rendered to the LLM (default 24)
    DefaultUserID string        // single-user installs may leave UserID empty on Turn; we fill it.
}

func (c *ProfileConfig) fill() {
    if c.Timeout <= 0 {
        c.Timeout = 6 * time.Second
    }
    if c.MaxSections <= 0 {
        c.MaxSections = 24
    }
    if c.DefaultUserID == "" {
        c.DefaultUserID = "default"
    }
}

type ProfileUpdater struct {
    store storage.Storage
    llm   core.LanguageModel
    cfg   ProfileConfig
}

func NewProfileUpdater(store storage.Storage, llm core.LanguageModel, cfg ProfileConfig) *ProfileUpdater {
    cfg.fill()
    return &ProfileUpdater{store: store, llm: llm, cfg: cfg}
}
```

- [ ] **Step 2: Apply method — LLM call → parse → SaveProfileDelta**

```go
// Apply runs one update cycle for a boundary. Best-effort — all errors
// log and return so the caller never blocks on this.
func (p *ProfileUpdater) Apply(ctx context.Context, b *Boundary) {
    if p == nil || !p.cfg.Enabled || p.llm == nil || b == nil || len(b.Turns) == 0 {
        return
    }
    userID := p.cfg.DefaultUserID

    cur, err := p.store.GetProfile(ctx, userID)
    if err != nil && err != storage.ErrNotFound {
        mlog.Warning("profile: GetProfile failed", mlog.String("err", err.Error()))
        return
    }

    callCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
    defer cancel()

    prompt := renderProfilePrompt(cur, b, p.cfg.MaxSections)
    resp, err := p.llm.Generate(callCtx, &core.Request{
        SystemPrompt: "You maintain a structured user profile from conversations.",
        Messages: []core.Message{{
            Role:    core.MESSAGE_ROLE_USER,
            Content: []core.ContentParter{core.TextPart{Text: prompt}},
        }},
    })
    if err != nil {
        mlog.Warning("profile: LLM call failed", mlog.String("err", err.Error()))
        return
    }
    delta := parseProfileDelta(extractText(resp), cur, userID)
    if delta == nil || (len(delta.Adds)+len(delta.Updates)+len(delta.Deletes)) == 0 {
        return
    }
    version, err := p.store.SaveProfileDelta(ctx, delta)
    if err != nil {
        mlog.Warning("profile: SaveProfileDelta failed", mlog.String("err", err.Error()))
        return
    }
    data, _ := json.Marshal(map[string]any{
        "user_id":  userID,
        "version":  version,
        "adds":     len(delta.Adds),
        "updates":  len(delta.Updates),
        "deletes":  len(delta.Deletes),
        "reason":   b.Reason,
    })
    _ = p.store.AppendMemoryEvent(ctx, time.Now().UTC(), "profile.updated", data)
}
```

- [ ] **Step 3: Prompt rendering with short-ID mapping**

```go
// renderProfilePrompt assigns s1..sN to each existing section, in the
// same order Get returned them. The corresponding map is rebuilt by
// parseProfileDelta when resolving update/delete IDs.
func renderProfilePrompt(cur *storage.Profile, b *Boundary, maxSections int) string {
    var sb strings.Builder
    if cur != nil && len(cur.Sections) > 0 {
        for i, sec := range cur.Sections {
            if i >= maxSections {
                break
            }
            fmt.Fprintf(&sb, "s%d | kind=%s | key=%s | value=%q | confidence=%.2f\n",
                i+1, sec.Kind, sec.Key, sec.Value, sec.Confidence)
        }
    } else {
        sb.WriteString("(empty)\n")
    }
    var turns strings.Builder
    for _, t := range b.Turns {
        fmt.Fprintf(&turns, "[turn %d]\nuser: %s\nassistant: %s\n\n",
            t.ID, t.UserMsg, t.Assistant)
    }
    p := strings.ReplaceAll(profileUpdatePromptTemplate, "{{CURRENT_SECTIONS}}", sb.String())
    return strings.ReplaceAll(p, "{{TURNS}}", turns.String())
}
```

- [ ] **Step 4: Parser with ID resolution**

```go
type rawDelta struct {
    Adds    []rawSection `json:"adds"`
    Updates []rawSection `json:"updates"`
    Deletes []struct {
        ID string `json:"id"`
    } `json:"deletes"`
}

type rawSection struct {
    ID          string  `json:"id"`           // present on updates
    Kind        string  `json:"kind"`
    Key         string  `json:"key"`
    Value       string  `json:"value"`
    Evidence    string  `json:"evidence"`
    SourceTurns []int64 `json:"source_turns"`
    Confidence  float64 `json:"confidence"`
}

func parseProfileDelta(text string, cur *storage.Profile, userID string) *storage.ProfileDelta {
    text = strings.TrimSpace(text)
    if i := strings.Index(text, "{"); i >= 0 {
        text = text[i:]
    }
    if j := strings.LastIndex(text, "}"); j >= 0 {
        text = text[:j+1]
    }
    var raw rawDelta
    if err := json.Unmarshal([]byte(text), &raw); err != nil {
        return nil
    }

    idToRef := make(map[string]storage.ProfileSectionRef, 0)
    if cur != nil {
        for i, sec := range cur.Sections {
            idToRef[fmt.Sprintf("s%d", i+1)] = storage.ProfileSectionRef{
                UserID: userID, Kind: sec.Kind, Key: sec.Key,
            }
        }
    }
    out := &storage.ProfileDelta{UserID: userID}
    for _, a := range raw.Adds {
        if isValidKind(a.Kind) && a.Key != "" && a.Value != "" {
            out.Adds = append(out.Adds, toSection(userID, a))
        }
    }
    for _, u := range raw.Updates {
        // If the LLM gave us a short id, use the existing key/kind. Else
        // fall back to the (kind, key) the LLM emitted.
        if ref, ok := idToRef[u.ID]; ok {
            sec := toSection(userID, u)
            sec.Kind, sec.Key = ref.Kind, ref.Key
            out.Updates = append(out.Updates, sec)
        } else if isValidKind(u.Kind) && u.Key != "" {
            out.Updates = append(out.Updates, toSection(userID, u))
        }
    }
    for _, d := range raw.Deletes {
        if ref, ok := idToRef[d.ID]; ok {
            out.Deletes = append(out.Deletes, ref)
        }
    }
    return out
}

func isValidKind(k string) bool { return k == "explicit" || k == "implicit" }

func toSection(userID string, r rawSection) storage.ProfileSection {
    return storage.ProfileSection{
        UserID:      userID,
        Kind:        r.Kind,
        Key:         r.Key,
        Value:       r.Value,
        Evidence:    r.Evidence,
        SourceTurns: r.SourceTurns,
        Confidence:  r.Confidence,
    }
}
```

- [ ] **Step 5: Tests in `profile_test.go`**

Cover:
- Empty profile + boundary with allergy claim → 1 add, kind=explicit.
- Existing section `s2` + contradicting turn → 1 update (same key) preserving Kind/Key from the old row even if LLM omits them.
- Existing section + retraction (`"deletes":[{"id":"s2"}]`) → 1 delete with the correct ref.
- LLM emits invalid JSON → Apply is a no-op (no SaveProfileDelta call); verify with a stub store.
- LLM emits empty `adds/updates/deletes` → Apply is a no-op (no `profile.updated` event written).

**Commit message:** `feat(memorylayer): ProfileUpdater — incremental Living Profile editor`

---

## Task 4: Wire ProfileUpdater into MemoryLayer.handleBoundary

**Files:**
- **Modify:** `agent/memorylayer/layer.go`
- **Modify:** `agent/memorylayer/layer_test.go`

**Context:** Profile updates must run on the same boundary signal that already drives `TaxonomyExtractor`. They are independent — both are best-effort and can fail independently. Don't block one on the other.

- [ ] **Step 1: Add field + constructor wiring**

```go
// agent/memorylayer/layer.go (additive)

type Config struct {
    Hybrid      HybridConfig
    Reranker    RerankerConfig
    Boundary    BoundaryConfig
    Taxonomy    TaxonomyConfig
    Agentic     AgenticConfig
    Lifecycle   LifecycleConfig
    Profile     ProfileConfig // NEW
    RecallLimit int
}

type MemoryLayer struct {
    // ... existing fields ...
    profile *ProfileUpdater // optional
}

func New(
    store storage.Storage,
    emb embedding.Embedder,
    base memprovider.Recaller,
    llm core.LanguageModel,
    cfg Config,
) *MemoryLayer {
    // ... existing wiring ...
    if cfg.Profile.Enabled {
        ml.profile = NewProfileUpdater(store, llm, cfg.Profile)
    }
    return ml
}
```

- [ ] **Step 2: Dispatch from `handleBoundary`**

```go
func (l *MemoryLayer) handleBoundary(b *Boundary) {
    ctx := context.Background()

    // Existing taxonomy extraction stays inline.
    mems, err := l.extractor.Extract(ctx, b)
    if err != nil {
        mlog.Warning("memorylayer: extractor failed", mlog.String("err", err.Error()))
    } else {
        for _, m := range mems {
            if err := l.store.SaveMemory(ctx, m); err != nil {
                mlog.Warning("memorylayer: SaveMemory failed", mlog.String("err", err.Error()))
                continue
            }
        }
    }

    // NEW — profile update runs in parallel; failures are isolated.
    if l.profile != nil {
        go l.profile.Apply(context.Background(), b)
    }

    // Existing event append stays.
    _ = l.store.AppendMemoryEvent(ctx, b.Turns[len(b.Turns)-1].Timestamp,
        "boundary.detected", []byte(`{"reason":"`+b.Reason+`","extracted":`+itoa(len(mems))+`}`))
}
```

- [ ] **Step 3: Test in `layer_test.go`**

Add a test that:
1. Seeds an LLM that returns the taxonomy JSON on call #1 and a valid profile delta JSON on call #2 (or vice-versa — both run in goroutines, accept any ordering).
2. Calls `ObserveTurn` 5 times to trigger a hard-turn boundary.
3. After a brief poll loop (≤2s), asserts:
   - `GetProfile("default")` returns 1 section with version=1.
   - A `profile.updated` event row exists.

**Commit message:** `feat(memorylayer): wire ProfileUpdater into boundary dispatch`

---

## Task 5: Inject profile into OnSessionStart

**Files:**
- **Modify:** `agent/memorylayer/lifecycle.go`
- **Modify:** `agent/memorylayer/lifecycle_test.go`

**Context:** Profile renders as one `## User Profile` block, *above* core / foresight pinned entries (engine handles ordering — see Task 6). The profile block lives inside `[]InjectedMemory` to keep the existing `Lifecycle.OnSessionStart` signature; downstream rendering treats it as a single-entry pinned block.

- [ ] **Step 1: Extend `LifecycleConfig`**

```go
// agent/memorylayer/lifecycle.go

type LifecycleConfig struct {
    InjectCoreOnStart      bool
    CoreMaxCount           int
    CoreMaxTokens          int
    InjectForesightOnStart bool
    ForesightMaxCount      int
    ForesightDaysAhead     int

    // P3 additions:
    InjectProfileOnStart bool
    ProfileMaxTokens     int    // default 800 (design §6.4)
    ProfileUserID        string // default "default"
}

func (c *LifecycleConfig) fill() {
    // ... existing defaults ...
    if c.ProfileMaxTokens <= 0 {
        c.ProfileMaxTokens = 800
    }
    if c.ProfileUserID == "" {
        c.ProfileUserID = "default"
    }
}
```

- [ ] **Step 2: Prepend profile block in `OnSessionStart`**

```go
func (l *Lifecycle) OnSessionStart(ctx context.Context) ([]memprovider.InjectedMemory, error) {
    out := []memprovider.InjectedMemory{}

    // NEW — profile first (it's the most stable signal).
    if l.cfg.InjectProfileOnStart {
        if block := l.renderProfile(ctx); block != "" {
            out = append(out, memprovider.InjectedMemory{
                ID:      "profile:" + l.cfg.ProfileUserID,
                Content: block,
            })
        }
    }

    // ... existing core + foresight loading unchanged ...
    return out, nil
}

func (l *Lifecycle) renderProfile(ctx context.Context) string {
    p, err := l.store.GetProfile(ctx, l.cfg.ProfileUserID)
    if err != nil || p == nil || len(p.Sections) == 0 {
        return ""
    }
    var sb strings.Builder
    sb.WriteString("## User Profile\n")
    tokens := len("## User Profile\n")
    for _, sec := range p.Sections {
        line := fmt.Sprintf("- [%s] %s: %s\n", sec.Kind, sec.Key, sec.Value)
        if tokens+len(line) > l.cfg.ProfileMaxTokens {
            break
        }
        sb.WriteString(line)
        tokens += len(line)
    }
    return strings.TrimRight(sb.String(), "\n")
}
```

(Existing `fmt`/`strings` imports already present in lifecycle.go.)

- [ ] **Step 3: Test in `lifecycle_test.go`**

- Seed two sections via `SaveProfileDelta` on an in-memory sqlite.
- Build a Lifecycle with `InjectProfileOnStart=true, InjectCoreOnStart=false, InjectForesightOnStart=false`.
- Assert `OnSessionStart` returns one entry whose `Content` starts with `## User Profile` and contains both `key: value` lines.
- Second test: cap `ProfileMaxTokens` smaller than total → only the first line(s) appear.
- Third test: empty profile → returns empty slice (no profile block).

**Commit message:** `feat(memorylayer): render profile block in OnSessionStart`

---

## Task 6: Foresight expiry archival in Consolidate

**Files:**
- **Modify:** `tool/memory/memprovider/consolidate.go`
- **Modify:** `tool/memory/memprovider/consolidate_test.go`

**Context:** Phase 1 introduced `ExpiresAt`, and `SearchMemories` already filters expired rows when `IncludeExpired=false`. But they accumulate in storage forever. Consolidate runs from cron / shutdown — that's the right place to archive. Don't archive in the retrieval path (would mask bugs in expiry computation).

- [ ] **Step 1: Add "foresight" to the consolidate type list**

```go
// tool/memory/memprovider/consolidate.go

// ...inside Consolidate:
types := []string{opts.MemType}
if opts.MemType == "" {
    types = []string{"episodic", "semantic", "preference", "foresight", ""}
}
```

- [ ] **Step 2: After dedup pass, archive expired foresights**

```go
// Append after the existing decay block (still inside the `for _, t := range types` loop):
if t == "foresight" {
    for _, m := range mems {
        if m.ExpiresAt.IsZero() {
            continue
        }
        if !m.ExpiresAt.Before(now) {
            continue
        }
        if m.Status != "" && m.Status != storage.MemoryStatusActive {
            continue
        }
        mm := *m
        mm.Status = storage.MemoryStatusArchived
        mm.UpdatedAt = now
        if err := store.SaveMemory(ctx, &mm); err == nil {
            report.Archived++
        }
    }
}
```

- [ ] **Step 3: Emit `memory.foresight_archived` rollup event**

Right before the existing `memory.consolidated` event write, append (only when archival happened):

```go
if foresightArchived > 0 {
    data, _ := json.Marshal(map[string]any{"archived": foresightArchived})
    _ = store.AppendMemoryEvent(ctx, time.Now().UTC(), "memory.foresight_archived", data)
}
```

(Track `foresightArchived` as a local counter inside the loop.)

- [ ] **Step 4: Tests in `consolidate_test.go`**

Add a focused test:
- Seed two foresight memories: one with `ExpiresAt` 1h in the past, one with `ExpiresAt` 1h in the future.
- Both with `Status=active`.
- Run `Consolidate(ctx, store, nil)`.
- Assert: expired one is now `archived`, future one stays `active`.
- Assert: `report.Archived == 1`.
- Assert: a `memory.foresight_archived` event row exists with `{"archived":1}`.

**Commit message:** `feat(memprovider): consolidate archives expired foresights`

---

## Task 7: Skill candidate emitter

**Files:**
- **Create:** `agent/memorylayer/skill_emitter.go`
- **Create:** `agent/memorylayer/skill_emitter_test.go`
- **Modify:** `agent/memorylayer/layer.go`
- **Modify:** `skills/evolver.go`
- **Modify:** `skills/evolver_test.go`

**Context:** Design §3.3 / §7.3 — the memory layer is *not* a skill writer. Per design (v2 §0): "Skill 由 `skills.Evolver` 唯一写入；记忆层只发候选事件." `Boundary` detection is a strong signal that a reusable skill *might* have surfaced — emit a candidate to the existing `skills.Evolver`, which decides whether to write a file based on the same heuristics it uses today.

- [ ] **Step 1: Define `SkillCandidate` type + emitter struct**

```go
// agent/memorylayer/skill_emitter.go
package memorylayer

import (
    "context"

    "github.com/odysseythink/mlog"
)

// SkillCandidate is a memory-layer signal that a boundary's turns may
// contain a reusable skill. The emitter does not decide; skills.Evolver
// remains the single writer.
type SkillCandidate struct {
    BoundaryReason string
    Turns          []TurnRef // last N turns; emitter trims to MaxTurns
}

type TurnRef struct {
    ID        int64
    UserMsg   string
    Assistant string
}

type SkillEmitterConfig struct {
    Enabled  bool
    MaxTurns int // default 8; cap on how much context the candidate carries
}

func (c *SkillEmitterConfig) fill() {
    if c.MaxTurns <= 0 {
        c.MaxTurns = 8
    }
}

type SkillEmitter struct {
    cfg SkillEmitterConfig
    // sink is set by SetSink; nil means "no listener" → emit is a no-op.
    sink func(SkillCandidate)
}

func NewSkillEmitter(cfg SkillEmitterConfig) *SkillEmitter {
    cfg.fill()
    return &SkillEmitter{cfg: cfg}
}

func (e *SkillEmitter) SetSink(fn func(SkillCandidate)) { e.sink = fn }

func (e *SkillEmitter) Emit(ctx context.Context, b *Boundary) {
    if e == nil || !e.cfg.Enabled || e.sink == nil || b == nil || len(b.Turns) == 0 {
        return
    }
    n := len(b.Turns)
    if n > e.cfg.MaxTurns {
        n = e.cfg.MaxTurns
    }
    turns := make([]TurnRef, 0, n)
    for _, t := range b.Turns[len(b.Turns)-n:] {
        turns = append(turns, TurnRef{ID: t.ID, UserMsg: t.UserMsg, Assistant: t.Assistant})
    }
    defer func() {
        if r := recover(); r != nil {
            mlog.Warning("skill_emitter: sink panicked", mlog.Any("panic", r))
        }
    }()
    e.sink(SkillCandidate{BoundaryReason: b.Reason, Turns: turns})
}
```

- [ ] **Step 2: Wire into `MemoryLayer`**

```go
// agent/memorylayer/layer.go (additive)

type Config struct {
    // ... existing fields ...
    SkillEmitter SkillEmitterConfig // NEW
}

type MemoryLayer struct {
    // ... existing fields ...
    skillEmitter *SkillEmitter
}

// In New(...):
if cfg.SkillEmitter.Enabled {
    ml.skillEmitter = NewSkillEmitter(cfg.SkillEmitter)
}

// New setter so api/server.go can wire the Evolver as sink:
func (l *MemoryLayer) SetSkillCandidateSink(fn func(SkillCandidate)) {
    if l == nil || l.skillEmitter == nil {
        return
    }
    l.skillEmitter.SetSink(fn)
}

// Inside handleBoundary, after the profile dispatch:
if l.skillEmitter != nil {
    go l.skillEmitter.Emit(context.Background(), b)
}
```

- [ ] **Step 3: Extend `skills.Evolver` with a candidate handler**

```go
// skills/evolver.go (additive)

// OnSkillCandidate, when set, is invoked for each memory-layer boundary
// that may contain a reusable skill. The Evolver decides whether to
// promote the candidate into a skill file by running the same legacy
// extraction prompt over the candidate's turns. Best-effort; errors log
// and do not bubble.
func (ev *Evolver) OnSkillCandidate(ctx context.Context, cand memorylayer.SkillCandidate) {
    if ev == nil || ev.Evolver == nil || len(cand.Turns) == 0 {
        return
    }
    msgs := make([]core.Message, 0, len(cand.Turns)*2)
    for _, t := range cand.Turns {
        if t.UserMsg != "" {
            msgs = append(msgs, core.Message{
                Role:    core.MESSAGE_ROLE_USER,
                Content: []core.ContentParter{core.TextPart{Text: t.UserMsg}},
            })
        }
        if t.Assistant != "" {
            msgs = append(msgs, core.Message{
                Role:    core.MESSAGE_ROLE_ASSISTANT,
                Content: []core.ContentParter{core.TextPart{Text: t.Assistant}},
            })
        }
    }
    // verdict=nil → legacy LLM-extraction path; identical to what we used
    // to run from the post-conversation hook. Evolver writes at most one
    // file or NONE.
    if err := ev.Evolver.Extract(ctx, msgs, nil); err != nil {
        mlog.Warning("evolver: OnSkillCandidate extract failed", mlog.String("err", err.Error()))
    }
}
```

> **Note on circular imports:** `skills/evolver.go` imports `agent/memorylayer` for the `SkillCandidate` type. Verify no reverse import — `agent/memorylayer/skill_emitter.go` must NOT import `skills`. The sink callback signature decouples them.

- [ ] **Step 4: Tests**

`agent/memorylayer/skill_emitter_test.go`:
- Sink not set → Emit is a no-op (panics silently swallowed).
- Sink set + boundary with 5 turns → callback receives 5 turns and reason.
- Boundary with 12 turns + `MaxTurns=8` → callback receives the last 8.
- Sink panics → no goroutine crash; warning logged.

`skills/evolver_test.go`:
- Build an Evolver with a stub LLM returning a valid skill body.
- Call `OnSkillCandidate(ctx, SkillCandidate{Turns:[…]})`.
- Assert a file was written under `skillDir` (use `t.TempDir()`).

**Commit message:** `feat(memorylayer): skill candidate emitter → Evolver legacy extract path`

---

## Task 8: Config schema for P3

**Files:**
- **Modify:** `config/config.go`
- **Modify:** `config/defaults.go`
- **Modify:** `config/defaults/agent.yaml` (only if such an example file is currently shipped — check first)

**Context:** Mirror the existing `*ConfigML` pattern. Defaults follow the design §11 numbers; everything off-by-default to keep upgrades safe.

- [ ] **Step 1: Add `ProfileConfigML` + extend `LifecycleConfigML` + `SkillEmitterConfigML`**

```go
// config/config.go (additive)

type MemoryLayerConfig struct {
    Hybrid       HybridConfigML       `yaml:"hybrid"`
    Reranker     RerankerConfigML     `yaml:"reranker"`
    Boundary     BoundaryConfigML     `yaml:"boundary"`
    Taxonomy     TaxonomyConfigML     `yaml:"taxonomy"`
    Agentic      AgenticConfigML      `yaml:"agentic"`
    Lifecycle    LifecycleConfigML    `yaml:"lifecycle"`
    Profile      ProfileConfigML      `yaml:"profile"`        // NEW
    SkillEmitter SkillEmitterConfigML `yaml:"skill_emitter"`  // NEW
    RecallLimit  int                  `yaml:"recall_limit"`
}

type LifecycleConfigML struct {
    InjectCoreOnStart      bool `yaml:"inject_core_on_start"`
    CoreMaxCount           int  `yaml:"core_max_count"`
    CoreMaxTokens          int  `yaml:"core_max_tokens"`
    InjectForesightOnStart bool `yaml:"inject_foresight_on_start"`
    ForesightMaxCount      int  `yaml:"foresight_max_count"`
    ForesightDaysAhead     int  `yaml:"foresight_days_ahead"`

    // P3 additions:
    InjectProfileOnStart bool   `yaml:"inject_profile_on_start"`
    ProfileMaxTokens     int    `yaml:"profile_max_tokens"`
    ProfileUserID        string `yaml:"profile_user_id"`
}

type ProfileConfigML struct {
    Enabled       bool   `yaml:"enabled"`
    TimeoutMS     int    `yaml:"timeout_ms"`
    MaxSections   int    `yaml:"max_sections"`
    DefaultUserID string `yaml:"default_user_id"`
}

type SkillEmitterConfigML struct {
    Enabled  bool `yaml:"enabled"`
    MaxTurns int  `yaml:"max_turns"`
}
```

- [ ] **Step 2: Defaults**

```go
// config/defaults.go — inside the MemoryLayer literal:

Profile: ProfileConfigML{
    Enabled:       true,
    TimeoutMS:     6000,
    MaxSections:   24,
    DefaultUserID: "default",
},
SkillEmitter: SkillEmitterConfigML{
    Enabled:  true,
    MaxTurns: 8,
},
Lifecycle: LifecycleConfigML{
    // ... existing P2 defaults ...
    InjectProfileOnStart: true,
    ProfileMaxTokens:     800,
    ProfileUserID:        "default",
},
```

- [ ] **Step 3: Map helper — `MemoryLayerConfig.ToMemoryLayer()` or wherever the conversion lives**

Add lines that translate `ProfileConfigML` → `memorylayer.ProfileConfig` and `SkillEmitterConfigML` → `memorylayer.SkillEmitterConfig`. Same pattern as the existing Hybrid/Reranker mappings — `time.Duration(TimeoutMS) * time.Millisecond`.

- [ ] **Step 4: Optional YAML example**

If `config/defaults/agent.yaml` is shipped, add a `profile:` and `skill_emitter:` block under `memory_layer:` with the default values, commented.

**Commit message:** `feat(config): profile + skill_emitter subsections for memory_layer`

---

## Task 9: Wire MemoryLayer ↔ Evolver in api/server.go

**Files:**
- **Modify:** `api/server.go`

**Context:** The Evolver and MemoryLayer are both already in `deps`. Connecting them is a single setter call after both are constructed.

- [ ] **Step 1: Right after `eng.SetMemoryLayer(deps.MemoryLayer)` (or wherever the layer is finalized)**

```go
if deps.MemoryLayer != nil && deps.SkillsEvolver != nil {
    deps.MemoryLayer.SetSkillCandidateSink(func(cand memorylayer.SkillCandidate) {
        // Run on the request's context; OnSkillCandidate is best-effort.
        deps.SkillsEvolver.OnSkillCandidate(runCtx, cand)
    })
}
```

> **Risk:** `runCtx` may be cancelled mid-extract. That's fine — `Extract` reads it and aborts. No leaked goroutines because the emitter is the goroutine boundary; the sink runs inline within the emitter's go-routine.

- [ ] **Step 2: Sanity smoke test**

Run `go build ./...` to verify no import cycles. The compile should succeed because `skills/evolver.go` imports `agent/memorylayer`, but `memorylayer` does not import `skills`.

**Commit message:** `feat(api): wire MemoryLayer skill emitter to Evolver`

---

## Task 10: Integration tests + docs

**Files:**
- **Modify:** `agent/memorylayer/integration_test.go`
- **Modify:** `docs/memory-layer.md` (if present)
- **Modify:** `CHANGELOG.md`

- [ ] **Step 1: `TestIntegration_ProfileRoundtrip`**

```go
func TestIntegration_ProfileRoundtrip(t *testing.T) {
    store, _ := sqlite.Open(":memory:")
    defer store.Close()
    _ = store.Migrate()

    llm := &profileStubLLM{
        // call 1: taxonomy returns []
        // call 2: profile delta returns 1 add (peanut allergy)
        // call 3+: reranker returns []
    }
    layer := memorylayer.New(store, &stubEmbedder{}, nil, llm, memorylayer.Config{
        Boundary: memorylayer.BoundaryConfig{HardTurnLimit: 3, EnableTopicShift: false},
        Taxonomy: memorylayer.TaxonomyConfig{Enabled: true, Timeout: time.Second},
        Profile:  memorylayer.ProfileConfig{Enabled: true, Timeout: time.Second, DefaultUserID: "default"},
        Lifecycle: memorylayer.LifecycleConfig{
            InjectProfileOnStart: true, ProfileMaxTokens: 400, ProfileUserID: "default",
        },
        RecallLimit: 3,
    })
    ctx := context.Background()
    for i := 1; i <= 3; i++ {
        layer.ObserveTurn(ctx, memorylayer.Turn{
            ID: int64(i), UserMsg: "I'm allergic to peanuts",
            Assistant: "Noted.", Timestamp: time.Now(),
        })
    }
    // Poll for async profile write (≤ 2s).
    var p *storage.Profile
    deadline := time.Now().Add(2 * time.Second)
    for time.Now().Before(deadline) {
        if got, err := store.GetProfile(ctx, "default"); err == nil && len(got.Sections) > 0 {
            p = got
            break
        }
        time.Sleep(50 * time.Millisecond)
    }
    require.NotNil(t, p)
    require.Len(t, p.Sections, 1)
    assert.Equal(t, "explicit", p.Sections[0].Kind)

    pinned, err := layer.LoadPinned(ctx)
    require.NoError(t, err)
    require.NotEmpty(t, pinned)
    assert.Contains(t, pinned[0].Content, "## User Profile")
    assert.Contains(t, pinned[0].Content, "peanuts")
}
```

- [ ] **Step 2: `TestIntegration_ForesightArchival`**

```go
func TestIntegration_ForesightArchival(t *testing.T) {
    store, _ := sqlite.Open(":memory:")
    defer store.Close()
    _ = store.Migrate()
    ctx := context.Background()
    now := time.Now().UTC()

    // Two foresights: one expired, one fresh.
    _ = store.SaveMemory(ctx, &storage.Memory{
        Content: "report due monday", MemType: "foresight", Status: "active",
        ExpiresAt: now.Add(-time.Hour), CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour),
    })
    _ = store.SaveMemory(ctx, &storage.Memory{
        Content: "demo next friday", MemType: "foresight", Status: "active",
        ExpiresAt: now.Add(72 * time.Hour), CreatedAt: now, UpdatedAt: now,
    })
    rep, err := memprovider.Consolidate(ctx, store, nil)
    require.NoError(t, err)
    assert.Equal(t, 1, rep.Archived)

    // Verify states.
    all, _ := store.ListMemoriesByType(ctx, "foresight", 100)
    var archived, active int
    for _, m := range all {
        if m.Status == storage.MemoryStatusArchived { archived++ }
        if m.Status == storage.MemoryStatusActive { active++ }
    }
    assert.Equal(t, 1, archived)
    assert.Equal(t, 1, active)
}
```

- [ ] **Step 3: `TestIntegration_SkillCandidateEmit`**

- Wire a fake sink (`func(SkillCandidate)`) into the layer via `SetSkillCandidateSink`.
- Trigger a boundary by `ObserveTurn` × HardTurnLimit.
- Assert sink was called once with non-empty `Turns`.
- Verify `Turns[0].UserMsg` matches the first seeded turn.

- [ ] **Step 4: docs + CHANGELOG**

```markdown
<!-- CHANGELOG.md -->

## [Unreleased] — Memory Layer Phase 3

### Added
- **Living Profile**: per-user incremental profile stored in `profiles` /
  `profile_sections` (schema v11). `ProfileUpdater` runs on each MemCell
  boundary; renders as `## User Profile` at OnSessionStart.
- **Foresight expiry archival**: `Consolidate` now archives `foresight`
  rows whose `ExpiresAt` is in the past; emits `memory.foresight_archived`
  events.
- **Skill candidate emitter**: boundary detector dispatches `SkillCandidate`
  events to `skills.Evolver.OnSkillCandidate`. `skills.Evolver` remains
  the single writer of skill files.

### Config (additions under `memory_layer:`)
- `profile.enabled / timeout_ms / max_sections / default_user_id`
- `skill_emitter.enabled / max_turns`
- `lifecycle.inject_profile_on_start / profile_max_tokens / profile_user_id`
```

**Commit message:** `test(memorylayer): P3 integration tests + docs/changelog`

---

## Done criteria

- [ ] Schema v11 applied on a fresh DB and on an existing v10 DB without errors.
- [ ] `go test ./...` green; specifically `agent/memorylayer/...`, `storage/sqlite/...`, `tool/memory/memprovider/...`, `skills/...`, `config/...`.
- [ ] `go build ./...` green (no circular imports between `skills/` and `agent/memorylayer/`).
- [ ] Integration tests in Task 10 pass without timing flakes (use polling, not fixed sleeps).
- [ ] CHANGELOG + docs updated.
- [ ] One end-to-end manual smoke: start the API, run a 4-turn conversation containing a stable preference statement, restart, verify `GET /api/memory/report?kinds=profile.updated` shows the event and the next session's system prompt prefix contains `## User Profile`.

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| LLM emits invalid delta JSON every turn → profile never updates | Parser is strict; events log on `Apply` warning; surface counter via `/api/memory/report?kinds=profile.updated` so silent failures show up in stats |
| Concurrent `SaveProfileDelta` collisions on the same user | All writes go through `WithTx`; SQLite serializes per-DB transactions; this is the design's intended model |
| Profile bloat (50+ sections) blows past prompt budget | `ProfileMaxTokens` caps rendered chars; LLM never sees more than `MaxSections` rows in the update prompt |
| Foresight archival deletes data a user still wants | Status only changes to `archived` (not deleted); `IncludeAll=true` in search reveals them; rollback path is a SaveMemory with Status="active" |
| Skill emitter creates near-duplicate skill files (one per boundary) | Evolver's legacy extract path returns NONE for boundaries with no reusable pattern; we accept occasional dupes — Evolver does NOT currently dedupe but this was true before P3 too |
| `runCtx` cancellation mid-Apply leaves partial profile | `SaveProfileDelta` is one tx; either all rows commit or none — partial state impossible |
| Circular import (`skills` ↔ `memorylayer`) | Sink type is a Go func value, defined inside `memorylayer`; `skills` imports `memorylayer`, never the reverse — verify with `go build` in Task 9 |

---

## What was deliberately NOT shipped in P3

These are referenced in the design but excluded from this phase:

- **Clustering / ClusterID writes** (design §9, P4). Column already present. Adds value only after P3 stabilizes — until then, the marginal recall gain is dominated by Hybrid+Rerank.
- **Profile rollback CLI** (design §14). The `version` column on `profiles` is the contract for a future implementation, but no current UX consumer.
- **Foresight "due-soon" reminder events** (design §8.2, v2.1). No UI consumer or scheduler exists to fire these on.
- **Profile redaction** (`profile.redact_sections`, design §14). Single-user installs don't need this yet; add when multi-user lands.
- **MetaClaw `working_summary` migration** (design §10.2). MetaClaw's implementation works; pulling it into the lifecycle hook is refactoring without intelligence gain (decision recorded in Phase 2 plan).

When these are needed, they become **Phase 4** — by which point the layer's interfaces should be stable enough to add them without a redesign.

---

*Drafted 2026-05-23. Predecessors shipped through commit `c0cebac`.*
