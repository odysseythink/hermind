# API v1 PR5 — OpenAI Compat Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the 4 OpenAI-compatible API v1 routes in `api_openai.go` so that LangChain Python, `openai>=1.0`, and OpenAI JS SDK clients can use AnythingLLM as a drop-in OpenAI replacement. Includes one small `ChatService` DTO extension (`HistoryOverride`) so OpenAI clients can supply their own history without being polluted by DB-stored chat rows.

**Architecture:** New handler file `api_openai.go` follows the same `api_embed.go` pattern (one handler struct, one method per route, `RegisterAPI*Routes` registers absolute `/v1/...` paths under the existing `api` group). Streaming uses **raw SSE `data:` frames** (NOT `c.SSEvent`) per design §6.3, to keep openai-python and LangChain SDKs happy.

**Tech Stack:** Go 1.22+, Gin, GORM, sqlite (test), testify, httptest, encoding/json.

**Source spec:** `.gpowers/designs/2026-05-25-api-v1-layer-design.md` §4.8 + §6.1–§6.3.

**Reference Node implementation:**
- `server/endpoints/api/openai/index.js` (4 routes)
- `server/endpoints/api/openai/helpers.js` (`extractTextContent`, `extractAttachments`)
- `server/utils/chats/openaiCompatible.js` (`OpenAICompatibleChat.chatSync`, `streamChat`)

**Depends on:** PR1 (workspace count helpers indirectly) and **PR2** (`SystemPromptOverride` + `TemperatureOverride` on ChatRequest are required to feed the OpenAI `system` message through to the LLM). PR3 / PR4 are independent — but `api_workspace.go` from PR4 already builds the workspace-level v1 surface, so PR5 piggybacks on the same handler conventions.

**State note:** PR2 is already committed (`260beb1` + `7738033`). The DTO has `SystemPromptOverride *string` and `TemperatureOverride *float64`. PR5 adds **one more optional DTO field** (`HistoryOverride []core.Message`) so OpenAI clients can pass their own conversation history.

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `WorkspaceService.List(ctx, userID int)` (`workspace_service.go:50`) — returns all workspaces when userID=0 (API context).
- `ChatService.Complete(ctx, ws, user, threadID, req)` (`chat_service.go:193`) — returns `*dto.ChatResponse{ID, Type, TextResponse, Sources, Close, Error}`. Currently honors `req.SystemPromptOverride`. **Will be extended in Task 1** to honor `req.HistoryOverride`.
- `ChatService.Stream(ctx, ws, user, threadID, req)` (`chat_service.go:96`) — returns `<-chan dto.StreamChatResponse`. Same extension.
- `ChatService.buildRAGContext(ctx, ws, user, threadID, message, systemPromptOverride)` (`chat_service.go:35`) — Task 1 changes its history-building branch to honor the override.
- `ChatService.buildChatHistory(ctx, workspaceID, threadID, limit)` — internal DB history pull. Stays as-is.
- `Embedder.EmbedTexts(ctx, texts) ([][]float32, error)` (`embedder/pantheon.go:60`). No `Model()` method on the interface — we'll pull the model name from `cfg.EmbeddingModel` (with `"text-embedding-3-small"` default mirroring `pantheon.go:48`).
- `ThreadService.GetBySlug(ctx, workspaceID, threadSlug)` (`thread_service.go:71`).
- `middleware.ValidAPIKey(apiKeySvc)` — gate.
- `core.Message{Role, Content}` + `core.NewTextContent(s string)` — internal LLM message shape from `pantheon/core`.

### Routes to add (4)

| # | Method | Path | Service | Notes |
|---|---|---|---|---|
| O1 | GET | `/v1/openai/models` | `WorkspaceService.List(ctx, 0)` | Returns `{object:"list", data:[{id:slug, object:"model", created, owned_by}]}` |
| O2 | POST | `/v1/openai/chat/completions` | `ChatService.Complete`/`Stream` | OpenAI request → ChatRequest translation; `model` may be `slug` or `slug:threadSlug` |
| O3 | POST | `/v1/openai/embeddings` | `Embedder.EmbedTexts` | `input` may be `string` or `[]string`; response is `{object:"list", data:[{object:"embedding", embedding, index}], model}` |
| O4 | GET | `/v1/openai/vector_stores` | `WorkspaceService.List` + `CountDocuments` | Returns `{data:[{id, object:"vector_store", name, file_counts:{total}, provider}], first_id, last_id, has_more}` |

### Out of scope (explicit)

- **`extractAttachments` (image_url base64)** — Node maps `image_url` content parts into AnythingLLM `Attachment` shape. PR5 v1 ignores image_url content; only text parts are extracted. File as a "multimodal attachments" follow-up. (Most OpenAI client integrations send text-only at first; image support is a separate hardening PR.)
- **`Telemetry.sendTelemetry("sent_chat", ...)`** — deferred.
- **`EventLogs.logEvent("api_sent_chat", ...)`** — deferred.
- **`TemperatureOverride` honored at LLM call site** — PR2 plumbed the DTO field; PR5 reads it from the OpenAI request but `providers.LLMProvider.Complete/Stream` does not yet accept a per-call temperature. File as a known gap; behavior: temperature is silently dropped on the LLM call (workspace default applies).
- **Pagination on `/v1/openai/vector_stores`** — Node short-circuits to `{data:[], has_more:false}` when any query param is present. Mirror.
- **Embedding rerank / batching** — `Embedder.EmbedTexts` already batches via `model.Embed`. No additional rerank.

### OpenAI SSE format (critical — do not use Gin's `c.SSEvent`)

OpenAI clients expect **unnamed event lines**:

```
data: {"id":"...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

```

Implementation must use raw writes + `http.Flusher.Flush()`:

```go
c.Header("Content-Type", "text/event-stream")
c.Header("Cache-Control", "no-cache")
c.Header("Connection", "keep-alive")
flusher, _ := c.Writer.(http.Flusher)

payload, _ := json.Marshal(chunk)
fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
flusher.Flush()
// ...
fmt.Fprint(c.Writer, "data: [DONE]\n\n")
flusher.Flush()
```

`c.SSEvent("message", payload)` writes `event: message\ndata: ...` which **breaks `openai-python` and LangChain's `ChatOpenAI`** (they ignore named events). Confirmed in §6.3 of the design doc.

### Response-shape conventions (Node parity)

- All success responses: HTTP **200**. Non-streaming chat returns full OpenAI `chat.completion` shape.
- Streaming uses the SSE format above. Final frame is `data: [DONE]\n\n`.
- `/v1/openai/models` returns 200 + `{object:"list", data:[...]}` even when no workspaces exist.
- `/v1/openai/embeddings` with empty input → 500 (per Node: `throw new Error("Input array cannot be empty.")` → caught → `response.status(500).end()`).

### Test setup helper

Reuse `newAPITestEnv(t, cfg)` from PR3 (`api_setup_test.go`). For PR5 tests that need a working `ChatService`, build it with a mock or nil-tolerant `LLMProvider` — see Task 5 for the stub LLM pattern (one used in existing `chat_service_test.go` if present, otherwise introduce a minimal `mockLLM`).

### TDD discipline

Each task: write failing test → run + confirm fail → minimal impl → run + confirm pass → commit.

---

## Task 1: Service extension — `HistoryOverride` on ChatRequest + StreamChatRequest

OpenAI clients send the full conversation history in the request. We must not mix it with DB history. Add `HistoryOverride []core.Message` to the two request DTOs and patch `buildRAGContext` to use it when set.

**Files:**
- Modify: `backend/internal/dto/chat.go`
- Modify: `backend/internal/services/chat_service.go`
- Modify: `backend/internal/services/chat_service_test.go`

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/services/chat_service_test.go`:

```go
func TestBuildRAGContext_HistoryOverride_BypassesDBLookup(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    vec := NewVectorService(cfg)
    svc := NewChatService(db, cfg, vec, nil, nil)

    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, db.Create(ws).Error)
    // Insert DB chat that would otherwise show up in history.
    require.NoError(t, db.Create(&models.WorkspaceChat{
        WorkspaceID: ws.ID,
        Prompt:      "DB question",
        Response:    `{"text":"DB answer"}`,
        Include:     utils.Ptr(true),
    }).Error)

    overrideHistory := []core.Message{
        {Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("override q")},
        {Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("override a")},
    }
    _, _, history, err := svc.buildRAGContext(
        context.Background(), ws, nil, nil, "hi", nil, overrideHistory,
    )
    require.NoError(t, err)
    require.Len(t, history, 2)
    // History came from override, not DB.
}

func TestBuildRAGContext_NilHistoryOverride_PullsFromDB(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil)

    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, db.Create(ws).Error)
    require.NoError(t, db.Create(&models.WorkspaceChat{
        WorkspaceID: ws.ID,
        Prompt:      "stored",
        Response:    `{"text":"stored-resp"}`,
        Include:     utils.Ptr(true),
    }).Error)

    _, _, history, err := svc.buildRAGContext(
        context.Background(), ws, nil, nil, "hi", nil, nil,
    )
    require.NoError(t, err)
    // 1 chat row → 2 messages (user prompt + assistant response).
    assert.Len(t, history, 2)
}
```

(Add `"github.com/odysseythink/pantheon/core"` to imports.)

- [ ] **Step 2: Run + confirm fail**

```bash
cd backend && go test ./internal/services/ -run TestBuildRAGContext_HistoryOverride -count=1
```

Expected failure: `buildRAGContext` signature has 6 args, test calls with 7.

- [ ] **Step 3: Implement**

In `internal/dto/chat.go`, extend both request types:

```go
type ChatRequest struct {
    Message              string         `json:"message"`
    Mode                 string         `json:"mode,omitempty"`
    SessionID            string         `json:"sessionId,omitempty"`
    Reset                bool           `json:"reset,omitempty"`
    Attachments          []string       `json:"attachments,omitempty"`
    SystemPromptOverride *string        `json:"systemPromptOverride,omitempty"`
    TemperatureOverride  *float64       `json:"temperatureOverride,omitempty"`
    HistoryOverride      []core.Message `json:"-"` // not over-the-wire; only set by OpenAI handler
}

type StreamChatRequest struct {
    Message              string         `json:"message"`
    Attachments          []string       `json:"attachments,omitempty"`
    SystemPromptOverride *string        `json:"systemPromptOverride,omitempty"`
    TemperatureOverride  *float64       `json:"temperatureOverride,omitempty"`
    HistoryOverride      []core.Message `json:"-"`
}
```

Add `"github.com/odysseythink/pantheon/core"` to dto imports.

In `chat_service.go`, extend `buildRAGContext` signature:

```go
func (s *ChatService) buildRAGContext(
    ctx context.Context,
    ws *models.Workspace,
    user *models.User,
    threadID *int,
    message string,
    systemPromptOverride *string,
    historyOverride []core.Message, // NEW
) (systemPrompt string, sources []any, history []core.Message, err error) {
    if historyOverride != nil {
        history = historyOverride
    } else {
        historyLimit := ws.OpenAiHistory
        if historyLimit <= 0 {
            historyLimit = 20
        }
        history, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
        if err != nil {
            return "", nil, nil, err
        }
    }
    // ... rest of function unchanged (system prompt + RAG)
}
```

Update both callsites in `Stream` and `Complete`:

```go
// Stream (~line 105):
systemPrompt, sources, history, err := s.buildRAGContext(
    ctx, ws, user, threadID, req.Message,
    req.SystemPromptOverride, req.HistoryOverride,
)
// Complete (~line 200): same change.
```

- [ ] **Step 4: Run + confirm pass**

```bash
cd backend && go test ./internal/services/ -run TestBuildRAGContext -count=1
```

All 4 existing override tests + the 2 new ones must pass.

- [ ] **Step 5: Wider build/test sweep**

```bash
cd backend && go build ./... && go test ./... -count=1
```

(`HistoryOverride` is `json:"-"` so it doesn't affect any over-the-wire payload; no PR3/PR4 tests should regress.)

- [ ] **Step 6: Commit**

```bash
git add backend/internal/dto/chat.go backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go
git commit -m "feat(api-v1): ChatRequest.HistoryOverride for OpenAI-compat history pass-through"
```

---

## Task 2: `api_openai.go` — GET /v1/openai/models (O1)

**Files:**
- Create: `backend/internal/handlers/api_openai.go`
- Create: `backend/internal/handlers/api_openai_test.go`

- [ ] **Step 1: Failing test**

```go
// File: backend/internal/handlers/api_openai_test.go
package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAPIOpenAI_Models(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "Chat 1", Slug: "chat-1"}).Error)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "Chat 2", Slug: "chat-2"}).Error)

    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/openai/models", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Object string `json:"object"`
        Data   []struct {
            ID      string `json:"id"`
            Object  string `json:"object"`
            OwnedBy string `json:"owned_by"`
        } `json:"data"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Equal(t, "list", body.Object)
    require.Len(t, body.Data, 2)
    slugs := []string{body.Data[0].ID, body.Data[1].ID}
    assert.Contains(t, slugs, "chat-1")
    assert.Contains(t, slugs, "chat-2")
    for _, m := range body.Data {
        assert.Equal(t, "model", m.Object)
        assert.NotEmpty(t, m.OwnedBy)
    }
}
```

- [ ] **Step 2: Run + confirm fail**

```bash
cd backend && go test ./internal/handlers/ -run TestAPIOpenAI_Models -count=1
```

- [ ] **Step 3: Implement**

```go
// File: backend/internal/handlers/api_openai.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/embedder"
    "github.com/odysseythink/hermind/backend/internal/middleware"
    "github.com/odysseythink/hermind/backend/internal/services"
    "gorm.io/gorm"
)

type APIOpenAIHandler struct {
    wsSvc     *services.WorkspaceService
    chatSvc   *services.ChatService
    threadSvc *services.ThreadService
    emb       embedder.Embedder
    db        *gorm.DB
    cfg       *config.Config
}

func NewAPIOpenAIHandler(
    wsSvc *services.WorkspaceService,
    chatSvc *services.ChatService,
    threadSvc *services.ThreadService,
    emb embedder.Embedder,
    db *gorm.DB,
    cfg *config.Config,
) *APIOpenAIHandler {
    return &APIOpenAIHandler{
        wsSvc: wsSvc, chatSvc: chatSvc, threadSvc: threadSvc,
        emb: emb, db: db, cfg: cfg,
    }
}

func (h *APIOpenAIHandler) Models(c *gin.Context) {
    workspaces, err := h.wsSvc.List(c.Request.Context(), 0)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    data := make([]gin.H, 0, len(workspaces))
    for _, ws := range workspaces {
        provider := h.cfg.LLMProvider
        if ws.ChatProvider != nil && *ws.ChatProvider != "" {
            provider = *ws.ChatProvider
        }
        model := h.cfg.LLMModel
        if ws.ChatModel != nil && *ws.ChatModel != "" {
            model = *ws.ChatModel
        }
        data = append(data, gin.H{
            "id":       ws.Slug,
            "object":   "model",
            "created":  ws.CreatedAt.Unix(),
            "owned_by": provider + "-" + model,
        })
    }
    c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func RegisterAPIOpenAIRoutes(
    r *gin.RouterGroup,
    apiKeySvc *services.APIKeyService,
    wsSvc *services.WorkspaceService,
    chatSvc *services.ChatService,
    threadSvc *services.ThreadService,
    emb embedder.Embedder,
    db *gorm.DB,
    cfg *config.Config,
) {
    h := NewAPIOpenAIHandler(wsSvc, chatSvc, threadSvc, emb, db, cfg)
    r.GET("/v1/openai/models", middleware.ValidAPIKey(apiKeySvc), h.Models)
    // Tasks 3-6 will append the rest.
}
```

> **Audit point**: `models.Workspace` field names — verify `ChatProvider`, `ChatModel` are `*string`. If they're `string`, drop the nil-deref. Same for `cfg.LLMProvider`/`cfg.LLMModel`. If config field is different (e.g. `LlmProvider`), adjust.

- [ ] **Step 4: Run + commit**

```bash
git add backend/internal/handlers/api_openai.go backend/internal/handlers/api_openai_test.go
git commit -m "feat(api-v1): GET /v1/openai/models"
```

---

## Task 3: `api_openai.go` — GET /v1/openai/vector_stores (O4)

Trivial-shape but needs per-workspace document count.

- [ ] **Step 1: Failing test**

```go
func TestAPIOpenAI_VectorStores_Empty(t *testing.T) {
    env := newAPITestEnv(t, nil)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Data    []any  `json:"data"`
        HasMore bool   `json:"has_more"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Empty(t, body.Data)
    assert.False(t, body.HasMore)
}

func TestAPIOpenAI_VectorStores_WithDocs(t *testing.T) {
    env := newAPITestEnv(t, nil)
    ws := &models.Workspace{Name: "My WS", Slug: "my-ws"}
    require.NoError(t, env.DB.Create(ws).Error)
    require.NoError(t, env.DB.Create(&models.WorkspaceDocument{
        DocId: "d1", Filename: "f1", Docpath: "x/1", WorkspaceID: ws.ID,
    }).Error)
    require.NoError(t, env.DB.Create(&models.WorkspaceDocument{
        DocId: "d2", Filename: "f2", Docpath: "x/2", WorkspaceID: ws.ID,
    }).Error)

    cfg := env.Cfg
    cfg.VectorDB = "lancedb"
    wsSvc := services.NewWorkspaceService(env.DB, cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, cfg)

    req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Data []struct {
            ID         string `json:"id"`
            Object     string `json:"object"`
            Name       string `json:"name"`
            FileCounts struct {
                Total int `json:"total"`
            } `json:"file_counts"`
            Provider string `json:"provider"`
        } `json:"data"`
        HasMore bool `json:"has_more"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    require.Len(t, body.Data, 1)
    assert.Equal(t, "my-ws", body.Data[0].ID)
    assert.Equal(t, "vector_store", body.Data[0].Object)
    assert.Equal(t, "My WS", body.Data[0].Name)
    assert.Equal(t, 2, body.Data[0].FileCounts.Total)
    assert.Equal(t, "lancedb", body.Data[0].Provider)
}

func TestAPIOpenAI_VectorStores_PaginationQueryShortCircuits(t *testing.T) {
    env := newAPITestEnv(t, nil)
    require.NoError(t, env.DB.Create(&models.Workspace{Name: "x", Slug: "x"}).Error)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

    req := httptest.NewRequest("GET", "/api/v1/openai/vector_stores?after=x", nil)
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Data    []any `json:"data"`
        HasMore bool  `json:"has_more"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Empty(t, body.Data)
    assert.False(t, body.HasMore)
}
```

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement**

Append to `api_openai.go`:

```go
import (
    "github.com/odysseythink/hermind/backend/internal/models"
)

func (h *APIOpenAIHandler) VectorStores(c *gin.Context) {
    // Node short-circuits when any query param is present.
    if len(c.Request.URL.Query()) > 0 {
        c.JSON(http.StatusOK, gin.H{"data": []any{}, "has_more": false})
        return
    }
    workspaces, err := h.wsSvc.List(c.Request.Context(), 0)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    provider := h.cfg.VectorDB
    if provider == "" {
        provider = "lancedb"
    }
    data := make([]gin.H, 0, len(workspaces))
    for _, ws := range workspaces {
        var total int64
        h.db.Model(&models.WorkspaceDocument{}).
            Where("workspace_id = ?", ws.ID).Count(&total)
        data = append(data, gin.H{
            "id":          ws.Slug,
            "object":      "vector_store",
            "name":        ws.Name,
            "file_counts": gin.H{"total": total},
            "provider":    provider,
        })
    }
    var firstID, lastID string
    if len(data) > 0 {
        firstID = workspaces[0].Slug
        lastID = workspaces[len(workspaces)-1].Slug
    }
    c.JSON(http.StatusOK, gin.H{
        "first_id": firstID,
        "last_id":  lastID,
        "data":     data,
        "has_more": false,
    })
}
```

Append registration:

```go
r.GET("/v1/openai/vector_stores", middleware.ValidAPIKey(apiKeySvc), h.VectorStores)
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): GET /v1/openai/vector_stores"
```

---

## Task 4: `api_openai.go` — POST /v1/openai/embeddings (O3)

Accept `input` as string OR array of strings; call `EmbedTexts`; format response.

- [ ] **Step 1: Failing tests**

```go
// To test without a real OpenAI key, use a mock embedder.
type mockEmbedder struct {
    vec [][]float32
}

func (m *mockEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
    out := make([][]float32, len(texts))
    for i := range texts {
        out[i] = []float32{1.0, float32(i)}
    }
    return out, nil
}
func (m *mockEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) { return nil, nil }
func (m *mockEmbedder) Dimensions() int                                            { return 2 }

func TestAPIOpenAI_Embeddings_ArrayInput(t *testing.T) {
    env := newAPITestEnv(t, nil)
    env.Cfg.EmbeddingModel = "text-embedding-3-small"
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, &mockEmbedder{}, env.DB, env.Cfg)

    payload := []byte(`{"input":["hello","world"],"model":null}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        Object string `json:"object"`
        Data   []struct {
            Object    string    `json:"object"`
            Embedding []float32 `json:"embedding"`
            Index     int       `json:"index"`
        } `json:"data"`
        Model string `json:"model"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Equal(t, "list", body.Object)
    require.Len(t, body.Data, 2)
    assert.Equal(t, "embedding", body.Data[0].Object)
    assert.Equal(t, 0, body.Data[0].Index)
    assert.Equal(t, 1, body.Data[1].Index)
    assert.Equal(t, "text-embedding-3-small", body.Model)
}

func TestAPIOpenAI_Embeddings_StringInputCoerced(t *testing.T) {
    env := newAPITestEnv(t, nil)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, &mockEmbedder{}, env.DB, env.Cfg)

    payload := []byte(`{"input":"hello"}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct{ Data []any `json:"data"` }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Len(t, body.Data, 1)
}

func TestAPIOpenAI_Embeddings_EmptyInput500(t *testing.T) {
    env := newAPITestEnv(t, nil)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, &mockEmbedder{}, env.DB, env.Cfg)

    payload := []byte(`{"input":[]}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAPIOpenAI_Embeddings_NilEmbedder503(t *testing.T) {
    env := newAPITestEnv(t, nil)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB, env.Cfg)

    payload := []byte(`{"input":"hi"}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/embeddings", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
```

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement**

Append to `api_openai.go`:

```go
func (h *APIOpenAIHandler) Embeddings(c *gin.Context) {
    if h.emb == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embedder not configured"})
        return
    }
    var raw map[string]any
    if err := c.ShouldBindJSON(&raw); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // Accept either "input" or legacy "inputs".
    rawInput := raw["input"]
    if rawInput == nil {
        rawInput = raw["inputs"]
    }
    var texts []string
    switch v := rawInput.(type) {
    case string:
        texts = []string{v}
    case []any:
        for _, item := range v {
            if s, ok := item.(string); ok {
                texts = append(texts, s)
                continue
            }
            c.JSON(http.StatusInternalServerError, gin.H{"error": "All inputs to be embedded must be strings."})
            return
        }
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Input must be a string or array of strings."})
        return
    }
    if len(texts) == 0 {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Input array cannot be empty."})
        return
    }
    vecs, err := h.emb.EmbedTexts(c.Request.Context(), texts)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    data := make([]gin.H, len(vecs))
    for i, v := range vecs {
        data[i] = gin.H{"object": "embedding", "embedding": v, "index": i}
    }
    model := h.cfg.EmbeddingModel
    if model == "" {
        model = "text-embedding-3-small"
    }
    c.JSON(http.StatusOK, gin.H{"object": "list", "data": data, "model": model})
}
```

Append registration:

```go
r.POST("/v1/openai/embeddings", middleware.ValidAPIKey(apiKeySvc), h.Embeddings)
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): POST /v1/openai/embeddings"
```

---

## Task 5: `api_openai.go` — POST /v1/openai/chat/completions non-streaming (O2 part 1)

Parse OpenAI request → resolve workspace (and optionally thread via `slug:threadSlug`) → translate messages → call `ChatService.Complete` → format OpenAI response.

- [ ] **Step 1: Failing tests**

```go
// Mock ChatService is impractical (concrete struct). For unit tests, use a
// real ChatService built with a no-op LLMProvider:
type mockLLM struct{ text string }

func (m *mockLLM) Complete(_ context.Context, _ []core.Message, _ string) (string, error) {
    return m.text, nil
}
func (m *mockLLM) Stream(_ context.Context, _ []core.Message, _ string) (<-chan core.StreamChunk, error) {
    ch := make(chan core.StreamChunk, 2)
    ch <- core.StreamChunk{TextDelta: m.text}
    ch <- core.StreamChunk{FinishReason: "stop"}
    close(ch)
    return ch, nil
}

func newChatSvcWithMock(t *testing.T, env *apiTestEnv, llmText string) *services.ChatService {
    t.Helper()
    cfg := env.Cfg
    vec := services.NewVectorService(cfg)
    return services.NewChatService(env.DB, cfg, vec, &mockLLM{text: llmText}, nil)
}

func TestAPIOpenAI_ChatCompletions_NonStreaming(t *testing.T) {
    env := newAPITestEnv(t, nil)
    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, env.DB.Create(ws).Error)

    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    chatSvc := newChatSvcWithMock(t, env, "Hello from LLM")
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

    payload := []byte(`{
        "model":"ws",
        "messages":[
            {"role":"system","content":"Be helpful."},
            {"role":"user","content":"Hi there"}
        ],
        "stream":false
    }`)
    req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    var body struct {
        ID      string `json:"id"`
        Object  string `json:"object"`
        Model   string `json:"model"`
        Choices []struct {
            Index   int `json:"index"`
            Message struct {
                Role    string `json:"role"`
                Content string `json:"content"`
            } `json:"message"`
            FinishReason string `json:"finish_reason"`
        } `json:"choices"`
    }
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    assert.Equal(t, "chat.completion", body.Object)
    assert.Equal(t, "ws", body.Model)
    require.Len(t, body.Choices, 1)
    assert.Equal(t, "assistant", body.Choices[0].Message.Role)
    assert.Equal(t, "Hello from LLM", body.Choices[0].Message.Content)
    assert.Equal(t, "stop", body.Choices[0].FinishReason)
}

func TestAPIOpenAI_ChatCompletions_UnknownModelReturns401(t *testing.T) {
    env := newAPITestEnv(t, nil)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    chatSvc := newChatSvcWithMock(t, env, "n/a")
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

    payload := []byte(`{"model":"ghost","messages":[{"role":"user","content":"hi"}]}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusUnauthorized, rec.Code) // Node parity: returns 401 on missing workspace
}

func TestAPIOpenAI_ChatCompletions_ThreadScopedModel(t *testing.T) {
    env := newAPITestEnv(t, nil)
    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, env.DB.Create(ws).Error)
    thread := &models.WorkspaceThread{Name: "t", Slug: "t1", WorkspaceID: ws.ID}
    require.NoError(t, env.DB.Create(thread).Error)

    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    chatSvc := newChatSvcWithMock(t, env, "thread-scoped")
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

    payload := []byte(`{"model":"ws:t1","messages":[{"role":"user","content":"hi"}]}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code) // succeeds with thread context
}

func TestAPIOpenAI_ChatCompletions_NoUserMessageReturns400(t *testing.T) {
    env := newAPITestEnv(t, nil)
    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, env.DB.Create(ws).Error)
    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    chatSvc := newChatSvcWithMock(t, env, "")
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

    // Last message is "assistant", not "user".
    payload := []byte(`{"model":"ws","messages":[{"role":"user","content":"q"},{"role":"assistant","content":"a"}]}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusBadRequest, rec.Code)
}
```

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement**

Append to `api_openai.go`:

```go
import (
    "github.com/google/uuid"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/pantheon/core"
    "strings"
    "time"
)

// extractTextContent flattens an OpenAI message Content (string OR []{type,text})
// into a plain string. Image-url parts are ignored (out of scope, see Pre-task).
func extractTextContent(content any) string {
    if s, ok := content.(string); ok {
        return s
    }
    parts, ok := content.([]any)
    if !ok {
        return ""
    }
    var sb strings.Builder
    first := true
    for _, p := range parts {
        m, ok := p.(map[string]any)
        if !ok {
            continue
        }
        if t, _ := m["type"].(string); t == "text" {
            if !first {
                sb.WriteString("\n")
            }
            txt, _ := m["text"].(string)
            sb.WriteString(txt)
            first = false
        }
    }
    return sb.String()
}

// openaiHistoryToCoreMessages converts the conversation prefix (without the
// trailing user message and any role:"system" entries) into core.Message slice.
// Role "user" → MESSAGE_ROLE_USER, "assistant" → MESSAGE_ROLE_ASSISTANT.
// Unknown roles are skipped.
func openaiHistoryToCoreMessages(msgs []dto.OpenAIMessage) []core.Message {
    out := make([]core.Message, 0, len(msgs))
    for _, m := range msgs {
        if m.Role == "system" {
            continue
        }
        var role string
        switch m.Role {
        case "user":
            role = core.MESSAGE_ROLE_USER
        case "assistant":
            role = core.MESSAGE_ROLE_ASSISTANT
        default:
            continue
        }
        out = append(out, core.Message{
            Role:    role,
            Content: core.NewTextContent(extractTextContent(m.Content)),
        })
    }
    return out
}

func (h *APIOpenAIHandler) ChatCompletions(c *gin.Context) {
    var req dto.OpenAIChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if len(req.Messages) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "messages cannot be empty"})
        return
    }
    // Last message must be role:user (Node parity).
    last := req.Messages[len(req.Messages)-1]
    if last.Role != "user" {
        c.JSON(http.StatusBadRequest, gin.H{
            "id":           uuid.New().String(),
            "type":         "abort",
            "textResponse": nil,
            "sources":      []any{},
            "close":        true,
            "error":        "No user prompt found. Must be last element in message array with 'user' role.",
        })
        return
    }
    prompt := extractTextContent(last.Content)

    // Resolve workspace (and optional thread via slug:threadSlug).
    wsSlug, threadSlug := req.Model, ""
    if idx := strings.Index(req.Model, ":"); idx >= 0 {
        wsSlug, threadSlug = req.Model[:idx], req.Model[idx+1:]
    }
    ws, err := h.wsSvc.GetBySlug(c.Request.Context(), wsSlug)
    if err != nil {
        c.AbortWithStatus(http.StatusUnauthorized) // Node parity
        return
    }
    var threadID *int
    if threadSlug != "" {
        thread, err := h.threadSvc.GetBySlug(c.Request.Context(), ws.ID, threadSlug)
        if err != nil {
            c.AbortWithStatus(http.StatusUnauthorized)
            return
        }
        threadID = &thread.ID
    }

    // Extract system message + history override.
    var systemOverride *string
    var historyMsgs []dto.OpenAIMessage
    for _, m := range req.Messages[:len(req.Messages)-1] {
        if m.Role == "system" {
            s := extractTextContent(m.Content)
            if s != "" {
                systemOverride = &s
            }
            continue
        }
        historyMsgs = append(historyMsgs, m)
    }
    history := openaiHistoryToCoreMessages(historyMsgs)

    chatReq := dto.ChatRequest{
        Message:              prompt,
        SystemPromptOverride: systemOverride,
        TemperatureOverride:  req.Temperature,
        HistoryOverride:      history,
    }

    if !req.Stream {
        resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, threadID, chatReq)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{
            "id":      "chatcmpl-" + uuid.New().String(),
            "object":  "chat.completion",
            "created": time.Now().Unix(),
            "model":   req.Model,
            "choices": []gin.H{{
                "index":         0,
                "message":       gin.H{"role": "assistant", "content": resp.TextResponse},
                "finish_reason": "stop",
            }},
            "usage": gin.H{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        })
        return
    }
    // Streaming path handled in Task 6.
    h.chatCompletionsStream(c, ws, threadID, chatReq, req.Model)
}
```

Append registration:

```go
r.POST("/v1/openai/chat/completions", middleware.ValidAPIKey(apiKeySvc), h.ChatCompletions)
```

Add a stub `chatCompletionsStream` method (Task 6 fills it in):

```go
func (h *APIOpenAIHandler) chatCompletionsStream(c *gin.Context, ws *models.Workspace, threadID *int, chatReq dto.ChatRequest, modelStr string) {
    // Implemented in Task 6
    c.AbortWithStatus(http.StatusNotImplemented)
}
```

- [ ] **Step 4: Run + commit**

```bash
git commit -m "feat(api-v1): POST /v1/openai/chat/completions non-streaming"
```

> **Audit point**: `core.MESSAGE_ROLE_USER` / `MESSAGE_ROLE_ASSISTANT` constants — verify via grep. Adjust if names differ.

---

## Task 6: `api_openai.go` — streaming chat completions (O2 part 2)

Convert internal `dto.StreamChatResponse` chunks → OpenAI SSE chunks. Use raw `data: ...\n\n` lines.

- [ ] **Step 1: Failing tests**

```go
func TestAPIOpenAI_ChatCompletions_Stream_EmitsDataFrames(t *testing.T) {
    env := newAPITestEnv(t, nil)
    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, env.DB.Create(ws).Error)

    wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
    chatSvc := newChatSvcWithMock(t, env, "Hello world")
    api := env.Router.Group("/api")
    RegisterAPIOpenAIRoutes(api, env.APIKeySvc, wsSvc, chatSvc, services.NewThreadService(env.DB), nil, env.DB, env.Cfg)

    payload := []byte(`{"model":"ws","messages":[{"role":"user","content":"hi"}],"stream":true}`)
    req := httptest.NewRequest("POST", "/api/v1/openai/chat/completions", bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+env.APIKey)
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    env.Router.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code)
    body := rec.Body.String()
    assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
    assert.Contains(t, body, "data: ")
    assert.Contains(t, body, `"object":"chat.completion.chunk"`)
    assert.Contains(t, body, `"delta":{"content":"Hello world"}`)
    assert.Contains(t, body, `"finish_reason":"stop"`)
    assert.Contains(t, body, "data: [DONE]")
    // No "event: " prefix anywhere — OpenAI clients reject named events.
    assert.NotContains(t, body, "event: ")
}
```

- [ ] **Step 2: Run + confirm fail**

- [ ] **Step 3: Implement**

Replace the stub `chatCompletionsStream` in `api_openai.go`:

```go
import (
    "encoding/json"
    "fmt"
)

func (h *APIOpenAIHandler) chatCompletionsStream(c *gin.Context, ws *models.Workspace, threadID *int, chatReq dto.ChatRequest, modelStr string) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Access-Control-Allow-Origin", "*")

    flusher, ok := c.Writer.(http.Flusher)
    if !ok {
        c.AbortWithStatus(http.StatusInternalServerError)
        return
    }

    streamReq := dto.StreamChatRequest{
        Message:              chatReq.Message,
        SystemPromptOverride: chatReq.SystemPromptOverride,
        TemperatureOverride:  chatReq.TemperatureOverride,
        HistoryOverride:      chatReq.HistoryOverride,
    }
    chunks, err := h.chatSvc.Stream(c.Request.Context(), ws, nil, threadID, streamReq)
    if err != nil {
        writeOpenAIErrorFrame(c.Writer, flusher, err.Error())
        return
    }

    chunkID := "chatcmpl-" + uuid.New().String()
    created := time.Now().Unix()

    for chunk := range chunks {
        switch chunk.Type {
        case "textResponseChunk":
            delta := ""
            if chunk.TextResponse != nil {
                delta = *chunk.TextResponse
            }
            writeOpenAIDeltaFrame(c.Writer, flusher, chunkID, created, modelStr,
                gin.H{"content": delta}, nil)
        case "finalizeResponseStream":
            writeOpenAIDeltaFrame(c.Writer, flusher, chunkID, created, modelStr,
                gin.H{}, openaiStringPtr("stop"))
        case "abort":
            errStr := "stream aborted"
            if chunk.Error != nil {
                errStr = *chunk.Error
            }
            writeOpenAIErrorFrame(c.Writer, flusher, errStr)
            return
        }
        if chunk.Close {
            break
        }
    }

    fmt.Fprint(c.Writer, "data: [DONE]\n\n")
    flusher.Flush()
}

func writeOpenAIDeltaFrame(w gin.ResponseWriter, flusher http.Flusher, id string, created int64, model string, delta gin.H, finishReason *string) {
    payload := gin.H{
        "id":      id,
        "object":  "chat.completion.chunk",
        "created": created,
        "model":   model,
        "choices": []gin.H{{
            "index":         0,
            "delta":         delta,
            "finish_reason": finishReason,
        }},
    }
    raw, _ := json.Marshal(payload)
    fmt.Fprintf(w, "data: %s\n\n", raw)
    flusher.Flush()
}

func writeOpenAIErrorFrame(w gin.ResponseWriter, flusher http.Flusher, msg string) {
    payload := gin.H{
        "error": gin.H{"message": msg, "type": "api_error"},
    }
    raw, _ := json.Marshal(payload)
    fmt.Fprintf(w, "data: %s\n\n", raw)
    flusher.Flush()
    fmt.Fprint(w, "data: [DONE]\n\n")
    flusher.Flush()
}

func openaiStringPtr(s string) *string { return &s }
```

- [ ] **Step 4: Run + confirm pass**

```bash
cd backend && go test ./internal/handlers/ -run TestAPIOpenAI_ChatCompletions_Stream -count=1
```

- [ ] **Step 5: Manual smoke-test with a real OpenAI client**

```bash
cd backend && go run ./cmd/server &
# In another shell, with Python openai>=1.0:
# (set BASE_URL=http://localhost:8080/api/v1/openai and API key in code)
python -c "
from openai import OpenAI
c = OpenAI(api_key='YOUR_KEY', base_url='http://localhost:8080/api/v1/openai')
for chunk in c.chat.completions.create(
    model='your-workspace-slug',
    messages=[{'role':'user','content':'hi'}],
    stream=True,
):
    print(chunk.choices[0].delta.content or '', end='')
print()
"
```

If the LangChain `ChatOpenAI` integration also works (no `event: message` parse errors), the SSE format is correct.

- [ ] **Step 6: Commit**

```bash
git commit -m "feat(api-v1): POST /v1/openai/chat/completions streaming SSE"
```

> **Audit point**: `chunk.Type` string constants — verify the exact values produced by `chat_service.go`: `"textResponseChunk"`, `"finalizeResponseStream"`, `"abort"`. Already confirmed via grep at `chat_service.go:159, 169, 110`.

---

## Task 7: main.go wiring + full-suite verify

- [ ] **Step 1: Edit main.go**

After the existing API v1 registration block (post-PR3 + PR4), append:

```go
handlers.RegisterAPIOpenAIRoutes(api, apiKeySvc, wsSvc, chatSvc, threadSvc, emb, db, cfg)
```

- [ ] **Step 2: Full-suite verify**

```bash
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test ./... -count=1
```

- [ ] **Step 3: Smoke routes**

```bash
cd backend && go run ./cmd/server &
sleep 2
curl -H "Authorization: Bearer <KEY>" http://localhost:8080/api/v1/openai/models
curl -H "Authorization: Bearer <KEY>" http://localhost:8080/api/v1/openai/vector_stores
# kill server
```

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(api-v1): wire OpenAI compat layer in main.go"
```

---

## Acceptance criteria

- [ ] All 4 OpenAI compat routes mounted under `/api/v1/openai/...` with `middleware.ValidAPIKey` gate.
- [ ] `/v1/openai/models` returns `{object:"list", data:[{id:slug, object:"model", created, owned_by}]}`.
- [ ] `/v1/openai/vector_stores` returns paginated-style shape with `file_counts.total` per workspace; query param triggers empty short-circuit.
- [ ] `/v1/openai/embeddings` accepts string OR array input; returns OpenAI shape; empty input → 500; nil embedder → 503.
- [ ] `/v1/openai/chat/completions`:
  - Non-streaming returns full `chat.completion` JSON with one choice + `finish_reason:"stop"`.
  - Streaming emits raw `data: {...}\n\n` SSE frames + `data: [DONE]\n\n` terminator.
  - **No `event: message` prefix** (would break openai-python).
  - Supports `model:"slug"` and `model:"slug:threadSlug"`.
  - `role:"system"` message → `req.SystemPromptOverride`.
  - Other non-system messages (excluding last user) → `req.HistoryOverride` (bypasses DB history).
  - Missing user message → 400 with Node-parity body.
  - Unknown workspace → 401.
- [ ] At least one LangChain Python or `openai>=1.0` smoke test runs end-to-end against a workspace.
- [ ] `go test ./... -count=1` green; `go vet ./...` clean; `go build ./...` succeeds.

---

## Known gaps after PR5 (track but DO NOT implement here)

1. **`image_url` content parts (multimodal)** — `extractAttachments` is skipped. Vision-capable workspaces won't receive images via OpenAI compat layer. File as "multimodal attachments" follow-up.
2. **`TemperatureOverride` not honored at LLM call** — `providers.LLMProvider.Complete/Stream` lacks a per-call temperature parameter. Field is set in DTO and ignored downstream. Extend LLMProvider in a separate hardening PR.
3. **Token usage counts** — `usage` field returns zeros. No tokenizer hook on the Go side yet. File as a separate PR if cost-tracking integrations need real counts.
4. **EventLogs + Telemetry** — `api_sent_chat`, `sent_chat` not emitted. Deferred to platform-wide wire-up.
5. **`model:slug:thread:nested` (3+ segments)** — handler currently splits on first `:` only; any colon in thread slug would parse incorrectly. Slugs are uuid-like in practice so this is mostly theoretical, but document.
6. **SSE keepalive pings** — for long-running streams behind reverse proxies with idle timeouts, emit periodic `: keepalive\n\n` comments. Not required for openai-python (which uses HTTP/1.1 chunked), but proxies like nginx with default 60s idle may close streams. File as a hardening PR.
7. **Pagination on `/v1/openai/vector_stores`** beyond short-circuit — Node's response has `first_id`, `last_id` but no actual pagination logic. Currently returns ALL workspaces in one shot. Acceptable for parity.
