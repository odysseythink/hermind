# P4a Admin Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the 9 missing `/api/admin/*` routes (user/workspace/API-key CRUD) in backend with response parity to Node `server/endpoints/admin.js`.

**Architecture:** Routes mount under `r.GET/POST/DELETE("/admin/...")` in `internal/handlers/admin.go`. Business rules (role validation, admin lock-out protection) live in `AdminService`. RBAC enforced by existing `middleware.FlexUserRoleValid([...])`. No new infrastructure; reuses `AdminService`, `APIKeyService`, `WorkspaceService`, `models.*`, and existing `utils.HashPassword`.

**Tech Stack:** Go 1.22+, Gin, GORM, SQLite (test), bcrypt (`pkg/utils.HashPassword`), testify.

**Source spec:** `.gpowers/designs/2026-05-22-backend-api-routes-design.md` §5.1.

**Reference Node implementation:** `server/endpoints/admin.js` lines 35-550 + `server/utils/helpers/admin/index.js`.

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `AdminHandler` already has: `ListUsers`, `DeleteUser`, `ListWorkspaces`, `CreateInvite`, `ListInvites`, `DeactivateInvite`, `SystemPreferencesFor`, `UpdateSystemPreferences` — keep all.
- `AdminService` already has: `ListUsers`, `DeleteUser`, `ListWorkspaces`, `CreateInvite`, `ListInvites`, `DeactivateInvite`, `generateInviteCode` — extend, do not rewrite.
- `APIKeyService` already has: `List`, `Create(ctx, createdBy *int, name *string)`, `Delete(ctx, id)`, `ValidateKey` — handlers are missing.
- `WorkspaceService.Create(ctx, userID, req)` already exists — can be reused by admin handler.
- `middleware.FlexUserRoleValid([]string{"admin"})` and `middleware.ValidatedRequest(authSvc)` are mounted via `RegisterAdminRoutes`.

### Routes to add (9)

| # | Method | Path | Roles |
|---|---|---|---|
| 1 | POST | `/admin/users/new` | admin, manager |
| 2 | POST | `/admin/user/:id` | admin, manager |
| 3 | DELETE | `/admin/user/:id` (replaces Go-side `DELETE /admin/users/:id` which doesn't match Node) | admin, manager |
| 4 | GET | `/admin/workspaces/:workspaceId/users` | admin, manager |
| 5 | POST | `/admin/workspaces/new` | admin, manager |
| 6 | POST | `/admin/workspaces/:workspaceId/update-users` | admin, manager |
| 7 | DELETE | `/admin/workspaces/:id` | admin, manager |
| 8 | GET | `/admin/api-keys` | admin only |
| 9 | POST | `/admin/generate-api-key` | admin only |
| 10 | DELETE | `/admin/delete-api-key/:id` | admin only |

(Total 10 handlers; #3 replaces the Go-side broken `DELETE /admin/users/:id`, so net new from Node-diff = 9.)

### Out of scope (explicit)

- `EventLogs.logEvent(...)` calls in handlers — Node logs `user_created`, `user_deleted`, `invite_created`, `invite_deleted`, `api_key_created`, `api_key_deleted`. **Skip for P4a**; emit nothing. Handled in a follow-up sub-plan when EventLog model is built.
- `BrowserExtensionApiKey.deleteAllForUser` on user-delete cascade — defer; Go has no `BrowserExtensionAPIKey` model yet.
- Telemetry calls (`Telemetry.sendTelemetry`).

### Response-shape conventions (match Node exactly)

- All success responses: HTTP **200** even on business validation errors. Errors carry `{success: false, error: "..."}` or `{user: null, error: "..."}`.
- HTTP 4xx/5xx only for: auth/permission middleware rejects (handled by middleware), bad JSON body (400), genuine server errors (500).
- `DELETE /admin/delete-api-key/:id` returns **empty 200 body** (Node uses `response.status(200).end()`); other endpoints return JSON.

### TDD discipline

Every task follows: write failing test → run & confirm fail → implement → run & confirm pass → commit. Each test must hit an HTTP route via `httptest`, not the service directly, unless the task explicitly creates a service-layer test.

---

## Task 1: RBAC business rules in AdminService

Add `ValidRoleSelection`, `CanModifyAdmin`, `ValidCanModify` to `AdminService`. These are pure functions used by tasks 2, 3, 4.

**Files:**
- Modify: `backend/internal/services/admin_service.go`
- Create: `backend/internal/services/admin_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/services/admin_service_test.go
package services

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestValidRoleSelection(t *testing.T) {
	svc := &AdminService{}
	admin := &models.User{Role: "admin"}
	manager := &models.User{Role: "manager"}

	// admin can assign any role
	ok, err := svc.ValidRoleSelection(admin, map[string]any{"role": "admin"})
	assert.True(t, ok)
	assert.Empty(t, err)

	// manager cannot assign admin role
	ok, err = svc.ValidRoleSelection(manager, map[string]any{"role": "admin"})
	assert.False(t, ok)
	assert.Equal(t, "Invalid role selection for user.", err)

	// manager can assign default/manager roles
	ok, _ = svc.ValidRoleSelection(manager, map[string]any{"role": "default"})
	assert.True(t, ok)
	ok, _ = svc.ValidRoleSelection(manager, map[string]any{"role": "manager"})
	assert.True(t, ok)

	// no role key in updates -> always valid
	ok, _ = svc.ValidRoleSelection(manager, map[string]any{"username": "x"})
	assert.True(t, ok)
}

func TestValidCanModify(t *testing.T) {
	svc := &AdminService{}
	admin := &models.User{Role: "admin"}
	manager := &models.User{Role: "manager"}
	defaultUser := &models.User{Role: "default"}
	otherAdmin := &models.User{Role: "admin"}

	// admin can modify anyone
	ok, _ := svc.ValidCanModify(admin, otherAdmin)
	assert.True(t, ok)

	// manager cannot modify admin
	ok, err := svc.ValidCanModify(manager, otherAdmin)
	assert.False(t, ok)
	assert.Equal(t, "Cannot perform that action on user.", err)

	// manager can modify default/manager
	ok, _ = svc.ValidCanModify(manager, defaultUser)
	assert.True(t, ok)
}

func TestCanModifyAdmin_lastAdminLockout(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	_ = enc
	svc := NewAdminService(db)

	hash, _ := utils.HashPassword("pw")
	soleAdmin := &models.User{Username: utils.Ptr("solo"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(soleAdmin).Error)

	// trying to demote the only admin -> blocked
	ok, err := svc.CanModifyAdmin(soleAdmin, map[string]any{"role": "default"})
	assert.False(t, ok)
	assert.Contains(t, err, "No system admins")

	// adding a 2nd admin then demoting first is allowed
	otherAdmin := &models.User{Username: utils.Ptr("co"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(otherAdmin).Error)
	ok, _ = svc.CanModifyAdmin(soleAdmin, map[string]any{"role": "default"})
	assert.True(t, ok)
}
```

This test file references `testCfg` and `testDB` helpers — they may not exist yet. Add them as part of step 1:

```go
// Append to admin_service_test.go
import (
	"github.com/odysseythink/hermind/backend/internal/config"
	"gorm.io/gorm"
)

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
}

func testDB(t *testing.T, cfg *config.Config) *gorm.DB {
	t.Helper()
	db, err := NewDB(cfg)
	assert.NoError(t, err)
	assert.NoError(t, AutoMigrate(db))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return db
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestValidRoleSelection -v`
Expected: FAIL with `svc.ValidRoleSelection undefined`.

- [ ] **Step 3: Implement the three methods**

Append to `backend/internal/services/admin_service.go`:

```go
// ValidRoleSelection mirrors server/utils/helpers/admin/index.js:validRoleSelection.
// Returns (valid, errorString). When updates has no "role" key, always valid.
// Admins can assign any role; managers can only assign manager/default.
func (s *AdminService) ValidRoleSelection(currentUser *models.User, updates map[string]any) (bool, string) {
	roleVal, ok := updates["role"]
	if !ok {
		return true, ""
	}
	if currentUser.Role == "admin" {
		return true, ""
	}
	if currentUser.Role == "manager" {
		newRole, _ := roleVal.(string)
		if newRole != "manager" && newRole != "default" {
			return false, "Invalid role selection for user."
		}
		return true, ""
	}
	return false, "Invalid condition for caller."
}

// ValidCanModify mirrors server/utils/helpers/admin/index.js:validCanModify.
// Admins can modify any user; managers can only modify manager/default users.
func (s *AdminService) ValidCanModify(currentUser, existingUser *models.User) (bool, string) {
	if currentUser.Role == "admin" {
		return true, ""
	}
	if currentUser.Role == "manager" {
		if existingUser.Role != "manager" && existingUser.Role != "default" {
			return false, "Cannot perform that action on user."
		}
		return true, ""
	}
	return false, "Invalid condition for caller."
}

// CanModifyAdmin mirrors server/utils/helpers/admin/index.js:canModifyAdmin.
// Prevents the last remaining admin from being demoted.
// Returns (valid, errorString).
func (s *AdminService) CanModifyAdmin(userToModify *models.User, updates map[string]any) (bool, string) {
	roleVal, ok := updates["role"]
	if !ok {
		return true, ""
	}
	if userToModify.Role != "admin" {
		return true, ""
	}
	newRole, _ := roleVal.(string)
	if newRole == "admin" {
		return true, ""
	}
	var count int64
	if err := s.db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return false, err.Error()
	}
	if count-1 <= 0 {
		return false, "No system admins will remain if you do this. Update failed."
	}
	return true, ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/services/ -run "TestValidRoleSelection|TestValidCanModify|TestCanModifyAdmin" -v`
Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/admin_service.go backend/internal/services/admin_service_test.go
git commit -m "feat(admin): add RBAC helpers (role validation, admin lockout)"
```

---

## Task 2: AdminService — create / update / find user

Wire the persistence methods that route handlers will call.

**Files:**
- Modify: `backend/internal/services/admin_service.go`
- Modify: `backend/internal/services/admin_service_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_service_test.go`:

```go
func TestAdminService_CreateUser(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewAdminService(db)

	u, errStr, err := svc.CreateUser(t.Context(), CreateUserInput{
		Username: "newbie",
		Password: "Password123!",
		Role:     "default",
	})
	assert.NoError(t, err)
	assert.Empty(t, errStr)
	assert.NotNil(t, u)
	assert.Equal(t, "newbie", *u.Username)
	assert.NotEqual(t, "Password123!", u.Password, "password must be hashed")

	// duplicate username -> business error returned (not Go error)
	_, errStr2, err2 := svc.CreateUser(t.Context(), CreateUserInput{
		Username: "newbie", Password: "Password123!", Role: "default",
	})
	assert.NoError(t, err2)
	assert.NotEmpty(t, errStr2)
}

func TestAdminService_UpdateUser(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewAdminService(db)

	created, _, _ := svc.CreateUser(t.Context(), CreateUserInput{
		Username: "u1", Password: "Password123!", Role: "default",
	})

	errStr, err := svc.UpdateUser(t.Context(), created.ID, map[string]any{
		"role": "manager",
	})
	assert.NoError(t, err)
	assert.Empty(t, errStr)

	got, err := svc.GetUserByID(t.Context(), created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "manager", got.Role)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run "TestAdminService_CreateUser|TestAdminService_UpdateUser" -v`
Expected: FAIL — `CreateUser`, `UpdateUser`, `GetUserByID`, `CreateUserInput` undefined.

- [ ] **Step 3: Implement**

Append to `admin_service.go`:

```go
import (
	// add to existing imports:
	"errors"
	"strings"

	"github.com/odysseythink/hermind/backend/pkg/utils"
)

type CreateUserInput struct {
	Username          string
	Password          string
	Role              string
	Bio               string
	DailyMessageLimit *int
}

// CreateUser returns (user, businessError, systemError). businessError is a user-facing
// validation/uniqueness message returned in JSON; systemError is a Go error.
func (s *AdminService) CreateUser(ctx context.Context, in CreateUserInput) (*models.User, string, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, "Username is required.", nil
	}
	if in.Password == "" {
		return nil, "Password is required.", nil
	}
	role := in.Role
	if role != "admin" && role != "manager" && role != "default" {
		role = "default"
	}
	hash, err := utils.HashPassword(in.Password)
	if err != nil {
		return nil, "", err
	}
	u := models.User{
		Username:          utils.Ptr(username),
		Password:          hash,
		Role:              role,
		Bio:               utils.Ptr(in.Bio),
		DailyMessageLimit: in.DailyMessageLimit,
		CreatedAt:         time.Now(),
		LastUpdatedAt:     time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&u).Error; err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			return nil, "A user with that username already exists", nil
		}
		return nil, "", err
	}
	return &u, "", nil
}

// UpdateUser applies a subset of writable fields. Returns (businessError, systemError).
func (s *AdminService) UpdateUser(ctx context.Context, id int, updates map[string]any) (string, error) {
	writable := map[string]bool{
		"username":          true,
		"password":          true,
		"role":              true,
		"suspended":         true,
		"bio":               true,
		"dailyMessageLimit": true,
		"pfpFilename":       true,
	}
	colMap := map[string]string{
		"dailyMessageLimit": "daily_message_limit",
		"pfpFilename":       "pfp_filename",
	}
	cleaned := map[string]any{}
	for k, v := range updates {
		if !writable[k] {
			continue
		}
		col := k
		if mapped, ok := colMap[k]; ok {
			col = mapped
		}
		if k == "password" {
			pw, _ := v.(string)
			if pw == "" {
				continue
			}
			hash, err := utils.HashPassword(pw)
			if err != nil {
				return "", err
			}
			cleaned[col] = hash
			continue
		}
		cleaned[col] = v
	}
	if len(cleaned) == 0 {
		return "No valid updates applied.", nil
	}
	cleaned["last_updated_at"] = time.Now()
	res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Updates(cleaned)
	if res.Error != nil {
		lower := strings.ToLower(res.Error.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			return "A user with that username already exists", nil
		}
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "User not found", nil
	}
	return "", nil
}

func (s *AdminService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var u models.User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/services/ -run "TestAdminService_CreateUser|TestAdminService_UpdateUser" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/admin_service.go backend/internal/services/admin_service_test.go
git commit -m "feat(admin): CreateUser/UpdateUser/GetUserByID with bcrypt + uniqueness handling"
```

---

## Task 3: Handler — POST /admin/users/new

**Files:**
- Modify: `backend/internal/handlers/admin.go`
- Create: `backend/tests/integration/admin_test.go`
- Modify: `backend/cmd/server/main.go` (route registration — done in Task 12)

- [ ] **Step 1: Write the failing test**

Create `backend/tests/integration/admin_test.go`:

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
	"gorm.io/gorm"
)

func setupAdminRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
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
	assert.NoError(t, services.AutoMigrate(db))
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	adminSvc := services.NewAdminService(db)
	sysSvc := services.NewSystemService(db)
	r := gin.New()
	handlers.RegisterAdminRoutes(r.Group("/api"), adminSvc, sysSvc, authSvc)
	return r, db, authSvc, cfg
}

// seedAdmin inserts an admin user and returns a valid JWT for that user.
func seedAdmin(t *testing.T, db *gorm.DB, cfg *config.Config) (*models.User, string) {
	t.Helper()
	hash, _ := utils.HashPassword("pw")
	u := &models.User{Username: utils.Ptr("root"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(u).Error)
	tok, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, 60*60)
	assert.NoError(t, err)
	return u, tok
}

func TestAdmin_CreateUserNew(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)

	body, _ := json.Marshal(map[string]any{
		"username": "newbie",
		"password": "Password123!",
		"role":     "default",
	})
	req, _ := http.NewRequest("POST", "/api/admin/users/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		User  *models.User `json:"user"`
		Error *string      `json:"error"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.User)
	assert.Nil(t, resp.Error)
	assert.Equal(t, "newbie", *resp.User.Username)
}

func TestAdmin_CreateUserNew_managerCannotMakeAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, 60*60)

	body, _ := json.Marshal(map[string]any{
		"username": "evilroot", "password": "Password123!", "role": "admin",
	})
	req, _ := http.NewRequest("POST", "/api/admin/users/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		User  any    `json:"user"`
		Error string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, resp.User)
	assert.Equal(t, "Invalid role selection for user.", resp.Error)
}

func TestAdmin_CreateUserNew_unauthorized(t *testing.T) {
	r, _, _, _ := setupAdminRouter(t)
	req, _ := http.NewRequest("POST", "/api/admin/users/new", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

```

Remove the unused `context` import if Go complains; only add it back when a later task needs `context.Background()`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_CreateUserNew -v`
Expected: FAIL — route returns 404 (route not registered yet).

- [ ] **Step 3: Implement handler**

Add to `backend/internal/handlers/admin.go`:

```go
// imports: ensure "github.com/odysseythink/hermind/backend/internal/services" is present (it is).

func (h *AdminHandler) CreateUserNew(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"user": nil, "error": err.Error()})
		return
	}
	if ok, errStr := h.adminSvc.ValidRoleSelection(currUser, body); !ok {
		c.JSON(http.StatusOK, gin.H{"user": nil, "error": errStr})
		return
	}
	in := services.CreateUserInput{
		Username: asString(body["username"]),
		Password: asString(body["password"]),
		Role:     asString(body["role"]),
		Bio:      asString(body["bio"]),
	}
	if dml, ok := body["dailyMessageLimit"].(float64); ok {
		v := int(dml)
		in.DailyMessageLimit = &v
	}
	u, businessErr, sysErr := h.adminSvc.CreateUser(c.Request.Context(), in)
	if sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"user": nil, "error": sysErr.Error()})
		return
	}
	if businessErr != "" {
		c.JSON(http.StatusOK, gin.H{"user": nil, "error": businessErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "error": nil})
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
```

And register the route in `RegisterAdminRoutes` (append before the closing brace of the function):

```go
	r.POST("/admin/users/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.CreateUserNew)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_CreateUserNew -v`
Expected: PASS all three subtests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "feat(admin): POST /admin/users/new with role validation"
```

---

## Task 4: Handler — POST /admin/user/:id (update)

**Files:**
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/tests/integration/admin_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_UpdateUser_byAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	// create a target user
	hash, _ := utils.HashPassword("pw")
	target := &models.User{Username: utils.Ptr("u1"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(target).Error)

	body, _ := json.Marshal(map[string]any{"role": "manager"})
	req, _ := http.NewRequest("POST", "/api/admin/user/"+itoa(target.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	var refreshed models.User
	db.First(&refreshed, target.ID)
	assert.Equal(t, "manager", refreshed.Role)
}

func TestAdmin_UpdateUser_managerCannotDemoteAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	adm := &models.User{Username: utils.Ptr("adm"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(adm).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, 60*60)

	body, _ := json.Marshal(map[string]any{"role": "default"})
	req, _ := http.NewRequest("POST", "/api/admin/user/"+itoa(adm.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Cannot perform that action on user.", resp.Error)
}

func TestAdmin_UpdateUser_lastAdminLockout(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	soleAdmin, tok := seedAdmin(t, db, cfg)

	body, _ := json.Marshal(map[string]any{"role": "default"})
	req, _ := http.NewRequest("POST", "/api/admin/user/"+itoa(soleAdmin.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "No system admins")
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }
```

Add `"fmt"` to imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_UpdateUser -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement handler**

Add to `admin.go`:

```go
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	target, err := h.adminSvc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if target == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "User not found"})
		return
	}
	if ok, errStr := h.adminSvc.ValidCanModify(currUser, target); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if ok, errStr := h.adminSvc.ValidRoleSelection(currUser, updates); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if ok, errStr := h.adminSvc.CanModifyAdmin(target, updates); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if businessErr, sysErr := h.adminSvc.UpdateUser(c.Request.Context(), id, updates); sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": sysErr.Error()})
		return
	} else if businessErr != "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": businessErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Register the route in `RegisterAdminRoutes`:

```go
	r.POST("/admin/user/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.UpdateUser)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_UpdateUser -v`
Expected: PASS all three subtests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "feat(admin): POST /admin/user/:id with role + lockout protection"
```

---

## Task 5: Handler — fix DELETE /admin/user/:id (replace broken /admin/users/:id)

The Go side currently registers `DELETE /admin/users/:id` which Node doesn't expose. Replace with the correct path **and** add the `ValidCanModify` check.

**Files:**
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/tests/integration/admin_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_DeleteUser_byAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	hash, _ := utils.HashPassword("pw")
	target := &models.User{Username: utils.Ptr("v1"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(target).Error)

	req, _ := http.NewRequest("DELETE", "/api/admin/user/"+itoa(target.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	var stillThere models.User
	err := db.First(&stillThere, target.ID).Error
	assert.Error(t, err) // record not found
}

func TestAdmin_DeleteUser_managerCannotDeleteAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	adm := &models.User{Username: utils.Ptr("adm"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(adm).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, 60*60)

	req, _ := http.NewRequest("DELETE", "/api/admin/user/"+itoa(adm.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Cannot perform that action on user.", resp.Error)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_DeleteUser -v`
Expected: FAIL — route `/admin/user/:id` is 404 (only `/admin/users/:id` exists).

- [ ] **Step 3: Implement: rename existing handler and update route registration**

In `admin.go`, the existing `DeleteUser` handler skips the canModify check. Replace it:

```go
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	target, err := h.adminSvc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if target == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "User not found"})
		return
	}
	if ok, errStr := h.adminSvc.ValidCanModify(currUser, target); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if err := h.adminSvc.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

In `RegisterAdminRoutes`, **replace**:

```go
	r.DELETE("/admin/users/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeleteUser)
```

with:

```go
	r.DELETE("/admin/user/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.DeleteUser)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_DeleteUser -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "fix(admin): correct DELETE /admin/user/:id path + add canModify check"
```

---

## Task 6: Handler — POST /admin/workspaces/new

**Files:**
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/cmd/server/main.go` (already wires WorkspaceService — confirm `wsSvc` is in scope of AdminHandler)
- Modify: `backend/internal/services/admin_service.go` (add CreateWorkspace wrapper if needed)
- Modify: `backend/tests/integration/admin_test.go`

We'll add a thin wrapper to AdminService so AdminHandler doesn't reach across to WorkspaceService directly. This keeps the handler dependencies clean and avoids changing `NewAdminHandler` signature multiple times.

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_CreateWorkspace(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	root, tok := seedAdmin(t, db, cfg)

	body, _ := json.Marshal(map[string]any{"name": "ProjectX"})
	req, _ := http.NewRequest("POST", "/api/admin/workspaces/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Workspace *models.Workspace `json:"workspace"`
		Error     *string           `json:"error"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Workspace)
	assert.Nil(t, resp.Error)
	assert.Equal(t, "ProjectX", resp.Workspace.Name)

	// creator should be linked as admin
	var wu models.WorkspaceUser
	assert.NoError(t, db.Where("user_id = ? AND workspace_id = ?", root.ID, resp.Workspace.ID).First(&wu).Error)
	assert.Equal(t, "admin", wu.Role)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_CreateWorkspace -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement**

To AdminHandler we need access to `WorkspaceService`. Refactor the handler constructor to accept it. Update `admin.go`:

```go
type AdminHandler struct {
	adminSvc *services.AdminService
	sysSvc   *services.SystemService
	wsSvc    *services.WorkspaceService
	apiKeySvc *services.APIKeyService
}

func NewAdminHandler(adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, apiKeySvc *services.APIKeyService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc, sysSvc: sysSvc, wsSvc: wsSvc, apiKeySvc: apiKeySvc}
}
```

Update `RegisterAdminRoutes` signature:

```go
func RegisterAdminRoutes(r *gin.RouterGroup, adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, apiKeySvc *services.APIKeyService, authSvc *services.AuthService) {
	h := NewAdminHandler(adminSvc, sysSvc, wsSvc, apiKeySvc)
	// ... existing route registrations unchanged ...
}
```

Add the handler:

```go
func (h *AdminHandler) CreateWorkspace(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"workspace": nil, "error": err.Error()})
		return
	}
	ws, err := h.wsSvc.Create(c.Request.Context(), currUser.ID, dto.CreateWorkspaceRequest{Name: body.Name})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "error": nil})
}
```

Add the route registration:

```go
	r.POST("/admin/workspaces/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.CreateWorkspace)
```

Update `setupAdminRouter` in `admin_test.go` to pass the new args:

```go
	wsSvc := services.NewWorkspaceService(db, cfg)
	apiKeySvc := services.NewAPIKeyService(db)
	handlers.RegisterAdminRoutes(r.Group("/api"), adminSvc, sysSvc, wsSvc, apiKeySvc, authSvc)
```

Update `cmd/server/main.go`:

```go
	handlers.RegisterAdminRoutes(api, adminSvc, sysSvc, wsSvc, apiKeySvc, authSvc)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_CreateWorkspace -v && cd backend && go build ./...`
Expected: PASS + build green.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/admin.go backend/tests/integration/admin_test.go backend/cmd/server/main.go
git commit -m "feat(admin): POST /admin/workspaces/new (refactor handler to accept WorkspaceSvc/APIKeySvc)"
```

---

## Task 7: Handler — GET /admin/workspaces/:workspaceId/users

**Files:**
- Modify: `backend/internal/services/workspace_service.go`
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/tests/integration/admin_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_ListWorkspaceUsers(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	root, tok := seedAdmin(t, db, cfg)

	hash, _ := utils.HashPassword("pw")
	alice := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(alice).Error)
	ws := &models.Workspace{Name: "W", Slug: "w-slug"}
	assert.NoError(t, db.Create(ws).Error)
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: root.ID, Role: "admin"}).Error)
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: alice.ID, Role: "default"}).Error)

	req, _ := http.NewRequest("GET", "/api/admin/workspaces/"+itoa(ws.ID)+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Users []map[string]any `json:"users"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Users, 2)
	// each entry has userId/username/role
	for _, u := range resp.Users {
		assert.Contains(t, u, "userId")
		assert.Contains(t, u, "username")
		assert.Contains(t, u, "role")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_ListWorkspaceUsers -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement**

Add to `workspace_service.go`:

```go
type WorkspaceUserInfo struct {
	UserID        int       `json:"userId"`
	Username      string    `json:"username"`
	Role          string    `json:"role"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

func (s *WorkspaceService) ListWorkspaceUsers(ctx context.Context, workspaceID int) ([]WorkspaceUserInfo, error) {
	var wus []models.WorkspaceUser
	if err := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&wus).Error; err != nil {
		return nil, err
	}
	if len(wus) == 0 {
		return []WorkspaceUserInfo{}, nil
	}
	userIDs := make([]int, 0, len(wus))
	for _, wu := range wus {
		userIDs = append(userIDs, wu.UserID)
	}
	var users []models.User
	if err := s.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	byID := map[int]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	out := make([]WorkspaceUserInfo, 0, len(wus))
	for _, wu := range wus {
		u, ok := byID[wu.UserID]
		if !ok {
			continue
		}
		username := ""
		if u.Username != nil {
			username = *u.Username
		}
		out = append(out, WorkspaceUserInfo{
			UserID:        u.ID,
			Username:      username,
			Role:          u.Role,
			LastUpdatedAt: wu.LastUpdatedAt,
		})
	}
	return out, nil
}
```

Add to `admin.go`:

```go
func (h *AdminHandler) ListWorkspaceUsers(c *gin.Context) {
	wsID, err := strconv.Atoi(c.Param("workspaceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"users": []any{}})
		return
	}
	users, err := h.wsSvc.ListWorkspaceUsers(c.Request.Context(), wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"users": []any{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}
```

Register the route:

```go
	r.GET("/admin/workspaces/:workspaceId/users",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.ListWorkspaceUsers)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_ListWorkspaceUsers -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/workspace_service.go backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "feat(admin): GET /admin/workspaces/:workspaceId/users"
```

---

## Task 8: Handler — POST /admin/workspaces/:workspaceId/update-users

Replaces ALL users currently in a workspace with the given userIds. Creator user is included by convention (Node behavior).

**Files:**
- Modify: `backend/internal/services/workspace_service.go`
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/tests/integration/admin_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_UpdateWorkspaceUsers(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)

	hash, _ := utils.HashPassword("pw")
	u1 := &models.User{Username: utils.Ptr("u1"), Password: hash, Role: "default"}
	u2 := &models.User{Username: utils.Ptr("u2"), Password: hash, Role: "default"}
	u3 := &models.User{Username: utils.Ptr("u3"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(u1).Error)
	assert.NoError(t, db.Create(u2).Error)
	assert.NoError(t, db.Create(u3).Error)
	ws := &models.Workspace{Name: "W", Slug: "w-slug"}
	assert.NoError(t, db.Create(ws).Error)
	// Initially has u1 and u2
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: u1.ID, Role: "default"}).Error)
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: u2.ID, Role: "default"}).Error)

	body, _ := json.Marshal(map[string]any{"userIds": []int{u2.ID, u3.ID}})
	req, _ := http.NewRequest("POST", "/api/admin/workspaces/"+itoa(ws.ID)+"/update-users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	var ids []int
	db.Model(&models.WorkspaceUser{}).Where("workspace_id = ?", ws.ID).Pluck("user_id", &ids)
	assert.ElementsMatch(t, []int{u2.ID, u3.ID}, ids)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_UpdateWorkspaceUsers -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement**

Add to `workspace_service.go`:

```go
func (s *WorkspaceService) UpdateUsers(ctx context.Context, workspaceID int, userIDs []int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.WorkspaceUser{}).Error; err != nil {
			return err
		}
		now := time.Now()
		rows := make([]models.WorkspaceUser, 0, len(userIDs))
		for _, uid := range userIDs {
			rows = append(rows, models.WorkspaceUser{
				WorkspaceID:   workspaceID,
				UserID:        uid,
				Role:          "default",
				CreatedAt:     now,
				LastUpdatedAt: now,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}
```

Add to `admin.go`:

```go
func (h *AdminHandler) UpdateWorkspaceUsers(c *gin.Context) {
	wsID, err := strconv.Atoi(c.Param("workspaceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	var body struct {
		UserIDs []int `json:"userIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.wsSvc.UpdateUsers(c.Request.Context(), wsID, body.UserIDs); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Register:

```go
	r.POST("/admin/workspaces/:workspaceId/update-users",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.UpdateWorkspaceUsers)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_UpdateWorkspaceUsers -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/workspace_service.go backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "feat(admin): POST /admin/workspaces/:id/update-users"
```

---

## Task 9: Handler — DELETE /admin/workspaces/:id

Full cascade: workspace_chats → document_vectors → workspace_documents → workspace → VectorDB namespace (best-effort).

**Files:**
- Modify: `backend/internal/services/workspace_service.go`
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/tests/integration/admin_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_DeleteWorkspace_cascade(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)

	ws := &models.Workspace{Name: "Doomed", Slug: "doomed"}
	assert.NoError(t, db.Create(ws).Error)
	assert.NoError(t, db.Create(&models.WorkspaceChat{WorkspaceID: ws.ID}).Error)
	assert.NoError(t, db.Create(&models.WorkspaceDocument{WorkspaceID: ws.ID, Filename: "f.txt", DocId: "d1"}).Error)
	assert.NoError(t, db.Create(&models.DocumentVector{DocId: "d1", VectorId: "v1"}).Error)

	req, _ := http.NewRequest("DELETE", "/api/admin/workspaces/"+itoa(ws.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct{ Success bool `json:"success"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	// confirm all related rows gone
	var chatCount, docCount, wsCount, vecCount int64
	db.Model(&models.WorkspaceChat{}).Where("workspace_id = ?", ws.ID).Count(&chatCount)
	db.Model(&models.WorkspaceDocument{}).Where("workspace_id = ?", ws.ID).Count(&docCount)
	db.Model(&models.Workspace{}).Where("id = ?", ws.ID).Count(&wsCount)
	db.Model(&models.DocumentVector{}).Where("doc_id = ?", "d1").Count(&vecCount)
	assert.Zero(t, chatCount)
	assert.Zero(t, docCount)
	assert.Zero(t, wsCount)
	assert.Zero(t, vecCount)
}

func TestAdmin_DeleteWorkspace_notFound(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	_ = db

	req, _ := http.NewRequest("DELETE", "/api/admin/workspaces/99999", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
}
```

Note: `models.WorkspaceDocument` field for the document ID is `DocId` (capital D, lowercase d at the end — already used above) and `models.DocumentVector` has no `WorkspaceID` column. The cascade implementation below joins via `WorkspaceDocument.doc_id`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_DeleteWorkspace -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `workspace_service.go`:

```go
func (s *WorkspaceService) DeleteByID(ctx context.Context, id int) (bool, error) {
	var ws models.Workspace
	if err := s.db.WithContext(ctx).First(&ws, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceChat{}).Error; err != nil {
			return err
		}
		// document_vectors has no workspace_id column — join via workspace_documents.doc_id
		var docIDs []string
		if err := tx.Model(&models.WorkspaceDocument{}).Where("workspace_id = ?", id).Pluck("doc_id", &docIDs).Error; err != nil {
			return err
		}
		if len(docIDs) > 0 {
			if err := tx.Where("doc_id IN ?", docIDs).Delete(&models.DocumentVector{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceDocument{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceUser{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Workspace{}, id).Error
	})
}

// Slug exposes the workspace slug for vector-namespace lookup.
func (s *WorkspaceService) GetByID(ctx context.Context, id int) (*models.Workspace, error) {
	var ws models.Workspace
	if err := s.db.WithContext(ctx).First(&ws, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ws, nil
}
```

Add `"errors"` to imports.

Add to `admin.go`:

```go
func (h *AdminHandler) DeleteWorkspaceByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	// fetch slug first so we can best-effort delete vector namespace
	ws, err := h.wsSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if ws == nil {
		c.Status(http.StatusNotFound)
		return
	}
	found, err := h.wsSvc.DeleteByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if !found {
		c.Status(http.StatusNotFound)
		return
	}
	// best-effort: vector namespace cleanup. We don't have a vector svc
	// reference in AdminHandler in P4a — defer to P5. Comment marks the gap.
	_ = ws.Slug
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Register:

```go
	r.DELETE("/admin/workspaces/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.DeleteWorkspaceByID)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_DeleteWorkspace -v`
Expected: PASS both subtests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/workspace_service.go backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "feat(admin): DELETE /admin/workspaces/:id with DB cascade"
```

---

## Task 10: APIKey handlers (GET list / POST generate / DELETE)

Three routes share a handler family. APIKey list must include `createdBy: {id, username, role}` joined from User.

**Files:**
- Modify: `backend/internal/services/api_key_service.go`
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/tests/integration/admin_test.go`

- [ ] **Step 1: Write the failing test**

Append to `admin_test.go`:

```go
func TestAdmin_APIKey_GenerateListDelete(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	_ = db

	// 1. generate
	body, _ := json.Marshal(map[string]any{"name": "ci-key"})
	req, _ := http.NewRequest("POST", "/api/admin/generate-api-key", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var genResp struct {
		APIKey *models.APIKey `json:"apiKey"`
		Error  *string        `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &genResp)
	assert.NotNil(t, genResp.APIKey)
	assert.NotNil(t, genResp.APIKey.Secret)

	// 2. list
	req, _ = http.NewRequest("GET", "/api/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var listResp struct {
		APIKeys []map[string]any `json:"apiKeys"`
		Error   any              `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.Len(t, listResp.APIKeys, 1)
	createdBy := listResp.APIKeys[0]["createdBy"]
	assert.NotNil(t, createdBy)
	cb, ok := createdBy.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "root", cb["username"])

	// 3. delete
	id := int(listResp.APIKeys[0]["id"].(float64))
	req, _ = http.NewRequest("DELETE", "/api/admin/delete-api-key/"+itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.String()) // Node returns empty 200 body

	// 4. confirm gone
	req, _ = http.NewRequest("GET", "/api/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.Len(t, listResp.APIKeys, 0)
}

func TestAdmin_APIKey_managerForbidden(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, 60*60)

	req, _ := http.NewRequest("GET", "/api/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_APIKey -v`
Expected: FAIL — routes 404.

- [ ] **Step 3: Implement**

Add to `api_key_service.go`:

```go
type APIKeyWithUser struct {
	models.APIKey
	CreatedByUser *struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"createdBy"`
}

func (s *APIKeyService) ListWithUser(ctx context.Context) ([]APIKeyWithUser, error) {
	var keys []models.APIKey
	if err := s.db.WithContext(ctx).Order("id desc").Find(&keys).Error; err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return []APIKeyWithUser{}, nil
	}
	uids := []int{}
	for _, k := range keys {
		if k.CreatedBy != nil {
			uids = append(uids, *k.CreatedBy)
		}
	}
	var users []models.User
	if len(uids) > 0 {
		if err := s.db.WithContext(ctx).Where("id IN ?", uids).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	byID := map[int]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	out := make([]APIKeyWithUser, 0, len(keys))
	for _, k := range keys {
		entry := APIKeyWithUser{APIKey: k}
		if k.CreatedBy != nil {
			if u, ok := byID[*k.CreatedBy]; ok {
				username := ""
				if u.Username != nil {
					username = *u.Username
				}
				entry.CreatedByUser = &struct {
					ID       int    `json:"id"`
					Username string `json:"username"`
					Role     string `json:"role"`
				}{ID: u.ID, Username: username, Role: u.Role}
			}
		}
		out = append(out, entry)
	}
	return out, nil
}
```

Note: Node's `whereWithUser` mutates `apiKey.createdBy` from int to an object. Our Go shape **overrides** the JSON via `CreatedByUser` field but the json tag matches `"createdBy"`. Since `models.APIKey.CreatedBy` is `*int` with a json tag `"createdBy"` too, there would be a duplicate-key collision. **Fix this by giving the embedded APIKey field a different shape** — instead of embedding, copy:

```go
type APIKeyWithUser struct {
	ID            int       `json:"id"`
	Name          *string   `json:"name"`
	Secret        *string   `json:"secret"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	CreatedBy     any       `json:"createdBy"` // nil OR {id,username,role}
}

func (s *APIKeyService) ListWithUser(ctx context.Context) ([]APIKeyWithUser, error) {
	var keys []models.APIKey
	if err := s.db.WithContext(ctx).Order("id desc").Find(&keys).Error; err != nil {
		return nil, err
	}
	uids := []int{}
	for _, k := range keys {
		if k.CreatedBy != nil {
			uids = append(uids, *k.CreatedBy)
		}
	}
	var users []models.User
	if len(uids) > 0 {
		if err := s.db.WithContext(ctx).Where("id IN ?", uids).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	byID := map[int]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	out := make([]APIKeyWithUser, 0, len(keys))
	for _, k := range keys {
		entry := APIKeyWithUser{
			ID: k.ID, Name: k.Name, Secret: k.Secret,
			CreatedAt: k.CreatedAt, LastUpdatedAt: k.LastUpdatedAt,
		}
		if k.CreatedBy != nil {
			if u, ok := byID[*k.CreatedBy]; ok {
				username := ""
				if u.Username != nil {
					username = *u.Username
				}
				entry.CreatedBy = map[string]any{
					"id": u.ID, "username": username, "role": u.Role,
				}
			} else {
				entry.CreatedBy = *k.CreatedBy
			}
		}
		out = append(out, entry)
	}
	return out, nil
}
```

Verify that `models.APIKey` actually has `Name`, `Secret`, `CreatedAt`, `LastUpdatedAt`, `CreatedBy` fields with those exact names. If field naming differs, adjust.

Add to `admin.go`:

```go
func (h *AdminHandler) ListAPIKeys(c *gin.Context) {
	keys, err := h.apiKeySvc.ListWithUser(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"apiKey": nil, "error": "Could not find an API Keys."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKeys": keys, "error": nil})
}

func (h *AdminHandler) GenerateAPIKey(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	var body struct {
		Name *string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	createdBy := &currUser.ID
	key, err := h.apiKeySvc.Create(c.Request.Context(), createdBy, body.Name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"apiKey": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKey": key, "error": nil})
}

func (h *AdminHandler) DeleteAPIKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.apiKeySvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Status(http.StatusOK) // empty body, matches Node
}
```

Register:

```go
	r.GET("/admin/api-keys",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListAPIKeys)
	r.POST("/admin/generate-api-key",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.GenerateAPIKey)
	r.DELETE("/admin/delete-api-key/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeleteAPIKey)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin_APIKey -v`
Expected: PASS both subtests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/api_key_service.go backend/internal/handlers/admin.go backend/tests/integration/admin_test.go
git commit -m "feat(admin): API key list/generate/delete with createdBy hydration"
```

---

## Task 11: Full-suite sanity + main.go wire confirmation

After tasks 1-10 the code compiles and tests pass in isolation. This task verifies the whole package still builds and all admin integration tests run green together.

**Files:**
- Read only: all touched

- [ ] **Step 1: Run full admin integration tests**

Run: `cd backend && go test ./tests/integration/ -run TestAdmin -v`
Expected: PASS all 12+ subtests.

- [ ] **Step 2: Run full service tests**

Run: `cd backend && go test ./internal/services/ -v`
Expected: PASS (including pre-existing).

- [ ] **Step 3: Run full build**

Run: `cd backend && go build ./...`
Expected: no errors.

- [ ] **Step 4: Run go vet**

Run: `cd backend && go vet ./...`
Expected: no warnings.

- [ ] **Step 5: Manually verify route table**

Run: `cd backend && grep -E '"/admin/' internal/handlers/admin.go | grep -E '\.(GET|POST|DELETE)' | wc -l`
Expected: **16** (8 pre-existing + 8 added by this plan — note one was a path *fix* not an addition).

Manually list to confirm:
- `GET /admin/users` ✅
- `DELETE /admin/user/:id` ✅ (fixed path)
- `GET /admin/workspaces` ✅
- `POST /admin/invite/new` ✅
- `GET /admin/invites` ✅
- `DELETE /admin/invite/:id` ✅
- `GET /admin/system-preferences-for` ✅
- `POST /admin/system-preferences` ✅
- `POST /admin/users/new` (new)
- `POST /admin/user/:id` (new)
- `POST /admin/workspaces/new` (new)
- `GET /admin/workspaces/:workspaceId/users` (new)
- `POST /admin/workspaces/:workspaceId/update-users` (new)
- `DELETE /admin/workspaces/:id` (new)
- `GET /admin/api-keys` (new)
- `POST /admin/generate-api-key` (new)
- `DELETE /admin/delete-api-key/:id` (new)

Total = 17. (Original 8 + 9 new — the count check above lists 16 because the deleted `DELETE /admin/users/:id` is removed.)

- [ ] **Step 6: Commit (if any final adjustments)**

If no changes were needed, skip. Otherwise:

```bash
git add -p
git commit -m "chore(admin): final P4a wiring + cleanup"
```

---

## Known gaps & follow-up

These are intentionally NOT in P4a; document in commit messages and design doc updates:

1. **EventLog persistence** — Node logs `user_created`/`user_deleted`/`api_key_created`/`api_key_deleted`. Add when `models.EventLog` and `EventLogService` are created in a future sub-plan.
2. **BrowserExtensionApiKey cascade on user delete** — model not yet in Go.
3. **VectorDB namespace cleanup on workspace delete** — `_ = ws.Slug` in Task 9 marks the gap. Wired in P5 when VectorDBService is properly bound to AdminHandler.
4. **Telemetry** — none of the Node `Telemetry.sendTelemetry` calls are mirrored; out of scope.
5. **Workspace.whereWithUsers `userIds` field** — Node's `ListWorkspaces` returns each workspace with a `userIds: [...]` field. The Go `AdminService.ListWorkspaces` does not. Consumers of this list (admin frontend "manage users" picker) may still work via the dedicated `/admin/workspaces/:id/users` endpoint added in Task 7. If frontend testing reveals breakage, add this field as a follow-up.

---

*Plan version: v1.0 — 2026-05-25*
*Source spec: `.gpowers/designs/2026-05-22-backend-api-routes-design.md` §5.1*
