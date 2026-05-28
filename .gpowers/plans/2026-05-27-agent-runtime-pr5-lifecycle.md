# Agent Runtime PR-AR-5 — Tool Approval + Lifecycle (Timeout/CORS/Telemetry/Shutdown) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close out the v1 main-line agent runtime. Land **tool approval** (per-requestId interactive gate, system-level + session-level auto-approve toggles), **total session timeout** (config-driven hard cap), **CORS hardening** (replace `CheckOrigin: true` with origin matching), **telemetry events** (`agent_chat_started`/`agent_chat_sent`/`agent_chat_terminated`), and **Shutdown polish** (drain pending approvals during graceful shutdown). After PR-AR-5, all design §10 lifecycle requirements are met and v1 is feature-complete.

**Architecture:**
- **Approval gate** is a thin wrap installed by `Builder.add` for sources tagged "needs approval" (MCP + AgentFlow). The wrap calls `Session.RequestApproval(skillName, payload, description)` inside the Handler before invoking the original. Built-in skills (rag-memory/docSummarizer/web-scraping/rechart) bypass.
- **Approval registry** lives on `Session` as `approvals map[requestID]chan approvalResp`, guarded by a mutex. Each `RequestApproval` allocates a UUID requestID, registers a channel, sends `toolApprovalRequest` frame, blocks on `select { ch | timeout | ctx.Done }`. The reader-loop wakes the corresponding channel on matching `toolApprovalResponse`.
- **Auto-approve** has two layers: global SystemSetting `agent_tool_auto_approve` (bool) and per-session client frame `{type:"setAutoApprove", enabled:bool}`. Both checked before allocating a requestID — when on, the gate is bypassed without ever sending an approval request.
- **Session timeout** is enforced via `context.WithTimeout(parentCtx, cfg.AgentSessionMaxDuration)` in `newSession`. When fires, ctx cancellation propagates to runLoop + LLM call + tool execution + pending approvals.
- **CORS** reads `cfg.AllowedOrigins []string` (CSV env). `*` = allow-any (with audit log warning on boot). Empty list = match the server's own host (safe default).
- **Telemetry** fires fire-and-forget goroutines that LogEvent with a 2s deadline; failures don't block the session (same pattern as PR-D's MCP audit).
- **Shutdown** is extended: `Abort("server shutting down")` already cancels ctx (PR-AR-2), but pending approvals must each receive an "aborted" response on their channels so the wrapped Handlers can return `tool.Error("approval cancelled")` and the agent loop unwinds cleanly.

**Tech Stack:** Go 1.25.5, gorilla/websocket v1.5.3, pantheon v0.0.9 (no upgrade). **No new dependencies.**

**Source spec:** `.gpowers/designs/2026-05-26-agent-runtime-design.md` §5.1, §5.2, §9, §10, §14 (PR-AR-5 row).

**Reference Node implementation:**
- `server/utils/agents/aibitat/plugins/websocket.js:106-180` — `requestToolApproval` request/response with 2-minute timeout
- `server/utils/agents/aibitat/plugins/websocket.js:24` — `WEBSOCKET_BAIL_COMMANDS` (already replicated in PR-AR-2's `reader.go`)
- `server/utils/agents/aibitat/plugins/websocket.js:107` — `skillIsAutoApproved({skillName})` (we map to single global toggle)
- `server/utils/helpers/agents.js` — `skillIsAutoApproved` impl + `AgentSkillWhitelist` (we deferred the whitelist — Phase 2)
- `server/endpoints/agentWebsocket.js:33` — `agentHandler.aibitat.abort()` (we map to `Session.Abort`)

---

## Pre-task: Read this section once before starting

### What landed in PR-AR-1 / 2 / 3 / 4 (use, don't re-implement)

Verified by direct file inspection:

- `internal/agent/types.go` — `FrameToolApprovalReq`/`FrameToolApprovalResp` constants **already declared**; `ClientFrame{Type, Feedback, Attachments, RequestID, Approved}` **already has RequestID + Approved fields** (PR-AR-1 was prescient)
- `internal/agent/reader.go:52-53` — already has the case stub:
  ```go
  case FrameToolApprovalResp:
      mlog.Info("agent: tool approval response received (handled in PR-AR-5)")
  ```
  This PR wires it properly.
- `internal/agent/runtime.go:48` — `CheckOrigin: func(r *http.Request) bool { return true }, // tightened in PR-AR-5` — exact replacement target.
- `internal/agent/session.go:64-65` — `// TODO(PR-AR-5): remove — temporary compatibility with PR-AR-1 handler.go` for the unused `conn *websocket.Conn` field. Delete in Task 3.
- `internal/agent/session.go:139-144` — `Session.Abort(reason)` already cancels ctx and sends wssFailure when reason != "".
- `internal/agent/runtime.go:53-78` — `Shutdown(ctx)` already loops via `Abort + poll`; PR-AR-5 extends to also drain approvals.
- `internal/agent/tools/builder.go:111-120` — `add(reg, seen, e, source)` is the single registration funnel; PR-AR-5 inserts the approval wrap here (or rather, in a new function `addWithApproval`).
- `internal/services/event_log_service.go:20` — `LogEvent(ctx, event, metadata, userID *int) error` — pre-existing.
- `internal/config/config.go:69-72` — MCP knobs already there; we add 2 more (`AgentSessionMaxDuration`, `AgentAllowedOrigins`).

### What's intentionally NOT in v1

- Per-tool / per-user whitelist (`AgentSkillWhitelist` DB table) — explicitly out per design §1.2; only the global + per-session toggle ship now
- Per-tool `IsInteractive` flag in pantheon tool.Entry — we ignore pantheon's flag and use our own source-based decision (MCP + Flow → require approval; default skills → no approval)
- `agent_chat_started` / `agent_chat_sent` to `Telemetry` (a separate service in Node) — we use `EventLogService.LogEvent` instead; if Telemetry exists in Go later, this is a one-line swap
- Reconnect after WS drop — out (Node also doesn't support)
- Approval payload preview rendering on FE — frontend concern; back end just emits the frame

### Tool approval flow (full sequence)

```
LLM generates tool_call (e.g., MCP tool "gmail-send")
  ↓
pantheon agent.executeTool → tool.Registry.Dispatch → wrapped Handler
  ↓
wrappedHandler:
  1. read auto-approve state (system setting + session toggle)
  2. if either is ON → call inner handler, return
  3. otherwise:
     - alloc requestID = uuid.NewString()
     - register approvalCh in session.approvals[requestID]
     - send WS frame: {type: "toolApprovalRequest", requestId, skillName, payload, description, timeoutMs}
     - select {
         case resp := <-approvalCh:   // user responded
             if resp.approved → call inner handler
             else → return tool.Error("rejected by user")
         case <-time.After(2 min):    // timeout
             return tool.Error("approval request timed out")
         case <-session.ctx.Done():   // shutdown/abort
             return tool.Error("approval cancelled (session ended)")
     }
     - in all cases: delete session.approvals[requestID]
  ↓
inner handler runs (e.g., MCP tool invocation)
  ↓
return result to pantheon agent
```

### `approvalResp` + payload shapes

Server frame (additions to `ServerFrame`):

```go
type ServerFrame struct {
    // ... existing fields ...
    RequestID    string `json:"requestId,omitempty"`     // for toolApprovalRequest
    SkillName    string `json:"skillName,omitempty"`     // for toolApprovalRequest
    Payload      any    `json:"payload,omitempty"`       // tool args, for toolApprovalRequest
    Description  string `json:"description,omitempty"`   // human-readable for toolApprovalRequest
    TimeoutMs    int    `json:"timeoutMs,omitempty"`     // for toolApprovalRequest
}
```

Client frame already has `RequestID` and `Approved` (PR-AR-1 was right).

### New surface (this PR)

```
backend/internal/agent/
├── approval.go                # NEW — Session.RequestApproval + approvals registry
├── approval_test.go           # NEW
├── telemetry.go               # NEW — logChatStarted / logChatSent / logChatTerminated (fire-and-forget)
├── telemetry_test.go          # NEW
├── cors.go                    # NEW — buildCheckOrigin(cfg) func(*http.Request) bool
├── cors_test.go               # NEW
├── runtime.go                 # MODIFY — CheckOrigin = buildCheckOrigin(cfg); Shutdown drains approvals
├── session.go                 # MODIFY — add approvals + autoApprove fields; total timeout ctx; remove conn TODO
├── handler.go                 # MODIFY — context.WithTimeout for session ctx; telemetry on Run start/finish
├── reader.go                  # MODIFY — implement FrameToolApprovalResp + setAutoApprove handler
├── types.go                   # MODIFY — add ClientFrame{Enabled bool} for setAutoApprove; ServerFrame approval fields
├── bridge.go                  # MODIFY — telemetry on OnMessage/OnTerminate

backend/internal/agent/tools/
├── builder.go                 # MODIFY — addWithApproval wraps Handler when source requires approval
├── builder_test.go            # MODIFY — assert approval wrap is applied for MCP/Flow sources

backend/internal/config/
└── config.go                  # MODIFY — add AgentSessionMaxDuration, AgentAllowedOrigins, AgentToolApprovalTimeout

backend/cmd/server/main.go   # MODIFY — wire AgentToolApprovalTimeout into Deps if needed
```

### Methods to ship (PR-AR-5 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `Session` | gains `approvals map[string]chan approvalResp`, `approvalsMu sync.Mutex`, `autoApprove atomic.Bool` | per-session approval registry |
| 2 | `Session` | `RequestApproval(ctx, skillName string, args any, desc string) (approved bool, reason string)` | blocking call, 2min timeout |
| 3 | `Session` | `setAutoApprove(b bool)` | called from reader on `setAutoApprove` client frame |
| 4 | `Session` | `cancelAllApprovals(reason string)` | called from Abort + Shutdown to wake pending approvals |
| 5 | `tools.Builder` | `addWithApproval(reg, seen, e, source, requiresApproval, approvalFn)` | wraps Handler when requiresApproval==true |
| 6 | `tools.ApprovalFn` | typedef `func(ctx context.Context, skillName string, args any, description string) (approved bool, reason string)` | passed to Builder |
| 7 | `tools.BuilderDeps` | gains `Approval ApprovalFn` (nilable; nil = always approve, used by tests) | |
| 8 | `agent` (unexported) | `buildCheckOrigin(cfg *config.Config) func(*http.Request) bool` | parses cfg.AgentAllowedOrigins CSV |
| 9 | `agent` (unexported) | `logChatStarted/Sent/Terminated(eventLog *services.EventLogService, userID *int, sessionUUID string, ...)` | each in own goroutine with 2s deadline |
| 10 | `Runtime.Shutdown` (extended) | also calls `cancelAllApprovals("server shutting down")` on each session | |
| 11 | `config.Config` | gains `AgentSessionMaxDuration time.Duration` (default 30m), `AgentAllowedOrigins string` (default empty = same-host), `AgentToolApprovalTimeout time.Duration` (default 2m) | env knobs |

### Client frame additions (`ClientFrame` extension)

```go
type ClientFrame struct {
    // ... existing fields ...
    Enabled bool `json:"enabled,omitempty"` // for setAutoApprove
}

// New const in types.go
const FrameSetAutoApprove = "setAutoApprove"
```

### Out of scope (explicit)

- `AgentSkillWhitelist` DB table + per-skill granular whitelist UI — Phase 2
- Per-user approval policy (vs the global toggle + session toggle) — Phase 2
- Approval audit trail beyond `LogEvent("agent.tool.approval", ...)` — minimal in PR-AR-5; richer reporting later
- Multi-WS reconnect (rebinding to existing approvals) — out; if WS drops, all pending approvals fail with `tool.Error("approval cancelled (session ended)")`
- `agent_chat_sent` Telemetry counts — we emit but don't aggregate; metrics dashboard is a Phase 2 concern
- "Bulk approve next N tool calls" UX — out; only single-call + session-wide on/off

### TDD discipline

Each task lands as **one commit**. Failing test → impl → green → full suite green → commit.

---

## Task 1: Session approval registry + reader wiring + 2min timeout

**Files:**
- `backend/internal/agent/types.go` (MODIFY — extend ServerFrame + ClientFrame, add FrameSetAutoApprove const)
- `backend/internal/agent/approval.go` (NEW)
- `backend/internal/agent/approval_test.go` (NEW)
- `backend/internal/agent/session.go` (MODIFY — add approvals fields)
- `backend/internal/agent/reader.go` (MODIFY — wire FrameToolApprovalResp + setAutoApprove)
- `backend/internal/agent/reader_test.go` (MODIFY — add 4 cases)
- `backend/internal/config/config.go` (MODIFY — add AgentToolApprovalTimeout)

**Tests:**
- `TestSession_RequestApproval_UserApproves_ReturnsTrue`
- `TestSession_RequestApproval_UserRejects_ReturnsFalse`
- `TestSession_RequestApproval_Timeout_ReturnsFalseWithReason`
- `TestSession_RequestApproval_CtxCancel_ReturnsFalseWithReason`
- `TestSession_RequestApproval_AutoApproveBypassesGate` (no frame sent)
- `TestSession_RequestApproval_ConcurrentRequests_RoutedByRequestID`
- `TestReader_ToolApprovalResp_WakesPendingApproval`
- `TestReader_SetAutoApprove_TogglesSession`
- `TestReader_ApprovalResp_UnknownRequestID_Ignored` (log + continue)

### Steps

- [ ] Extend `types.go`:
  ```go
  const FrameSetAutoApprove = "setAutoApprove"

  type ServerFrame struct {
      // ... existing ...
      RequestID   string `json:"requestId,omitempty"`
      SkillName   string `json:"skillName,omitempty"`
      Payload     any    `json:"payload,omitempty"`
      Description string `json:"description,omitempty"`
      TimeoutMs   int    `json:"timeoutMs,omitempty"`
  }

  type ClientFrame struct {
      // ... existing ...
      Enabled bool `json:"enabled,omitempty"`
  }
  ```

- [ ] Add config knob:
  ```go
  // In config.go, alongside MCP knobs:
  AgentToolApprovalTimeout time.Duration `env:"AGENT_TOOL_APPROVAL_TIMEOUT" envDefault:"2m"`
  ```

- [ ] Extend `Session` (in `session.go`):
  ```go
  type Session struct {
      // ... existing fields ...

      // PR-AR-5: approval registry
      approvalsMu   sync.Mutex
      approvals     map[string]chan approvalResp
      autoApprove   atomic.Bool

      // PR-AR-5: timeout
      approvalTTL time.Duration  // copied from cfg in newSession
  }

  type approvalResp struct {
      approved bool
      reason   string
  }
  ```

  Update `newSession` to:
  ```go
  approvals:    make(map[string]chan approvalResp),
  approvalTTL:  2 * time.Minute, // overwritten by handler from cfg
  ```

  > **Initial value injection**: `newSession` doesn't currently take cfg. Easiest: pass approval TTL via a new parameter `approvalTTL time.Duration` or read from `r.deps.Cfg.AgentToolApprovalTimeout` and pass through. Don't over-complicate — add a single parameter to `newSession`.

- [ ] Implement `approval.go`:
  ```go
  package agent

  import (
      "context"
      "fmt"
      "time"

      "github.com/google/uuid"
      "github.com/odysseythink/mlog"
  )

  // RequestApproval blocks until the user approves/rejects the tool call,
  // the request times out, or the session context is cancelled.
  //
  // skillName is the tool name (e.g., "gmail-send"); args is the marshalled
  // tool arguments; description is a human-readable explanation shown to the
  // user. Returns (approved, reason). On non-approval, reason is populated.
  func (s *Session) RequestApproval(ctx context.Context, skillName string, args any, description string) (bool, string) {
      if s.autoApprove.Load() {
          return true, "auto-approved (session toggle)"
      }
      requestID := uuid.NewString()
      ch := make(chan approvalResp, 1)
      s.approvalsMu.Lock()
      s.approvals[requestID] = ch
      s.approvalsMu.Unlock()
      defer func() {
          s.approvalsMu.Lock()
          delete(s.approvals, requestID)
          s.approvalsMu.Unlock()
      }()

      ttl := s.approvalTTL
      if ttl <= 0 { ttl = 2 * time.Minute }
      if err := s.wsConn.Send(ServerFrame{
          Type:        FrameToolApprovalReq,
          RequestID:   requestID,
          SkillName:   skillName,
          Payload:     args,
          Description: description,
          TimeoutMs:   int(ttl / time.Millisecond),
      }); err != nil {
          mlog.Warning("agent: approval request send failed: ", err)
          return false, "approval request could not be delivered"
      }

      select {
      case resp := <-ch:
          return resp.approved, resp.reason
      case <-time.After(ttl):
          return false, fmt.Sprintf("approval timed out after %s", ttl)
      case <-ctx.Done():
          return false, "approval cancelled (context cancelled)"
      case <-s.ctx.Done():
          return false, "approval cancelled (session ended)"
      }
  }

  // handleApprovalResponse routes a toolApprovalResponse client frame to its waiting goroutine.
  // Unknown requestIDs are logged and dropped (idempotent — clients may retry).
  func (s *Session) handleApprovalResponse(requestID string, approved bool) {
      s.approvalsMu.Lock()
      ch, ok := s.approvals[requestID]
      s.approvalsMu.Unlock()
      if !ok {
          mlog.Info("agent: unknown approval requestID: ", requestID)
          return
      }
      reason := "approved by user"
      if !approved { reason = "rejected by user" }
      // Non-blocking send — chan is buffered 1; second response (unlikely) is dropped.
      select {
      case ch <- approvalResp{approved: approved, reason: reason}:
      default:
          mlog.Warning("agent: approval channel full (duplicate response?)")
      }
  }

  // cancelAllApprovals delivers a synthetic rejection to all pending approvals.
  // Used by Abort + Shutdown.
  func (s *Session) cancelAllApprovals(reason string) {
      s.approvalsMu.Lock()
      defer s.approvalsMu.Unlock()
      for _, ch := range s.approvals {
          select {
          case ch <- approvalResp{approved: false, reason: reason}:
          default:
          }
      }
      s.approvals = make(map[string]chan approvalResp)
  }

  // SetAutoApprove toggles per-session auto-approval. Called from reader on `setAutoApprove` client frame.
  func (s *Session) SetAutoApprove(b bool) {
      s.autoApprove.Store(b)
  }
  ```

- [ ] Update `reader.go` switch:
  ```go
  case FrameToolApprovalResp:
      if f.RequestID == "" {
          mlog.Warning("agent: toolApprovalResponse with empty requestId")
          continue
      }
      s.handleApprovalResponse(f.RequestID, f.Approved)
  case FrameSetAutoApprove:
      s.SetAutoApprove(f.Enabled)
      mlog.Info("agent: setAutoApprove → ", f.Enabled)
  ```

- [ ] Hook `cancelAllApprovals` into `Session.Abort`:
  ```go
  func (s *Session) Abort(reason string) {
      if reason != "" {
          _ = s.wsConn.Send(ServerFrame{Type: FrameWSSFailure, Content: reason})
      }
      s.cancelAllApprovals(reason)  // NEW
      s.cancel()
  }
  ```

- [ ] Write 9 tests (use direct `Session` calls via `NewSessionForTesting`, plus reader-loop e2e for the routing tests). Verify all pass.

### Acceptance

- All 9 tests pass
- Concurrent approvals (10 in parallel) route to correct channels by requestID
- Timeout returns within `approvalTTL + 100ms`
- Ctx cancellation propagates to all pending approvals (verified via fan-out test)
- Unknown requestID is logged, no panic, no deadlock
- `SetAutoApprove(true)` short-circuits subsequent `RequestApproval` calls (no WS frame sent — verify with frame counter)

### Commit

`feat(agent): tool-approval flow — Session.RequestApproval + reader routing`

---

## Task 2: Wrap MCP + AgentFlow tools with approval gate in Builder

**Files:**
- `backend/internal/agent/tools/builder.go` (MODIFY — add `addWithApproval` + ApprovalFn type)
- `backend/internal/agent/tools/builder_test.go` (MODIFY — assert wrap presence per source)
- `backend/internal/agent/handler.go` (MODIFY — pass session.RequestApproval into Builder)

**Tests:**
- `TestBuilder_DefaultSkills_NoApprovalWrap` (call Dispatch; assert ApprovalFn NOT invoked)
- `TestBuilder_MCPTools_HaveApprovalWrap` (Dispatch through MCP-projected tool → ApprovalFn invoked)
- `TestBuilder_FlowTools_HaveApprovalWrap`
- `TestBuilder_ApprovalRejects_HandlerNotCalled` (ApprovalFn returns false; inner Handler call counter == 0; result is tool.Error)
- `TestBuilder_GlobalAutoApprove_BypassesGate` (system setting `agent_tool_auto_approve` = "true" → ApprovalFn NOT invoked even for MCP)

### Steps

- [ ] Add `ApprovalFn` typedef + `Approval` field to `BuilderDeps`:
  ```go
  // builder.go
  type ApprovalFn func(ctx context.Context, skillName string, args any, description string) (approved bool, reason string)

  type BuilderDeps struct {
      // ... existing ...
      Approval ApprovalFn  // nil = always approve (test default)
  }
  ```

- [ ] Add a global auto-approve check in `Build`:
  ```go
  globalAutoApprove := settings["agent_tool_auto_approve"] == "true"
  ```
  Pass to `addWithApproval`:

- [ ] Refactor `add` → `addWithApproval`:
  ```go
  func (b *Builder) addWithApproval(reg *tool.Registry, seen map[string]string, e *tool.Entry, source string, requiresApproval bool, globalAutoApprove bool) {
      if requiresApproval && !globalAutoApprove && b.deps.Approval != nil {
          inner := e.Handler
          e.Handler = func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args any
              _ = json.Unmarshal(raw, &args)
              approved, reason := b.deps.Approval(ctx, e.Name, args, e.Description)
              if !approved {
                  return tool.Error("Tool execution rejected: " + reason), nil
              }
              return inner(ctx, raw)
          }
      }
      // existing dedup + register
      if prior, ok := seen[e.Name]; ok && b.deps.EventLog != nil {
          _ = b.deps.EventLog.LogEvent(context.Background(), "agent.tool.override",
              map[string]any{"tool": e.Name, "from": prior, "to": source}, nil)
      }
      seen[e.Name] = source
      reg.Register(e)
  }
  ```

- [ ] Update `Build` callsites:
  ```go
  // Source 1: default skills → false (no approval)
  for _, e := range []*tool.Entry{...} {
      b.addWithApproval(reg, seen, e, "default", false, globalAutoApprove)
  }
  // Source 2: MCP → true
  ... b.addWithApproval(reg, seen, mcpToolToEntry(p, emit), "mcp:"+serverName, true, globalAutoApprove)
  // Source 3: Flow → true
  ... b.addWithApproval(reg, seen, e, "flow:"+f.UUID, true, globalAutoApprove)
  ```

- [ ] Wire in `handler.go`'s `buildSessionRegistry`:
  ```go
  // Modify signature to take a Session-or-nil pointer for approval routing,
  // OR (cleaner): expose Session.RequestApproval through a closure constructed
  // AFTER newSession. Since approval is per-session, we need the session
  // before building the registry — but newSession needs the registry. Circular.

  // RESOLUTION: build registry with a placeholder ApprovalFn first, then
  // patch it after newSession by setting a session-local "approver" field
  // that buildSessionRegistry refers to via closure.

  // Simpler implementation: pass a `*Session` pointer-to-pointer (or a holder)
  // that's populated after newSession. Cleanest is to construct the Builder
  // with a closure that reads `session.RequestApproval` lazily:
  ```

  Actually the cleanest pattern: **build the registry AFTER newSession** and inject via a setter:
  ```go
  // In handler.go HandleWS, reorder:
  // 1. resolve LM
  // 2. construct wsConn
  // 3. construct Session (with empty registry placeholder)
  // 4. build registry with session.RequestApproval as ApprovalFn
  // 5. attach registry to session.pAgent
  ```

  Add a Session method:
  ```go
  // SetRegistry rebuilds the pantheon agent with the new registry.
  // Called by HandleWS once after newSession to break the circular dep.
  func (s *Session) SetRegistry(reg *tool.Registry) {
      s.pAgent = pantheonAgent.New(s.lm,
          pantheonAgent.WithRegistry(reg),
          pantheonAgent.WithMaxSteps(10),
      )
      // re-register @agent participant with new agent
      // ... or we accept that Session is built with empty reg first, then patched
  }
  ```

  > **Cleanest pattern actually**: have `newSession` accept an `approver ApprovalFn` parameter (nilable). The function reads `s.approvals` etc. as a method on `Session`, but the closure passed to Builder takes only `(skillName, args, desc) → (approved, reason)`. We construct the closure in `HandleWS` after `newSession`, then build the registry, then mutate `s.pAgent` via a `SetToolRegistry(reg)` method.
  >
  > Yet simpler: since `RequestApproval` is a `Session` method, the closure is just `session.RequestApproval`. So we can do:
  > ```go
  > sess := newSession(...)
  > reg, err := buildSessionRegistry(..., sess.RequestApproval)  // pass the method directly
  > sess.pAgent = pantheonAgent.New(lm, WithRegistry(reg), WithMaxSteps(10))
  > // re-register the participant with the new pAgent... but pantheon doesn't allow re-registration
  > ```
  > pantheon's `Conversation.RegisterParticipant` is a map insert, so re-registering with the same Name **does** overwrite. So this works:
  > ```go
  > sess := newSession(ctx, uuid, &ws, user, lm, systemPrompt, nil /* placeholder reg */, wc)
  > reg, err := buildSessionRegistry(ctx, deps, &ws, user, lm, settings, emit, sess.RequestApproval)
  > sess.pAgent = pantheonAgent.New(lm, pantheonAgent.WithRegistry(reg), pantheonAgent.WithMaxSteps(10))
  > sess.conv.RegisterParticipant(&conversation.Participant{Name: participantAgent, Role: systemPrompt, Agent: sess.pAgent})
  > ```

- [ ] Implement the wiring above in `handler.go`. The two-step construction is mildly awkward but very local.

- [ ] Verify `buildSessionRegistry` accepts an `ApprovalFn` and passes through to `BuilderDeps.Approval`.

- [ ] Run tests; full suite green.

### Acceptance

- All 5 tests pass
- Default skills NEVER consult ApprovalFn (verified via mock that panics if called)
- MCP + Flow tools ALWAYS consult ApprovalFn (unless globalAutoApprove)
- `agent_tool_auto_approve=true` system setting bypasses the wrap entirely
- Rejection returns `tool.Error("Tool execution rejected: rejected by user")`, agent loop continues
- pantheon agent re-registration with new registry works (no panic, no leaked state)

### Commit

`feat(agent/tools): wrap MCP+Flow tools with per-session approval gate`

---

## Task 3: Total session timeout + CORS hardening + cleanup TODO field

**Files:**
- `backend/internal/config/config.go` (MODIFY — add AgentSessionMaxDuration, AgentAllowedOrigins)
- `backend/internal/agent/cors.go` (NEW)
- `backend/internal/agent/cors_test.go` (NEW)
- `backend/internal/agent/runtime.go` (MODIFY — wire CheckOrigin)
- `backend/internal/agent/handler.go` (MODIFY — wrap ctx with WithTimeout)
- `backend/internal/agent/session.go` (MODIFY — delete `conn *websocket.Conn` TODO field)

**Tests:**
- `TestBuildCheckOrigin_Empty_AllowsSameHost`
- `TestBuildCheckOrigin_Wildcard_AllowsAny` (logs warning)
- `TestBuildCheckOrigin_CSV_MatchesExactOrigin`
- `TestBuildCheckOrigin_NoOriginHeader_Allows` (server-to-server tools without Origin)
- `TestHandler_SessionExceedsMaxDuration_EmitsWSSFailureAndCloses`
- `TestSession_NoTODOConnField_StructDoesNotHaveIt` (compile-time check via type assertion / reflection)

### Steps

- [ ] Add config knobs:
  ```go
  // config.go
  AgentSessionMaxDuration time.Duration `env:"AGENT_SESSION_MAX_DURATION" envDefault:"30m"`
  AgentAllowedOrigins     string        `env:"AGENT_ALLOWED_ORIGINS" envDefault:""` // CSV; "" = same-host; "*" = any
  ```

- [ ] Implement `cors.go`:
  ```go
  package agent

  import (
      "net/http"
      "net/url"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/config"
      "github.com/odysseythink/mlog"
  )

  // buildCheckOrigin returns a CheckOrigin function for the WebSocket upgrader
  // based on the config.AgentAllowedOrigins CSV.
  //
  // Behavior:
  //   - "" (default): allow only when Origin matches Host header (same-host)
  //   - "*": allow any origin (logs a startup warning)
  //   - "https://a.com,https://b.com": exact match against the list
  //   - Missing Origin header: allowed (non-browser clients like curl/CI)
  func buildCheckOrigin(cfg *config.Config) func(*http.Request) bool {
      raw := strings.TrimSpace(cfg.AgentAllowedOrigins)
      if raw == "*" {
          mlog.Warning("agent: AGENT_ALLOWED_ORIGINS=* — allowing any origin. Tighten in production.")
          return func(*http.Request) bool { return true }
      }
      if raw == "" {
          return func(r *http.Request) bool {
              origin := r.Header.Get("Origin")
              if origin == "" { return true }  // non-browser
              u, err := url.Parse(origin)
              if err != nil { return false }
              return u.Host == r.Host
          }
      }
      allowed := make(map[string]bool)
      for _, o := range strings.Split(raw, ",") {
          allowed[strings.TrimSpace(o)] = true
      }
      return func(r *http.Request) bool {
          origin := r.Header.Get("Origin")
          if origin == "" { return true }
          return allowed[origin]
      }
  }
  ```

- [ ] Wire in `NewRuntime`:
  ```go
  upgrader: websocket.Upgrader{
      ReadBufferSize:   4096,
      WriteBufferSize:  4096,
      HandshakeTimeout: 10 * time.Second,
      CheckOrigin:      buildCheckOrigin(d.Cfg),  // was: returning true
  },
  ```

- [ ] Wire total timeout in `handler.go HandleWS`:
  ```go
  // Replace `c.Request.Context()` passed into newSession with a bounded context:
  ttl := r.deps.Cfg.AgentSessionMaxDuration
  if ttl <= 0 { ttl = 30 * time.Minute }
  sessCtx, sessCancel := context.WithTimeout(c.Request.Context(), ttl)
  defer sessCancel()

  // ... build registry ...
  sess := newSession(sessCtx, ...)
  ```

  The existing `<-sess.ctx.Done()` waits already handle timeout — when sessCtx expires, sess.cancel() fires and runLoop exits with ctx.Err() == context.DeadlineExceeded.

  Add a friendly wssFailure path:
  ```go
  runErr := sess.Run(inv.Prompt)
  if runErr != nil && !errors.Is(runErr, context.Canceled) {
      content := runErr.Error()
      if errors.Is(runErr, context.DeadlineExceeded) {
          content = "Session reached maximum duration (" + ttl.String() + "). Ending now."
      }
      _ = wc.Send(ServerFrame{Type: FrameWSSFailure, Content: content})
      return
  }
  ```

- [ ] Delete `conn *websocket.Conn` field from `Session` (it's the PR-AR-1 leftover marked TODO):
  ```go
  type Session struct {
      // ... existing ...
      // (DELETED) conn *websocket.Conn  // PR-AR-1 leftover
  }
  ```
  Verify all references are gone (it shouldn't be — the comment said `// TODO(PR-AR-5): remove — temporary compatibility with PR-AR-1 handler.go`).

- [ ] Run all 6 tests; full suite green.

### Acceptance

- All 6 tests pass
- Server boots without panic; `*` triggers startup warning log
- Session with `AgentSessionMaxDuration=2s` ends within 2s + 500ms and emits `wssFailure` containing "Session reached maximum duration"
- WS dial from origin matching CheckOrigin succeeds; non-matching gets 403 at upgrade
- `Session` struct has no `conn` field

### Commit

`feat(agent): total session timeout + CORS origin allowlist + remove PR-AR-1 leftover`

---

## Task 4: Telemetry events (chat_started/sent/terminated)

**Files:**
- `backend/internal/agent/telemetry.go` (NEW)
- `backend/internal/agent/telemetry_test.go` (NEW)
- `backend/internal/agent/handler.go` (MODIFY — call logChatStarted/Terminated)
- `backend/internal/agent/bridge.go` (MODIFY — call logChatSent in OnMessage)

**Tests:**
- `TestTelemetry_ChatStarted_FiresOnSessionRun`
- `TestTelemetry_ChatSent_FiresOnEachNonUserMessage`
- `TestTelemetry_ChatSent_NotFiredForUserMessages`
- `TestTelemetry_ChatTerminated_FiresOnNormalExit`
- `TestTelemetry_ChatTerminated_FiresOnAbort`
- `TestTelemetry_FireAndForget_DoesNotBlockOnSlowEventLog` (mock eventLog with 5s sleep; chat completes in <500ms)

### Steps

- [ ] Implement `telemetry.go`:
  ```go
  package agent

  import (
      "context"
      "time"

      "github.com/odysseythink/hermind/backend/internal/services"
  )

  func logChatStarted(eventLog *services.EventLogService, userID *int, sessionUUID string, workspaceID int, provider, model string) {
      if eventLog == nil { return }
      go func() {
          ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
          defer cancel()
          _ = eventLog.LogEvent(ctx, "agent_chat_started", map[string]any{
              "session_uuid": sessionUUID,
              "workspace_id": workspaceID,
              "provider":     provider,
              "model":        model,
          }, userID)
      }()
  }

  func logChatSent(eventLog *services.EventLogService, userID *int, sessionUUID string, from, to string) {
      if eventLog == nil { return }
      go func() {
          ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
          defer cancel()
          _ = eventLog.LogEvent(ctx, "agent_chat_sent", map[string]any{
              "session_uuid": sessionUUID,
              "from":         from,
              "to":           to,
          }, userID)
      }()
  }

  func logChatTerminated(eventLog *services.EventLogService, userID *int, sessionUUID string, reason string, duration time.Duration) {
      if eventLog == nil { return }
      go func() {
          ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
          defer cancel()
          _ = eventLog.LogEvent(ctx, "agent_chat_terminated", map[string]any{
              "session_uuid": sessionUUID,
              "reason":       reason,
              "duration_ms":  duration.Milliseconds(),
          }, userID)
      }()
  }
  ```

- [ ] Call `logChatStarted` in `handler.go` right after `r.sessions.Store(...)`:
  ```go
  logChatStarted(r.deps.EventLog, ptrUserID(user), inv.UUID, ws.ID, lm.Provider(), lm.Model())
  ```

- [ ] Call `logChatTerminated` just before the `defer` cleanup completes — easier: at the bottom of HandleWS:
  ```go
  defer func() {
      duration := time.Since(sess.startedAt)
      reason := "normal"
      if runErr != nil {
          if errors.Is(runErr, context.DeadlineExceeded) { reason = "timeout" }
          else if errors.Is(runErr, context.Canceled) { reason = "cancelled" }
          else { reason = "error" }
      }
      logChatTerminated(r.deps.EventLog, ptrUserID(user), inv.UUID, reason, duration)
      r.sessions.Delete(inv.UUID)
      _ = r.CloseInvocation(context.Background(), inv.UUID)
      wc.Close()
  }()
  ```

- [ ] Call `logChatSent` in `bridge.go` OnMessage (only for non-USER messages, which is already the muted path):
  ```go
  s.conv.OnMessage(func(chat conversation.Chat, _ *conversation.Conversation) {
      if s.muteUser && chat.From == participantUser { return }
      _ = s.wsConn.Send(ServerFrame{ /*...existing...*/ })
      // PR-AR-5: telemetry
      logChatSent(s.eventLog, s.UserID, s.UUID, chat.From, chat.To)
  })
  ```
  > Session needs an `eventLog` field. Add it; pass from handler.

- [ ] Add `eventLog` to `Session` struct + `newSession` parameter chain.

- [ ] Write tests using a mock `EventLogService` that records calls. Verify all 6 cases.

### Acceptance

- All 6 tests pass
- `agent_chat_started` fires exactly once per session
- `agent_chat_sent` fires for every non-USER `OnMessage` (typically 1 per LLM reply)
- `agent_chat_terminated` always fires with one of `{normal, timeout, cancelled, error}` reasons
- A 5s-blocked eventLog never delays the session — verified by completing a normal chat in under 500ms
- Nil `eventLog` is safe (no panic, no error)

### Commit

`feat(agent): telemetry events for chat lifecycle (started/sent/terminated)`

---

## Task 5: Shutdown polish + lifecycle e2e

**Files:**
- `backend/internal/agent/runtime.go` (MODIFY — Shutdown drains approvals)
- `backend/internal/agent/lifecycle_e2e_test.go` (NEW)

**Tests:**
- `TestRuntime_Shutdown_CancelsPendingApprovals` (open session waiting on approval → Shutdown → approval returns false within 1s)
- `TestRuntime_Shutdown_MultipleSessions_EachDrained`
- `TestE2E_Approval_Approved_FullPath`
- `TestE2E_Approval_Rejected_AgentContinuesWithToolError`
- `TestE2E_Approval_Timeout_AgentContinuesWithToolError`
- `TestE2E_Approval_SetAutoApproveOnSession_BypassesGate`
- `TestE2E_Approval_GlobalSetting_BypassesGate`
- `TestE2E_BailDuringApproval_CancelsApprovalAndCloses`
- `TestE2E_TotalTimeout_ClosesSession`

### Steps

- [ ] Extend `Runtime.Shutdown`:
  ```go
  func (r *Runtime) Shutdown(ctx context.Context) error {
      r.sessions.Range(func(key, value any) bool {
          if s, ok := value.(*Session); ok {
              s.cancelAllApprovals("server shutting down")  // NEW — wakes pending approvals first
              s.Abort("server shutting down")
          }
          return true
      })
      // ... existing poll loop unchanged ...
  }
  ```

  > **Why drain before Abort**: `Abort` cancels ctx which would also wake `RequestApproval`'s `<-s.ctx.Done()` arm, but the explicit `cancelAllApprovals` gives a clearer reason string in the eventLog. Both work; the explicit version is documentable.

- [ ] Write `lifecycle_e2e_test.go` with the 9 scenarios. Reuse the existing `agentTestEnv` helper (from PR-AR-1/2). Mock LLM provides scripted replies with tool calls.

  Example for `TestE2E_Approval_Approved_FullPath`:
  ```go
  func TestE2E_Approval_Approved_FullPath(t *testing.T) {
      env := newAgentTestEnv(t)
      mock := &mockLanguageModel{
          provider: "openai", model: "gpt-4o-mini",
          // First reply: tool call to an MCP tool requiring approval
          // Second reply: final text after tool result
          replies: []string{
              toolCallReply("mcp-test-tool", `{"q":"hello"}`),
              "All done!",
              "TERMINATE",
          },
      }
      env.Runtime.SetTestLanguageModelOverride(mock)
      // ... seed workspace, an MCP server fixture with "test-tool" ...

      uid, _ := env.Runtime.CreateInvocation(ctx, ws, env.User, nil, "@agent run test-tool")
      tok := env.IssueTempToken(t, env.User.ID, time.Minute)
      conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)

      _ = expectFrame(t, conn, agent.FrameStatusResponse)  // welcome
      req := expectFrame(t, conn, agent.FrameToolApprovalReq)
      require.Equal(t, "mcp-test-tool", req.SkillName)
      require.NotEmpty(t, req.RequestID)

      // User approves
      require.NoError(t, conn.WriteJSON(agent.ClientFrame{
          Type: agent.FrameToolApprovalResp, RequestID: req.RequestID, Approved: true,
      }))

      // Expect: chat message → close on TERMINATE
      chat := expectFrame(t, conn, "")
      require.Equal(t, "All done!", chat.Content)
      _, _, err := conn.ReadMessage()
      require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
  }
  ```

- [ ] Provide a `toolCallReply` helper in test helpers — constructs a pantheon `core.Message` with a `ToolCallPart` (verify exact constructor in `pantheon/core/content.go` if not already done in PR-AR-3).

- [ ] Run all 9 tests. Verify each:
  - Approval-approved path completes
  - Approval-rejected → agent sees `tool.Error("rejected by user")` and continues to TERMINATE
  - Approval-timeout (use 200ms TTL for the test) → same as rejected with reason "approval timed out"
  - Per-session auto-approve: send `{type:"setAutoApprove", enabled:true}` before tool call → no approval frame, tool runs
  - Global auto-approve: set `agent_tool_auto_approve=true` system setting → no approval frame
  - Bail during approval: send `exit` while approval pending → approval channel cancelled, conn closes
  - Total timeout: set `AgentSessionMaxDuration=2s`, LLM stalls → wssFailure "Session reached maximum duration"

- [ ] Full suite green; race detector clean.

### Acceptance

- All 9 e2e tests pass
- `-race` clean (concurrent approvals + cancels + reader is the densest concurrency surface in v1)
- Shutdown of 5 concurrent sessions with 3 of them mid-approval completes in <2s
- No goroutine leaks (sample before/after: `runtime.NumGoroutine()` returns ≤ baseline+2 after `time.Sleep(50ms)`)

### Commit

`feat(agent): Shutdown drains approvals + comprehensive lifecycle e2e`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` 100% green
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] `internal/agent/doc.go` updated — note v1 main-line is complete; PR-AR-6 is sql/fs/createFiles skills, future PRs handle whitelist + OAuth providers
- [ ] All four `TODO(PR-AR-5)` markers in source are resolved (verify with `grep -rn "PR-AR-5" backend/`; expected count: 0 after this PR)
- [ ] `.gpowers/decisions/2026-05-27-tool-approval-global-toggle-only.md` written explaining why we shipped only global+session toggles (not per-user whitelist)
- [ ] `.gpowers/decisions/2026-05-27-agent-session-30min-cap.md` written explaining the 30-minute default
- [ ] Manual smoke with a real OPEN_AI_KEY and a real MCP server (e.g., `npx -y @modelcontextprotocol/server-everything`) — verify approval prompt + approve + tool runs
- [ ] FE companion PR brief (from PR-AR-4) referenced — confirm WS query token still works after CheckOrigin change
- [ ] No new TODOs without ticket reference

## Risk notes

| Risk | Mitigation |
|---|---|
| Approval gate adds latency to MCP tool execution (network round-trip with user) | Acceptable — the gate is the user's explicit choice. Both auto-approve toggles bypass entirely |
| Per-session SetAutoApprove state lost on reconnect | Session lifetime = WS lifetime by design; reconnect = new session, settings reset. Document |
| Two-step Session construction (newSession → buildRegistry → SetRegistry-equivalent) leaks intermediate state | The intermediate Session uses `nil` registry which pantheon agent tolerates (empty tools list); brief window <100ms |
| `cancelAllApprovals` racing with a just-arrived approval response | Both are mutex-guarded; channel buffered 1 so an extra response is dropped silently. Verified by `-race` |
| Telemetry goroutines on a crashed EventLogService → fanout of zombie goroutines | Each has 2s ctx; eventually GC'd. Risk acceptable for v1; revisit if logged event volume gets large |
| `AGENT_ALLOWED_ORIGINS=*` left enabled in production | Boot-time warning log; document in `.env.example` |
| Approval timeout shorter than session timeout (2m < 30m default) — could "leak" approvals on a paused-tab user | Channel is closed on session end; approval just returns reason "session ended" — fine |
| `Shutdown` cancels approvals before tools complete, but pantheon agent loop still sees `tool.Error` — does it retry? | pantheon `Agent.Run`: if a tool returns an error, the agent receives it as a `ToolResultPart` with `IsError: true` and may decide to retry or give up. Mock tests verify the loop unwinds within maxSteps (10) regardless |
| Re-registering `@agent` Participant in pantheon Conversation could leak old Agent | Verified by source read: `RegisterParticipant` is a map insert; old `*pantheonAgent.Agent` becomes unreachable, GC'd. No leak |

## Estimate

| Task | Hours |
|---|---|
| 1. Session approval registry + reader wiring + timeout | 2.0 |
| 2. Builder approval wrap for MCP+Flow | 1.5 |
| 3. Total timeout + CORS hardening + cleanup TODO | 1.5 |
| 4. Telemetry events | 1.0 |
| 5. Shutdown polish + lifecycle e2e (9 tests) | 2.0 |
| **Total** | **8.0** (design estimate 6-8h, top of range ✓) |

—— end of plan
