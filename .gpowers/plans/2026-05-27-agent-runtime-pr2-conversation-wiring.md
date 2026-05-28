# Agent Runtime PR-AR-2 — Pantheon Conversation + Agent Wiring (tool-less) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace PR-AR-1's echo loop with a real conversation: construct a `pantheon/conversation.Conversation` with two participants (USER + @agent), wire `OnMessage/OnError/OnTerminate/OnInterrupt` to WebSocket frames, kick off `Conversation.Start(ctx, USER, @agent, prompt)` and stream the resulting whole-message replies back to the client. **No tools registered** — `Participant.Model` only; `Agent` field stays nil. Tools land in PR-AR-3.

**Architecture:** A thin `wsConn` wrapper around `*websocket.Conn` provides a serialised writer + ping/pong, replacing the ad-hoc `conn.WriteJSON` calls in PR-AR-1. A `Session` owns the `Conversation` + its `context.CancelFunc` + the `wsConn` + a `feedback` channel for resuming after interrupt. `Runtime.HandleWS` rewrites to: lookup invocation → load workspace+user → build `core.LanguageModel` → create `Session` → install event bridges → call `conv.Start(...)` in a goroutine → block on session lifetime → cleanup. A new `agent.buildLanguageModel(ws, settings, cfg)` factory resolves provider+model from the workspace, falling back to system settings then global config, and caches per provider+model key inside the `Runtime`.

**Tech Stack:** Go 1.25.5, Gin v1.10, gorilla/websocket v1.5.3, `github.com/odysseythink/pantheon/conversation v0.0.9`, `github.com/odysseythink/pantheon/core v0.0.9`. **No new dependencies** — pantheon is already in `go.mod`.

**Source spec:** `.gpowers/designs/2026-05-26-agent-runtime-design.md` §3.1, §3.2, §4.2, §5, §10, §14 (PR-AR-2 row).

**Reference Node implementation:**
- `server/utils/agents/aibitat/index.js` — `start({content})`, `_chat()`, `onMessage/onError/onTerminate/onInterrupt`
- `server/utils/agents/aibitat/plugins/websocket.js` — `aibitat.introspect`, `socket.askForFeedback` (we replicate the wire frames, not the JS structure)
- `server/utils/agents/index.js` `AgentHandler` — system-prompt assembly + USER/@agent participant shape

---

## Pre-task: Read this section once before starting

### What landed in PR-AR-1 (do not re-implement)

- `internal/agent/runtime.go` — `Runtime`, `Deps`, `NewRuntime`, `Shutdown` (stub), `upgrader`, `sessions sync.Map`
- `internal/agent/handler.go` — `(*Runtime).HandleWS` echo loop **to be replaced** in Task 5
- `internal/agent/types.go` — `ServerFrame`, `ClientFrame`, `FrameXxx` constants (all still valid; we'll use more of them this PR)
- `internal/agent/invocation.go` — `CreateInvocation` / `GetInvocation` / `CloseInvocation` (used as-is)
- `internal/middleware/ws_auth.go` — `WSValidatedRequest` (used as-is)
- `internal/handlers/agent_token.go` — `POST /workspace/:slug/agent-token` (used as-is)
- `internal/handlers/agent.go` — `RegisterAgentRoutes` (used as-is)
- `internal/models/workspace_agent_invocation.go` — model + automigrate
- `internal/services/temporary_auth_token_service.go` — `IssueWithTTL`

### Pantheon API contract (verified against `github.com/odysseythink/pantheon@v0.0.9`)

```go
// conversation/conversation.go
func New(opts ...Option) *Conversation
func (c *Conversation) RegisterParticipant(p *Participant)
func (c *Conversation) RegisterChannel(ch *Channel)
func (c *Conversation) OnStart(StartHandler)
func (c *Conversation) OnMessage(MessageHandler)        // fn(chat Chat, conv *Conversation)
func (c *Conversation) OnError(ErrorHandler)            // fn(err error, route Route, conv *Conversation)
func (c *Conversation) OnTerminate(TerminateHandler)    // fn(node string, conv *Conversation)
func (c *Conversation) OnInterrupt(InterruptHandler)    // fn(route Route, conv *Conversation)
func (c *Conversation) Start(ctx, from, to, content string) error
func (c *Conversation) Continue(ctx context.Context, feedback string) error
func (c *Conversation) Retry(ctx context.Context) error
func (c *Conversation) Use(plugins ...Plugin) error

// conversation/participant.go
type Participant struct {
    Name      string
    Role      string                // system prompt
    Model     core.LanguageModel    // PR-AR-2 path
    Agent     *agent.Agent          // PR-AR-3+
    Interrupt InterruptMode         // "NEVER" | "ALWAYS"
}

// conversation/history.go
type Chat struct { From, To, Content string; State ChatState }    // "success"|"error"|"interrupt"
type Route struct { From, To string }

// core/provider.go
type LanguageModel interface {
    Generate(ctx context.Context, req *Request) (*Response, error)
    Stream(ctx context.Context, req *Request) (StreamResponse, error)
    GenerateObject(ctx context.Context, req *ObjectRequest) (*ObjectResponse, error)
    Provider() string
    Model() string
}
```

**Critical observations:**

1. `Conversation.Start` runs **synchronously** until terminate/interrupt/error. Must be invoked in a goroutine if the caller also wants to read WS frames.
2. There is **no public `Abort`**. We cancel via `context.CancelFunc`; `Conversation.runLoop` checks `ctx.Err()` at each iteration top, so cancel propagates within one round-trip.
3. `Conversation.reply` uses `Model.Generate` (sync) — **PR-AR-2 emits whole-message replies only.** No token-level streaming on the wire. PR-AR-3+ may switch to a custom path via `Agent.RunStream` when tools land.
4. `Continue(ctx, feedback)` resumes from the **last Chat with `State == Interrupt`**. Returns `ErrNoChatToContinue` otherwise. We map `awaitingFeedback` client frame → `Continue` call.
5. `OnMessage` fires for **every** message, including the USER seed message and assistant replies. Node mutes USER echoes; we replicate that with a `muteUserReply: true` default.
6. `reply` accepts `Content == "TERMINATE"` or `"INTERRUPT"` as control sentinels. Our mock LLMs in tests can return these to drive lifecycle.

### New surface (this PR)

```
backend/internal/agent/
├── wsconn.go               # NEW — *wsConn: serialised writer goroutine, ping/pong, close
├── wsconn_test.go          # NEW — concurrent-write serialisation + close idempotency
├── session.go              # MODIFY — Session{} fleshed out with Conversation, cancel, feedbackCh
├── session_test.go         # NEW — Run happy path, cancel via ctx, interrupt → Continue
├── llm_factory.go          # NEW — buildLanguageModel(ws, settings, cfg) + per-Runtime cache
├── llm_factory_test.go     # NEW — provider routing + cache hit + missing-key error
├── system_prompt.go        # NEW — resolveSystemPrompt(ws, user) (workspace.OpenAiPrompt || default)
├── system_prompt_test.go   # NEW — workspace override + fallback default
├── bridge.go               # NEW — installEventBridges(conv, sess)
├── bridge_test.go          # NEW — each handler maps to the right ServerFrame
├── handler.go              # MODIFY — HandleWS rewritten end-to-end
├── handler_test.go         # MODIFY — augment with conversation e2e (mock LLM)
└── mockllm_test.go         # NEW (test-only) — *mockLanguageModel scripted replies

backend/internal/providers/
└── llm.go                  # MODIFY — add (h *PantheonLLM) LanguageModel() core.LanguageModel; expose factory plumbing
```

### Methods to ship (PR-AR-2 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `providers.LLMProvider` | `LanguageModel() core.LanguageModel` | new interface method; returns nil for `noopLLM` |
| 2 | `agent` (unexported) | `buildLanguageModel(ws *models.Workspace, settings map[string]string, cfg *config.Config) (core.LanguageModel, error)` | resolves provider+model from workspace → settings → cfg |
| 3 | `agent.Runtime` | `(*Runtime) languageModelFor(ws *models.Workspace) (core.LanguageModel, error)` | wraps `buildLanguageModel` with a `sync.Map` cache keyed `provider+":"+model` |
| 4 | `agent` (unexported) | `resolveSystemPrompt(ws *models.Workspace, user *models.User) string` | workspace.OpenAiPrompt > defaults.HermindSystemPrompt |
| 5 | `agent` (unexported) | `newWSConn(conn *websocket.Conn) *wsConn` | wraps + starts writer goroutine |
| 6 | `agent.*wsConn` | `Send(frame ServerFrame) error` | non-blocking enqueue with 8-slot buffer; on overflow returns ErrSlowReader |
| 7 | `agent.*wsConn` | `Close()` | closes writer chan, signals shutdown; idempotent |
| 8 | `agent.Session` (PR-AR-1 stub → full) | fields: `Conv *conversation.Conversation`, `ctx context.Context`, `cancel context.CancelFunc`, `wsConn *wsConn`, `feedbackCh chan feedbackMsg`, `terminated chan struct{}` | |
| 9 | `agent.*Session` | `Run(ctx context.Context, prompt string) error` | calls `Conv.Start`; returns on terminate/error |
| 10 | `agent.*Session` | `Continue(feedback string)` | pushes onto `feedbackCh` |
| 11 | `agent.*Session` | `Abort(reason string)` | calls cancel; sends `wssFailure` if reason!="" |
| 12 | `agent` (unexported) | `installEventBridges(s *Session)` | registers OnMessage/OnError/OnTerminate/OnInterrupt |
| 13 | `agent.Runtime` | `(*Runtime) HandleWS(c *gin.Context)` | **REWRITE**: upgrade → buildSession → run → drain reader |
| 14 | `agent.Runtime` | `(*Runtime) Shutdown(ctx)` | **REWRITE**: iterate `sessions` and Abort, bounded by ctx |

### Frame protocol (PR-AR-2 specifics — fills out PR-AR-1 reservations)

| Direction | When | Frame |
|---|---|---|
| S→C | upgrade success | `{type:"statusResponse", content:"@agent runtime ready", animate:false}` |
| S→C | conversation OnMessage (non-USER) | `{type:"<chat>", from, to, content, state}` *(note: `type` field is the chat marker — Node uses no type field on chat messages; for parity we **drop type** and use bare `{from, to, content, state}`)* |
| S→C | conversation OnError | `{type:"wssFailure", content:"<err>"}` |
| S→C | conversation OnTerminate | (none — WS closes with code 1000 right after) |
| S→C | conversation OnInterrupt | `{type:"WAITING_ON_INPUT", question:"Provide feedback to <to> as <from>."}` |
| C→S | `{type:"awaitingFeedback", feedback, attachments?}` | pushed onto `feedbackCh` → reader goroutine calls `Conv.Continue(ctx, feedback)` |
| C→S | bare bail string (`exit`/`/exit`/`stop`/`/stop`/`halt`/`/halt`/`/reset`) | `Session.Abort("user requested exit")` |
| C→S | `{type:"toolApprovalResponse", ...}` | PR-AR-5 — ignored in PR-AR-2 with a debug log line |

> **Backward compat note:** PR-AR-1 sent every echo as `{type:"__unhandled", content}`. PR-AR-2 stops emitting `__unhandled` entirely; the constant is kept for future use.

### Chat-frame `type` field — Node parity

Node's `aibitat.onMessage` does `socket.send(JSON.stringify(message))` where `message = {from, to, content, state}`. **No `type` field.** Our `ServerFrame` already declared `from/to/content/state` but with `type` always populated; for chat messages we serialise with `type` empty. Add `omitempty` tags so the wire format matches Node's exactly:

```go
type ServerFrame struct {
    Type     string `json:"type,omitempty"`      // omitempty so chat messages don't carry it
    Content  string `json:"content,omitempty"`
    Animate  bool   `json:"animate,omitempty"`
    From     string `json:"from,omitempty"`
    To       string `json:"to,omitempty"`
    State    string `json:"state,omitempty"`
    Question string `json:"question,omitempty"`   // NEW (PR-AR-2) — WAITING_ON_INPUT
}
```

> Update Task 1 of `types.go` accordingly — this is a 1-line `omitempty` addition + new `Question` field.

### Test helpers (extend PR-AR-1's helpers)

```go
// internal/agent/mockllm_test.go (test-only)

type mockLanguageModel struct {
    provider, model string
    replies         []string // queue of canned replies
    calls           atomic.Int32
}

func (m *mockLanguageModel) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
    idx := int(m.calls.Add(1)) - 1
    if idx >= len(m.replies) {
        return nil, fmt.Errorf("mock ran out of replies after %d calls", idx)
    }
    return &core.Response{
        Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, m.replies[idx]),
        Usage: core.Usage{TotalTokens: 1},
    }, nil
}
func (m *mockLanguageModel) Stream(...) (core.StreamResponse, error) { return nil, errors.New("not implemented") }
func (m *mockLanguageModel) GenerateObject(...) (*core.ObjectResponse, error) { return nil, errors.New("not implemented") }
func (m *mockLanguageModel) Provider() string { return m.provider }
func (m *mockLanguageModel) Model() string    { return m.model }

// helper to inject mock into Runtime cache without needing a real API key
func (env *agentTestEnv) InjectMockLLM(m *mockLanguageModel) {
    env.Runtime.SetTestLanguageModelOverride(m)  // exported-for-testing setter, see Task 1
}
```

> Add a `SetTestLanguageModelOverride` method to `Runtime` with a doc comment marking it test-only. Production code never calls it.

### Out of scope (explicit)

- Tool registration / `pantheon/agent.Agent` — PR-AR-3
- HTTP→WS handoff in `chat_service.Stream` — PR-AR-4
- `toolApprovalRequest/Response` — PR-AR-5
- Streaming text-delta over WS — PR-AR-3+ via `Agent.RunStream` (only when tools are present)
- Chat history persistence (`workspace_chats` upserts) — moved to PR-AR-3 along with the `chat-history` skill equivalent
- Telemetry events — out (Node fires `agent_chat_started`/`agent_chat_sent`; we add in PR-AR-5)
- Multi-channel — out (`pantheon/conversation` supports it but UI doesn't)

### TDD discipline

Each task lands as **one commit**. Within a task:

1. Write the failing test(s).
2. `cd backend && go test ./internal/agent/... -run <NewTest>` → red.
3. Implement.
4. Same test → green.
5. `cd backend && go test ./...` → full suite green.
6. Commit `feat(agent): <task summary>`.

---

## Task 1: LLM factory + system prompt resolution + test override

**Files:**
- `backend/internal/providers/llm.go` (MODIFY)
- `backend/internal/agent/llm_factory.go` (NEW)
- `backend/internal/agent/llm_factory_test.go` (NEW)
- `backend/internal/agent/system_prompt.go` (NEW)
- `backend/internal/agent/system_prompt_test.go` (NEW)
- `backend/internal/agent/runtime.go` (MODIFY — add cache + test override)
- `backend/internal/agent/types.go` (MODIFY — add `Question` field + `omitempty` on `Type`)

**Tests:**
- `TestLLMFactory_Ollama_BuildsModel`
- `TestLLMFactory_OpenAI_BuildsModel`
- `TestLLMFactory_NoAPIKey_ReturnsError`
- `TestLLMFactory_WorkspaceOverride_PreferredOverGlobal`
- `TestRuntime_LanguageModelFor_CachesByProviderAndModel`
- `TestSystemPrompt_WorkspaceOverride`
- `TestSystemPrompt_FallbackDefault`

### Steps

- [ ] Extend `providers.LLMProvider` interface:
  ```go
  // internal/providers/llm.go
  type LLMProvider interface {
      Stream(...) (<-chan LLMChunk, error)
      Complete(...) (string, error)
      LanguageModel() core.LanguageModel    // NEW
  }
  ```

- [ ] Implement on `PantheonLLM`:
  ```go
  func (p *PantheonLLM) LanguageModel() core.LanguageModel { return p.model }
  ```

- [ ] Implement on `noopLLM` (return `nil`; callers must check).

- [ ] Write failing `llm_factory_test.go`:
  ```go
  func TestLLMFactory_Ollama_BuildsModel(t *testing.T) {
      cfg := &config.Config{LLMProvider: "ollama", LLMModel: "llama3"}
      ws := &models.Workspace{}
      lm, err := agent.BuildLanguageModelForTesting(ws, map[string]string{
          "LLMProvider":         "ollama",
          "OllamaLLMBasePath":   "http://127.0.0.1:11434",
          "OllamaLLMModelPref":  "llama3",
      }, cfg)
      require.NoError(t, err)
      require.Equal(t, "ollama", lm.Provider())
      require.Equal(t, "llama3", lm.Model())
  }
  ```

- [ ] Implement `internal/agent/llm_factory.go`:
  ```go
  package agent

  import (
      "context"
      "fmt"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/config"
      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/providers/ollama"
      "github.com/odysseythink/pantheon/providers/openai"
  )

  // BuildLanguageModelForTesting exposes buildLanguageModel for unit tests.
  func BuildLanguageModelForTesting(ws *models.Workspace, settings map[string]string, cfg *config.Config) (core.LanguageModel, error) {
      return buildLanguageModel(ws, settings, cfg)
  }

  func buildLanguageModel(ws *models.Workspace, settings map[string]string, cfg *config.Config) (core.LanguageModel, error) {
      providerName := pick("LLMProvider", settings, cfg.LLMProvider)
      modelID := resolveModelID(providerName, ws, settings, cfg)

      switch providerName {
      case "ollama":
          baseURL := strings.TrimSuffix(strings.TrimSuffix(pick("OllamaLLMBasePath", settings, "http://127.0.0.1:11434"), "/"), "/v1")
          p, err := ollama.New("", ollama.WithBaseURL(baseURL))
          if err != nil { return nil, fmt.Errorf("ollama provider: %w", err) }
          return p.LanguageModel(context.Background(), modelID)
      default:
          apiKey := pick("LLMApiKey", settings, cfg.LLMApiKey)
          if apiKey == "" { apiKey = pick("OpenAiKey", settings, cfg.OpenAiKey) }
          if apiKey == "" { return nil, fmt.Errorf("no LLM API key configured for provider %q", providerName) }
          p, err := openai.New(apiKey)
          if err != nil { return nil, fmt.Errorf("openai provider: %w", err) }
          return p.LanguageModel(context.Background(), modelID)
      }
  }

  func pick(key string, settings map[string]string, fallback string) string {
      if v, ok := settings[key]; ok && v != "" { return v }
      return fallback
  }

  func resolveModelID(provider string, ws *models.Workspace, settings map[string]string, cfg *config.Config) string {
      // 1. workspace.AgentModel / workspace.ChatModel (future fields; both optional)
      // 2. per-provider settings key
      // 3. cfg.LLMModel
      switch provider {
      case "ollama":  return pick("OllamaLLMModelPref", settings, cfg.LLMModel)
      case "openai":  return pick("OpenAiModelPref", settings, cfg.LLMModel)
      default:        return cfg.LLMModel
      }
  }
  ```

- [ ] Add Runtime cache + test override:
  ```go
  // internal/agent/runtime.go (additions)
  type Runtime struct {
      // ... existing fields ...
      lmCache    sync.Map  // string("provider:model") → core.LanguageModel
      lmOverride core.LanguageModel  // test-only; if non-nil, languageModelFor returns it
  }

  // SetTestLanguageModelOverride installs a fixed LanguageModel that bypasses
  // the cache and the buildLanguageModel factory. Test-only.
  func (r *Runtime) SetTestLanguageModelOverride(m core.LanguageModel) {
      r.lmOverride = m
  }

  func (r *Runtime) languageModelFor(ws *models.Workspace, settings map[string]string) (core.LanguageModel, error) {
      if r.lmOverride != nil { return r.lmOverride, nil }
      provider := pick("LLMProvider", settings, r.deps.Cfg.LLMProvider)
      model := resolveModelID(provider, ws, settings, r.deps.Cfg)
      key := provider + ":" + model
      if cached, ok := r.lmCache.Load(key); ok { return cached.(core.LanguageModel), nil }
      lm, err := buildLanguageModel(ws, settings, r.deps.Cfg)
      if err != nil { return nil, err }
      r.lmCache.Store(key, lm)
      return lm, nil
  }
  ```

- [ ] Write failing `system_prompt_test.go`:
  ```go
  func TestSystemPrompt_WorkspaceOverride(t *testing.T) {
      ws := &models.Workspace{OpenAiPrompt: utils.Ptr("You are pirate Bob.")}
      got := agent.ResolveSystemPromptForTesting(ws, nil)
      require.Equal(t, "You are pirate Bob.", got)
  }
  func TestSystemPrompt_FallbackDefault(t *testing.T) {
      ws := &models.Workspace{}
      got := agent.ResolveSystemPromptForTesting(ws, nil)
      require.Contains(t, got, "helpful")  // matches default
  }
  ```

- [ ] Implement `system_prompt.go`:
  ```go
  package agent

  import "github.com/odysseythink/hermind/backend/internal/models"

  const defaultSystemPrompt = `You are a helpful AI assistant. You can use available tools to answer the user's questions.`

  func resolveSystemPrompt(ws *models.Workspace, user *models.User) string {
      if ws != nil && ws.OpenAiPrompt != nil && *ws.OpenAiPrompt != "" {
          return *ws.OpenAiPrompt
      }
      return defaultSystemPrompt
  }

  func ResolveSystemPromptForTesting(ws *models.Workspace, user *models.User) string {
      return resolveSystemPrompt(ws, user)
  }
  ```

- [ ] Update `internal/agent/types.go` — add `Question` field + `omitempty` on `Type`:
  ```go
  type ServerFrame struct {
      Type     string `json:"type,omitempty"`
      Content  string `json:"content,omitempty"`
      Animate  bool   `json:"animate,omitempty"`
      From     string `json:"from,omitempty"`
      To       string `json:"to,omitempty"`
      State    string `json:"state,omitempty"`
      Question string `json:"question,omitempty"`
  }
  ```

- [ ] Update `internal/services/chat_service.go` — if any chat code references `llmProv.Stream/Complete`, no change needed; if it references the old interface concretely, ensure compile passes.

- [ ] `go vet ./...` + `go test ./...` clean.

### Acceptance

- All 7 tests pass
- `providers.LLMProvider` interface includes `LanguageModel()`
- `Runtime.languageModelFor` reuses cached models on second call
- `SetTestLanguageModelOverride` short-circuits the factory
- `omitempty` on `ServerFrame.Type` means chat frames serialise as `{"from":"...","to":"...",...}` with no `"type"` key (Node parity)

### Commit

`feat(agent): LLM factory + system prompt resolver + test-mock override`

---

## Task 2: WebSocket connection wrapper (serialised writer + ping/pong)

**Files:**
- `backend/internal/agent/wsconn.go` (NEW)
- `backend/internal/agent/wsconn_test.go` (NEW)

**Tests:**
- `TestWSConn_ConcurrentSendsAreSerialised` (1000 goroutines call Send; reader sees 1000 frames, no JSON corruption)
- `TestWSConn_SendAfterCloseReturnsError`
- `TestWSConn_CloseIsIdempotent`
- `TestWSConn_PingPongResetsReadDeadline` (slow-tick test using 100ms deadlines)
- `TestWSConn_SlowReaderTriggersErrSlowReader` (buffer overflow)

### Steps

- [ ] Write failing `wsconn_test.go`:
  ```go
  func TestWSConn_ConcurrentSendsAreSerialised(t *testing.T) {
      srv, clientConn := newPipedWS(t)  // helper using net.Pipe + websocket.NewServerConn equivalent
      defer srv.Close(); defer clientConn.Close()

      wc := agent.NewWSConnForTesting(srv)
      var wg sync.WaitGroup
      for i := 0; i < 1000; i++ {
          wg.Add(1)
          go func(n int) {
              defer wg.Done()
              _ = wc.Send(agent.ServerFrame{Type: agent.FrameStatusResponse, Content: fmt.Sprintf("%d", n)})
          }(i)
      }
      wg.Wait()

      seen := map[string]bool{}
      for i := 0; i < 1000; i++ {
          var f agent.ServerFrame
          require.NoError(t, clientConn.ReadJSON(&f))
          seen[f.Content] = true
      }
      require.Len(t, seen, 1000)
  }
  ```

- [ ] Implement `wsconn.go`:
  ```go
  package agent

  import (
      "context"
      "errors"
      "fmt"
      "sync"
      "time"

      "github.com/gorilla/websocket"
  )

  const (
      wsOutboundBuffer = 8
      wsWriteTimeout   = 30 * time.Second
      wsPingInterval   = 30 * time.Second
      wsReadDeadline   = 5 * time.Minute
  )

  var (
      ErrSlowReader = errors.New("ws outbound buffer full (slow reader)")
      ErrConnClosed = errors.New("ws connection closed")
  )

  type wsConn struct {
      conn     *websocket.Conn
      outbound chan ServerFrame
      done     chan struct{}
      closeOnce sync.Once
      err      error
      writerDone chan struct{}
  }

  func newWSConn(conn *websocket.Conn) *wsConn {
      wc := &wsConn{
          conn:       conn,
          outbound:   make(chan ServerFrame, wsOutboundBuffer),
          done:       make(chan struct{}),
          writerDone: make(chan struct{}),
      }
      go wc.writerLoop()
      _ = wc.conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
      wc.conn.SetPongHandler(func(string) error {
          return wc.conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
      })
      return wc
  }

  // NewWSConnForTesting wraps a raw conn; test-only.
  func NewWSConnForTesting(conn *websocket.Conn) *wsConn { return newWSConn(conn) }

  func (w *wsConn) Send(f ServerFrame) error {
      select {
      case <-w.done:
          return ErrConnClosed
      case w.outbound <- f:
          return nil
      default:
          return ErrSlowReader
      }
  }

  func (w *wsConn) Close() {
      w.closeOnce.Do(func() {
          close(w.done)
          // wait briefly for writer to drain, then force-close conn
          select {
          case <-w.writerDone:
          case <-time.After(2 * time.Second):
          }
          _ = w.conn.WriteControl(websocket.CloseMessage,
              websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
              time.Now().Add(time.Second))
          _ = w.conn.Close()
      })
  }

  func (w *wsConn) ReadMessage() (int, []byte, error) {
      return w.conn.ReadMessage()
  }

  func (w *wsConn) writerLoop() {
      defer close(w.writerDone)
      pingTicker := time.NewTicker(wsPingInterval)
      defer pingTicker.Stop()

      for {
          select {
          case <-w.done:
              return
          case <-pingTicker.C:
              _ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
              if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                  w.err = fmt.Errorf("ping: %w", err)
                  return
              }
          case f := <-w.outbound:
              _ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
              if err := w.conn.WriteJSON(f); err != nil {
                  w.err = fmt.Errorf("write json: %w", err)
                  return
              }
          }
      }
  }
  ```

  > **Slow-reader caveat**: the `default` arm in `Send` returns `ErrSlowReader` immediately when the buffer is full. The session's error path will Abort on this — slow client should not block agent execution. Alternatively a `select { case ..., case <-time.After(100ms): }` would block briefly; we pick non-blocking to keep semantics simple.

- [ ] Run tests, confirm pass.

### Acceptance

- All 5 tests pass
- No goroutine leaks (use `goleak.VerifyTestMain` or equivalent? — out of scope; instead include a `wsConn.Close()` defer check by hand)
- Concurrent senders never produce malformed JSON on the wire

### Commit

`feat(agent): wsConn — serialised writer + ping/pong + idempotent close`

---

## Task 3: Session + event bridges

**Files:**
- `backend/internal/agent/session.go` (MODIFY — PR-AR-1 left it as a 4-field stub)
- `backend/internal/agent/bridge.go` (NEW)
- `backend/internal/agent/bridge_test.go` (NEW)
- `backend/internal/agent/session_test.go` (NEW)
- `backend/internal/agent/mockllm_test.go` (NEW)

**Tests:**
- `TestBridge_OnMessageUSERIsMutedByDefault`
- `TestBridge_OnMessageAgentEmitsChatFrame`
- `TestBridge_OnErrorEmitsWSSFailure`
- `TestBridge_OnInterruptEmitsWaitingOnInput`
- `TestBridge_OnTerminateClosesSession`
- `TestSession_Run_SingleTurnReply` (mock LLM returns "Hello back!" once → assert one chat frame; then conversation terminates because mock returns "TERMINATE" on 2nd call — see note)
- `TestSession_Run_ContextCancelStopsRunLoop`

> **Note on Single-turn test mechanics**: `Conversation.Start` enters `runLoop` and alternates routes until `Reply` returns "TERMINATE" or `hasReachedMaxRounds` trips. To keep tests fast, the mock returns the assistant reply, then on the next call returns `"TERMINATE"` (the sentinel pantheon honors).

### Steps

- [ ] Write failing `session_test.go`:
  ```go
  func TestSession_Run_SingleTurnReply(t *testing.T) {
      env := newAgentTestEnv(t)
      ws := seedWorkspace(t, env.DB)
      mock := &mockLanguageModel{
          provider: "mock", model: "mock-model",
          replies: []string{"Hello back!", "TERMINATE"},
      }
      env.Runtime.SetTestLanguageModelOverride(mock)
      uid, _ := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hi")
      tok := env.IssueTempToken(t, env.User.ID, time.Minute)
      conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, tok)

      _ = expectFrame(t, conn, agent.FrameStatusResponse)  // welcome
      chat := expectFrame(t, conn, "" /*type empty*/)
      require.Equal(t, "@agent", chat.From)
      require.Equal(t, "USER", chat.To)
      require.Equal(t, "Hello back!", chat.Content)
      require.Equal(t, "success", chat.State)

      // After terminate, server closes; ReadMessage should return close.
      _, _, err := conn.ReadMessage()
      require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
  }
  ```

- [ ] Add `mockllm_test.go` (see §Test helpers above).

- [ ] Implement Session in `session.go`:
  ```go
  package agent

  import (
      "context"
      "errors"
      "sync"
      "time"

      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/mlog"
      "github.com/odysseythink/pantheon/conversation"
      "github.com/odysseythink/pantheon/core"
  )

  const (
      participantUser   = "USER"
      participantAgent  = "@agent"
      defaultMaxRounds  = 50
  )

  type Session struct {
      UUID         string
      WorkspaceID  int
      UserID       *int

      conv      *conversation.Conversation
      lm        core.LanguageModel
      systemPrompt string

      wsConn     *wsConn
      ctx        context.Context
      cancel     context.CancelFunc
      feedbackCh chan feedbackMsg
      terminated chan struct{}
      muteUser   bool

      startedAt time.Time
      once      sync.Once
  }

  type feedbackMsg struct {
      Content     string
      Attachments []any
  }

  func newSession(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User, lm core.LanguageModel, systemPrompt string, conn *wsConn) *Session {
      ctx, cancel := context.WithCancel(parentCtx)
      s := &Session{
          UUID:         uuid,
          WorkspaceID:  ws.ID,
          lm:           lm,
          systemPrompt: systemPrompt,
          wsConn:       conn,
          ctx:          ctx,
          cancel:       cancel,
          feedbackCh:   make(chan feedbackMsg, 1),
          terminated:   make(chan struct{}),
          muteUser:     true,
          startedAt:    time.Now(),
      }
      if user != nil { s.UserID = &user.ID }
      s.conv = conversation.New(conversation.WithMaxRounds(defaultMaxRounds))
      s.conv.RegisterParticipant(&conversation.Participant{
          Name:      participantUser,
          Role:      "I am the human user.",
          Interrupt: conversation.InterruptAlways,
      })
      s.conv.RegisterParticipant(&conversation.Participant{
          Name:  participantAgent,
          Role:  systemPrompt,
          Model: lm,
      })
      installEventBridges(s)
      return s
  }

  func (s *Session) Run(prompt string) error {
      err := s.conv.Start(s.ctx, participantUser, participantAgent, prompt)
      // Terminate handler also closes terminated; await it for at-most 1s safety net
      s.once.Do(func() { close(s.terminated) })
      return err
  }

  func (s *Session) Continue(feedback string, attachments []any) {
      select {
      case s.feedbackCh <- feedbackMsg{Content: feedback, Attachments: attachments}:
      default:
          mlog.Warning("agent: dropped feedback (channel full)")
      }
  }

  func (s *Session) Abort(reason string) {
      if reason != "" {
          _ = s.wsConn.Send(ServerFrame{Type: FrameWSSFailure, Content: reason})
      }
      s.cancel()
  }

  var ErrSessionTerminated = errors.New("session terminated")
  ```

- [ ] Implement `bridge.go`:
  ```go
  package agent

  import (
      "fmt"

      "github.com/odysseythink/pantheon/conversation"
  )

  func installEventBridges(s *Session) {
      s.conv.OnMessage(func(chat conversation.Chat, _ *conversation.Conversation) {
          if s.muteUser && chat.From == participantUser { return }
          _ = s.wsConn.Send(ServerFrame{
              From:    chat.From,
              To:      chat.To,
              Content: chat.Content,
              State:   string(chat.State),
          })
      })
      s.conv.OnError(func(err error, _ conversation.Route, _ *conversation.Conversation) {
          _ = s.wsConn.Send(ServerFrame{
              Type:    FrameWSSFailure,
              Content: err.Error(),
          })
          s.cancel()
      })
      s.conv.OnInterrupt(func(route conversation.Route, _ *conversation.Conversation) {
          _ = s.wsConn.Send(ServerFrame{
              Type:     FrameWaitingOnInput,
              Question: fmt.Sprintf("Provide feedback to %s as %s.", route.To, route.From),
          })
          // Reader-loop in HandleWS will forward awaitingFeedback → s.Continue → call Conv.Continue.
          // The blocking Conv.Start returns when Continue completes the loop OR cancel fires.
          go func() {
              select {
              case fb := <-s.feedbackCh:
                  _ = s.conv.Continue(s.ctx, fb.Content)
              case <-s.ctx.Done():
                  return
              }
          }()
      })
      s.conv.OnTerminate(func(_ string, _ *conversation.Conversation) {
          s.once.Do(func() { close(s.terminated) })
      })
  }
  ```

  > **Subtle**: `OnInterrupt` fires on `Conversation.runLoop` returning. `Continue` then re-enters runLoop synchronously. We launch a goroutine so we don't deadlock the original goroutine that was about to return from `Start`.

- [ ] Run tests, confirm pass.

### Acceptance

- All 7 tests pass
- Chat frame serialisation omits `type` (verify via `json.Marshal` in a small unit test)
- `Session.Abort("reason")` sends `wssFailure` exactly once

### Commit

`feat(agent): Session + conversation event → WS frame bridges`

---

## Task 4: Reader-loop client frame routing

**Files:**
- `backend/internal/agent/reader.go` (NEW)
- `backend/internal/agent/reader_test.go` (NEW)

**Tests:**
- `TestReader_BailCommands_AbortSession`  (each of `exit`/`/exit`/`stop`/`/stop`/`halt`/`/halt`/`/reset` triggers cancel)
- `TestReader_AwaitingFeedback_ForwardsToSession`
- `TestReader_InvalidJSONIsIgnored`
- `TestReader_BinaryFramesIgnored`
- `TestReader_OnPanicTerminatesGracefully`

### Steps

- [ ] Write failing `reader_test.go`:
  ```go
  func TestReader_BailCommands_AbortSession(t *testing.T) {
      for _, cmd := range []string{"exit", "/exit", "stop", "/stop", "halt", "/halt", "/reset"} {
          t.Run(cmd, func(t *testing.T) {
              env := newAgentTestEnv(t)
              mock := &mockLanguageModel{provider: "mock", model: "m",
                  replies: []string{slowReply, "TERMINATE"}}  // slowReply blocks until ctx cancel
              env.Runtime.SetTestLanguageModelOverride(mock)
              ws := seedWorkspace(t, env.DB)
              uid, _ := env.Runtime.CreateInvocation(ctx, ws, env.User, nil, "@agent")
              conn, _ := env.DialWS(t, "/api/agent-invocation/"+uid, env.IssueTempToken(t, env.User.ID, time.Minute))
              _ = expectFrame(t, conn, agent.FrameStatusResponse)
              require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(cmd)))
              // server should close normally within ~1s
              conn.SetReadDeadline(time.Now().Add(2 * time.Second))
              for {
                  _, _, err := conn.ReadMessage()
                  if err != nil {
                      require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
                      return
                  }
              }
          })
      }
  }
  ```

- [ ] Implement `reader.go`:
  ```go
  package agent

  import (
      "encoding/json"

      "github.com/gorilla/websocket"
      "github.com/odysseythink/mlog"
  )

  var bailCommands = map[string]struct{}{
      "exit": {}, "/exit": {}, "stop": {}, "/stop": {}, "halt": {}, "/halt": {}, "/reset": {},
  }

  // readerLoop runs in a goroutine until Session ctx is cancelled or the conn errors.
  func (s *Session) readerLoop() {
      defer func() {
          if r := recover(); r != nil {
              mlog.Error("agent reader panic: ", r)
              s.Abort("internal reader panic")
          }
      }()
      for {
          mt, raw, err := s.wsConn.ReadMessage()
          if err != nil {
              s.cancel()
              return
          }
          if mt != websocket.TextMessage { continue }

          // 1. bare bail-command string
          trimmed := string(raw)
          if _, ok := bailCommands[trimmed]; ok {
              s.Abort("")  // graceful cancel, no wssFailure
              return
          }

          // 2. JSON frame
          var f ClientFrame
          if err := json.Unmarshal(raw, &f); err != nil {
              mlog.Warning("agent: ignored non-JSON frame: ", trimmed)
              continue
          }
          switch f.Type {
          case FrameAwaitingFeedback:
              if _, ok := bailCommands[f.Feedback]; ok {
                  s.Abort("")
                  return
              }
              s.Continue(f.Feedback, f.Attachments)
          case FrameToolApprovalResp:
              mlog.Info("agent: tool approval response received (handled in PR-AR-5)")
          default:
              mlog.Warning("agent: unknown client frame type=", f.Type)
          }
      }
  }
  ```

- [ ] Add a `slowReply` helper to `mockllm_test.go`:
  ```go
  const slowReply = "__SLOW__"

  // In Generate, if reply == "__SLOW__", block on ctx.Done():
  func (m *mockLanguageModel) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
      idx := int(m.calls.Add(1)) - 1
      if idx >= len(m.replies) { return nil, fmt.Errorf(...) }
      r := m.replies[idx]
      if r == slowReply {
          <-ctx.Done()
          return nil, ctx.Err()
      }
      return &core.Response{ Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, r) }, nil
  }
  ```

- [ ] Run tests, confirm pass.

### Acceptance

- All 5 tests pass
- All 7 bail commands trigger graceful cancel within 2s
- Non-JSON noise is logged but doesn't crash the reader
- Reader panic → `Abort("internal reader panic")`

### Commit

`feat(agent): WS reader loop — bail commands + feedback routing + panic guard`

---

## Task 5: HandleWS rewrite + Shutdown impl + e2e

**Files:**
- `backend/internal/agent/handler.go` (REWRITE — replaces echo)
- `backend/internal/agent/handler_test.go` (MODIFY — replace PR-AR-1 echo tests with conversation e2e)
- `backend/internal/agent/runtime.go` (MODIFY — implement Shutdown body)
- `backend/internal/services/workspace_service.go` (MODIFY — ensure `GetWorkspaceByID` or `FindBySlug` returns full workspace; verify before assuming)

**Tests:**
- `TestHandleWS_FullConversation_SingleTurn` (mock LLM returns "Hello", then "TERMINATE")
- `TestHandleWS_FullConversation_InterruptAndContinue` (mock returns "INTERRUPT", then on continue returns "OK", then "TERMINATE")
- `TestHandleWS_ContextCancelOnSocketClose` (client closes; server returns within 2s, invocation marked closed)
- `TestRuntime_Shutdown_ClosesAllSessions` (open 3 sessions, call Shutdown, all close within ctx deadline)
- `TestHandleWS_LLMError_EmitsWSSFailure` (mock LLM returns an error; assert `wssFailure` frame received and conn closed)
- `TestHandleWS_NoLLMAvailable_RejectsBeforeUpgrade` (workspace has no provider config → 503 before upgrade)

### Steps

- [ ] Write failing tests (sketch in `handler_test.go`):
  ```go
  func TestHandleWS_FullConversation_InterruptAndContinue(t *testing.T) {
      env := newAgentTestEnv(t)
      mock := &mockLanguageModel{
          provider: "mock", model: "m",
          replies: []string{"INTERRUPT", "Continuing now.", "TERMINATE"},
      }
      env.Runtime.SetTestLanguageModelOverride(mock)
      ws := seedWorkspace(t, env.DB)
      uid, _ := env.Runtime.CreateInvocation(ctx, ws, env.User, nil, "@agent")
      conn, _ := env.DialWS(t, ..., env.IssueTempToken(t, env.User.ID, time.Minute))

      _ = expectFrame(t, conn, agent.FrameStatusResponse)
      waiting := expectFrame(t, conn, agent.FrameWaitingOnInput)
      require.Contains(t, waiting.Question, "Provide feedback")

      require.NoError(t, conn.WriteJSON(agent.ClientFrame{Type: agent.FrameAwaitingFeedback, Feedback: "go ahead"}))

      chat := expectFrame(t, conn, "")
      require.Equal(t, "Continuing now.", chat.Content)
      // server closes after TERMINATE
      _, _, err := conn.ReadMessage()
      require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))
  }
  ```

- [ ] Rewrite `handler.go`:
  ```go
  func (r *Runtime) HandleWS(c *gin.Context) {
      id := c.Param("uuid")
      if id == "" { c.AbortWithStatus(http.StatusBadRequest); return }

      inv, err := r.GetInvocation(c.Request.Context(), id)
      if err != nil {
          mlog.Warning("agent: invocation lookup failed: ", id, " err=", err)
          c.AbortWithStatus(http.StatusNotFound); return
      }

      // Resolve workspace + user
      var ws models.Workspace
      if err := r.deps.DB.WithContext(c.Request.Context()).First(&ws, inv.WorkspaceID).Error; err != nil {
          c.AbortWithStatus(http.StatusNotFound); return
      }
      var user *models.User
      if u, ok := c.Get("user"); ok {
          if uu, ok := u.(*models.User); ok { user = uu }
      }

      // Resolve LLM before upgrade — if config is broken, return 503 with JSON
      settings, _ := r.deps.SysSvc.GetAll(c.Request.Context())  // see note
      lm, err := r.languageModelFor(&ws, settings)
      if err != nil {
          c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "agent: " + err.Error()})
          return
      }
      systemPrompt := resolveSystemPrompt(&ws, user)

      conn, err := r.upgrader.Upgrade(c.Writer, c.Request, nil)
      if err != nil {
          mlog.Error("agent: ws upgrade failed: ", err); return
      }
      wc := newWSConn(conn)
      sess := newSession(c.Request.Context(), inv.UUID, &ws, user, lm, systemPrompt, wc)
      r.sessions.Store(inv.UUID, sess)
      defer func() {
          r.sessions.Delete(inv.UUID)
          _ = r.CloseInvocation(context.Background(), inv.UUID)
          wc.Close()
      }()

      _ = wc.Send(ServerFrame{Type: FrameStatusResponse, Content: "@agent runtime ready"})

      // reader is a separate goroutine; main goroutine runs the conversation
      go sess.readerLoop()
      runErr := sess.Run(inv.Prompt)
      if runErr != nil && !errors.Is(runErr, context.Canceled) {
          _ = wc.Send(ServerFrame{Type: FrameWSSFailure, Content: runErr.Error()})
      }
      // wait briefly for terminate signal so OnTerminate-driven close ordering is clean
      select {
      case <-sess.terminated:
      case <-time.After(500 * time.Millisecond):
      }
  }
  ```

  > **Note on `SysSvc.GetAll`**: this is the existing SystemSettings reader. If it doesn't exist with that exact name, use whatever returns `map[string]string` of all keys. If no such API, default `settings = nil` and rely entirely on `cfg`.

- [ ] Implement `Shutdown` body in `runtime.go`:
  ```go
  func (r *Runtime) Shutdown(ctx context.Context) error {
      r.sessions.Range(func(key, value any) bool {
          if s, ok := value.(*Session); ok {
              s.Abort("server shutting down")
          }
          return true
      })
      // wait for sessions to drain, bounded by ctx
      deadline, hasDeadline := ctx.Deadline()
      ticker := time.NewTicker(50 * time.Millisecond)
      defer ticker.Stop()
      for {
          done := true
          r.sessions.Range(func(_, _ any) bool { done = false; return false })
          if done { return nil }
          if hasDeadline && time.Now().After(deadline) {
              return fmt.Errorf("agent shutdown: timeout with active sessions")
          }
          select {
          case <-ctx.Done(): return ctx.Err()
          case <-ticker.C:
          }
      }
  }
  ```

- [ ] Add `Runtime.Deps` field for `SysSvc` if not present already (only if needed for settings; otherwise pass `nil settings` from chat_service later in PR-AR-4 and document it).

- [ ] Run **full** test suite. Verify nothing regresses, especially:
  - `internal/handlers/...`
  - `internal/services/chat_service_test.go` (the new `LanguageModel()` interface method)

### Acceptance

- All 6 new e2e tests pass
- Shutdown drains 3 concurrent sessions within 2s
- LLM build failure returns 503 BEFORE upgrade (no zombie conn)
- Welcome frame still sent on connect
- Single-turn happy path: USER seed → assistant reply → TERMINATE → conn close 1000 with `closed=true` in DB

### Commit

`feat(agent): HandleWS conversation runtime + Shutdown drain`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` 100% green (race detector!)
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] `internal/agent/doc.go` updated — note that PR-AR-2 is tool-less, PR-AR-3 adds tools
- [ ] `Runtime.Shutdown` invoked from `main.go` graceful shutdown path (verify wiring still good post-PR-AR-1)
- [ ] No new TODOs without a `PR-AR-N` reference
- [ ] `internal/agent/handler_test.go` no longer has the old PR-AR-1 echo test (deleted)
- [ ] Manual smoke per `internal/agent/doc.go` — set `OPEN_AI_KEY` env, dial WS, send `@agent hi`, expect a real assistant reply

## Risk notes

| Risk | Mitigation |
|---|---|
| Pantheon `Conversation.Continue` requires last chat state == Interrupt; if reader processes a stale `awaitingFeedback`, `Continue` returns `ErrNoChatToContinue` | Bridge's interrupt goroutine consumes the next feedback msg exactly once; if frame arrives before interrupt fires it's discarded with a warning log |
| `OnInterrupt` goroutine + main goroutine racing on `Continue` | Continue is sync; `Run` and `Continue` cannot overlap because Start blocks until runLoop returns, and Continue re-enters runLoop. We launch the Continue caller in a new goroutine after OnInterrupt, so OnInterrupt itself is non-blocking |
| Mock test flakiness from goroutine scheduling | Use `eventually` polling helpers (testify) with 2s timeout; tests are channel-driven not sleep-driven |
| `core.LanguageModel` cache stale after settings change | PR-AR-2 caches by `provider:model`; admin updates to API key require server restart to take effect. Document; PR-AR-5 adds invalidation. |
| `ServerFrame` `omitempty` on `Type` could break chat-only frames if `From==""` — JSON emits `{}` | Verified in bridge_test: chat frames always have non-empty From; we assert `f.From != ""` before sending |
| Slow client buffer overflow returns `ErrSlowReader`, currently silently dropped by `bridge` | Add a single `Abort("slow reader")` if `Send` returns `ErrSlowReader` — fast-fail rather than silently lose frames |

## Estimate

| Task | Hours |
|---|---|
| 1. LLM factory + system prompt + test override | 2.0 |
| 2. wsConn wrapper | 1.5 |
| 3. Session + event bridges | 2.5 |
| 4. Reader loop (bail + feedback) | 1.5 |
| 5. HandleWS rewrite + Shutdown + e2e | 2.5 |
| **Total** | **10.0** (design estimate 8-10h, top of range ✓) |

—— end of plan
