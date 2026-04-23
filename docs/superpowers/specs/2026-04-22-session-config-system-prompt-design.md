# Session config: explicit system prompt & in-session settings drawer

**Date:** 2026-04-22
**Status:** draft
**Scope:** web chat UI + agent/prompt + agent/conversation + storage + http api

## Problem

Today, on session creation, `agent.ensureSession` composes the stored
`SystemPrompt` as `defaultPrompt + "\n\n" + firstUserMessage` and freezes it
for the session's lifetime (`agent/conversation.go:292-294`). This conflates
two concerns — the agent's persona/role and the user's first request — into
one opaque string, and gives the user no way to configure either explicitly.

On the web frontend, the top-right corner of `ConversationHeader` exposes a
bare `<select>` of hardcoded models (`web/src/components/chat/ConversationHeader.tsx:15`,
`web/src/components/chat/ChatWorkspace.tsx:24`). There is no place to view or
edit the system prompt, no place to change model mid-conversation with
persistence, and no separation between "the agent's default behavior" and
"this specific conversation's overrides".

## Goal

Replace the implicit first-message concatenation with an explicit,
configuration-driven system prompt, and replace the top-right model dropdown
with a settings surface that lets the user edit both the **model** and the
**system prompt** of the current session at any time.

## Non-goals

- No prompt template library (named presets picker).
- No generalized per-session overrides map (temperature, max_turns, top_p, etc.).
- No `/config system <prompt>` command for Telegram.
- No data migration of existing sessions — their stored `SystemPrompt` column
  stays as-is (continues to contain the historical concatenation).
- No optimistic-locking or edit-conflict UI for multi-tab editing; last write
  wins, SSE `session_updated` keeps tabs eventually consistent.

## Decisions (from brainstorming)

1. **Default source**: Global default + per-session override. A new
   `config.Agent.DefaultSystemPrompt` field feeds the value; `ensureSession`
   snapshots it into `Session.SystemPrompt` at creation (frozen-at-creation
   semantics preserved).
2. **Editability window**: Anytime. Edits apply from the next turn. In-flight
   turns use the value locked at the top of `RunConversation`.
3. **Scope**: Back-end change is universal (all sources, including Telegram,
   stop concatenating the first message). UI is web-only for now.
4. **Migration**: None. Existing rows are left untouched.
5. **Settings surface**: Right-side drawer (~40% width, min 360px), opens
   over the message area.
6. **Identity relationship**: `defaultIdentity` (baked-in Hermind/platform
   identity) always comes first; the user's `DefaultSystemPrompt` is appended
   after it, before any active-skills block.

## Architecture overview

```
┌─────────────────────────────────────────────────────────────────────┐
│  Config                                                             │
│    Config.Agent.DefaultSystemPrompt  (NEW, yaml: default_system_prompt) │
└─────────────────────────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  agent.PromptBuilder                                                │
│    Build() = defaultIdentity                                        │
│              + "\n\n" + DefaultSystemPrompt  (if non-empty)         │
│              + "\n\n" + renderActiveSkills(...)  (if any)           │
└─────────────────────────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  agent.ensureSession (on NEW row)                                   │
│    Session.SystemPrompt = PromptBuilder.Build(...)    ← no concat   │
│    Session.Model        = RunOptions.Model                          │
│    Session.Title        = DeriveTitle(firstMsg)       ← unchanged   │
└─────────────────────────────────────────────────────────────────────┘
                 │
                 ▼  (at every turn start)
┌─────────────────────────────────────────────────────────────────────┐
│  RunConversation                                                    │
│    effectivePrompt = sess.SystemPrompt  ← already in code           │
│    model           = sess.Model         ← NEW: prefer session value │
└─────────────────────────────────────────────────────────────────────┘
                 ▲
                 │ PATCH /api/sessions/{id}
┌─────────────────────────────────────────────────────────────────────┐
│  storage.Storage.UpdateSession(ctx, id, *SessionUpdate{Model,SysPrompt}) │
└─────────────────────────────────────────────────────────────────────┘
                 ▲
                 │
┌─────────────────────────────────────────────────────────────────────┐
│  Web: SessionSettingsDrawer (NEW)  ←  gear button in ConversationHeader │
│    model <select>, system prompt <textarea>, Save → PATCH           │
└─────────────────────────────────────────────────────────────────────┘
```

## Data model

### New config field

```go
// config/config.go
type AgentConfig struct {
    MaxTurns            int               `yaml:"max_turns"`
    GatewayTimeout      int               `yaml:"gateway_timeout,omitempty"`
    Compression         CompressionConfig `yaml:"compression,omitempty"`
    DefaultSystemPrompt string            `yaml:"default_system_prompt,omitempty"` // NEW
}
```

Empty string means "no user-configured prompt"; the PromptBuilder emits only
the identity block (current behavior when identity is all we have).

### Storage interface

The existing `storage.SessionUpdate` struct (defined in `storage/types.go:59`)
and the `Storage.UpdateSession` method already exist. We extend the struct
with two new optional fields:

```go
// storage/types.go
type SessionUpdate struct {
    EndedAt      *time.Time
    EndReason    string
    Title        string     // existing: empty string = leave unchanged (current semantic)
    MessageCount *int
    Model        *string    // NEW: nil = leave unchanged; empty string = clear
    SystemPrompt *string    // NEW: nil = leave unchanged; empty string = clear
}
```

Why `*string` for the new fields but plain `string` for `Title`? Existing
callers treat empty `Title` as "unchanged"; preserving that is important for
backward compatibility. For the new fields, distinguishing "not provided"
from "explicitly empty" matters because setting an empty system prompt is a
legitimate operation. Title could be migrated to `*string` in a follow-up,
but that is out of scope here.

### SQLite implementation

Extend `storage/sqlite/session.go` `UpdateSession` and `storage/sqlite/tx.go`
`txImpl.UpdateSession` to include the two new columns in the dynamic
`UPDATE sessions SET ...` builder when their pointers are non-nil. No schema
migration needed — the columns already exist on the `sessions` table.

## PromptBuilder changes

```go
// agent/prompt.go
type PromptBuilder struct {
    platform            string
    defaultSystemPrompt string  // NEW
}

func NewPromptBuilder(platform, defaultSystemPrompt string) *PromptBuilder {
    return &PromptBuilder{platform: platform, defaultSystemPrompt: defaultSystemPrompt}
}

func (pb *PromptBuilder) Build(opts *PromptOptions) string {
    var parts []string
    parts = append(parts, defaultIdentity)
    if strings.TrimSpace(pb.defaultSystemPrompt) != "" {
        parts = append(parts, pb.defaultSystemPrompt)
    }
    if opts != nil && len(opts.ActiveSkills) > 0 {
        parts = append(parts, renderActiveSkills(opts.ActiveSkills))
    }
    return strings.Join(parts, "\n\n")
}
```

Callers of `NewPromptBuilder` update accordingly:

- `agent.NewEngine` / `NewEngineWithTools*` accept the prompt via
  `AgentConfig.DefaultSystemPrompt` (already present on the `Engine` init
  path through `config.Config.Agent`).
- Tests in `agent/prompt_test.go` pass `""` to preserve existing identity-only
  assertions; a new test case covers the append behavior.

## ensureSession changes

```go
// agent/conversation.go
func (e *Engine) ensureSession(ctx context.Context, opts *RunOptions,
    defaultPrompt, firstMsg, model string) (*storage.Session, bool, error) {

    if s, err := e.storage.GetSession(ctx, opts.SessionID); err == nil {
        return s, false, nil
    } else if !errors.Is(err, storage.ErrNotFound) {
        return nil, false, err
    }

    s := &storage.Session{
        ID:           opts.SessionID,
        Source:       e.platform,
        UserID:       opts.UserID,
        Model:        model,
        SystemPrompt: defaultPrompt,          // NO concatenation
        Title:        DeriveTitle(firstMsg),  // unchanged
        StartedAt:    time.Now().UTC(),
    }
    if err := e.storage.CreateSession(ctx, s); err != nil {
        return nil, false, err
    }
    return s, true, nil
}
```

The `firstMsg` parameter stays (Title derivation still needs it), but no
longer flows into `SystemPrompt`.

## RunConversation changes

One additional line: after `effectivePrompt = sess.SystemPrompt` in the
`e.storage != nil` branch, also refresh `model` from the session row so that
later turns respect user PATCH updates:

```go
if sess.Model != "" {
    model = sess.Model
}
```

This preserves the current fallback (`"claude-opus-4-6"`) when neither
RunOptions nor Session carries a model.

## HTTP API

### PATCH /api/sessions/{id}

Handler: `api/handlers_sessions.go` — extend the existing PATCH (title-only)
to accept `system_prompt` and `model`.

**Request body:**

```json
{ "title": "optional", "system_prompt": "optional", "model": "optional" }
```

Decoded into a struct of `*string` fields; nil = omit. Empty string IS a
valid value (clears the field). Validation (all three constants new, defined
near the handler):

- `title`: `len(*patch.Title) <= MaxSessionTitleBytes` (= 256)
- `system_prompt`: `len(*patch.SystemPrompt) <= MaxSystemPromptBytes` (= 32 * 1024)
- `model`: `len(*patch.Model) <= MaxModelNameBytes` (= 128)

If the current title-patch handler uses an inline literal instead of a named
constant, the plan stage replaces it with `MaxSessionTitleBytes` in the same
change.

Returns `200 OK` with the updated `SessionSummary`. Propagates via
`streams.Publish`:

```json
{ "type": "session_updated", "session_id": "<id>",
  "data": { "title": "...", "model": "...", "system_prompt": "..." } }
```

**Errors:** 400 on oversize / invalid JSON, 404 when session does not exist,
500 on storage failure.

### GET /api/sessions/{id}

If absent, add a handler returning `SessionSummary` with `title`, `model`,
`system_prompt`, `source`, `started_at`. The frontend uses this to hydrate
the drawer when opened on a session the current page hasn't seen mutate.

### POST /api/sessions/{id}/messages

Remove the `model` field from the request body. Model is now strictly a
session attribute, not per-message. Existing callers that rely on it get a
compile-time break in the Go layer; the TS client schema
(`MessageSubmitRequestSchema` in `web/src/api/schemas.ts`) drops the field.

## SSE events

Extend `api/sse.go` event catalog:

| Event             | Trigger                        | Payload                                           |
|-------------------|--------------------------------|---------------------------------------------------|
| `session_created` | `ensureSession` creates a row  | `SessionSummary` (existing)                       |
| `session_updated` | `PATCH /api/sessions/{id}` ok  | `{title, model, system_prompt}` (partial allowed) |

Encoder lives in the same place as `session_created`. Consumer shape matches
the existing encode contract so WebSocket and SSE stay aligned.

## Frontend components

### web/src/components/chat/ConversationHeader.tsx

Replace `ModelSelector` with a `SettingsButton` (gear icon). Props change:

```ts
type Props = {
  title: string;
  onOpenSettings: () => void;
};
```

Button: 24×24, 1px border, 2px radius, hover accent amber `#FFB800`
(consistent with DESIGN.md). `aria-label="Session settings"`. Keyboard:
Enter / Space opens.

### web/src/components/chat/SessionSettingsDrawer.tsx (NEW)

```tsx
type Props = {
  open: boolean;
  session: SessionSummary;            // current saved values
  modelOptions: string[];
  onClose: () => void;
  onSave: (patch: { model?: string; system_prompt?: string }) => Promise<void>;
};
```

- Absolute-positioned panel inside `<main>`, `right: 0`, `top/bottom: 0`,
  `width: clamp(360px, 40vw, 540px)`, `border-left: 1px solid` amber when
  focused else `#333`.
- Local state: `draftModel`, `draftPrompt` initialized from `session` on open.
- Textarea: `min-height: 180px`, monospace (DESIGN.md code font), 13px body
  size, resizable vertically only.
- Actions: right-aligned row, `Cancel` (discards draft + onClose) and
  `Save` (primary, amber border). Save disabled while draft equals session.
- Behavior:
  - Esc key → Cancel
  - Click outside → **no action** (prevent accidental draft loss)
  - If `open` prop flips to false externally, drop draft without saving.
- A11y: `role="dialog" aria-modal="true"`, focus trap, initial focus on
  textarea.
- **Conflict banner**: if the session prop's `system_prompt` or `model`
  changes while the drawer is open *and* differs from draft, render a small
  notice at the top: `"This session was updated in another window. Your
  changes are unsaved — save to overwrite, or cancel to discard."`

### web/src/components/chat/ChatWorkspace.tsx

- Delete `MODEL_OPTIONS` hardcoded array from this file; move to a shared
  constant (`web/src/api/models.ts` or similar) and import into both the
  drawer and `ChatWorkspace`.
- Delete `state.composer.selectedModel` from the reducer; it is replaced by
  reading `activeSession.model` directly.
- Remove the `model` argument from the `/messages` POST (matches backend
  change).
- Wire up drawer:
  ```tsx
  const [settingsOpen, setSettingsOpen] = useState(false);
  const activeSession = sessions.find(s => s.id === sessionId);
  // ...
  <ConversationHeader title={activeTitle} onOpenSettings={() => setSettingsOpen(true)} />
  {activeSession && (
    <SessionSettingsDrawer
      open={settingsOpen}
      session={activeSession}
      modelOptions={MODEL_OPTIONS}
      onClose={() => setSettingsOpen(false)}
      onSave={async (patch) => {
        await apiFetch(`/api/sessions/${encodeURIComponent(sessionId!)}`, {
          method: 'PATCH', body: patch,
        });
        setSettingsOpen(false);
        // SSE session_updated handles local state refresh
      }}
    />
  )}
  ```

### web/src/hooks/useSessionList.ts

- `SessionSummarySchema` gains `model` and `system_prompt` fields (already
  returned by backend; just surface them in TS).
- Rename `renameSession(id, title)` → `patchSession(id, patch)`. Callers
  in `ChatWorkspace.handleRename` update to `patchSession(id, { title })`.

### web/src/hooks/useChatStream.ts

Add `case 'session_updated'` next to `session_created`:

```ts
case 'session_updated': {
  const payload = SessionUpdatePayloadSchema.parse(parsed.data);
  dispatch({ type: 'chat/session/updated', sessionId: parsed.session_id!, patch: payload });
  break;
}
```

Reducer gains `chat/session/updated` action that merges the patch into the
sessions list via `useSessionList`'s imperative handle.

### /settings/agent page (global default prompt)

Extend the existing settings shell (`SettingsSidebar` + `GroupSection` pattern
used by `/settings/models`, `/settings/gateway`, etc.):

- New sidebar entry: "Agent"
- Panel: one `TextArea` field bound to `config.agent.default_system_prompt`,
  saved through the same config-write endpoint all other groups use.
- No new backend endpoint — the agent section slots into the existing config
  schema path.

## Error handling

| Scenario                                    | Behavior                                         |
|---------------------------------------------|--------------------------------------------------|
| PATCH 400 (oversize)                        | Toast: "System prompt too long (max 32KB)"       |
| PATCH 400 (invalid JSON)                    | Toast: generic "Failed to save settings"         |
| PATCH 404                                   | Close drawer + Toast: "Session not found"        |
| PATCH 500                                   | Keep drawer open, button re-enabled + Toast      |
| Network failure                             | Same as 500                                      |
| SSE `session_updated` with unknown id       | Ignored (no-op)                                  |
| Provider rejects new model on next turn     | Existing per-turn error surface (`chat.errorNoProvider`) |
| User sends message while drawer has draft   | In-flight turn uses **saved** values; draft is client-only until Save clicked |
| Two tabs Save concurrently                  | Last write wins. SSE `session_updated` reconciles both tabs. |

## Testing

### Backend

| Test                                                       | File                                           |
|------------------------------------------------------------|------------------------------------------------|
| `TestPromptBuilder_AppendsDefaultSystemPrompt`             | `agent/prompt_test.go`                         |
| `TestPromptBuilder_EmptyDefaultPreservesIdentityOnly`      | `agent/prompt_test.go`                         |
| `TestEnsureSession_NewRow_UsesDefaultPromptOnly` (replaces `ComposesPromptAndTitle`) | `agent/ensure_session_test.go` |
| `TestEnsureSession_NewRow_DerivesTitleFromFirstMsg`        | `agent/ensure_session_test.go`                 |
| `TestRunConversation_PrefersSessionModelOverRunOptions`    | `agent/engine_test.go` (add to existing file)  |
| `TestUpdateSession_PatchesModelAndSystemPrompt`            | `storage/sqlite/sqlite_test.go`                |
| `TestUpdateSession_EmptyStringClearsFields`                | `storage/sqlite/sqlite_test.go`                |
| `TestPatchSession_UpdatesModelAndSystemPrompt`             | `api/handlers_sessions_test.go`                |
| `TestPatchSession_EnforcesSystemPromptSizeLimit`           | `api/handlers_sessions_test.go`                |
| `TestPatchSession_404OnMissingSession`                     | `api/handlers_sessions_test.go`                |
| `TestPostMessage_RejectsModelField` (model no longer accepted) | `api/handlers_sessions_test.go`             |
| `TestSessionUpdatedEventBroadcast`                         | `api/sse_test.go` (or new `sse_session_updated_test.go`) |

### Frontend

| Test                                                            | File                                          |
|-----------------------------------------------------------------|-----------------------------------------------|
| Drawer: opens/closes, Esc cancels, outside click does not close | `SessionSettingsDrawer.test.tsx`              |
| Drawer: Save triggers PATCH with only changed fields            | `SessionSettingsDrawer.test.tsx`              |
| Drawer: conflict banner on external session_updated             | `SessionSettingsDrawer.test.tsx`              |
| Drawer: oversize prompt surfaces inline error                   | `SessionSettingsDrawer.test.tsx`              |
| Workspace: model comes from session, not composer state         | `ChatWorkspace.test.tsx`                      |
| Workspace: session_updated SSE merges into sessions list        | `ChatWorkspace.test.tsx`                      |
| Header: gear button replaces ModelSelector                      | `ConversationHeader.test.tsx`                 |
| Hook: `patchSession` supersedes `renameSession`                 | `useSessionList.test.ts` (existing or new)    |

## Rollout

Single PR. No feature flag — the behavior change is intentional and desired.
Merge order:

1. Backend (config field, PromptBuilder, ensureSession, storage, HTTP PATCH,
   SSE) — fully backward-compatible on-disk (no schema change, old sessions
   unaffected).
2. Frontend (drawer, hooks, sse handler, settings/agent page) — depends on
   the new PATCH endpoint and `session_updated` event from step 1.

Because the TS client schemas change (`model` removed from message POST,
`model`/`system_prompt` added to SessionSummary), the Go test that embeds the
Vite bundle (`api/webroot_embed_test.go` or equivalent) must see a rebuild of
`web/` before CI passes. Instructions in `web/README.md` cover the rebuild.

## Open questions for plan stage

- Does the existing `/settings` shell have a stable pattern for
  `<textarea>`-style fields? (`TextInput` is single-line.) If not, the plan
  stage adds a `TextAreaInput` field component mirroring `TextInput`.
- Should model options come from a live provider-discovery endpoint instead
  of a hardcoded list? **Answered: out of scope here — keep the hardcoded
  list; address separately.**
