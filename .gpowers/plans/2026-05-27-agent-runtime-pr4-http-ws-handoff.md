# Agent Runtime PR-AR-4 — HTTP → WS Handoff + Temp Token Issuance + E2E Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the `chat_service.Stream` SSE flow to detect `@agent`/`automatic-with-native-tools` triggers, create a `WorkspaceAgentInvocation` row, mint a 3-minute temp token, and emit the Node-compatible `agentInitWebsocketConnection` chunk + closing `statusResponse` chunk so the frontend can dial the WebSocket at `/api/agent-invocation/:uuid?token=…`. Includes the necessary tiny frontend change (one new field consumed) — documented but not implemented; that's a separate FE PR. After PR-AR-4, a real chat round-trip from POST /workspace/:slug/stream-chat through to a live `@agent` WS session is end-to-end testable.

**Architecture:**
- A new `services.AgentInvoker` interface lives in `internal/services/agent_invoker.go`; `agent.Runtime` implements it. ChatService takes `AgentInvoker` (interface, nilable) — keeps `services` ↔ `agent` decoupled and avoids forcing all chat callers to pull in the WS stack.
- `Runtime.PrepareInvocationHandoff(ctx, ws, user, thread, prompt)` does **both** `CreateInvocation` (PR-AR-1) and `tempTokenSvc.IssueWithTTL(3min)` (PR-AR-1) atomically and returns a `Handoff{UUID, WSToken}`. If token issuance fails, the invocation row is deleted (rollback) so we never orphan a "ready" UUID that can't be authenticated against.
- `Runtime.IsAgentInvocation(ctx, ws, message)` replicates Node `WorkspaceAgentInvocation.parseAgents` + `chatMode=='automatic' && supportsNativeToolCalling`. Native-tool-calling is a static whitelist (pantheon doesn't expose this capability flag).
- `chat_service.Stream` early-returns with two SSE chunks BEFORE running RAG retrieval — saves cost when the agent path takes over.
- `main.go` construction order: `agentRuntime` first → `chatSvc` second (with `agentRuntime` passed in). No circular deps.

**Tech Stack:** Go 1.25.5, Gin v1.10, GORM. **No new dependencies.**

**Source spec:** `.gpowers/designs/2026-05-26-agent-runtime-design.md` §6, §9, §14 (PR-AR-4 row).

**Reference Node implementation:**
- `server/utils/chats/agents.js` `grepAgents()` — the canonical handoff logic (DB write + 2 SSE chunks)
- `server/utils/chats/stream.js:42` — `// If is agent enabled chat we will exit this flow early` (this is the integration point)
- `server/models/workspaceAgentInvocation.js` `parseAgents` (`@agent` prefix detection)
- `server/models/workspace.js` `supportsNativeToolCalling` — the agentProvider→capability flag check
- `frontend/src/utils/chat/index.js:140` — current FE consumer of `agentInitWebsocketConnection`
- `frontend/src/components/WorkspaceChat/ChatContainer/index.jsx:281` — current WS URL builder (must be touched in the FE-side companion PR)

---

## Pre-task: Read this section once before starting

### What landed in PR-AR-1 / 2 / 3 (use, don't re-implement)

- `agent.Runtime.CreateInvocation(ctx, ws, user, thread, prompt) (uuid, err)` — PR-AR-1
- `agent.Runtime.GetInvocation(ctx, uuid)` — PR-AR-1
- `agent.Runtime.CloseInvocation(ctx, uuid)` — PR-AR-1
- `services.TemporaryAuthTokenService.IssueWithTTL(ctx, userID, ttl)` — PR-AR-1
- `middleware.WSValidatedRequest(authSvc, tempTokenSvc)` — PR-AR-1 (single-use query token)
- `agent.Runtime.HandleWS` — PR-AR-2/3 (consumes the temp token via middleware before reaching the handler)
- `models.WorkspaceAgentInvocation` — PR-AR-1 (we'll need a `Delete(uuid)` method this PR for rollback)
- `dto.StreamChatResponse` — PR-AR-1 added no fields; PR-AR-2/3 didn't touch it; **this PR extends it**

### Current `chat_service.Stream` shape (verified)

```go
func (s *ChatService) Stream(ctx, ws, user, threadID, req) (<-chan dto.StreamChatResponse, error) {
    msgID := uuid.New().String()
    out := make(chan dto.StreamChatResponse, 16)
    go func() {
        defer close(out)
        // 1. buildRAGContext  (expensive — vector DB + embedder)
        // 2. assemble messages
        // 3. stream via Pantheon LLM
        // 4. emit chunks
    }()
    return out, nil
}
```

**PR-AR-4 inserts the agent-handoff branch _before_ buildRAGContext** so we don't pay the RAG cost for the agent path. The agent's own `rag-memory` skill will do RAG on demand (and only when the LLM asks for it via tool call).

### Current `dto.StreamChatResponse` (verified)

```go
type StreamChatResponse struct {
    UUID         string  `json:"uuid"`
    Type         string  `json:"type"`
    TextResponse *string `json:"textResponse,omitempty"`
    Sources      []any   `json:"sources,omitempty"`
    Close        bool    `json:"close"`
    Error        *string `json:"error,omitempty"`
}
```

**This PR adds three fields** (`WebsocketUUID`, `WebsocketToken`, `Animate`) with `omitempty` so existing tests don't see them in their snapshots.

### Frontend integration — one-line change (NOT in this PR)

Currently `frontend/src/components/WorkspaceChat/ChatContainer/index.jsx:281` builds:
```js
new WebSocket(`${websocketURI()}/api/agent-invocation/${socketId}`)
```

After PR-AR-4 lands, the FE companion PR must:
1. Store `chatResult.websocketToken` alongside `socketId` in `frontend/src/utils/chat/index.js:140` (currently `setWebsocket(chatResult.websocketUUID)`)
2. Append `?token=<token>` to the WS URL in `ChatContainer/index.jsx:281`

That's it. **Total FE diff: ~4 lines.** This delta is required because browser WebSocket API doesn't support `Authorization` headers — we _must_ tunnel auth through query string or subprotocol.

> **Decision artefact**: write `.gpowers/decisions/2026-05-27-ws-query-token.md` summarising: chose query token over `Sec-WebSocket-Protocol` to keep WSValidatedRequest from PR-AR-1 unchanged; query is logged but the token is single-use + 3min TTL so log exposure window is tiny.

### Node `supportsNativeToolCalling` → Go static whitelist

Pantheon does not expose a `SupportsNativeToolCalling()` capability flag. We hardcode a whitelist matching the providers that pantheon's openaicompat backend tool-calling has been validated against. Initial set (mirrors Node's behaviour):

```go
var providersWithNativeToolCalling = map[string]bool{
    "openai":    true,
    "anthropic": true,
    "groq":      true,
    "ollama":    true,
    "mistral":   true,
    "google":    true,  // gemini
    "deepseek":  true,
    "openrouter": true,
}
```

Other providers fall back to `@agent`-prefix-only invocation. Document in `.gpowers/decisions/2026-05-27-native-tool-calling-whitelist.md`.

### New surface (this PR)

```
backend/internal/agent/
├── handoff.go                  # NEW — Handoff struct, PrepareInvocationHandoff, IsAgentInvocation
├── handoff_test.go             # NEW
├── native_tool_calling.go      # NEW — supportsNativeToolCalling map + helper
├── native_tool_calling_test.go # NEW
└── invocation.go               # MODIFY — add DeleteInvocation(uuid) for rollback path

backend/internal/services/
├── agent_invoker.go            # NEW — interface AgentInvoker
├── chat_service.go             # MODIFY — accept AgentInvoker dep; early-return branch
├── chat_service_test.go        # MODIFY — handoff scenarios

backend/internal/dto/
└── chat.go                     # MODIFY — StreamChatResponse gains 3 fields

backend/cmd/server/main.go    # MODIFY — reorder agentRuntime ↑ chatSvc; pass invoker
```

### Methods to ship (PR-AR-4 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `agent.Handoff` | struct `{UUID string; WSToken string}` | returned from PrepareInvocationHandoff |
| 2 | `agent.Runtime` | `IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error)` | Pure logic + reads workspace config |
| 3 | `agent.Runtime` | `PrepareInvocationHandoff(ctx, ws, user, thread, prompt) (*Handoff, error)` | Create row + issue token; rollback row on token err |
| 4 | `agent.Runtime` | `DeleteInvocation(ctx, uuid string) error` | Used by Prepare's rollback path; safe to expose |
| 5 | `agent` (unexported) | `supportsNativeToolCalling(provider string) bool` | static map lookup |
| 6 | `agent` (unexported) | `parseAgentHandles(message string) []string` | Replicate Node's `parseAgents` — `@agent` prefix detection |
| 7 | `services.AgentInvoker` | `interface { IsAgentInvocation(...); PrepareInvocationHandoff(...) }` | Two-method interface; `agent.Runtime` satisfies |
| 8 | `services.ChatService` | gains `AgentInvoker services.AgentInvoker` field; constructor accepts | Nilable — old tests pass `nil` |
| 9 | `services.ChatService.Stream` | early-detect branch | Emits 2 chunks + close |
| 10 | `dto.StreamChatResponse` | gain `WebsocketUUID *string`, `WebsocketToken *string`, `Animate bool` | All `json:",omitempty"` |

### SSE chunk shapes (Node parity + new token field)

```jsonc
// Chunk 1
{
  "uuid": "<msgID>",
  "type": "agentInitWebsocketConnection",
  "close": false,
  "websocketUUID": "<invocation uuid>",
  "websocketToken": "allm-tat-<...>"      // NEW — required for Go-side single-use query auth
}

// Chunk 2 (immediately after)
{
  "uuid": "<msgID>",
  "type": "statusResponse",
  "textResponse": "@agent: Swapping over to agent chat. Type /exit to exit agent execution loop early.",
  "close": true,
  "animate": true
}
```

### Out of scope (explicit)

- Frontend changes — companion PR (FE-side, ~4 lines)
- Telemetry events (`agent_chat_started`) — PR-AR-5
- Workspace-thread context propagation refinement (existing thread routing is good enough; just pass through)
- `EphemeralAgentHandler` (API-key path agent invocation) — Phase 2
- Multi-agent dispatch (`@@channel`, custom handle parsing) — out; only `@agent` prefix is honoured
- Per-user agent permission gates (`AgentSkillWhitelist`) — PR-AR-5
- Attachment forwarding through to invocation (Node has `cacheInvocationAttachments`) — out for v1; attachment field reserved but unused

### TDD discipline

Each task lands as **one commit**. Failing test → impl → green → full suite green → commit.

---

## Task 1: parseAgentHandles + supportsNativeToolCalling + IsAgentInvocation

**Files:**
- `backend/internal/agent/native_tool_calling.go` (NEW)
- `backend/internal/agent/native_tool_calling_test.go` (NEW)
- `backend/internal/agent/handoff.go` (NEW)
- `backend/internal/agent/handoff_test.go` (NEW)

**Tests:**
- `TestParseAgentHandles_AtAgentPrefix_Detected`
- `TestParseAgentHandles_NoPrefix_Empty`
- `TestParseAgentHandles_MidSentenceAt_Ignored` (only leading `@agent` counts, per Node)
- `TestParseAgentHandles_LeadingWhitespace_Tolerated`
- `TestSupportsNativeToolCalling_KnownProvider`
- `TestSupportsNativeToolCalling_UnknownProvider_False`
- `TestSupportsNativeToolCalling_CaseInsensitive`
- `TestIsAgentInvocation_AtAgentMessage_True`
- `TestIsAgentInvocation_AutomaticModeNativeProvider_True`
- `TestIsAgentInvocation_AutomaticModeUnknownProvider_False`
- `TestIsAgentInvocation_ChatModeNoPrefix_False`
- `TestIsAgentInvocation_NilWorkspace_False`

### Steps

- [ ] Write failing `native_tool_calling_test.go`:
  ```go
  func TestSupportsNativeToolCalling_KnownProvider(t *testing.T) {
      require.True(t, agent.SupportsNativeToolCallingForTesting("openai"))
      require.True(t, agent.SupportsNativeToolCallingForTesting("anthropic"))
      require.True(t, agent.SupportsNativeToolCallingForTesting("ollama"))
  }
  func TestSupportsNativeToolCalling_CaseInsensitive(t *testing.T) {
      require.True(t, agent.SupportsNativeToolCallingForTesting("OpenAI"))
      require.True(t, agent.SupportsNativeToolCallingForTesting("OLLAMA"))
  }
  ```

- [ ] Implement `native_tool_calling.go`:
  ```go
  package agent

  import "strings"

  var providersWithNativeToolCalling = map[string]bool{
      "openai":     true,
      "anthropic":  true,
      "groq":       true,
      "ollama":     true,
      "mistral":    true,
      "google":     true,
      "deepseek":   true,
      "openrouter": true,
  }

  func supportsNativeToolCalling(provider string) bool {
      return providersWithNativeToolCalling[strings.ToLower(strings.TrimSpace(provider))]
  }

  // SupportsNativeToolCallingForTesting is the unexported predicate exposed for tests.
  func SupportsNativeToolCallingForTesting(provider string) bool {
      return supportsNativeToolCalling(provider)
  }
  ```

- [ ] Write failing `handoff_test.go` for parseAgentHandles + IsAgentInvocation:
  ```go
  func TestParseAgentHandles_AtAgentPrefix_Detected(t *testing.T) {
      require.True(t, len(agent.ParseAgentHandlesForTesting("@agent help me")) > 0)
  }
  func TestParseAgentHandles_MidSentenceAt_Ignored(t *testing.T) {
      require.Empty(t, agent.ParseAgentHandlesForTesting("hi @agent in the middle"))
  }
  func TestIsAgentInvocation_AutomaticModeNativeProvider_True(t *testing.T) {
      env := newAgentTestEnv(t)
      ws := &models.Workspace{
          ChatMode:      utils.Ptr("automatic"),
          AgentProvider: utils.Ptr("openai"),
          AgentModel:    utils.Ptr("gpt-4o-mini"),
      }
      got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "what time is it?")
      require.NoError(t, err)
      require.True(t, got)
  }
  ```

- [ ] Implement `handoff.go` (Part 1 — parsing + decision):
  ```go
  package agent

  import (
      "context"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/models"
  )

  // ParseAgentHandlesForTesting wraps parseAgentHandles for tests.
  func ParseAgentHandlesForTesting(s string) []string { return parseAgentHandles(s) }

  func parseAgentHandles(message string) []string {
      msg := strings.TrimLeft(message, " \t\n\r")
      if !strings.HasPrefix(msg, "@agent") { return nil }
      out := []string{}
      for _, tok := range strings.Fields(msg) {
          if strings.HasPrefix(tok, "@") { out = append(out, tok) }
      }
      return out
  }

  // IsAgentInvocation returns true when:
  //   1. message starts with "@agent", OR
  //   2. workspace.chatMode == "automatic" AND the workspace's agent provider supports native tool calling.
  func (r *Runtime) IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error) {
      if ws == nil { return false, nil }
      if len(parseAgentHandles(message)) > 0 { return true, nil }
      mode := ""
      if ws.ChatMode != nil { mode = *ws.ChatMode }
      if mode != "automatic" { return false, nil }

      provider := ""
      switch {
      case ws.AgentProvider != nil && *ws.AgentProvider != "":
          provider = *ws.AgentProvider
      case ws.ChatProvider != nil && *ws.ChatProvider != "":
          provider = *ws.ChatProvider
      default:
          provider = r.deps.Cfg.LLMProvider
      }
      return supportsNativeToolCalling(provider), nil
  }
  ```

- [ ] Run all 12 tests; verify pass. Full suite green.

### Acceptance

- All 12 tests pass
- `parseAgentHandles("hi @agent")` returns empty (mid-sentence)
- `IsAgentInvocation` returns `false` for nil workspace, no panic
- Automatic-mode + openai → true; automatic-mode + xai (not in whitelist) → false

### Commit

`feat(agent): parseAgentHandles + supportsNativeToolCalling whitelist + IsAgentInvocation`

---

## Task 2: PrepareInvocationHandoff with rollback + AgentInvoker interface

**Files:**
- `backend/internal/agent/handoff.go` (MODIFY — add Handoff + PrepareInvocationHandoff)
- `backend/internal/agent/handoff_test.go` (MODIFY)
- `backend/internal/agent/invocation.go` (MODIFY — add DeleteInvocation)
- `backend/internal/agent/invocation_test.go` (MODIFY — add delete test)
- `backend/internal/services/agent_invoker.go` (NEW)
- `backend/internal/services/agent_invoker_test.go` (NEW — verify agent.Runtime satisfies)

**Tests:**
- `TestRuntime_DeleteInvocation_RemovesRow`
- `TestRuntime_DeleteInvocation_IdempotentOnMissing`
- `TestHandoff_HappyPath_ReturnsUUIDAndToken`
- `TestHandoff_NoUser_ReturnsErrorWithoutCreatingRow`
- `TestHandoff_TempTokenFailure_RollsBackInvocation` (mock tempTokenSvc to return error; verify no row left)
- `TestAgentInvokerInterface_RuntimeSatisfies` (compile-time check `var _ services.AgentInvoker = (*agent.Runtime)(nil)`)

### Steps

- [ ] Add `DeleteInvocation` to `invocation.go`:
  ```go
  func (r *Runtime) DeleteInvocation(ctx context.Context, id string) error {
      // Idempotent: no error if row missing.
      return r.deps.DB.WithContext(ctx).
          Where("uuid = ?", id).
          Delete(&models.WorkspaceAgentInvocation{}).Error
  }
  ```

- [ ] Write failing `Handoff` tests:
  ```go
  func TestHandoff_TempTokenFailure_RollsBackInvocation(t *testing.T) {
      env := newAgentTestEnvWithFailingTokenSvc(t)  // helper that injects a stub returning error
      ws := seedWorkspace(t, env.DB)
      _, err := env.Runtime.PrepareInvocationHandoff(context.Background(), ws, env.User, nil, "@agent hi")
      require.Error(t, err)

      // Verify NO orphan invocation row
      var count int64
      env.DB.Model(&models.WorkspaceAgentInvocation{}).Count(&count)
      require.Equal(t, int64(0), count)
  }
  ```

- [ ] Extend `handoff.go`:
  ```go
  package agent

  type Handoff struct {
      UUID    string
      WSToken string
  }

  func (r *Runtime) PrepareInvocationHandoff(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (*Handoff, error) {
      if user == nil || user.ID == 0 {
          // Auth-disabled mode: we issue the sentinel from PR-AR-1's middleware contract.
          // CreateInvocation needs a workspace, not a user.
          uid, err := r.CreateInvocation(ctx, ws, nil, thread, prompt)
          if err != nil { return nil, err }
          return &Handoff{UUID: uid, WSToken: middleware.AuthDisabledBypassToken}, nil
      }
      uid, err := r.CreateInvocation(ctx, ws, user, thread, prompt)
      if err != nil { return nil, fmt.Errorf("create invocation: %w", err) }

      tok, err := r.deps.TempTokenSvc.IssueWithTTL(ctx, user.ID, 3*time.Minute)
      if err != nil {
          _ = r.DeleteInvocation(ctx, uid)  // rollback
          return nil, fmt.Errorf("issue WS token: %w", err)
      }
      return &Handoff{UUID: uid, WSToken: tok}, nil
  }
  ```

  > **Auth-disabled mode**: in single-user mode, user.ID is 0 (admin bypass user). We hand back the literal `AUTH_DISABLED_BYPASS` sentinel from PR-AR-1; `WSValidatedRequest` accepts it.

- [ ] Define `services.AgentInvoker`:
  ```go
  // internal/services/agent_invoker.go
  package services

  import (
      "context"

      "github.com/odysseythink/hermind/backend/internal/models"
  )

  // AgentInvoker is the narrow interface ChatService uses to handle @agent triggers.
  // Implemented by agent.Runtime. Kept in services/ to avoid making chat_service.go
  // import the agent package directly.
  type AgentInvoker interface {
      IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error)
      PrepareInvocationHandoff(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (*AgentHandoff, error)
  }

  // AgentHandoff mirrors agent.Handoff to avoid the package import.
  type AgentHandoff struct {
      UUID    string
      WSToken string
  }
  ```

- [ ] Add an adapter on `agent.Runtime` so it satisfies `services.AgentInvoker`:
  ```go
  // internal/agent/handoff.go
  func (r *Runtime) PrepareInvocationHandoffAsService(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (*services.AgentHandoff, error) {
      h, err := r.PrepareInvocationHandoff(ctx, ws, user, thread, prompt)
      if err != nil { return nil, err }
      return &services.AgentHandoff{UUID: h.UUID, WSToken: h.WSToken}, nil
  }
  ```

  > **Simpler alternative**: rename the existing `PrepareInvocationHandoff` to return `*services.AgentHandoff` directly. Picks: cleaner public API but couples `agent` to `services`. Both packages are infrastructure; the coupling already exists indirectly via `Deps`. Go with the simpler alternative.

  Final shape:
  ```go
  func (r *Runtime) PrepareInvocationHandoff(...) (*services.AgentHandoff, error)
  ```
  Delete the adapter. Update Handoff struct in agent package to just be an alias if used internally, or remove entirely.

- [ ] Add the compile-time interface assertion:
  ```go
  // internal/services/agent_invoker_test.go
  package services_test

  import (
      "github.com/odysseythink/hermind/backend/internal/agent"
      "github.com/odysseythink/hermind/backend/internal/services"
  )

  var _ services.AgentInvoker = (*agent.Runtime)(nil)
  ```

- [ ] Run all 6 tests; full suite green.

### Acceptance

- `Handoff` returned with valid UUID + non-empty token in happy path
- TempToken issuance failure → no orphan invocation row (verify count == 0)
- Auth-disabled path returns sentinel token
- `agent.Runtime` satisfies `services.AgentInvoker` at compile time
- `DeleteInvocation` idempotent

### Commit

`feat(agent): PrepareInvocationHandoff with rollback + AgentInvoker interface`

---

## Task 3: dto.StreamChatResponse extension + ChatService early-detect branch

**Files:**
- `backend/internal/dto/chat.go` (MODIFY)
- `backend/internal/services/chat_service.go` (MODIFY)
- `backend/internal/services/chat_service_test.go` (MODIFY)

**Tests:**
- `TestStreamChatResponse_OmitEmptyDoesNotEmitWSFields` (negative — non-agent flow shouldn't have these fields)
- `TestChatService_Stream_NoAgentInvoker_FallsThrough` (nil AgentInvoker → existing path runs)
- `TestChatService_Stream_NotAgentMessage_FallsThrough` (regular chat, no @agent)
- `TestChatService_Stream_AgentMessage_EmitsTwoChunks` (sends `@agent`, asserts agentInitWebsocketConnection + statusResponse close)
- `TestChatService_Stream_AgentMessage_HandoffError_EmitsAbort`
- `TestChatService_Stream_AgentMessage_SkipsRAG` (mock vectorSvc — assert NOT called)

### Steps

- [ ] Update `dto/chat.go`:
  ```go
  type StreamChatResponse struct {
      UUID           string  `json:"uuid"`
      Type           string  `json:"type"`
      TextResponse   *string `json:"textResponse,omitempty"`
      Sources        []any   `json:"sources,omitempty"`
      Close          bool    `json:"close"`
      Error          *string `json:"error,omitempty"`
      // NEW (PR-AR-4)
      WebsocketUUID  *string `json:"websocketUUID,omitempty"`
      WebsocketToken *string `json:"websocketToken,omitempty"`
      Animate        bool    `json:"animate,omitempty"`
  }
  ```

  > **Animate** is a Node-parity field used by the FE to drive typing animations on `statusResponse` chunks. Confirmed in `frontend/src/utils/chat/index.js` and the `statusResponse` handler. The `omitempty` makes it absent for non-statusResponse chunks (Go's `bool` `omitempty` = absent when false ✓).

- [ ] Update `ChatService` struct + constructor:
  ```go
  type ChatService struct {
      // ... existing fields ...
      agentInvoker AgentInvoker
  }

  func NewChatService(db *gorm.DB, cfg *config.Config, vectorSvc *VectorService, llmProv providers.LLMProvider, emb embedder.Embedder, agentInvoker AgentInvoker) *ChatService {
      return &ChatService{ /*...*/ agentInvoker: agentInvoker }
  }
  ```

- [ ] Insert early-detect branch at the **top** of `Stream`'s goroutine, **before** `buildRAGContext`:
  ```go
  go func() {
      defer close(out)

      // PR-AR-4: @agent handoff to WebSocket runtime
      if s.agentInvoker != nil {
          invoked, err := s.agentInvoker.IsAgentInvocation(ctx, ws, req.Message)
          if err != nil {
              mlog.Warning("ChatService.Stream: IsAgentInvocation error: ", err)
              // fall through to non-agent path
          } else if invoked {
              var thread *models.WorkspaceThread
              if threadID != nil {
                  thread = &models.WorkspaceThread{ID: *threadID}
              }
              ho, err := s.agentInvoker.PrepareInvocationHandoff(ctx, ws, user, thread, req.Message)
              if err != nil {
                  mlog.Error("ChatService.Stream: PrepareInvocationHandoff failed: ", err)
                  out <- dto.StreamChatResponse{
                      UUID: msgID, Type: "abort", Close: true,
                      Error: utils.Ptr("agent invocation could not be prepared: " + err.Error()),
                  }
                  return
              }
              out <- dto.StreamChatResponse{
                  UUID:           msgID,
                  Type:           "agentInitWebsocketConnection",
                  WebsocketUUID:  &ho.UUID,
                  WebsocketToken: &ho.WSToken,
                  Close:          false,
              }
              out <- dto.StreamChatResponse{
                  UUID:         msgID,
                  Type:         "statusResponse",
                  TextResponse: utils.Ptr("@agent: Swapping over to agent chat. Type /exit to exit agent execution loop early."),
                  Close:        true,
                  Animate:      true,
              }
              return  // do NOT run RAG / LLM stream
          }
      }

      // (existing non-agent flow continues unchanged below)
      var fullText strings.Builder
      systemPrompt, sources, history, err := s.buildRAGContext(...)
      // ...
  }()
  ```

- [ ] Write failing tests in `chat_service_test.go`. Use a mock `AgentInvoker`:
  ```go
  type mockAgentInvoker struct {
      isAgentRet      bool
      isAgentErr      error
      handoffRet      *services.AgentHandoff
      handoffErr      error
      isAgentCalls    int
      handoffCalls    int
  }
  func (m *mockAgentInvoker) IsAgentInvocation(...) (bool, error) {
      m.isAgentCalls++
      return m.isAgentRet, m.isAgentErr
  }
  func (m *mockAgentInvoker) PrepareInvocationHandoff(...) (*services.AgentHandoff, error) {
      m.handoffCalls++
      return m.handoffRet, m.handoffErr
  }

  func TestChatService_Stream_AgentMessage_EmitsTwoChunks(t *testing.T) {
      svc := newChatServiceWithMockInvoker(t, &mockAgentInvoker{
          isAgentRet: true,
          handoffRet: &services.AgentHandoff{UUID: "uid-1", WSToken: "tok-1"},
      })
      ch, err := svc.Stream(ctx, ws, user, nil, dto.StreamChatRequest{Message: "@agent hi"})
      require.NoError(t, err)

      c1 := <-ch
      require.Equal(t, "agentInitWebsocketConnection", c1.Type)
      require.Equal(t, "uid-1", *c1.WebsocketUUID)
      require.Equal(t, "tok-1", *c1.WebsocketToken)
      require.False(t, c1.Close)

      c2 := <-ch
      require.Equal(t, "statusResponse", c2.Type)
      require.Contains(t, *c2.TextResponse, "Swapping over")
      require.True(t, c2.Close)
      require.True(t, c2.Animate)

      _, more := <-ch
      require.False(t, more, "channel must be closed after handoff")
  }
  ```

- [ ] Run all 6 tests; full suite green.

### Acceptance

- All 6 tests pass
- Existing chat_service tests still pass with `nil` AgentInvoker (compat)
- The non-agent flow doesn't emit `websocketUUID`/`websocketToken`/`animate:true` (omitempty verification)
- Handoff failure path emits abort chunk with Error populated
- RAG path is **NOT** entered on agent invocation (assert via mocked vectorSvc counter)

### Commit

`feat(chat): @agent SSE handoff — emit init+status chunks, skip RAG`

---

## Task 4: main.go wiring (reorder construction) + decision artefacts

**Files:**
- `backend/cmd/server/main.go` (MODIFY)
- `.gpowers/decisions/2026-05-27-ws-query-token.md` (NEW)
- `.gpowers/decisions/2026-05-27-native-tool-calling-whitelist.md` (NEW)

**Tests:** none new (main.go startup is covered by existing smoke + boot-checks).

### Steps

- [ ] **Reorder construction in `main.go`** — `agentRuntime` must be built before `chatSvc` because chatSvc now takes it as a dep:
  ```go
  // BEFORE PR-AR-4 order:
  //   line 107  chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv, emb)
  //   line 148  agentRuntime := agent.NewRuntime(agent.Deps{...})

  // AFTER PR-AR-4 order:
  agentRuntime := agent.NewRuntime(agent.Deps{
      DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
      SysSvc: sysSvc, VectorSearchSvc: vectorSearchSvc, DocSvc: docSvc,
      MCPHv: mcpHv, FlowSvc: agentFlowSvc, EventLog: eventLogSvc,
  })
  chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv, emb, agentRuntime)
  // ^ agentRuntime satisfies services.AgentInvoker
  ```

- [ ] Verify the rest of the boot order (no service downstream of chatSvc depends on agentRuntime; the only consumer is chatSvc itself).

- [ ] Manual smoke procedure (document in main.go comment block; do not automate):
  ```bash
  # Terminal A
  cd backend && go run ./cmd/server

  # Terminal B
  TOKEN=...  # session JWT (or AUTH_DISABLED_BYPASS if auth off)
  curl -N -X POST http://localhost:3001/api/workspace/<slug>/stream-chat \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"message":"@agent what time is it?"}'

  # expect two SSE chunks (agentInitWebsocketConnection then statusResponse close),
  # then connection closes. Capture websocketUUID and websocketToken from chunk 1.

  websocat "ws://localhost:3001/api/agent-invocation/<UUID>?token=<TOK>"
  # expect welcome statusResponse frame, then agent reply chat frame.
  ```

- [ ] Write `.gpowers/decisions/2026-05-27-ws-query-token.md`:
  ```markdown
  # WS Auth via Query Token (PR-AR-4)

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: Browser WebSocket API can't send Authorization headers; need to ferry auth across upgrade.

  ## Options
  - **Query string `?token=`**: ✓ adopted. Reuses PR-AR-1 WSValidatedRequest as-is.
  - `Sec-WebSocket-Protocol` subprotocol: rejected — adds middleware complexity; not all proxies preserve the header.
  - Cookie-based: rejected — would require fetching CSRF+session cookies from FE; mismatched with our Bearer-JWT auth design.

  ## Risk
  Token logged in proxy access logs. Mitigation: 3-minute TTL + single-use (deleted on Validate). Window is ~5 seconds in practice (FE dials immediately after SSE chunk).
  ```

- [ ] Write `.gpowers/decisions/2026-05-27-native-tool-calling-whitelist.md`:
  ```markdown
  # Static Native-Tool-Calling Provider Whitelist (PR-AR-4)

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: Pantheon v0.0.9 doesn't expose a capability flag for "supports native tool calling". Node uses a per-provider class check; we mirror that as a Go-side static map.

  ## Whitelist
  openai, anthropic, groq, ollama, mistral, google, deepseek, openrouter

  ## Rationale
  Conservative initial set — only providers whose tool-calling has been validated against pantheon's openai-compatible adapter. Other providers fall back to `@agent`-prefix-only.

  ## Maintenance
  Update map in `internal/agent/native_tool_calling.go` as new providers are confirmed. Optional follow-up: probe pantheon's provider for a runtime capability discovery method.
  ```

- [ ] Boot the server, run the manual smoke. Verify both SSE chunks + WS welcome.

### Acceptance

- Server boots without panic
- Manual smoke confirms full HTTP→WS flow
- Decision artefacts present
- `go test ./...` 100% green

### Commit

`feat(server): wire AgentInvoker into ChatService + decision artefacts`

---

## Task 5: E2E test — full handoff via httptest

**Files:**
- `backend/internal/agent/e2e_handoff_test.go` (NEW)

**Tests:**
- `TestE2E_AgentChat_FullHandoff`

### Steps

- [ ] Write the e2e test that exercises the entire flow without mocking anything below the LLM:
  ```go
  package agent_test

  import (
      "bufio"
      "context"
      "encoding/json"
      "io"
      "net/http"
      "net/url"
      "strings"
      "testing"
      "time"

      "github.com/gorilla/websocket"
      "github.com/stretchr/testify/require"

      "github.com/odysseythink/hermind/backend/internal/agent"
      "github.com/odysseythink/hermind/backend/internal/dto"
  )

  func TestE2E_AgentChat_FullHandoff(t *testing.T) {
      env := newFullAgentE2EEnv(t)  // wires chatSvc + agentRuntime + handlers
      ws := seedWorkspace(t, env.DB)
      mock := &mockLanguageModel{
          provider: "openai", model: "gpt-4o-mini",
          replies: []string{"Hello back!", "TERMINATE"},
      }
      env.Runtime.SetTestLanguageModelOverride(mock)

      // Phase 1: POST stream-chat with @agent prefix
      reqBody := `{"message":"@agent hi"}`
      req, _ := http.NewRequest("POST", env.Server.URL+"/api/workspace/"+ws.Slug+"/stream-chat", strings.NewReader(reqBody))
      req.Header.Set("Authorization", "Bearer "+env.SessionJWT)
      req.Header.Set("Content-Type", "application/json")
      resp, err := http.DefaultClient.Do(req)
      require.NoError(t, err)
      defer resp.Body.Close()

      // Parse first two SSE chunks
      var initChunk, statusChunk dto.StreamChatResponse
      readSSEChunk(t, resp.Body, &initChunk)
      readSSEChunk(t, resp.Body, &statusChunk)
      require.Equal(t, "agentInitWebsocketConnection", initChunk.Type)
      require.NotNil(t, initChunk.WebsocketUUID)
      require.NotNil(t, initChunk.WebsocketToken)
      require.Equal(t, "statusResponse", statusChunk.Type)
      require.True(t, statusChunk.Close)

      // Phase 2: dial WS with the issued token
      u, _ := url.Parse(env.Server.URL)
      u.Scheme = "ws"
      u.Path = "/api/agent-invocation/" + *initChunk.WebsocketUUID
      q := u.Query(); q.Set("token", *initChunk.WebsocketToken); u.RawQuery = q.Encode()

      conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
      require.NoError(t, err)
      defer conn.Close()

      // Phase 3: expect welcome + assistant reply + close
      var welcome agent.ServerFrame
      require.NoError(t, conn.ReadJSON(&welcome))
      require.Equal(t, agent.FrameStatusResponse, welcome.Type)

      var chat agent.ServerFrame
      require.NoError(t, conn.ReadJSON(&chat))
      require.Equal(t, "@agent", chat.From)
      require.Equal(t, "Hello back!", chat.Content)

      conn.SetReadDeadline(time.Now().Add(2 * time.Second))
      _, _, err = conn.ReadMessage()
      require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
  }

  // readSSEChunk reads one `data: <json>\n\n` block from the SSE body and decodes into target.
  func readSSEChunk(t *testing.T, body io.Reader, target *dto.StreamChatResponse) {
      scanner := bufio.NewScanner(body)
      scanner.Buffer(make([]byte, 256*1024), 256*1024)
      for scanner.Scan() {
          line := scanner.Text()
          if !strings.HasPrefix(line, "data: ") { continue }
          payload := strings.TrimPrefix(line, "data: ")
          require.NoError(t, json.Unmarshal([]byte(payload), target))
          // Skip blank line
          scanner.Scan()
          return
      }
      t.Fatal("SSE stream ended before chunk arrived")
  }
  ```

- [ ] Implement `newFullAgentE2EEnv(t)` helper that:
  - Sets up the full stack: db, services, agentRuntime, chatSvc (with invoker), Gin engine with **all** relevant routes registered (`stream-chat`, `agent-invocation`)
  - Returns `{DB, Server, SessionJWT, User, Runtime}`
  - The helper is large but it's a one-time investment; subsequent e2e tests reuse it

  > **JWT vs auth-disabled**: pick auth-enabled for this test so the AUTH_DISABLED_BYPASS path is exercised separately by unit tests. Mint a JWT via `authSvc.IssueToken(ctx, user)` (or equivalent) — verify the method exists in `internal/services/auth_service.go`.

- [ ] Run test; if scaffolding takes more than 2h, split into a separate `e2e_setup_test.go` for the helper.

- [ ] Full suite green; e2e runs in <2s.

### Acceptance

- `TestE2E_AgentChat_FullHandoff` passes
- The test exercises **every** PR-AR-1 through PR-AR-4 wiring without mocking any of it (only the LLM is mocked)
- No leaked goroutines (verify by checking `runtime.NumGoroutine()` before/after with +/-2 tolerance)
- Test completes in under 2s

### Commit

`test(agent): e2e — full HTTP stream-chat → WS @agent reply round-trip`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` 100% green
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] Two decision artefacts present in `.gpowers/decisions/`
- [ ] Manual smoke documented in main.go comment block
- [ ] **FE companion PR brief written and filed** (single markdown note describing the 4-line FE change required: read `websocketToken` from chunk, append `?token=` to WS URL). File location: `.gpowers/notes/2026-05-27-fe-companion-pr-AR-4.md`
- [ ] No new TODOs without `PR-AR-N` reference

## Risk notes

| Risk | Mitigation |
|---|---|
| Frontend NOT updated → `?token=` missing → WS upgrade rejected with 401 | The FE companion PR is a hard blocker. The chunk format includes the new `websocketToken` field; FE that ignores it will fail-fast at WS dial, not silently break |
| Static native-tool-calling whitelist drifts from pantheon's actual capability | Tag the whitelist file with a `// Last verified against pantheon v0.0.9` comment; refresh on pantheon upgrade |
| `parseAgentHandles` differs from Node's regex (Node uses `\s+` split) | Test cases lifted directly from Node behaviour; review against `workspaceAgentInvocation.js:6-9` |
| `Animate bool` + `omitempty` — Go omits `false`, but FE may treat absence as `undefined` → distinct from `false` | Verify FE statusResponse path treats `!animate` same as `animate===false`. Reviewed: `frontend/src/utils/chat/index.js` reads via destructure with default, so this is safe |
| Two SSE chunks emitted in tight loop — buffered channel size 16, no risk | n/a (PR-AR-1 chose 16) |
| `PrepareInvocationHandoff` race: token issued, invocation created — if process crashes between, orphan token is harmless (TTL 3min) | Token TTL is the safety net; document |
| ChatService construction order — must move; any other service that already takes ChatService now needs reordering too | Verified in main.go: ChatService is consumed by `handlers.RegisterChatRoutes` only. No deeper chain. |
| Test `TestE2E_AgentChat_FullHandoff` slow due to real DB | Use sqlite-mem (`:memory:`); existing `agentTestEnv` does this |

## Estimate

| Task | Hours |
|---|---|
| 1. parseAgentHandles + supportsNativeToolCalling + IsAgentInvocation | 2.0 |
| 2. PrepareInvocationHandoff + rollback + AgentInvoker interface | 2.0 |
| 3. StreamChatResponse extension + early-detect branch | 2.0 |
| 4. main.go wiring + decision artefacts | 1.0 |
| 5. E2E test | 2.5 |
| **Total** | **9.5** (design estimate 8-10h, mid-range ✓) |

—— end of plan
