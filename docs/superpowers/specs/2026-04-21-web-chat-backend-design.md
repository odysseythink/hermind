# Web Chat Backend (Phase 1 of 3) — Design Spec

**Date:** 2026-04-21
**Status:** Draft — awaiting user review
**Scope:** Add HTTP endpoints that let a web client submit a chat message, have the agent `Engine` run it, and receive streamed events via the existing WebSocket / SSE endpoints. No front-end work, no TUI removal.

---

## 1. Why a phase 1

The larger goal is to retire the TUI (`cli/ui/`) and make the web UI the only chat interface. That work splits into three phases, each landable independently:

| Phase | Deliverable | Still works after? |
|---|---|---|
| **1 (this spec)** | Backend chat dispatch + cancel endpoints; `sessionrun` runner; shared engine-deps builder. | TUI unchanged, web has chat over `curl` / future React client. |
| 2 | React chat screen consuming these endpoints + existing WS stream. | TUI unchanged. |
| 3 | Delete `cli/ui/`, point `hermind` / `hermind run` at the web server. | TUI gone. |

Phase 1 is the enabling server work. Nothing user-visible ships yet — that's Phase 2 — but after this phase, the backend is sufficient for a REST client to drive a full chat round-trip.

## 2. Goals

- A REST client can submit one user message, have the agent respond, and observe the response as streamed events on the existing `GET /api/sessions/{id}/stream/{ws|sse}` endpoints.
- A client can cancel an in-flight response.
- A single session cannot be driven by two concurrent submissions (second submission rejected with 409).
- The code path is the HTTP equivalent of the current TUI dispatcher (`cli/ui/run.go:58-101`), reusing the agent `Engine` and storage layer unchanged.

## 3. Non-goals

- React front-end (Phase 2).
- TUI removal or `hermind run` rewiring (Phase 3).
- Preserving the assistant's partial output when a run is cancelled mid-stream.
- New slash-command / skill-toggle APIs.
- A degraded-mode stub provider response path — the TUI's `newStubProvider` fallback is dropped for the web. Web returns `503` until the user configures a provider via the existing Config panel.
- Changes to the agent `Engine`, `StreamHub`, `storage` layer, or existing WS/SSE endpoints.

## 4. Architecture overview

```
HTTP Client
  POST /api/sessions/{id}/messages {text,model?}  ─┐
  POST /api/sessions/{id}/cancel                 ─┐│
  GET  /api/sessions/{id}/stream/{ws|sse}       ←─││───┐
                                                 ││   │
                                                 ▼▼   │
         ┌────────────────────────────────────────────│──┐
         │  api/handlers_session_run.go               │  │
         │    handleSessionMessagesPost               │  │
         │    handleSessionCancel                     │  │
         └────────┬───────────────────────────────────│──┘
                  │                                   │
                  ▼                                   │
         ┌──────────────────────────────────────────┐ │
         │  api/sessionrun/runner.go                │ │
         │    Run(ctx, Deps, Request) error         │ │
         │    - builds agent.Engine                 │ │
         │    - wires stream callbacks → Hub.Publish│ │
         │    - calls Engine.RunConversation        │ │
         │    - recovers panics                     │ │
         │    - publishes terminal status event     │ │
         └────────┬─────────────────────────────────┘ │
                  │                                   │
                  ▼                                   │
         ┌──────────────────────────────────────────┐ │
         │  api.StreamHub (existing)   ─────────────┼─┘
         │  storage.SQLite (existing)                │
         └──────────────────────────────────────────┘
```

Session running state is tracked in `api.SessionRegistry`, a small `map[string]context.CancelFunc` behind a `sync.Mutex`. The POST handler registers a cancel func; the cancel handler looks it up and invokes it; the runner's goroutine clears its own entry via `defer`.

## 5. File layout

**Create:**

- `api/session_registry.go` + `api/session_registry_test.go` — `SessionRegistry` with `Register / Cancel / IsBusy / Clear`. No dependencies.
- `api/sessionrun/runner.go` + `api/sessionrun/runner_test.go` — `Deps`, `Request`, `Run(ctx, Deps, Request) error`. Pure logic, no HTTP.
- `api/handlers_session_run.go` + `api/handlers_session_run_test.go` — two handlers: `handleSessionMessagesPost`, `handleSessionCancel`.
- `cli/engine_deps.go` (or similar) — `BuildEngineDeps(cfg *config.Config) (sessionrun.Deps, error)` extracted from `cli/repl.go:44-145`.

**Modify:**

- `api/server.go` — register two new routes under `/api`, add `SessionRegistry` field on `Server`, new `ServerOpts` fields for the `sessionrun.Deps` pre-built.
- `api/dto.go` — add `MessageSubmitRequest{Text, Model}` and `MessageSubmitResponse{SessionID, Status}`.
- `cli/repl.go` — replace the provider/aux/tool/skills construction block (currently lines ~44-145) with a call to `BuildEngineDeps`.
- `cli/web.go` — call `BuildEngineDeps` to populate `ServerOpts`.

**Do not touch:**

- `agent/engine.go`, `storage/`, `api/ws.go`, `api/sse.go`, `api/stream_hook.go`, `api/handlers_sessions.go`, `api/handlers_messages.go`, `cli/ui/`, `web/` frontend.

## 6. Endpoint contracts

### 6.1 `POST /api/sessions/{id}/messages`

**Request:**
```
POST /api/sessions/abc-123/messages
Authorization: Bearer <token>
Content-Type: application/json

{"text": "hello", "model": "claude-opus-4-7"}
```
`model` is optional; empty string falls back to `cfg.Model`.

**Responses:**

| Status | Body | Condition |
|---|---|---|
| 202 | `{"session_id":"abc-123","status":"accepted"}` | Engine goroutine launched successfully. |
| 400 | `{"error":"invalid json"}` | malformed body. |
| 400 | `{"error":"text is required"}` | missing / empty `text`. |
| 400 | `{"error":"missing session id"}` | empty URL param. |
| 401 | — | missing / wrong bearer token (existing middleware). |
| 409 | `{"error":"session busy"}` | `SessionRegistry.IsBusy(id)` is true. |
| 503 | `{"error":"provider not configured; open Config panel to set api_key"}` | `cfg.Provider.APIKey == ""`. |

202 returns in milliseconds; it never blocks on Engine. The actual response is delivered via the stream endpoint.

### 6.2 `POST /api/sessions/{id}/cancel`

**Request:**
```
POST /api/sessions/abc-123/cancel
Authorization: Bearer <token>
```
No body.

**Responses:**

| Status | Body | Condition |
|---|---|---|
| 204 | — | cancel dispatched. |
| 400 | `{"error":"missing session id"}` | empty URL param. |
| 401 | — | missing / wrong bearer token. |
| 404 | `{"error":"session not running"}` | id not in registry. Idempotent: a second cancel returns 404. |

## 7. Data flow

### 7.1 Submit path

```
POST /messages received
  ├─ 1. JSON decode → MessageSubmitRequest
  ├─ 2. id := chi.URLParam("id"); if empty → 400
  ├─ 3. if req.Text == "" → 400
  ├─ 4. if cfg.Provider.APIKey == "" → 503
  ├─ 5. if registry.IsBusy(id) → 409
  ├─ 6. ctx, cancel := context.WithCancel(context.Background())
  │     (not r.Context() — that dies when handler returns)
  ├─ 7. registry.Register(id, cancel)
  ├─ 8. go func() {
  │        defer registry.Clear(id)
  │        _ = sessionrun.Run(ctx, deps, Request{SessionID:id, UserMessage:req.Text, Model:req.Model})
  │     }()
  └─ 9. 202 Accepted
```

### 7.2 Runner loop (inside `sessionrun.Run`)

```
Run(ctx, deps, req):
  defer recover() → on panic, Hub.Publish(status, state=error, error="internal: …"); return panicErr
  engine := agent.NewEngineWithToolsAndAux(deps.Provider, deps.AuxProvider, deps.Storage, deps.ToolReg, deps.AgentCfg, "web")
  if deps.SkillsReg != nil { engine.SetActiveSkillsProvider(…) }
  engine.SetStreamDeltaCallback(d => Hub.Publish(StreamEvent{Type:"token", SessionID, Data: d}))
  engine.SetToolStartCallback(c => Hub.Publish(StreamEvent{Type:"tool_call", …}))
  engine.SetToolResultCallback((c,r) => Hub.Publish(StreamEvent{Type:"tool_result", …}))
  Hub.Publish(StreamEvent{Type:"status", Data:{"state":"running"}})
  result, err := engine.RunConversation(ctx, &agent.RunOptions{UserMessage:req.UserMessage, SessionID:req.SessionID, Model:req.Model})
  switch {
    case errors.Is(err, context.Canceled):
      Hub.Publish(StreamEvent{Type:"status", Data:{"state":"cancelled"}})
      return err
    case err != nil:
      Hub.Publish(StreamEvent{Type:"status", Data:{"state":"error","error":err.Error()}})
      return err
    default:
      Hub.Publish(StreamEvent{Type:"message_complete", Data:{"assistant_text": result.Text, "message_id": result.MessageID}})
      Hub.Publish(StreamEvent{Type:"status", Data:{"state":"idle"}})
      return nil
  }
```

### 7.3 Cancel path

```
POST /cancel received
  ├─ 1. id := chi.URLParam("id"); if empty → 400
  ├─ 2. ok := registry.Cancel(id)    // invokes the stored cancelFn, deletes the entry
  ├─ 3. if !ok → 404
  └─ 4. 204 No Content

The running goroutine sees ctx.Err() = context.Canceled, Engine returns,
Run publishes status(cancelled), defer registry.Clear(id) runs (no-op since
Cancel already removed the entry).
```

### 7.4 Persistence semantics

Engine's existing behavior drives this:
- The user message is persisted at the start of `RunConversation`.
- The assistant message is persisted on completion.
- On cancel: the user message is already in storage; the assistant message is NOT. History shows the user's turn unanswered.

This spec doesn't change any Engine-side persistence. If we decide later to keep partial assistant output, that's a Phase 2+ follow-up.

## 8. Error handling

| Failure | Location | Behavior |
|---|---|---|
| Malformed JSON / missing text | POST handler | 400 with specific message. |
| No provider configured | POST handler | 503 with actionable message. |
| Session busy | POST handler | 409. |
| Cancel missing session | Cancel handler | 404 (idempotent). |
| Engine returns `context.Canceled` | `sessionrun.Run` | Publish `status={state:cancelled}`, return err. |
| Engine returns any other err | `sessionrun.Run` | Publish `status={state:error,error:…}`, return err. |
| Panic inside Engine / callback | `sessionrun.Run` defer recover | Publish `status={state:error,error:"internal: …"}`, server stays alive. |
| Storage write fails | bubbled by Engine → Run's err branch | Same as "other err" above. |
| `api.Server` shutdown | extend `Server.Shutdown` | Walk registry, cancel every entry, best-effort wait up to 2s. |

**Invariants:**
1. POST handler returns a status line in milliseconds.
2. `registry.Clear(id)` always executes — `defer` in the launching goroutine.
3. Cancel is idempotent (second 204-or-404 is fine, no server state corruption).
4. `state=running` is always followed by exactly one terminal state (`cancelled | error | idle`).

## 9. Testing strategy

Three test files, decomposed by unit seam.

### 9.1 `session_registry_test.go`

- `TestRegistry_RegisterAndCancel` — Register → Cancel returns true and invokes fn; second Cancel returns false.
- `TestRegistry_IsBusy` — tracks lifecycle accurately.
- `TestRegistry_Concurrent` — 100 goroutines Register+Clear on distinct ids, run with `-race`.
- `TestRegistry_DuplicateRegister` — Register on same id twice: second returns false / noop (caller is expected to have gated with IsBusy).

### 9.2 `sessionrun/runner_test.go`

All tests use: in-memory storage, fake `provider.Provider`, empty `tool.Registry` (or one fake tool), fresh `StreamHub`.

- `TestRun_HappyPath` — fake provider emits 3 deltas. Collect events from Hub; assert sequence `status(running), token, token, token, message_complete, status(idle)`.
- `TestRun_ToolCall` — fake provider wires one tool use; assert events include `tool_call` and `tool_result`.
- `TestRun_ProviderError` — fake provider returns err on first delta; event sequence ends in `status(error, …)`; Run returns err.
- `TestRun_ContextCancelled` — start Run in goroutine, cancel ctx, assert `status(cancelled)` and Run returns `context.Canceled`.
- `TestRun_PanicRecovered` — fake tool panics; assert `status(error, msg~="internal")`; process still alive.

### 9.3 `handlers_session_run_test.go`

Use `httptest.Server` + real router + `ServerOpts` with fake provider injected.

- `TestMessagesPost_Accepted` — valid request → 202; subscribe SSE → see `status(running)` within 1s.
- `TestMessagesPost_MissingText` — 400.
- `TestMessagesPost_InvalidJSON` — 400.
- `TestMessagesPost_NoProvider` — config with empty `APIKey` → 503.
- `TestMessagesPost_Busy` — fake provider with blocking delta channel keeps first run alive; second POST same id → 409.
- `TestMessagesPost_Unauthorized` — no Bearer → 401 (validates middleware wiring).
- `TestCancelPost_Running` — start a long run → POST /cancel → 204; subscribe SSE → see `status(cancelled)`.
- `TestCancelPost_NotRunning` — no prior POST → 404.
- `TestCancelPost_Idempotent` — after successful 204, second cancel → 404.

### 9.4 Deliberately not tested here

- Real provider round-trip (covered by `agent/` tests).
- WS/SSE transport (covered by `api/ws_test.go`, `api/sse_test.go`).
- TUI path (untouched in Phase 1).

## 10. Out of scope / future

- Persisting partial assistant output on cancel.
- Request cancellation via WebSocket inbound frame.
- Multi-user auth (single-token model stays).
- "Stop + edit last prompt" UX in the API (client can POST cancel → POST new message themselves).
- Streaming user message echo back to the stream (the Engine writes to storage directly; subscribers observing via `message_complete` and storage history see it).

## 11. Approval

After user review, next step is `writing-plans` to produce a task-by-task implementation plan against this spec.
