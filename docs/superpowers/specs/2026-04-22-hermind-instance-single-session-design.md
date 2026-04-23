# Hermind: Instance-Bound, Single-Conversation Redesign

**Date**: 2026-04-22
**Status**: Approved for implementation
**Ships in**: Two sequential PRs (`config-dir`, then `purge-sessions`)

## Motivation

Two usage findings from actually running hermind:

1. **Config location forces single-instance semantics.** Today's config search order
   (`$HERMIND_HOME` → `~/.hermind`) means every hermind process on a machine shares
   the same config/state.db/skills/trajectories. Running two independent instances
   from two working directories requires juggling `HERMIND_HOME` by hand.
2. **Multi-session is overhead, not value.** Each hermind instance is used as a
   single long-running workbench. The session list, new-chat button, and per-session
   settings drawer add UI complexity, storage complexity, and API surface for a
   capability no one uses. The per-session `system_prompt` field in particular is a
   recent addition that doesn't pay for itself.

The redesign collapses these: **one hermind process = one cwd = one conversation**.

## Design Decisions

All decisions made during brainstorming, recorded here as the source of truth.

| # | Decision |
|---|---|
| D1 | Config directory is `./.hermind/` (cwd-rooted). No home directory fallback. |
| D2 | `HERMIND_HOME` environment variable is retained as an explicit override. |
| D3 | Zero automatic migration from `~/.hermind/`. Stderr prints a one-time hint. |
| D4 | The `Session` concept is removed from every layer: storage, agent, API, UI. |
| D5 | `gateway/` (multi-platform bot bridge) and `acp` (protocol bridge) are deleted. |
| D6 | The single conversation persists forever in `./.hermind/state.db`. No reset UI — users can `rm` the DB. |
| D7 | Runtime model can be switched per-request from the UI; the change is in-memory only and never persisted. System prompt lives solely in `config.yaml` (`agent.default_system_prompt`). |
| D8 | Storage schema is flattened (deep surgery): `sessions` table dropped, `messages` becomes instance-scoped. Agent APIs drop all `Session*` parameters. |
| D9 | `cron` jobs are retained but run ephemerally: each run gets its own trajectory file and does not write to the main `messages` table. |
| D10 | Shipped as two PRs: (1) `config-dir` refactor, independent and low-risk; (2) `purge-sessions` — bundled Session removal + gateway/acp deletion + UI simplification. |
| D11 | On upgrade, a v1 `state.db` is renamed to `state.db.v1-backup` and a fresh v2 DB is created. No data merge. |
| D12 | Web server binds `127.0.0.1` on a random port in `[30000, 40000)`. The URL query `?t=<token>` authentication is removed. |
| D13 | Frontend header displays the instance absolute path prominently; `document.title` includes the instance path end-segment so browser tabs are distinguishable across instances. |

## PR 1: `config-dir`

**Goal**: Make every hermind process a self-contained instance rooted at cwd, with
independent config and state.

### Path resolution

Single entry point: `config.InstanceRoot()`.

```go
func InstanceRoot() (string, error) {
    if v := strings.TrimSpace(os.Getenv("HERMIND_HOME")); v != "" {
        return v, nil
    }
    cwd, err := os.Getwd()
    if err != nil {
        return "", err
    }
    return filepath.Join(cwd, ".hermind"), nil
}
```

All derived paths read from `InstanceRoot()`:

| Old | New |
|---|---|
| `~/.hermind/config.yaml` | `<root>/config.yaml` |
| `~/.hermind/state.db` (hardcoded in `config/loader.go:71`) | `<root>/state.db` |
| `$HERMIND_HOME/skills` \| `~/.hermind/skills` | `<root>/skills` |
| `$HERMIND_HOME/trajectories` \| `~/.hermind/trajectories` | `<root>/trajectories` |
| `$HERMIND_HOME/profiles/<name>/enabled_skills.yaml` | `<root>/enabled_skills.yaml` |
| `$HERMIND_HOME/plugins` | `<root>/plugins` |

### First-run behavior (`cli/app.go:NewApp`)

1. `root := config.InstanceRoot()`
2. If `<root>` does not exist: `mkdir -p`, write default `config.yaml`.
3. If `~/.hermind/` exists and `HERMIND_HOME` is not set and `<root>/.migration_notice_shown` does not exist: print to stderr
   ```
   hermind: legacy config at ~/.hermind/ is not auto-inherited by this instance.
   If you want to reuse it, copy it manually: cp -r ~/.hermind/. ./.hermind/
   ```
   Then touch `<root>/.migration_notice_shown` so the hint only fires once per instance.

### Profile concept removal

- Delete `cli/profile.go` in its entirety.
- Remove `HERMIND_PROFILE` env var references.
- `cli/plugins.go`, `cli/app_test.go` and friends read `enabled_skills.yaml` directly from `<root>/enabled_skills.yaml`.

### Random port + drop token auth

New helper `cli/listen.go`:

```go
const (
    portMin      = 30000
    portMax      = 40000
    portAttempts = 50
)

func listenRandomLocalhost() (net.Listener, error) {
    for i := 0; i < portAttempts; i++ {
        port := portMin + rand.IntN(portMax-portMin)
        ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
        if err == nil {
            return ln, nil
        }
        if !errors.Is(err, syscall.EADDRINUSE) {
            return nil, err
        }
    }
    return nil, fmt.Errorf("no free localhost port in [%d,%d) after %d attempts",
        portMin, portMax, portAttempts)
}
```

Usage in `cli/web.go:runWeb`:
- If `opts.Addr` is empty or explicitly the default sentinel: call `listenRandomLocalhost()`.
- If `--addr` is set to a non-default value: honor it (escape hatch for bookmarks / external integration).

Token removal:
- Delete `api/auth.go` (`GenerateToken`, `NewAuthMiddleware`, `checkToken`) and `api/auth_test.go`.
- `api.ServerOpts.Token` field is dropped.
- `cli/web.go` no longer generates, prints, or embeds a token.
- `runWeb` banner:
  ```
  hermind web listening on http://127.0.0.1:34721
  instance:  /Users/…/.hermind
  open:      http://127.0.0.1:34721/
  ```
- Frontend: `main.tsx` drops `?t=` parsing; `api/client.ts` drops `Authorization` header; WS URL has no token query.

### API `/api/meta` extension

```json
{
  "version": "0.3.0",
  "instance_root": "/Users/ranwei/workspace/myproject/.hermind"
}
```

### Frontend: instance path display

- `ConversationHeader.tsx` adds an instance label on the left:
  - `font-family: monospace; font-size: 13px;` (per DESIGN.md)
  - `dir="rtl"` for right-anchored truncation of long paths (keeps the trailing segment readable)
  - `title` attribute shows the full absolute path on hover
- `main.tsx` sets `document.title = \`hermind — ${lastSegment(instanceRoot)}\``.

### Non-goals (PR 1)

- No storage schema change.
- No Session removal. No UI simplification beyond the header label.
- No gateway/acp changes.

### Files touched (PR 1)

**Added**: `config/instance.go`, `cli/listen.go`.
**Modified**: `config/loader.go`, `cli/app.go`, `cli/bootstrap.go`, `cli/plugins.go`, `cli/models.go`, `cli/auth.go`, `cli/setup.go`, `cli/web.go`, `cli/skills.go`, `cli/doctor.go`, `agent/trajectory.go`, `agent/prompt.go` (doc strings), `api/handlers_meta.go`, `api/server.go`, `web/src/components/chat/ConversationHeader.tsx`, `web/src/main.tsx`, `web/src/api/*.ts`.
**Deleted**: `cli/profile.go`, `api/auth.go`, `api/auth_test.go`.
**Env**: `HERMIND_HOME` kept; `HERMIND_PROFILE` removed.

### PR 1 acceptance

1. `cd /tmp/a && hermind web` and `cd /tmp/b && hermind web` run concurrently on distinct random ports with independent `./.hermind/`.
2. A user with `~/.hermind/` present sees the stderr hint exactly once per new instance root.
3. `HERMIND_HOME=/some/path hermind web` uses `/some/path/.hermind` as the instance root. (Caller's choice of whether `$HERMIND_HOME` already includes `.hermind` is honored literally.)
4. Frontend header shows the absolute instance path; `document.title` reflects the trailing segment.
5. No `?t=` anywhere; `Authorization` header absent; server binds `127.0.0.1` only.
6. `HERMIND_PROFILE` and `profiles/<name>/` code paths are gone.
7. `/api/meta` returns `instance_root`.
8. Existing test suite passes; new `config.InstanceRoot` and `listenRandomLocalhost` unit tests pass.

## PR 2: `purge-sessions`

**Goal**: Collapse the `Session` abstraction from every layer. Delete `gateway/`
and `acp`. Simplify the UI to its minimal viable shape.

### Conceptual shift

Before: `Instance → Session[] → Messages[]` (three levels).
After: `Instance → Messages[]` (two levels; instance *is* the conversation).

### Module-level disposition

| Module | Action |
|---|---|
| `gateway/` (incl. `platforms/*`), `cli/gatewayctl/`, `cli/gateway.go`, `cli/gateway_build.go`, `cli/gateway_test.go` | **Delete** |
| `cli/acp.go`, `cli/acp_test.go`, any ACP bridge code in `agent/` | **Delete** |
| `api/session_registry.go`, `api/sessionrun/*`, `api/sessionrun_bridge.go`, `api/session_patch_limits.go`, `api/handlers_sessions*.go` | **Delete** |
| `api/handlers_messages.go` | **Rewrite** → `api/handlers_conversation.go` (no session ID) |
| `storage.Storage` interface + `storage/types.go` | **Rewrite** (see §Storage below) |
| `storage/sqlite/*` | **Add v2 migration** (see §Migration below) |
| `agent/engine.go`, `agent/conversation.go`, `agent/compression.go` | **Rewrite signatures** (drop `SessionID`, `UserID`) |
| `cli/cron.go`, `cron/` | **Change** to ephemeral engine runs |
| `web/src/components/chat/*`, `web/src/hooks/*`, `web/src/shell/*`, `web/src/state.ts`, `web/src/App.tsx`, `web/src/locales/*`, `web/src/api/*` | **Prune** (see §Frontend below) |
| `api/webroot/` | **Rebuild** at end of PR |

### Storage: SQLite schema v2

```sql
-- messages: instance-scoped, no session_id
CREATE TABLE messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    role        TEXT NOT NULL,
    content     TEXT NOT NULL,     -- JSON-encoded content blocks
    created_at  INTEGER NOT NULL,  -- unix ms
    token_count INTEGER DEFAULT 0
);
CREATE INDEX idx_messages_created ON messages(created_at);

-- conversation_state: singleton row (id=1)
CREATE TABLE conversation_state (
    id                    INTEGER PRIMARY KEY CHECK (id = 1),
    system_prompt_cache   TEXT,
    total_input_tokens    INTEGER DEFAULT 0,
    total_output_tokens   INTEGER DEFAULT 0,
    total_cost_usd        REAL    DEFAULT 0,
    updated_at            INTEGER NOT NULL
);

-- memories: unchanged from v1 (never was session-scoped)
```

### Storage: `Storage` interface

```go
type Storage interface {
    // Conversation
    AppendMessage(ctx context.Context, msg *StoredMessage) error
    GetHistory(ctx context.Context, limit, offset int) ([]*StoredMessage, error)
    SearchMessages(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)

    // System prompt cache (for prefix caching)
    UpdateSystemPromptCache(ctx context.Context, prompt string) error

    // Usage
    UpdateUsage(ctx context.Context, usage *UsageUpdate) error

    // Memory — unchanged
    SaveMemory(ctx context.Context, memory *Memory) error
    GetMemory(ctx context.Context, id string) (*Memory, error)
    SearchMemories(ctx context.Context, query string, opts *MemorySearchOptions) ([]*Memory, error)
    DeleteMemory(ctx context.Context, id string) error

    // Lifecycle
    WithTx(ctx context.Context, fn func(tx Tx) error) error
    Close() error
    Migrate() error
}
```

`SearchOptions` loses its `SessionID` field. `Tx` interface is similarly flattened.

### Migration: v1 → v2

**When this migration fires**: Only when a v1-schema `state.db` exists at
`<instance-root>/state.db`. Because PR 1 has already moved the instance root
from `~/.hermind` to `./.hermind`, the typical upgrader's v1 data sits in
`~/.hermind/state.db` and is *not* touched by this migration — it stays
orphaned there, consistent with D3 (zero auto-migration from home). The
migration's realistic customer is: a user who ran on PR 1 for a while,
accumulated v1-schema data in `./.hermind/state.db` (because PR 1 does not
remove Session code), and then upgrades to PR 2 in the same cwd.

Logic in `storage/sqlite/migrate.go`:

1. Open DB, read `PRAGMA user_version`.
2. If `user_version < 2` and the `sessions` table exists:
   - Close the DB.
   - Rename the file to `state.db.v1-backup`. If `state.db.v1-backup` already exists, append a unix-ms suffix (`state.db.v1-backup.1713801234567`) so nothing is ever overwritten.
   - Open a fresh `state.db`, run v2 schema DDL, set `PRAGMA user_version = 2`.
   - Print to stderr:
     ```
     hermind: legacy state.db (v1 multi-session) backed up to state.db.v1-backup.
     The new schema is single-conversation; your message history has been preserved
     in the backup but is not migrated.
     ```
3. Otherwise, proceed normally.

### Agent: `Engine` and `RunOptions`

```go
type RunOptions struct {
    UserMessage string
    Model       string  // ephemeral per-request override; empty = cfg.Model
    Ephemeral   bool    // true: do not read/write storage.messages
    History     []message.Message // used only when Ephemeral=true (cron injects its own context)
}

type ConversationResult struct {
    Response   message.Message
    Messages   []message.Message // full history after the run (or just this run if Ephemeral)
    Usage      message.Usage
    Iterations int
}
```

Semantics:

- **Default (`Ephemeral=false`)**: engine reads `storage.GetHistory()` for context, appends the user message, runs the provider loop, and appends the assistant message + tool calls. Compression operates directly on the storage-backed history.
- **Ephemeral (`Ephemeral=true`)**: engine uses `opts.History` as the only context. It emits events to the configured stream sink but never touches the `messages` table. Compression is disabled for the ephemeral run.

`RunOptions.SessionID` and `RunOptions.UserID` are removed. `ConversationResult.SessionID` is removed.

### HTTP API surface

**Deleted routes**:
- `GET/POST /api/sessions`
- `GET/PATCH /api/sessions/{id}`
- `GET /api/sessions/{id}/messages`
- `POST /api/sessions/{id}/run`
- `GET /api/sessions/{id}/sse`
- `WS /api/sessions/{id}/ws`
- All `session_*` SSE event types (`session_created`, `session_updated`, `session_deleted`)
- `GET /api/platforms` (contents become empty once gateway is gone)

**Added/renamed routes**:
- `GET /api/conversation?limit=&offset=` → returns message history.
- `POST /api/conversation/messages` with body `{ "user_message": "...", "model": "opus" }` → returns 202, events stream via SSE.
- `POST /api/conversation/cancel` → cancels the current in-flight run (preserves existing stop semantics).
- `GET /api/sse` → single SSE stream of engine events (`message_chunk`, `tool_call`, `tool_result`, `usage_update`, `done`, `error`).
- `GET /api/meta` → adds `current_model` to the existing response (used by UI to pre-select the active model in the dropdown). System prompt is not exposed here — the UI does not display or edit it; the config editor page already covers it.

**Kept unchanged**:
- `GET /api/config/...` (config editor)
- `GET /api/providers/{name}/models` (for model dropdown)
- `GET /api/skills`, `GET /api/tools`

### Cron: ephemeral runs

`cli/cron.go` changes:

- Each scheduled job calls:
  ```go
  engine.RunConversation(ctx, &agent.RunOptions{
      UserMessage: job.Prompt,
      Model:       job.Model,   // may be ""
      Ephemeral:   true,
      History:     nil,
  })
  ```
- Event output is written to `./.hermind/trajectories/cron-<job>-<unix>.jsonl`.
- Errors are logged but do not affect the main conversation.
- Each job has its own context with `cfg.Agent.GatewayTimeout` as the timeout (reused as-is; `GatewayTimeout` is a misleading name post-gateway-removal but renaming it is out of scope — a follow-up config-key rename can happen independently).

### Frontend: simplification

**Deleted components and helpers**:

- `web/src/components/chat/ChatSidebar.{tsx,module.css,test.tsx}`
- `web/src/components/chat/SessionList.{tsx,module.css,test.tsx}`
- `web/src/components/chat/SessionItem.{tsx,module.css,test.tsx}`
- `web/src/components/chat/NewChatButton.{tsx,module.css}`
- `web/src/components/chat/SessionSettingsDrawer.{tsx,module.css,test.tsx}`
- `web/src/components/chat/SettingsButton.{tsx,module.css}`
- `web/src/hooks/useSessionList.{ts,test.ts}`
- `web/src/shell/keyedInstances.{ts,test.ts}`
- `web/src/shell/listInstances.{ts,test.ts}`
- `web/src/shell/summaries.{tsx,test.tsx}` (session summary rendering)

**Simplified components**:

- `ChatWorkspace.tsx`: single-column layout, no sidebar, no session switching.
- `ConversationHeader.tsx`: left = instance path label (introduced in PR 1), center = empty or title, right = model dropdown + stop button.
- `App.tsx`: no session routing; root route renders `ChatWorkspace`.
- `useChatStream.ts`: takes no session ID; SSE URL is `/api/sse`; POST target is `/api/conversation/messages`.
- `state.ts`: single conversation state (`messages`, `runtimeModel`, `streaming`, `error`). No per-session map.

**`shell/` directory**: retain helpers serving the **config editor** (e.g., `groups.ts`, `sections.ts`, `firstSubkey.ts` if used by `ConfigSection`/`Editor`). Delete anything serving the multi-session shell. The plan should include a concrete audit step; this spec's rule is: *"session-related → delete; config-editor-related → keep"*.

**i18n**: strip all session-related keys from `web/src/locales/*`. Add an "Instance" key for the new header label.

**New UI skeleton**:

```
┌────────────────────────────────────────────────────────┐
│ ConversationHeader                                     │
│ ┌──────────────────────────┐  ┌──────────┐ ┌────────┐ │
│ │ /Users/…/myproject/.hermind│  │ model ▾  │ │  stop  │ │
│ └──────────────────────────┘  └──────────┘ └────────┘ │
├────────────────────────────────────────────────────────┤
│                                                        │
│                   MessageList                          │
│                                                        │
├────────────────────────────────────────────────────────┤
│ ComposerBar                                            │
└────────────────────────────────────────────────────────┘
```

- No left sidebar.
- No "new chat" button.
- No gear / settings drawer.
- Model dropdown sets `state.runtimeModel` in-memory and attaches it to the next POST. Not persisted. Reloading the page falls back to `/api/meta.current_model` (which is sourced from `config.yaml`).
- Stop button POSTs to `/api/conversation/cancel`.

### PR 2 acceptance

1. `hermind gateway` and `hermind acp` cobra subcommands do not exist.
2. `./.hermind/state.db` contains only `messages`, `conversation_state`, `memories` — no `sessions` table.
3. All `/api/sessions*` routes return 404. `GET /api/conversation` works and returns history.
4. Frontend has no sidebar, no new-chat button, no settings drawer. Header is `[instance path] [model dropdown] [stop]`.
5. Switching the model in the dropdown only affects the next request; after reload the dropdown reflects `config.yaml:model` again.
6. Cron-triggered prompts do not appear in the main conversation history; `./.hermind/trajectories/cron-*.jsonl` files are produced instead.
7. An older v1 `state.db` on disk gets renamed to `state.db.v1-backup` (with ms-suffix on collision) on first boot, and the new v2 DB starts empty.
8. CHANGELOG records: gateway/acp removal, session API route removal, state.db schema v2, per-session settings removal.
9. `go test ./...` and `pnpm test` are green. `api/webroot/` is rebuilt to match `web/dist/`.

## Testing Strategy

### PR 1

- `config.InstanceRoot()` unit tests: cwd-only, `HERMIND_HOME` override, both present (override wins).
- `cli/app.go:NewApp` integration tests using `t.Chdir(tmp)`: fresh directory creates `.hermind`; the `~/.hermind` notice fires exactly once per instance.
- `cli/listen.go:listenRandomLocalhost` unit tests: occupy a port, confirm retry succeeds; simulate all 50 attempts failing (inject a blocker).
- `/api/meta` handler returns `instance_root`.
- Frontend tests: `ConversationHeader` renders path label, truncates via `dir="rtl"`, sets `title`; `main.tsx` boots without `?t=`.
- Manual: two terminals, two cwds, two concurrent instances, messages do not cross.

### PR 2

- `storage/sqlite` migration test with a v1 fixture DB: assert `state.db.v1-backup` is created and new `state.db` is empty v2.
- `storage.Storage` new-interface CRUD tests (AppendMessage / GetHistory / SearchMessages / Usage / Memory).
- `agent.Engine.RunConversation` tests:
  - `Ephemeral=false`: user and assistant messages appear in `GetHistory()` afterward.
  - `Ephemeral=true`: `GetHistory()` is unchanged; events are still emitted.
- `agent/compression_test.go` passes without `SessionID` param.
- `cli/cron_test.go`: after cron fires, `GetHistory()` length is unchanged and a `cron-*.jsonl` file exists in `./.hermind/trajectories/`.
- API tests: empty history returns `[]`; POST triggers SSE; old session routes return 404.
- Frontend e2e: send message → stream → switch model → send another → reload → history persists.
- Deleted test suites: `api/handlers_sessions*_test.go`, `api/session_registry_test.go`, `gateway/**/*_test.go`, `cli/gateway_test.go`, `cli/acp_test.go`, and all frontend session-related tests.

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Migration bug destroys a user's `state.db` | High | `rename` target is stat-checked; if the backup name is taken, append unix-ms suffix; refuse to start on any rename error rather than overwrite. Unit test covers the collision path. |
| gateway/acp deletion silently breaks downstream integrations | Medium | Announce in CHANGELOG and README as a 0.3.0 breaking change. Tag the last 0.2.x commit so integrators can cherry-pick. |
| `api/webroot/` drifts from `web/dist/` | Low | Retain the existing `chore(webroot): rebuild …` commit convention. PR 2 ends with a dedicated bundle-rebuild commit. |
| Cron behavior change (no longer appears in main history) surprises a user | Low | CHANGELOG entry; cron is a niche path. |
| Random port prevents fixed URL bookmarking | Low | `--addr` flag retained as escape hatch; banner clearly prints the chosen port. |
| Intermediate state between PR 1 and PR 2 merges | Low | Land PRs sequentially. PR 1 does not touch session code, so no cross-bug is expected. |
| i18n keys left orphaned | Low | Grep checklist before PR 2 merge. |

## Out of Scope

- No replacement gateway/bot framework. If someone needs multi-platform bot behavior
  later, it grows back as a separate product.
- No UI for clearing the conversation. Users `rm ./.hermind/state.db` if they want a
  clean slate.
- No data migration tooling. The backup is left for manual archaeology.
- No change to memory providers' configuration shape. Instance-scoped config is
  already sufficient for honcho/mem0/etc.
