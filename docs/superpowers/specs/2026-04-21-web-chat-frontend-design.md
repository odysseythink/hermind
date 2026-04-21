# Web Chat Frontend (Phase 2 of 3) — Design Spec

**Date:** 2026-04-21
**Status:** Draft — awaiting user review
**Scope:** Add a React chat workspace to the existing web UI that consumes Phase 1's backend dispatch + cancel endpoints and streams events over SSE. Config screens become a secondary "Settings" mode. No backend changes, no TUI removal.

---

## 1. Why a phase 2

Part of a three-phase retirement of the TUI:

| Phase | Deliverable | State after merge |
|---|---|---|
| 1 | Backend dispatch + cancel endpoints; `sessionrun` runner. | TUI unchanged. Web has chat over `curl` / API clients. |
| **2 (this spec)** | React chat workspace; config moves into a Settings mode. | TUI unchanged. Browser users get a real chat UI. |
| 3 | Delete `cli/ui/`, point `hermind run` at the web server. | TUI gone. |

**Depends on Phase 1.** Merging Phase 2 before Phase 1 ships broken chat (POST 404s). During development, msw stubs Phase 1's endpoints so the two can progress in parallel; they must ship together.

## 2. Goals

- Open the web UI → land in a chat workspace by default.
- Send a message, watch it stream back token by token.
- Cancel a running response.
- Browse and resume past sessions from a left sidebar.
- Configuration (the current seven config groups) moves into a Settings mode reached from a top-bar toggle.

## 3. Non-goals

- Backend / API changes (Phase 1).
- TUI removal or `hermind run` rewiring (Phase 3).
- Attachments (images, files). User opted out.
- Observability / telemetry panels (GroupId=`observability`, unbuilt).
- Message search / export.
- Persisting partial assistant output on cancel (Phase 1 already chose "drop"; UI honors that — interrupted drafts disappear on next reload).
- Virtualized message rendering (deferred; DOM works fine up to a few thousand messages).
- Multi-user auth.

## 4. Architecture

### 4.1 Top-level modes

Two modes, selected from hash routing:

| Hash | Mode | Left sidebar | Main area |
|---|---|---|---|
| `#/` or `#/chat` | Chat | Session list + New Chat | Most-recent or empty state |
| `#/chat/:sessionId` | Chat | Session list (item highlighted) | That session's messages |
| `#/settings` | Settings | — (redirect to `#/settings/models`) | — |
| `#/settings/:groupId` | Settings | Seven config groups nav | Existing config panels |

TopBar gains a right-side toggle: `[ Chat | Settings ]`, highlighting the current mode.

### 4.2 Component tree (Chat mode)

```
<App>
  <TopBar mode="chat">                    // includes Chat/Settings toggle
  <ChatWorkspace>
    <ChatSidebar>                         // web/src/components/chat/ChatSidebar.tsx
      <NewChatButton>
      <SessionList>
        <SessionItem ... />
    <ChatMain>                            // ChatWorkspace.tsx root child
      <ConversationHeader>                // title + <ModelSelector>
      <MessageList>                       // scroll area, stick-to-bottom
        <MessageBubble role>
          <MessageContent />              // markdown + code + math + mermaid
          <ToolCallCard />                // one per in-flight or past tool call
          <StreamingCursor />             // only on the actively streaming bubble
      <ComposerBar>
        <TextArea />                      // multi-line, Enter=send
        <SlashMenu />                     // overlay when text starts with "/"
        <SendButton />
        <StopButton />                    // only while status=running
  <Footer>                                // unchanged
```

### 4.3 Existing shell refactor

- `Sidebar.tsx` currently renders the seven config groups. Rename to `SettingsSidebar.tsx` (logic unchanged), introduce new `ChatSidebar.tsx`.
- `ContentPanel.tsx` → `SettingsPanel.tsx`; new `ChatPanel.tsx` renders `ChatWorkspace`.
- `App.tsx` chooses which pair to render based on `mode = 'chat' | 'settings'` parsed from the hash.
- `shell/hash.ts` parses the new two-mode hash; `migrateLegacyHash` rewrites any pre-Phase-2 links to `#/settings/...`.

## 5. File layout

**Create:**

- `web/src/components/chat/` (new directory, ~20 files):
  - `ChatWorkspace.tsx`, `ChatSidebar.tsx`, `NewChatButton.tsx`, `SessionList.tsx`, `SessionItem.tsx`
  - `ConversationHeader.tsx`, `ModelSelector.tsx`
  - `MessageList.tsx`, `MessageBubble.tsx`, `MessageContent.tsx`, `StreamingCursor.tsx`
  - `ToolCallCard.tsx`
  - `ComposerBar.tsx`, `SlashMenu.tsx`, `StopButton.tsx`
  - `markdown/CodeBlock.tsx`, `markdown/MermaidBlock.tsx`, `markdown/MathBlock.tsx`
- `web/src/hooks/useChatStream.ts`, `useSessionList.ts`, `useChatState.ts`
- `web/src/state/chat.ts` — new chat reducer. As a bonus, lift the current `web/src/state.ts` config reducer to `web/src/state/config.ts` so the two live side by side.
- `web/src/test/fakeEventSource.ts` — minimal EventSource polyfill for vitest.
- `web/src/components/chat/__tests__/` — co-located component tests (per existing convention where tests live next to components).

**Modify:**

- `web/src/App.tsx` — wire the two-mode router, mount ChatWorkspace or SettingsPanel.
- `web/src/shell/hash.ts` — parse `#/chat/:id` and `#/settings/:groupId`; legacy migration.
- `web/src/shell/groups.ts` — unchanged (still drives Settings mode).
- `web/src/components/shell/Sidebar.tsx` → rename file to `SettingsSidebar.tsx`; fix imports.
- `web/src/components/shell/ContentPanel.tsx` → rename to `SettingsPanel.tsx`; fix imports.
- `web/src/components/shell/TopBar.tsx` — add Chat/Settings mode toggle on the right.
- `web/src/api/schemas.ts` — add `MessageSubmitRequestSchema`, `MessageSubmitResponseSchema`, `SessionSummarySchema`, `ChatMessageSchema`, `StreamEventSchema` (a zod discriminated union over event `type`).
- `web/src/api/client.ts` — extend with POST helpers if only GET exists today.
- `web/package.json` — add deps: `react-markdown`, `remark-gfm`, `shiki`, `rehype-katex`, `katex` (for the CSS file), `mermaid` (marked for lazy-import; bundler respects dynamic imports).
- `web/src/main.tsx` or a dedicated styles file — import `katex/dist/katex.min.css`.

**Do not touch:**

- `api/` (Phase 1) — except that Phase 2 code consumes its endpoints.
- `web/src/components/groups/*` — the seven config panels stay exactly as they are.
- `cli/` (Phase 3).

## 6. State model

```ts
type ChatState = {
  activeSessionId: string | null;
  sessions: SessionSummary[];                    // left rail
  messagesBySession: Record<string, Message[]>;  // history cache
  streaming: {
    sessionId: string | null;                    // who is streaming now
    assistantDraft: string;                      // token-by-token buffer
    toolCalls: ToolCallSnapshot[];               // in-flight / done this turn
    status: 'idle' | 'running' | 'cancelling' | 'error';
    truncated: boolean;                          // set when cancelled or errored
    error: string | null;
  };
  composer: {
    text: string;
    selectedModel: string;                       // defaults to cfg.model
  };
};

type Message = {
  id: string;                                    // from storage or draft-*
  role: 'user' | 'assistant' | 'system';
  content: string;
  toolCalls?: ToolCallSnapshot[];                // only for assistant
  timestamp: number;
  truncated?: true;                              // UI-only flag for cancelled draft
};

type ToolCallSnapshot = {
  id: string;                                    // dedup key; server-sent
  name: string;
  input: unknown;
  result?: string;
  state: 'running' | 'done' | 'error';
};

type SessionSummary = {
  id: string;
  title: string;                                 // server may provide; else "New conversation"
  updatedAt: number;
};
```

**Action catalog:**

- `chat/session/select(id)` — swap activeSessionId; trigger lazy history fetch if not cached.
- `chat/session/created(id, title)` — optimistic insert on NewChat; later reconciled from the server.
- `chat/session/listLoaded(sessions)` — after GET /api/sessions.
- `chat/messages/loaded(id, messages)` — after GET /api/sessions/{id}/messages.
- `chat/stream/start(sessionId, userText)` — optimistic push the user message, flip status → running.
- `chat/stream/rollbackUserMessage(sessionId)` — undo the optimistic push on 409/503/500.
- `chat/stream/token(delta)` — append to assistantDraft.
- `chat/stream/toolCall(call)` / `toolResult(id, result)` — maintain toolCalls[].
- `chat/stream/complete(text, messageId)` — promote draft to final message; clear streaming.
- `chat/stream/cancelled` — set truncated, keep draft as a UI-only message; clear status.
- `chat/stream/error(message)` — same as cancelled, plus error text.
- `chat/composer/setText(text)`, `setModel(model)`.

## 7. Data flow

### 7.1 Sending a message

```
User types → SendButton click or Enter:
  1. dispatch(chat/stream/start(activeSessionId, text))
     // Optimistic user message appears immediately; status → running
  2. POST /api/sessions/{activeSessionId}/messages
       body: { text, model: composer.selectedModel }
       202 → do nothing; SSE will arrive
       409 → dispatch(rollbackUserMessage) + toast "Session busy"
       503 → dispatch(rollbackUserMessage) + toast with link to #/settings/models
       401 → trigger token-refresh flow (apiFetch handles)
       other → dispatch(rollbackUserMessage) + toast "Send failed: <msg>"
```

### 7.2 Receiving stream events

SSE is subscribed in `useChatStream.ts`. On `activeSessionId` change: close old `EventSource`, open new `GET /api/sessions/{id}/stream/sse?t=<token>`. Filter events where `data.session_id !== activeSessionId`.

> **Auth note:** Browser `EventSource` cannot set custom headers, so the token goes in the query string. This requires the auth middleware to accept `?t=<token>` as an alternative to the `Authorization: Bearer` header for the SSE route (and already does for the index HTML). If the current middleware only accepts the header, Phase 1 or the first Phase 2 task adds query-string fallback for `GET /api/sessions/{id}/stream/*`.

```
onEvent(evt):
  switch (evt.type):
    "status"      → state field "running" is already set locally; "idle" is a no-op;
                    "cancelled" → dispatch(stream/cancelled);
                    "error"     → dispatch(stream/error, data.error)
    "token"       → throttledDispatch(stream/token, evt.data.text)
                    (see §8 — 60fps rAF throttle)
    "tool_call"   → dispatch(stream/toolCall, {id, name, input, state:'running'})
    "tool_result" → dispatch(stream/toolResult, {id, result, state:'done'})
    "message_complete" → dispatch(stream/complete, {text, messageId})
```

### 7.3 Cancel

```
StopButton click:
  1. POST /api/sessions/{activeSessionId}/cancel
       204 → status = 'cancelling'; wait for SSE status(cancelled)
       404 → ignore (race: response completed naturally)
```

### 7.4 Session list

`useSessionList.ts` fetches `GET /api/sessions?limit=50` on mount, stores summaries. New Chat:

```
NewChatButton click:
  1. id := crypto.randomUUID()
  2. dispatch(session/created(id, "New conversation"))
  3. history.replaceState(hash = #/chat/<id>)
  // Backend hasn't heard of the session yet; first POST /messages will
  // create the row on the server side.
```

Selecting an existing session:

```
SessionItem click:
  1. hash → #/chat/<id>
  2. dispatch(session/select(id))
  3. if messagesBySession[id] missing:
       GET /api/sessions/<id>/messages?limit=200
       dispatch(messages/loaded(id, rows))
```

### 7.5 Slash menu

Trigger: the composer text is exactly `/...` (leading slash at column 0). Menu items:

| Command | Action |
|---|---|
| `/new` | Close menu, NewChatButton's handler |
| `/clear` | Clear composer text |
| `/settings` | Navigate to `#/settings/models` |
| `/model` | Focus `<ModelSelector>` (no text inserted) |

Selecting a command clears the composer (except `/model`). Keyboard nav: ↑/↓ through items, Enter to accept, Esc to close.

## 8. Error handling and UX polish

| Condition | UI behavior |
|---|---|
| 409 Session busy | Rollback optimistic user message; toast "Session busy — wait for current response" (5s). |
| 503 No provider | Rollback; toast "Provider not configured — open Settings" with clickable link. |
| 401 Token bad | `apiFetch` interceptor triggers refresh; no chat-side handling. |
| Stream cancelled | Keep `assistantDraft` as a message with `truncated:true`; render "— interrupted by user" in muted color. Note: not persisted, vanishes on reload. |
| Stream error | Same as cancelled, with red "Request failed: \<msg\>". |
| SSE connect fail (initial) | Banner at top of ChatMain: "Can't connect to stream. [Retry]". Composer disabled until connected. |
| SSE disconnect mid-stream | EventSource auto-reconnect attempts; if all fail, banner + allow new messages (previous assistantDraft stays as truncated). |
| User scrolls up mid-stream | Release stick-to-bottom; show floating "↓ New messages" chip; click returns to bottom + re-sticks. |

**Token throttling:** assistantDraft deltas arrive one token at a time; dispatching on every token can fire 100+ times per second. Buffer in a ref, flush to state via `requestAnimationFrame` (≤ 60fps). Implementation lives inside `useChatStream.ts`.

**Message actions (Phase 2 minimum):** Copy button on assistant bubbles. No edit / retry / delete — those are follow-ups.

## 9. Markdown rendering

Pipeline inside `MessageContent.tsx`:

```
react-markdown
  plugins: remark-gfm, remark-math
  rehype: rehype-katex
  components overrides:
    code → CodeBlock (shiki for fenced code; inline stays <code>)
    pre  → pass-through (CodeBlock owns the <pre>)
```

`CodeBlock.tsx` detects `language === "mermaid"` → renders `MermaidBlock`, which lazy-imports `mermaid` via dynamic import on first render.

`MermaidBlock.tsx` caches the `mermaid` module in module-level state so subsequent renders are synchronous.

`MathBlock.tsx` is implicit (KaTeX via rehype plugin; no custom component needed unless we want block styling).

**Images:** `<img>` via react-markdown default. No special handling in Phase 2.

## 10. Bundle size budget

Initial JS (gzipped):

| Piece | Size |
|---|---|
| Today (config-only) | ~250KB |
| + react-markdown + remark-gfm + remark-math | ~60KB |
| + shiki (eager, common languages) | ~300KB |
| + rehype-katex + katex CSS | ~80KB |
| + chat components + reducer | ~60KB |
| **Phase 2 total target** | ~750KB gzipped initial |
| + mermaid (lazy-loaded only when seen) | +500KB on-demand |

If the initial budget turns out painful on slow networks post-merge, follow-up optimization: lazy-load shiki too, show a `pre` placeholder until the highlighter boots. Not worth pre-optimizing.

## 11. Testing strategy

Three layers: reducer (pure), hooks (renderHook + msw + fake EventSource), components (testing-library + msw).

### 11.1 Reducer `state/chat.test.ts`

- Session select / created / listLoaded / messages/loaded paths
- Every `chat/stream/*` action on a known starting state
- Rollback correctness: `stream/start` → `rollbackUserMessage` leaves state identical to pre-start

### 11.2 Hooks

- `useChatStream_subscribes` — mount builds `EventSource`; change activeSessionId → close+reopen
- `useChatStream_dispatchesEvents` — feed each event type, verify dispatch
- `useChatStream_filtersStaleSession` — event with wrong session_id → dropped
- `useChatStream_reconnects` — simulate error, recover
- `useSessionList_loads` — msw returns fixed set
- `useSessionList_optimisticNew` — new button → list head + active swap

### 11.3 Components

- `ComposerBar`: Enter sends, Shift+Enter inserts newline, slash triggers menu
- `SlashMenu`: keyboard nav, selecting `/clear` wipes text, Esc closes
- `ModelSelector`: default is cfg.model, change dispatches setModel
- `MessageBubble`: role styling; user vs assistant alignment
- `MessageContent`:
  - markdown basics (bold, list)
  - fenced `python` → shiki container visible with language label
  - fenced `mermaid` → lazy-loads mermaid (verified via `vi.mock('mermaid')`)
  - inline `$x^2$` → KaTeX DOM present
- `ToolCallCard`: collapsed/expanded toggle; running vs done styling
- `SessionList`: active item highlighted; click swaps active
- `StopButton`: hidden when idle; click issues POST /cancel (msw match)

### 11.4 Integration (`App.test.tsx` additions)

- Happy path: mount → sessions list → type → Enter → POST intercepted → SSE simulated → UI renders draft → complete event → final message
- Mode switching: Chat ↔ Settings preserves the other mode's state
- 409 busy: second send shows toast + no duplicate user bubble

### 11.5 Infra

- `web/src/test/fakeEventSource.ts` — minimal polyfill (addEventListener, `dispatchEvent({type, data})`), returned from a helper that replaces the global `EventSource` for the test's lifetime.
- msw handlers for: `GET /api/sessions`, `GET /api/sessions/{id}/messages`, `POST /api/sessions/{id}/messages`, `POST /api/sessions/{id}/cancel`, `GET /api/config`, `GET /api/config/schema`, `GET /api/platforms/schema` (the last three already needed by existing App.test.tsx).

### 11.6 Deliberately not tested

- `shiki`, `mermaid`, `katex` internal rendering — their job.
- Visual regression — no Storybook / Chromatic in-tree.
- Real network against a running backend — msw only.

## 12. Out of scope / future

- Virtualized MessageList (when a session grows past ~5k messages, we'll want `react-virtuoso` or similar).
- Search within / across sessions.
- Export session (markdown, JSON).
- Edit/retry/delete individual messages.
- Attachments (explicitly dropped).
- Observability panel.
- Mobile-friendly responsive breakpoints; current sidebar layout assumes desktop widths.
- Theme / dark mode toggle (stick with current styles).

## 13. Approval

After user review: invoke `writing-plans` skill to produce a task-by-task implementation plan against this spec. Plan size estimate: 20-30 tasks given the component count; may split into sub-batches (shell plumbing → chat internals → markdown → tests) during plan writing.
