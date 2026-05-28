# Hermind Go Backend — Phase 3 Design Document

> **Date:** 2026-05-22  
> **Topic:** Chat-focused core completion — workspace threads, threaded chat streaming, chat management, onboarding, password reset, invite system  
> **Status:** Approved  
> **Depends on:** Phase 2 (Collector, Embedding, Vector DB, LLM streaming)

---

## 1. Overview

Phase 3 completes the user-facing chat experience by adding workspace threads, chat history management, onboarding, password reset, and invite flows. All endpoints are API-compatible with the Node.js backend.

### Deliverables

- **2 new files:** `internal/services/thread_service.go`, `internal/handlers/thread.go`
- **8 extended files:** `chat_service.go`, `chat.go`, `auth_service.go`, `auth.go`, `system_service.go`, `system.go`, `admin_service.go`, `admin.go`
- **DTO additions:** `dto/workspace.go`, `dto/chat.go`, `dto/auth.go`
- **12 new endpoints** (see Section 6)
- **5 test files** covering threads, threaded chat, chat management, auth extensions, onboarding

---

## 2. Architecture

### 2.1 Service Map

| Feature | Service | Handler | Status |
|---------|---------|---------|--------|
| Workspace Threads | `ThreadService` (new) | `ThreadHandler` (new) | New |
| Threaded Stream Chat | `ChatService` (extended) | `ChatHandler` (extended) | Extended |
| Chat Management | `ChatService` (extended) | `ChatHandler` (extended) | Extended |
| Onboarding | `SystemService` (extended) | `SystemHandler` (extended) | Extended |
| Password Reset | `AuthService` (extended) | `AuthHandler` (extended) | Extended |
| Invite System | `AuthService` (extended) | `AuthHandler` (extended) | Extended |
| Admin Invites | `AdminService` (extended) | `AdminHandler` (extended) | Extended |

### 2.2 Design Principles

- **Layered services:** Each service has a single responsibility. Threads are separate from chat streaming.
- **Thread-aware streaming:** The existing `ChatService.Stream()` is extended with an optional `threadID *int` parameter rather than duplicated.
- **Model reuse:** No new models needed. `WorkspaceThread`, `WorkspaceChat`, `Invite`, `PasswordResetToken` already exist.
- **Middleware reuse:** Existing `ValidatedRequest`, `FlexUserRoleValid`, `ValidWorkspaceSlug` are used. One new middleware: `ValidWorkspaceAndThreadSlug`.

---

## 3. Thread Service Design

### 3.1 Interface

```go
type ThreadService struct {
    db *gorm.DB
}

func NewThreadService(db *gorm.DB) *ThreadService

func (s *ThreadService) Create(ctx context.Context, workspaceID int, userID *int, req dto.CreateThreadRequest) (*models.WorkspaceThread, error)
func (s *ThreadService) List(ctx context.Context, workspaceID int) ([]models.WorkspaceThread, error)
func (s *ThreadService) GetBySlug(ctx context.Context, workspaceID int, slug string) (*models.WorkspaceThread, error)
func (s *ThreadService) Update(ctx context.Context, thread *models.WorkspaceThread, req dto.UpdateThreadRequest) error
func (s *ThreadService) Delete(ctx context.Context, workspaceID int, slug string) error
func (s *ThreadService) BulkDelete(ctx context.Context, workspaceID int, slugs []string) error
func (s *ThreadService) GetThreadChats(ctx context.Context, threadID int) ([]models.WorkspaceChat, error)
func (s *ThreadService) DeleteThreadEditedChats(ctx context.Context, threadID int) error
func (s *ThreadService) UpdateThreadChat(ctx context.Context, chatID int, req dto.UpdateChatRequest) error
```

### 3.2 Slug Generation

Matches Node's `WorkspaceThread.slugify()` exactly:

- Uses `github.com/gosimple/slug` with custom replacements:
  - `+` → "plus", `!` → "bang", `@` → "at", `*` → "splat", `.` → "dot"
  - Strip `:`, `~`, `(`, `)`, `'`, `"`, `|`
- Lowercase output
- If `req.Name` is empty, falls back to UUID v4
- If `req.Slug` is provided, slugify it; otherwise slugify `req.Name`

### 3.3 Thread-Aware Chat History

`ChatService.buildChatHistory()` gains a `threadID *int` parameter:

```go
func (s *ChatService) buildChatHistory(workspaceID int, threadID *int, limit int) ([]core.Message, error)
```

Query logic:
- `threadID == nil`: `WHERE workspace_id = ? AND thread_id IS NULL AND include = true`
- `threadID != nil`: `WHERE workspace_id = ? AND thread_id = ? AND include = true`

This matches Node's behavior: `forWorkspace` uses `thread_id: null`, while thread chats filter by `thread_id`.

### 3.4 Thread-Aware Chat Save

`ChatService.saveChatResponse()` gains a `threadID *int` parameter:

```go
func (s *ChatService) saveChatResponse(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, prompt, response string)
```

The `WorkspaceChat` record includes `ThreadID: threadID`.

---

## 4. Chat Service Extensions

### 4.1 Threaded Streaming

`ChatService.Stream()` signature:

```go
func (s *ChatService) Stream(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, req dto.StreamChatRequest) (<-chan dto.StreamChatResponse, error)
```

The existing streaming logic is unchanged. Only `buildChatHistory` and `saveChatResponse` are affected by `threadID`.

### 4.2 Chat Management Methods

```go
func (s *ChatService) DeleteWorkspaceChats(ctx context.Context, workspaceID int) error
// DELETE FROM workspace_chats WHERE workspace_id = ? AND thread_id IS NULL

func (s *ChatService) DeleteWorkspaceEditedChats(ctx context.Context, workspaceID int) error
// DELETE FROM workspace_chats WHERE workspace_id = ? AND thread_id IS NULL AND prompt != response

func (s *ChatService) UpdateChat(ctx context.Context, workspaceID int, chatID int, req dto.UpdateChatRequest) error
// UPDATE workspace_chats SET response = ?, include = ? WHERE id = ? AND workspace_id = ?

func (s *ChatService) UpdateChatFeedback(ctx context.Context, chatID int, score *bool) error
// UPDATE workspace_chats SET feedback_score = ? WHERE id = ?
```

### 4.3 Thread-Scoped Management

Delegated to `ThreadService` (used by thread-specific routes):

```go
func (s *ThreadService) DeleteThreadEditedChats(ctx context.Context, threadID int) error
func (s *ThreadService) UpdateThreadChat(ctx context.Context, chatID int, req dto.UpdateChatRequest) error
```

---

## 5. Auth Service Extensions

### 5.1 Password Reset Flow

**Token-based, 10-minute expiry.** Uses existing `PasswordResetToken` model.

```go
// POST /system/reset-password
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error
// 1. Find user where username = email
// 2. Create PasswordResetToken{UserID: user.ID, Token: uuid.New(), ExpiresAt: now + 10m}
// 3. Return nil (do not reveal if user exists)

// POST /system/recover-account
func (s *AuthService) RecoverAccount(ctx context.Context, token, newPassword string) error
// 1. Find token where token = ? AND expires_at > now
// 2. Hash newPassword with bcrypt
// 3. Update user password
// 4. Delete all reset tokens for this user
// 5. Return nil

// POST /system/update-password
func (s *AuthService) UpdatePassword(ctx context.Context, userID int, currentPassword, newPassword string) error
// 1. Find user, verify currentPassword with bcrypt
// 2. Hash newPassword, update user record
// 3. Return nil
```

### 5.2 Invite System

```go
// GET /invite/:code
func (s *AuthService) GetInvite(ctx context.Context, code string) (*models.Invite, error)
// Returns invite if status == "pending", nil otherwise

// POST /invite/:code
func (s *AuthService) AcceptInvite(ctx context.Context, code, username, password string) (*models.User, error)
// 1. Validate invite is pending
// 2. Create user with bcrypt password, role="default"
// 3. Mark invite claimed: status="claimed", claimedBy=user.ID
// 4. If invite.WorkspaceIds is non-empty, parse JSON array and create WorkspaceUser records
// 5. Return user
```

### 5.3 Admin Invite Management

```go
// POST /admin/invite/new
func (s *AdminService) CreateInvite(ctx context.Context, createdBy int, workspaceIDs []int) (*models.Invite, error)
// Generate random code (alphanumeric, 32 chars), store with createdBy + workspaceIds JSON

// GET /admin/invites
func (s *AdminService) ListInvites(ctx context.Context) ([]models.Invite, error)

// DELETE /admin/invite/:id
func (s *AdminService) DeactivateInvite(ctx context.Context, inviteID int) error
// UPDATE invites SET status = "disabled" WHERE id = ?
```

---

## 6. System Service Extensions — Onboarding

```go
// GET /onboarding
func (s *SystemService) GetOnboardingStatus(ctx context.Context) (bool, error)
// Query system_settings WHERE key = "onboarding_complete"
// Return true if found and value == "true"

// POST /onboarding
func (s *SystemService) CompleteOnboarding(ctx context.Context) error
// Upsert system_settings{key="onboarding_complete", value="true"}
```

---

## 7. Handler Routes & Middleware

### 7.1 New Middleware

```go
func ValidWorkspaceAndThreadSlug(db *gorm.DB) gin.HandlerFunc
```

1. Loads workspace by `:slug` (same as `ValidWorkspaceSlug`)
2. Loads thread by `:threadSlug` WHERE workspace_id = ws.ID
3. Sets `c.Set("thread", thread)`
4. Calls `c.Next()`

Returns 404 if workspace or thread not found.

### 7.2 Route Registration

```go
// internal/handlers/thread.go
func RegisterThreadRoutes(r *gin.RouterGroup, threadSvc *services.ThreadService, authSvc *services.AuthService, db *gorm.DB)

// internal/handlers/chat.go — extended RegisterChatRoutes
// + POST /workspace/:slug/thread/:threadSlug/stream-chat
// + DELETE /workspace/:slug/delete-chats
// + DELETE /workspace/:slug/delete-edited-chats
// + POST /workspace/:slug/update-chat
// + POST /workspace/:slug/chat-feedback/:chatId

// internal/handlers/auth.go — extended RegisterAuthRoutes
// + POST /system/reset-password
// + POST /system/recover-account
// + POST /system/update-password
// + GET /invite/:code
// + POST /invite/:code

// internal/handlers/system.go — extended RegisterSystemRoutes
// + GET /onboarding
// + POST /onboarding

// internal/handlers/admin.go — extended RegisterAdminRoutes
// + POST /admin/invite/new
// + GET /admin/invites
// + DELETE /admin/invite/:id
```

### 7.3 Middleware by Route

| Route | Auth | Role | Workspace | Thread |
|-------|------|------|-----------|--------|
| `POST /workspace/:slug/thread/new` | ✅ | all | ✅ | — |
| `GET /workspace/:slug/threads` | ✅ | all | ✅ | — |
| `DELETE /workspace/:slug/thread/:threadSlug` | ✅ | all | ✅ | ✅ |
| `DELETE /workspace/:slug/thread-bulk-delete` | ✅ | all | ✅ | — |
| `GET /workspace/:slug/thread/:threadSlug/chats` | ✅ | all | ✅ | ✅ |
| `POST /workspace/:slug/thread/:threadSlug/update` | ✅ | all | ✅ | ✅ |
| `DELETE /workspace/:slug/thread/:threadSlug/delete-edited-chats` | ✅ | all | ✅ | ✅ |
| `POST /workspace/:slug/thread/:threadSlug/update-chat` | ✅ | all | ✅ | ✅ |
| `POST /workspace/:slug/thread/:threadSlug/stream-chat` | ✅ | all | ✅ | ✅ |
| `DELETE /workspace/:slug/delete-chats` | ✅ | all | ✅ | — |
| `DELETE /workspace/:slug/delete-edited-chats` | ✅ | all | ✅ | — |
| `POST /workspace/:slug/update-chat` | ✅ | all | ✅ | — |
| `POST /workspace/:slug/chat-feedback/:chatId` | ✅ | all | ✅ | — |
| `POST /system/reset-password` | — | — | — | — |
| `POST /system/recover-account` | — | — | — | — |
| `POST /system/update-password` | ✅ | — | — | — |
| `GET /invite/:code` | — | — | — | — |
| `POST /invite/:code` | — | — | — | — |
| `GET /onboarding` | — | — | — | — |
| `POST /onboarding` | ✅ | — | — | — |
| `POST /admin/invite/new` | ✅ | admin | — | — |
| `GET /admin/invites` | ✅ | admin | — | — |
| `DELETE /admin/invite/:id` | ✅ | admin | — | — |

---

## 8. DTOs

### 8.1 Workspace DTOs

```go
type CreateThreadRequest struct {
    Name string `json:"name"`
    Slug string `json:"slug"` // optional, auto-generated if empty
}

type UpdateThreadRequest struct {
    Name string `json:"name"`
}
```

### 8.2 Chat DTOs

```go
type UpdateChatRequest struct {
    Response string `json:"response"`
    Include  *bool  `json:"include"`
}
```

### 8.3 Auth DTOs

```go
type ResetPasswordRequest struct {
    Email string `json:"email"`
}

type RecoverAccountRequest struct {
    Token       string `json:"token"`
    NewPassword string `json:"newPassword"`
}

type UpdatePasswordRequest struct {
    CurrentPassword string `json:"currentPassword"`
    NewPassword     string `json:"newPassword"`
}

type AcceptInviteRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}
```

### 8.4 Admin DTOs

```go
type CreateInviteRequest struct {
    WorkspaceIDs []int `json:"workspaceIds"`
}
```

---

## 9. Models

No model changes required. Existing models:

- `WorkspaceThread` — ID, Name, Slug, WorkspaceID, UserID, CreatedAt, LastUpdatedAt
- `WorkspaceChat` — ID, WorkspaceID, Prompt, Response, Include, UserID, ThreadID, APISessionID, CreatedAt, LastUpdatedAt, FeedbackScore
- `Invite` — ID, Code, Status, ClaimedBy, WorkspaceIds, CreatedAt, CreatedBy, LastUpdatedAt
- `PasswordResetToken` — ID, UserID, Token, ExpiresAt, CreatedAt

---

## 10. Testing Strategy

| Test File | Coverage |
|-----------|----------|
| `tests/integration/thread_test.go` | Thread CRUD: create (with slug generation), list, delete, bulk delete. Verify thread isolation (chats in thread A not visible in default thread). Verify `ValidWorkspaceAndThreadSlug` middleware rejects invalid slugs. |
| `tests/integration/chat_thread_test.go` | Threaded stream-chat: mock LLM provider returns chunks, verify history is filtered by thread, verify saved chat has `thread_id` set. Verify default stream-chat still works (thread_id IS NULL). |
| `tests/integration/chat_management_test.go` | Delete workspace chats, delete edited chats, update chat response/include, update feedback score. Verify DB state after each operation. |
| `tests/integration/auth_extended_test.go` | Password reset: request token (verify created in DB), recover with valid token (verify password changed, token deleted), reject expired token. Update password: verify current password required. Invite: create, get pending, accept (verify user created, invite claimed, workspace auto-assigned). |
| `tests/integration/onboarding_test.go` | GET returns false initially, POST sets true, subsequent GET returns true. |

All integration tests use SQLite in-memory (`:memory:`) with `gin.TestEngine()` and the full middleware chain. No external dependencies (Postgres, Collector, LLM) required — LLM and vector search are mocked.

---

## 11. Implementation Order

1. **ThreadService + ThreadHandler** — Foundation for all thread features
2. **ChatService extensions** — Thread-aware streaming + chat management
3. **AuthService extensions** — Password reset + invite system
4. **SystemService extensions** — Onboarding
5. **AdminService extensions** — Admin invite management
6. **Middleware** — `ValidWorkspaceAndThreadSlug`
7. **Main wiring** — Register all new routes in `main.go`
8. **Tests** — Integration tests for each feature group

---

## 12. Self-Review Checklist

- [x] No TBD/TODO placeholders
- [x] All 12 endpoints mapped to specific handler methods
- [x] Service boundaries are clear (ThreadService vs ChatService vs AuthService)
- [x] Middleware requirements specified for every route
- [x] Model changes explicitly listed (none needed)
- [x] DTOs have JSON tags matching Node backend expectations
- [x] Testing strategy covers happy path and edge cases
- [x] Implementation order is sequential with no circular dependencies
