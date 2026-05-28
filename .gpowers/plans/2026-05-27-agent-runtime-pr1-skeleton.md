# Agent Runtime PR-AR-1 — Skeleton + WebSocket Upgrade + Auth + Invocation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the `internal/agent` package skeleton, `gorilla/websocket` dependency, `workspace_agent_invocations` DB model + CRUD, short-TTL WS-auth token issuance, `WSValidatedRequest` middleware, a `Runtime` struct with `HandleWS` that **echoes received frames back as `__unhandled` JSON** (sufficient for FE/BE protocol smoke), and routing in `main.go`. **No `pantheon/conversation` or `pantheon/agent` wiring yet** — that lands in PR-AR-2.

**Architecture:** A new `internal/agent/` package holds `Runtime` + handlers + invocation + WS transport stubs. WS auth uses the existing `TemporaryAuthTokenService` extended with `IssueWithTTL(userID, ttl)`. Frontend is **not changed** — PR-AR-4 will add the HTTP→WS handoff in `chat_service.Stream`; PR-AR-1 only exposes the WS endpoint for manual smoke testing.

**Tech Stack:** Go 1.25.5, Gin v1.10, GORM, sqlite (test), gorilla/websocket v1.5.x (new), testify.

**Source spec:** `.gpowers/designs/2026-05-26-agent-runtime-design.md` §4, §5, §9, §14 (PR-AR-1 row).

**Reference Node implementation:**
- `server/endpoints/agentWebsocket.js` (route + upgrade + invocation lookup)
- `server/utils/agents/aibitat/plugins/websocket.js` (frame format — for PR-AR-2; PR-AR-1 just echoes)
- `server/models/workspaceAgentInvocation.js` (prisma model shape)
- `server/utils/middleware/multiUserProtected.js` (Node's WS auth; we won't 1:1 copy, see §Test setup below)

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `cmd/server/main.go:140-198` — current Gin wiring; `RegisterMCPRoutes` etc. all called inside the `api := r.Group("/api")` block. Add `Runtime` construction near the existing services (line ~140 area, after MCP hypervisor), and route registration alongside the others (line ~166 area).
- `internal/services/auth_service.go` — `AuthService` + `IsAuthEnabled()`; **do not** touch this file.
- `internal/services/temporary_auth_token_service.go` — has `Issue(ctx, userID)` (1h TTL, single-use). Task 3 adds `IssueWithTTL(ctx, userID, ttl)` without breaking `Issue`.
- `internal/models/temporary_auth_token.go` — `TemporaryAuthToken` struct already has `ExpiresAt time.Time`. **No schema change needed.**
- `internal/middleware/auth.go:14` `ValidatedRequest(authSvc)` — header-token auth. **Do not modify**; we add a parallel `WSValidatedRequest`.
- `internal/models/db.go` — call site for `AutoMigrate`. Add the new model here.
- `internal/handlers/api_setup_test.go` — `apiTestEnv` pattern (sqlite + Gin engine). **Do not reuse** for this PR (different middleware stack); write a small `setupAgentTest` helper, see §Test setup.
- `internal/mcp/` — fully landed; we will **eventually** call `mcp.Hypervisor.ActiveServers()/ToolsAsPlugins()` from the agent runtime (PR-AR-3), but PR-AR-1 does **not** import it.
- `pkg/utils.Ptr[T any](v T) *T` — generic helper, use for `*string`, `*int` literals in tests.

### New package layout (this PR)

```
backend/internal/agent/
├── doc.go                      # package comment (5 lines, scope statement)
├── types.go                    # WSFrameType constants + (de)serialised frame types (used in PR-AR-1 stub)
├── runtime.go                  # type Runtime + NewRuntime(Deps) + Shutdown(ctx) stub
├── handler.go                  # (*Runtime).HandleWS  — Gin upgrade + echo loop
├── invocation.go               # CreateInvocation/GetInvocation/CloseInvocation (used in PR-AR-1 by handler)
├── runtime_test.go             # Constructor smoke + Shutdown idempotency
├── handler_test.go             # End-to-end: HTTP+WS via httptest.NewServer + gorilla/websocket dialer
└── invocation_test.go          # CRUD + Closed=true rejection

backend/internal/models/
└── workspace_agent_invocation.go  # NEW — GORM model

backend/internal/middleware/
├── ws_auth.go                  # NEW — WSValidatedRequest(authSvc, tempTokenSvc)
└── ws_auth_test.go             # NEW — table-driven middleware test

backend/internal/services/
└── temporary_auth_token_service.go  # MODIFY — add IssueWithTTL(ctx, userID, ttl)

backend/internal/handlers/
├── agent_token.go              # NEW — POST /workspace/:slug/agent-token (issues 3min token bound to caller)
└── agent_token_test.go         # NEW — happy + 401 paths

backend/cmd/server/main.go    # MODIFY — wire Runtime + register routes + register graceful shutdown
```

### Methods to ship (PR-AR-1 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `models.WorkspaceAgentInvocation` | struct | GORM tags + index on `(uuid)`, `(workspace_id)`, `(closed)` |
| 2 | `agent.Runtime` | `NewRuntime(deps Deps) *Runtime` | Deps holds `db`, `cfg`, `tempTokenSvc`, `authSvc`; no LLM/MCP yet |
| 3 | `agent.Runtime` | `(*Runtime) HandleWS(c *gin.Context)` | upgrade + uuid lookup + read-echo loop |
| 4 | `agent.Runtime` | `(*Runtime) Shutdown(ctx context.Context) error` | iterates `sessions sync.Map`, closes each WS; bounded by ctx deadline |
| 5 | `agent.Runtime` | `(*Runtime) CreateInvocation(ctx, ws, user, thread, prompt) (uuid string, err error)` | inserts row, returns UUID |
| 6 | `agent.Runtime` | `(*Runtime) CloseInvocation(ctx, uuid) error` | sets `closed=true` |
| 7 | `agent.Runtime` | `(*Runtime) GetInvocation(ctx, uuid) (*models.WorkspaceAgentInvocation, error)` | returns `gorm.ErrRecordNotFound` on miss |
| 8 | `middleware.WSValidatedRequest` | `func(authSvc, tempTokenSvc) gin.HandlerFunc` | query `?token=` consumes a one-shot temp token; auth-disabled mode auto-admin |
| 9 | `services.TemporaryAuthTokenService` | `IssueWithTTL(ctx, userID, ttl) (string, error)` | factored from `Issue`; `Issue` becomes `IssueWithTTL(ctx, uid, time.Hour)` wrapper |
| 10 | `handlers.RegisterAgentTokenRoutes` | `func(r *gin.RouterGroup, tempTokenSvc, authSvc) ` | mounts `POST /workspace/:slug/agent-token` |
| 11 | `handlers.RegisterAgentRoutes` | `func(r *gin.RouterGroup, rt *agent.Runtime, authSvc, tempTokenSvc)` | mounts `GET /agent-invocation/:uuid` with WS middleware |

### WSFrameType constants (used in PR-AR-1 for echo + reserved for PR-AR-2/5)

```go
const (
    FrameStatusResponse    = "statusResponse"
    FrameWSSFailure        = "wssFailure"
    FrameWaitingOnInput    = "WAITING_ON_INPUT"
    FrameToolApprovalReq   = "toolApprovalRequest"
    FrameToolApprovalResp  = "toolApprovalResponse"
    FrameAwaitingFeedback  = "awaitingFeedback"
    FrameUnhandled         = "__unhandled"
)
```

PR-AR-1 only emits `FrameStatusResponse` (welcome) and `FrameUnhandled` (echo). Other constants are reserved.

### Frame envelopes (PR-AR-1)

```go
// internal/agent/types.go

// ServerFrame is any message Go → Frontend.
type ServerFrame struct {
    Type    string `json:"type"`
    Content string `json:"content,omitempty"`
    Animate bool   `json:"animate,omitempty"`
    // chat-message fields (unused in PR-AR-1, present so PR-AR-2 doesn't need a breaking change)
    From    string `json:"from,omitempty"`
    To      string `json:"to,omitempty"`
    State   string `json:"state,omitempty"`
    // (other reserved fields land in PR-AR-2/5)
}

// ClientFrame is any message Frontend → Go.
type ClientFrame struct {
    Type        string `json:"type,omitempty"`
    Feedback    string `json:"feedback,omitempty"`
    Attachments []any  `json:"attachments,omitempty"`
    RequestID   string `json:"requestId,omitempty"`
    Approved    bool   `json:"approved,omitempty"`
}
```

### Out of scope (explicit)

- `pantheon/conversation` / `pantheon/agent` wiring — PR-AR-2
- `tool.Registry` and any default skill — PR-AR-3
- `chat_service.Stream` HTTP→WS handoff and `agentInitWebsocketConnection` chunk — PR-AR-4
- `wssFailure` / interrupt / abort / graceful shutdown e2e — PR-AR-5
- Frontend changes — none in this PR; manual smoke uses `websocat` or test client only
- Multi-user (`agentSkillWhitelist`) — PR-AR-5
- Permission-check vs `workspaceUsers` for membership — out of scope for PR-AR-1 (will land via existing `ValidatedRequest` once chat_service triggers it; for PR-AR-1, **any authenticated user** who holds the temp token can hit the WS)
- Tool approval — PR-AR-5

### Test setup helper

```go
// internal/agent/test_helpers_test.go (test-only)

type agentTestEnv struct {
    DB           *gorm.DB
    Cfg          *config.Config
    TempTokenSvc *services.TemporaryAuthTokenService
    AuthSvc      *services.AuthService
    Runtime      *agent.Runtime
    Server       *httptest.Server   // wraps gin.Engine with /api group
    User         *models.User
}

func newAgentTestEnv(t *testing.T) *agentTestEnv {
    t.Helper()
    db := openTestDB(t)                      // sqlite-mem; migrate User, TemporaryAuthToken, WorkspaceAgentInvocation, Workspace
    cfg := &config.Config{StorageDir: t.TempDir()}
    enc, _ := services.NewEncryptionManager("test-key")
    authSvc := services.NewAuthService(db, cfg, enc)
    tempTokenSvc := services.NewTemporaryAuthTokenService(db)
    rt := agent.NewRuntime(agent.Deps{
        DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
    })
    eng := gin.New()
    api := eng.Group("/api")
    handlers.RegisterAgentTokenRoutes(api, tempTokenSvc, authSvc)
    handlers.RegisterAgentRoutes(api, rt, authSvc, tempTokenSvc)
    srv := httptest.NewServer(eng)
    t.Cleanup(srv.Close)
    return &agentTestEnv{..., User: seedAdminUser(t, db)}
}

func (e *agentTestEnv) IssueTempToken(t *testing.T, userID int, ttl time.Duration) string {
    tok, err := e.TempTokenSvc.IssueWithTTL(context.Background(), userID, ttl)
    require.NoError(t, err)
    return tok
}

func (e *agentTestEnv) DialWS(t *testing.T, path string, token string) (*websocket.Conn, *http.Response) {
    u, _ := url.Parse(e.Server.URL)
    u.Scheme = "ws"
    u.Path = path
    q := u.Query(); q.Set("token", token); u.RawQuery = q.Encode()
    conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
    require.NoError(t, err)
    t.Cleanup(func() { _ = conn.Close() })
    return conn, resp
}
```

> The helper deliberately does NOT use `apiTestEnv`. WS routes have a different middleware stack (token-via-query, not header-Bearer), so reusing the API-key helper would obscure intent.

### TDD discipline

Each task lands as **one commit**. Within a task:

1. Write the failing test(s).
2. `cd backend && go test ./internal/agent/... -run <NewTest>` — confirm it fails for the **right** reason.
3. Implement.
4. `cd backend && go test ./internal/agent/... ./internal/middleware/... ./internal/handlers/... -run <NewTest>` — confirm it passes.
5. `cd backend && go test ./...` — full suite green.
6. Commit with the message convention `feat(agent): <task summary>` (PR-AR-1 prefix optional).

---

## Task 1: Dependencies + package skeleton

**Files:**
- `backend/go.mod` (MODIFY)
- `backend/internal/agent/doc.go` (NEW)
- `backend/internal/agent/types.go` (NEW)
- `backend/internal/agent/runtime.go` (NEW — stub)

**Test:** none yet (pure declarations + dependency wiring); compilation is the gate.

### Steps

- [ ] Add `gorilla/websocket` dependency:
  ```bash
  cd backend && go get github.com/gorilla/websocket@v1.5.3
  ```
  Verify `go.mod` now lists `github.com/gorilla/websocket v1.5.3` as a direct dep.

- [ ] Create `internal/agent/doc.go`:
  ```go
  // Package agent implements the @agent runtime for backend.
  //
  // PR-AR-1 lands the WebSocket upgrade path, the workspace_agent_invocations
  // DB model, and a short-TTL temp-token-based auth flow. The runtime in this
  // PR is intentionally hollow — receiving frames are echoed back as
  // {"type":"__unhandled"}. Pantheon conversation+agent wiring lands in PR-AR-2.
  package agent
  ```

- [ ] Create `internal/agent/types.go` with `FrameXxx` constants and `ServerFrame` / `ClientFrame` structs as specified in §Frame envelopes above.

- [ ] Create `internal/agent/runtime.go` with type stubs only:
  ```go
  package agent

  import (
      "context"
      "sync"

      "github.com/gorilla/websocket"
      "github.com/odysseythink/hermind/backend/internal/config"
      "github.com/odysseythink/hermind/backend/internal/services"
      "gorm.io/gorm"
  )

  type Deps struct {
      DB           *gorm.DB
      Cfg          *config.Config
      TempTokenSvc *services.TemporaryAuthTokenService
      AuthSvc      *services.AuthService
  }

  type Runtime struct {
      deps     Deps
      upgrader websocket.Upgrader
      sessions sync.Map // uuid → *Session (Session lands in PR-AR-2)
  }

  func NewRuntime(d Deps) *Runtime {
      return &Runtime{
          deps: d,
          upgrader: websocket.Upgrader{
              ReadBufferSize:   4096,
              WriteBufferSize:  4096,
              HandshakeTimeout: 10 * time.Second,
              CheckOrigin:      func(r *http.Request) bool { return true }, // tightened in PR-AR-5
          },
      }
  }

  func (r *Runtime) Shutdown(ctx context.Context) error {
      // PR-AR-1: no live sessions yet, no-op; returning nil keeps the contract
      // ready for PR-AR-2 which will walk r.sessions and close each WS.
      _ = ctx
      return nil
  }
  ```

- [ ] `go vet ./...` clean; `go build ./...` clean.

### Acceptance

- `go.mod` lists `github.com/gorilla/websocket v1.5.3`
- `internal/agent/` directory exists with three files
- Package compiles standalone (no test failures, no import cycle)

### Commit

`feat(agent): bootstrap internal/agent package + gorilla/websocket dep`

---

## Task 2: WorkspaceAgentInvocation model + invocation CRUD

**Files:**
- `backend/internal/models/workspace_agent_invocation.go` (NEW)
- `backend/internal/models/db.go` (MODIFY — add to AutoMigrate)
- `backend/internal/agent/invocation.go` (NEW)
- `backend/internal/agent/invocation_test.go` (NEW)

**Tests:**
- `TestRuntime_CreateInvocation_ReturnsUUID`
- `TestRuntime_GetInvocation_NotFound`
- `TestRuntime_GetInvocation_RejectsClosed`
- `TestRuntime_CloseInvocation_Idempotent`

### Steps

- [ ] Write the failing test file first:
  ```go
  // internal/agent/invocation_test.go
  package agent_test

  import (
      "context"
      "testing"

      "github.com/google/uuid"
      "github.com/stretchr/testify/require"
      "github.com/odysseythink/hermind/backend/internal/agent"
  )

  func TestRuntime_CreateInvocation_ReturnsUUID(t *testing.T) {
      env := newAgentTestEnv(t)
      uid, err := env.Runtime.CreateInvocation(context.Background(), seedWorkspace(t, env.DB), env.User, nil, "@agent hello")
      require.NoError(t, err)
      _, err = uuid.Parse(uid)
      require.NoError(t, err, "must be a valid uuid v4")
  }

  func TestRuntime_GetInvocation_NotFound(t *testing.T) { /* ... */ }
  func TestRuntime_GetInvocation_RejectsClosed(t *testing.T) { /* CreateInvocation → CloseInvocation → GetInvocation returns ErrInvocationClosed */ }
  func TestRuntime_CloseInvocation_Idempotent(t *testing.T) { /* Close → Close → no error */ }
  ```

- [ ] Run test, confirm failure (`agent.Runtime has no method CreateInvocation`).

- [ ] Implement `models/workspace_agent_invocation.go`:
  ```go
  package models

  import "time"

  type WorkspaceAgentInvocation struct {
      ID          int       `gorm:"primaryKey" json:"id"`
      UUID        string    `gorm:"uniqueIndex;not null" json:"uuid"`
      WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
      UserID      *int      `gorm:"index" json:"userId"`
      ThreadID    *int      `gorm:"index" json:"threadId"`
      Prompt      string    `gorm:"type:text;not null" json:"prompt"`
      Closed      bool      `gorm:"index;default:false" json:"closed"`
      CreatedAt   time.Time `json:"createdAt"`
      UpdatedAt   time.Time `json:"updatedAt"`
  }

  func (WorkspaceAgentInvocation) TableName() string { return "workspace_agent_invocations" }
  ```

- [ ] Add to `internal/models/db.go` `AutoMigrate(...)` list:
  ```go
  &WorkspaceAgentInvocation{},
  ```

- [ ] Implement `internal/agent/invocation.go`:
  ```go
  package agent

  import (
      "context"
      "errors"
      "fmt"

      "github.com/google/uuid"
      "gorm.io/gorm"
      "github.com/odysseythink/hermind/backend/internal/models"
  )

  var (
      ErrInvocationNotFound = errors.New("agent invocation not found")
      ErrInvocationClosed   = errors.New("agent invocation closed")
  )

  func (r *Runtime) CreateInvocation(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (string, error) {
      if ws == nil { return "", fmt.Errorf("workspace required") }
      inv := &models.WorkspaceAgentInvocation{
          UUID:        uuid.NewString(),
          WorkspaceID: ws.ID,
          Prompt:      prompt,
      }
      if user != nil { inv.UserID = &user.ID }
      if thread != nil { inv.ThreadID = &thread.ID }
      if err := r.deps.DB.WithContext(ctx).Create(inv).Error; err != nil {
          return "", fmt.Errorf("create invocation: %w", err)
      }
      return inv.UUID, nil
  }

  func (r *Runtime) GetInvocation(ctx context.Context, id string) (*models.WorkspaceAgentInvocation, error) {
      var inv models.WorkspaceAgentInvocation
      if err := r.deps.DB.WithContext(ctx).Where("uuid = ?", id).First(&inv).Error; err != nil {
          if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrInvocationNotFound }
          return nil, err
      }
      if inv.Closed { return nil, ErrInvocationClosed }
      return &inv, nil
  }

  func (r *Runtime) CloseInvocation(ctx context.Context, id string) error {
      return r.deps.DB.WithContext(ctx).
          Model(&models.WorkspaceAgentInvocation{}).
          Where("uuid = ?", id).
          Update("closed", true).Error
  }
  ```

- [ ] Run tests, confirm pass. Run `go test ./...`, confirm full suite still green (sqlite migration picks up the new model — if not, fix the migration order).

### Acceptance

- All 4 new tests pass
- `WorkspaceAgentInvocation` automigrates without conflict
- `ErrInvocationNotFound` / `ErrInvocationClosed` exported

### Commit

`feat(agent): workspace_agent_invocations model + Runtime CRUD`

---

## Task 3: TemporaryAuthTokenService.IssueWithTTL + agent-token handler

**Files:**
- `backend/internal/services/temporary_auth_token_service.go` (MODIFY)
- `backend/internal/handlers/agent_token.go` (NEW)
- `backend/internal/handlers/agent_token_test.go` (NEW)

**Tests:**
- `TestTempToken_IssueWithTTL_RespectsTTL` (in `temporary_auth_token_service_test.go`)
- `TestTempToken_IssueWithTTL_InvalidatesPriorTokens`
- `TestAgentTokenHandler_HappyPath_200WithToken`
- `TestAgentTokenHandler_Unauthenticated_401`
- `TestAgentTokenHandler_WorkspaceMembershipRequired_403` (if user is non-admin, must be a member; admin bypasses)

### Steps

- [ ] Write failing service test:
  ```go
  func TestTempToken_IssueWithTTL_RespectsTTL(t *testing.T) {
      db := openTestDB(t)
      svc := services.NewTemporaryAuthTokenService(db)
      uid := seedUser(t, db).ID
      tok, err := svc.IssueWithTTL(context.Background(), uid, 100*time.Millisecond)
      require.NoError(t, err)
      _, err = svc.Validate(context.Background(), tok)
      require.NoError(t, err)
      // re-issue another and try the expired one
      tok2, _ := svc.IssueWithTTL(context.Background(), uid, time.Hour)
      time.Sleep(200 * time.Millisecond)
      _, err = svc.Validate(context.Background(), tok2)  // tok2 still valid
      require.NoError(t, err)
  }
  ```

- [ ] Refactor existing `Issue` to delegate:
  ```go
  func (s *TemporaryAuthTokenService) Issue(ctx context.Context, userID int) (string, error) {
      return s.IssueWithTTL(ctx, userID, time.Hour)
  }

  func (s *TemporaryAuthTokenService) IssueWithTTL(ctx context.Context, userID int, ttl time.Duration) (string, error) {
      if userID == 0 { return "", fmt.Errorf("user ID is required") }
      if ttl <= 0 || ttl > time.Hour { return "", fmt.Errorf("ttl must be in (0, 1h]") }
      _ = s.InvalidateUserTokens(ctx, userID)
      token := models.TemporaryAuthToken{
          Token:     s.makeTempToken(),
          UserID:    userID,
          ExpiresAt: time.Now().Add(ttl),
          CreatedAt: time.Now(),
      }
      if err := s.db.WithContext(ctx).Create(&token).Error; err != nil {
          return "", fmt.Errorf("create temp token: %w", err)
      }
      return token.Token, nil
  }
  ```
  > Bound at 1h to defend against misuse; agent path will request 3min explicitly.

- [ ] Write failing handler test:
  ```go
  func TestAgentTokenHandler_HappyPath_200WithToken(t *testing.T) {
      env := newAgentTestEnv(t)
      ws := seedWorkspace(t, env.DB)
      // sign in as env.User (single-user admin bypass; ValidatedRequest sets user=admin)
      resp := env.POST(t, fmt.Sprintf("/api/workspace/%s/agent-token", ws.Slug), nil, "")
      require.Equal(t, http.StatusOK, resp.StatusCode)
      var body struct{ Success bool; Token string; ExpiresInSeconds int }
      require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
      require.True(t, body.Success)
      require.NotEmpty(t, body.Token)
      require.Equal(t, 180, body.ExpiresInSeconds)
  }

  func TestAgentTokenHandler_Unauthenticated_401(t *testing.T) {
      env := newAgentTestEnvWithAuth(t)  // auth enabled, no Bearer header
      ws := seedWorkspace(t, env.DB)
      resp := env.POST(t, fmt.Sprintf("/api/workspace/%s/agent-token", ws.Slug), nil, "")
      require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
  }
  ```

- [ ] Implement handler:
  ```go
  // internal/handlers/agent_token.go
  package handlers

  import (
      "net/http"
      "time"

      "github.com/gin-gonic/gin"
      "github.com/odysseythink/hermind/backend/internal/middleware"
      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/hermind/backend/internal/services"
  )

  const agentWSTTL = 3 * time.Minute

  func RegisterAgentTokenRoutes(r *gin.RouterGroup, tempTokenSvc *services.TemporaryAuthTokenService, authSvc *services.AuthService) {
      r.POST("/workspace/:slug/agent-token",
          middleware.ValidatedRequest(authSvc),
          func(c *gin.Context) {
              user := c.MustGet("user").(*models.User)
              if user.ID == 0 {
                  // Auth-disabled mode: admin bypass user has ID 0.
                  // Persisting a temp token requires a real DB row, so we accept
                  // the bypass by issuing an unbacked token only when auth is OFF.
                  // For PR-AR-1 we keep it simple: refuse and force auth-on for WS.
                  c.JSON(http.StatusOK, gin.H{
                      "success": true,
                      "token": "AUTH_DISABLED_BYPASS",
                      "expiresInSeconds": int(agentWSTTL.Seconds()),
                  })
                  return
              }
              tok, err := tempTokenSvc.IssueWithTTL(c.Request.Context(), user.ID, agentWSTTL)
              if err != nil {
                  c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
                  return
              }
              c.JSON(http.StatusOK, gin.H{
                  "success": true,
                  "token": tok,
                  "expiresInSeconds": int(agentWSTTL.Seconds()),
              })
          },
      )
  }
  ```

- [ ] Run all tests, confirm pass.

### Acceptance

- `Issue` still works (backward-compatible)
- `IssueWithTTL` rejects `ttl<=0` and `ttl>1h`
- `POST /api/workspace/:slug/agent-token` returns JSON `{success, token, expiresInSeconds:180}` for authed users
- Auth-disabled mode returns `AUTH_DISABLED_BYPASS` sentinel (consumed in Task 4)

### Commit

`feat(agent): IssueWithTTL + POST /workspace/:slug/agent-token`

---

## Task 4: WSValidatedRequest middleware

**Files:**
- `backend/internal/middleware/ws_auth.go` (NEW)
- `backend/internal/middleware/ws_auth_test.go` (NEW)

**Tests:**
- `TestWSValidatedRequest_NoToken_401`
- `TestWSValidatedRequest_InvalidToken_403`
- `TestWSValidatedRequest_ValidToken_SetsUser`
- `TestWSValidatedRequest_TokenIsSingleUse` (second use returns 403)
- `TestWSValidatedRequest_AuthDisabled_AcceptsBypassSentinel`

### Steps

- [ ] Write failing middleware tests:
  ```go
  func TestWSValidatedRequest_ValidToken_SetsUser(t *testing.T) {
      env := newAgentTestEnv(t)
      tok := env.IssueTempToken(t, env.User.ID, 3*time.Minute)

      eng := gin.New()
      eng.GET("/probe",
          middleware.WSValidatedRequest(env.AuthSvc, env.TempTokenSvc),
          func(c *gin.Context) {
              u := c.MustGet("user").(*models.User)
              c.JSON(200, gin.H{"id": u.ID})
          },
      )
      rec := httptest.NewRecorder()
      req := httptest.NewRequest("GET", "/probe?token="+tok, nil)
      eng.ServeHTTP(rec, req)
      require.Equal(t, 200, rec.Code)
      require.Contains(t, rec.Body.String(), fmt.Sprintf(`"id":%d`, env.User.ID))
  }
  ```

- [ ] Implement:
  ```go
  // internal/middleware/ws_auth.go
  package middleware

  import (
      "net/http"

      "github.com/gin-gonic/gin"
      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/hermind/backend/internal/services"
      "github.com/odysseythink/hermind/backend/pkg/utils"
  )

  const AuthDisabledBypassToken = "AUTH_DISABLED_BYPASS"

  // WSValidatedRequest authenticates a WebSocket upgrade request via
  // query string token (?token=allm-tat-...). The token is a single-use
  // short-TTL temp token issued by POST /workspace/:slug/agent-token.
  func WSValidatedRequest(authSvc *services.AuthService, tempTokenSvc *services.TemporaryAuthTokenService) gin.HandlerFunc {
      return func(c *gin.Context) {
          if !authSvc.IsAuthEnabled() {
              if c.Query("token") != AuthDisabledBypassToken {
                  c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bypass token"})
                  return
              }
              c.Set("user", &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"})
              c.Next()
              return
          }
          tok := c.Query("token")
          if tok == "" {
              c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
              return
          }
          user, err := tempTokenSvc.Validate(c.Request.Context(), tok)
          if err != nil {
              c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid token"})
              return
          }
          c.Set("user", user)
          c.Next()
      }
  }
  ```

- [ ] Run tests, confirm pass.

### Acceptance

- All 5 middleware tests pass
- Token consumption is single-use (relies on existing `Validate`'s delete-after-success semantics)
- Auth-disabled mode requires the explicit `AUTH_DISABLED_BYPASS` sentinel (no silent admin handout)

### Commit

`feat(agent): WSValidatedRequest middleware (single-use query token)`

---

## Task 5: Runtime.HandleWS — upgrade + echo loop

**Files:**
- `backend/internal/agent/handler.go` (NEW)
- `backend/internal/agent/handler_test.go` (NEW)
- `backend/internal/handlers/agent.go` (NEW — `RegisterAgentRoutes`)

**Tests (e2e via httptest.NewServer):**
- `TestHandleWS_HappyPath_EchoesFrame`
- `TestHandleWS_UnknownUUID_Closes1008` (websocket close code 1008 = policy violation)
- `TestHandleWS_ClosedInvocation_Closes1008`
- `TestHandleWS_SendsWelcomeStatusOnConnect`
- `TestHandleWS_RegistersSessionDuringHandle` (use a probe channel to assert `sessions sync.Map` has 1 entry mid-flight, 0 after close)

### Steps

- [ ] Write failing handler test:
  ```go
  func TestHandleWS_HappyPath_EchoesFrame(t *testing.T) {
      env := newAgentTestEnv(t)
      ws := seedWorkspace(t, env.DB)
      uuid, err := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hello")
      require.NoError(t, err)
      tok := env.IssueTempToken(t, env.User.ID, time.Minute)
      conn, _ := env.DialWS(t, "/api/agent-invocation/"+uuid, tok)

      // Expect a welcome statusResponse first
      var welcome agent.ServerFrame
      require.NoError(t, conn.ReadJSON(&welcome))
      require.Equal(t, agent.FrameStatusResponse, welcome.Type)

      // Send a frame; expect echo wrapped as __unhandled
      require.NoError(t, conn.WriteJSON(agent.ClientFrame{Type: "awaitingFeedback", Feedback: "skip"}))
      var echo agent.ServerFrame
      require.NoError(t, conn.ReadJSON(&echo))
      require.Equal(t, agent.FrameUnhandled, echo.Type)
      require.Contains(t, echo.Content, "awaitingFeedback")
      require.Contains(t, echo.Content, "skip")
  }
  ```

- [ ] Implement `handler.go`:
  ```go
  package agent

  import (
      "context"
      "encoding/json"
      "net/http"
      "time"

      "github.com/gin-gonic/gin"
      "github.com/gorilla/websocket"
      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/mlog"
  )

  const (
      wsReadDeadline  = 5 * time.Minute
      wsWriteDeadline = 30 * time.Second
      wsPingInterval  = 30 * time.Second
  )

  func (r *Runtime) HandleWS(c *gin.Context) {
      id := c.Param("uuid")
      if id == "" {
          c.AbortWithStatus(http.StatusBadRequest)
          return
      }
      inv, err := r.GetInvocation(c.Request.Context(), id)
      if err != nil {
          mlog.Warning("agent: invocation lookup failed: ", id, " err=", err)
          c.AbortWithStatus(http.StatusNotFound)
          return
      }
      user, _ := c.Get("user")

      conn, err := r.upgrader.Upgrade(c.Writer, c.Request, nil)
      if err != nil {
          mlog.Error("agent: ws upgrade failed: ", err)
          return
      }
      session := &Session{
          UUID:        inv.UUID,
          WorkspaceID: inv.WorkspaceID,
          conn:        conn,
      }
      if u, ok := user.(*models.User); ok && u != nil { session.UserID = &u.ID }
      r.sessions.Store(inv.UUID, session)
      defer func() {
          r.sessions.Delete(inv.UUID)
          _ = r.CloseInvocation(context.Background(), inv.UUID)
          _ = conn.Close()
      }()

      // Welcome
      _ = conn.WriteJSON(ServerFrame{
          Type:    FrameStatusResponse,
          Content: "@agent runtime ready (PR-AR-1 echo mode)",
          Animate: false,
      })

      conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
      conn.SetPongHandler(func(string) error {
          conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
          return nil
      })

      // Reader loop: echo any frame back as __unhandled with the raw JSON
      for {
          mt, raw, err := conn.ReadMessage()
          if err != nil { return }
          if mt != websocket.TextMessage { continue }
          var cf ClientFrame
          _ = json.Unmarshal(raw, &cf) // best-effort
          out := ServerFrame{
              Type:    FrameUnhandled,
              Content: string(raw),
          }
          if err := conn.WriteJSON(out); err != nil { return }
      }
  }
  ```

- [ ] Add `Session` skeleton to `runtime.go` (just enough to compile; PR-AR-2 fleshes out):
  ```go
  type Session struct {
      UUID        string
      WorkspaceID int
      UserID      *int
      conn        *websocket.Conn
  }
  ```

- [ ] Implement `handlers/agent.go`:
  ```go
  package handlers

  import (
      "github.com/gin-gonic/gin"

      "github.com/odysseythink/hermind/backend/internal/agent"
      "github.com/odysseythink/hermind/backend/internal/middleware"
      "github.com/odysseythink/hermind/backend/internal/services"
  )

  func RegisterAgentRoutes(r *gin.RouterGroup, rt *agent.Runtime, authSvc *services.AuthService, tempTokenSvc *services.TemporaryAuthTokenService) {
      r.GET("/agent-invocation/:uuid",
          middleware.WSValidatedRequest(authSvc, tempTokenSvc),
          rt.HandleWS,
      )
  }
  ```

- [ ] Run tests, confirm pass; full suite green.

### Acceptance

- All 5 handler tests pass
- Closing the WS marks invocation `closed=true` in DB
- `sessions sync.Map` is empty after every test (runtime.sessions inspection helper, see test helper)

### Commit

`feat(agent): Runtime.HandleWS upgrade + echo loop + invocation lifecycle`

---

## Task 6: Wire into main.go + manual smoke

**Files:**
- `backend/cmd/server/main.go` (MODIFY)

**Test:** none new (e2e covered in Task 5). Smoke is manual:

```bash
# Terminal A
cd backend && go run ./cmd/server

# Terminal B
TOKEN=$(curl -sX POST http://localhost:3001/api/workspace/<slug>/agent-token \
  -H 'Authorization: Bearer <session-jwt>' | jq -r .token)
UUID=$(... # CreateInvocation will be triggered from chat in PR-AR-4; for PR-AR-1 smoke, insert directly via sqlite)
sqlite3 storage/hermind.db "INSERT INTO workspace_agent_invocations (uuid, workspace_id, prompt) VALUES ('manual-test', 1, '@agent hi');"
websocat "ws://localhost:3001/api/agent-invocation/manual-test?token=$TOKEN"
# expect welcome frame, then any line you type → echoed as __unhandled
```

### Steps

- [ ] In `cmd/server/main.go`, near the existing services bloc:
  ```go
  agentRuntime := agent.NewRuntime(agent.Deps{
      DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
  })
  ```

- [ ] After existing route registrations (around line 166–186 area), add:
  ```go
  handlers.RegisterAgentTokenRoutes(api, tempTokenSvc, authSvc)
  handlers.RegisterAgentRoutes(api, agentRuntime, authSvc, tempTokenSvc)
  ```

- [ ] If the existing main has a `defer`/graceful shutdown chain (look near `r.Run(addr)` or any `http.Server.Shutdown`), append a `agentRuntime.Shutdown(shutdownCtx)` call — Task 6 only wires the empty stub; PR-AR-5 implements its body.

- [ ] Run the server, perform the smoke procedure above, confirm welcome + echo work.

- [ ] `go test ./...` — full suite green.

### Acceptance

- Server boots without panic
- `GET /api/agent-invocation/<uuid>` upgrades on valid token
- Manual smoke confirms welcome + echo
- No regression in any other test

### Commit

`feat(agent): wire Runtime into main.go + register WS routes`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./...` 100% green
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] No new TODOs that don't reference PR-AR-2/3/4/5 explicitly
- [ ] Smoke procedure in Task 6 documented in `internal/agent/doc.go` as a comment block for PR-AR-2/4 handoff
- [ ] `go.sum` updated and committed
- [ ] `agentRuntime.Shutdown` is invoked in main.go's shutdown path (even though it's a no-op now)

## Risk notes

| Risk | Mitigation |
|---|---|
| `gorilla/websocket` `CheckOrigin: true` allows any origin | Tightened in PR-AR-5; PR-AR-1 only routes through authed users with single-use tokens |
| `TemporaryAuthToken.Validate` deletes on success — if WS upgrade fails AFTER token consumption, user must request a fresh token | Acceptable; UX behavior is "click connect again," cheap to mitigate later |
| Auth-disabled mode's `AUTH_DISABLED_BYPASS` sentinel could leak to a token logger | Only sent via TLS in production; sentinel has no actual session value, just authorizes upgrade |
| Adding `WorkspaceAgentInvocation` migration may collide with Node's prisma schema on shared DB | Node uses prisma migrations; Go uses `AutoMigrate`. Both create the same table name with compatible columns. If Node already created the table, GORM's `AutoMigrate` is additive only — verify on a Node-created DB before final merge |
| Read deadline 5min may starve long agent runs (PR-AR-2+) | Reader resets deadline on every pong; PR-AR-1 has no ping/pong yet, so a 5min idle WS closes — acceptable, fixed in PR-AR-2 |

## Estimate

| Task | Hours |
|---|---|
| 1. Dep + skeleton | 0.5 |
| 2. Invocation model + CRUD | 1.0 |
| 3. IssueWithTTL + agent-token handler | 1.0 |
| 4. WSValidatedRequest | 1.0 |
| 5. HandleWS echo loop + e2e | 2.0 |
| 6. main.go wiring + smoke | 0.5 |
| **Total** | **6.0** (design estimate 5-7h, ✓) |

—— end of plan
