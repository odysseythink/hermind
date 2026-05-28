# API v1 PR3 — Trivial Handlers (auth / user / admin / system) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land 22 API v1 routes across 4 new handler files (`api_auth.go`, `api_user.go`, `api_admin.go`, `api_system.go`) that are thin wrappers over already-implemented services. Achieve Node response-shape parity. No new business logic — all business rules live in the existing `AdminService`, `SystemService`, `WorkspaceService`, `WorkspaceChatService`, `TemporaryAuthTokenService`, `VectorService`.

**Architecture:** Each new handler file mirrors `api_embed.go` (`handlers/api_embed.go:1-148`):
- Struct `APIxxxHandler` holds service deps.
- `NewAPIxxxHandler(...)` factory.
- One method per route.
- `RegisterAPIxxxRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, ...)` registers absolute paths `/v1/...` under the existing `api` group (NOT a sub-`v1` group — see design §3.3).

**Tech Stack:** Go 1.22+, Gin, GORM, sqlite (test), testify, httptest.

**Source spec:** `.gpowers/designs/2026-05-25-api-v1-layer-design.md` §4.1, §4.2, §4.3, §4.4.

**Reference Node implementation:**
- `server/endpoints/api/auth/index.js` (1 route)
- `server/endpoints/api/userManagement/index.js` (2 routes)
- `server/endpoints/api/admin/index.js` (13 routes)
- `server/endpoints/api/system/index.js` (6 routes)
- `server/utils/middleware/multiUserProtected.js` — the `multiUserMode(response)` helper that returns 401 when MultiUserMode is off.

**Depends on:** PR1 (DocumentService.PurgeByDocName for `/v1/system/remove-documents`). PR2 not required (no chat handlers in PR3).

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `middleware.ValidAPIKey(apiKeySvc)` (`middleware/api_key.go:12`) — Bearer-token gate. Use exactly as `api_embed.go:130` does.
- `middleware.IsMultiUserSetup(cfg)` — exists but returns **403 with `{"error":"Invalid request"}`**. Node API v1 routes return **401 with `Instance is not in Multi-User mode. Method denied`**. Do **not** use this middleware here; inline an `apiV1RequireMultiUser(c, cfg)` helper instead (Task 1).
- `AdminService` (`admin_service.go:18`): `ListUsers / CreateUser / UpdateUser / DeleteUser / GetUserByID / ListInvites / CreateInvite / DeactivateInvite / ValidRoleSelection / ValidCanModify / CanModifyAdmin`.
- `WorkspaceService` (`workspace_service.go:22`): `List / ListWorkspaceUsers / UpdateUsers(ctx, workspaceID, userIDs) / GetBySlug / GetByID`.
- `WorkspaceChatService` (`workspace_chat_service.go:47`): `ListChats(ctx, offset, limit) / CountChats / ExportChats(ctx, format)`.
- `SystemService` (`system_service.go:17`): `GetAllSettings / SetSetting / GetSetting`.
- `SystemHandler.UpdateEnv` (`handlers/system.go:134`) — already implemented; the v1 route reuses **the handler-level POST body parsing** by calling a small thin reuse function (see Task 8).
- `SystemHandler.EnvDump` (`handlers/system.go:503`) — noop 200; same here.
- `TemporaryAuthTokenService.Issue(ctx, userID)` (`temporary_auth_token_service.go:25`) → `(token, err)`.
- `VectorService.TotalVectors(ctx)` (`vector_service.go:78`).
- `models.User` filter helper — admin handler uses `FilterUserFields` pattern; for `/v1/users` we filter to `{id, username, role}` manually since Node shape is narrow.

### Routes to add (22)

| # | File | Method | Path | Multi-user required? |
|---|---|---|---|---|
| 1 | `api_auth.go` | GET | `/v1/auth` | no |
| 2 | `api_user.go` | GET | `/v1/users` | yes |
| 3 | `api_user.go` | GET | `/v1/users/:id/issue-auth-token` | yes (+ simple SSO) |
| 4 | `api_admin.go` | GET | `/v1/admin/is-multi-user-mode` | no |
| 5 | `api_admin.go` | GET | `/v1/admin/users` | yes |
| 6 | `api_admin.go` | POST | `/v1/admin/users/new` | yes |
| 7 | `api_admin.go` | POST | `/v1/admin/users/:id` | yes |
| 8 | `api_admin.go` | DELETE | `/v1/admin/users/:id` | yes |
| 9 | `api_admin.go` | GET | `/v1/admin/invites` | yes |
| 10 | `api_admin.go` | POST | `/v1/admin/invite/new` | yes |
| 11 | `api_admin.go` | DELETE | `/v1/admin/invite/:id` | yes |
| 12 | `api_admin.go` | GET | `/v1/admin/workspaces/:workspaceId/users` | yes |
| 13 | `api_admin.go` | POST | `/v1/admin/workspaces/:workspaceId/update-users` | yes |
| 14 | `api_admin.go` | POST | `/v1/admin/workspaces/:workspaceSlug/manage-users` | yes |
| 15 | `api_admin.go` | POST | `/v1/admin/workspace-chats` | yes |
| 16 | `api_admin.go` | POST | `/v1/admin/preferences` | yes |
| 17 | `api_system.go` | GET | `/v1/system` | no |
| 18 | `api_system.go` | GET | `/v1/system/env-dump` | no |
| 19 | `api_system.go` | GET | `/v1/system/vector-count` | no |
| 20 | `api_system.go` | POST | `/v1/system/update-env` | no |
| 21 | `api_system.go` | GET | `/v1/system/export-chats` | no |
| 22 | `api_system.go` | DELETE | `/v1/system/remove-documents` | no |

### Out of scope (explicit)

- **`/v1/admin/workspaces/:slug/manage-users` "create new users" branch** — Node's `manage-users` will create new user rows if `userIds` contains a `{username,password}` object instead of an int. The Go port honors only the `userIds: []int` case in PR3; the `create-on-bind` branch is a follow-up (file as known gap).
- **`/v1/users/:id/issue-auth-token` cookie/redirect flow** — Go returns `{token, loginPath}`; the `/sso/simple` GET that consumes the token is PR4 (workspace + thread) follow-up, not PR3.
- **`EventLogs.logEvent(...)`** in Node (e.g. `api_user_deleted`) — not emitted in PR3. Platform-wide EventLog wire-up tracked separately.
- **`Telemetry.sendTelemetry(...)`** calls — same, deferred.
- **API key role scoping (admin-only vs default)** — design §3.4 confirms parity-with-Node posture: any valid key can call admin routes. Do not add role checks here.

### Multi-user-mode helper

Node's `multiUserMode(response)` reads from `SystemSettings.currentSettings()`. In Go, `cfg.MultiUserMode` (set at startup) is the source of truth. Helper signature:

```go
// File: backend/internal/handlers/api_helpers.go (new)
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
)

// apiV1RequireMultiUser returns true if the request can proceed.
// Returns false (and writes the Node-parity 401) when MultiUserMode is off.
func apiV1RequireMultiUser(c *gin.Context, cfg *config.Config) bool {
    if !cfg.MultiUserMode {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "Instance is not in Multi-User mode. Method denied",
        })
        return false
    }
    return true
}
```

This is a free function (not a method) so any handler can call it inline.

### Response-shape conventions (match Node exactly)

- All success responses: HTTP **200** even on business validation errors. Errors carry `{success: false, error: "..."}` or `{user: null, error: "..."}` per Node.
- HTTP 4xx/5xx only for: auth/permission rejection, malformed JSON (400), genuine server errors (500), Node's explicit `sendStatus(401|403|404)` for missing rows.
- `/v1/system/remove-documents` returns `{success: true, message: "Documents removed successfully"}`.
- `/v1/admin/preferences` returns `{success: true, error: null}`.
- `/v1/admin/workspace-chats` returns `{chats: [...], hasPages: bool}`.
- `/v1/users` returns `{users: [{id, username, role}]}` only — strip password/recovery codes/etc.

### TDD discipline

Each task: write failing test → run + confirm fail → minimal impl → run + confirm pass → commit. Tests are HTTP-level (use `httptest.NewRecorder` + a router built by `setupAPIV1Router(t)`). One commit per task = one route group.

### Test setup helper

A small shared helper in a private `_test.go` file (e.g. `api_setup_test.go`) inside the `handlers` package, mirroring existing test patterns:

```go
// File: backend/internal/handlers/api_setup_test.go
package handlers

import (
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

type apiTestEnv struct {
    Router    *gin.Engine
    DB        *gorm.DB
    Cfg       *config.Config
    APIKeySvc *services.APIKeyService
    APIKey    string // raw secret to put in Authorization: Bearer <APIKey>
}

func newAPITestEnv(t *testing.T, cfg *config.Config) *apiTestEnv {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, services.AutoMigrate(db))
    if cfg == nil {
        cfg = &config.Config{MultiUserMode: true}
    }
    keySvc := services.NewAPIKeyService(db)
    key, err := keySvc.Create(context.Background(), nil, nil)
    require.NoError(t, err)
    require.NotNil(t, key.Secret)

    gin.SetMode(gin.TestMode)
    r := gin.New()

    return &apiTestEnv{
        Router:    r,
        DB:        db,
        Cfg:       cfg,
        APIKeySvc: keySvc,
        APIKey:    *key.Secret, // models.APIKey.Secret is *string
    }
}
```

Each task customizes by calling the relevant `Register*` function inside the test setup, then issuing HTTP requests via `httptest`. Use `r.ServeHTTP(rec, req)` and decode JSON from `rec.Body`.

If `models.APIKey` does not expose a `Secret` field directly, adapt to whatever the existing `APIKeyService.Create` returns (the value the user puts in `Authorization: Bearer`). Audit `api_key_service.go:26` before writing the helper.

---

## Task 1: API v1 multi-user-mode helper

Create the shared helper that all multi-user-gated handlers call. Single file, two lines of logic, but lock it down with a unit test first so behavioral drift is caught.

**Files:**
- Create: `backend/internal/handlers/api_helpers.go`
- Create: `backend/internal/handlers/api_helpers_test.go`

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/handlers/api_helpers_test.go
package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/stretchr/testify/assert"
)

func TestApiV1RequireMultiUser_Allows(t *testing.T) {
    gin.SetMode(gin.TestMode)
    rec := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(rec)
    cfg := &config.Config{MultiUserMode: true}
    assert.True(t, apiV1RequireMultiUser(c, cfg))
    assert.Equal(t, http.StatusOK, rec.Code) // unset, default 200
}

func TestApiV1RequireMultiUser_Denies(t *testing.T) {
    gin.SetMode(gin.TestMode)
    rec := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(rec)
    cfg := &config.Config{MultiUserMode: false}
    assert.False(t, apiV1RequireMultiUser(c, cfg))
    assert.Equal(t, http.StatusUnauthorized, rec.Code)
    assert.Contains(t, rec.Body.String(), "Multi-User mode")
}
```

- [ ] **Step 2: Run and confirm fail**

```bash
cd backend && go test ./internal/handlers/ -run TestApiV1RequireMultiUser -count=1
```

- [ ] **Step 3: Implement**

```go
// File: backend/internal/handlers/api_helpers.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
)

func apiV1RequireMultiUser(c *gin.Context, cfg *config.Config) bool {
    if !cfg.MultiUserMode {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "Instance is not in Multi-User mode. Method denied",
        })
        return false
    }
    return true
}
```

- [ ] **Step 4: Run + commit**

```bash
cd backend && go test ./internal/handlers/ -run TestApiV1RequireMultiUser -count=1
git add backend/internal/handlers/api_helpers.go backend/internal/handlers/api_helpers_test.go
git commit -m "feat(api-v1): apiV1RequireMultiUser helper for Node-parity 401 response"
```

---

## Task 2: `api_auth.go` — GET /v1/auth (1 route)

Trivial: middleware passes → return `{authenticated: true}`.

**Files:**
- Create: `backend/internal/handlers/api_auth.go`
- Create: `backend/internal/handlers/api_auth_test.go`

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/handlers/api_auth_test.go
package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAPIAuth_Authenticated(t *testing.T) {
    env := newAPITestEnv(t, nil)
    api := env.Router.Group("/api")
    RegisterAPIAuthRoutes(api, env.APIKeySvc)

    req := httptest.NewRequest("GET", "/api/v1/auth", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    var body map[string]any
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Equal(t, true, body["authenticated"])
}

func TestAPIAuth_RejectsInvalidKey(t *testing.T) {
    env := newAPITestEnv(t, nil)
    api := env.Router.Group("/api")
    RegisterAPIAuthRoutes(api, env.APIKeySvc)

    req := httptest.NewRequest("GET", "/api/v1/auth", nil)
    req.Header.Set("Authorization", "Bearer bogus")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusForbidden, rec.Code)
}
```

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement**

```go
// File: backend/internal/handlers/api_auth.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/services"
)

func RegisterAPIAuthRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService) {
    r.GET("/v1/auth",
        middleware.ValidAPIKey(apiKeySvc),
        func(c *gin.Context) {
            c.JSON(http.StatusOK, gin.H{"authenticated": true})
        })
}
```

- [ ] **Step 4: Run + commit**

```bash
git add backend/internal/handlers/api_auth.go backend/internal/handlers/api_auth_test.go
git commit -m "feat(api-v1): GET /v1/auth"
```

---

## Task 3: `api_user.go` — /v1/users + /v1/users/:id/issue-auth-token (2 routes)

`/v1/users` is multi-user-gated, returns filtered user list. `/v1/users/:id/issue-auth-token` additionally requires `cfg.SimpleSSOEnabled`.

**Files:**
- Create: `backend/internal/handlers/api_user.go`
- Create: `backend/internal/handlers/api_user_test.go`

- [ ] **Step 1: Failing test**

```go
// File: backend/internal/handlers/api_user_test.go
package handlers

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/odysseythink/hermind/backend/pkg/utils"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAPIUsers_Lists(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.User{Username: utils.Ptr("alice"), Role: "admin"}).Error)
    require.NoError(t, env.DB.Create(&models.User{Username: utils.Ptr("bob"), Role: "default"}).Error)

    adminSvc := services.NewAdminService(env.DB)
    tempSvc := services.NewTemporaryAuthTokenService(env.DB)
    api := env.Router.Group("/api")
    RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/users", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Users []map[string]any `json:"users"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    require.Len(t, body.Users, 2)
    // Sensitive fields not in payload
    for _, u := range body.Users {
        _, hasPassword := u["password"]
        assert.False(t, hasPassword)
        _, hasUsername := u["username"]
        assert.True(t, hasUsername)
        _, hasRole := u["role"]
        assert.True(t, hasRole)
    }
}

func TestAPIUsers_DeniedWhenNotMultiUser(t *testing.T) {
    env := newAPITestEnv(t, &config.Config{MultiUserMode: false})
    adminSvc := services.NewAdminService(env.DB)
    tempSvc := services.NewTemporaryAuthTokenService(env.DB)
    api := env.Router.Group("/api")
    RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/users", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIUsers_IssueAuthToken(t *testing.T) {
    cfg := &config.Config{MultiUserMode: true, SimpleSSOEnabled: true}
    env := newAPITestEnv(t, cfg)
    u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
    require.NoError(t, env.DB.Create(u).Error)

    adminSvc := services.NewAdminService(env.DB)
    tempSvc := services.NewTemporaryAuthTokenService(env.DB)
    api := env.Router.Group("/api")
    RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/users/"+strconv.Itoa(u.ID)+"/issue-auth-token", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Token     string `json:"token"`
        LoginPath string `json:"loginPath"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.NotEmpty(t, body.Token)
    assert.Contains(t, body.LoginPath, "/sso/simple?token=")

    _ = context.Background() // pacify unused import in fuller test files
}

func TestAPIUsers_IssueAuthToken_RequiresSimpleSSO(t *testing.T) {
    cfg := &config.Config{MultiUserMode: true, SimpleSSOEnabled: false}
    env := newAPITestEnv(t, cfg)
    u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
    require.NoError(t, env.DB.Create(u).Error)

    adminSvc := services.NewAdminService(env.DB)
    tempSvc := services.NewTemporaryAuthTokenService(env.DB)
    api := env.Router.Group("/api")
    RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/users/"+strconv.Itoa(u.ID)+"/issue-auth-token", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
```

(Add `"strconv"` to imports.)

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement**

```go
// File: backend/internal/handlers/api_user.go
package handlers

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/services"
)

type APIUserHandler struct {
    adminSvc *services.AdminService
    tempSvc  *services.TemporaryAuthTokenService
    cfg      *config.Config
}

func NewAPIUserHandler(adminSvc *services.AdminService, tempSvc *services.TemporaryAuthTokenService, cfg *config.Config) *APIUserHandler {
    return &APIUserHandler{adminSvc: adminSvc, tempSvc: tempSvc, cfg: cfg}
}

func (h *APIUserHandler) List(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) {
        return
    }
    users, err := h.adminSvc.ListUsers(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    out := make([]gin.H, 0, len(users))
    for _, u := range users {
        username := ""
        if u.Username != nil {
            username = *u.Username
        }
        out = append(out, gin.H{"id": u.ID, "username": username, "role": u.Role})
    }
    c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *APIUserHandler) IssueAuthToken(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) {
        return
    }
    if !h.cfg.SimpleSSOEnabled {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "Simple SSO is not enabled on this instance.",
        })
        return
    }
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }
    user, err := h.adminSvc.GetUserByID(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }
    token, err := h.tempSvc.Issue(c.Request.Context(), user.ID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "token":     token,
        "loginPath": "/sso/simple?token=" + token,
    })
}

func RegisterAPIUserRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, adminSvc *services.AdminService, tempSvc *services.TemporaryAuthTokenService, cfg *config.Config) {
    h := NewAPIUserHandler(adminSvc, tempSvc, cfg)
    r.GET("/v1/users", middleware.ValidAPIKey(apiKeySvc), h.List)
    r.GET("/v1/users/:id/issue-auth-token", middleware.ValidAPIKey(apiKeySvc), h.IssueAuthToken)
}
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/users + /v1/users/:id/issue-auth-token"
```

---

## Task 4: `api_admin.go` — is-multi-user-mode + users CRUD (5 routes)

5 routes: GET is-multi-user-mode, GET users, POST users/new, POST users/:id (update), DELETE users/:id.

**Files:**
- Create: `backend/internal/handlers/api_admin.go`
- Create: `backend/internal/handlers/api_admin_test.go`

- [ ] **Step 1: Write failing tests** (one Go test per route; ~5 short tests)

Sketch (full body in the same pattern as Task 3):

```go
// /api/v1/admin/is-multi-user-mode is the only one that doesn't gate on MultiUserMode.
// Returns {"isMultiUser": true|false}.

func TestAPIAdmin_IsMultiUserMode_True(t *testing.T) {
    env := newAPITestEnv(t, &config.Config{MultiUserMode: true})
    /* register routes; GET /api/v1/admin/is-multi-user-mode; expect 200 + {"isMultiUser":true} */
}
func TestAPIAdmin_IsMultiUserMode_False(t *testing.T) { /* similar with MultiUserMode:false → "isMultiUser":false but still 200 */ }
func TestAPIAdmin_Users_List(t *testing.T)            { /* seed 2 users; expect 200 + 2 users */ }
func TestAPIAdmin_Users_NewSuccess(t *testing.T)       { /* POST {username,password,role:"default"}; 200 + {user:{...},error:null} */ }
func TestAPIAdmin_Users_NewBusinessError(t *testing.T) { /* empty password; 200 + {user:null,error:"..."} */ }
func TestAPIAdmin_Users_UpdateSuccess(t *testing.T)    { /* POST /admin/users/:id {bio:"x"}; 200 + {success:true,error:null} */ }
func TestAPIAdmin_Users_DeleteSuccess(t *testing.T)    { /* DELETE; 200 + {success:true,error:null} */ }
func TestAPIAdmin_Users_DeniedWithoutMultiUser(t *testing.T) { /* MultiUserMode:false; expect 401 */ }
```

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement (5 handler methods + Register call)**

```go
// File: backend/internal/handlers/api_admin.go
package handlers

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/services"
)

type APIAdminHandler struct {
    adminSvc   *services.AdminService
    sysSvc     *services.SystemService
    wsSvc      *services.WorkspaceService
    wsChatSvc  *services.WorkspaceChatService
    cfg        *config.Config
}

func NewAPIAdminHandler(adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, wsChatSvc *services.WorkspaceChatService, cfg *config.Config) *APIAdminHandler {
    return &APIAdminHandler{adminSvc: adminSvc, sysSvc: sysSvc, wsSvc: wsSvc, wsChatSvc: wsChatSvc, cfg: cfg}
}

func (h *APIAdminHandler) IsMultiUserMode(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"isMultiUser": h.cfg.MultiUserMode})
}

func (h *APIAdminHandler) ListUsers(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) {
        return
    }
    users, err := h.adminSvc.ListUsers(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    // Full filter helper exists elsewhere; here we return safe subset.
    c.JSON(http.StatusOK, gin.H{"users": filterUsersForAdminAPI(users)})
}

// Helper inside file (or moved to api_helpers.go later if reused):
func filterUsersForAdminAPI(users []models.User) []gin.H {
    out := make([]gin.H, 0, len(users))
    for _, u := range users {
        username := ""
        if u.Username != nil {
            username = *u.Username
        }
        out = append(out, gin.H{
            "id":       u.ID,
            "username": username,
            "role":     u.Role,
            "bio":      derefStr(u.Bio),
            "suspended": u.Suspended,
            // Do NOT include password / recovery codes / webPushSubscriptionConfig
        })
    }
    return out
}

func (h *APIAdminHandler) CreateUser(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) {
        return
    }
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
        Role     string `json:"role"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    user, bizErr, sysErr := h.adminSvc.CreateUser(c.Request.Context(), services.CreateUserInput{
        Username: req.Username, Password: req.Password, Role: req.Role,
    })
    if sysErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": sysErr.Error()})
        return
    }
    if bizErr != "" {
        c.JSON(http.StatusOK, gin.H{"user": nil, "error": bizErr})
        return
    }
    c.JSON(http.StatusOK, gin.H{"user": user, "error": nil})
}

func (h *APIAdminHandler) UpdateUser(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) {
        return
    }
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }
    var updates map[string]any
    if err := c.ShouldBindJSON(&updates); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    bizErr, sysErr := h.adminSvc.UpdateUser(c.Request.Context(), id, updates)
    if sysErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": sysErr.Error()})
        return
    }
    if bizErr != "" {
        c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) DeleteUser(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) {
        return
    }
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }
    if err := h.adminSvc.DeleteUser(c.Request.Context(), id); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterAPIAdminRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, wsChatSvc *services.WorkspaceChatService, cfg *config.Config) {
    h := NewAPIAdminHandler(adminSvc, sysSvc, wsSvc, wsChatSvc, cfg)

    // Multi-user gate is inline per handler (see helper).
    r.GET("/v1/admin/is-multi-user-mode", middleware.ValidAPIKey(apiKeySvc), h.IsMultiUserMode)
    r.GET("/v1/admin/users", middleware.ValidAPIKey(apiKeySvc), h.ListUsers)
    r.POST("/v1/admin/users/new", middleware.ValidAPIKey(apiKeySvc), h.CreateUser)
    r.POST("/v1/admin/users/:id", middleware.ValidAPIKey(apiKeySvc), h.UpdateUser)
    r.DELETE("/v1/admin/users/:id", middleware.ValidAPIKey(apiKeySvc), h.DeleteUser)
    // Tasks 5/6/7 will append the rest here.
}
```

Add `"github.com/odysseythink/hermind/backend/internal/models"` to imports.

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/admin/is-multi-user-mode + users CRUD (5 routes)"
```

---

## Task 5: `api_admin.go` — invites (3 routes)

GET /v1/admin/invites, POST /v1/admin/invite/new, DELETE /v1/admin/invite/:id.

- [ ] **Step 1: Failing tests** — 3 short HTTP tests (list, create, deactivate).

- [ ] **Step 2-3: Implement** — append three methods to `api_admin.go`:

```go
func (h *APIAdminHandler) ListInvites(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    invites, err := h.adminSvc.ListInvites(c.Request.Context())
    if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"invites": invites})
}

func (h *APIAdminHandler) CreateInvite(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    var req struct { WorkspaceIDs []int `json:"workspaceIds"` }
    _ = c.ShouldBindJSON(&req) // body optional
    // Node passes createdBy=null for API context; we mimic by passing 0 (admin_service handles nullable createdBy via pointer if extended)
    inv, err := h.adminSvc.CreateInvite(c.Request.Context(), 0, req.WorkspaceIDs)
    if err != nil { c.JSON(http.StatusOK, gin.H{"invite": nil, "error": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"invite": inv, "error": nil})
}

func (h *APIAdminHandler) DeactivateInvite(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
    if err := h.adminSvc.DeactivateInvite(c.Request.Context(), id); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Append to `RegisterAPIAdminRoutes`:

```go
r.GET("/v1/admin/invites", middleware.ValidAPIKey(apiKeySvc), h.ListInvites)
r.POST("/v1/admin/invite/new", middleware.ValidAPIKey(apiKeySvc), h.CreateInvite)
r.DELETE("/v1/admin/invite/:id", middleware.ValidAPIKey(apiKeySvc), h.DeactivateInvite)
```

> **Note on `createdBy`**: `AdminService.CreateInvite(ctx, createdBy int, workspaceIDs)` requires a non-nullable int. Node passes `null` because API context has no user. Verify the current signature; if `createdBy` is non-pointer, either pass 0 and tolerate a foreign-key constraint failure as "no FK in sqlite", or extend the service signature to accept `*int`. **Audit before implementing this task.**

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/admin/invites (3 routes)"
```

---

## Task 6: `api_admin.go` — workspaces (3 routes)

GET `/v1/admin/workspaces/:workspaceId/users`, POST `/v1/admin/workspaces/:workspaceId/update-users`, POST `/v1/admin/workspaces/:workspaceSlug/manage-users`.

- [ ] **Step 1: Failing tests** — 3 HTTP tests; for manage-users send `{userIds: [1,2]}`.

- [ ] **Step 2-3: Implement**

```go
func (h *APIAdminHandler) ListWorkspaceUsers(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    wsID, err := strconv.Atoi(c.Param("workspaceId"))
    if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspaceId"}); return }
    users, err := h.wsSvc.ListWorkspaceUsers(c.Request.Context(), wsID)
    if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *APIAdminHandler) UpdateWorkspaceUsers(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    wsID, err := strconv.Atoi(c.Param("workspaceId"))
    if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspaceId"}); return }
    var req struct { UserIDs []int `json:"userIds"` }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return
    }
    if err := h.wsSvc.UpdateUsers(c.Request.Context(), wsID, req.UserIDs); err != nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()}); return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) ManageWorkspaceUsers(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    slug := c.Param("workspaceSlug")
    ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "workspace not found"}); return
    }
    var req struct {
        UserIDs []int `json:"userIds"`
        Reset   bool  `json:"reset"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return
    }
    if err := h.wsSvc.UpdateUsers(c.Request.Context(), ws.ID, req.UserIDs); err != nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()}); return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Append to `RegisterAPIAdminRoutes`:

```go
r.GET("/v1/admin/workspaces/:workspaceId/users", middleware.ValidAPIKey(apiKeySvc), h.ListWorkspaceUsers)
r.POST("/v1/admin/workspaces/:workspaceId/update-users", middleware.ValidAPIKey(apiKeySvc), h.UpdateWorkspaceUsers)
r.POST("/v1/admin/workspaces/:workspaceSlug/manage-users", middleware.ValidAPIKey(apiKeySvc), h.ManageWorkspaceUsers)
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/admin/workspaces/* (3 routes)"
```

---

## Task 7: `api_admin.go` — workspace-chats + preferences (2 routes)

POST `/v1/admin/workspace-chats` (paginated), POST `/v1/admin/preferences` (batch set).

- [ ] **Step 1: Failing tests**

```go
func TestAPIAdmin_WorkspaceChats_Pagination(t *testing.T) {
    /* seed 25 chats; POST {offset:0}; expect chats:[...20 items], hasPages:true */
    /* POST {offset:1}; expect 5 items, hasPages:false */
}
func TestAPIAdmin_Preferences_Updates(t *testing.T) {
    /* POST {support_email:"x@y", title:"My"}; assert SystemService.GetSetting returns "x@y" */
}
```

- [ ] **Step 2-3: Implement**

```go
func (h *APIAdminHandler) WorkspaceChats(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    var req struct { Offset int `json:"offset"` }
    _ = c.ShouldBindJSON(&req)
    const pageSize = 20
    chats, _, err := h.wsChatSvc.ListChats(c.Request.Context(), req.Offset*pageSize, pageSize)
    if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
    total, err := h.wsChatSvc.CountChats(c.Request.Context())
    if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
    hasPages := total > int64((req.Offset+1)*pageSize)
    c.JSON(http.StatusOK, gin.H{"chats": chats, "hasPages": hasPages})
}

func (h *APIAdminHandler) UpdatePreferences(c *gin.Context) {
    if !apiV1RequireMultiUser(c, h.cfg) { return }
    var updates map[string]any
    if err := c.ShouldBindJSON(&updates); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return
    }
    for k, v := range updates {
        // Coerce values to string; SystemService.SetSetting persists strings only.
        if err := h.sysSvc.SetSetting(c.Request.Context(), k, fmt.Sprintf("%v", v)); err != nil {
            c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()}); return
        }
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Add `"fmt"` import.

Append registrations:

```go
r.POST("/v1/admin/workspace-chats", middleware.ValidAPIKey(apiKeySvc), h.WorkspaceChats)
r.POST("/v1/admin/preferences", middleware.ValidAPIKey(apiKeySvc), h.UpdatePreferences)
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/admin/workspace-chats + preferences (2 routes)"
```

---

## Task 8: `api_system.go` — 6 routes (system / env-dump / vector-count / update-env / export-chats / remove-documents)

System routes are **not** multi-user-gated.

**Files:**
- Create: `backend/internal/handlers/api_system.go`
- Create: `backend/internal/handlers/api_system_test.go`

- [ ] **Step 1: Failing tests** (one per route, plus auth-fail)

```go
func TestAPISystem_GetAll(t *testing.T)      { /* seed system_settings rows; expect 200 + {settings:{...}} */ }
func TestAPISystem_EnvDump(t *testing.T)      { /* 200, body may be empty */ }
func TestAPISystem_VectorCount(t *testing.T)  { /* nil vector provider → 200 + {vectorCount:0} */ }
func TestAPISystem_UpdateEnv(t *testing.T)    { /* POST {SomeKey:"value"}; assert persisted in system_settings */ }
func TestAPISystem_ExportChats_JSONL(t *testing.T) { /* seed chats; GET ?type=jsonl; content-type x-jsonlines; non-empty body */ }
func TestAPISystem_RemoveDocuments(t *testing.T) { /* seed doc via PR1's SaveRawText or direct DB; DELETE {names:[...]}; assert success:true */ }
```

- [ ] **Step 2-3: Implement**

```go
// File: backend/internal/handlers/api_system.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/services"
)

type APISystemHandler struct {
    sysSvc    *services.SystemService
    vectorSvc *services.VectorService
    docSvc    *services.DocumentService
    wsChatSvc *services.WorkspaceChatService
    // Reuse the existing web SystemHandler for update-env (its body parse + provider reload is non-trivial)
    webSysHdlr *SystemHandler
}

func NewAPISystemHandler(sysSvc *services.SystemService, vectorSvc *services.VectorService, docSvc *services.DocumentService, wsChatSvc *services.WorkspaceChatService, webSysHdlr *SystemHandler) *APISystemHandler {
    return &APISystemHandler{sysSvc: sysSvc, vectorSvc: vectorSvc, docSvc: docSvc, wsChatSvc: wsChatSvc, webSysHdlr: webSysHdlr}
}

func (h *APISystemHandler) GetAll(c *gin.Context) {
    settings, err := h.sysSvc.GetAllSettings(c.Request.Context())
    if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h *APISystemHandler) EnvDump(c *gin.Context) {
    // Go runtime does not write a .env file; noop 200 (Node parity for non-prod).
    c.Status(http.StatusOK)
}

func (h *APISystemHandler) VectorCount(c *gin.Context) {
    n, err := h.vectorSvc.TotalVectors(c.Request.Context())
    if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"vectorCount": n})
}

func (h *APISystemHandler) UpdateEnv(c *gin.Context) {
    // Delegate to the existing web handler which already parses + persists + reloads providers.
    h.webSysHdlr.UpdateEnv(c)
}

func (h *APISystemHandler) ExportChats(c *gin.Context) {
    format := c.DefaultQuery("type", "jsonl")
    contentType, data, err := h.wsChatSvc.ExportChats(c.Request.Context(), format)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return
    }
    c.Header("Content-Type", contentType)
    c.Data(http.StatusOK, contentType, data)
}

func (h *APISystemHandler) RemoveDocuments(c *gin.Context) {
    var req dto.APISystemRemoveDocumentsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return
    }
    for _, name := range req.Names {
        if err := h.docSvc.PurgeByDocName(c.Request.Context(), name); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()}); return
        }
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Documents removed successfully"})
}

func RegisterAPISystemRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, sysSvc *services.SystemService, vectorSvc *services.VectorService, docSvc *services.DocumentService, wsChatSvc *services.WorkspaceChatService, webSysHdlr *SystemHandler) {
    h := NewAPISystemHandler(sysSvc, vectorSvc, docSvc, wsChatSvc, webSysHdlr)
    r.GET("/v1/system", middleware.ValidAPIKey(apiKeySvc), h.GetAll)
    r.GET("/v1/system/env-dump", middleware.ValidAPIKey(apiKeySvc), h.EnvDump)
    r.GET("/v1/system/vector-count", middleware.ValidAPIKey(apiKeySvc), h.VectorCount)
    r.POST("/v1/system/update-env", middleware.ValidAPIKey(apiKeySvc), h.UpdateEnv)
    r.GET("/v1/system/export-chats", middleware.ValidAPIKey(apiKeySvc), h.ExportChats)
    r.DELETE("/v1/system/remove-documents", middleware.ValidAPIKey(apiKeySvc), h.RemoveDocuments)
}
```

> **Audit point**: `SystemHandler` (`handlers/system.go:27`) is constructed via a large `NewSystemHandler(...)` factory in `RegisterSystemRoutes`. Decide whether to refactor so the constructor is exported, or whether `api_system.go` should accept the already-built handler instance via main.go. The plan above takes option B (accept built handler). Verify and adjust based on the actual `SystemHandler` constructor signature.

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): api_system.go — 6 system routes"
```

---

## Task 9: main.go wiring

Mount all four `RegisterAPI*Routes` calls inside the existing `api := r.Group("/api")` block, right after `RegisterAPIEmbedRoutes` (`main.go:162`).

- [ ] **Step 1: Append to main.go (after `handlers.RegisterAPIEmbedRoutes(api, embedSvc, apiKeySvc, db)`)**

```go
handlers.RegisterAPIAuthRoutes(api, apiKeySvc)
handlers.RegisterAPIUserRoutes(api, apiKeySvc, adminSvc, tempTokenSvc, cfg)
handlers.RegisterAPIAdminRoutes(api, apiKeySvc, adminSvc, sysSvc, wsSvc, wsChatSvc, cfg)
// SystemHandler must already be built earlier in the block; if not, hoist construction.
handlers.RegisterAPISystemRoutes(api, apiKeySvc, sysSvc, vectorSvc, docSvc, wsChatSvc, systemHandler)
```

If `systemHandler` is not currently held in a named variable (it's created inline inside `RegisterSystemRoutes`), hoist it:

```go
systemHandler := handlers.NewSystemHandler(sysSvc, /* ...same deps as RegisterSystemRoutes uses... */)
handlers.RegisterSystemRoutesWithHandler(api, systemHandler) // optional refactor
```

A cleaner alternative: export the constructor and re-use it. Audit `RegisterSystemRoutes` first; if the only consumer of the inner handler instance is itself, refactor minimally to share.

- [ ] **Step 2: Build + run full suite**

```bash
cd backend && go build ./...
cd backend && go test ./... -count=1
```

- [ ] **Step 3: Smoke-test with curl** (manual, optional)

```bash
# Assumes server running on :8080 with at least one API key seeded:
curl -H "Authorization: Bearer <KEY>" http://localhost:8080/api/v1/auth
curl -H "Authorization: Bearer <KEY>" http://localhost:8080/api/v1/admin/is-multi-user-mode
```

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(api-v1): wire 22 trivial handlers in main.go"
```

---

## Task 10: Full-suite verify + lint

- [ ] **Step 1**: `cd backend && go test ./... -count=1` — must be green.
- [ ] **Step 2**: `cd backend && go vet ./...` — clean.
- [ ] **Step 3**: `cd backend && go build ./...` — succeeds.
- [ ] **Step 4**: Sanity-check route table:
  ```bash
  cd backend && go run ./cmd/server &
  sleep 2
  curl http://localhost:8080/api/v1/auth   # → 401 (no Bearer)
  # ... kill server
  ```

---

## Acceptance criteria

- [ ] 22 routes mounted under `/api/v1/...` with `middleware.ValidAPIKey` gate.
- [ ] All multi-user-gated routes return Node-parity 401 `Instance is not in Multi-User mode. Method denied` when `cfg.MultiUserMode=false`.
- [ ] `/v1/users` returns only `{id, username, role}` per user — no password, no recovery codes.
- [ ] `/v1/users/:id/issue-auth-token` returns `{token, loginPath: "/sso/simple?token=..."}` only when `cfg.SimpleSSOEnabled=true`.
- [ ] `/v1/admin/workspace-chats` paginates by 20, returns `{chats, hasPages}`.
- [ ] `/v1/admin/preferences` persists all keys via `SystemService.SetSetting`.
- [ ] `/v1/system/remove-documents` calls `DocumentService.PurgeByDocName` for each name; returns `{success, message}`.
- [ ] `/v1/system/export-chats?type=jsonl` returns content-type matching the export format.
- [ ] All pre-existing tests still pass.
- [ ] No new lint or vet warnings.

---

## Known gaps after PR3 (track but DO NOT implement here)

1. **`manage-users` user-creation branch** — Node accepts `userIds` items as either `int` or `{username,password,role}` to create-and-bind. Go port only honors `[]int`. File as PR4 follow-up.
2. **`AdminService.CreateInvite(createdBy int)` ↔ Node `createdBy: null`** — currently passes `0`. If FK constraint exists on `users.id`, this may fail on Postgres (works on sqlite without FK). Audit and either extend signature to `*int` or use a sentinel.
3. **`SystemService.SetSetting` only accepts string values** — `/v1/admin/preferences` coerces with `fmt.Sprintf("%v", v)`. JSON booleans become `"true"`/`"false"`; nested objects flatten to `"map[...]"` strings. Acceptable for Node parity but document the lossy cast. A proper batch `UpdateSettings(map[string]any)` is a follow-up.
4. **`EventLogs.logEvent` calls** — `api_user_created`, `api_user_deleted`, `api_invite_created`, `api_invite_deleted`, `api_workspace_chats_exported`, etc. — deferred to platform-wide EventLog wire-up.
5. **`SystemHandler.UpdateEnv` reuse** assumes the web handler's body-parse + provider-reload semantics are appropriate for API v1. If the web handler relies on session context (e.g. `c.GetInt("userId")`), API v1 callers may hit a nil reference. Audit before merge.
6. **API Key role scoping** — design §3.4 — still no enforcement here. Tracked.
