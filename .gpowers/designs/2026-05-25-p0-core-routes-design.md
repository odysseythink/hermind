# P0 Core Routes — Design Document

> backend 替代 Node server 的 P0 核心路由实现设计
> Date: 2026-05-25
> Scope: 非流式聊天、文档上传/嵌入工作流、工作区搜索、向量搜索

---

## 1. 架构总览

P0 新增 3 层基础设施 + 4 个领域扩展：

```
┌─────────────────────────────────────────────────────────────┐
│  Handler Layer                                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐ │
│  │ Chat     │ │ Workspace│ │ Document │ │ Search/Vector  │ │
│  │ (sync)   │ │ (upload) │ │ (CRUD)   │ │                │ │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └───────┬────────┘ │
│       │            │            │               │           │
│  ┌────┴────────────┴────────────┴───────────────┴────────┐ │
│  │  Service Layer                                          │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │ │
│  │  │ChatService│ │Workspace │ │Document  │ │Search    │  │ │
│  │  │(+Sync)   │ │Service   │ │Service   │ │Service   │  │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │ EmbeddingProgressManager (SSE)                   │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  └─────────────────────────────────────────────────────────┘ │
│       │            │            │               │            │
│  ┌────┴────────────┴────────────┴───────────────┴────────┐ │
│  │  Infrastructure Layer                                   │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │ │
│  │  │ LLMProv  │ │ Embedder │ │ VectorDB │ │ Pantheon │  │ │
│  │  │(+Sync)  │ │          │ │          │ │          │  │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 基础设施层（第一层）

### 2.1 EmbeddingProgressManager — SSE 进度推送

Node 的 `EmbeddingWorkerManager` 管理 SSE 连接，在嵌入过程中推送进度事件。Go 中需要等效实现。

**设计要点：**
- 每个 workspace slug 对应一个 SSE 客户端集合（支持多客户端同时监听同一 workspace）
- 使用 `sync.RWMutex` 保护连接映射
- 进度事件通过 channel 异步分发
- 客户端断开时自动清理

```go
// internal/services/embedding_progress.go
type EmbeddingProgressManager struct {
    mu     sync.RWMutex
    conns  map[string]map[string]*SSEConn // workspaceSlug -> connID -> conn
}

type SSEConn struct {
    Writer  http.ResponseWriter
    Flusher http.Flusher
    Done    chan struct{}
}

type EmbedProgressEvent struct {
    Type     string `json:"type"`     // "progress", "complete", "error"
    Message  string `json:"message"`
    Percent  int    `json:"percent,omitempty"`
    Document string `json:"document,omitempty"`
}
```

**方法：**
- `AddConnection(workspaceSlug string, w http.ResponseWriter) (connID string, done chan struct{})`
- `RemoveConnection(workspaceSlug, connID string)`
- `Broadcast(workspaceSlug string, event EmbedProgressEvent)`
- `BroadcastProgress(workspaceSlug, document string, percent int)`

**Handler 用法：**
```go
func (h *WorkspaceHandler) EmbedProgress(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    connID, done := h.progressMgr.AddConnection(ws.Slug, c.Writer)
    defer h.progressMgr.RemoveConnection(ws.Slug, connID)
    
    // Block until client disconnects
    <-done
}
```

**嵌入服务用法：**
```go
func (s *DocumentService) embedWithProgress(ctx context.Context, wsSlug string, docs []models.Document) {
    for i, doc := range docs {
        s.progressMgr.BroadcastProgress(wsSlug, doc.Title, i*100/len(docs))
        // ... do embedding
    }
    s.progressMgr.Broadcast(wsSlug, EmbedProgressEvent{Type: "complete"})
}
```

### 2.2 同步 LLM 调用

当前 `ChatService.Stream()` 返回 channel。非流式聊天需要同步调用。

**方案：** 在 `providers.LLMProvider` 接口中新增 `Complete` 方法，或者在 `ChatService` 中包装 stream 为同步调用。

**选择：ChatService 包装**（更简洁，不改 provider 接口）

```go
// internal/services/chat_service.go
func (s *ChatService) Complete(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, req dto.ChatRequest) (*dto.ChatResponse, error) {
    // Reuse the same RAG context building as Stream()
    // But call llmProv.Complete() instead of Stream()
}
```

**需要新增 provider 方法：**
```go
// internal/providers/llm.go
type LLMProvider interface {
    Stream(ctx context.Context, messages []core.Message, systemPrompt string) (<-chan LLMChunk, error)
    Complete(ctx context.Context, messages []core.Message, systemPrompt string) (string, error) // NEW
}
```

**为什么新增 Complete 而不是包装 Stream：**
- 某些 provider 可能支持真正的同步 API（如 OpenAI 的 `chat.completions` 非 stream 模式）
- 包装 Stream 需要收集所有 chunk，对于大响应可能内存问题
- 更 clean 的抽象

**ChatRequest DTO 扩展：**
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

---

## 3. 服务层（第二层）

### 3.1 SearchService

```go
// internal/services/search_service.go
type SearchService struct {
    db *gorm.DB
}

func (s *SearchService) SearchWorkspaceAndThreads(ctx context.Context, searchTerm string, userID *int) (*dto.SearchResults, error) {
    // 1. searchTerm length >= 3
    // 2. Query workspaces (filtered by userID if multi-user)
    // 3. Query threads (filtered by userID, with workspace relation)
    // 4. Fuzzy match: startsWith / includes / endsWith / levenshtein distance <= 3
    // 5. Return {workspaces: [{slug, name}], threads: [{slug, name, workspace: {slug, name}}]}
}
```

**Levenshtein 实现：** 引入 `github.com/agnivade/levenshtein` 或自实现（~50 行）

### 3.2 DocumentService 扩展

当前 `DocumentService` 只有 `SaveUpload`, `EmbedDocument`, `GetByDocId`, `DeleteByDocId`。

**新增方法：**

```go
// Workspace-scoped upload (POST /workspace/:slug/upload)
func (s *DocumentService) UploadToWorkspace(ctx context.Context, wsSlug string, file *multipart.FileHeader) (*models.Document, error)

// Link upload (POST /workspace/:slug/upload-link)
func (s *DocumentService) UploadLink(ctx context.Context, wsSlug string, link string) ([]*models.Document, error)

// Upload and embed for chat drag-drop (POST /workspace/:slug/upload-and-embed)
func (s *DocumentService) UploadAndEmbed(ctx context.Context, wsSlug string, file *multipart.FileHeader) (*models.Document, error)

// Bulk add/remove documents from workspace vector DB (POST /workspace/:slug/update-embeddings)
func (s *DocumentService) UpdateEmbeddings(ctx context.Context, wsSlug string, adds []string, removes []string) error

// Remove document + purge vectors (DELETE /workspace/:slug/remove-and-unembed)
func (s *DocumentService) RemoveAndUnembed(ctx context.Context, wsSlug string, docId string) error

// Create folder (POST /document/create-folder)
func (s *DocumentService) CreateFolder(ctx context.Context, name string) error

// Move files (POST /document/move-files)
func (s *DocumentService) MoveFiles(ctx context.Context, moves []dto.FileMove) error

// List documents (GET /documents)
func (s *DocumentService) ListDocuments(ctx context.Context, folder string) ([]models.Document, error)

// List folder documents (GET /documents/folder/:folderName)
func (s *DocumentService) ListFolderDocuments(ctx context.Context, folderName string) ([]models.Document, error)

// Get document by name (GET /document/:docName) — unify with existing /document/:docId
func (s *DocumentService) GetByDocName(ctx context.Context, docName string) (*models.Document, error)

// Delete document + vectors (enhance existing DeleteByDocId)
func (s *DocumentService) DeleteByDocId(ctx context.Context, docId string) error
```

### 3.3 VectorSearchService

```go
// internal/services/vector_search_service.go
type VectorSearchService struct {
    vectorSvc *VectorService
    embedder  embedder.Embedder
}

func (s *VectorSearchService) Search(ctx context.Context, ws *models.Workspace, query string, topN *int, scoreThreshold *float64) ([]dto.VectorSearchResult, error) {
    // 1. Embed query string
    // 2. Check if workspace has vectors (CountVectors > 0)
    // 3. Call vectorSvc.SimilaritySearch with workspace defaults
    // 4. Return results with id, text, metadata, distance, score
}
```

---

## 4. Handler 层（第三层）

### 4.1 Chat Handler 扩展

```go
// Non-streaming chat — top-level route
r.POST("/workspace/:slug/chat",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.Chat)

// Non-streaming chat — API route  
r.POST("/v1/workspace/:slug/chat",
    middleware.ValidApiKey(apiKeySvc),  // API key auth, not session
    middleware.ValidWorkspaceSlug(db),
    h.ApiChat)
```

### 4.2 Workspace Handler 扩展

```go
// Upload to workspace
r.POST("/workspace/:slug/upload",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    middleware.ValidWorkspaceSlug(db),
    h.UploadToWorkspace)

// Upload link
r.POST("/workspace/:slug/upload-link",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    middleware.ValidWorkspaceSlug(db),
    h.UploadLink)

// Upload and embed (chat drag-drop)
r.POST("/workspace/:slug/upload-and-embed",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.UploadAndEmbed)

// Update embeddings (bulk add/remove)
r.POST("/workspace/:slug/update-embeddings",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    middleware.ValidWorkspaceSlug(db),
    h.UpdateEmbeddings)

// Remove and unembed
r.DELETE("/workspace/:slug/remove-and-unembed",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    middleware.ValidWorkspaceSlug(db),
    h.RemoveAndUnembed)

// Embed progress SSE
r.GET("/workspace/:slug/embed-progress",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    middleware.ValidWorkspaceSlug(db),
    h.EmbedProgress)

// Workspace search
r.POST("/workspace/search",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    h.SearchWorkspaces)

// Vector search
r.POST("/workspace/:slug/vector-search",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.VectorSearch)
```

### 4.3 Document Handler 扩展

```go
// Create folder
r.POST("/document/create-folder",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    h.CreateFolder)

// Move files
r.POST("/document/move-files",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin", "manager"}),
    h.MoveFiles)

// List all documents
r.GET("/documents",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    h.ListDocuments)

// List folder documents
r.GET("/documents/folder/:folderName",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    h.ListFolderDocuments)

// Get document by name (align with Node: /document/:docName)
r.GET("/document/:docName",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    h.GetDocumentByName)

// Accept file types (align with Node: /document/accepted-file-types)
r.GET("/document/accepted-file-types",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    h.AcceptedFileTypes)
```

**注意：** Go 已有 `/document/:docId` 和 `/document/accepted-extensions`，需要决定是否保留旧路由做兼容，还是替换为新路由。

**决策：** 保留旧路由做向后兼容（返回 301 或直接支持两种参数），同时新增 Node 对齐的路由。

---

## 5. 数据流

### 5.1 非流式聊天

```
Client → POST /workspace/:slug/chat
    → ChatHandler.Chat
        → ChatService.Complete
            → buildChatHistory (同 Stream)
            → embedder.EmbedQuery (同 Stream)
            → vectorSvc.SimilaritySearch (同 Stream)
            → llmProv.Complete (新增同步调用)
            → saveChatResponse (同 Stream)
    → JSON Response {id, type, textResponse, sources, close}
```

### 5.2 文档上传 + 嵌入 + SSE 进度

```
Client → POST /workspace/:slug/upload
    → WorkspaceHandler.UploadToWorkspace
        → DocumentService.UploadToWorkspace
            → Save file to disk
            → Create DB record
            → spawn goroutine: embedWithProgress
                → EmbeddingProgressManager.BroadcastProgress (多次)
                → vectorSvc.AddVectors
                → EmbeddingProgressManager.Broadcast(complete)
    → JSON Response {success, documents}

Client → GET /workspace/:slug/embed-progress (SSE)
    → EmbeddingProgressManager.AddConnection
    → Listen for events → SSE stream
```

### 5.3 工作区搜索

```
Client → POST /workspace/search {searchTerm}
    → WorkspaceHandler.SearchWorkspaces
        → SearchService.SearchWorkspaceAndThreads
            → DB query workspaces (with user filter)
            → DB query threads (with user filter + workspace relation)
            → Fuzzy match with levenshtein
    → JSON Response {workspaces, threads}
```

---

## 6. 错误处理

**通用模式：** 沿用现有 Go 项目的错误处理风格：
- 输入验证错误 → `400 Bad Request`
- 权限错误 → `403 Forbidden`（或 Node 对齐的 `200 + {error}`）
- 资源不存在 → `404 Not Found`
- 服务端错误 → `500 Internal Server Error`
- Node 行为优先：部分路由返回 `200 + {success: false, error: "..."}`

**SSE 错误：** 通过 SSE event 发送 `{type: "error", message: "..."}`，然后关闭连接。

---

## 7. 测试策略

| 层级 | 测试类型 | 覆盖目标 |
|---|---|---|
| Service | 单元测试 | SearchService (模糊匹配逻辑)、DocumentService (CRUD)、VectorSearchService |
| Handler | 集成测试 | 所有 P0 路由的 HTTP 请求/响应、SSE 连接管理 |
| E2E | 集成测试 | 上传 → 嵌入 → 搜索 完整流程 |

**SSE 测试：** 使用 `httptest.ResponseRecorder` + goroutine 模拟客户端断开。

---

## 8. 新增/修改文件清单

### 新增文件

```
backend/internal/services/
    embedding_progress.go        # SSE 进度管理器
    search_service.go             # 工作区/线程搜索
    vector_search_service.go      # 向量搜索服务

backend/internal/dto/
    search.go                     # SearchResults, FileMove
    chat.go (extend)              # ChatRequest, ChatResponse

backend/internal/providers/
    llm.go (extend)               # LLMProvider + Complete method

backend/internal/handlers/
    (extend existing files)

backend/tests/integration/
    p0_chat_test.go               # 非流式聊天集成测试
    p0_document_test.go           # 文档工作流集成测试
    p0_search_test.go             # 搜索集成测试
```

### 修改文件

```
backend/internal/services/
    chat_service.go               # 新增 Complete 方法
    document_service.go           # 扩展文档生命周期方法

backend/internal/handlers/
    chat.go                       # 新增 Chat, ApiChat handler
    workspace.go                  # 新增 upload/embed/search handler
    document.go                   # 新增 folder/CRUD handler

backend/internal/dto/
    chat.go                       # 扩展 DTO
    document.go                   # 扩展 DTO
    workspace.go                  # 扩展 DTO

backend/cmd/
    main.go                       # 注册新路由、注入新 service

backend/internal/providers/
    (all provider impls)          # 实现 Complete 方法
```

---

## 9. 实现顺序（方案 A）

### Step 1: 基础设施
1. `EmbeddingProgressManager` — SSE 连接管理
2. `LLMProvider.Complete` — 同步调用接口 + 所有 provider 实现

### Step 2: 服务层
3. `SearchService`
4. `VectorSearchService`
5. `DocumentService` 扩展（分批：upload → embed/unembed → folder CRUD）
6. `ChatService.Complete`

### Step 3: Handler 层
7. 工作区搜索 + 向量搜索 handler（最小，先交付）
8. 非流式聊天 handler（API + 顶层）
9. 文档 handler（folder CRUD）
10. Workspace handler（upload/embed/remove + SSE progress）

### Step 4: 测试 + 集成
11. 单元测试
12. 集成测试
13. Regression test（确保已有路由不受影响）

---

## 10. 风险评估

| 风险 | 影响 | 缓解措施 |
|---|---|---|
| `Complete` 方法在各 provider 中实现不一致 | 高 | 先实现 OpenAI provider 的 `Complete`，其他 provider 先用 Stream 包装 fallback |
| SSE 连接在大量并发时内存泄漏 | 中 | 设置连接超时、客户端断开自动清理、定期扫描死连接 |
| 文档嵌入和向量删除的并发安全 | 中 | 使用 workspace slug 级别的 mutex |
| Levenshtein 性能（大数据量） | 低 | 先全量加载再过滤，后续可优化为数据库 LIKE + 应用层 fuzzy |
| 路由路径冲突（旧 `/document/:docId` vs 新 `/document/:docName`） | 低 | 保留旧路由，新增 `/document/by-name/:docName` 或统一参数处理 |

---

*Design complete. Ready for review.*
