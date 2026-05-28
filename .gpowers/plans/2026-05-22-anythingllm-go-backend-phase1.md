# Hermind Go Backend Phase 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the Hermind Node.js/Express backend to Go (Gin + GORM + Pantheon + mlog), producing a single self-contained binary with 100% API compatibility for Phase 1 endpoints (Auth, System, Workspace, Chat SSE, Document, Admin).

**Architecture:** Layered architecture matching existing Express structure — `handlers/` (HTTP layer), `services/` (business logic), `models/` (GORM data layer), `middleware/` (auth/RBAC/workspace validation), `vectordb/` (pluggable vector DB interface). Frontend served via `//go:embed` with SPA fallback.

**Tech Stack:** Go 1.23, Gin, GORM (SQLite/PostgreSQL), Pantheon (LLM/embeddings), mlog (structured logging), `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `github.com/caarlos0/env/v11`.

---

## File Structure

Before tasks begin, here is the complete file map. Each file below will be created or modified by one or more tasks in this plan.

```
backend/
├── cmd/server/main.go
├── go.mod
├── Makefile
├── internal/
│   ├── config/config.go
│   ├── models/
│   │   ├── user.go
│   │   ├── workspace.go
│   │   ├── workspace_chat.go
│   │   ├── workspace_document.go
│   │   ├── document_vector.go
│   │   ├── system_setting.go
│   │   ├── workspace_user.go
│   │   ├── workspace_thread.go
│   │   ├── invite.go
│   │   ├── api_key.go
│   │   ├── password_reset_token.go
│   │   └── recovery_code.go
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── rbac.go
│   │   └── workspace.go
│   ├── dto/
│   │   ├── auth.go
│   │   ├── workspace.go
│   │   ├── chat.go
│   │   ├── document.go
│   │   └── system.go
│   ├── services/
│   │   ├── auth_service.go
│   │   ├── system_service.go
│   │   ├── workspace_service.go
│   │   ├── chat_service.go
│   │   ├── document_service.go
│   │   ├── vector_service.go
│   │   └── admin_service.go
│   ├── handlers/
│   │   ├── system.go
│   │   ├── auth.go
│   │   ├── workspace.go
│   │   ├── chat.go
│   │   ├── document.go
│   │   └── admin.go
│   ├── providers/
│   │   └── llm.go
│   ├── vectordb/
│   │   ├── interface.go
│   │   ├── lancedb.go
│   │   └── pgvector.go
│   └── static/
│       └── frontend.go
├── pkg/
│   └── utils/
│       ├── jwt.go
│       ├── bcrypt.go
│       ├── encryption.go
│       └── ptr.go
└── tests/
    ├── integration/
    │   ├── auth_test.go
    │   ├── workspace_test.go
    │   └── chat_test.go
    └── unit/
        └── auth_service_test.go
```

---

## Task Dependency Graph

```
Task 1 (Scaffold)
  → Task 2 (Config)
    → Task 3 (Logging)
      → Task 4 (Utils)
        → Task 5 (Models Group A)
        → Task 6 (Models Group B)
          → Task 7 (DB Init)
            → Task 8 (Auth Service)
              → Task 9 (Middleware)
                → Task 10 (System Handlers)
                → Task 11 (Auth Handlers)
                → Task 12 (Workspace Service+Handlers)
                → Task 13 (Vector Interface)
                  → Task 14 (LanceDB)
                  → Task 15 (PGVector)
                    → Task 16 (Chat Service)
                      → Task 17 (Chat Handlers)
                → Task 18 (Document Service+Handlers)
                → Task 19 (Admin Handlers)
              → Task 20 (Static Frontend)
            → Task 21 (Main Entry)
          → Task 22 (Integration Tests)
```

> Tasks without arrows between them can be executed in parallel by separate subagents.

---

### Task 1: Project Scaffold

**Files:**
- Create: `backend/go.mod`
- Create: `backend/Makefile`
- Create: `backend/cmd/server/main.go` (stub)

- [ ] **Step 1: Create go.mod**

Create `backend/go.mod`:

```go
module github.com/odysseythink/hermind/backend

go 1.23

require (
	github.com/caarlos0/env/v11 v11.3.1
	github.com/gin-gonic/gin v1.10.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/odysseythink/mlog v0.1.0
	github.com/odysseythink/pantheon v0.1.0
	golang.org/x/crypto v0.31.0
	gorm.io/driver/postgres v1.5.11
	gorm.io/driver/sqlite v1.5.7
	gorm.io/gorm v1.25.12
)
```

Run: `cd backend && go mod tidy`
Expected: `go.sum` generated, no errors.

- [ ] **Step 2: Create Makefile**

Create `backend/Makefile`:

```makefile
.PHONY: build build-frontend build-server dev test lint

FRONTEND_DIST := frontend/dist
GOFLAGS := -tags="fts5"

build-frontend:
	cd ../frontend && yarn install && yarn build

build-server: build-frontend
	cp -r ../frontend/dist ./frontend/dist
	go build $(GOFLAGS) -o ../hermind ./cmd/server/

dev:
	go run $(GOFLAGS) ./cmd/server/ -logtostderr

test:
	go test -v ./...

lint:
	golangci-lint run ./...
```

- [ ] **Step 3: Create directory structure**

Run:
```bash
cd backend && mkdir -p cmd/server internal/{config,models,middleware,dto,services,handlers,providers,vectordb,static} pkg/utils tests/{integration,unit}
```

- [ ] **Step 4: Create stub main.go**

Create `backend/cmd/server/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("Hermind Go server starting...")
}
```

Run: `cd backend && go run ./cmd/server/`
Expected: `Hermind Go server starting...`

- [ ] **Step 5: Commit**

```bash
git add backend/
git commit -m "feat: scaffold backend project structure"
```

---

### Task 2: Configuration Management

**Files:**
- Create: `backend/internal/config/config.go`
- Create: `backend/.env.example`

- [ ] **Step 1: Write config.go**

Create `backend/internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	ServerPort       string `env:"SERVER_PORT" envDefault:"3001"`
	StorageDir       string `env:"STORAGE_DIR" envDefault:"./storage"`
	JWTSecret        string `env:"JWT_SECRET" envDefault:"dev-secret-change-me"`
	SigKey           string `env:"SIG_KEY" envDefault:"dev-sig-key"`
	SigSalt          string `env:"SIG_SALT" envDefault:"dev-sig-salt"`
	AuthToken        string `env:"AUTH_TOKEN"`
	VectorDB         string `env:"VECTOR_DB" envDefault:"lancedb"`
	LLMProvider      string `env:"LLM_PROVIDER" envDefault:"openai"`
	LLMModel         string `env:"LLM_MODEL" envDefault:"gpt-4o-mini"`
	EmbeddingEngine  string `env:"EMBEDDING_ENGINE" envDefault:"openai"`
	EmbeddingModel   string `env:"EMBEDDING_MODEL" envDefault:"text-embedding-3-small"`
	TTSProvider      string `env:"TTS_PROVIDER" envDefault:"native"`
	EnableHTTPS      bool   `env:"ENABLE_HTTPS" envDefault:"false"`
	MultiUserMode    bool   `env:"MULTI_USER_MODE" envDefault:"false"`
	DebugMode        bool   `env:"DEBUG_MODE" envDefault:"false"`
	OpenAiKey        string `env:"OPEN_AI_KEY"`
	DatabaseURL      string `env:"DATABASE_URL"`
	CollectorURL     string `env:"COLLECTOR_URL" envDefault:"http://localhost:8888"`
	CommunicationKey string `env:"COMMUNICATION_KEY" envDefault:"hermind"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.StorageDir == "" {
		cfg.StorageDir = "./storage"
	}
	if err := os.MkdirAll(cfg.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &cfg, nil
}
```

- [ ] **Step 2: Write .env.example**

Create `backend/.env.example` with all env vars listed in the Config struct.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/config/config.go backend/.env.example
git commit -m "feat: add configuration management"
```

---

### Task 3: Logging Setup (mlog)

**Files:**
- Create: `backend/pkg/utils/logger.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write logger.go**

Create `backend/pkg/utils/logger.go`:

```go
package utils

import "github.com/odysseythink/mlog"

func InitLogger(logDir string) {
	mlog.SetEncoder(mlog.NewJSONEncoder())
	mlog.SetLogDir(logDir)
}

func SyncLogger() {
	mlog.Flush()
}
```

- [ ] **Step 2: Update main.go to call InitLogger**

Add to main.go after config.Load():
```go
logDir := cfg.StorageDir + "/logs"
os.MkdirAll(logDir, 0755)
utils.InitLogger(logDir)
defer utils.SyncLogger()
```

- [ ] **Step 3: Commit**

```bash
git add backend/pkg/utils/logger.go backend/cmd/server/main.go
git commit -m "feat: integrate mlog structured logging"
```

---

### Task 4: Utility Package

**Files:**
- Create: `backend/pkg/utils/ptr.go`
- Create: `backend/pkg/utils/bcrypt.go`
- Create: `backend/pkg/utils/jwt.go`
- Create: `backend/pkg/utils/encryption.go`

- [ ] **Step 1: Write ptr.go**

```go
package utils

func Ptr[T any](v T) *T { return &v }
```

- [ ] **Step 2: Write bcrypt.go**

```go
package utils

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

- [ ] **Step 3: Write jwt.go**

```go
package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateJWT(secret string, claims map[string]any, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	})
	for k, v := range claims {
		token.Claims.(jwt.MapClaims)[k] = v
	}
	return token.SignedString([]byte(secret))
}

func ParseJWT(secret string, tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token claims")
}
```

- [ ] **Step 4: Write encryption.go**

Create PEM-based AES-GCM encryption manager (see design doc section 6.1 for full logic):
- Generate/load RSA 2048 key pair from storage/encryption/
- Derive AES key from SHA256(private key PEM)
- Encrypt/Decrypt with AES-GCM

- [ ] **Step 5: Commit**

```bash
git add backend/pkg/utils/
git commit -m "feat: add utility package (jwt, bcrypt, encryption, ptr)"
```

---

### Task 5: GORM Models — Group A (Users, Auth, Invites)

**Files:**
- Create: `backend/internal/models/user.go`
- Create: `backend/internal/models/invite.go`
- Create: `backend/internal/models/api_key.go`
- Create: `backend/internal/models/password_reset_token.go`
- Create: `backend/internal/models/recovery_code.go`

- [ ] **Step 1-5: Write each model file**

See design doc section 4.2 for full model definitions with GORM tags and JSON tags.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/user.go backend/internal/models/invite.go backend/internal/models/api_key.go backend/internal/models/password_reset_token.go backend/internal/models/recovery_code.go
git commit -m "feat: add GORM models group A (users, auth, invites)"
```

---

### Task 6: GORM Models — Group B

**Files:**
- Create: `backend/internal/models/workspace.go`
- Create: `backend/internal/models/workspace_user.go`
- Create: `backend/internal/models/workspace_chat.go`
- Create: `backend/internal/models/workspace_document.go`
- Create: `backend/internal/models/document_vector.go`
- Create: `backend/internal/models/workspace_thread.go`
- Create: `backend/internal/models/system_setting.go`

- [ ] **Step 1-7: Write each model file**

See design doc section 4.2 for full model definitions.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/models/workspace*.go backend/internal/models/document_vector.go backend/internal/models/system_setting.go
git commit -m "feat: add GORM models group B (workspace, documents, chats, threads, settings)"
```

---

### Task 7: Database Connection, AutoMigrate, Seed

**Files:**
- Create: `backend/internal/services/db.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write db.go**

Create `backend/internal/services/db.go`:
- `NewDB(cfg)` — SQLite (default) or PostgreSQL via GORM
- `AutoMigrate(db)` — all 11 models
- `SeedDefaults(db)` — insert default system_settings if empty

- [ ] **Step 2: Update main.go**

Add after logging init:
```go
db, err := services.NewDB(cfg)
if err != nil { mlog.Fatal("failed to connect db", mlog.Err(err)) }
if err := services.AutoMigrate(db); err != nil { mlog.Fatal("failed to migrate db", mlog.Err(err)) }
if err := services.SeedDefaults(db); err != nil { mlog.Fatal("failed to seed db", mlog.Err(err)) }
mlog.Info("database initialized")
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/db.go backend/cmd/server/main.go
git commit -m "feat: add DB connection, AutoMigrate, and seed defaults"
```

---

### Task 8: Authentication Service

**Files:**
- Create: `backend/internal/services/auth_service.go`
- Create: `backend/internal/dto/auth.go`

- [ ] **Step 1: Write auth DTO**

```go
package dto

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User    any    `json:"user"`
	Token   string `json:"token"`
	Message string `json:"message,omitempty"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RequestTokenResponse struct {
	Token string `json:"token"`
}
```

- [ ] **Step 2: Write auth_service.go**

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
	enc *utils.EncryptionManager
}

func NewAuthService(db *gorm.DB, cfg *config.Config, enc *utils.EncryptionManager) *AuthService {
	return &AuthService{db: db, cfg: cfg, enc: enc}
}

func (s *AuthService) ValidateToken(ctx context.Context, tokenStr string) (*models.User, error) {
	if !s.cfg.MultiUserMode {
		return s.validateSingleUserToken(tokenStr)
	}
	return s.validateMultiUserSession(tokenStr)
}

func (s *AuthService) validateSingleUserToken(tokenStr string) (*models.User, error) {
	claims, err := utils.ParseJWT(s.cfg.JWTSecret, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	pVal, ok := claims["p"].(string)
	if !ok {
		return nil, fmt.Errorf("missing payload")
	}
	decrypted, err := s.enc.Decrypt(pVal)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}
	if !utils.CheckPassword(s.cfg.AuthToken, decrypted) {
		return nil, fmt.Errorf("token mismatch")
	}
	return &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"}, nil
}

func (s *AuthService) validateMultiUserSession(tokenStr string) (*models.User, error) {
	claims, err := utils.ParseJWT(s.cfg.JWTSecret, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}
	userID, ok := claims["userId"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing userId")
	}
	var user models.User
	if err := s.db.First(&user, int(userID)).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	var user models.User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !utils.CheckPassword(req.Password, user.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}
	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return &dto.LoginResponse{User: user, Token: token, Message: "ok"}, nil
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.LoginResponse, error) {
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user := models.User{Username: utils.Ptr(req.Username), Password: hash, Role: "default"}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return &dto.LoginResponse{User: user, Token: token, Message: "ok"}, nil
}

func (s *AuthService) CreateSingleUserToken(ctx context.Context) (string, error) {
	encrypted, err := s.enc.Encrypt(s.cfg.AuthToken)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}
	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"p": encrypted}, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/auth_service.go backend/internal/dto/auth.go
git commit -m "feat: add authentication service with single/multi-user modes"
```

---

### Task 9: Middleware Chain

**Files:**
- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/rbac.go`
- Create: `backend/internal/middleware/workspace.go`
- Create: `backend/internal/dto/error.go`

- [ ] **Step 1: Write error DTO**

```go
package dto

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
```

- [ ] **Step 2: Write auth middleware**

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/services"
)

func ValidatedRequest(authSvc *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "No auth token found"})
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		user, err := authSvc.ValidateToken(c.Request.Context(), tokenStr)
		if err != nil {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid auth token"})
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}
```

- [ ] **Step 3: Write RBAC middleware**

```go
package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
)

func FlexUserRoleValid(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "No user in context"})
			c.Abort()
			return
		}
		user := userVal.(*models.User)
		if !slices.Contains(allowed, user.Role) && !slices.Contains(allowed, "all") {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Write workspace middleware**

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

func ValidWorkspaceSlug(db *gorm.DB) gin.HandlerFunc {
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
		c.Next()
	}
}
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/ backend/internal/dto/error.go
git commit -m "feat: add middleware chain (auth, rbac, workspace slug)"
```

---

### Task 10: System Handlers

**Files:**
- Create: `backend/internal/services/system_service.go`
- Create: `backend/internal/handlers/system.go`
- Create: `backend/internal/dto/system.go`

- [ ] **Step 1: Write system_service.go**

```go
package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type SystemService struct {
	db    *gorm.DB
	cache *sync.Map
}

func NewSystemService(db *gorm.DB) *SystemService {
	return &SystemService{db: db, cache: &sync.Map{}}
}

func (s *SystemService) GetSetting(ctx context.Context, key string) (string, error) {
	if val, ok := s.cache.Load(key); ok {
		return val.(string), nil
	}
	var setting models.SystemSetting
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return "", fmt.Errorf("setting not found: %w", err)
	}
	s.cache.Store(key, setting.Value)
	return setting.Value, nil
}

func (s *SystemService) SetSetting(ctx context.Context, key, value string) error {
	var setting models.SystemSetting
	result := s.db.Where("key = ?", key).First(&setting)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			setting = models.SystemSetting{Key: key, Value: value}
			if err := s.db.Create(&setting).Error; err != nil {
				return err
			}
		} else {
			return result.Error
		}
	} else {
		setting.Value = value
		if err := s.db.Save(&setting).Error; err != nil {
			return err
		}
	}
	s.cache.Store(key, value)
	return nil
}

func (s *SystemService) IsSetupComplete(ctx context.Context) bool {
	val, err := s.GetSetting(ctx, "setup_complete")
	return err == nil && val == "true"
}

func (s *SystemService) GetAllSettings(ctx context.Context) (map[string]string, error) {
	var settings []models.SystemSetting
	if err := s.db.Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}
```

- [ ] **Step 2: Write system DTO**

```go
package dto

type SystemSettingsResponse struct {
	Settings map[string]string `json:"settings"`
}

type UpdateSettingRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PingResponse struct {
	Status string `json:"status"`
}
```

- [ ] **Step 3: Write system handler**

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type SystemHandler struct {
	sysSvc *services.SystemService
	cfg    *config.Config
}

func NewSystemHandler(sysSvc *services.SystemService, cfg *config.Config) *SystemHandler {
	return &SystemHandler{sysSvc: sysSvc, cfg: cfg}
}

func (h *SystemHandler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, dto.PingResponse{Status: "ok"})
}

func (h *SystemHandler) SetupComplete(c *gin.Context) {
	complete := h.sysSvc.IsSetupComplete(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"setupComplete": complete})
}

func (h *SystemHandler) GetSystemSettings(c *gin.Context) {
	settings, err := h.sysSvc.GetAllSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.SystemSettingsResponse{Settings: settings})
}

func (h *SystemHandler) UpdateSystemSetting(c *gin.Context) {
	var req dto.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.sysSvc.SetSetting(c.Request.Context(), req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterSystemRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, cfg *config.Config) {
	h := NewSystemHandler(sysSvc, cfg)
	r.GET("/ping", h.Ping)
	r.GET("/setup-complete", h.SetupComplete)
	r.GET("/system", h.GetSystemSettings)
	r.POST("/system", h.UpdateSystemSetting)
}
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/system_service.go backend/internal/handlers/system.go backend/internal/dto/system.go
git commit -m "feat: add system handlers (ping, setup, settings)"
```

---

### Task 11: Auth Handlers

**Files:**
- Create: `backend/internal/handlers/auth.go`

- [ ] **Step 1: Write auth handler**

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type AuthHandler struct {
	authSvc *services.AuthService
}

func NewAuthHandler(authSvc *services.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) RequestToken(c *gin.Context) {
	token, err := h.authSvc.CreateSingleUserToken(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.RequestTokenResponse{Token: token})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterAuthRoutes(r *gin.RouterGroup, authSvc *services.AuthService) {
	h := NewAuthHandler(authSvc)
	r.POST("/request-token", h.RequestToken)
	r.POST("/login", h.Login)
	r.POST("/register", h.Register)
	r.POST("/logout", h.Logout)
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/handlers/auth.go
git commit -m "feat: add auth handlers (login, register, request-token, logout)"
```

---

### Task 12: Workspace Service and Handlers

**Files:**
- Create: `backend/internal/services/workspace_service.go`
- Create: `backend/internal/handlers/workspace.go`
- Create: `backend/internal/dto/workspace.go`

- [ ] **Step 1: Write workspace DTO**

```go
package dto

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

type UpdateWorkspaceRequest struct {
	Name                 string   `json:"name,omitempty"`
	OpenAiTemp           *float64 `json:"openAiTemp,omitempty"`
	OpenAiHistory        int      `json:"openAiHistory,omitempty"`
	OpenAiPrompt         *string  `json:"openAiPrompt,omitempty"`
	SimilarityThreshold  *float64 `json:"similarityThreshold,omitempty"`
	ChatProvider         *string  `json:"chatProvider,omitempty"`
	ChatModel            *string  `json:"chatModel,omitempty"`
	TopN                 *int     `json:"topN,omitempty"`
	ChatMode             *string  `json:"chatMode,omitempty"`
	QueryRefusalResponse *string  `json:"queryRefusalResponse,omitempty"`
}

type WorkspaceListResponse struct {
	Workspaces []any `json:"workspaces"`
}
```

- [ ] **Step 2: Write workspace_service.go**

```go
package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type WorkspaceService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewWorkspaceService(db *gorm.DB, cfg *config.Config) *WorkspaceService {
	return &WorkspaceService{db: db, cfg: cfg}
}

func (s *WorkspaceService) Create(ctx context.Context, userID int, req dto.CreateWorkspaceRequest) (*models.Workspace, error) {
	slug := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-")) + "-" + uuid.New().String()[:8]
	ws := models.Workspace{
		Name:          req.Name,
		Slug:          slug,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&ws).Error; err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	wu := models.WorkspaceUser{
		WorkspaceID:   ws.ID,
		UserID:        userID,
		Role:          "admin",
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&wu).Error; err != nil {
		return nil, fmt.Errorf("create workspace user: %w", err)
	}
	return &ws, nil
}

func (s *WorkspaceService) List(ctx context.Context, userID int) ([]models.Workspace, error) {
	var wus []models.WorkspaceUser
	if err := s.db.Where("user_id = ?", userID).Preload("Workspace").Find(&wus).Error; err != nil {
		return nil, err
	}
	workspaces := make([]models.Workspace, 0, len(wus))
	for _, wu := range wus {
		workspaces = append(workspaces, wu.Workspace)
	}
	return workspaces, nil
}

func (s *WorkspaceService) GetBySlug(ctx context.Context, slug string) (*models.Workspace, error) {
	var ws models.Workspace
	if err := s.db.Where("slug = ?", slug).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

func (s *WorkspaceService) Update(ctx context.Context, slug string, req dto.UpdateWorkspaceRequest) error {
	var ws models.Workspace
	if err := s.db.Where("slug = ?", slug).First(&ws).Error; err != nil {
		return err
	}
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.OpenAiTemp != nil {
		updates["open_ai_temp"] = *req.OpenAiTemp
	}
	if req.OpenAiHistory > 0 {
		updates["open_ai_history"] = req.OpenAiHistory
	}
	if req.OpenAiPrompt != nil {
		updates["open_ai_prompt"] = *req.OpenAiPrompt
	}
	if req.SimilarityThreshold != nil {
		updates["similarity_threshold"] = *req.SimilarityThreshold
	}
	if req.ChatProvider != nil {
		updates["chat_provider"] = *req.ChatProvider
	}
	if req.ChatModel != nil {
		updates["chat_model"] = *req.ChatModel
	}
	if req.TopN != nil {
		updates["top_n"] = *req.TopN
	}
	if req.ChatMode != nil {
		updates["chat_mode"] = *req.ChatMode
	}
	if req.QueryRefusalResponse != nil {
		updates["query_refusal_response"] = *req.QueryRefusalResponse
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&ws).Updates(updates).Error
}

func (s *WorkspaceService) Delete(ctx context.Context, slug string) error {
	return s.db.Where("slug = ?", slug).Delete(&models.Workspace{}).Error
}

func (s *WorkspaceService) GetChats(ctx context.Context, workspaceID int) ([]models.WorkspaceChat, error) {
	var chats []models.WorkspaceChat
	if err := s.db.Where("workspace_id = ?", workspaceID).Order("id DESC").Find(&chats).Error; err != nil {
		return nil, err
	}
	return chats, nil
}
```

- [ ] **Step 3: Write workspace handler**

```go
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

type WorkspaceHandler struct {
	wsSvc *services.WorkspaceService
}

func NewWorkspaceHandler(wsSvc *services.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{wsSvc: wsSvc}
}

func (h *WorkspaceHandler) CreateWorkspace(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req dto.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	ws, err := h.wsSvc.Create(c.Request.Context(), user.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "message": "Workspace created"})
}

func (h *WorkspaceHandler) ListWorkspaces(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	workspaces, err := h.wsSvc.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.WorkspaceListResponse{Workspaces: []any{workspaces}})
}

func (h *WorkspaceHandler) GetWorkspace(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	c.JSON(http.StatusOK, gin.H{"workspace": ws})
}

func (h *WorkspaceHandler) UpdateWorkspace(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkspaceHandler) DeleteWorkspace(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.wsSvc.Delete(c.Request.Context(), ws.Slug); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkspaceHandler) GetWorkspaceChats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	chats, err := h.wsSvc.GetChats(c.Request.Context(), ws.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chats": chats})
}

func RegisterWorkspaceRoutes(r *gin.RouterGroup, wsSvc *services.WorkspaceService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewWorkspaceHandler(wsSvc)
	r.POST("/workspace/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.CreateWorkspace)
	r.GET("/workspace",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.ListWorkspaces)
	r.GET("/workspace/:slug",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.GetWorkspace)
	r.POST("/workspace/:slug/update",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UpdateWorkspace)
	r.DELETE("/workspace/:slug",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.DeleteWorkspace)
	r.GET("/workspace/:slug/chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.GetWorkspaceChats)
}
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/workspace_service.go backend/internal/handlers/workspace.go backend/internal/dto/workspace.go
git commit -m "feat: add workspace service and handlers (CRUD, chats)"
```


---

### Task 13: Vector Database Interface

**Files:**
- Create: `backend/internal/vectordb/interface.go`
- Create: `backend/internal/services/vector_service.go`

- [ ] **Step 1: Write interface.go**

```go
package vectordb

import "context"

type VectorChunk struct {
	ID       string
	Vector   []float32
	Metadata map[string]any
}

type SearchResult struct {
	DocId    string
	Text     string
	Score    float64
	Metadata map[string]any
}

type VectorDatabase interface {
	Name() string
	Connect(ctx context.Context) error
	Heartbeat(ctx context.Context) (map[string]any, error)
	AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error
	DeleteVectors(ctx context.Context, namespace string, docIds []string) error
	SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]SearchResult, error)
	Tables(ctx context.Context) ([]string, error)
	TotalVectors(ctx context.Context) (int64, error)
}
```

- [ ] **Step 2: Write vector_service.go**

```go
package services

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
)

type VectorService struct {
	cfg      *config.Config
	provider vectordb.VectorDatabase
}

func NewVectorService(cfg *config.Config) *VectorService {
	return &VectorService{cfg: cfg}
}

func (s *VectorService) Connect(ctx context.Context) error {
	return fmt.Errorf("not implemented: use Task 14 or 15")
}

func (s *VectorService) SetProvider(p vectordb.VectorDatabase) {
	s.provider = p
}

func (s *VectorService) SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]vectordb.SearchResult, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("vector provider not connected")
	}
	return s.provider.SimilaritySearch(ctx, namespace, query, topN)
}

func (s *VectorService) AddVectors(ctx context.Context, namespace string, chunks []vectordb.VectorChunk) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.AddVectors(ctx, namespace, chunks)
}

func (s *VectorService) DeleteVectors(ctx context.Context, namespace string, docIds []string) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.DeleteVectors(ctx, namespace, docIds)
}

func (s *VectorService) Heartbeat(ctx context.Context) (map[string]any, error) {
	if s.provider == nil {
		return map[string]any{"status": "not configured"}, nil
	}
	return s.provider.Heartbeat(ctx)
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/vectordb/interface.go backend/internal/services/vector_service.go
git commit -m "feat: add vector database interface and service stub"
```

---

### Task 14: LanceDB Implementation

**Files:**
- Create: `backend/internal/vectordb/lancedb.go`

- [ ] **Step 1: Write lancedb.go**

```go
package vectordb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type LanceDB struct {
	uri string
}

func NewLanceDB(storageDir string) *LanceDB {
	return &LanceDB{uri: filepath.Join(storageDir, "lancedb")}
}

func (l *LanceDB) Name() string { return "lancedb" }

func (l *LanceDB) Connect(ctx context.Context) error {
	return os.MkdirAll(l.uri, 0755)
}

func (l *LanceDB) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "lancedb", "uri": l.uri}, nil
}

func (l *LanceDB) Tables(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(l.uri)
	if err != nil {
		return nil, err
	}
	var tables []string
	for _, e := range entries {
		if e.IsDir() {
			tables = append(tables, e.Name())
		}
	}
	return tables, nil
}

func (l *LanceDB) TotalVectors(ctx context.Context) (int64, error) {
	return 0, nil
}

func (l *LanceDB) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	return fmt.Errorf("lancedb add vectors not yet implemented - placeholder for SDK integration")
}

func (l *LanceDB) DeleteVectors(ctx context.Context, namespace string, docIds []string) error {
	return fmt.Errorf("lancedb delete vectors not yet implemented - placeholder for SDK integration")
}

func (l *LanceDB) SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]SearchResult, error) {
	return nil, fmt.Errorf("lancedb similarity search not yet implemented - placeholder for SDK integration")
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/vectordb/lancedb.go
git commit -m "feat: add LanceDB vector provider stub"
```

---

### Task 15: PGVector Implementation

**Files:**
- Create: `backend/internal/vectordb/pgvector.go`

- [ ] **Step 1: Write pgvector.go**

```go
package vectordb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGVector struct {
	pool    *pgxpool.Pool
	connStr string
}

func NewPGVector(connStr string) *PGVector {
	return &PGVector{connStr: connStr}
}

func (p *PGVector) Name() string { return "pgvector" }

func (p *PGVector) Connect(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, p.connStr)
	if err != nil {
		return fmt.Errorf("pgx connect: %w", err)
	}
	p.pool = pool
	_, err = p.pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("create extension: %w", err)
	}
	return nil
}

func (p *PGVector) Heartbeat(ctx context.Context) (map[string]any, error) {
	var version string
	if err := p.pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return nil, err
	}
	return map[string]any{"name": "pgvector", "version": version}, nil
}

func (p *PGVector) Tables(ctx context.Context) ([]string, error) {
	rows, err := p.pool.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (p *PGVector) TotalVectors(ctx context.Context) (int64, error) {
	return 0, nil
}

func (p *PGVector) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	return fmt.Errorf("pgvector add vectors not yet implemented - placeholder for embedding integration")
}

func (p *PGVector) DeleteVectors(ctx context.Context, namespace string, docIds []string) error {
	return fmt.Errorf("pgvector delete vectors not yet implemented - placeholder")
}

func (p *PGVector) SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]SearchResult, error) {
	return nil, fmt.Errorf("pgvector similarity search not yet implemented - placeholder for embedding integration")
}
```

- [ ] **Step 2: Update go.mod**

Run: `cd backend && go get github.com/jackc/pgx/v5`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/vectordb/pgvector.go backend/go.mod backend/go.sum
git commit -m "feat: add PGVector provider stub"
```

---

### Task 16: Chat Service (Core)

**Files:**
- Create: `backend/internal/services/chat_service.go`
- Create: `backend/internal/dto/chat.go`
- Create: `backend/internal/providers/llm.go`

- [ ] **Step 1: Write chat DTO**

```go
package dto

type StreamChatRequest struct {
	Message     string   `json:"message"`
	Attachments []string `json:"attachments,omitempty"`
}

type StreamChatResponse struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"` // textResponseChunk, abort, finalize
	TextResponse *string `json:"textResponse,omitempty"`
	Sources      []any   `json:"sources,omitempty"`
	Close        bool    `json:"close"`
	Error        *string `json:"error,omitempty"`
}
```

- [ ] **Step 2: Write llm provider stub**

```go
package providers

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/config"
)

type LLMProvider struct {
	cfg *config.Config
}

func NewLLMProvider(cfg *config.Config) *LLMProvider {
	return &LLMProvider{cfg: cfg}
}

func (p *LLMProvider) GetModel(ctx context.Context, provider, model string) (any, error) {
	return nil, fmt.Errorf("Pantheon SDK not yet integrated - model provider=%s model=%s", provider, model)
}

func (p *LLMProvider) GetEmbeddingModel(ctx context.Context, engine, model string) (any, error) {
	return nil, fmt.Errorf("Pantheon embedding not yet integrated - engine=%s model=%s", engine, model)
}
```

- [ ] **Step 3: Write chat_service.go**

```go
package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

type ChatService struct {
	db        *gorm.DB
	cfg       *config.Config
	vectorSvc *VectorService
	llmProv   *providers.LLMProvider
}

func NewChatService(db *gorm.DB, cfg *config.Config, vectorSvc *VectorService, llmProv *providers.LLMProvider) *ChatService {
	return &ChatService{db: db, cfg: cfg, vectorSvc: vectorSvc, llmProv: llmProv}
}

func (s *ChatService) Stream(ctx context.Context, ws *models.Workspace, user *models.User, req dto.StreamChatRequest) (<-chan dto.StreamChatResponse, error) {
	msgID := uuid.New().String()
	out := make(chan dto.StreamChatResponse, 16)

	go func() {
		defer close(out)
		var fullText strings.Builder

		history, err := s.buildChatHistory(ctx, ws.ID, ws.OpenAiHistory)
		if err != nil {
			mlog.Error("build history failed", mlog.Err(err))
			out <- dto.StreamChatResponse{
				ID: msgID, Type: "abort",
				Close: true, Error: utils.Ptr(err.Error()),
			}
			return
		}

		var sources []any
		if s.vectorSvc.provider != nil {
			results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, req.Message, *ws.TopN)
			if err == nil {
				for _, r := range results {
					sources = append(sources, map[string]any{
						"docId":    r.DocId,
						"text":     r.Text,
						"score":    r.Score,
						"metadata": r.Metadata,
					})
				}
			}
		}

		simulated := "This is a placeholder streaming response. Pantheon SDK integration required for real LLM responses."
		for _, word := range strings.Split(simulated, " ") {
			fullText.WriteString(word + " ")
			out <- dto.StreamChatResponse{
				ID:           msgID,
				Type:         "textResponseChunk",
				TextResponse: utils.Ptr(word + " "),
				Sources:      sources,
			}
		}

		out <- dto.StreamChatResponse{
			ID:      msgID,
			Type:    "textResponseChunk",
			Close:   true,
			Sources: sources,
		}

		s.saveChatResponse(ctx, ws, user, req.Message, fullText.String())
	}()

	return out, nil
}

func (s *ChatService) buildChatHistory(ctx context.Context, workspaceID, limit int) ([]any, error) {
	var chats []models.WorkspaceChat
	if err := s.db.Where("workspace_id = ? AND include = ?", workspaceID, true).
		Order("id DESC").
		Limit(limit).
		Find(&chats).Error; err != nil {
		return nil, err
	}

	history := make([]any, 0, len(chats))
	for i := len(chats) - 1; i >= 0; i-- {
		c := chats[i]
		history = append(history, map[string]any{
			"role":    "user",
			"content": c.Prompt,
		})
		history = append(history, map[string]any{
			"role":    "assistant",
			"content": c.Response,
		})
	}
	return history, nil
}

func (s *ChatService) saveChatResponse(ctx context.Context, ws *models.Workspace, user *models.User, prompt, response string) {
	chat := models.WorkspaceChat{
		WorkspaceID:   ws.ID,
		UserID:        user.ID,
		Prompt:        prompt,
		Response:      response,
		Include:       true,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&chat).Error; err != nil {
		mlog.Error("save chat failed", mlog.Err(err))
	}
}

func (s *ChatService) GetSuggestedMessages(ctx context.Context, ws *models.Workspace) ([]string, error) {
	return []string{"Tell me more", "Can you summarize?", "What are the key points?"}, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/dto/chat.go backend/internal/providers/llm.go
git commit -m "feat: add chat service with simulated streaming (Pantheon integration pending)"
```

---

### Task 17: Chat Handlers (SSE Streaming)

**Files:**
- Create: `backend/internal/handlers/chat.go`

- [ ] **Step 1: Write chat handler**

```go
package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type ChatHandler struct {
	chatSvc *services.ChatService
}

func NewChatHandler(chatSvc *services.ChatService) *ChatHandler {
	return &ChatHandler{chatSvc: chatSvc}
}

func (h *ChatHandler) StreamChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ws := c.MustGet("workspace").(*models.Workspace)
	user := c.MustGet("user").(*models.User)

	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	stream, err := h.chatSvc.Stream(c.Request.Context(), ws, user, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	c.Stream(func(w io.Writer) bool {
		for chunk := range stream {
			if err := json.NewEncoder(w).Encode(chunk); err != nil {
				return false
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		return false
	})
}

func (h *ChatHandler) SuggestedMessages(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	msgs, err := h.chatSvc.GetSuggestedMessages(c.Request.Context(), ws)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestedMessages": msgs})
}

func RegisterChatRoutes(r *gin.RouterGroup, chatSvc *services.ChatService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewChatHandler(chatSvc)
	r.POST("/workspace/:slug/stream-chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.StreamChat)
	r.GET("/workspace/:slug/suggested-messages",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.SuggestedMessages)
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/handlers/chat.go
git commit -m "feat: add chat SSE handler with streaming response"
```

---

### Task 18: Document Service and Handlers

**Files:**
- Create: `backend/internal/services/document_service.go`
- Create: `backend/internal/handlers/document.go`
- Create: `backend/internal/dto/document.go`

- [ ] **Step 1: Write document DTO**

```go
package dto

type UploadDocumentResponse struct {
	Documents []any  `json:"documents"`
	Message   string `json:"message,omitempty"`
}
```

- [ ] **Step 2: Write document_service.go**

```go
package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type DocumentService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewDocumentService(db *gorm.DB, cfg *config.Config) *DocumentService {
	return &DocumentService{db: db, cfg: cfg}
}

func (s *DocumentService) SaveUpload(ctx context.Context, workspaceID int, fileHeader *multipart.FileHeader) (*models.WorkspaceDocument, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	docId := uuid.New().String()
	ext := filepath.Ext(fileHeader.Filename)
	destPath := filepath.Join(s.cfg.StorageDir, "documents", docId+ext)
	os.MkdirAll(filepath.Dir(destPath), 0755)

	dest, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return nil, err
	}

	doc := models.WorkspaceDocument{
		WorkspaceID:   workspaceID,
		DocId:         docId,
		Title:         fileHeader.Filename,
		DocPath:       destPath,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&doc).Error; err != nil {
		return nil, fmt.Errorf("save document record: %w", err)
	}
	return &doc, nil
}

func (s *DocumentService) GetByDocId(ctx context.Context, docId string) (*models.WorkspaceDocument, error) {
	var doc models.WorkspaceDocument
	if err := s.db.Where("doc_id = ?", docId).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *DocumentService) DeleteByDocId(ctx context.Context, docId string) error {
	return s.db.Where("doc_id = ?", docId).Delete(&models.WorkspaceDocument{}).Error
}

func (s *DocumentService) GetWorkspaceBySlug(ctx context.Context, slug string, ws *models.Workspace) error {
	return s.db.Where("slug = ?", slug).First(ws).Error
}
```

- [ ] **Step 3: Write document handler**

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type DocumentHandler struct {
	docSvc *services.DocumentService
}

func NewDocumentHandler(docSvc *services.DocumentService) *DocumentHandler {
	return &DocumentHandler{docSvc: docSvc}
}

func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	workspaceSlug := c.PostForm("workspaceSlug")
	var ws models.Workspace
	if err := h.docSvc.GetWorkspaceBySlug(c.Request.Context(), workspaceSlug, &ws); err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	doc, err := h.docSvc.SaveUpload(c.Request.Context(), ws.ID, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": doc, "message": "Document uploaded"})
}

func (h *DocumentHandler) GetDocument(c *gin.Context) {
	docId := c.Param("docId")
	doc, err := h.docSvc.GetByDocId(c.Request.Context(), docId)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": doc})
}

func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	docId := c.Param("docId")
	if err := h.docSvc.DeleteByDocId(c.Request.Context(), docId); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *DocumentHandler) AcceptedExtensions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"accepted": []string{
		".txt", ".md", ".pdf", ".docx", ".csv", ".json", ".html",
	}})
}

func RegisterDocumentRoutes(r *gin.RouterGroup, docSvc *services.DocumentService, authSvc *services.AuthService) {
	h := NewDocumentHandler(docSvc)
	r.POST("/document/upload",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.UploadDocument)
	r.GET("/document/:docId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.GetDocument)
	r.DELETE("/document/:docId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.DeleteDocument)
	r.GET("/document/accepted-extensions",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.AcceptedExtensions)
}
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/document_service.go backend/internal/handlers/document.go backend/internal/dto/document.go
git commit -m "feat: add document service and handlers (upload, get, delete)"
```

---

### Task 19: Admin Handlers

**Files:**
- Create: `backend/internal/services/admin_service.go`
- Create: `backend/internal/handlers/admin.go`

- [ ] **Step 1: Write admin_service.go**

```go
package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type AdminService struct {
	db *gorm.DB
}

func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

func (s *AdminService) ListUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *AdminService) DeleteUser(ctx context.Context, id int) error {
	return s.db.Delete(&models.User{}, id).Error
}

func (s *AdminService) ListWorkspaces(ctx context.Context) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := s.db.Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}
```

- [ ] **Step 2: Write admin handler**

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type AdminHandler struct {
	adminSvc *services.AdminService
}

func NewAdminHandler(adminSvc *services.AdminService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.adminSvc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.adminSvc.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ListWorkspaces(c *gin.Context) {
	workspaces, err := h.adminSvc.ListWorkspaces(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func RegisterAdminRoutes(r *gin.RouterGroup, adminSvc *services.AdminService, authSvc *services.AuthService) {
	h := NewAdminHandler(adminSvc)
	r.GET("/admin/users",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListUsers)
	r.DELETE("/admin/users/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeleteUser)
	r.GET("/admin/workspaces",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListWorkspaces)
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/admin_service.go backend/internal/handlers/admin.go
git commit -m "feat: add admin service and handlers (users, workspaces)"
```

---

### Task 20: Static Frontend Embed

**Files:**
- Create: `backend/internal/static/frontend.go`

- [ ] **Step 1: Write frontend.go**

```go
package static

import "embed"

//go:embed all:frontend/dist
var FrontendFS embed.FS
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/static/frontend.go
git commit -m "feat: add frontend embed directive"
```

---

### Task 21: Main Entry — Wire Everything Together

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write complete main.go**

```go
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/internal/static"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
)

func main() {
	logtostderr := flag.Bool("logtostderr", false, "log to stderr")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logDir := cfg.StorageDir + "/logs"
	os.MkdirAll(logDir, 0755)
	utils.InitLogger(logDir)
	defer utils.SyncLogger()

	if *logtostderr {
		mlog.SetLogDir("")
	}

	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	if err != nil {
		mlog.Fatal("failed to init encryption", mlog.Err(err))
	}

	db, err := services.NewDB(cfg)
	if err != nil {
		mlog.Fatal("failed to connect db", mlog.Err(err))
	}
	if err := services.AutoMigrate(db); err != nil {
		mlog.Fatal("failed to migrate db", mlog.Err(err))
	}
	if err := services.SeedDefaults(db); err != nil {
		mlog.Fatal("failed to seed db", mlog.Err(err))
	}

	authSvc := services.NewAuthService(db, cfg, enc)
	sysSvc := services.NewSystemService(db)
	wsSvc := services.NewWorkspaceService(db, cfg)
	vectorSvc := services.NewVectorService(cfg)
	llmProv := providers.NewLLMProvider(cfg)
	chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv)
	docSvc := services.NewDocumentService(db, cfg)
	adminSvc := services.NewAdminService(db)

	if cfg.VectorDB == "lancedb" {
		ldb := vectordb.NewLanceDB(cfg.StorageDir)
		if err := ldb.Connect(nil); err != nil {
			mlog.Warn("lancedb connect failed", mlog.Err(err))
		} else {
			vectorSvc.SetProvider(ldb)
		}
	}

	if !cfg.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		handlers.RegisterSystemRoutes(api, sysSvc, cfg)
		handlers.RegisterAuthRoutes(api, authSvc)
		handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db)
		handlers.RegisterChatRoutes(api, chatSvc, authSvc, db)
		handlers.RegisterDocumentRoutes(api, docSvc, authSvc)
		handlers.RegisterAdminRoutes(api, adminSvc, authSvc)
	}

	staticServer := http.FileServer(http.FS(static.FrontendFS))
	r.GET("/assets/*filepath", gin.WrapH(staticServer))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		index, err := static.FrontendFS.ReadFile("frontend/dist/index.html")
		if err != nil {
			c.JSON(500, gin.H{"error": "index.html not found"})
			return
		}
		c.Data(200, "text/html; charset=utf-8", index)
	})

	addr := ":" + cfg.ServerPort
	mlog.Info("server starting", mlog.String("addr", addr))
	if err := r.Run(addr); err != nil {
		mlog.Fatal("server failed", mlog.Err(err))
	}
}
```

- [ ] **Step 2: Build test**

Run: `cd backend && go build ./cmd/server/`
Expected: compilation succeeds.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: wire all services, handlers, and routes in main.go"
```

---

### Task 22: Integration Tests

**Files:**
- Create: `backend/tests/integration/auth_test.go`
- Create: `backend/tests/integration/workspace_test.go`
- Create: `backend/tests/integration/system_test.go`

- [ ] **Step 1: Write auth integration test**

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

func setupAuthRouter(t *testing.T) (*gin.Engine, *config.Config) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, _ := services.NewDB(cfg)
	services.AutoMigrate(db)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)

	r := gin.New()
	handlers.RegisterAuthRoutes(r.Group("/api"), authSvc)
	return r, cfg
}

func TestRegisterAndLogin(t *testing.T) {
	r, _ := setupAuthRouter(t)

	body, _ := json.Marshal(dto.RegisterRequest{Username: "alice", Password: "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var loginResp dto.LoginResponse
	body, _ = json.Marshal(dto.LoginRequest{Username: "alice", Password: "secret"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/login", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	assert.NotEmpty(t, loginResp.Token)
}
```

- [ ] **Step 2: Write workspace integration test**

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

func setupWorkspaceRouter(t *testing.T) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
	db, _ := services.NewDB(cfg)
	services.AutoMigrate(db)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	wsSvc := services.NewWorkspaceService(db, cfg)

	authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	loginResp, _ := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db)
	return r, loginResp.Token
}

func TestCreateAndListWorkspace(t *testing.T) {
	r, token := setupWorkspaceRouter(t)

	body, _ := json.Marshal(dto.CreateWorkspaceRequest{Name: "My Workspace"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
```

- [ ] **Step 3: Write system integration test**

```go
package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir()}
	db, _ := services.NewDB(cfg)
	services.AutoMigrate(db)
	sysSvc := services.NewSystemService(db)

	r := gin.New()
	handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ping", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./tests/integration/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add backend/tests/integration/
git commit -m "test: add auth, workspace, and system integration tests"
```

---

## Self-Review Checklist

### 1. Spec Coverage

| Spec Section | Task(s) |
|---|---|
| Project scaffold (go.mod, Makefile) | Task 1 |
| Config management | Task 2 |
| Logging (mlog) | Task 3 |
| Utils (JWT, bcrypt, encryption, ptr) | Task 4 |
| GORM models (28 models → 11 Go files) | Task 5, Task 6 |
| DB connection + AutoMigrate + seed | Task 7 |
| Auth service (single/multi-user) | Task 8 |
| Middleware (auth, rbac, workspace) | Task 9 |
| System handlers | Task 10 |
| Auth handlers | Task 11 |
| Workspace service + handlers | Task 12 |
| Vector DB interface | Task 13 |
| LanceDB stub | Task 14 |
| PGVector stub | Task 15 |
| Chat service (core + SSE) | Task 16 |
| Chat SSE handler | Task 17 |
| Document service + handlers | Task 18 |
| Admin handlers | Task 19 |
| Frontend embed | Task 20 |
| Main entry (wire everything) | Task 21 |
| Integration tests | Task 22 |

**No gaps identified.** All Phase 1 endpoints from the design doc are covered.

### 2. Placeholder Scan

- No `TBD`, `TODO`, `implement later` found in task steps.
- Vector DB implementations (LanceDB, PGVector) contain explicit `fmt.Errorf("...not yet implemented")` stubs — this is intentional per design doc risk mitigation (SDK availability). These are functional placeholders with clear upgrade paths.
- Pantheon SDK integration is stubbed in `providers/llm.go` — also intentional for Phase 1 MVP. The chat service provides a simulated streaming response so the SSE pipeline can be tested end-to-end.

### 3. Type Consistency

- `utils.Ptr[T]` used consistently across all services.
- `dto.ErrorResponse` used in all handlers.
- `models.User`, `models.Workspace` types match between middleware context setting and handler retrieval.
- `gin.Context` middleware chain ordering is consistent across all route registrations.

### 4. Known Limitations (Out of Phase 1 Scope)

1. **Pantheon SDK integration**: Real LLM streaming requires SDK integration in a follow-up task.
2. **Vector search**: LanceDB/PGVector `SimilaritySearch` needs embedding provider + SDK; stubs allow API compatibility testing.
3. **Collector communication**: Document processing pipeline to collector (port 8888) not yet implemented.
4. **WebSocket**: Agent WebSocket is Phase 2.
5. **File size limit**: Gin multipart parser defaults to 32MB; Node backend uses 3GB. Add `MaxMultipartMemory` configuration in Task 18 refinement if needed.

---

## Execution Handoff

**Plan complete and saved to `.gpowers/plans/2026-05-22-hermind-go-backend-phase1.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task, review between tasks, fast iteration. REQUIRED SUB-SKILL: gpowers:subagent-driven-development.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints for review. REQUIRED SUB-SKILL: gpowers:executing-plans.

**Which approach would you prefer?**
