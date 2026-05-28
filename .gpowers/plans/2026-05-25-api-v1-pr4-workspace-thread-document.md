# API v1 PR4 — Workspace / Thread / Document Handlers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land 29 API v1 routes across three new handler files (`api_workspace.go`, `api_document.go`, `api_thread.go`) wired against existing services + PR1 service extensions. Achieve Node response-shape parity. Move the temporary `/v1/workspace/:slug/chat` mount from `main.go:168` into `api_workspace.go` so v1 surface is in one place.

**Architecture:** Each handler mirrors the `api_embed.go` (`handlers/api_embed.go:1-148`) / `api_admin.go` (PR3) pattern: `APIxxxHandler` struct + factory + one method per route + `RegisterAPIxxxRoutes(r, apiKeySvc, ...)` registering absolute `/v1/...` paths under the existing `api` group. Streaming routes write SSE via the existing `writeSSEChunk(...)` helper (referenced by `ChatHandler.StreamChat` at `handlers/chat.go:76`).

**Tech Stack:** Go 1.22+, Gin, GORM, sqlite (test), testify, httptest, multipart.

**Source spec:** `.gpowers/designs/2026-05-25-api-v1-layer-design.md` §4.5, §4.6, §4.7.

**Reference Node implementation:**
- `server/endpoints/api/workspace/index.js` (11 routes)
- `server/endpoints/api/document/index.js` (12 routes)
- `server/endpoints/api/workspaceThread/index.js` (6 routes)

**Depends on:** PR1 (`WorkspaceService.UpdatePin`, `DocumentService.SaveRawText`, `DocumentService.RemoveFolder`). PR2 not strictly required for PR4 routes, but if PR2 has landed the override fields are available — handlers pass them through transparently.

**State note:** PR1 + PR2 are already committed (`3646d3c..7738033`). PR3 may or may not have landed yet — if it has, the new `api_helpers.go` (`apiV1RequireMultiUser`) is available but PR4 routes do **not** need it (none of the workspace/document/thread v1 routes are multi-user gated in Node).

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `WorkspaceService` (`workspace_service.go:22`): `Create / List / GetBySlug / Update / Delete / GetChats / UpdatePin` + helpers. Note: `Create(ctx, userID int, req)` — for API context pass `userID = 0`; service tolerates missing user (audit: it currently does not error on userID=0, but the workspace row's `created_by_user_id` will be 0; behavior must match Node which passes `null`).
- `ChatService.Stream(ctx, ws, user, threadID, req)` (`chat_service.go:94`) returns `<-chan dto.StreamChatResponse`.
- `ChatService.Complete(ctx, ws, user, threadID, req)` (`chat_service.go:191`) returns `*dto.ChatResponse`.
- `ChatHandler.ApiChat` (`handlers/chat.go:105`) — the existing temporary mount for `/v1/workspace/:slug/chat`. PR4 reuses its body but moves it into `api_workspace.go`.
- `writeSSEChunk(w, chunk)` (used at `handlers/chat.go:76`) — existing SSE writer.
- `VectorSearchService.Search(ctx, ws, req)` (`vector_search_service.go:22`).
- `DocumentService`: `SaveUpload / UploadToWorkspace / UploadLink / ListDocuments / ListFolderDocuments / GetByDocName / CreateFolder / MoveFiles / UpdateEmbeddings / SaveRawText / PurgeByDocName / RemoveFolder`.
- `FileSystemService.AcceptedDocumentTypes() map[string]string` (`filesystem_service.go:83`) — used for `/v1/document/accepted-file-types`.
- `ThreadService` (`thread_service.go:17`): `Create(ctx, workspaceID, userID *int, req) / GetBySlug(ctx, workspaceID, threadSlug) / Update / Delete / GetThreadChats`.
- `middleware.ValidWorkspaceSlug(db)` (`middleware/workspace.go`) populates `c.MustGet("workspace")`. Reuse — saves redundant lookup code in each v1 handler. Mount order: `ValidAPIKey` → `ValidWorkspaceSlug`.
- `middleware.ValidWorkspaceAndThreadSlug(db)` (`middleware/workspace_thread.go:12`) — populates **both** `c.MustGet("workspace")` and `c.MustGet("thread")` in a single pass. Reuse for thread routes that have both `:slug` and `:threadSlug` path params. Do **not** chain it after `ValidWorkspaceSlug` — it would double-fetch the workspace row.

### Routes to add (29)

#### Workspace (11)

| # | Method | Path | Notes |
|---|---|---|---|
| W1 | GET | `/v1/workspaces` | `WorkspaceService.List(ctx, 0)` (API context has no user filter) |
| W2 | POST | `/v1/workspace/new` | `Create(ctx, 0, req)`; returns `{workspace, message}` |
| W3 | GET | `/v1/workspace/:slug` | `GetBySlug` |
| W4 | POST | `/v1/workspace/:slug/update` | `Update(ctx, slug, req)` |
| W5 | POST | `/v1/workspace/:slug/update-pin` | `UpdatePin` (PR1 service) |
| W6 | DELETE | `/v1/workspace/:slug` | `Delete(ctx, slug)` |
| W7 | GET | `/v1/workspace/:slug/chats` | `GetChats(ctx, ws.ID)` |
| W8 | POST | `/v1/workspace/:slug/stream-chat` | SSE; nil user / nil threadID |
| W9 | POST | `/v1/workspace/:slug/chat` | **Relocated from `main.go:168`** |
| W10 | POST | `/v1/workspace/:slug/vector-search` | `VectorSearchService.Search` |
| W11 | POST | `/v1/workspace/:slug/update-embeddings` | `DocumentService.UpdateEmbeddings` |

#### Document (12)

| # | Method | Path | Notes |
|---|---|---|---|
| D1 | POST | `/v1/document/upload` | multipart; folder=`custom-documents` |
| D2 | POST | `/v1/document/upload/:folderName` | multipart; specified folder |
| D3 | POST | `/v1/document/upload-link` | `{link, addToWorkspaces, metadata}` |
| D4 | POST | `/v1/document/raw-text` | `DocumentService.SaveRawText` (PR1) |
| D5 | POST | `/v1/document/create-folder` | `{name}` |
| D6 | POST | `/v1/document/move-files` | `{files: [{from,to}]}` |
| D7 | GET | `/v1/documents` | flat list |
| D8 | GET | `/v1/documents/folder/:folderName` | folder list |
| D9 | GET | `/v1/document/:docName` | single doc fetch |
| D10 | GET | `/v1/document/metadata-schema` | hardcoded `{schema: {...}}` |
| D11 | GET | `/v1/document/accepted-file-types` | from `FileSystemService.AcceptedDocumentTypes()` |
| D12 | DELETE | `/v1/document/remove-folder` | `RemoveFolder` (PR1) |

#### Thread (6)

| # | Method | Path |
|---|---|---|
| T1 | POST | `/v1/workspace/:slug/thread/new` |
| T2 | POST | `/v1/workspace/:slug/thread/:threadSlug/update` |
| T3 | DELETE | `/v1/workspace/:slug/thread/:threadSlug` |
| T4 | GET | `/v1/workspace/:slug/thread/:threadSlug/chats` |
| T5 | POST | `/v1/workspace/:slug/thread/:threadSlug/chat` |
| T6 | POST | `/v1/workspace/:slug/thread/:threadSlug/stream-chat` |

### Out of scope (explicit)

- **OpenAI compat layer** — PR5.
- **Document `Collector.online()` health check** before upload — Node short-circuits with 500 when Collector is offline. Go relies on `s.docSvc.UploadToWorkspace` failure return; if Collector is down the upload errors with the underlying message. **Not** an explicit pre-check in PR4.
- **`/v1/document/upload-link` multi-workspace fan-out** — Node loops `addToWorkspaces` and binds. Go's `UploadLink(wsSlug, link, progressMgr)` takes a single slug; handler loops if multiple slugs provided. This is **intentional** but document as known gap if multi-bind semantics differ in edge cases.
- **`workspace.workspace_users` admin scoping** — `/v1/workspaces` returns ALL workspaces (no per-user filter) since API context is anonymous. Same as Node.
- **EventLogs / Telemetry** — deferred to platform-wide wire-up.
- **API key role checks** (§3.4) — Node + Go parity: any valid key.

### Response-shape conventions (match Node exactly)

- All success responses: HTTP **200**. Errors that are user-facing return `{success: false, error: "..."}` or `{...: null, error: "..."}` at HTTP 200.
- `4xx/5xx` only for malformed body, missing rows (404), or server faults.
- Workspace endpoints return `{workspace: ..., message: "..."}` (singular create) and `{workspaces: [...]}` (list).
- Document upload returns `{success, error, documents}` where `documents` is the array of created `WorkspaceDocument` rows (or `[]` on workspace bind failure).
- Streaming uses `text/event-stream` with the existing `writeSSEChunk(...)` helper.
- DELETE returns `{success: true, error: null}` unless Node has a specific shape (e.g. `/v1/workspace/:slug` returns `{message: "..."}` in some Node paths — audit before mirroring).

### Test setup helper

Reuse the `newAPITestEnv(t, cfg)` helper introduced in PR3 (`api_setup_test.go`). For PR4 tests that need workspace/thread middleware context, build them with `db` so `ValidWorkspaceSlug(db)` resolves. If PR3 has not landed yet, copy the same helper into `api_setup_test.go` as part of PR4 Task 1 (and remove duplication when PR3 lands).

### TDD discipline

Each task: write failing test → run + confirm fail → minimal impl → run + confirm pass → commit. Tests are HTTP-level; for streaming use a `httptest.NewRecorder` and assert on `rec.Body.String()` containing expected `data: {...}` SSE frames.

---

## Task 1: `api_workspace.go` — CRUD (6 routes: W1–W6)

**Files:**
- Create: `backend/internal/handlers/api_workspace.go`
- Create: `backend/internal/handlers/api_workspace_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// File: backend/internal/handlers/api_workspace_test.go
package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Helper to register the workspace CRUD routes for tests.
func registerWorkspaceCRUDForTest(env *apiTestEnv) {
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIWorkspaceCRUDRoutes(api, env.APIKeySvc, wsSvc, env.DB)
}

func TestAPIWorkspace_List(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "a", Slug: "a"}).Error)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "b", Slug: "b"}).Error)
    registerWorkspaceCRUDForTest(env)

    req := httptest.NewRequest("GET", "/api/v1/workspaces", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Workspaces []models.Workspace `json:"workspaces"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    require.Len(t, body.Workspaces, 2)
}

func TestAPIWorkspace_Create(t *testing.T) {
    env := newAPITestEnv(t, nil)
    registerWorkspaceCRUDForTest(env)

    payload, _ := json.Marshal(map[string]string{"name": "new-ws"})
    req := httptest.NewRequest("POST", "/api/v1/workspace/new", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Workspace *models.Workspace `json:"workspace"`
        Message   string            `json:"message"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    require.NotNil(t, body.Workspace)
    assert.Equal(t, "new-ws", body.Workspace.Name)
}

func TestAPIWorkspace_GetBySlug(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w-slug"}).Error)
    registerWorkspaceCRUDForTest(env)

    req := httptest.NewRequest("GET", "/api/v1/workspace/w-slug", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Workspace *models.Workspace `json:"workspace"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Equal(t, "w-slug", body.Workspace.Slug)
}

func TestAPIWorkspace_Update(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
    registerWorkspaceCRUDForTest(env)

    payload := []byte(`{"name":"renamed"}`)
    req := httptest.NewRequest("POST", "/api/v1/workspace/w/update", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var got models.Workspace
    require.NoError(t, env.DB.Where("slug=?", "w").First(&got).Error)
    assert.Equal(t, "renamed", got.Name)
}

func TestAPIWorkspace_UpdatePin(t *testing.T) {
    env := newAPITestEnv(t, nil)
    ws := &models.Workspace{Name: "w", Slug: "w"}
    require.NoError(t, env.DB.Create(ws).Error)
    f := false
    require.NoError(t, env.DB.Create(&models.WorkspaceDocument{
        DocId: "d1", Filename: "a.txt", Docpath: "custom-documents/a.json",
        WorkspaceID: ws.ID, Pinned: &f,
    }).Error)
    registerWorkspaceCRUDForTest(env)

    payload := []byte(`{"docPath":"custom-documents/a.json","pinStatus":true}`)
    req := httptest.NewRequest("POST", "/api/v1/workspace/w/update-pin", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIWorkspace_Delete(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
    registerWorkspaceCRUDForTest(env)

    req := httptest.NewRequest("DELETE", "/api/v1/workspace/w", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)
    require.Equal(t, http.StatusOK, rec.Code)

    var count int64
    env.DB.Model(&models.Workspace{}).Count(&count)
    assert.Equal(t, int64(0), count)
    _ = context.Background()
}
```

- [ ] **Step 2: Run + confirm fail**

```bash
cd backend && go test ./internal/handlers/ -run TestAPIWorkspace -count=1
```

- [ ] **Step 3: Implement**

```go
// File: backend/internal/handlers/api_workspace.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "gorm.io/gorm"
)

type APIWorkspaceHandler struct {
    wsSvc *services.WorkspaceService
}

func NewAPIWorkspaceHandler(wsSvc *services.WorkspaceService) *APIWorkspaceHandler {
    return &APIWorkspaceHandler{wsSvc: wsSvc}
}

func (h *APIWorkspaceHandler) List(c *gin.Context) {
    // API context: pass userID=0 → service returns ALL workspaces (Node parity).
    workspaces, err := h.wsSvc.List(c.Request.Context(), 0)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *APIWorkspaceHandler) Create(c *gin.Context) {
    var req dto.CreateWorkspaceRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    ws, err := h.wsSvc.Create(c.Request.Context(), 0, req)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"workspace": ws, "message": "Workspace created"})
}

func (h *APIWorkspaceHandler) Get(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    c.JSON(http.StatusOK, gin.H{"workspace": ws})
}

func (h *APIWorkspaceHandler) Update(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.UpdateWorkspaceRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req); err != nil {
        c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": err.Error()})
        return
    }
    updated, _ := h.wsSvc.GetBySlug(c.Request.Context(), ws.Slug)
    c.JSON(http.StatusOK, gin.H{"workspace": updated, "message": "Workspace updated"})
}

func (h *APIWorkspaceHandler) UpdatePin(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.APIUpdatePinRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if err := h.wsSvc.UpdatePin(c.Request.Context(), ws.ID, req.DocPath, req.PinValue); err != nil {
        if err == gorm.ErrRecordNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Pin status updated successfully"})
}

func (h *APIWorkspaceHandler) Delete(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    if err := h.wsSvc.Delete(c.Request.Context(), ws.Slug); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Workspace " + ws.Slug + " deleted"})
}

func RegisterAPIWorkspaceCRUDRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, wsSvc *services.WorkspaceService, db *gorm.DB) {
    h := NewAPIWorkspaceHandler(wsSvc)
    r.GET("/v1/workspaces", middleware.ValidAPIKey(apiKeySvc), h.List)
    r.POST("/v1/workspace/new", middleware.ValidAPIKey(apiKeySvc), h.Create)
    r.GET("/v1/workspace/:slug", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Get)
    r.POST("/v1/workspace/:slug/update", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Update)
    r.POST("/v1/workspace/:slug/update-pin", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.UpdatePin)
    r.DELETE("/v1/workspace/:slug", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Delete)
}
```

- [ ] **Step 4: Run + commit**

```bash
cd backend && go test ./internal/handlers/ -run TestAPIWorkspace -count=1
git add backend/internal/handlers/api_workspace.go backend/internal/handlers/api_workspace_test.go
git commit -m "feat(api-v1): /v1/workspaces + /v1/workspace/:slug CRUD (W1-W6)"
```

> **Audit point**: `WorkspaceService.Create(ctx, userID int, ...)` non-pointer `userID`. Passing 0 means `created_by_user_id = 0` which is valid in sqlite but may fail on Postgres if FK exists. If FK enforcement matters, extend service signature to `*int` (separate hardening PR, not blocking PR4).

---

## Task 2: `api_workspace.go` — chats / vector-search / update-embeddings (W7, W10, W11)

3 routes: GET `/v1/workspace/:slug/chats`, POST `/v1/workspace/:slug/vector-search`, POST `/v1/workspace/:slug/update-embeddings`.

- [ ] **Step 1: Failing tests**

```go
func TestAPIWorkspace_Chats(t *testing.T)            { /* seed 2 workspace_chats; expect 200 + history list */ }
func TestAPIWorkspace_VectorSearch_NoEmbedder(t *testing.T) { /* nil emb → 503 with "embedder not configured" */ }
func TestAPIWorkspace_UpdateEmbeddings(t *testing.T)  { /* POST {adds:[], removes:[]}; expect 200 + {workspace,message} */ }
```

- [ ] **Step 2-3: Implement**

```go
func (h *APIWorkspaceHandler) Chats(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    history, err := h.wsSvc.GetChats(c.Request.Context(), ws.ID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"history": history})
}

// New struct fields on APIWorkspaceHandler:
type APIWorkspaceHandler struct {
    wsSvc        *services.WorkspaceService
    vectorSearch *services.VectorSearchService // nil-tolerant
    docSvc       *services.DocumentService
}

func (h *APIWorkspaceHandler) VectorSearch(c *gin.Context) {
    if h.vectorSearch == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embedder not configured"})
        return
    }
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.VectorSearchRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    results, err := h.vectorSearch.Search(c.Request.Context(), ws, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *APIWorkspaceHandler) UpdateEmbeddings(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.UpdateEmbeddingsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if err := h.docSvc.UpdateEmbeddings(c.Request.Context(), ws.Slug, req.Adds, req.Deletes); err != nil {
        c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": err.Error()})
        return
    }
    updated, _ := h.wsSvc.GetBySlug(c.Request.Context(), ws.Slug)
    c.JSON(http.StatusOK, gin.H{"workspace": updated, "message": "Workspace embeddings updated"})
}
```

Refactor `NewAPIWorkspaceHandler` to accept the new deps (default nil-tolerant). Append to `RegisterAPIWorkspaceCRUDRoutes` (rename to `RegisterAPIWorkspaceRoutes` since it now covers more than CRUD):

```go
func RegisterAPIWorkspaceRoutes(
    r *gin.RouterGroup,
    apiKeySvc *services.APIKeyService,
    wsSvc *services.WorkspaceService,
    vectorSearch *services.VectorSearchService,
    docSvc *services.DocumentService,
    db *gorm.DB,
) {
    h := &APIWorkspaceHandler{wsSvc: wsSvc, vectorSearch: vectorSearch, docSvc: docSvc}
    // ... existing W1-W6 ...
    r.GET("/v1/workspace/:slug/chats", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Chats)
    r.POST("/v1/workspace/:slug/vector-search", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.VectorSearch)
    r.POST("/v1/workspace/:slug/update-embeddings", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.UpdateEmbeddings)
}
```

> **Audit point**: `dto.UpdateEmbeddingsRequest` field names — verify if it's `Adds/Deletes` or `Adds/Removes`. Node sends `adds: [], deletes: []`. Adjust accordingly.

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/workspace/:slug chats + vector-search + update-embeddings (W7,W10,W11)"
```

---

## Task 3: `api_workspace.go` — chat + stream-chat (W8, W9) & relocate from main.go

This task **relocates** the temporary `/v1/workspace/:slug/chat` mount from `main.go:167-171` into `api_workspace.go` and adds `/v1/workspace/:slug/stream-chat`.

- [ ] **Step 1: Failing tests**

```go
func TestAPIWorkspace_Chat(t *testing.T) {
    /* Build env with mock ChatService that returns a fixed ChatResponse.
       POST {"message":"hi"} → 200 + textResponse. */
}
func TestAPIWorkspace_StreamChat(t *testing.T) {
    /* Same mock; assert response body contains "data: " SSE frames. */
}
```

(For the stream test, ChatService is hard to mock without an interface. If mocking is non-trivial, smoke-test with a partial Stream by writing a no-op LLM provider that emits a single chunk + close — same pattern used in chat_service_test.go if it exists.)

- [ ] **Step 2: Implement handlers**

```go
type APIWorkspaceHandler struct {
    wsSvc        *services.WorkspaceService
    chatSvc      *services.ChatService
    vectorSearch *services.VectorSearchService
    docSvc       *services.DocumentService
}

func (h *APIWorkspaceHandler) Chat(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
        return
    }
    // API context: no user, no thread.
    resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, nil, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
        return
    }
    c.JSON(http.StatusOK, resp)
}

func (h *APIWorkspaceHandler) StreamChat(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.StreamChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
        return
    }

    stream, err := h.chatSvc.Stream(c.Request.Context(), ws, nil, nil, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
        return
    }

    for chunk := range stream {
        if err := writeSSEChunk(c.Writer, chunk); err != nil {
            break
        }
        if f, ok := c.Writer.(http.Flusher); ok {
            f.Flush()
        }
    }
}
```

Update `RegisterAPIWorkspaceRoutes` signature + body:

```go
func RegisterAPIWorkspaceRoutes(
    r *gin.RouterGroup,
    apiKeySvc *services.APIKeyService,
    wsSvc *services.WorkspaceService,
    chatSvc *services.ChatService,
    vectorSearch *services.VectorSearchService,
    docSvc *services.DocumentService,
    db *gorm.DB,
) {
    h := &APIWorkspaceHandler{
        wsSvc: wsSvc, chatSvc: chatSvc,
        vectorSearch: vectorSearch, docSvc: docSvc,
    }
    // ... W1-W7, W10, W11 from Tasks 1+2 ...
    r.POST("/v1/workspace/:slug/chat",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceSlug(db),
        h.Chat)
    r.POST("/v1/workspace/:slug/stream-chat",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceSlug(db),
        h.StreamChat)
}
```

- [ ] **Step 3: Remove temporary mount from `main.go`**

Delete these lines (`main.go:166-171`):

```go
// API v1 routes (API key auth)
chatHandler := handlers.NewChatHandler(chatSvc)
v1 := api.Group("/v1")
v1.POST("/workspace/:slug/chat",
    middleware.ValidAPIKey(apiKeySvc),
    middleware.ValidWorkspaceSlug(db),
    chatHandler.ApiChat)
```

Replace with a comment marker for Task 9's full wiring (placeholder — Task 9 inserts the actual calls):

```go
// API v1 routes registered via handlers.RegisterAPI*Routes (see PR3/PR4 wiring below).
```

Build immediately to confirm no orphan imports:

```bash
cd backend && go build ./...
```

If `middleware` is unused after removal, audit other uses (it's used elsewhere in the file: lines 169 — but only by the removed line itself).

- [ ] **Step 4: Run full suite + commit**

```bash
cd backend && go test ./... -count=1
git commit -m "feat(api-v1): /v1/workspace/:slug chat + stream-chat (W8,W9); retire main.go temp mount"
```

> **Audit point**: If the existing `tests/integration/` or unit tests refer to the temporary `/api/v1/workspace/:slug/chat` mount, they will fail until Task 9 wires it back. Mark this task as "blocking compile of main.go" if the build breaks — Task 9 fix-up is required before next commit.

---

## Task 4: `api_document.go` — uploads (D1, D2, D3, D4)

4 routes: POST `/v1/document/upload`, POST `/v1/document/upload/:folderName`, POST `/v1/document/upload-link`, POST `/v1/document/raw-text`.

**Files:**
- Create: `backend/internal/handlers/api_document.go`
- Create: `backend/internal/handlers/api_document_test.go`

- [ ] **Step 1: Failing tests**

```go
func TestAPIDocument_RawText_Success(t *testing.T) {
    /* POST /v1/document/raw-text with {textContent, title, metadata:{title:"x"}};
       expect 200 + {success:true, documents:[...]}; verify file under custom-documents/ exists */
}
func TestAPIDocument_RawText_MissingTitle(t *testing.T) {
    /* POST without metadata.title; expect 422 + {success:false, error:"..."} matching Node */
}
func TestAPIDocument_RawText_EmptyContent(t *testing.T) {
    /* POST with empty textContent; expect 422 + error */
}
// Multipart upload tests can be skipped if Collector is not wired in test env;
// document-level upload-link can also stub via UploadLink mock or use a real
// http test server if needed. If Collector dep is hard to mock, mark these
// tests with t.Skip and rely on integration tests in tests/integration/.
```

- [ ] **Step 2-3: Implement**

```go
// File: backend/internal/handlers/api_document.go
package handlers

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/services"
)

type APIDocumentHandler struct {
    docSvc      *services.DocumentService
    fs          *services.FileSystemService
    progressMgr *services.EmbeddingProgressManager
}

func NewAPIDocumentHandler(docSvc *services.DocumentService, fs *services.FileSystemService, progressMgr *services.EmbeddingProgressManager) *APIDocumentHandler {
    return &APIDocumentHandler{docSvc: docSvc, fs: fs, progressMgr: progressMgr}
}

// splitSlugs splits Node's comma-delimited `addToWorkspaces` string into a slice.
// Returns nil for empty input.
func splitSlugs(s string) []string {
    s = strings.TrimSpace(s)
    if s == "" {
        return nil
    }
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if p = strings.TrimSpace(p); p != "" {
            out = append(out, p)
        }
    }
    return out
}

func (h *APIDocumentHandler) Upload(c *gin.Context) {
    h.handleUpload(c, "custom-documents")
}

func (h *APIDocumentHandler) UploadToFolder(c *gin.Context) {
    h.handleUpload(c, c.Param("folderName"))
}

func (h *APIDocumentHandler) handleUpload(c *gin.Context, folder string) {
    fileHeader, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "file field required"})
        return
    }
    addTo := c.PostForm("addToWorkspaces")
    slugs := splitSlugs(addTo)

    if len(slugs) == 0 {
        // No workspace bind — still save file to disk via SaveUpload(workspaceID=0).
        // SaveUpload assumes a workspace; fall back to FileSystem-only path.
        // Audit: if SaveUpload(0,...) creates a stray workspace_documents row, instead
        // call FileSystemService.SaveFile + skip DB.
        c.JSON(http.StatusOK, gin.H{
            "success":   false,
            "error":     "addToWorkspaces required when no folder workspace binding is implicit",
            "documents": []any{},
        })
        return
    }

    var created []any
    for _, slug := range slugs {
        doc, err := h.docSvc.UploadToWorkspace(c.Request.Context(), slug, fileHeader, h.progressMgr)
        if err != nil {
            c.JSON(http.StatusOK, gin.H{
                "success": false, "error": err.Error(), "documents": created,
            })
            return
        }
        created = append(created, doc)
    }
    _ = folder // folder parameter currently advisory; multi-folder upload path can be added if needed
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "documents": created})
}

func (h *APIDocumentHandler) UploadLink(c *gin.Context) {
    var req struct {
        Link            string `json:"link"`
        AddToWorkspaces string `json:"addToWorkspaces"`
        Metadata        any    `json:"metadata"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    slugs := splitSlugs(req.AddToWorkspaces)
    if len(slugs) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false, "error": "addToWorkspaces is required", "documents": []any{},
        })
        return
    }
    var created []any
    for _, slug := range slugs {
        docs, err := h.docSvc.UploadLink(c.Request.Context(), slug, req.Link, h.progressMgr)
        if err != nil {
            c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "documents": created})
            return
        }
        for _, d := range docs {
            created = append(created, d)
        }
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "documents": created})
}

func (h *APIDocumentHandler) RawText(c *gin.Context) {
    var req dto.APIRawTextRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    if req.Text == "" {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "error": "The 'textContent' key cannot have an empty value."})
        return
    }
    // Require metadata.title (Node parity).
    md, _ := req.Metadata.(map[string]any)
    if md == nil || md["title"] == nil || md["title"].(string) == "" {
        c.JSON(http.StatusUnprocessableEntity, gin.H{
            "success": false,
            "error":   "You are missing required metadata key:value pairs in your request. Required metadata key:values are 'title'",
        })
        return
    }
    title, _ := md["title"].(string)
    slugs := splitSlugs(req.AddToWorkspaces)
    docs, err := h.docSvc.SaveRawText(c.Request.Context(), req.Text, title, md, slugs)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "documents": docs})
}

func RegisterAPIDocumentRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, docSvc *services.DocumentService, fs *services.FileSystemService, progressMgr *services.EmbeddingProgressManager) {
    h := NewAPIDocumentHandler(docSvc, fs, progressMgr)
    r.POST("/v1/document/upload", middleware.ValidAPIKey(apiKeySvc), h.Upload)
    r.POST("/v1/document/upload/:folderName", middleware.ValidAPIKey(apiKeySvc), h.UploadToFolder)
    r.POST("/v1/document/upload-link", middleware.ValidAPIKey(apiKeySvc), h.UploadLink)
    r.POST("/v1/document/raw-text", middleware.ValidAPIKey(apiKeySvc), h.RawText)
    // D5-D12 added in Tasks 5-7.
}
```

- [ ] **Step 4: Run + commit**

```bash
cd backend && go test ./internal/handlers/ -run TestAPIDocument_RawText -count=1
git commit -m "feat(api-v1): /v1/document upload + upload-link + raw-text (D1-D4)"
```

> **Audit point**: `DocumentService.UploadToWorkspace(ctx, wsSlug, fileHeader, progressMgr)` returns a single `*WorkspaceDocument`. If Node's per-upload semantics return parsed metadata richer than the DB row (Collector output), responses will differ. Document as known gap if integration consumers complain.

---

## Task 5: `api_document.go` — folder ops (D5, D6, D12)

3 routes: POST `/v1/document/create-folder`, POST `/v1/document/move-files`, DELETE `/v1/document/remove-folder`.

- [ ] **Step 1: Failing tests** — folder create/move/remove integration tests using `t.TempDir()` for `cfg.StorageDir`.

- [ ] **Step 2-3: Implement**

```go
func (h *APIDocumentHandler) CreateFolder(c *gin.Context) {
    var req struct { Name string `json:"name"` }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    if req.Name == "" {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "name required"})
        return
    }
    if err := h.docSvc.CreateFolder(c.Request.Context(), req.Name); err != nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Folder created"})
}

func (h *APIDocumentHandler) MoveFiles(c *gin.Context) {
    var req struct {
        Files []dto.FileMove `json:"files"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    res, err := h.docSvc.MoveFiles(c.Request.Context(), req.Files)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": res})
}

func (h *APIDocumentHandler) RemoveFolder(c *gin.Context) {
    var req dto.APIDocumentRemoveFolderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    if err := h.docSvc.RemoveFolder(c.Request.Context(), req.Name); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to remove folder: " + err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Folder removed successfully"})
}
```

Append to `RegisterAPIDocumentRoutes`:

```go
r.POST("/v1/document/create-folder", middleware.ValidAPIKey(apiKeySvc), h.CreateFolder)
r.POST("/v1/document/move-files", middleware.ValidAPIKey(apiKeySvc), h.MoveFiles)
r.DELETE("/v1/document/remove-folder", middleware.ValidAPIKey(apiKeySvc), h.RemoveFolder)
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/document folder ops — create / move / remove (D5,D6,D12)"
```

---

## Task 6: `api_document.go` — listing (D7, D8, D9)

3 routes: GET `/v1/documents`, GET `/v1/documents/folder/:folderName`, GET `/v1/document/:docName`.

- [ ] **Step 1: Failing tests**

```go
func TestAPIDocument_List(t *testing.T)            { /* seed docs; GET; expect {localFiles:{...}} or {documents:[...]} per Node shape */ }
func TestAPIDocument_FolderList(t *testing.T)      { /* GET .../folder/x; expect filtered */ }
func TestAPIDocument_GetByDocName(t *testing.T)    { /* seed doc; GET .../document/:docName; expect 200 + payload */ }
func TestAPIDocument_GetByDocName_NotFound(t *testing.T) { /* expect 404 */ }
```

> **Audit Node response shape**: `/v1/documents` returns `{localFiles: {...}}` per `viewLocalFiles` recursive tree, not a flat array. Verify in `server/endpoints/api/document/index.js:622` and adjust handler to call `FileSystemService.ListLocalFiles("")` returning the tree, NOT `DocumentService.ListDocuments`.

- [ ] **Step 2-3: Implement**

```go
func (h *APIDocumentHandler) ListAll(c *gin.Context) {
    files, err := h.fs.ListLocalFiles("")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"localFiles": files})
}

func (h *APIDocumentHandler) ListFolder(c *gin.Context) {
    folder := c.Param("folderName")
    files, err := h.fs.ListLocalFiles(folder)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"localFiles": files})
}

func (h *APIDocumentHandler) GetByDocName(c *gin.Context) {
    docName := c.Param("docName")
    doc, err := h.docSvc.GetByDocName(c.Request.Context(), docName)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"document": doc})
}
```

Append:

```go
r.GET("/v1/documents", middleware.ValidAPIKey(apiKeySvc), h.ListAll)
r.GET("/v1/documents/folder/:folderName", middleware.ValidAPIKey(apiKeySvc), h.ListFolder)
// IMPORTANT: register /v1/document/:docName AFTER hardcoded routes (Task 7) per Node ordering note.
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/documents + /v1/document/:docName (D7-D9)"
```

---

## Task 7: `api_document.go` — hardcoded responses (D10, D11) + `:docName` registration order

2 routes: GET `/v1/document/metadata-schema`, GET `/v1/document/accepted-file-types`. **Crucial**: `/v1/document/:docName` (D9) must be registered AFTER these — otherwise Gin treats `metadata-schema` and `accepted-file-types` as path parameters and routes them to `GetByDocName`.

- [ ] **Step 1: Failing tests**

```go
func TestAPIDocument_MetadataSchema(t *testing.T) {
    /* GET /v1/document/metadata-schema; expect {schema:{title:"string", ...}} */
}
func TestAPIDocument_AcceptedFileTypes(t *testing.T) {
    /* GET /v1/document/accepted-file-types; expect {types:{".txt":"text/plain", ...}} */
}
func TestAPIDocument_DocNameDoesNotShadow(t *testing.T) {
    /* GET /v1/document/metadata-schema after registering :docName route afterwards;
       still returns the schema response, not 404 from doc lookup. */
}
```

- [ ] **Step 2-3: Implement**

```go
func (h *APIDocumentHandler) MetadataSchema(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "schema": gin.H{
            "url":         "string | nullable",
            "title":       "string",
            "docAuthor":   "string | nullable",
            "description": "string | nullable",
            "docSource":   "string | nullable",
            "chunkSource": "string | nullable",
            "published":   "epoch timestamp in ms | nullable",
        },
    })
}

func (h *APIDocumentHandler) AcceptedFileTypes(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"types": h.fs.AcceptedDocumentTypes()})
}
```

**Reorder `RegisterAPIDocumentRoutes`** so the registration sequence is:

```go
// ... uploads / folder ops ...

// Hardcoded specific paths FIRST
r.GET("/v1/document/metadata-schema", middleware.ValidAPIKey(apiKeySvc), h.MetadataSchema)
r.GET("/v1/document/accepted-file-types", middleware.ValidAPIKey(apiKeySvc), h.AcceptedFileTypes)
r.GET("/v1/documents", middleware.ValidAPIKey(apiKeySvc), h.ListAll)
r.GET("/v1/documents/folder/:folderName", middleware.ValidAPIKey(apiKeySvc), h.ListFolder)

// Param routes LAST — Gin treats earlier exact matches first.
r.GET("/v1/document/:docName", middleware.ValidAPIKey(apiKeySvc), h.GetByDocName)
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): /v1/document metadata-schema + accepted-file-types (D10,D11) with route ordering fix"
```

---

## Task 8: `api_thread.go` — all 6 thread routes

**Files:**
- Create: `backend/internal/handlers/api_thread.go`
- Create: `backend/internal/handlers/api_thread_test.go`

- [ ] **Step 1: Failing tests** — 6 short HTTP tests, one per route.

```go
func TestAPIThread_Create(t *testing.T)       { /* POST .../thread/new {name:"x"}; expect 200 + {thread, message} */ }
func TestAPIThread_Update(t *testing.T)       { /* POST .../thread/:slug/update {name:"y"}; expect 200 */ }
func TestAPIThread_Delete(t *testing.T)       { /* DELETE .../thread/:slug; expect 200 */ }
func TestAPIThread_Chats(t *testing.T)        { /* seed thread chats; GET; expect list */ }
func TestAPIThread_Chat(t *testing.T)         { /* mock ChatSvc; expect 200 */ }
func TestAPIThread_StreamChat(t *testing.T)   { /* assert SSE frames */ }
```

- [ ] **Step 2-3: Implement**

```go
// File: backend/internal/handlers/api_thread.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/odysseythink/hermind/backend/pkg/utils"
    "gorm.io/gorm"
)

type APIThreadHandler struct {
    threadSvc *services.ThreadService
    chatSvc   *services.ChatService
}

func NewAPIThreadHandler(threadSvc *services.ThreadService, chatSvc *services.ChatService) *APIThreadHandler {
    return &APIThreadHandler{threadSvc: threadSvc, chatSvc: chatSvc}
}

func (h *APIThreadHandler) Create(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.CreateThreadRequest
    _ = c.ShouldBindJSON(&req) // body optional (Node accepts empty)
    thread, err := h.threadSvc.Create(c.Request.Context(), ws.ID, nil, req)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"thread": nil, "message": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"thread": thread, "message": "Thread created"})
}

func (h *APIThreadHandler) Update(c *gin.Context) {
    thread := c.MustGet("thread").(*models.WorkspaceThread)
    var req dto.UpdateThreadRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"thread": nil, "message": err.Error()})
        return
    }
    if err := h.threadSvc.Update(c.Request.Context(), thread, req); err != nil {
        c.JSON(http.StatusOK, gin.H{"thread": nil, "message": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"thread": thread, "message": "Thread updated"})
}

func (h *APIThreadHandler) Delete(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    thread := c.MustGet("thread").(*models.WorkspaceThread)
    if err := h.threadSvc.Delete(c.Request.Context(), ws.ID, thread.Slug); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Thread deleted"})
}

func (h *APIThreadHandler) GetChats(c *gin.Context) {
    thread := c.MustGet("thread").(*models.WorkspaceThread)
    history, err := h.threadSvc.GetThreadChats(c.Request.Context(), thread.ID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *APIThreadHandler) Chat(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    thread := c.MustGet("thread").(*models.WorkspaceThread)
    var req dto.ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
        return
    }
    resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, &thread.ID, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
        return
    }
    c.JSON(http.StatusOK, resp)
}

func (h *APIThreadHandler) StreamChat(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    ws := c.MustGet("workspace").(*models.Workspace)
    thread := c.MustGet("thread").(*models.WorkspaceThread)
    var req dto.StreamChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
        return
    }
    stream, err := h.chatSvc.Stream(c.Request.Context(), ws, nil, &thread.ID, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
        return
    }
    for chunk := range stream {
        if err := writeSSEChunk(c.Writer, chunk); err != nil {
            break
        }
        if f, ok := c.Writer.(http.Flusher); ok {
            f.Flush()
        }
    }
}

func RegisterAPIThreadRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, threadSvc *services.ThreadService, chatSvc *services.ChatService, db *gorm.DB) {
    h := NewAPIThreadHandler(threadSvc, chatSvc)
    // /thread/new only has :slug — use the single-slug middleware.
    r.POST("/v1/workspace/:slug/thread/new",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceSlug(db),
        h.Create)
    // Routes with both :slug and :threadSlug use the combined middleware,
    // which sets BOTH c.MustGet("workspace") and c.MustGet("thread").
    r.POST("/v1/workspace/:slug/thread/:threadSlug/update",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceAndThreadSlug(db),
        h.Update)
    r.DELETE("/v1/workspace/:slug/thread/:threadSlug",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceAndThreadSlug(db),
        h.Delete)
    r.GET("/v1/workspace/:slug/thread/:threadSlug/chats",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceAndThreadSlug(db),
        h.GetChats)
    r.POST("/v1/workspace/:slug/thread/:threadSlug/chat",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceAndThreadSlug(db),
        h.Chat)
    r.POST("/v1/workspace/:slug/thread/:threadSlug/stream-chat",
        middleware.ValidAPIKey(apiKeySvc),
        middleware.ValidWorkspaceAndThreadSlug(db),
        h.StreamChat)
}
```

> **Audit point**: `middleware.ValidWorkspaceAndThreadSlug(db)` already exists at `middleware/workspace_thread.go:12` — it handles BOTH `:slug` and `:threadSlug` in one pass and sets `c.MustGet("workspace")` + `c.MustGet("thread")`. No new middleware needed. Just don't double-chain `ValidWorkspaceSlug` before it.

- [ ] **Step 4: Run + commit**

```bash
cd backend && go test ./internal/handlers/ -run TestAPIThread -count=1
git commit -m "feat(api-v1): /v1/workspace/:slug/thread/* (T1-T6)"
```

---

## Task 9: main.go — wire all PR4 handlers + retire temporary chat mount

- [ ] **Step 1: Edit `main.go`**

Inside the `api := r.Group("/api")` block, after `RegisterAPIEmbedRoutes`:

```go
handlers.RegisterAPIWorkspaceRoutes(api, apiKeySvc, wsSvc, chatSvc, vectorSearchSvc, docSvc, db)
handlers.RegisterAPIDocumentRoutes(api, apiKeySvc, docSvc, fsSvc, progressMgr)
handlers.RegisterAPIThreadRoutes(api, apiKeySvc, threadSvc, chatSvc, db)
```

If Task 3 already removed the temporary chat mount and the placeholder comment, no further deletions needed. Otherwise remove `main.go:166-171`:

```go
// REMOVE:
// chatHandler := handlers.NewChatHandler(chatSvc)
// v1 := api.Group("/v1")
// v1.POST("/workspace/:slug/chat", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), chatHandler.ApiChat)
```

- [ ] **Step 2: Build + run full suite**

```bash
cd backend && go build ./...
cd backend && go test ./... -count=1
```

- [ ] **Step 3: Smoke test** (manual)

```bash
cd backend && go run ./cmd/server &
sleep 2
curl -H "Authorization: Bearer <KEY>" http://localhost:8080/api/v1/workspaces
curl -H "Authorization: Bearer <KEY>" http://localhost:8080/api/v1/document/metadata-schema
# kill server
```

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(api-v1): wire 29 workspace/document/thread handlers in main.go"
```

---

## Task 10: Full-suite verify + lint

- [ ] `go test ./... -count=1` — green.
- [ ] `go vet ./...` — clean.
- [ ] `go build ./...` — succeeds.
- [ ] Manually verify in browser-less env that:
  - `/api/v1/auth` (if PR3 landed) still works.
  - `/api/v1/workspaces` returns expected list.
  - `/api/v1/document/metadata-schema` returns the schema.
  - `/api/v1/document/:docName` does NOT shadow the hardcoded routes.
- [ ] No new lint or vet warnings.

---

## Acceptance criteria

- [ ] 29 routes mounted under `/api/v1/...`. Auth gate: `middleware.ValidAPIKey`. Workspace-only gate: `middleware.ValidWorkspaceSlug(db)`. Combined workspace+thread gate: `middleware.ValidWorkspaceAndThreadSlug(db)`.
- [ ] `/v1/workspaces` returns all workspaces with no per-user filter.
- [ ] `/v1/workspace/:slug/chat` and `.../stream-chat` produce equivalent output to the existing web `/api/workspace/:slug/chat` (sans user attribution).
- [ ] `/v1/workspace/:slug/update-pin` returns 404 for unknown docPath.
- [ ] `/v1/document/raw-text` enforces `metadata.title`; empty `textContent` returns 422.
- [ ] `/v1/document/upload` requires `addToWorkspaces`; multi-slug fan-out works.
- [ ] `/v1/document/metadata-schema` and `/v1/document/accepted-file-types` do NOT get shadowed by `/v1/document/:docName`.
- [ ] `/v1/document/remove-folder` refuses `custom-documents` (PR1 behavior).
- [ ] Thread routes: 5 of 6 require both workspace + thread slug middleware; thread/new requires workspace only.
- [ ] Temporary `v1.POST("/workspace/:slug/chat", ...)` block at `main.go:166-171` removed.
- [ ] All pre-existing tests still pass.

---

## Known gaps after PR4 (track but DO NOT implement here)

1. **Document upload without workspace bind** — `/v1/document/upload` without `addToWorkspaces` currently returns an error in PR4 (no service path takes only a file without workspace). Node tolerates this (saves to disk under `custom-documents/`). Add a `DocumentService.SaveOrphanUpload` path if needed.
2. **`UploadLink` multi-workspace progressMgr** — handler loops slugs, each call creates a fresh progress entry. UX may show N progress streams. Acceptable for v1.
3. **Collector online-check** — Node's pre-check returns 500 fast; Go relies on downstream errors. May want a `services.CollectorClient.Ping(ctx)` pre-check in Task 4 handlers for faster failures.
4. **`workspace.created_by_user_id = 0`** — API context bypass; if Postgres FK enforces non-null user, this fails. Tracked as broader hardening item.
5. **`ChatService` mockability for tests** — concrete struct, no interface. Task 3/8 streaming tests likely require a real LLM provider stub. If unblocked tests are needed, introduce a `chatStreamer` interface in a follow-up.
6. **EventLogs / Telemetry** — every v1 mutation in Node fires events; deferred.
7. **`ListLocalFiles` tree shape** — Node returns nested folder tree. Confirm `FileSystemService.ListLocalFiles` returns a compatible shape before declaring D7/D8 done; if not, add a serializer.
