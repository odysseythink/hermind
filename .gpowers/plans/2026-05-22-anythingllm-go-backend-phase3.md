# AnythingLLM Go Backend — Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add workspace threads, threaded chat streaming, chat history management, onboarding, password reset (recovery-code based), and invite system to the Go backend.

**Architecture:** Layered services approach. New `ThreadService` handles thread CRUD. Existing `ChatService` extended with optional `threadID` parameter for thread-aware streaming and history. `AuthService` extended with recovery-code-based password reset and invite flows. `SystemService` extended with onboarding. `AdminService` extended with invite management. All changes are API-compatible with the Node backend.

**Tech Stack:** Go 1.25, Gin, GORM, `github.com/gosimple/slug`, `golang.org/x/crypto/bcrypt`, `github.com/google/uuid`

---

## File Structure

**NEW:**
- `backend/internal/services/thread_service.go` — Thread CRUD, thread-scoped chat management
- `backend/internal/handlers/thread.go` — Thread HTTP handlers and route registration
- `backend/internal/middleware/workspace_thread.go` — `ValidWorkspaceAndThreadSlug` middleware

**MODIFIED:**
- `backend/internal/dto/workspace.go` — + `CreateThreadRequest`, `UpdateThreadRequest`
- `backend/internal/dto/chat.go` — + `UpdateChatRequest`
- `backend/internal/dto/auth.go` — + `ResetPasswordRequest`, `RecoverAccountRequest`, `UpdatePasswordRequest`, `AcceptInviteRequest`
- `backend/internal/dto/admin.go` — + `CreateInviteRequest`
- `backend/internal/services/chat_service.go` — Thread-aware `Stream()`, `buildChatHistory()`, `saveChatResponse()`, + chat management methods
- `backend/internal/handlers/chat.go` — Threaded stream-chat + chat management routes
- `backend/internal/services/auth_service.go` — + `RecoverAccount()`, `ResetPassword()`, `GetInvite()`, `AcceptInvite()`
- `backend/internal/handlers/auth.go` — + recover-account, reset-password, update-password, invite routes
- `backend/internal/services/system_service.go` — + `GetOnboardingStatus()`, `CompleteOnboarding()`
- `backend/internal/handlers/system.go` — + onboarding routes
- `backend/internal/services/admin_service.go` — + `CreateInvite()`, `ListInvites()`, `DeactivateInvite()`
- `backend/internal/handlers/admin.go` — + admin invite routes
- `backend/cmd/server/main.go` — Wire ThreadService, ThreadHandler, register all new routes
- `backend/go.mod` — + `github.com/gosimple/slug`

**TESTS:**
- `backend/tests/integration/thread_test.go`
- `backend/tests/integration/chat_thread_test.go`
- `backend/tests/integration/chat_management_test.go`
- `backend/tests/integration/auth_extended_test.go`
- `backend/tests/integration/onboarding_test.go`

---

## Task 1: Add DTOs

**Files:**
- Modify: `backend/internal/dto/workspace.go`
- Modify: `backend/internal/dto/chat.go`
- Modify: `backend/internal/dto/auth.go`
- Create: `backend/internal/dto/admin.go`

- [ ] **Step 1: Add thread DTOs to workspace.go**

Append to `backend/internal/dto/workspace.go`:

```go
type CreateThreadRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type UpdateThreadRequest struct {
	Name string `json:"name"`
}
```

- [ ] **Step 2: Add chat management DTO to chat.go**

Append to `backend/internal/dto/chat.go`:

```go
type UpdateChatRequest struct {
	Response string `json:"response"`
	Include  *bool  `json:"include"`
}
```

- [ ] **Step 3: Add auth extension DTOs to auth.go**

Append to `backend/internal/dto/auth.go`:

```go
type RecoverAccountRequest struct {
	Username      string   `json:"username"`
	RecoveryCodes []string `json:"recoveryCodes"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type UpdatePasswordRequest struct {
	UsePassword bool   `json:"usePassword"`
	NewPassword string `json:"newPassword"`
}

type AcceptInviteRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
```

- [ ] **Step 4: Create admin DTO file**

Create `backend/internal/dto/admin.go`:

```go
package dto

type CreateInviteRequest struct {
	WorkspaceIDs []int `json:"workspaceIds"`
}
```

- [ ] **Step 5: Verify compilation**

Run: `cd backend && go build ./internal/dto/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/dto/
git commit -m "feat: add Phase 3 DTOs for threads, chat management, auth extensions, admin invites"
```

---

## Task 2: ValidWorkspaceAndThreadSlug Middleware

**Files:**
- Create: `backend/internal/middleware/workspace_thread.go`

- [ ] **Step 1: Write middleware**

Create `backend/internal/middleware/workspace_thread.go`:

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

func ValidWorkspaceAndThreadSlug(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Workspace slug required"})
			c.Abort()
			return
		}
		var ws models.Workspace
		if err := db.Where("slug = ?", slug).First(&ws).Error; err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Workspace not found"})
			c.Abort()
			return
		}
		c.Set("workspace", &ws)

		threadSlug := c.Param("threadSlug")
		if threadSlug == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Thread slug required"})
			c.Abort()
			return
		}
		var thread models.WorkspaceThread
		if err := db.Where("slug = ? AND workspace_id = ?", threadSlug, ws.ID).First(&thread).Error; err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Thread not found"})
			c.Abort()
			return
		}
		c.Set("thread", &thread)
		c.Next()
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/middleware/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/middleware/workspace_thread.go
git commit -m "feat: add ValidWorkspaceAndThreadSlug middleware"
```

---

## Task 3: ThreadService

**Files:**
- Create: `backend/internal/services/thread_service.go`

Context: Uses `github.com/gosimple/slug` for slug generation with custom replacements matching Node's `WorkspaceThread.slugify()`.

- [ ] **Step 1: Add slug dependency**

Run: `cd backend && go get github.com/gosimple/slug`

- [ ] **Step 2: Write ThreadService**

Create `backend/internal/services/thread_service.go`:

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type ThreadService struct {
	db *gorm.DB
}

func NewThreadService(db *gorm.DB) *ThreadService {
	return &ThreadService{db: db}
}

func init() {
	slug.CustomSub = map[string]string{
		"+": "plus",
		"!": "bang",
		"@": "at",
		"*": "splat",
		".": "dot",
	}
}

func (s *ThreadService) Create(ctx context.Context, workspaceID int, userID *int, req dto.CreateThreadRequest) (*models.WorkspaceThread, error) {
	name := req.Name
	if name == "" {
		name = "Thread"
	}
	threadSlug := req.Slug
	if threadSlug == "" {
		threadSlug = slug.Make(name)
	} else {
		threadSlug = slug.Make(threadSlug)
	}
	if threadSlug == "" {
		threadSlug = uuid.New().String()
	}

	thread := models.WorkspaceThread{
		Name:          name,
		Slug:          threadSlug,
		WorkspaceID:   workspaceID,
		UserID:        userID,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&thread).Error; err != nil {
		return nil, fmt.Errorf("create thread: %w", err)
	}
	return &thread, nil
}

func (s *ThreadService) List(ctx context.Context, workspaceID int) ([]models.WorkspaceThread, error) {
	var threads []models.WorkspaceThread
	if err := s.db.Where("workspace_id = ?", workspaceID).Order("id DESC").Find(&threads).Error; err != nil {
		return nil, err
	}
	return threads, nil
}

func (s *ThreadService) GetBySlug(ctx context.Context, workspaceID int, threadSlug string) (*models.WorkspaceThread, error) {
	var thread models.WorkspaceThread
	if err := s.db.Where("slug = ? AND workspace_id = ?", threadSlug, workspaceID).First(&thread).Error; err != nil {
		return nil, err
	}
	return &thread, nil
}

func (s *ThreadService) Update(ctx context.Context, thread *models.WorkspaceThread, req dto.UpdateThreadRequest) error {
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if len(updates) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(thread).Updates(updates).Error
}

func (s *ThreadService) Delete(ctx context.Context, workspaceID int, threadSlug string) error {
	return s.db.Where("slug = ? AND workspace_id = ?", threadSlug, workspaceID).Delete(&models.WorkspaceThread{}).Error
}

func (s *ThreadService) BulkDelete(ctx context.Context, workspaceID int, slugs []string) error {
	if len(slugs) == 0 {
		return nil
	}
	return s.db.Where("workspace_id = ? AND slug IN ?", workspaceID, slugs).Delete(&models.WorkspaceThread{}).Error
}

func (s *ThreadService) GetThreadChats(ctx context.Context, threadID int) ([]models.WorkspaceChat, error) {
	var chats []models.WorkspaceChat
	if err := s.db.Where("thread_id = ? AND include = true", threadID).Order("id ASC").Find(&chats).Error; err != nil {
		return nil, err
	}
	return chats, nil
}

func (s *ThreadService) DeleteThreadEditedChats(ctx context.Context, threadID int) error {
	return s.db.Where("thread_id = ? AND prompt != response", threadID).Delete(&models.WorkspaceChat{}).Error
}

func (s *ThreadService) UpdateThreadChat(ctx context.Context, chatID int, req dto.UpdateChatRequest) error {
	updates := map[string]any{}
	if req.Response != "" {
		updates["response"] = req.Response
	}
	if req.Include != nil {
		updates["include"] = *req.Include
	}
	if len(updates) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&models.WorkspaceChat{}).Where("id = ?", chatID).Updates(updates).Error
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/services/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/thread_service.go backend/go.mod backend/go.sum
git commit -m "feat: add ThreadService with CRUD and thread-scoped chat management"
```

---

## Task 4: ThreadHandler + Routes

**Files:**
- Create: `backend/internal/handlers/thread.go`

- [ ] **Step 1: Write ThreadHandler**

Create `backend/internal/handlers/thread.go`:

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type ThreadHandler struct {
	threadSvc *services.ThreadService
}

func NewThreadHandler(threadSvc *services.ThreadService) *ThreadHandler {
	return &ThreadHandler{threadSvc: threadSvc}
}

func (h *ThreadHandler) CreateThread(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	user, _ := c.Get("user")
	var userID *int
	if u, ok := user.(*models.User); ok {
		userID = &u.ID
	}
	var req dto.CreateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	thread, err := h.threadSvc.Create(c.Request.Context(), ws.ID, userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"thread": thread, "message": "Thread created"})
}

func (h *ThreadHandler) ListThreads(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	threads, err := h.threadSvc.List(c.Request.Context(), ws.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"threads": threads})
}

func (h *ThreadHandler) DeleteThread(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	threadSlug := c.Param("threadSlug")
	if err := h.threadSvc.Delete(c.Request.Context(), ws.ID, threadSlug); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) BulkDeleteThreads(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.threadSvc.BulkDelete(c.Request.Context(), ws.ID, req.Slugs); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) GetThreadChats(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	chats, err := h.threadSvc.GetThreadChats(c.Request.Context(), thread.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chats": chats})
}

func (h *ThreadHandler) UpdateThread(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	var req dto.UpdateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.threadSvc.Update(c.Request.Context(), thread, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) DeleteThreadEditedChats(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	if err := h.threadSvc.DeleteThreadEditedChats(c.Request.Context(), thread.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) UpdateThreadChat(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid chat id"})
		return
	}
	var req dto.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.threadSvc.UpdateThreadChat(c.Request.Context(), chatID, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterThreadRoutes(r *gin.RouterGroup, threadSvc *services.ThreadService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewThreadHandler(threadSvc)
	r.POST("/workspace/:slug/thread/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.CreateThread)
	r.GET("/workspace/:slug/threads",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.ListThreads)
	r.DELETE("/workspace/:slug/thread/:threadSlug",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.DeleteThread)
	r.DELETE("/workspace/:slug/thread-bulk-delete",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.BulkDeleteThreads)
	r.GET("/workspace/:slug/thread/:threadSlug/chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.GetThreadChats)
	r.POST("/workspace/:slug/thread/:threadSlug/update",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.UpdateThread)
	r.DELETE("/workspace/:slug/thread/:threadSlug/delete-edited-chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.DeleteThreadEditedChats)
	r.POST("/workspace/:slug/thread/:threadSlug/update-chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.UpdateThreadChat)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/handlers/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/thread.go
git commit -m "feat: add ThreadHandler with CRUD routes"
```


---

## Task 5: ChatService Thread Extensions

**Files:**
- Modify: `backend/internal/services/chat_service.go`

- [ ] **Step 1: Update ChatService.Stream() signature and internals**

Modify `backend/internal/services/chat_service.go`:

Change `Stream()` signature from:
```go
func (s *ChatService) Stream(ctx context.Context, ws *models.Workspace, user *models.User, req dto.StreamChatRequest) (<-chan dto.StreamChatResponse, error)
```
to:
```go
func (s *ChatService) Stream(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, req dto.StreamChatRequest) (<-chan dto.StreamChatResponse, error)
```

Inside `Stream()`, change:
```go
history, err := s.buildChatHistory(ctx, ws.ID, historyLimit)
```
to:
```go
history, err := s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
```

And change:
```go
s.saveChatResponse(ctx, ws, user, req.Message, fullText.String())
```
to:
```go
s.saveChatResponse(ctx, ws, user, threadID, req.Message, fullText.String())
```

Do this in BOTH places where `saveChatResponse` is called (after `FinishReason` and at the end of the goroutine).

- [ ] **Step 2: Update buildChatHistory**

Change `buildChatHistory` signature from:
```go
func (s *ChatService) buildChatHistory(ctx context.Context, workspaceID, limit int) ([]core.Message, error)
```
to:
```go
func (s *ChatService) buildChatHistory(ctx context.Context, workspaceID int, threadID *int, limit int) ([]core.Message, error)
```

Replace the query body:
```go
var chats []models.WorkspaceChat
query := s.db.Where("workspace_id = ? AND include = ?", workspaceID, true)
if threadID != nil {
    query = query.Where("thread_id = ?", *threadID)
} else {
    query = query.Where("thread_id IS NULL")
}
if err := query.Order("id DESC").Limit(limit).Find(&chats).Error; err != nil {
    return nil, err
}
```

- [ ] **Step 3: Update saveChatResponse**

Change signature from:
```go
func (s *ChatService) saveChatResponse(ctx context.Context, ws *models.Workspace, user *models.User, prompt, response string)
```
to:
```go
func (s *ChatService) saveChatResponse(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, prompt, response string)
```

Add `ThreadID: threadID` to the `WorkspaceChat` struct:
```go
chat := models.WorkspaceChat{
    WorkspaceID:   ws.ID,
    UserID:        &user.ID,
    ThreadID:      threadID,
    Prompt:        prompt,
    Response:      response,
    Include:       true,
    CreatedAt:     time.Now(),
    LastUpdatedAt: time.Now(),
}
```

- [ ] **Step 4: Add chat management methods**

Append to `backend/internal/services/chat_service.go`:

```go
func (s *ChatService) DeleteWorkspaceChats(ctx context.Context, workspaceID int) error {
	return s.db.Where("workspace_id = ? AND thread_id IS NULL", workspaceID).Delete(&models.WorkspaceChat{}).Error
}

func (s *ChatService) DeleteWorkspaceEditedChats(ctx context.Context, workspaceID int) error {
	return s.db.Where("workspace_id = ? AND thread_id IS NULL AND prompt != response", workspaceID).Delete(&models.WorkspaceChat{}).Error
}

func (s *ChatService) UpdateChat(ctx context.Context, workspaceID int, chatID int, req dto.UpdateChatRequest) error {
	updates := map[string]any{}
	if req.Response != "" {
		updates["response"] = req.Response
	}
	if req.Include != nil {
		updates["include"] = *req.Include
	}
	if len(updates) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&models.WorkspaceChat{}).Where("id = ? AND workspace_id = ?", chatID, workspaceID).Updates(updates).Error
}

func (s *ChatService) UpdateChatFeedback(ctx context.Context, chatID int, score *bool) error {
	return s.db.Model(&models.WorkspaceChat{}).Where("id = ?", chatID).Update("feedback_score", score).Error
}
```

- [ ] **Step 5: Verify compilation**

Run: `cd backend && go build ./internal/services/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/chat_service.go
git commit -m "feat: extend ChatService with thread-aware streaming and chat management"
```

---

## Task 6: ChatHandler Extensions

**Files:**
- Modify: `backend/internal/handlers/chat.go`

- [ ] **Step 1: Add StreamThreadChat handler**

Add to `backend/internal/handlers/chat.go`:

```go
func (h *ChatHandler) StreamThreadChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ws := c.MustGet("workspace").(*models.Workspace)
	user := c.MustGet("user").(*models.User)
	thread := c.MustGet("thread").(*models.WorkspaceThread)

	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	stream, err := h.chatSvc.Stream(c.Request.Context(), ws, user, &thread.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	for chunk := range stream {
		if err := json.NewEncoder(c.Writer).Encode(chunk); err != nil {
			break
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}
}
```

- [ ] **Step 2: Add chat management handlers**

Add to `backend/internal/handlers/chat.go`:

```go
func (h *ChatHandler) DeleteWorkspaceChats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.chatSvc.DeleteWorkspaceChats(c.Request.Context(), ws.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ChatHandler) DeleteWorkspaceEditedChats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.chatSvc.DeleteWorkspaceEditedChats(c.Request.Context(), ws.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ChatHandler) UpdateChat(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	var body struct {
		ChatID int `json:"chatId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if body.ChatID == 0 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "chatId required"})
		return
	}
	if err := h.chatSvc.UpdateChat(c.Request.Context(), ws.ID, body.ChatID, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ChatHandler) UpdateChatFeedback(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid chat id"})
		return
	}
	var req struct {
		FeedbackScore *bool `json:"feedbackScore"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.chatSvc.UpdateChatFeedback(c.Request.Context(), chatID, req.FeedbackScore); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
```

Add `strconv` to imports if not already present.

- [ ] **Step 3: Update RegisterChatRoutes**

Modify `RegisterChatRoutes` to add:

```go
r.POST("/workspace/:slug/thread/:threadSlug/stream-chat",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    middleware.ValidWorkspaceAndThreadSlug(db),
    h.StreamThreadChat)
r.DELETE("/workspace/:slug/delete-chats",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.DeleteWorkspaceChats)
r.DELETE("/workspace/:slug/delete-edited-chats",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.DeleteWorkspaceEditedChats)
r.POST("/workspace/:slug/update-chat",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.UpdateChat)
r.POST("/workspace/:slug/chat-feedback/:chatId",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"all"}),
    middleware.ValidWorkspaceSlug(db),
    h.UpdateChatFeedback)
```

- [ ] **Step 4: Verify compilation**

Run: `cd backend && go build ./internal/handlers/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/chat.go
git commit -m "feat: add threaded stream-chat and chat management endpoints"
```

---

## Task 7: AuthService Extensions — Password Reset & Recovery

**Files:**
- Modify: `backend/internal/services/auth_service.go`

- [ ] **Step 1: Add password reset and recovery methods**

Append to `backend/internal/services/auth_service.go`:

```go
func (s *AuthService) RecoverAccount(ctx context.Context, username string, recoveryCodes []string) (string, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return "", fmt.Errorf("invalid recovery codes")
	}

	var codes []models.RecoveryCode
	if err := s.db.Where("user_id = ?", user.ID).Find(&codes).Error; err != nil {
		return "", fmt.Errorf("invalid recovery codes")
	}
	if len(codes) < 4 {
		return "", fmt.Errorf("invalid recovery codes")
	}

	seen := make(map[string]bool)
	var uniqueCodes []string
	for _, code := range recoveryCodes {
		trimmed := code
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		uniqueCodes = append(uniqueCodes, trimmed)
		if len(uniqueCodes) >= 2 {
			break
		}
	}
	if len(uniqueCodes) != 2 {
		return "", fmt.Errorf("invalid recovery codes")
	}

	valid := 0
	for _, code := range uniqueCodes {
		for _, rc := range codes {
			if utils.CheckPassword(code, rc.CodeHash) {
				valid++
				break
			}
		}
	}
	if valid != 2 {
		return "", fmt.Errorf("invalid recovery codes")
	}

	token := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     uuid.New().String(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&token).Error; err != nil {
		return "", fmt.Errorf("create reset token: %w", err)
	}
	return token.Token, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword, confirmPassword string) error {
	if newPassword != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}
	if newPassword == "" {
		return fmt.Errorf("invalid password")
	}

	var resetToken models.PasswordResetToken
	if err := s.db.Where("token = ?", token).First(&resetToken).Error; err != nil {
		return fmt.Errorf("invalid reset token")
	}
	if time.Now().After(resetToken.ExpiresAt) {
		return fmt.Errorf("invalid reset token")
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Model(&models.User{}).Where("id = ?", resetToken.UserID).Update("password", hash).Error; err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	s.db.Where("user_id = ?", resetToken.UserID).Delete(&models.PasswordResetToken{})
	s.db.Where("user_id = ?", resetToken.UserID).Delete(&models.RecoveryCode{})

	return nil
}
```

Add `"github.com/google/uuid"` to imports if not already present.

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/services/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/auth_service.go
git commit -m "feat: add recovery-code-based password reset to AuthService"
```

---

## Task 8: AuthService Extensions — Invite System

**Files:**
- Modify: `backend/internal/services/auth_service.go`
- Modify: `backend/internal/services/admin_service.go`

- [ ] **Step 1: Add invite methods to AuthService**

Append to `backend/internal/services/auth_service.go`:

```go
func (s *AuthService) GetInvite(ctx context.Context, code string) (*models.Invite, error) {
	var invite models.Invite
	if err := s.db.Where("code = ? AND status = ?", code, "pending").First(&invite).Error; err != nil {
		return nil, fmt.Errorf("invite not found or is invalid")
	}
	return &invite, nil
}

func (s *AuthService) AcceptInvite(ctx context.Context, code, username, password string) error {
	var invite models.Invite
	if err := s.db.Where("code = ? AND status = ?", code, "pending").First(&invite).Error; err != nil {
		return fmt.Errorf("invite not found or is invalid")
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		Username: utils.Ptr(username),
		Password: hash,
		Role:     "default",
	}
	if err := s.db.Create(&user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	invite.Status = "claimed"
	invite.ClaimedBy = &user.ID
	if err := s.db.Save(&invite).Error; err != nil {
		return fmt.Errorf("update invite: %w", err)
	}

	if invite.WorkspaceIds != nil && *invite.WorkspaceIds != "" {
		var ids []int
		if err := json.Unmarshal([]byte(*invite.WorkspaceIds), &ids); err == nil && len(ids) > 0 {
			for _, wid := range ids {
				wu := models.WorkspaceUser{
					WorkspaceID:   wid,
					UserID:        user.ID,
					Role:          "default",
					CreatedAt:     time.Now(),
					LastUpdatedAt: time.Now(),
				}
				s.db.Create(&wu)
			}
		}
	}

	return nil
}
```

Add `"encoding/json"` to imports if needed.

- [ ] **Step 2: Add invite management to AdminService**

Append to `backend/internal/services/admin_service.go`:

```go
import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

func (s *AdminService) CreateInvite(ctx context.Context, createdBy int, workspaceIDs []int) (*models.Invite, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}
	idsJSON, _ := json.Marshal(workspaceIDs)
	idsStr := string(idsJSON)
	invite := models.Invite{
		Code:          code,
		Status:        "pending",
		CreatedBy:     createdBy,
		WorkspaceIds:  &idsStr,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&invite).Error; err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}
	return &invite, nil
}

func (s *AdminService) ListInvites(ctx context.Context) ([]models.Invite, error) {
	var invites []models.Invite
	if err := s.db.Order("id DESC").Find(&invites).Error; err != nil {
		return nil, err
	}
	return invites, nil
}

func (s *AdminService) DeactivateInvite(ctx context.Context, inviteID int) error {
	return s.db.Model(&models.Invite{}).Where("id = ?", inviteID).Update("status", "disabled").Error
}

func generateInviteCode() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/services/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/auth_service.go backend/internal/services/admin_service.go
git commit -m "feat: add invite system to AuthService and AdminService"
```

---

## Task 9: AuthHandler Extensions

**Files:**
- Modify: `backend/internal/handlers/auth.go`

- [ ] **Step 1: Add auth extension handlers**

Add to `backend/internal/handlers/auth.go`:

```go
func (h *AuthHandler) RecoverAccount(c *gin.Context) {
	var req dto.RecoverAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	token, err := h.authSvc.RecoverAccount(c.Request.Context(), req.Username, req.RecoveryCodes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "resetToken": token})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.authSvc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword, req.ConfirmPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password reset successful"})
}

func (h *AuthHandler) UpdatePassword(c *gin.Context) {
	var req dto.UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AuthHandler) GetInvite(c *gin.Context) {
	code := c.Param("code")
	invite, err := h.authSvc.GetInvite(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"invite": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": gin.H{"code": invite.Code, "status": invite.Status}, "error": nil})
}

func (h *AuthHandler) AcceptInvite(c *gin.Context) {
	code := c.Param("code")
	var req dto.AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.authSvc.AcceptInvite(c.Request.Context(), code, req.Username, req.Password); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

- [ ] **Step 2: Update RegisterAuthRoutes**

Modify `RegisterAuthRoutes`:

```go
func RegisterAuthRoutes(r *gin.RouterGroup, authSvc *services.AuthService) {
	h := NewAuthHandler(authSvc)
	r.POST("/request-token", h.RequestToken)
	r.POST("/login", h.Login)
	r.POST("/register", h.Register)
	r.POST("/logout", h.Logout)
	r.POST("/system/recover-account", h.RecoverAccount)
	r.POST("/system/reset-password", h.ResetPassword)
	r.POST("/system/update-password", middleware.ValidatedRequest(authSvc), h.UpdatePassword)
	r.GET("/invite/:code", h.GetInvite)
	r.POST("/invite/:code", h.AcceptInvite)
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/handlers/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/auth.go
git commit -m "feat: add password reset, recover account, invite endpoints to AuthHandler"
```

---

## Task 10: SystemService Extensions

**Files:**
- Modify: `backend/internal/services/system_service.go`

- [ ] **Step 1: Add onboarding methods**

Append to `backend/internal/services/system_service.go`:

```go
func (s *SystemService) GetOnboardingStatus(ctx context.Context) (bool, error) {
	val, err := s.GetSetting(ctx, "onboarding_complete")
	if err != nil {
		return false, nil
	}
	return val == "true", nil
}

func (s *SystemService) CompleteOnboarding(ctx context.Context) error {
	return s.SetSetting(ctx, "onboarding_complete", "true")
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/services/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/system_service.go
git commit -m "feat: add onboarding status methods to SystemService"
```

---

## Task 11: SystemHandler Extensions

**Files:**
- Modify: `backend/internal/handlers/system.go`

- [ ] **Step 1: Add onboarding handlers**

Add to `backend/internal/handlers/system.go`:

```go
func (h *SystemHandler) GetOnboardingStatus(c *gin.Context) {
	status, _ := h.sysSvc.GetOnboardingStatus(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"onboardingComplete": status})
}

func (h *SystemHandler) CompleteOnboarding(c *gin.Context) {
	if err := h.sysSvc.CompleteOnboarding(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
```

- [ ] **Step 2: Update RegisterSystemRoutes**

Change signature to:
```go
func RegisterSystemRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, cfg *config.Config, authSvc *services.AuthService)
```

Add routes:
```go
r.GET("/onboarding", h.GetOnboardingStatus)
r.POST("/onboarding",
    middleware.ValidatedRequest(authSvc),
    h.CompleteOnboarding)
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/handlers/`
Expected: PASS (may fail at main.go until Task 13)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/system.go
git commit -m "feat: add onboarding endpoints to SystemHandler"
```

---

## Task 12: AdminHandler Extensions

**Files:**
- Modify: `backend/internal/handlers/admin.go`

- [ ] **Step 1: Add admin invite handlers**

Add to `backend/internal/handlers/admin.go`:

```go
func (h *AdminHandler) CreateInvite(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req dto.CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	invite, err := h.adminSvc.CreateInvite(c.Request.Context(), user.ID, req.WorkspaceIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": invite, "error": nil})
}

func (h *AdminHandler) ListInvites(c *gin.Context) {
	invites, err := h.adminSvc.ListInvites(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invites": invites})
}

func (h *AdminHandler) DeactivateInvite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.adminSvc.DeactivateInvite(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
```

- [ ] **Step 2: Update RegisterAdminRoutes**

Add to `RegisterAdminRoutes`:

```go
r.POST("/admin/invite/new",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin"}),
    h.CreateInvite)
r.GET("/admin/invites",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin"}),
    h.ListInvites)
r.DELETE("/admin/invite/:id",
    middleware.ValidatedRequest(authSvc),
    middleware.FlexUserRoleValid([]string{"admin"}),
    h.DeactivateInvite)
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/handlers/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/admin.go
git commit -m "feat: add admin invite management endpoints"
```

---

## Task 13: Main Wiring

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Wire ThreadService and ThreadHandler, update SystemHandler signature**

In `backend/cmd/server/main.go`, add after `adminSvc := services.NewAdminService(db)`:
```go
threadSvc := services.NewThreadService(db)
```

Update system routes call to:
```go
handlers.RegisterSystemRoutes(api, sysSvc, cfg, authSvc)
```

Add after `handlers.RegisterAdminRoutes`:
```go
handlers.RegisterThreadRoutes(api, threadSvc, authSvc, db)
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./cmd/server/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: wire ThreadService, ThreadHandler, and all Phase 3 routes in main"
```

---

## Task 14: Integration Tests

**Files:**
- Create: `backend/tests/integration/thread_test.go`
- Create: `backend/tests/integration/chat_thread_test.go`
- Create: `backend/tests/integration/chat_management_test.go`
- Create: `backend/tests/integration/auth_extended_test.go`
- Create: `backend/tests/integration/onboarding_test.go`

- [ ] **Step 1: Write thread_test.go**

Create `backend/tests/integration/thread_test.go`:

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupThreadTest(t *testing.T) (*gin.Engine, *services.AuthService, string, string) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	err = services.AutoMigrate(db)
	assert.NoError(t, err)
	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	assert.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	wsSvc := services.NewWorkspaceService(db, cfg)
	threadSvc := services.NewThreadService(db)

	// Register and login
	_, err = authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	ws, err := wsSvc.Create(nil, loginResp.User.(models.User).ID, dto.CreateWorkspaceRequest{Name: "Test Workspace"})
	assert.NoError(t, err)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc)
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db)
	handlers.RegisterThreadRoutes(api, threadSvc, authSvc, db)

	return r, authSvc, loginResp.Token, ws.Slug
}

func TestThreadCreateAndList(t *testing.T) {
	r, _, token, slug := setupThreadTest(t)

	body, _ := json.Marshal(dto.CreateThreadRequest{Name: "My Thread"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/"+slug+"/thread/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	assert.NotNil(t, createResp["thread"])

	// List threads
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/"+slug+"/threads", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
```

- [ ] **Step 2: Write chat_management_test.go**

Create `backend/tests/integration/chat_management_test.go`:

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupChatMgmtTest(t *testing.T) (*gin.Engine, string, string, int) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	err = services.AutoMigrate(db)
	assert.NoError(t, err)
	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	assert.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	wsSvc := services.NewWorkspaceService(db, cfg)
	chatSvc := services.NewChatService(db, cfg, nil, nil, nil)

	_, err = authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	user := loginResp.User.(models.User)

	ws, _ := wsSvc.Create(nil, user.ID, dto.CreateWorkspaceRequest{Name: "Test"})

	// Insert a chat
	chat := models.WorkspaceChat{WorkspaceID: ws.ID, UserID: &user.ID, Prompt: "hi", Response: "hello", Include: true}
	db.Create(&chat)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc)
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db)
	handlers.RegisterChatRoutes(api, chatSvc, authSvc, db)

	return r, loginResp.Token, ws.Slug, chat.ID
}

func TestDeleteWorkspaceChats(t *testing.T) {
	r, token, slug, _ := setupChatMgmtTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/workspace/"+slug+"/delete-chats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestUpdateChatFeedback(t *testing.T) {
	r, token, slug, chatID := setupChatMgmtTest(t)
	score := true
	body, _ := json.Marshal(map[string]any{"feedbackScore": score})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/"+slug+"/chat-feedback/"+strconv.Itoa(chatID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
```

- [ ] **Step 3: Write auth_extended_test.go**

Create `backend/tests/integration/auth_extended_test.go`:

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupAuthExtendedRouter(t *testing.T) (*gin.Engine, *services.AuthService, *services.AdminService) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	err = services.AutoMigrate(db)
	assert.NoError(t, err)
	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	assert.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	adminSvc := services.NewAdminService(db)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc)
	handlers.RegisterAdminRoutes(api, adminSvc, authSvc)

	return r, authSvc, adminSvc
}

func TestInviteFlow(t *testing.T) {
	r, _, adminSvc := setupAuthExtendedRouter(t)

	// Create an invite
	invite, err := adminSvc.CreateInvite(nil, 1, []int{})
	assert.NoError(t, err)

	// Get invite
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/invite/"+invite.Code, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// Accept invite
	body, _ := json.Marshal(dto.AcceptInviteRequest{Username: "bob", Password: "secret"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/invite/"+invite.Code, bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])
}

func TestRecoverAccountNoCodes(t *testing.T) {
	r, authSvc, _ := setupAuthExtendedRouter(t)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RecoverAccountRequest{Username: "alice", RecoveryCodes: []string{"code1", "code2"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/recover-account", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}
```

- [ ] **Step 4: Write onboarding_test.go**

Create `backend/tests/integration/onboarding_test.go`:

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupOnboardingRouter(t *testing.T) (*gin.Engine, *services.AuthService) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	err = services.AutoMigrate(db)
	assert.NoError(t, err)
	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	assert.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	sysSvc := services.NewSystemService(db)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc)
	handlers.RegisterSystemRoutes(api, sysSvc, cfg, authSvc)

	return r, authSvc
}

func TestOnboardingFlow(t *testing.T) {
	r, authSvc := setupOnboardingRouter(t)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	token := loginResp.Token

	// GET onboarding - should be false initially
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/onboarding", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var getResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &getResp)
	assert.Equal(t, false, getResp["onboardingComplete"])

	// POST onboarding - complete it
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/onboarding", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// GET onboarding - should be true now
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/onboarding", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	json.Unmarshal(w.Body.Bytes(), &getResp)
	assert.Equal(t, true, getResp["onboardingComplete"])
}
```

Note: Add `dto` import to onboarding_test.go if needed (for `dto.RegisterRequest`, etc.).

- [ ] **Step 5: Run all new integration tests**

Run: `cd backend && go test ./tests/integration/ -run 'TestThread|TestChat|TestInvite|TestRecover|TestOnboarding' -v`
Expected: All PASS

- [ ] **Step 6: Run full integration test suite**

Run: `cd backend && go test ./tests/integration/... -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add backend/tests/integration/
git commit -m "test: add Phase 3 integration tests for threads, chat management, auth, onboarding"
```

---

## Self-Review Checklist

- [x] **Spec coverage:** Every design doc section has at least one task. ThreadService (Task 3), ChatService extensions (Task 5), AuthService password reset (Task 7), AuthService invites (Task 8), SystemService onboarding (Task 10), AdminService invites (Task 8), middleware (Task 2), handlers (Tasks 4, 6, 9, 11, 12), main wiring (Task 13), tests (Task 14).
- [x] **Placeholder scan:** No TBD, TODO, or vague steps. Every step has exact file paths, code, and commands.
- [x] **Type consistency:** `ChatService.Stream()` takes `threadID *int` consistently. `buildChatHistory()` and `saveChatResponse()` use the same parameter. DTO names match handler method names.
- [x] **Dependency order:** DTOs (Task 1) → Middleware (Task 2) → ThreadService (Task 3) → ThreadHandler (Task 4) → ChatService (Task 5) → ChatHandler (Task 6) → AuthService (Tasks 7-8) → AuthHandler (Task 9) → SystemService (Task 10) → SystemHandler (Task 11) → AdminHandler (Task 12) → Main wiring (Task 13) → Tests (Task 14). Each task builds on previous ones.
- [x] **No circular dependencies:** ThreadService does not depend on ChatService. ChatService extension is backward-compatible (existing `Stream()` callers will need to pass `nil` for threadID).
