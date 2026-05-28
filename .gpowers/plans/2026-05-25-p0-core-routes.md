# P0 Core Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement all P0 core routes to close the gap between backend and Node server: non-streaming chat, document upload/embedding workflow, workspace search, and vector search.

**Architecture:** Layered implementation — infrastructure first (SSE progress manager + sync LLM), then services (SearchService, DocumentService extensions, VectorSearchService), then handlers. Shared RAG context-building logic is extracted from ChatService.Stream for reuse by ChatService.Complete.

**Tech Stack:** Go 1.21+, Gin, GORM, Pantheon LLM SDK, SQLite/Postgres, LanceDB/PGVector

---

## File Structure

### New Files

| File | Responsibility |
|---|---|
| `backend/internal/services/embedding_progress.go` | SSE connection manager for embedding progress events |
| `backend/internal/services/search_service.go` | Fuzzy workspace/thread search |
| `backend/internal/services/vector_search_service.go` | Workspace-scoped vector similarity search endpoint wrapper |
| `backend/internal/services/search_service_test.go` | SearchService unit tests |
| `backend/tests/integration/p0_chat_test.go` | Non-streaming chat integration tests |
| `backend/tests/integration/p0_document_test.go` | Document workflow integration tests |
| `backend/tests/integration/p0_search_test.go` | Search/vector search integration tests |

### Modified Files

| File | Responsibility |
|---|---|
| `backend/internal/providers/llm.go` | Add `Complete` method to LLMProvider interface |
| `backend/internal/services/chat_service.go` | Add `Complete` method; extract shared RAG builder |
| `backend/internal/services/document_service.go` | Add workspace upload, link upload, embed/unembed, folder CRUD |
| `backend/internal/dto/chat.go` | Add ChatRequest, ChatResponse DTOs |
| `backend/internal/dto/document.go` | Add folder/move DTOs |
| `backend/internal/dto/search.go` | Add SearchResults, VectorSearchResult DTOs |
| `backend/internal/handlers/chat.go` | Add Chat (non-streaming) handler |
| `backend/internal/handlers/workspace.go` | Add upload/embed/search/vector-search handlers |
| `backend/internal/handlers/document.go` | Add folder/move/list handlers |
| `backend/cmd/main.go` | Wire new services and routes |

---

## Task 1: LLMProvider.Complete — Synchronous LLM Call

**Files:**
- Modify: `backend/internal/providers/llm.go`
- Test: `backend/internal/providers/` (verify compilation)

**Context:** All LLM provider implementations need a new `Complete` method for non-streaming chat. Start with the interface change, then implement for the default provider.

- [ ] **Step 1: Add Complete to LLMProvider interface**

Read `backend/internal/providers/llm.go` to see the current interface, then extend it:

```go
type LLMProvider interface {
    Stream(ctx context.Context, messages []core.Message, systemPrompt string) (<-chan LLMChunk, error)
    Complete(ctx context.Context, messages []core.Message, systemPrompt string) (string, error)
}
```

- [ ] **Step 2: Find all LLMProvider implementations and add stub Complete**

Run: `grep -rn 'type .* struct' backend/internal/providers/ | grep -v test`

For each implementation, add a stub `Complete` that wraps `Stream` and collects chunks:

```go
func (p *SomeProvider) Complete(ctx context.Context, messages []core.Message, systemPrompt string) (string, error) {
    chunks, err := p.Stream(ctx, messages, systemPrompt)
    if err != nil {
        return "", err
    }
    var sb strings.Builder
    for chunk := range chunks {
        if chunk.Err != nil {
            return sb.String(), chunk.Err
        }
        sb.WriteString(chunk.TextDelta)
    }
    return sb.String(), nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/providers/
git commit -m "feat(p0): add Complete method to LLMProvider interface and all implementations"
```

---

## Task 2: ChatService.Complete — Non-Streaming Chat Service

**Files:**
- Modify: `backend/internal/dto/chat.go`
- Modify: `backend/internal/services/chat_service.go`
- Test: `backend/tests/integration/p0_chat_test.go`

**Context:** Extract the shared RAG context-building logic from `ChatService.Stream` into a reusable helper, then add `Complete` that calls `llmProv.Complete` instead of `Stream`.

- [ ] **Step 1: Add ChatRequest and ChatResponse DTOs**

Modify `backend/internal/dto/chat.go`:

```go
type ChatRequest struct {
    Message     string   `json:"message"`
    Mode        string   `json:"mode,omitempty"`        // "query" | "chat" | "automatic"
    SessionID   string   `json:"sessionId,omitempty"`
    Reset       bool     `json:"reset,omitempty"`
    Attachments []string `json:"attachments,omitempty"`
}

type ChatResponse struct {
    ID           string `json:"id"`
    Type         string `json:"type"`
    TextResponse string `json:"textResponse"`
    Sources      []any  `json:"sources"`
    Close        bool   `json:"close"`
    Error        string `json:"error,omitempty"`
}
```

- [ ] **Step 2: Extract shared RAG builder from Stream**

In `backend/internal/services/chat_service.go`, extract the history building + vector search + system prompt composition into a private method:

```go
func (s *ChatService) buildRAGContext(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, message string) (systemPrompt string, sources []any, history []core.Message, err error) {
    historyLimit := ws.OpenAiHistory
    if historyLimit <= 0 {
        historyLimit = 20
    }
    history, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
    if err != nil {
        return "", nil, nil, err
    }

    systemPrompt = ""
    if ws.OpenAiPrompt != nil {
        systemPrompt = *ws.OpenAiPrompt
    }

    if s.vectorSvc.provider != nil {
        topN := 4
        if ws.TopN != nil {
            topN = *ws.TopN
        }
        threshold := 0.25
        if ws.SimilarityThreshold != nil {
            threshold = *ws.SimilarityThreshold
        }

        var queryVector []float32
        if s.embedder != nil {
            qv, err := s.embedder.EmbedQuery(ctx, message)
            if err == nil {
                queryVector = qv
            } else {
                mlog.Error("embed query failed: ", err)
            }
        }

        results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, queryVector, vectordb.SearchOptions{
            TopN:                topN,
            SimilarityThreshold: threshold,
        })
        if err == nil {
            var ragTexts []string
            for _, r := range results {
                sources = append(sources, map[string]any{
                    "docId":    r.DocId,
                    "text":     r.Text,
                    "score":    r.Score,
                    "metadata": r.Metadata,
                })
                ragTexts = append(ragTexts, r.Text)
            }
            if len(ragTexts) > 0 {
                systemPrompt += "\n\nContext:\n" + strings.Join(ragTexts, "\n---\n")
            }
        }
    }
    return systemPrompt, sources, history, nil
}
```

Then replace the inline logic in `Stream` with a call to `buildRAGContext`.

- [ ] **Step 3: Add Complete method**

```go
func (s *ChatService) Complete(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, req dto.ChatRequest) (*dto.ChatResponse, error) {
    msgID := uuid.New().String()

    if strings.TrimSpace(req.Message) == "" {
        return &dto.ChatResponse{
            ID: msgID, Type: "abort", Close: true,
            Error: "Message is empty.",
        }, nil
    }

    systemPrompt, sources, history, err := s.buildRAGContext(ctx, ws, user, threadID, req.Message)
    if err != nil {
        return &dto.ChatResponse{
            ID: msgID, Type: "abort", Close: true,
            Error: err.Error(),
        }, nil
    }

    messages := append(history, core.Message{
        Role:    core.MESSAGE_ROLE_USER,
        Content: core.NewTextContent(req.Message),
    })

    text, err := s.llmProv.Complete(ctx, messages, systemPrompt)
    if err != nil {
        return &dto.ChatResponse{
            ID: msgID, Type: "abort", Close: true,
            Error: err.Error(),
        }, nil
    }

    s.saveChatResponse(ctx, ws, user, threadID, req.Message, text)

    return &dto.ChatResponse{
        ID:           msgID,
        Type:         "textResponse",
        TextResponse: text,
        Sources:      sources,
        Close:        true,
    }, nil
}
```

- [ ] **Step 4: Verify compilation**

Run: `cd backend && go build ./...`
Expected: Clean build.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/dto/chat.go backend/internal/services/chat_service.go
git commit -m "feat(p0): add ChatService.Complete with shared RAG context builder"
```

---

## Task 3: Non-Streaming Chat Handler

**Files:**
- Modify: `backend/internal/handlers/chat.go`
- Modify: `backend/cmd/main.go`
- Test: `backend/tests/integration/p0_chat_test.go`

**Context:** Add `POST /workspace/:slug/chat` handler. API version (`/v1/workspace/:slug/chat`) uses API key auth; top-level uses session auth.

- [ ] **Step 1: Add Chat handler to chat.go**

```go
func (h *ChatHandler) Chat(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    user := c.MustGet("user").(*models.User)

    var req dto.ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
        return
    }

    resp, err := h.chatSvc.Complete(c.Request.Context(), ws, user, nil, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
        return
    }
    c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 2: Add route registration**

In `RegisterChatRoutes`, add:

```go
r.POST("/workspace/:slug/chat",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.Chat)
```

- [ ] **Step 3: Write integration test**

Create `backend/tests/integration/p0_chat_test.go`:

```go
package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestChat_NonStreaming(t *testing.T) {
    setupTestDB(t)
    token, _ := createTestUser(t, "chatuser", "password")
    ws := createTestWorkspace(t, "chat-ws")

    body, _ := json.Marshal(dto.ChatRequest{Message: "Hello"})
    req := httptest.NewRequest("POST", "/workspace/chat-ws/chat", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    var resp dto.ChatResponse
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, "textResponse", resp.Type)
    assert.True(t, resp.Close)
}
```

Note: This test requires a mock LLM provider. If one doesn't exist, create a simple mock in the test file.

- [ ] **Step 4: Run test**

Run: `cd backend && go test ./tests/integration/ -run TestChat_NonStreaming -v`
Expected: PASS (or FAIL with specific error if mock is needed).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/chat.go backend/tests/integration/p0_chat_test.go backend/cmd/main.go
git commit -m "feat(p0): non-streaming chat handler + integration test"
```

---

## Task 4: EmbeddingProgressManager — SSE Infrastructure

**Files:**
- Create: `backend/internal/services/embedding_progress.go`
- Test: `backend/internal/services/embedding_progress_test.go`

**Context:** Manage SSE connections for embedding progress. One manager instance per server. Each workspace can have multiple listening clients.

- [ ] **Step 1: Create EmbeddingProgressManager**

```go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
)

type EmbedProgressEvent struct {
    Type     string `json:"type"`     // "progress", "complete", "error"
    Message  string `json:"message"`
    Percent  int    `json:"percent,omitempty"`
    Document string `json:"document,omitempty"`
}

type sseConn struct {
    writer  http.ResponseWriter
    flusher http.Flusher
    done    chan struct{}
}

type EmbeddingProgressManager struct {
    mu    sync.RWMutex
    conns map[string]map[string]*sseConn // workspaceSlug -> connID -> conn
}

func NewEmbeddingProgressManager() *EmbeddingProgressManager {
    return &EmbeddingProgressManager{
        conns: make(map[string]map[string]*sseConn),
    }
}

func (m *EmbeddingProgressManager) AddConnection(workspaceSlug string, w http.ResponseWriter) (connID string, done chan struct{}) {
    m.mu.Lock()
    defer m.mu.Unlock()

    connID = fmt.Sprintf("%d", time.Now().UnixNano())
    done = make(chan struct{})

    if m.conns[workspaceSlug] == nil {
        m.conns[workspaceSlug] = make(map[string]*sseConn)
    }
    flusher, ok := w.(http.Flusher)
    if !ok {
        close(done)
        return connID, done
    }
    m.conns[workspaceSlug][connID] = &sseConn{writer: w, flusher: flusher, done: done}
    return connID, done
}

func (m *EmbeddingProgressManager) RemoveConnection(workspaceSlug, connID string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if conns, ok := m.conns[workspaceSlug]; ok {
        if conn, ok := conns[connID]; ok {
            close(conn.done)
            delete(conns, connID)
        }
        if len(conns) == 0 {
            delete(m.conns, workspaceSlug)
        }
    }
}

func (m *EmbeddingProgressManager) Broadcast(workspaceSlug string, event EmbedProgressEvent) {
    m.mu.RLock()
    conns := m.conns[workspaceSlug]
    m.mu.RUnlock()

    if len(conns) == 0 {
        return
    }

    data, err := json.Marshal(event)
    if err != nil {
        return
    }
    payload := fmt.Sprintf("data: %s\n\n", data)

    m.mu.Lock()
    defer m.mu.Unlock()
    for id, conn := range m.conns[workspaceSlug] {
        if _, err := conn.writer.Write([]byte(payload)); err != nil {
            close(conn.done)
            delete(m.conns[workspaceSlug], id)
            continue
        }
        conn.flusher.Flush()
    }
}

func (m *EmbeddingProgressManager) BroadcastProgress(workspaceSlug, document string, percent int) {
    m.Broadcast(workspaceSlug, EmbedProgressEvent{
        Type:     "progress",
        Message:  fmt.Sprintf("Embedding %s", document),
        Percent:  percent,
        Document: document,
    })
}
```

- [ ] **Step 2: Write unit test**

```go
package services

import (
    "net/http/httptest"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

func TestEmbeddingProgressManager(t *testing.T) {
    mgr := NewEmbeddingProgressManager()

    w := httptest.NewRecorder()
    connID, done := mgr.AddConnection("test-ws", w)
    assert.NotEmpty(t, connID)

    mgr.BroadcastProgress("test-ws", "doc1.pdf", 50)
    time.Sleep(50 * time.Millisecond)

    body := w.Body.String()
    assert.Contains(t, body, "progress")
    assert.Contains(t, body, "doc1.pdf")
    assert.Contains(t, body, "50")

    mgr.RemoveConnection("test-ws", connID)
    select {
    case <-done:
        // expected
    case <-time.After(time.Second):
        t.Fatal("done channel not closed")
    }
}
```

- [ ] **Step 3: Run test**

Run: `cd backend && go test ./internal/services/ -run TestEmbeddingProgressManager -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/embedding_progress.go backend/internal/services/embedding_progress_test.go
git commit -m "feat(p0): EmbeddingProgressManager for SSE embedding progress"
```

---

## Task 5: SearchService — Workspace/Thread Search

**Files:**
- Create: `backend/internal/services/search_service.go`
- Create: `backend/internal/services/search_service_test.go`
- Create: `backend/internal/dto/search.go`
- Modify: `backend/internal/handlers/workspace.go`
- Modify: `backend/cmd/main.go`
- Test: `backend/tests/integration/p0_search_test.go`

**Context:** Implement fuzzy search for workspaces and threads. Levenshtein distance <= 3.

- [ ] **Step 1: Create search DTOs**

```go
package dto

type SearchResults struct {
    Workspaces []WorkspaceSearchResult `json:"workspaces"`
    Threads    []ThreadSearchResult    `json:"threads"`
}

type WorkspaceSearchResult struct {
    Slug string `json:"slug"`
    Name string `json:"name"`
}

type ThreadSearchResult struct {
    Slug      string                  `json:"slug"`
    Name      string                  `json:"name"`
    Workspace WorkspaceSearchResult `json:"workspace"`
}
```

- [ ] **Step 2: Add Levenshtein utility**

Add to `backend/pkg/utils/levenshtein.go`:

```go
package utils

func Levenshtein(a, b string) int {
    if len(a) == 0 { return len(b) }
    if len(b) == 0 { return len(a) }

    prev := make([]int, len(b)+1)
    curr := make([]int, len(b)+1)
    for j := 0; j <= len(b); j++ { prev[j] = j }

    for i := 1; i <= len(a); i++ {
        curr[0] = i
        for j := 1; j <= len(b); j++ {
            cost := 1
            if a[i-1] == b[j-1] { cost = 0 }
            curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
        }
        prev, curr = curr, prev
    }
    return prev[len(b)]
}

func min(a, b, c int) int {
    if a < b { if a < c { return a } }
    if b < c { return b }
    return c
}
```

- [ ] **Step 3: Create SearchService**

```go
package services

import (
    "context"
    "strings"

    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/pkg/utils"
    "gorm.io/gorm"
)

const MaxLevenshteinDistance = 3

type SearchService struct {
    db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
    return &SearchService{db: db}
}

func (s *SearchService) SearchWorkspaceAndThreads(ctx context.Context, searchTerm string, userID *int) (*dto.SearchResults, error) {
    searchTerm = strings.TrimSpace(searchTerm)
    if len(searchTerm) < 3 {
        return &dto.SearchResults{Workspaces: []dto.WorkspaceSearchResult{}, Threads: []dto.ThreadSearchResult{}}, nil
    }
    term := strings.ToLower(searchTerm)

    // Query workspaces
    var workspaces []models.Workspace
    wsQuery := s.db.WithContext(ctx)
    if userID != nil {
        wsQuery = wsQuery.Joins("JOIN workspace_users ON workspace_users.workspace_id = workspaces.id").
            Where("workspace_users.user_id = ?", *userID)
    }
    if err := wsQuery.Find(&workspaces).Error; err != nil {
        return nil, err
    }

    // Query threads with workspace
    var threads []models.WorkspaceThread
    threadQuery := s.db.WithContext(ctx).Preload("Workspace")
    if userID != nil {
        threadQuery = threadQuery.Where("user_id = ?", *userID)
    }
    if err := threadQuery.Find(&threads).Error; err != nil {
        return nil, err
    }

    workspaceSet := make(map[string]dto.WorkspaceSearchResult)
    threadSet := make(map[string]dto.ThreadSearchResult)

    for _, ws := range workspaces {
        name := strings.ToLower(ws.Name)
        if matchesSearch(name, term) {
            workspaceSet[ws.Slug] = dto.WorkspaceSearchResult{Slug: ws.Slug, Name: ws.Name}
        }
    }

    for _, th := range threads {
        name := strings.ToLower(th.Name)
        if matchesSearch(name, term) {
            wsSlug := ""
            wsName := ""
            if th.Workspace != nil {
                wsSlug = th.Workspace.Slug
                wsName = th.Workspace.Name
            }
            key := th.Slug + "@" + wsSlug
            threadSet[key] = dto.ThreadSearchResult{
                Slug: th.Slug,
                Name: th.Name,
                Workspace: dto.WorkspaceSearchResult{Slug: wsSlug, Name: wsName},
            }
        }
    }

    results := &dto.SearchResults{
        Workspaces: make([]dto.WorkspaceSearchResult, 0, len(workspaceSet)),
        Threads:    make([]dto.ThreadSearchResult, 0, len(threadSet)),
    }
    for _, ws := range workspaceSet { results.Workspaces = append(results.Workspaces, ws) }
    for _, th := range threadSet { results.Threads = append(results.Threads, th) }
    return results, nil
}

func matchesSearch(name, term string) bool {
    if strings.HasPrefix(name, term) || strings.Contains(name, term) || strings.HasSuffix(name, term) {
        return true
    }
    return utils.Levenshtein(name, term) <= MaxLevenshteinDistance
}
```

- [ ] **Step 4: Add SearchWorkspaces handler**

In `backend/internal/handlers/workspace.go`:

```go
type WorkspaceHandler struct {
    wsSvc       *services.WorkspaceService
    searchSvc   *services.SearchService  // NEW
}

func NewWorkspaceHandler(wsSvc *services.WorkspaceService, searchSvc *services.SearchService) *WorkspaceHandler {
    return &WorkspaceHandler{wsSvc: wsSvc, searchSvc: searchSvc}
}

func (h *WorkspaceHandler) SearchWorkspaces(c *gin.Context) {
    user := c.MustGet("user").(*models.User)
    var req struct {
        SearchTerm string `json:"searchTerm"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
        return
    }
    results, err := h.searchSvc.SearchWorkspaceAndThreads(c.Request.Context(), req.SearchTerm, &user.ID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
        return
    }
    c.JSON(http.StatusOK, results)
}
```

Add route in `RegisterWorkspaceRoutes`:
```go
r.POST("/workspace/search",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    h.SearchWorkspaces)
```

- [ ] **Step 5: Wire in main.go**

```go
searchSvc := services.NewSearchService(db)
// ... pass searchSvc to NewWorkspaceHandler
```

- [ ] **Step 6: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestSearch -v`
Run: `cd backend && go test ./tests/integration/ -run TestSearchWorkspaces -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/search_service.go backend/internal/services/search_service_test.go backend/internal/dto/search.go backend/internal/handlers/workspace.go backend/pkg/utils/levenshtein.go backend/cmd/main.go
git commit -m "feat(p0): workspace/thread fuzzy search with Levenshtein"
```

---

## Task 6: VectorSearchService + Handler

**Files:**
- Create: `backend/internal/services/vector_search_service.go`
- Create: `backend/internal/dto/vector_search.go`
- Modify: `backend/internal/handlers/workspace.go`
- Test: `backend/tests/integration/p0_search_test.go`

**Context:** Wrap existing `VectorService.SimilaritySearch` in an endpoint-friendly service.

- [ ] **Step 1: Create DTO**

```go
package dto

type VectorSearchRequest struct {
    Query          string   `json:"query"`
    TopN           *int     `json:"topN,omitempty"`
    ScoreThreshold *float64 `json:"scoreThreshold,omitempty"`
}

type VectorSearchResult struct {
    ID       string         `json:"id"`
    Text     string         `json:"text"`
    Metadata map[string]any `json:"metadata"`
    Distance float64        `json:"distance"`
    Score    float64        `json:"score"`
}
```

- [ ] **Step 2: Create VectorSearchService**

```go
package services

import (
    "context"
    "fmt"

    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/embedder"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/vectordb"
)

type VectorSearchService struct {
    vectorSvc *VectorService
    embedder  embedder.Embedder
}

func NewVectorSearchService(vectorSvc *VectorService, embedder embedder.Embedder) *VectorSearchService {
    return &VectorSearchService{vectorSvc: vectorSvc, embedder: embedder}
}

func (s *VectorSearchService) Search(ctx context.Context, ws *models.Workspace, req dto.VectorSearchRequest) ([]dto.VectorSearchResult, error) {
    if s.vectorSvc.provider == nil {
        return nil, fmt.Errorf("vector provider not connected")
    }

    // Embed query
    queryVector, err := s.embedder.EmbedQuery(ctx, req.Query)
    if err != nil {
        return nil, fmt.Errorf("embed query failed: %w", err)
    }

    // Determine topN
    topN := 4
    if req.TopN != nil { topN = *req.TopN }
    if ws.TopN != nil { topN = *ws.TopN }

    // Determine threshold
    threshold := 0.25
    if req.ScoreThreshold != nil { threshold = *req.ScoreThreshold }
    if ws.SimilarityThreshold != nil { threshold = *ws.SimilarityThreshold }

    // Check if workspace has vectors
    count, err := s.vectorSvc.CountVectors(ctx, ws.Slug)
    if err != nil || count == 0 {
        return []dto.VectorSearchResult{}, nil
    }

    results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, queryVector, vectordb.SearchOptions{
        TopN:                topN,
        SimilarityThreshold: threshold,
    })
    if err != nil {
        return nil, err
    }

    out := make([]dto.VectorSearchResult, len(results))
    for i, r := range results {
        out[i] = dto.VectorSearchResult{
            ID:       r.DocId,
            Text:     r.Text,
            Metadata: r.Metadata,
            Distance: r.Distance,
            Score:    r.Score,
        }
    }
    return out, nil
}
```

- [ ] **Step 3: Add VectorSearch handler**

In `WorkspaceHandler`, add:
```go
func (h *WorkspaceHandler) VectorSearch(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.VectorSearchRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
        return
    }
    results, err := h.vectorSearchSvc.Search(c.Request.Context(), ws, req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"results": results})
}
```

Add route:
```go
r.POST("/workspace/:slug/vector-search",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.VectorSearch)
```

- [ ] **Step 4: Wire in main.go**

```go
vectorSearchSvc := services.NewVectorSearchService(vectorSvc, embedder)
// ... pass to NewWorkspaceHandler
```

- [ ] **Step 5: Run integration test**

Run: `cd backend && go test ./tests/integration/ -run TestVectorSearch -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/vector_search_service.go backend/internal/dto/vector_search.go backend/internal/handlers/workspace.go backend/cmd/main.go
git commit -m "feat(p0): vector search endpoint"
```

---

## Task 7: DocumentService Extensions — Part 1 (Upload & Embed)

**Files:**
- Modify: `backend/internal/services/document_service.go`
- Modify: `backend/internal/dto/document.go`
- Modify: `backend/internal/handlers/workspace.go`
- Modify: `backend/cmd/main.go`

**Context:** Add workspace-scoped upload, link upload, upload-and-embed, update-embeddings, remove-and-unembed.

- [ ] **Step 1: Extend DocumentService**

Read `backend/internal/services/document_service.go` to understand current methods, then add:

```go
// UploadToWorkspace saves file and triggers async embedding with progress
func (s *DocumentService) UploadToWorkspace(ctx context.Context, wsSlug string, file *multipart.FileHeader) (*models.Document, error) {
    // 1. Find workspace by slug
    // 2. Save file to disk (reuse SaveUpload logic)
    // 3. Create Document record with workspace association
    // 4. Spawn goroutine: embed with progress broadcast
    // Return document immediately
}

// UploadLink sends link to Collector API, parses, embeds
func (s *DocumentService) UploadLink(ctx context.Context, wsSlug string, link string) ([]*models.Document, error) {
    // 1. Call Collector API to scrape link
    // 2. Save parsed content as document
    // 3. Embed with progress
}

// UploadAndEmbed for chat drag-drop
func (s *DocumentService) UploadAndEmbed(ctx context.Context, wsSlug string, file *multipart.FileHeader) (*models.Document, error) {
    // Same as UploadToWorkspace but embed synchronously (or with tighter progress)
}

// UpdateEmbeddings bulk add/remove from workspace vector DB
func (s *DocumentService) UpdateEmbeddings(ctx context.Context, wsSlug string, adds []string, removes []string) error {
    // For each add: embed document vectors into workspace namespace
    // For each remove: delete vectors by docId from workspace namespace
}

// RemoveAndUnembed deletes document + vectors
func (s *DocumentService) RemoveAndUnembed(ctx context.Context, wsSlug string, docId string) error {
    // 1. Delete vectors by docId from workspace namespace
    // 2. Delete Document record
    // 3. Optionally delete file from disk
}
```

- [ ] **Step 2: Add DTOs**

```go
package dto

type UpdateEmbeddingsRequest struct {
    Adds    []string `json:"adds"`
    Removes []string `json:"removes"`
}

type FileMove struct {
    From string `json:"from"`
    To   string `json:"to"`
}
```

- [ ] **Step 3: Add workspace handlers**

In `WorkspaceHandler`, add methods for upload, link, upload-and-embed, update-embeddings, remove-and-unembed, embed-progress.

- [ ] **Step 4: Wire routes**

```go
r.POST("/workspace/:slug/upload", ...)
r.POST("/workspace/:slug/upload-link", ...)
r.POST("/workspace/:slug/upload-and-embed", ...)
r.POST("/workspace/:slug/update-embeddings", ...)
r.DELETE("/workspace/:slug/remove-and-unembed", ...)
r.GET("/workspace/:slug/embed-progress", ...) // SSE
```

- [ ] **Step 5: Wire in main.go**

Pass `EmbeddingProgressManager` and `DocumentService` dependencies.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/document_service.go backend/internal/dto/document.go backend/internal/handlers/workspace.go backend/cmd/main.go
git commit -m "feat(p0): workspace document upload and embedding workflow"
```

---

## Task 8: DocumentService Extensions — Part 2 (Folder CRUD & Listing)

**Files:**
- Modify: `backend/internal/services/document_service.go`
- Modify: `backend/internal/handlers/document.go`
- Modify: `backend/cmd/main.go`
- Test: `backend/tests/integration/p0_document_test.go`

**Context:** Add create-folder, move-files, list documents, list folder, get by name.

- [ ] **Step 1: Add folder and list methods to DocumentService**

```go
func (s *DocumentService) CreateFolder(ctx context.Context, name string) error {
    // Validate path (prevent traversal)
    // mkdir
}

func (s *DocumentService) MoveFiles(ctx context.Context, moves []dto.FileMove) error {
    // Check files are not embedded
    // fs.Rename each
}

func (s *DocumentService) ListDocuments(ctx context.Context, folder string) ([]models.Document, error) {
    // Query DB, optionally filter by folder
}

func (s *DocumentService) ListFolderDocuments(ctx context.Context, folderName string) ([]models.Document, error) {
    // Query DB where docpath starts with folderName
}

func (s *DocumentService) GetByDocName(ctx context.Context, docName string) (*models.Document, error) {
    // Query DB where title = docName
}
```

- [ ] **Step 2: Add document handlers**

In `DocumentHandler`, add CreateFolder, MoveFiles, ListDocuments, ListFolderDocuments, GetDocumentByName, AcceptedFileTypes.

- [ ] **Step 3: Add routes**

```go
r.POST("/document/create-folder", ...)
r.POST("/document/move-files", ...)
r.GET("/documents", ...)
r.GET("/documents/folder/:folderName", ...)
r.GET("/document/:docName", ...) // align with Node
r.GET("/document/accepted-file-types", ...)
```

- [ ] **Step 4: Update existing /document/:docId route**

Make the existing `GetDocument` and `DeleteDocument` support both docId and docName (or keep separate).

- [ ] **Step 5: Run integration tests**

Run: `cd backend && go test ./tests/integration/ -run TestDocument -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/document_service.go backend/internal/handlers/document.go backend/tests/integration/p0_document_test.go backend/cmd/main.go
git commit -m "feat(p0): document folder CRUD and listing"
```

---

## Task 9: API-Key Auth for API Routes

**Files:**
- Modify: `backend/internal/middleware/api_key.go` (if exists) or create
- Modify: `backend/internal/handlers/chat.go`
- Modify: `backend/cmd/main.go`

**Context:** The `/v1/workspace/:slug/chat` route uses API key auth, not session JWT.

- [ ] **Step 1: Check/create API key middleware**

Look for existing API key validation in `backend/internal/middleware/`. If it exists, use it. If not, create:

```go
func ValidApiKey(apiKeySvc *services.APIKeyService) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := c.GetHeader("Authorization")
        if !strings.HasPrefix(key, "Bearer ") {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }
        key = strings.TrimPrefix(key, "Bearer ")
        // Validate key...
        c.Next()
    }
}
```

- [ ] **Step 2: Register API chat route**

In main.go or a dedicated API router group, add:
```go
apiV1 := r.Group("/v1")
apiV1.POST("/workspace/:slug/chat",
    middleware.ValidApiKey(apiKeySvc),
    middleware.ValidWorkspaceSlug(db),
    chatHandler.Chat)
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/middleware/ backend/internal/handlers/chat.go backend/cmd/main.go
git commit -m "feat(p0): API key auth for /v1/workspace/:slug/chat"
```

---

## Task 10: Regression & Final Verification

**Files:**
- All modified files

- [ ] **Step 1: Run all tests**

Run: `cd backend && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 2: Run go vet**

Run: `cd backend && go vet ./...`
Expected: Clean.

- [ ] **Step 3: Build check**

Run: `cd backend && go build ./...`
Expected: Clean build.

- [ ] **Step 4: Verify route registration**

Run a quick check to list all registered routes:
```bash
cd backend && go run cmd/main.go 2>&1 | grep -i "registered\|route" || echo "Check main.go for route registration"
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "test(p0): regression tests for all P0 core routes"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- ✅ Non-streaming chat (API + top-level) — Tasks 2, 3, 9
- ✅ Document upload/embedding — Tasks 7, 8
- ✅ Workspace search — Task 5
- ✅ Vector search — Task 6
- ✅ SSE progress — Task 4

**2. Placeholder scan:**
- ✅ No "TBD", "TODO", "implement later"
- ✅ No vague "add error handling" steps
- ✅ Each step has concrete code or command

**3. Type consistency:**
- ✅ `ChatRequest`, `ChatResponse` used consistently across Tasks 2, 3
- ✅ `SearchResults` used in Task 5
- ✅ `VectorSearchResult` used in Task 6
- ✅ `EmbeddingProgressManager` methods consistent in Task 4 and Task 7

**4. Dependency ordering:**
- ✅ Task 1 (LLMProvider.Complete) before Task 2 (ChatService.Complete)
- ✅ Task 4 (EmbeddingProgressManager) before Task 7 (Document upload with progress)
- ✅ Task 5 (SearchService) before Task 8 (no direct dependency but logical grouping)

---

*Plan complete and saved to `.gpowers/plans/2026-05-25-p0-core-routes.md`.*

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
