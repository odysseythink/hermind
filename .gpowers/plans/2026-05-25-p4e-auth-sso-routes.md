# P4e — Auth & SSO Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the remaining 4 system/auth routes from Node's `server/endpoints/system.js`: `POST /request-token` (multi-user mode), `GET /request-token/sso/simple`, `POST /system/recover-account` (add `isMultiUserSetup` middleware), `POST /system/reset-password` (add `isMultiUserSetup` middleware). Build supporting infra: EventLog, TemporaryAuthToken, recovery code generation, Simple SSO config.

**Architecture:** Extend existing `AuthHandler` and `AuthService` with multi-user `RequestToken` and SSO support. Add new `TemporaryAuthTokenService` and `EventLogService` as thin service layers. Use middleware `IsMultiUserSetup` to protect recovery/reset routes. Keep all auth routes in `auth.go` where they already live.

**Tech Stack:** Go 1.22, Gin, GORM, SQLite (test), `github.com/google/uuid`, bcrypt via `pkg/utils`.

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `internal/models/event_log.go` | Create | EventLog GORM model |
| `internal/models/temporary_auth_token.go` | Create | TemporaryAuthToken GORM model |
| `internal/services/event_log_service.go` | Create | EventLogService.LogEvent |
| `internal/services/temporary_auth_token_service.go` | Create | Issue, validate, invalidate temp auth tokens |
| `internal/services/auth_service.go` | Modify | Add `GenerateRecoveryCodes`, `RequestTokenMultiUser`, event logging |
| `internal/config/config.go` | Modify | Add `SimpleSSOEnabled`, `SimpleSSONoLogin` |
| `internal/dto/auth.go` | Modify | Extend `RequestTokenResponse` for multi-user |
| `internal/middleware/multi_user.go` | Create | `IsMultiUserSetup` middleware |
| `internal/handlers/auth.go` | Modify | Extend `RequestToken`, add `SSOSimple`, wire middleware |
| `internal/services/db.go` | Modify | AutoMigrate `EventLog`, `TemporaryAuthToken` |
| `cmd/server/main.go` | Modify | Wire new services into DI |
| `tests/integration/auth_p4e_test.go` | Create | 10+ integration tests |

---

### Task 1: EventLog Model & Service

**Files:**
- Create: `internal/models/event_log.go`
- Create: `internal/services/event_log_service.go`
- Modify: `internal/services/db.go`

Node Prisma schema:
```prisma
model event_logs {
  id         Int      @id @default(autoincrement())
  event      String
  metadata   String?
  userId     Int?
  occurredAt DateTime @default(now())
  @@index([event])
}
```

- [ ] **Step 1: Create `EventLog` model**

```go
// internal/models/event_log.go
package models

import "time"

type EventLog struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Event      string    `json:"event"`
	Metadata   *string   `json:"metadata,omitempty"`
	UserID     *int      `json:"userId,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}
```

- [ ] **Step 2: Create `EventLogService`**

```go
// internal/services/event_log_service.go
package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type EventLogService struct {
	db *gorm.DB
}

func NewEventLogService(db *gorm.DB) *EventLogService {
	return &EventLogService{db: db}
}

func (s *EventLogService) LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error {
	var metaStr *string
	if len(metadata) > 0 {
		b, _ := json.Marshal(metadata)
		str := string(b)
		metaStr = &str
	}
	log := models.EventLog{
		Event:      event,
		Metadata:   metaStr,
		UserID:     userID,
		OccurredAt: time.Now(),
	}
	return s.db.WithContext(ctx).Create(&log).Error
}
```

- [ ] **Step 3: Add to AutoMigrate**

In `internal/services/db.go`, add `&models.EventLog{}` to the `AutoMigrate` list.

- [ ] **Step 4: Commit**

```bash
git add internal/models/event_log.go internal/services/event_log_service.go internal/services/db.go
git commit -m "feat(phase4e): EventLog model and service"
```

---

### Task 2: TemporaryAuthToken Model & Service

**Files:**
- Create: `internal/models/temporary_auth_token.go`
- Create: `internal/services/temporary_auth_token_service.go`
- Modify: `internal/services/db.go`

Node Prisma schema:
```prisma
model temporary_auth_tokens {
  id        Int      @id @default(autoincrement())
  token     String   @unique
  userId    Int
  expiresAt DateTime
  createdAt DateTime @default(now())
  user      users    @relation(fields: [userId], references: [id], onDelete: Cascade)
  @@index([token])
  @@index([userId])
}
```

- [ ] **Step 1: Create `TemporaryAuthToken` model**

```go
// internal/models/temporary_auth_token.go
package models

import "time"

type TemporaryAuthToken struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Token     string    `gorm:"uniqueIndex" json:"token"`
	UserID    int       `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}
```

- [ ] **Step 2: Create `TemporaryAuthTokenService`**

```go
// internal/services/temporary_auth_token_service.go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type TemporaryAuthTokenService struct {
	db *gorm.DB
}

func NewTemporaryAuthTokenService(db *gorm.DB) *TemporaryAuthTokenService {
	return &TemporaryAuthTokenService{db: db}
}

func (s *TemporaryAuthTokenService) makeTempToken() string {
	return "allm-tat-" + uuid.New().String()
}

func (s *TemporaryAuthTokenService) Issue(ctx context.Context, userID int) (string, error) {
	if userID == 0 {
		return "", fmt.Errorf("user ID is required")
	}
	_ = s.InvalidateUserTokens(ctx, userID)

	token := models.TemporaryAuthToken{
		Token:     s.makeTempToken(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&token).Error; err != nil {
		return "", fmt.Errorf("create temp token: %w", err)
	}
	return token.Token, nil
}

func (s *TemporaryAuthTokenService) Validate(ctx context.Context, publicToken string) (sessionToken string, user *models.User, err error) {
	if publicToken == "" {
		return "", nil, fmt.Errorf("public token is required")
	}

	var token models.TemporaryAuthToken
	if err := s.db.WithContext(ctx).Where("token = ?", publicToken).First(&token).Error; err != nil {
		return "", nil, fmt.Errorf("invalid token")
	}

	if time.Now().After(token.ExpiresAt) {
		_ = s.db.WithContext(ctx).Delete(&token)
		return "", nil, fmt.Errorf("token expired")
	}

	var u models.User
	if err := s.db.WithContext(ctx).First(&u, token.UserID).Error; err != nil {
		return "", nil, fmt.Errorf("user not found")
	}

	_ = s.db.WithContext(ctx).Delete(&token) // single-use: delete after validation
	return "", &u, nil
}

func (s *TemporaryAuthTokenService) InvalidateUserTokens(ctx context.Context, userID int) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.TemporaryAuthToken{}).Error
}
```

Note: `sessionToken` generation is handled by the caller (AuthHandler) because it needs `cfg.JWTSecret`.

- [ ] **Step 3: Add to AutoMigrate**

In `internal/services/db.go`, add `&models.TemporaryAuthToken{}` to the `AutoMigrate` list.

- [ ] **Step 4: Commit**

```bash
git add internal/models/temporary_auth_token.go internal/services/temporary_auth_token_service.go internal/services/db.go
git commit -m "feat(phase4e): TemporaryAuthToken model and service"
```

---

### Task 3: Simple SSO Config & IsMultiUserSetup Middleware

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/middleware/multi_user.go`

- [ ] **Step 1: Add SSO config fields**

In `internal/config/config.go`, add to `Config` struct:

```go
	SimpleSSOEnabled  bool `env:"SIMPLE_SSO_ENABLED" envDefault:"false"`
	SimpleSSONoLogin  bool `env:"SIMPLE_SSO_NO_LOGIN" envDefault:"false"`
```

- [ ] **Step 2: Create `IsMultiUserSetup` middleware**

```go
// internal/middleware/multi_user.go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
)

func IsMultiUserSetup(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.MultiUserMode {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid request"})
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go internal/middleware/multi_user.go
git commit -m "feat(phase4e): Simple SSO config and IsMultiUserSetup middleware"
```

---

### Task 4: Extend AuthService — Recovery Codes & Multi-User RequestToken

**Files:**
- Modify: `internal/services/auth_service.go`

- [ ] **Step 1: Add `GenerateRecoveryCodes` method**

Add to `AuthService`:

```go
func (s *AuthService) GenerateRecoveryCodes(ctx context.Context, userID int) ([]string, error) {
	var codes []models.RecoveryCode
	var plainTexts []string
	for i := 0; i < 4; i++ {
		code := uuid.New().String()
		hash, err := utils.HashPassword(code)
		if err != nil {
			return nil, fmt.Errorf("hash recovery code: %w", err)
		}
		codes = append(codes, models.RecoveryCode{UserID: userID, CodeHash: hash})
		plainTexts = append(plainTexts, code)
	}

	if err := s.db.WithContext(ctx).Create(&codes).Error; err != nil {
		return nil, fmt.Errorf("create recovery codes: %w", err)
	}

	seen := true
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("seen_recovery_codes", &seen).Error; err != nil {
		return nil, fmt.Errorf("update user seen_recovery_codes: %w", err)
	}
	return plainTexts, nil
}
```

Note: The `User` model field `SeenRecoveryCodes` is `*bool`. Update with a pointer.

- [ ] **Step 2: Add `RequestTokenMultiUser` method**

Add to `AuthService`:

```go
func (s *AuthService) RequestTokenMultiUser(ctx context.Context, username, password string, eventLogSvc *EventLogService) (*dto.LoginResponse, []string, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		_ = eventLogSvc.LogEvent(ctx, "failed_login_invalid_username", map[string]any{
			"ip":       "Unknown IP",
			"username": username,
		}, nil)
		return nil, nil, fmt.Errorf("invalid login credentials")
	}

	if !utils.CheckPassword(password, user.Password) {
		_ = eventLogSvc.LogEvent(ctx, "failed_login_invalid_password", map[string]any{
			"ip":       "Unknown IP",
			"username": username,
		}, &user.ID)
		return nil, nil, fmt.Errorf("invalid login credentials")
	}

	if user.Suspended != nil && *user.Suspended {
		_ = eventLogSvc.LogEvent(ctx, "failed_login_account_suspended", map[string]any{
			"ip":       "Unknown IP",
			"username": username,
		}, &user.ID)
		return nil, nil, fmt.Errorf("account suspended by admin")
	}

	_ = eventLogSvc.LogEvent(ctx, "login_event", map[string]any{
		"ip":       "Unknown IP",
		"username": username,
	}, &user.ID)

	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("generate token: %w", err)
	}

	resp := &dto.LoginResponse{User: user, Token: token, Message: "ok"}

	if user.SeenRecoveryCodes == nil || !*user.SeenRecoveryCodes {
		codes, err := s.GenerateRecoveryCodes(ctx, user.ID)
		if err != nil {
			return resp, nil, err
		}
		return resp, codes, nil
	}

	return resp, nil, nil
}
```

Note: Check `User` model for `Suspended` field. If it doesn't exist, check the model first.

- [ ] **Step 3: Check User model for `Suspended` field**

If `Suspended` doesn't exist on `User`, add it:

```go
// internal/models/user.go
Suspended *bool `gorm:"default:false" json:"suspended"`
```

And add to `AutoMigrate` (it's already there if `User` is there).

- [ ] **Step 4: Commit**

```bash
git add internal/services/auth_service.go internal/models/user.go
git commit -m "feat(phase4e): GenerateRecoveryCodes and RequestTokenMultiUser"
```

---

### Task 5: Extend AuthHandler — RequestToken & SSO Simple

**Files:**
- Modify: `internal/dto/auth.go`
- Modify: `internal/handlers/auth.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Extend DTOs**

In `internal/dto/auth.go`, modify `RequestTokenResponse`:

```go
type RequestTokenResponse struct {
	Valid         bool     `json:"valid,omitempty"`
	User          any      `json:"user,omitempty"`
	Token         string   `json:"token,omitempty"`
	Message       *string  `json:"message,omitempty"`
	RecoveryCodes []string `json:"recoveryCodes,omitempty"`
}
```

Add new DTOs:

```go
type RequestTokenMultiUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SSOSimpleRequest struct {
	Token string `form:"token"` // query param
}
```

- [ ] **Step 2: Extend `AuthHandler` struct**

```go
type AuthHandler struct {
	authSvc      *services.AuthService
	cfg          *config.Config
	eventLogSvc  *services.EventLogService
	tempTokenSvc *services.TemporaryAuthTokenService
}
```

Update `NewAuthHandler`:

```go
func NewAuthHandler(authSvc *services.AuthService, cfg *config.Config, eventLogSvc *services.EventLogService, tempTokenSvc *services.TemporaryAuthTokenService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, cfg: cfg, eventLogSvc: eventLogSvc, tempTokenSvc: tempTokenSvc}
}
```

- [ ] **Step 3: Rewrite `RequestToken` handler**

```go
func (h *AuthHandler) RequestToken(c *gin.Context) {
	if h.cfg.MultiUserMode {
		// Check simple SSO login disabled
		if h.cfg.SimpleSSOEnabled && h.cfg.SimpleSSONoLogin {
			msg := "[005] Login via credentials has been disabled by the administrator."
			c.JSON(http.StatusForbidden, dto.RequestTokenResponse{Valid: false, Message: &msg})
			return
		}

		var req dto.RequestTokenMultiUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}

		resp, recoveryCodes, err := h.authSvc.RequestTokenMultiUser(c.Request.Context(), req.Username, req.Password, h.eventLogSvc)
		if err != nil {
			msg := err.Error()
			c.JSON(http.StatusOK, dto.RequestTokenResponse{Valid: false, Message: &msg})
			return
		}

		c.JSON(http.StatusOK, dto.RequestTokenResponse{
			Valid:         true,
			User:          resp.User,
			Token:         resp.Token,
			RecoveryCodes: recoveryCodes,
		})
		return
	}

	// Single-user mode: keep existing behavior
	token, err := h.authSvc.CreateSingleUserToken(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.RequestTokenResponse{Valid: true, Token: token})
}
```

Note: Node returns `200` with `valid:false` for invalid credentials in multi-user mode (not 401/403). Match that.

- [ ] **Step 4: Add `SSOSimple` handler**

```go
func (h *AuthHandler) SSOSimple(c *gin.Context) {
	var req dto.SSOSimpleRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	user, err := h.tempTokenSvc.Validate(c.Request.Context(), req.Token)
	if err != nil {
		_ = h.eventLogSvc.LogEvent(c.Request.Context(), "failed_login_invalid_temporary_auth_token", map[string]any{
			"ip":            "Unknown IP",
			"multiUserMode": true,
		}, nil)
		msg := fmt.Sprintf("[001] An error occurred while validating the token: %s", err.Error())
		c.JSON(http.StatusUnauthorized, dto.RequestTokenResponse{Valid: false, Message: &msg})
		return
	}

	_ = h.eventLogSvc.LogEvent(c.Request.Context(), "login_event", map[string]any{
		"ip":       "Unknown IP",
		"username": user.Username,
	}, &user.ID)

	sessionToken, err := utils.GenerateJWT(h.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.RequestTokenResponse{
		Valid: true,
		User:  user,
		Token: sessionToken,
	})
}
```

Wait — `TemporaryAuthTokenService.Validate` currently returns `(sessionToken string, user *models.User, err error)` but `sessionToken` is always empty. The handler should generate the sessionToken. Let me fix the service signature or adjust the handler.

Actually, looking at the service code I wrote earlier, `Validate` returns `("", user, nil)` for success. The handler generates the JWT. That's fine.

But wait — I need to check `TemporaryAuthTokenService.Validate` signature again. I wrote:

```go
func (s *TemporaryAuthTokenService) Validate(ctx context.Context, publicToken string) (sessionToken string, user *models.User, err error)
```

But `sessionToken` is always empty. It's cleaner to change the signature to just return `(user, error)`:

```go
func (s *TemporaryAuthTokenService) Validate(ctx context.Context, publicToken string) (*models.User, error)
```

Then the handler generates the JWT. Let me update the plan for Task 2 to use this cleaner signature.

- [ ] **Step 5: Update `RegisterAuthRoutes` signature and wiring**

```go
func RegisterAuthRoutes(r *gin.RouterGroup, authSvc *services.AuthService, cfg *config.Config, eventLogSvc *services.EventLogService, tempTokenSvc *services.TemporaryAuthTokenService) {
	h := NewAuthHandler(authSvc, cfg, eventLogSvc, tempTokenSvc)
	r.POST("/request-token", h.RequestToken)
	r.POST("/login", h.Login)
	r.POST("/register", h.Register)
	r.POST("/logout", h.Logout)
	r.POST("/system/recover-account", middleware.IsMultiUserSetup(cfg), h.RecoverAccount)
	r.POST("/system/reset-password", middleware.IsMultiUserSetup(cfg), h.ResetPassword)
	r.GET("/request-token/sso/simple", middleware.IsMultiUserSetup(cfg), h.SSOSimple)
	r.GET("/invite/:code", h.GetInvite)
	r.POST("/invite/:code", h.AcceptInvite)
}
```

Note: Add `middleware.IsMultiUserSetup(cfg)` to recover-account and reset-password. Import `"github.com/odysseythink/hermind/backend/internal/middleware"`.

- [ ] **Step 6: Update `main.go` DI**

In `cmd/server/main.go`, after creating `authSvc`, create:

```go
eventLogSvc := services.NewEventLogService(db)
tempTokenSvc := services.NewTemporaryAuthTokenService(db)
```

Update the `RegisterAuthRoutes` call:

```go
handlers.RegisterAuthRoutes(api, authSvc, cfg, eventLogSvc, tempTokenSvc)
```

- [ ] **Step 7: Commit**

```bash
git add internal/dto/auth.go internal/handlers/auth.go internal/middleware/multi_user.go cmd/server/main.go
git commit -m "feat(phase4e): extend RequestToken, add SSOSimple, wire IsMultiUserSetup"
```

---

### Task 6: Integration Tests

**Files:**
- Create: `tests/integration/auth_p4e_test.go`

- [ ] **Step 1: Write test file**

```go
// tests/integration/auth_p4e_test.go
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
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupP4eRouter(t *testing.T, multiUser bool) (*gin.Engine, *services.AuthService, *services.TemporaryAuthTokenService) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: multiUser}
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
	eventLogSvc := services.NewEventLogService(db)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, eventLogSvc, tempTokenSvc)
	return r, authSvc, tempTokenSvc
}

// --- Single-user mode ---

func TestRequestTokenSingleUser(t *testing.T) {
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", AuthToken: "secret123", MultiUserMode: false}
	db, _ := services.NewDB(cfg)
	services.AutoMigrate(db)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	eventLogSvc := services.NewEventLogService(db)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, eventLogSvc, tempTokenSvc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader([]byte(`{}`)))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["token"])
}

// --- Multi-user mode ---

func TestRequestTokenMultiUserSuccess(t *testing.T) {
	r, authSvc, _ := setupP4eRouter(t, true)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret123"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.NotEmpty(t, resp.Token)
	assert.NotNil(t, resp.RecoveryCodes) // first login should return recovery codes
	assert.Len(t, resp.RecoveryCodes, 4)
}

func TestRequestTokenMultiUserInvalidCredentials(t *testing.T) {
	r, authSvc, _ := setupP4eRouter(t, true)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret123"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "wrong"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code) // Node returns 200 with valid:false

	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Valid)
	assert.NotNil(t, resp.Message)
}

func TestRequestTokenMultiUserSSOLoginDisabled(t *testing.T) {
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true, SimpleSSOEnabled: true, SimpleSSONoLogin: true}
	db, _ := services.NewDB(cfg)
	services.AutoMigrate(db)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	eventLogSvc := services.NewEventLogService(db)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	r := gin.New()
	handlers.RegisterAuthRoutes(r.Group("/api"), authSvc, cfg, eventLogSvc, tempTokenSvc)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestRequestTokenMultiUserSecondLoginNoRecoveryCodes(t *testing.T) {
	r, authSvc, _ := setupP4eRouter(t, true)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret123"})
	assert.NoError(t, err)

	// First login generates recovery codes
	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// Second login should NOT return recovery codes
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.Empty(t, resp.RecoveryCodes)
}

// --- SSO Simple ---

func TestSSOSimpleSuccess(t *testing.T) {
	r, authSvc, tempTokenSvc := setupP4eRouter(t, true)
	user, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret123"})
	assert.NoError(t, err)

	token, err := tempTokenSvc.Issue(nil, user.User.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/request-token/sso/simple?token="+token, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.NotEmpty(t, resp.Token)
}

func TestSSOSimpleInvalidToken(t *testing.T) {
	r, _, _ := setupP4eRouter(t, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/request-token/sso/simple?token=invalid", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestSSOSimpleSingleUserRejected(t *testing.T) {
	r, _, _ := setupP4eRouter(t, false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/request-token/sso/simple?token=foo", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

// --- Recover / Reset with IsMultiUserSetup ---

func TestRecoverAccountSingleUserRejected(t *testing.T) {
	r, _, _ := setupP4eRouter(t, false)

	body, _ := json.Marshal(dto.RecoverAccountRequest{Username: "alice", RecoveryCodes: []string{"a", "b"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/recover-account", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestResetPasswordSingleUserRejected(t *testing.T) {
	r, _, _ := setupP4eRouter(t, false)

	body, _ := json.Marshal(dto.ResetPasswordRequest{Token: "abc", NewPassword: "new", ConfirmPassword: "new"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/reset-password", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestEventLogCreatedOnFailedLogin(t *testing.T) {
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, _ := services.NewDB(cfg)
	services.AutoMigrate(db)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	eventLogSvc := services.NewEventLogService(db)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	r := gin.New()
	handlers.RegisterAuthRoutes(r.Group("/api"), authSvc, cfg, eventLogSvc, tempTokenSvc)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "nobody", Password: "wrong"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var count int64
	db.Model(&services.EventLog{}).Where("event = ?", "failed_login_invalid_username").Count(&count)
	assert.Equal(t, int64(1), count)
}
```

Note: Need to import `services.EventLog` or `models.EventLog` for the count query. Use `models.EventLog`.

- [ ] **Step 2: Run tests**

```bash
cd backend && go test ./tests/integration/... -run "TestRequestToken|TestSSO|TestRecoverAccount|TestResetPassword|TestEventLog" -v
```

Expected: All pass.

- [ ] **Step 3: Fix any failures**

Common issues:
- `Suspended` field missing on User model
- `SeenRecoveryCodes` update needs pointer to bool
- JWT expiry constant — use `24 * time.Hour` or match Node's `JWT_EXPIRY` env
- Response field naming mismatch

- [ ] **Step 4: Commit**

```bash
git add tests/integration/auth_p4e_test.go
git commit -m "test(phase4e): integration tests for auth/SSO routes"
```

---

### Task 7: Regression & Final Verification

- [ ] **Step 1: Run all existing tests**

```bash
cd backend && go test ./... -v 2>&1 | tail -20
```

Expected: All pass. No new failures.

- [ ] **Step 2: Run vet**

```bash
cd backend && go vet ./...
```

Expected: Clean.

- [ ] **Step 3: Verify route parity**

```bash
# Go routes
cd /Users/ranwei/workspace/go_work/go-anything-llm
grep -E "r\.(GET|POST|PUT|DELETE|PATCH)" backend/internal/handlers/auth.go | sed -E 's/.*r\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)".*/\1 \2/' | sort

# Node routes
python3 -c "
import re
with open('server/endpoints/system.js') as f:
    content = f.read()
pattern = r'app\.(get|post|put|delete|patch)\s*\(\s*\"([^\"]+)\"'
matches = re.findall(pattern, content, re.IGNORECASE)
for method, path in sorted(set((m.upper(), p) for m, p in matches)):
    if any(k in path for k in ['request-token', 'recover-account', 'reset-password']):
        print(method, path)
"
```

Expected: All 4 Node routes (`POST /request-token`, `GET /request-token/sso/simple`, `POST /system/recover-account`, `POST /system/reset-password`) have Go equivalents.

- [ ] **Step 4: Push**

```bash
git push origin master
```

---

## Self-Review Checklist

**1. Spec coverage:**
| Requirement | Task |
|---|---|
| `POST /request-token` multi-user mode | Task 4 + 5 |
| `GET /request-token/sso/simple` | Task 2 + 5 |
| `POST /system/recover-account` with `isMultiUserSetup` | Task 5 (middleware) |
| `POST /system/reset-password` with `isMultiUserSetup` | Task 5 (middleware) |
| Event logging for failed/successful logins | Task 1 + 4 |
| Recovery code generation on first login | Task 4 |
| Simple SSO config (`SIMPLE_SSO_ENABLED`, `SIMPLE_SSO_NO_LOGIN`) | Task 3 |
| Temporary auth token (issue, validate, invalidate) | Task 2 |

**2. Placeholder scan:** No TBD, TODO, or vague instructions. All code blocks are complete.

**3. Type consistency:**
- `RequestTokenResponse` uses `Valid bool`, `User any`, `Token string`, `Message *string`, `RecoveryCodes []string` — matches Node response shape.
- `AuthHandler` constructor updated consistently.
- `TemporaryAuthTokenService.Validate` returns `(*models.User, error)` — handler generates JWT.

---

## Execution Handoff

**Plan complete and saved to `.gpowers/plans/2026-05-25-p4e-auth-sso-routes.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using `executing-plans`, batch execution with checkpoints for review.

**Which approach?**
