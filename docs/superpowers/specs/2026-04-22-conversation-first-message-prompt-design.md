# Conversation first-message-driven prompt & title

**Status:** draft
**Date:** 2026-04-22

## Problem

The current chat/session model in hermind doesn't match how the operator thinks
about conversations:

- Web chat creates sessions eagerly (front-end UUID, inserts a "New conversation"
  placeholder into the sidebar before any message is sent).
- IM gateway creates sessions lazily (on first message arrival).
- The two flows produce different session lists; the web sidebar doesn't show
  IM-originated conversations at all.
- `system_prompt` is snapshotted from `config.agent.system_prompt` at session
  creation time, but there is no mechanism to incorporate the user's first
  message as part of the persona.
- `title` is either empty or a generic "New conversation" string — users can't
  tell conversations apart at a glance.

## Goals

1. A unified conversation list across web-chat and IM-gateway sources.
2. Each conversation is created lazily on the *first* user message, in both flows.
3. At creation, the conversation's system prompt is frozen as
   `config.agent.system_prompt + "\n\n" + first_user_message` and never
   recomputed.
4. At creation, the conversation's title is the first 10 Unicode runes of the
   first user message (whitespace-trimmed, newlines replaced with spaces), and
   is editable by the operator.
5. Existing session and message data is dropped via migration (clean slate) —
   not retrofitted.
6. The operator cannot edit the system prompt post-creation. Only the title.

## Non-Goals

- Letting the operator edit, reset, or introspect the system prompt after
  creation. A future spec can add this; this one explicitly does not.
- Per-IM-instance persona binding (a separate discussion the user withdrew).
- Multi-user / auth beyond the existing web token gate.
- Changing how messages are streamed, stored, or displayed within a conversation.
- Any change to the `messages` table schema.

## Design

### Architecture: extend `ensureSession`

Both the web chat and IM gateway message-dispatch paths already funnel through
`agent.Engine.RunConversation → ensureSession` (where the session row is first
materialized). This is the single place where "first user message" semantics
live. Route 1 of the brainstorm.

Alternatives considered and rejected:

- **Two-phase creation (create empty row, then finalize).** Adds a write, leaves
  a race window, no benefit.
- **Move prompt composition into the storage layer.** Creates a reverse
  dependency from storage → config, and expands the test surface. Storage
  stays oblivious.

### Data model

No schema changes. The existing `sessions.system_prompt TEXT` and `sessions.title
TEXT` columns already exist and gain new *semantics*:

| Column | Before | After |
|---|---|---|
| `system_prompt` | snapshot of `config.agent.system_prompt` at creation | `config.agent.system_prompt + "\n\n" + first_user_message`, frozen |
| `title` | empty or `"New conversation"` | `deriveTitle(first_user_message)`, editable via API |

No new migrations for adding columns; one migration for wiping existing rows
(see **Migration** below).

### `deriveTitle`

Pure function in a new file `agent/title.go`. Rules, already decided:

- Replace `\n` and `\r` with `" "`.
- `strings.TrimSpace`.
- Truncate to 10 Unicode code points (`[]rune(s)[:10]`).
- If the result is empty (the user sent pure whitespace or an empty message —
  defensive; the chat API rejects empty bodies), return the localized `Untitled`
  literal. Frontend renders the i18n equivalent when it sees an empty title in
  the DTO.

```go
// agent/title.go
package agent

import "strings"

const titleMaxRunes = 10

// DeriveTitle produces a short display title from the first user message.
// Newlines collapse to spaces; surrounding whitespace is trimmed; the result
// is capped at titleMaxRunes Unicode code points.
func DeriveTitle(msg string) string {
    s := strings.ReplaceAll(msg, "\n", " ")
    s = strings.ReplaceAll(s, "\r", " ")
    s = strings.TrimSpace(s)
    runes := []rune(s)
    if len(runes) > titleMaxRunes {
        runes = runes[:titleMaxRunes]
    }
    return string(runes)
}
```

### `ensureSession` changes

Current signature (`agent/conversation.go:267`) takes the already-built default
prompt and stores it as-is:

```go
func (e *Engine) ensureSession(ctx, opts *RunOptions, systemPrompt, model string) error
```

New signature returns the **effective** prompt so `RunConversation` can use the
frozen value (from storage) on subsequent turns, not the re-built default:

```go
// ensureSession creates a new session row if it doesn't exist. On new rows,
// the stored system prompt is composed as `defaultPrompt + "\n\n" + firstMsg`
// and title is DeriveTitle(firstMsg). Returns the session as stored (fresh or
// existing) so the caller uses a frozen prompt, not a rebuilt one.
func (e *Engine) ensureSession(
    ctx context.Context,
    opts *RunOptions,
    defaultPrompt, firstMsg, model string,
) (*storage.Session, error)
```

Body sketch:

```go
if s, err := e.storage.GetSession(ctx, opts.SessionID); err == nil {
    return s, nil  // existing row — return unchanged, do not re-compose
} else if !errors.Is(err, storage.ErrNotFound) {
    return nil, err
}

composed := defaultPrompt
if strings.TrimSpace(firstMsg) != "" {
    composed = defaultPrompt + "\n\n" + firstMsg
}
s := &storage.Session{
    ID:           opts.SessionID,
    Source:       e.platform,
    UserID:       opts.UserID,
    Model:        model,
    SystemPrompt: composed,
    Title:        DeriveTitle(firstMsg),
    StartedAt:    time.Now().UTC(),
}
if err := e.storage.CreateSession(ctx, s); err != nil {
    return nil, err
}
return s, nil
```

### `RunConversation` changes

Current flow (`agent/conversation.go:51-56`):

```go
systemPrompt := e.prompt.Build(&PromptOptions{Model: model, ActiveSkills: ...})
if e.storage != nil {
    e.ensureSession(ctx, opts, systemPrompt, model)  // stores the default
}
// … loop: req.SystemPrompt = systemPrompt  (rebuilt every call)
```

New flow — the rebuilt prompt is used only when storage is absent; otherwise
`session.SystemPrompt` wins and stays constant for the life of the session:

```go
defaultPrompt := e.prompt.Build(&PromptOptions{Model: model, ActiveSkills: ...})
effective := defaultPrompt
if e.storage != nil {
    s, err := e.ensureSession(ctx, opts, defaultPrompt, opts.UserMessage, model)
    if err != nil { return nil, err }
    effective = s.SystemPrompt
}
// … loop: req.SystemPrompt = effective
```

This consolidates the two subtle current bugs: edits to
`config.agent.system_prompt` leaking into long-running sessions, and the
per-turn rebuild cost.

### Unified session list

`storage.ListSessions` already returns all sources (`api`, `telegram`, `feishu`,
…). No change there. The web API handler must not filter:

```go
// api/handlers_sessions.go — GET /api/sessions
sessions, err := s.storage.ListSessions(ctx, storage.ListOpts{
    Limit:   clampInt(query("limit"), defaultLimit, maxLimit),
    OrderBy: "started_at DESC",
})
```

**Sort order:** `started_at DESC`. Newest conversation on top. No re-sort on
new messages (the user is typing; the list should not move under them). A
future "sort by last activity" toggle is explicitly out of scope.

**Backend DTO:** `api/dto.go::SessionDTO` already exposes `source`, `model`,
`started_at`, `message_count`, `title`. No Go change.

**Frontend schema widening:** `web/src/api/schemas.ts::SessionSummarySchema`
currently only types `{id, title?, updated_at?}` — narrower than what the
backend returns. Widen it to mirror `SessionDTO`:

```ts
export const SessionSummarySchema = z.object({
  id: z.string(),
  title: z.string().optional(),
  source: z.string(),
  model: z.string().optional(),
  started_at: z.number().optional(),
  ended_at: z.number().optional(),
  message_count: z.number().optional(),
});
```

The `updated_at` field is removed (backend doesn't emit it). Any consumer
referencing `updated_at` is updated to use `started_at`.

### Title rename API

New endpoint:

```
PATCH /api/sessions/{id}
Body:     {"title": "new title"}
Response: 200 + SessionDTO
Errors:
  400  empty title, title > 200 runes, malformed body
  404  session not found
  401  missing token
  500  storage error
```

**No new storage method needed.** `storage.Storage` already exposes
`UpdateSession(ctx, id, *SessionUpdate)` and the existing `SessionUpdate`
struct already has a `Title` field. The SQLite implementation at
`storage/sqlite/session.go:97` already handles `Title`: non-empty values are
written, empty is treated as "no change". The PATCH handler:

- Trims and validates `title` (non-empty, ≤ 200 runes) at the handler layer.
- Calls `UpdateSession(ctx, id, &SessionUpdate{Title: trimmed})`.
- Maps `storage.ErrNotFound` → 404; other errors → 500.
- On success fetches the fresh session via `GetSession` and returns it as
  `SessionDTO`.

The storage layer does not need to add new validation (empty-string reject is
handler-layer); storage tests stay focused on the SQL path.

### Frontend: session creation

`useSessionList.newSession()` currently generates a UUID client-side and
optimistically inserts a placeholder into the sidebar. Change it to:

- Clear the active `sessionId` in hash-state (route to `#/chat` with no id).
- Do **not** insert a placeholder row into the sidebar.
- Clear composer state.

A new session only appears in the sidebar after the backend emits a
`session_created` SSE event (or the next poll fetch returns it — see below).

### Frontend: session appears via `session_created` SSE event + polling

Two delivery channels, covering the two creation sources:

**(a) Web-chat (same-browser):** existing per-session SSE hub. Flow:

1. User clicks "+ New conversation". Frontend generates a UUID and sets
   `sessionId` in hash state. **Does not** insert anything into the sidebar
   list state.
2. `ChatWorkspace` re-renders with the new `sessionId`; `useChatStream` opens an
   SSE subscription for that id.
3. User types and hits send. `POST /api/sessions/{id}/messages` → 202 accepted,
   goroutine starts.
4. Inside `sessionrun.Run`, after `engine.RunConversation`'s internal
   `ensureSession` successfully creates a row, publish a new `StreamEvent`:

   ```go
   // api/stream_hook.go — Event Type values add "session_created"
   hub.Publish(StreamEvent{
       Type:      "session_created",
       SessionID: sessionID,
       Data:      sessionToDTO(session),  // full SessionDTO payload
   })
   ```

5. Frontend receives the event on its already-open subscription, dispatches
   `chat/session/created`, reducer inserts at the top of the sidebar list
   (idempotent on duplicate id).

No race: step 2 completes before step 3 (the user types in between), so the
subscription is registered before the goroutine publishes in step 4.

**Refinement — how `ensureSession` triggers the publish:** `sessionrun.Run`
calls `engine.RunConversation` which calls `engine.ensureSession`. To notify
the hub, `ensureSession` returns the `*storage.Session` and a `bool` flag
(`created`). `sessionrun.Run` reads the result via a new callback on
`RunOptions` or from the returned `ConversationResult`. Simpler option: the
engine exposes an optional `OnSessionCreated func(*storage.Session)` callback;
`sessionrun.Run` sets it to publish the `session_created` event.

**(b) IM-gateway-created sessions:** the web browser has no way to know about
a Telegram-user-initiated session in realtime. Add lightweight polling:

- `useSessionList` refetches `GET /api/sessions` every 10 seconds.
- Also refetches on `window.focus`.
- Merges results with local state by id (SSE-delivered rows take precedence
  if they arrive first; identical ids are deduped).

Polling is kept minimal (small payload, browser-only when tab focused) — good
enough for a single-operator local tool. A future spec can replace polling
with a websocket / server-push channel if latency becomes a concern.

### Frontend: source badge

`SessionItem` renders a small badge next to the title showing `source` in
uppercase mono, one-line, `--fs-xs`, `--muted`, 1px border, 2px radius — matches
the existing chip styling in `FallbackProviderEditor`. Web-created sessions use
`api` (the current source string in storage).

### Frontend: double-click rename

`SessionItem` adds an editing state:

```
idle       — renders the title as plain text
editing    — renders an <input>, autofocused, preselected
```

Transitions:

- `dblclick` on the title region → `editing` (don't break single-click selection).
- `Enter` or `blur` → `PATCH /api/sessions/{id}` with the trimmed value,
  optimistic update, rollback on failure.
- `Esc` → discard, return to idle with original title.
- Empty title or > 200 runes → save button disabled (same validation as the
  backend rejects, but lets the user fix it without a roundtrip).

Tooltip on the title text: `chat.sidebar.doubleClickToRename` ("Double-click to
rename" / "双击重命名").

Rename failure toasts: `chat.renameFailed` ("Rename failed: {msg}" / "重命名
失败：{msg}").

### i18n additions

```
chat.untitled                     "Untitled" / "未命名"
chat.sidebar.doubleClickToRename  "Double-click to rename" / "双击重命名"
chat.renameFailed                 "Rename failed: {{msg}}" / "重命名失败：{{msg}}"
```

## Migration

**Context:** `storage/sqlite/migrate.go` currently has no version-tracking
table. `Migrate()` runs the full `schemaSQL` with `CREATE TABLE IF NOT EXISTS`
and that's it. The inline comment already flags this as a gap: *"Does NOT yet
support incremental migrations beyond v1 — future plans will add a migrations
table."* This plan adds the minimum machinery needed.

### Lightweight `schema_meta` table

Add to `schemaSQL`:

```sql
CREATE TABLE IF NOT EXISTS schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '1');
```

### Migration runner

Extend `Migrate()` to run any versioned steps in order after ensuring the base
schema is present:

```go
func (s *Store) Migrate() error {
    if _, err := s.db.Exec(schemaSQL); err != nil {
        return fmt.Errorf("sqlite: migrate base: %w", err)
    }
    v, err := s.schemaVersion()
    if err != nil { return err }
    for v < currentSchemaVersion {
        next := v + 1
        if err := s.applyVersion(next); err != nil {
            return fmt.Errorf("sqlite: migrate v%d: %w", next, err)
        }
        v = next
    }
    return nil
}
```

### Version 2 step

```go
// applyVersion(2): drop all pre-first-message-prompt session data.
func (s *Store) applyV2(tx *sql.Tx) error {
    if _, err := tx.Exec("DELETE FROM messages"); err != nil { return err }
    if _, err := tx.Exec("DELETE FROM sessions"); err != nil { return err }
    _, err := tx.Exec(
        "UPDATE schema_meta SET value = '2' WHERE key = 'version'",
    )
    return err
}
```

Runs in a single transaction so partial failure leaves `version = 1`. This
deletes rows but keeps the schema — schema unchanged means older binaries can
still open the DB after rollback (they just see empty tables).

### Operator preflight checklist

Document in the PR and release notes:

1. Stop all running `hermind web` / `hermind gateway` processes.
2. Back up `~/.hermind/hermind.db` if any current conversations matter.
3. Pull / rebuild.
4. First startup applies migration v2 automatically (wipes sessions +
   messages). Confirm it ran: `sqlite3 hermind.db "SELECT * FROM schema_meta"`
   → `version|2`.
5. First subsequent message on any channel creates the first new-style
   conversation.

### Rollback

Revert the merge commit. The `schema_meta` table and version row are harmless
to older binaries (they ignore unknown tables). Tables are unchanged, rows
are already empty. No reverse migration needed.

## Testing

### Go

- `agent.DeriveTitle` table test with at least 8 inputs: empty, pure-whitespace,
  `"abc"`, exactly 10 runes, > 10 runes, contains `\n`, emoji, mixed CJK/ASCII.
- `ensureSession` when row does not exist: `SystemPrompt == default + "\n\n" +
  first`, `Title == DeriveTitle(first)`, `Source` / `UserID` / `Model`
  propagated correctly.
- `ensureSession` when row exists: no write, no change to `SystemPrompt` or
  `Title`.
- `RunConversation` uses `session.SystemPrompt` unchanged across turns; later
  edits to `config.agent.system_prompt` do not affect in-flight sessions.
- `storage.UpdateSession` with `Title` set: happy path writes the new title
  and reports `ErrNotFound` on missing id. This path already exists and has a
  test (`storage/sqlite/session_test.go`); add one more assertion for the
  not-found branch if it isn't covered.
- Migration `v2`: starting from a DB populated with multiple sessions and
  messages at `schema_meta.version = '1'`, run `Migrate()`, assert both
  `sessions` and `messages` are empty, assert `memories` is untouched, assert
  `schema_meta.version = '2'`.
- Migration `v2` idempotency: running `Migrate()` twice in a row leaves the DB
  in the same state (version stays at 2, no additional wipes).
- `GET /api/sessions`: returns mixed-source list ordered `started_at DESC`.
- `PATCH /api/sessions/{id}`: 200 with valid body, 400 on empty / 201-rune /
  malformed body, 404 on missing id, 401 without token.

### Frontend (Vitest + React Testing Library)

- `SessionItem` double-click → editing state, input autofocused and preselected.
- `SessionItem` Enter → `PATCH` called with `{title}`; title updates in place.
- `SessionItem` Esc → no API call, title restored.
- `SessionItem` blur with dirty value → saves.
- `SessionItem` blur with empty value → save blocked, error toast fired.
- Reducer `chat/session/created` action: new session inserts at top of
  sessionsBySource; dispatching the same id twice is a no-op (idempotent).
- `ChatWorkspace` first-message flow: when `POST /api/sessions/{id}/messages`
  returns `{session: {...}}`, the reducer receives `chat/session/created` with
  that DTO; sidebar shows the new row.
- `ChatSidebar` renders the `source` badge for each session.
- `ChatWorkspace`: clicking "+ New conversation" clears the composer and route,
  does **not** insert a placeholder row.

### Intentionally not tested

- Go stdlib `strings.TrimSpace` / `ReplaceAll` behavior.
- Front-end locale string resolution (i18n tests live elsewhere).

## Impact surface

| File / module | Change |
|---|---|
| `agent/title.go` | New: `DeriveTitle` pure function |
| `agent/conversation.go::ensureSession` | Compose prompt + derive title |
| `agent/conversation.go::RunConversation` | Read frozen `session.SystemPrompt`, stop rebuilding per-turn |
| `storage/sqlite/migrate.go` | Add `schema_meta` table, version-runner, v2 step |
| `api/handlers_sessions.go` | `PATCH /api/sessions/{id}`; confirm `listSessions` has no source filter |
| `api/stream_hook.go` (docs only) | Add `session_created` as a documented `Type` value |
| `api/sessionrun/runner.go` | Publish `session_created` event after engine's `ensureSession` creates a row |
| `web/src/api/schemas.ts` | Widen `SessionSummarySchema`; new `PatchSessionBodySchema` |
| `web/src/hooks/useSessionList.ts` | `newSession()` no longer inserts placeholder; poll + focus-refetch |
| `web/src/hooks/useChatStream.ts` | Handle `session_created` SSE event → reducer |
| `web/src/state/chat.ts` | Reducer action `chat/session/created` (idempotent insert) |
| `web/src/components/chat/SessionItem.tsx` | Double-click edit state, source badge |
| `web/src/components/chat/SessionList.tsx` | Pass `source` through |
| `web/src/components/chat/ChatWorkspace.tsx` | New-conversation click clears composer, no placeholder |
| `web/src/locales/{en,zh-CN}/ui.json` | `chat.untitled`, `chat.sidebar.doubleClickToRename`, `chat.renameFailed` |

## Open questions

None after Q1–Q6 and the three section-level gates were approved.

## Out of scope / future work

- Editing the system prompt after creation.
- Per-IM persona binding ("each IM binds to one conversation") — user withdrew this ask.
- Sort conversations by last activity.
- Per-conversation agent-config overrides (model, tools, skills).
- Archiving / deleting conversations from the UI (CRUD beyond rename).
