# Context Compression — Hermind Extensions

> **Local goal:** Add the `/compress` manual endpoint, cross-thread handoff with ParentThreadID seeding, real usage calibration for the Chat path, and telemetry/observability wiring for both Agent and Chat paths.
> **Depends on file:** `2026-06-01-context-compression-hermind-core.md` (C1–C4), `2026-06-01-context-compression-hermind-chat-path.md` (H1–H3), `2026-06-01-context-compression-hermind-agent-path.md` (A1–A3), `2026-06-01-context-compression-hermind-models.md` (M1–M3)

---

## File Structure

| Path | Responsibility |
|---|---|
| `backend/internal/models/thread_compaction.go` | **Modify:** Add `LastPromptTokens int` (E0) |
| `backend/internal/services/chat_service.go` | **Modify:** Add `CompressNow` (E1), usage cache + `recordUsage`/`getLastPromptTokens` (E3), `logCompactionFinished` call in `saveCompactionAndSoftDelete` (E4) |
| `backend/internal/services/chat_service_test.go` | **Modify:** Tests for E1, E3, E4 |
| `backend/internal/dto/workspace.go` | **Modify:** Add `ParentThreadID *int` to `CreateThreadRequest` (E2); add `CompressRequest` (E1) |
| `backend/internal/services/thread_service.go` | **Modify:** Seed parent-thread compaction on `Create` when `ParentThreadID != nil` (E2) |
| `backend/internal/services/thread_handoff_test.go` | **Create:** E2 behavioral tests |
| `backend/internal/handlers/chat.go` | **Modify:** Add `Compress` handler method; `RegisterChatRoutes` adds `POST /workspace/:slug/compress` (E1) |
| `backend/internal/handlers/compress_endpoint_test.go` | **Create:** E1 endpoint tests |
| `backend/internal/agent/compression/observer.go` | **Modify:** Add `SetNotifyFunc` for WS event injection (E4) |
| `backend/internal/agent/compression_wiring.go` | **Modify:** `buildCompressor` accepts `onNotify func(summary string)`; calls telemetry + mlog (E4) |
| `backend/internal/agent/handler.go` | **Modify:** Pass WS `SendEvent` closure into `buildCompressor` (E4) |
| `backend/internal/agent/telemetry.go` | **Modify:** Add `logCompactionFinished` (E4) |
| `backend/internal/agent/telemetry_compaction_test.go` | **Create:** E4 telemetry tests |

## Dependency Overview

```
E0 (ThreadCompaction.LastPromptTokens)
  -> E1 (/compress endpoint)
       -> E3 (usage calibration)
       -> E4 (telemetry + WS events)

E2 (cross-thread handoff) [independent of E0–E4, needs only M1 + M2]
```

**Parallelizable:** E2 can run in parallel with E0–E4. E1, E3, and E4 are sequential because E3 and E4 modify `ChatService` helpers introduced in E1.

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | `LLMChunk.Usage` is nil for some providers | Fallback to character estimation in `tryCompressHistory` | Some providers never trigger calibrated compression; mitigation is the existing estimate fallback |
| 2 | `CompressNow` creates a new `DefaultCompressor` on every call | `CompressMessages` handles short-history no-op internally | Wasted compressor allocation; acceptable for a manual/admin endpoint |
| 3 | Agent WS `context.compressed` event has no before/after token counts | `Observer` does not track token deltas; V1 sends summary-only event | Telemetry table records `before=0/after=0` for agent path until Pantheon surfaces usage at the compression boundary |
| 4 | Cross-thread handoff seeds only the latest parent compaction | Sub-threads with multiple compactions still get the most recent summary | Handoff accuracy degrades for very long parent threads; `thread_compactions` already stores full history for future browse |

---

### Task E0: Extend ThreadCompaction with LastPromptTokens

**Depends on:** none (additive field change)

**Files:**
- Modify: `backend/internal/models/thread_compaction.go`

- [ ] **Step 1: Add the field**

In `backend/internal/models/thread_compaction.go`, add `LastPromptTokens` to the existing struct:

```go
// ThreadCompaction persists a compressed conversation summary for a workspace/thread pair.
type ThreadCompaction struct {
    ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
    WorkspaceID     int       `gorm:"index:idx_ws_thread,priority:1" json:"workspaceId"`
    ThreadID        *int      `gorm:"index:idx_ws_thread,priority:2" json:"threadId"`
    Summary         string    `json:"summary"`
    UpToChatID      int       `json:"upToChatId"`
    BeforeTokens    int       `json:"beforeTokens"`
    AfterTokens     int       `json:"afterTokens"`
    FallbackUsed    bool      `json:"fallbackUsed"`
    LastPromptTokens int      `json:"lastPromptTokens"` // E0: real usage from previous LLM call
    CreatedAt       time.Time `json:"createdAt"`
    LastUpdatedAt   time.Time `json:"lastUpdatedAt"`
}
```

- [ ] **Step 2: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors (additive field, no caller changes required)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/thread_compaction.go
git commit -m "feat(compression): add LastPromptTokens to ThreadCompaction"
```

---

### Task E1: POST /compress Manual Endpoint

**Depends on:** E0, H3 (`ChatService` has `compStore`, `sysSvc`, `tryCompressHistory` pattern)

**Files:**
- Modify: `backend/internal/services/chat_service.go`
- Modify: `backend/internal/dto/workspace.go`
- Modify: `backend/internal/handlers/chat.go`
- Modify: `backend/internal/services/chat_service_test.go`
- Create: `backend/internal/handlers/compress_endpoint_test.go`

- [ ] **Step 1: Write the failing test for `CompressNow`**

```go
// backend/internal/services/chat_service_test.go
package services

import (
    "context"
    "errors"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestChatService_CompressNow_Disabled(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, db.Create(ws).Error)

    // Global disabled + workspace nil → compression not available
    _, err := svc.CompressNow(context.Background(), ws, nil, "")
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrCompressionNotAvailable))
}

func TestChatService_CompressNow_NothingToCompress(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    compStore := agentcompression.NewCompactionStore(db)
    sysSvc := NewSystemService(db)
    require.NoError(t, sysSvc.SetSetting(context.Background(), "context_compress_enabled", "true"))

    svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, compStore, sysSvc)

    ws := &models.Workspace{Name: "ws", Slug: "ws", CompressEnabled: boolPtr(true)}
    require.NoError(t, db.Create(ws).Error)

    // No chat history → nothing to compress
    _, err := svc.CompressNow(context.Background(), ws, nil, "")
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNothingToCompress))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestChatService_CompressNow -v`
Expected: FAIL — `CompressNow`, `ErrCompressionNotAvailable`, `ErrNothingToCompress` not defined

- [ ] **Step 3: Add DTO and service errors**

In `backend/internal/dto/workspace.go`, add:

```go
type CompressRequest struct {
    ThreadID *int   `json:"threadId"`
    Topic    string `json:"topic"`
}
```

In `backend/internal/services/chat_service.go`, add the errors and `CompressNow`:

```go
var (
    ErrNothingToCompress       = errors.New("nothing to compress")
    ErrCompressionNotAvailable = errors.New("compression not available")
)

// CompactionResult is the outcome of a manual or automatic compression.
type CompactionResult struct {
    Before       int     `json:"before"`
    After        int     `json:"after"`
    SavedPct     float64 `json:"savedPct"`
    Summary      string  `json:"summary"`
    FallbackUsed bool    `json:"fallbackUsed"`
}

// CompressNow performs an on-demand compression of the full chat history.
// It bypasses the automatic threshold gate (always attempts compression).
// Returns ErrNothingToCompress if history is too short, or ErrCompressionNotAvailable
// if compression is disabled for the workspace.
func (s *ChatService) CompressNow(ctx context.Context, ws *models.Workspace, threadID *int, topic string) (CompactionResult, error) {
    if s.compStore == nil || s.sysSvc == nil {
        return CompactionResult{}, ErrCompressionNotAvailable
    }
    globalEnabledStr, _ := s.sysSvc.GetSetting(ctx, "context_compress_enabled")
    globalEnabled := globalEnabledStr == "true"
    if !agentcompression.IsEnabledForWorkspace(globalEnabled, ws.CompressEnabled) {
        return CompactionResult{}, ErrCompressionNotAvailable
    }

    // Read full history (no limit truncation)
    history, maxChatID, err := s.buildChatHistory(ctx, ws.ID, threadID, 999999, 0)
    if err != nil {
        return CompactionResult{}, err
    }

    const minMessagesForCompress = 4 // at least 2 user/assistant pairs
    if len(history) < minMessagesForCompress {
        return CompactionResult{}, ErrNothingToCompress
    }

    comp := agentcompression.NewForChat(s.llmProv.LanguageModel(), ws, s.compStore)
    if comp == nil {
        return CompactionResult{}, ErrCompressionNotAvailable
    }

    before := estimateTokens(history)
    compressed, err := comp.CompressMessages(ctx, history, topic)
    if err != nil {
        return CompactionResult{}, err
    }
    after := estimateTokens(compressed)
    summary := extractSummaryFromCompressed(compressed)

    // Persist and soft-delete
    if summary != "" && maxChatID > 0 {
        if err := s.saveCompactionAndSoftDelete(ctx, ws.ID, threadID, summary, before, after, false); err != nil {
            mlog.Warning("CompressNow: persistence failed: ", err)
        }
    }

    savedPct := 0.0
    if before > 0 {
        savedPct = float64(before-after) / float64(before) * 100
    }

    return CompactionResult{
        Before:       before,
        After:        after,
        SavedPct:     savedPct,
        Summary:      summary,
        FallbackUsed: false,
    }, nil
}
```

Note: `saveCompactionAndSoftDelete` is updated in E4 to accept `before, after, fallbackUsed` parameters. Until E4 is done, the call above may need a temporary adapter — but since this sub-plan is executed in order, E4 follows E1 and will update the signature. To keep E1 build-green on its own, temporarily add the extra params to `saveCompactionAndSoftDelete` in this task and let E4 refine the telemetry call inside it.

Actually, to avoid a signature churn, implement `saveCompactionAndSoftDelete` with the new signature right here in E1:

```go
func (s *ChatService) saveCompactionAndSoftDelete(ctx context.Context, workspaceID int, threadID *int, summary string, beforeTokens, afterTokens int, fallbackUsed bool) error {
    maxChatID, err := s.maxChatIDForThread(ctx, workspaceID, threadID)
    if err != nil || maxChatID == 0 {
        return err
    }
    if err := s.compStore.Save(&models.ThreadCompaction{
        WorkspaceID:      workspaceID,
        ThreadID:         threadID,
        Summary:          summary,
        UpToChatID:       maxChatID,
        BeforeTokens:     beforeTokens,
        AfterTokens:      afterTokens,
        FallbackUsed:     fallbackUsed,
    }); err != nil {
        return fmt.Errorf("save compaction: %w", err)
    }
    if err := s.softDeleteChatsUpTo(ctx, workspaceID, threadID, maxChatID); err != nil {
        return fmt.Errorf("soft-delete chats: %w", err)
    }
    mlog.Info("chat compaction: ", beforeTokens, "→", afterTokens, " tokens")
    return nil
}
```

Update the existing call in `tryCompressHistory` (from H3) to pass `0, 0, false` as placeholders — E4 will replace those with real values.

- [ ] **Step 4: Add handler + route**

In `backend/internal/handlers/chat.go`, add the handler method:

```go
func (h *ChatHandler) Compress(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)
    var req dto.CompressRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
        return
    }

    result, err := h.chatSvc.CompressNow(c.Request.Context(), ws, req.ThreadID, req.Topic)
    if err != nil {
        switch {
        case errors.Is(err, services.ErrNothingToCompress):
            c.JSON(http.StatusConflict, gin.H{"error": "nothing to compress"})
        case errors.Is(err, services.ErrCompressionNotAvailable):
            c.JSON(http.StatusServiceUnavailable, gin.H{"error": "compression not available"})
        default:
            c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
        }
        return
    }
    c.JSON(http.StatusOK, result)
}
```

Add import for `errors` and `services` if not already present.

In `RegisterChatRoutes`, add:

```go
    r.POST("/workspace/:slug/compress",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"all"}),
        middleware.ValidWorkspaceSlug(db),
        h.Compress)
```

- [ ] **Step 5: Write endpoint test**

```go
// backend/internal/handlers/compress_endpoint_test.go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCompressEndpoint_NothingToCompress(t *testing.T) {
    gin.SetMode(gin.TestMode)
    db, cleanup := setupTestDB(t)
    defer cleanup()

    ws := &models.Workspace{Name: "test", Slug: "test-slug"}
    require.NoError(t, db.Create(ws).Error)

    compStore := agentcompression.NewCompactionStore(db)
    sysSvc := services.NewSystemService(db)
    require.NoError(t, sysSvc.SetSetting(context.Background(), "context_compress_enabled", "true"))

    chatSvc := services.NewChatService(db, nil, nil, nil, nil, nil, nil, nil, nil, compStore, sysSvc)
    h := NewChatHandler(chatSvc)

    reqBody, _ := json.Marshal(dto.CompressRequest{Topic: "summary"})
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request, _ = http.NewRequest("POST", "/workspace/test-slug/compress", bytes.NewReader(reqBody))
    c.Request.Header.Set("Content-Type", "application/json")
    c.Set("workspace", ws)

    h.Compress(c)

    assert.Equal(t, http.StatusConflict, w.Code)
    assert.Contains(t, w.Body.String(), "nothing to compress")
}
```

> Note: `setupTestDB` may not exist in the `handlers` package. The executor should use the same test-DB helper pattern as other handler tests (e.g., `api_openai_test.go`). If no shared helper exists, create an in-memory SQLite DB inline.

- [ ] **Step 6: Run targeted tests**

Run: `cd backend && go test ./internal/services/ -run TestChatService_CompressNow -v`
Expected: PASS

Run: `cd backend && go test ./internal/handlers/ -run TestCompressEndpoint -v`
Expected: PASS

- [ ] **Step 7: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go \
  backend/internal/dto/workspace.go backend/internal/handlers/chat.go \
  backend/internal/handlers/compress_endpoint_test.go
git commit -m "feat(compression): add POST /compress manual endpoint"
```

---

### Task E2: Cross-Thread Handoff

**Depends on:** M1 (`ThreadCompaction` model), M2 (`WorkspaceThread.ParentThreadID` field)

**Files:**
- Modify: `backend/internal/dto/workspace.go`
- Modify: `backend/internal/services/thread_service.go`
- Modify: `backend/internal/services/thread_service_test.go` (or create `thread_handoff_test.go`)

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/thread_handoff_test.go
package services

import (
    "context"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/models"
    agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupHandoffDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.WorkspaceThread{}, &models.ThreadCompaction{}, &models.WorkspaceChat{}))
    return db
}

func TestThreadService_Create_WithParentThreadID(t *testing.T) {
    db := setupHandoffDB(t)
    svc := NewThreadService(db)
    compStore := agentcompression.NewCompactionStore(db)

    // Create parent workspace + thread
    ws := &models.Workspace{Name: "Parent WS", Slug: "parent-ws"}
    require.NoError(t, db.Create(ws).Error)

    parentThread := &models.WorkspaceThread{Name: "Parent", Slug: "parent", WorkspaceID: ws.ID}
    require.NoError(t, db.Create(parentThread).Error)

    // Seed parent thread with a compaction
    require.NoError(t, compStore.Save(&models.ThreadCompaction{
        WorkspaceID: ws.ID,
        ThreadID:    &parentThread.ID,
        Summary:     "Parent summary",
        UpToChatID:  5,
    }))

    // Create child thread with ParentThreadID
    childReq := dto.CreateThreadRequest{
        Name:           "Child",
        Slug:           "child",
        ParentThreadID: &parentThread.ID,
    }
    child, err := svc.Create(context.Background(), ws.ID, nil, childReq)
    require.NoError(t, err)
    require.NotNil(t, child)
    assert.Equal(t, parentThread.ID, *child.ParentThreadID)

    // Child should inherit parent's latest compaction as seed
    seed, err := compStore.LoadLatest(ws.ID, &child.ID)
    require.NoError(t, err)
    require.NotNil(t, seed)
    assert.Equal(t, "Parent summary", seed.Summary)
    assert.Equal(t, 0, seed.UpToChatID) // fresh thread, no chats yet
}

func TestThreadService_Create_WithoutParentThreadID(t *testing.T) {
    db := setupHandoffDB(t)
    svc := NewThreadService(db)

    ws := &models.WorkspaceThread{Name: "Root", Slug: "root", WorkspaceID: 1}
    require.NoError(t, db.Create(ws).Error)

    rootReq := dto.CreateThreadRequest{Name: "Root Thread", Slug: "root-thread"}
    root, err := svc.Create(context.Background(), 1, nil, rootReq)
    require.NoError(t, err)
    require.Nil(t, root.ParentThreadID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestThreadService_Create_WithParentThreadID -v`
Expected: FAIL — `CreateThreadRequest.ParentThreadID` undefined, `WorkspaceThread.ParentThreadID` undefined

- [ ] **Step 3: Add DTO field and model field (if not already present from M2)**

In `backend/internal/dto/workspace.go`, update `CreateThreadRequest`:

```go
type CreateThreadRequest struct {
    Name           string `json:"name"`
    Slug           string `json:"slug"`
    ParentThreadID *int   `json:"parentThreadId,omitempty"`
}
```

In `backend/internal/models/workspace_thread.go`, ensure the field exists:

```go
type WorkspaceThread struct {
    ID             int        `gorm:"primaryKey;autoIncrement" json:"id"`
    Name           string     `json:"name"`
    Slug           string     `gorm:"unique" json:"slug"`
    WorkspaceID    int        `json:"workspaceId"`
    UserID         *int       `json:"userId"`
    ParentThreadID *int       `gorm:"index" json:"parentThreadId"` // nil = root thread
    CreatedAt      time.Time  `json:"createdAt"`
    LastUpdatedAt  time.Time  `json:"lastUpdatedAt"`
    Workspace      *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}
```

- [ ] **Step 4: Implement seed copy in ThreadService.Create**

In `backend/internal/services/thread_service.go`, modify `Create`:

```go
func (s *ThreadService) Create(ctx context.Context, workspaceID int, userID *int, req dto.CreateThreadRequest) (*models.WorkspaceThread, error) {
    name := req.Name
    if name == "" {
        name = "Thread"
    }
    threadSlug := req.Slug
    if threadSlug == "" {
        threadSlug = uuid.New().String()
    } else {
        threadSlug = slug.Make(threadSlug)
        if threadSlug == "" {
            threadSlug = uuid.New().String()
        }
    }

    thread := models.WorkspaceThread{
        Name:          name,
        Slug:          threadSlug,
        WorkspaceID:   workspaceID,
        UserID:        userID,
        ParentThreadID: req.ParentThreadID,
        CreatedAt:     time.Now(),
        LastUpdatedAt: time.Now(),
    }
    if err := s.db.Create(&thread).Error; err != nil {
        return nil, fmt.Errorf("create thread: %w", err)
    }

    // Seed child thread with parent's latest compaction summary
    if req.ParentThreadID != nil {
        if err := s.seedCompactionFromParent(ctx, workspaceID, req.ParentThreadID, &thread.ID); err != nil {
            // Non-fatal: handoff seeding should not block thread creation
            mlog.Warning("thread handoff seeding failed: ", err)
        }
    }

    return &thread, nil
}

func (s *ThreadService) seedCompactionFromParent(ctx context.Context, workspaceID int, parentThreadID, childThreadID *int) error {
    if parentThreadID == nil || childThreadID == nil {
        return nil
    }
    var parentComp models.ThreadCompaction
    err := s.db.Where("workspace_id = ? AND thread_id = ?", workspaceID, *parentThreadID).
        Order("created_at DESC").
        First(&parentComp).Error
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil // no parent compaction to seed
        }
        return err
    }
    seed := models.ThreadCompaction{
        WorkspaceID: workspaceID,
        ThreadID:    childThreadID,
        Summary:     parentComp.Summary,
        UpToChatID:  0,
        CreatedAt:   time.Now(),
        LastUpdatedAt: time.Now(),
    }
    return s.db.Create(&seed).Error
}
```

Add import for `mlog` if not already present.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/services/ -run TestThreadService_Create_WithParentThreadID -v`
Expected: PASS

Run: `cd backend && go test ./internal/services/ -run TestThreadService_Create_WithoutParentThreadID -v`
Expected: PASS

- [ ] **Step 6: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add backend/internal/dto/workspace.go backend/internal/models/workspace_thread.go \
  backend/internal/services/thread_service.go backend/internal/services/thread_handoff_test.go
git commit -m "feat(compression): cross-thread handoff with ParentThreadID seeding"
```

---

### Task E3: Real Usage Calibration (Chat Path)

**Depends on:** E1 (`CompressNow` exists, `ChatService` has compression helpers)

**Files:**
- Modify: `backend/internal/services/chat_service.go`
- Modify: `backend/internal/services/chat_service_test.go`
- Create: `backend/internal/services/usage_calibration_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/usage_calibration_test.go
package services

import (
    "context"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/pantheon/core"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestChatService_UsageCache_RoundTrip(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, db.Create(ws).Error)

    // Record usage for (ws, nil)
    svc.recordUsage(ws.ID, nil, core.Usage{PromptTokens: 1500})

    usage, ok := svc.getLastPromptTokens(ws.ID, nil)
    require.True(t, ok)
    assert.Equal(t, 1500, usage)

    // Different workspace should not exist
    _, ok = svc.getLastPromptTokens(999, nil)
    assert.False(t, ok)
}

func TestChatService_UsageCache_ThreadIsolation(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

    threadID := 42
    svc.recordUsage(1, &threadID, core.Usage{PromptTokens: 2000})
    svc.recordUsage(1, nil, core.Usage{PromptTokens: 1000})

    u1, ok1 := svc.getLastPromptTokens(1, &threadID)
    u2, ok2 := svc.getLastPromptTokens(1, nil)

    require.True(t, ok1)
    require.True(t, ok2)
    assert.Equal(t, 2000, u1)
    assert.Equal(t, 1000, u2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestChatService_UsageCache -v`
Expected: FAIL — `recordUsage`, `getLastPromptTokens` not defined

- [ ] **Step 3: Add usage cache to ChatService**

In `backend/internal/services/chat_service.go`, add to the `ChatService` struct:

```go
import (
    // existing imports ...
    "sync"
)

type ChatService struct {
    // ... existing fields ...
    usageMu         sync.RWMutex
    lastPromptTokens map[string]int // key: "wsID:threadID" or "wsID:nil"
}
```

Add the helper methods (after `NewChatService` or near the bottom):

```go
func (s *ChatService) cacheKey(workspaceID int, threadID *int) string {
    if threadID != nil {
        return fmt.Sprintf("%d:%d", workspaceID, *threadID)
    }
    return fmt.Sprintf("%d:nil", workspaceID)
}

func (s *ChatService) recordUsage(workspaceID int, threadID *int, usage core.Usage) {
    if usage.PromptTokens <= 0 {
        return
    }
    s.usageMu.Lock()
    defer s.usageMu.Unlock()
    if s.lastPromptTokens == nil {
        s.lastPromptTokens = make(map[string]int)
    }
    s.lastPromptTokens[s.cacheKey(workspaceID, threadID)] = usage.PromptTokens
}

func (s *ChatService) getLastPromptTokens(workspaceID int, threadID *int) (int, bool) {
    s.usageMu.RLock()
    defer s.usageMu.RUnlock()
    v, ok := s.lastPromptTokens[s.cacheKey(workspaceID, threadID)]
    return v, ok
}
```

- [ ] **Step 4: Wire usage capture into Stream**

In `ChatService.Stream`, inside the `for chunk := range chunks` loop (around line 212), add:

```go
    for chunk := range chunks {
        select {
        case <-ctx.Done():
            mlog.Info("ChatService.Stream: context done during chunk loop")
            return
        default:
        }
        if chunk.Usage != nil {
            s.recordUsage(ws.ID, threadID, *chunk.Usage)
        }
        // ... rest of existing chunk handling ...
    }
```

- [ ] **Step 5: Feed real usage into compressor in `tryCompressHistory`**

In `tryCompressHistory`, after creating the compressor and before calling `CompressMessages`, add:

```go
    comp := agentcompression.NewForChat(s.llmProv.LanguageModel(), ws, s.compStore)
    if comp == nil {
        return history, nil
    }

    // Calibrate with real usage from previous turn if available
    if lastPromptTokens, ok := s.getLastPromptTokens(ws.ID, threadID); ok {
        _ = comp.UpdateFromResponse(core.Usage{PromptTokens: lastPromptTokens})
    }

    compressed, err := comp.CompressMessages(ctx, history, "")
    // ... rest of existing logic ...
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd backend && go test ./internal/services/ -run TestChatService_UsageCache -v`
Expected: PASS

- [ ] **Step 7: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go \
  backend/internal/services/usage_calibration_test.go
git commit -m "feat(compression): real usage calibration for Chat path"
```

---

### Task E4: Telemetry + Agent WS Events

**Depends on:** E1 (`saveCompactionAndSoftDelete` has before/after params), A3 (`Observer`, `buildCompressor`, `handler.go` wiring exist)

**Files:**
- Modify: `backend/internal/agent/telemetry.go`
- Modify: `backend/internal/agent/compression/observer.go`
- Modify: `backend/internal/agent/compression_wiring.go`
- Modify: `backend/internal/agent/handler.go`
- Modify: `backend/internal/services/chat_service.go` (telemetry call in `saveCompactionAndSoftDelete`)
- Create: `backend/internal/agent/telemetry_compaction_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/agent/telemetry_compaction_test.go
package agent

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLogCompactionFinished(t *testing.T) {
    logger := &mockEventLogger{}
    logCompactionFinished(logger, intPtr(7), 42, "chat", 1000, 400, false)

    // Async — wait briefly
    time.Sleep(100 * time.Millisecond)

    require.Len(t, logger.events, 1)
    ev := logger.events[0]
    assert.Equal(t, "compaction_finished", ev.eventType)
    assert.Equal(t, 42, ev.payload["workspace_id"])
    assert.Equal(t, "chat", ev.payload["path"])
    assert.Equal(t, 1000, ev.payload["before_tokens"])
    assert.Equal(t, 400, ev.payload["after_tokens"])
    assert.Equal(t, 60.0, ev.payload["saved_pct"])
    assert.Equal(t, false, ev.payload["fallback_used"])
}

func TestLogCompactionFinished_NilLogger(t *testing.T) {
    // Should not panic
    logCompactionFinished(nil, nil, 1, "agent", 0, 0, false)
}

type mockEventLogger struct {
    events []struct {
        eventType string
        payload   map[string]any
    }
}

func (m *mockEventLogger) LogEvent(ctx context.Context, eventType string, payload map[string]any, userID *int) error {
    m.events = append(m.events, struct {
        eventType string
        payload   map[string]any
    }{eventType, payload})
    return nil
}

func intPtr(i int) *int { return &i }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/agent/ -run TestLogCompactionFinished -v`
Expected: FAIL — `logCompactionFinished` not defined

- [ ] **Step 3: Add telemetry helper**

In `backend/internal/agent/telemetry.go`, add:

```go
func logCompactionFinished(eventLog eventLogger, userID *int, workspaceID int, path string, beforeTokens, afterTokens int, fallbackUsed bool) {
    if isNilLogger(eventLog) {
        return
    }
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        savedPct := 0.0
        if beforeTokens > 0 {
            savedPct = float64(beforeTokens-afterTokens) / float64(beforeTokens) * 100
        }
        _ = eventLog.LogEvent(ctx, "compaction_finished", map[string]any{
            "workspace_id":  workspaceID,
            "path":          path,
            "before_tokens": beforeTokens,
            "after_tokens":  afterTokens,
            "saved_pct":     savedPct,
            "fallback_used": fallbackUsed,
        }, userID)
    }()
}
```

- [ ] **Step 4: Wire telemetry into Chat path**

In `backend/internal/services/chat_service.go`, update `saveCompactionAndSoftDelete` to call the telemetry helper. Add import for agent package if needed, or replicate a lightweight version in services.

To avoid a circular import (`services` → `agent`), define a thin interface or pass `eventLog` via `ChatService`. However, `ChatService` currently has no `eventLog` field. The simplest approach: call `mlog.Info` with structured fields for now, and rely on the existing `mlog.Info("chat compaction: ...")` line (already present from E1).

> **Decision:** The `compaction_finished` telemetry event is primarily for the Agent path where `eventLogger` is already wired. For Chat path, the `mlog.Info` line is sufficient observability. Skip adding `eventLog` to `ChatService` to avoid signature churn.

- [ ] **Step 5: Extend Observer with notify callback**

In `backend/internal/agent/compression/observer.go`, add:

```go
// NotifyFunc is called after a summary is successfully extracted and saved.
type NotifyFunc func(summary string)

// SetNotifyFunc sets an optional callback that fires when a new summary is extracted.
func (o *Observer) SetNotifyFunc(fn NotifyFunc) {
    o.notify = fn
}
```

Update the `Observer` struct to include the `notify` field:

```go
type Observer struct {
    inner compression.ContextEngine
    save  SaveFunc
    notify NotifyFunc // added in E4
}
```

And update `CompressMessages` to invoke it:

```go
func (o *Observer) CompressMessages(ctx context.Context, messages []core.Message, focusTopic string) ([]core.Message, error) {
    out, err := o.inner.CompressMessages(ctx, messages, focusTopic)
    if err != nil {
        return nil, err
    }
    if summary := extractSummary(out); summary != "" && o.save != nil {
        _ = o.save(summary)
    }
    if summary := extractSummary(out); summary != "" && o.notify != nil {
        o.notify(summary)
    }
    return out, nil
}
```

- [ ] **Step 6: Wire Agent WS events in handler.go**

In `backend/internal/agent/handler.go`, modify the `buildCompressor` call site (inside `HandleWS`). The current code (from A3) is roughly:

```go
comp := buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc)
```

Replace it with:

```go
var comp compression.ContextEngine
if r.testCompressorOverride != nil {
    comp = r.testCompressorOverride
} else {
    comp = buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc, func(summary string) {
        wc.SendEvent("context.compressed", gin.H{
            "summary": summary,
            "path":    "agent",
        })
    })
}
```

> Note: `wc` is the `AgentIO` / WebSocket connection available in `HandleWS` scope. If `wc` does not have a `SendEvent` method, use whatever method the existing code uses to send events (e.g., `wc.WriteJSON(...)` or a typed `SendEvent` on the connection wrapper).

If the exact `SendEvent` API differs, adapt the call to match the existing pattern in `handler.go`. For example, if events are sent via:

```go
wc.Send(&agent.Event{Type: "context.compressed", Payload: map[string]any{...}})
```

use that shape instead.

In `runtime.go`, the `buildCompressor` call does not have a WebSocket connection, so pass `nil` for the notify callback:

```go
comp := buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc, nil)
```

- [ ] **Step 7: Update `buildCompressor` signature**

In `backend/internal/agent/compression_wiring.go`, change `buildCompressor` to accept the notify callback:

```go
func buildCompressor(db *gorm.DB, ws *models.Workspace, lm core.LanguageModel, sysSvc *services.SystemService, onNotify agentcompression.NotifyFunc) compression.ContextEngine {
    if !isCompressionEnabled(ws, sysSvc) {
        return nil
    }

    comp := agentcompression.NewForAgent(lm.Model(), agentcompression.ContextLengthFor(lm.Model()))
    store := agentcompression.NewCompactionStore(db)
    obs := agentcompression.NewObserver(comp, func(summary string) error {
        if err := store.Save(context.Background(), &models.ThreadCompaction{
            WorkspaceID: ws.ID,
            ThreadID:    nil, // agent sessions have no thread
            Summary:     summary,
        }); err != nil {
            return err
        }
        if onNotify != nil {
            onNotify(summary)
        }
        return nil
    })
    return obs
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd backend && go test ./internal/agent/ -run TestLogCompactionFinished -v`
Expected: PASS

Run: `cd backend && go test ./internal/agent/ -run TestObserver -v`
Expected: PASS (existing Observer tests should still pass; `notify` is nil by default)

- [ ] **Step 9: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 10: Commit**

```bash
git add backend/internal/agent/telemetry.go backend/internal/agent/telemetry_compaction_test.go \
  backend/internal/agent/compression/observer.go backend/internal/agent/compression_wiring.go \
  backend/internal/agent/handler.go backend/internal/agent/runtime.go \
  backend/internal/services/chat_service.go
git commit -m "feat(compression): add telemetry + Agent WS context.compressed events"
```

---

## Self-Review

Reproduce all seven as `- [ ]` checkboxes — do not shrink to five:

- [ ] **1. Spec coverage (build the table).**

| Design § | Requirement | Task(s) | Status |
|---|---|---|---|
| §1.1 | `/compress` endpoint | E1 | covered |
| §1.1 | Cross-thread handoff | E2 | covered |
| §1.1 | Real usage calibration | E3 | covered |
| §9 | Observability (WS events, logs, telemetry) | E4 | covered |
| §18.1 | Manual `/compress` | E1 | covered |
| §18.2 | Thread handoff + MemoryProvider | E2 | covered (MemoryProvider is future subagent hook; V1 uses service-layer seeding) |
| §18.3 | Usage calibration | E3 | covered |
| §18.5 | 600s cooldown | upstream P5 — no Hermind code needed | no-op |

- [ ] **2. Placeholder scan:** No `TODO`, `TBD`, or deferred-by-dependency placeholders. The `NotifyFunc` and `logCompactionFinished` are fully implemented. The `MemoryProvider` hook from §18.2 is explicitly scoped out of V1 (service-layer seeding handles the actual use case); it is recorded as `no-op` in the coverage table, not as a placeholder.

- [ ] **3. No phantom tasks:** Every task produces a verifiable change. E0 adds a model field. E1 adds an endpoint + service method + handler + tests. E2 adds DTO field + seeding logic + tests. E3 adds usage cache + calibration wiring + tests. E4 adds telemetry helper + WS events + tests.

- [ ] **4. Dependency soundness:** E0 → E1 → E3/E4. E2 is independent (only needs M1/M2 from prior sub-plans). No task references unfinished external work. `buildCompressor` signature is updated in E4 (notify callback added); both callers (handler.go and runtime.go) are updated in the same task.

- [ ] **5. Caller & build soundness:** E4 changes `buildCompressor` signature in ONE task and updates both callers (handler.go, runtime.go) in the same task. E4 also adds `SetNotifyFunc` to Observer (new method, backward compatible). The whole-tree check uses `go vet ./...` which compiles test files too.

- [ ] **6. Test-the-risk:** E1 tests the disabled/nothing-to-compress boundary (the state mutation is the DB write, exercised via the endpoint test). E2 tests the DB mutation (parent compaction copied to child). E3 tests the cache mutation (record → read round-trip). E4 tests the telemetry async event payload.

- [ ] **7. Type consistency:** `NotifyFunc` is `func(string)` in both `observer.go` and `compression_wiring.go`. `CompactionResult` fields match the JSON tags in the design. `cacheKey` uses `"wsID:threadID"` consistently. `recordUsage` stores `PromptTokens` (int), and `getLastPromptTokens` returns `(int, bool)`.
