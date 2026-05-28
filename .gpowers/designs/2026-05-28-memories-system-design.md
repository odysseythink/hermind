# Memories System — Design

**Date**: 2026-05-28
**Status**: Draft
**Authors**: ranwei + Claude
**Companion docs**:
- `2026-05-28-phase2-quick-wins-design.md`
- `2026-05-28-scheduled-jobs-design.md`
- `2026-05-28-webpush-design.md`
- Source: `.gpowers/reports/server-collector-vs-go-server-gap-analysis.md` §16 (Memories row)

## 1. Purpose

Add an independent long-term-memory system mirroring anything-llm's `memories` table: per-user, per-(workspace|global) scope, hard-capped, auto-injected into chat system prompt, and (eventually) auto-populated by a two-phase LLM extractor running periodically over recent chat history.

Does NOT alter the existing `rag-memory` agent tool (which is just a vector-search wrapper) — that remains as-is for in-conversation tool calls.

## 2. Scope

### In
- New `memories` table + `memory_processed` column on `workspace_chats`
- `MemoryService` with CRUD + scope promotion/demotion + atomic batch-apply
- 7 endpoints (list/create/update/delete/promote/demote/replace)
- `PromptInjector.PromptWithMemories(...)` hook called from `ChatService.buildRAGContext`
- Reranker-driven top-5 workspace selection (delegates to existing `reranker.Reranker` interface)
- Observer→Reflector extraction worker (PR2) running every 3 hours via existing `workers.Manager`
- `SystemSetting` toggles: `memories_enabled` and `memories_auto_extraction_enabled`

### Out (this design)
- Subagent/MCP memory-store integration (deferred — existing rag-memory tool covers that surface)
- Frontend UI (anything-llm frontend is React, separate change scope)
- Memory embedding-based retrieval (anything-llm doesn't do it either — text reranker is sufficient for 5 items)
- Memory expiry / decay (anything-llm doesn't have it; `lastUsedAt` is informational only)
- Sub-memory clustering (out of all phases)

## 3. Anything-LLM source comparison

### 3.1 Table

`memories`: `(id, user_id NULL, workspace_id NULL, scope='workspace'|'global', content TEXT, last_used_at, created_at, updated_at)`. Indexed on `(user_id, workspace_id)` and `(user_id, scope)`. Limits:
- `GLOBAL_LIMIT = 5`
- `WORKSPACE_LIMIT = 20`
- `MAX_INJECTED_WORKSPACE_LIMIT = 5` (the rerank topK at injection time)

### 3.2 CRUD model (`server/models/memory.js`, 451 LoC)

Key methods worth replicating:
- `forUserWorkspace(userId, wsId)` / `globalForUser(userId)` — listings
- `create({userId, workspaceId, scope, content})` — enforces limit before insert
- `update(id, {content})`
- `delete(id)`
- `promoteToGlobal(id)` / `demoteToWorkspace(id, wsId)` — scope transitions with limit checks
- `updateLastUsed(ids[])` — batch stamp
- `replaceWorkspaceMemories(userId, wsId, contents[])` — transactional delete-then-insert
- `applyExtractedMemories(userId, wsId, items[], globalSlots)` — transactional batch from extractor

### 3.3 Injection (`server/utils/memories/index.js`, 140 LoC)

`promptWithMemories({systemPrompt, userId, workspaceId, prompt, rawHistory})` → returns `systemPrompt + "\n\n## Things I Remember About You\n- ..."` if any memories exist and `memories_enabled=true`. Workspace memories are reranked when count > 5; reranker key is `prompt + last 3 history user messages`. Falls back to "first 5 by recency" if reranker fails. Stamps `lastUsedAt` on injected IDs.

### 3.4 Extractor (`server/jobs/extract-memories.js`, 192 LoC)

- Cron: every 3 hours (`MEMORY_EXTRACTION_INTERVAL`, default `3hr`)
- Walks `workspace_chats WHERE memory_processed IS NULL AND include = true`, groups by `(user_id, workspace_id)`
- Per group: require ≥ 5 chats; require group idle for ≥ 20 min (`MEMORY_IDLE_THRESHOLD_MS`)
- **Phase 1 (Observer)**: LLM call w/ tool-call schema `extract_candidate_facts(facts:[{content, confidence, reasoning}])`. Reads conversation text.
- **Phase 2 (Reflector)**: LLM call w/ tool-call schema `decide_memory_actions(memories:[{content, scope:WORKSPACE|GLOBAL, action:create|update, updateId?, reasoning}])`. Reads candidates + existing global/workspace memories + `globalSlots`.
- `Memory.applyExtractedMemories(...)` writes atomically.
- Marks chat IDs processed regardless of LLM success (avoids re-runs on broken pipelines).

### 3.5 Backend current state

- No `memories` table.
- `workspace_chats.memory_processed` column does not exist.
- The agent tool `rag-memory` is a vector-search wrapper; its `store` action is `deferred` — unrelated to this system.
- `chat_service.go:buildRAGContext` line 38–117 is where system prompt is assembled; this is the injection insertion point.

## 4. Design

### 4.1 Schema additions

```go
// backend/internal/models/memory.go
type Memory struct {
    ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID      *int   `gorm:"index:idx_user_ws;index:idx_user_scope" json:"userId"`
    WorkspaceID *int   `gorm:"index:idx_user_ws" json:"workspaceId"`
    Scope       string `gorm:"not null;default:workspace;index:idx_user_scope" json:"scope"` // workspace|global
    Content     string `gorm:"type:text;not null" json:"content"`
    LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
    CreatedAt   time.Time  `gorm:"autoCreateTime" json:"createdAt"`
    UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}
```

```go
// add to backend/internal/models/workspace_chat.go
MemoryProcessed *bool `gorm:"index" json:"memoryProcessed,omitempty"`
```

Constants live in the service:
```go
const (
    GlobalMemoryLimit          = 5
    WorkspaceMemoryLimit       = 20
    MaxInjectedWorkspaceLimit  = 5
)
```

### 4.2 PR1 · CRUD + injection (~600 LoC)

#### `services/memory_service.go`
- `ListWorkspace(ctx, userID *int, wsID int) ([]Memory, error)` — DESC createdAt
- `ListGlobal(ctx, userID *int) ([]Memory, error)`
- `Create(ctx, m *Memory) (*Memory, error)` — limit-check before insert, returns `ErrLimitReached`
- `Update(ctx, id int, content string) (*Memory, error)`
- `Delete(ctx, id int) error`
- `PromoteToGlobal(ctx, id int) (*Memory, error)`
- `DemoteToWorkspace(ctx, id int, wsID int) (*Memory, error)`
- `UpdateLastUsed(ctx, ids []int) error`
- `ReplaceWorkspace(ctx, userID *int, wsID int, contents []string) error` — transactional

#### `services/memory_injector.go`
```go
type MemoryInjector struct {
    memSvc   *MemoryService
    settings SettingsReader   // memoriesEnabled()
    rerank   reranker.Reranker
}

func (mi *MemoryInjector) PromptWithMemories(ctx context.Context, base string,
    userID *int, wsID int, currentMessage string, history []core.Message) string {
    if !mi.settings.MemoriesEnabled(ctx) { return base }

    globals, _ := mi.memSvc.ListGlobal(ctx, userID)
    workspace, _ := mi.memSvc.ListWorkspace(ctx, userID, wsID)
    if len(globals) == 0 && len(workspace) == 0 { return base }

    selected := workspace
    if len(workspace) > MaxInjectedWorkspaceLimit {
        // Build rerank query from current msg + last 3 user-role history texts
        rqry := buildRerankQuery(currentMessage, history)
        if r, ok := mi.rerank.(*reranker.PantheonReranker); ok {
            ranked, err := r.RerankTexts(ctx, rqry,
                texts(workspace), MaxInjectedWorkspaceLimit)
            if err == nil { selected = pickByIndex(workspace, ranked.Indices) }
            // err -> fall through to recency
        }
        if len(selected) == 0 || len(selected) > MaxInjectedWorkspaceLimit {
            selected = workspace[:min(len(workspace), MaxInjectedWorkspaceLimit)]
        }
    }

    // Stamp lastUsedAt (fire-and-forget)
    ids := append(idsOf(globals), idsOf(selected)...)
    go mi.memSvc.UpdateLastUsed(context.Background(), ids)

    return base + "\n\n## Things I Remember About You\n" +
        renderBullets(globals) + renderBullets(selected)
}
```

#### `services/chat_service.go` (modify `buildRAGContext`)
Inject between system prompt resolution (line 53–56) and the RAG context concat (line 112):
```go
systemPrompt = mi.PromptWithMemories(ctx, systemPrompt,
    userID, ws.ID, message, history)
```

#### Settings
- New `SystemSetting` key `memories_enabled` (default `"true"`)
- Read via `settingsSvc.GetBool("memories_enabled", true)` in the injector

#### Endpoints
| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/memory/workspace/:slug` | — | `{workspace[], global[]}` |
| POST | `/memory` | `{scope, content, workspaceId?}` | `{memory}` or `{message: "limit reached"}` |
| PATCH | `/memory/:id` | `{content}` | `{memory}` |
| DELETE | `/memory/:id` | — | `{success}` |
| POST | `/memory/:id/promote` | — | `{memory}` |
| POST | `/memory/:id/demote/:workspaceId` | — | `{memory}` |
| PUT | `/memory/workspace/:slug/replace` | `{memories[]}` | `{success}` |

Auth: `ValidatedRequest` + workspace-membership for workspace-scoped ops; user-scoped for global ops.

### 4.3 PR2 · Observer/Reflector extraction (~900 LoC)

#### Schema migration
- AutoMigrate updates `workspace_chats.memory_processed` column (existing rows = NULL = unprocessed)

#### `workers/extract_memories.go`
- Implements `workers.Job` interface
- `Schedule() = "0 */3 * * *"` (every 3 hours)
- `Enabled()` reads `memories_auto_extraction_enabled` setting (default `"false"` to match anything-llm)
- `Run(ctx)`:
  1. `SELECT * FROM workspace_chats WHERE memory_processed IS NULL AND include = true ORDER BY created_at ASC`
  2. Group by `(user_id, workspace_id)`
  3. For each group:
     - If `len(chats) < 5` → skip (don't mark processed)
     - If `now - last_chat.created_at < 20min` → skip (group still active)
     - Else: run extraction, then mark all `chat.id`s in this group processed (regardless of outcome)

#### `services/memory_extractor.go`
```go
type Extractor struct {
    llm    LLMClient        // resolved per workspace from existing provider factory
    memSvc *MemoryService
    log    *logger
}

func (e *Extractor) ProcessGroup(ctx context.Context, userID *int, wsID int, chats []WorkspaceChat) error {
    cands, err := e.runObserver(ctx, wsID, chats)
    if err != nil || len(cands) == 0 { return err }

    existingWS, _   := e.memSvc.ListWorkspace(ctx, userID, wsID)
    existingGlobal, _ := e.memSvc.ListGlobal(ctx, userID)
    globalSlots := GlobalMemoryLimit - len(existingGlobal)
    if globalSlots <= 0 && len(existingWS) >= WorkspaceMemoryLimit {
        return nil // capacity full
    }

    actions, err := e.runReflector(ctx, wsID, cands, existingWS, existingGlobal, globalSlots)
    if err != nil || len(actions) == 0 { return err }

    return e.memSvc.ApplyExtracted(ctx, userID, wsID, actions, globalSlots)
}
```

#### Tool-call schemas (prompts/memory_observer.txt / memory_reflector.txt)
Match anything-llm's tool-call definitions exactly so behavior is comparable:
- **Observer tool**: `extract_candidate_facts(facts: [{content, confidence: 0-1, reasoning}])`
- **Reflector tool**: `decide_memory_actions(memories: [{content, scope: "WORKSPACE"|"GLOBAL", action: "create"|"update", updateId?: number, reasoning}])`

System prompts also lifted from anything-llm's helpers (`memory-extraction-utils.js`) and translated to template files under `backend/prompts/`.

#### `services/memory_service.go` additions
```go
func (s *MemoryService) ApplyExtracted(ctx context.Context, userID *int, wsID int,
    actions []ExtractedAction, globalSlots int) (result struct{ WS, Global, Updated int }, err error) {

    creates, updates := splitActions(actions)
    newWS := filterScope(creates, "WORKSPACE", WorkspaceMemoryLimit)
    newGl := filterScope(creates, "GLOBAL", max(0, globalSlots))

    err = s.db.Transaction(func(tx *gorm.DB) error {
        for _, c := range newWS {
            if e := tx.Create(&Memory{UserID: userID, WorkspaceID: &wsID, Scope:"workspace", Content:c.Content}).Error; e != nil { return e }
        }
        for _, c := range newGl {
            if e := tx.Create(&Memory{UserID: userID, Scope:"global", Content:c.Content}).Error; e != nil { return e }
        }
        for _, u := range updates {
            if e := tx.Model(&Memory{}).Where("id = ?", u.UpdateID).Updates(map[string]any{"content": u.Content, "updated_at": time.Now()}).Error; e != nil { return e }
        }
        return nil
    })
    return
}
```

#### `models/workspace_chat.go` helper
```go
func (s *WorkspaceChatService) MarkMemoryProcessed(ctx context.Context, ids []int) error {
    return s.db.Model(&WorkspaceChat{}).Where("id IN ?", ids).
        Update("memory_processed", true).Error
}
```

## 5. Configuration

| Env | Default | Purpose |
|---|---|---|
| `MEMORY_EXTRACTION_INTERVAL` | `0 */3 * * *` | cron expression; matches anything-llm 3-hour interval |
| `MEMORY_IDLE_THRESHOLD_MS` | `1200000` (20min) | group skipped if last chat within this window |
| `MEMORY_MIN_CHATS` | `5` | group skipped if fewer chats |

| SystemSetting | Default | Purpose |
|---|---|---|
| `memories_enabled` | `"true"` | toggles prompt injection (admin-managed) |
| `memories_auto_extraction_enabled` | `"false"` | enables the extraction worker; matches anything-llm |

## 6. Boot-time wiring

- `cmd/server/main.go`:
  - Construct `MemoryService(db)`, `MemoryInjector(memSvc, settings, reranker)`
  - Pass injector to `ChatService` constructor
  - Register `workers.NewExtractMemoriesJob(memSvc, extractor, settings)` in the Manager (PR2)
- Settings change to `memories_auto_extraction_enabled`: re-call `manager.SyncJob("extract-memories")` to add/remove the job at runtime (similar pattern to anything-llm's `BackgroundService.syncMemoryJob`)

## 7. Risk register

| # | Risk | Mitigation |
|---|------|------------|
| R1 | Reranker provider down → injection silently degrades | Fall back to most-recent N already implemented; injector error is non-fatal (logged) |
| R2 | LLM extraction floods the LLM provider | Per-group 20-min idle gate; cron interval 3h; SystemSetting toggle defaults `false` |
| R3 | Reflector creates duplicate memories despite filter | Tool-call schema requires `update` action with `updateId` for revisions; extraction prompt explicitly tells the model to dedupe |
| R4 | `memory_processed` column added to a hot table (`workspace_chats`) — index cost | `gorm:"index"` on a nullable bool is small in SQLite. Postgres deployments may want a partial index — note in CHANGELOG |
| R5 | Manual UI overwrite races with extraction worker | `replaceWorkspaceMemories` and `applyExtractedMemories` both run in transactions; last writer wins. Not worth a CRDT |
| R6 | Privacy — auto-extracted memories may surface PII into all future conversations | Toggle defaults `false`; users must opt in. Future: per-workspace toggle |
| R7 | Native-reranker absence (gap §9) makes prompts cold-start to recency | Acceptable in PR1 — same behavior as anything-llm when its native reranker fails |

## 8. Done criteria

- `go test ./backend/...` green
- PR1 manual smoke:
  - `POST /memory {scope:"global", content:"User prefers Go"}` → 200; second call exceeds limit → `{message}`
  - `POST /memory/:id/promote` toggles scope
  - With 6+ workspace memories, chat-API request → assert system prompt contains `## Things I Remember About You` block with ≤ 5 bullets
  - With `memories_enabled=false` → block absent
- PR2 manual smoke:
  - Configure 1 workspace + 1 user + ≥ 5 chats, wait 20 min idle
  - Manually trigger `extract-memories` worker → assert ≥ 1 row in `memories` and matching chat rows have `memory_processed=true`
- Integration test that round-trips Observer→Reflector→Apply against a mock LLM

## 9. PR sequencing

- **PR1** (CRUD + injection): independent, shippable on its own — provides manual-management UX
- **PR2** (extractor + memory_processed column): depends on PR1 (writes through `MemoryService`)

## 10. Open questions

None at design time. All resolved during brainstorm:
- Auto-extraction: ship as separate PR2 (incremental delivery)
- Reranker: reuse existing backend reranker abstraction; fall back to recency on failure
- Settings: split `memories_enabled` (injection) from `memories_auto_extraction_enabled` (worker) — matches anything-llm
- Default for auto-extraction: `false` (opt-in)
