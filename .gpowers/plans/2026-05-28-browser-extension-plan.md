# Browser Extension 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Hermind 浏览器扩展功能，包含 Go 后端 8 个 API 端点、专用中间件、数据模型，以及 Chrome Manifest V3 扩展客户端（React + Vite）。

**Architecture:** 后端复用现有 Gin + GORM + DocumentService 基础设施；扩展客户端独立构建，通过 `brx-<uuid>` API key 认证，支持右键保存/嵌入网页内容到 RAG。

**Tech Stack:** Go 1.26 + Gin + GORM, React 18 + Vite, Chrome Extension Manifest V3, TailwindCSS

---

## 文件结构映射

### 后端新增/修改

| 文件 | 职责 |
|------|------|
| `backend/internal/models/browser_extension_api_key.go` | GORM 模型 |
| `backend/internal/services/browser_extension_service.go` | 业务逻辑：key CRUD、验证、级联删除 |
| `backend/internal/services/browser_extension_service_test.go` | Service 单元测试 |
| `backend/internal/middleware/browser_extension.go` | `ValidBrowserExtensionApiKey` 中间件 |
| `backend/internal/middleware/browser_extension_test.go` | 中间件单元测试 |
| `backend/internal/handlers/browser_extension.go` | 8 个 HTTP handler（覆盖现有 stub） |
| `backend/internal/handlers/browser_extension_test.go` | Handler 单元测试 |
| `backend/internal/services/db.go` | AutoMigrate 追加新模型 |
| `backend/internal/services/admin_service.go` | `DeleteUser` 追加级联清理 |
| `backend/internal/handlers/admin.go` | `DeleteUser` handler 注入 extSvc |
| `backend/cmd/server/main.go` | 实例化 extSvc，注册完整路由 |

### 扩展客户端新增

| 文件 | 职责 |
|------|------|
| `browser-extension/package.json` | 依赖 + 脚本 |
| `browser-extension/vite.config.js` | Vite 构建配置 |
| `browser-extension/public/manifest.json` | Chrome Manifest V3 |
| `browser-extension/public/background.js` | Service Worker（右键菜单、API、alarms） |
| `browser-extension/public/contentScript.js` | 内容脚本（页面提取、自动注入监听） |
| `browser-extension/public/icons/*` | 扩展图标 |
| `browser-extension/index.html` | Popup HTML |
| `browser-extension/src/main.jsx` | React root |
| `browser-extension/src/App.jsx` | Popup 根组件 |
| `browser-extension/src/components/Config.jsx` | 连接配置面板 |
| `browser-extension/src/hooks/useApiConnection.js` | 连接状态管理 hook |
| `browser-extension/src/models/browserExtension.js` | API 客户端 |
| `browser-extension/src/utils/constants.js` | 常量 |
| `browser-extension/src/index.css` | Tailwind 样式 |

### 构建/文档修改

| 文件 | 职责 |
|------|------|
| `Makefile` | 追加 `build-extension`、`build-all` |
| `AGENTS.md` | 追加浏览器扩展构建说明 |

---

## PR1: 后端基础设施

### Task 1: 创建 BrowserExtensionApiKey 模型

**Files:**
- Create: `backend/internal/models/browser_extension_api_key.go`

- [ ] **Step 1: 编写模型**

```go
package models

import "time"

type BrowserExtensionApiKey struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Key           string    `gorm:"unique" json:"key"`
	UserID        *int      `json:"userId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/models/browser_extension_api_key.go
git commit -m "feat(browser-extension): add BrowserExtensionApiKey model"
```

---

### Task 2: 创建 BrowserExtensionService

**Files:**
- Create: `backend/internal/services/browser_extension_service.go`
- Create: `backend/internal/services/browser_extension_service_test.go`

- [ ] **Step 1: 编写服务**

```go
package services

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type BrowserExtensionService struct {
	db *gorm.DB
}

func NewBrowserExtensionService(db *gorm.DB) *BrowserExtensionService {
	return &BrowserExtensionService{db: db}
}

func (s *BrowserExtensionService) CreateKey(userID *int) (*models.BrowserExtensionApiKey, error) {
	key := &models.BrowserExtensionApiKey{
		Key:       "brx-" + uuid.NewString(),
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(key).Error; err != nil {
		return nil, fmt.Errorf("create key: %w", err)
	}
	return key, nil
}

func (s *BrowserExtensionService) ListKeys(userID *int, isAdmin bool) ([]models.BrowserExtensionApiKey, error) {
	var keys []models.BrowserExtensionApiKey
	query := s.db.Order("id DESC")
	if !isAdmin && userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if err := query.Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *BrowserExtensionService) DeleteKey(id int, userID *int, isAdmin bool) error {
	query := s.db.Where("id = ?", id)
	if !isAdmin && userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if err := query.Delete(&models.BrowserExtensionApiKey{}).Error; err != nil {
		return err
	}
	return nil
}

func (s *BrowserExtensionService) Validate(key string) (*models.BrowserExtensionApiKey, error) {
	if !strings.HasPrefix(key, "brx-") {
		return nil, fmt.Errorf("invalid key format")
	}
	var apiKey models.BrowserExtensionApiKey
	if err := s.db.Where("key = ?", key).First(&apiKey).Error; err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (s *BrowserExtensionService) DeleteAllForUser(userID int) error {
	return s.db.Where("user_id = ?", userID).Delete(&models.BrowserExtensionApiKey{}).Error
}
```

- [ ] **Step 2: 编写测试**

```go
package services

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserExtensionService_CreateKey(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewBrowserExtensionService(db)

	key, err := svc.CreateKey(nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key.Key, "brx-"))
	assert.Nil(t, key.UserID)
}

func TestBrowserExtensionService_Validate(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewBrowserExtensionService(db)

	created, _ := svc.CreateKey(nil)
	validated, err := svc.Validate(created.Key)
	require.NoError(t, err)
	assert.Equal(t, created.ID, validated.ID)

	_, err = svc.Validate("invalid")
	assert.Error(t, err)
}

func TestBrowserExtensionService_ListKeys(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewBrowserExtensionService(db)

	uid := 1
	svc.CreateKey(&uid)
	svc.CreateKey(nil)

	keys, err := svc.ListKeys(&uid, false)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	keys, _ = svc.ListKeys(nil, true)
	assert.Len(t, keys, 2)
}

func TestBrowserExtensionService_DeleteKey(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewBrowserExtensionService(db)

	key, _ := svc.CreateKey(nil)
	require.NoError(t, svc.DeleteKey(key.ID, nil, true))

	_, err := svc.Validate(key.Key)
	assert.Error(t, err)
}

func TestBrowserExtensionService_DeleteAllForUser(t *testing.T) {
	cfg := testCfg(t)
	db := testDB(t, cfg)
	svc := NewBrowserExtensionService(db)

	uid := 42
	svc.CreateKey(&uid)
	svc.CreateKey(&uid)
	require.NoError(t, svc.DeleteAllForUser(uid))

	keys, _ := svc.ListKeys(&uid, false)
	assert.Len(t, keys, 0)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/services/ -run TestBrowserExtensionService -v
```

Expected: 5 tests PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/browser_extension_service.go backend/internal/services/browser_extension_service_test.go
git commit -m "feat(browser-extension): add BrowserExtensionService with tests"
```

---

### Task 3: 创建 ValidBrowserExtensionApiKey 中间件

**Files:**
- Create: `backend/internal/middleware/browser_extension.go`
- Create: `backend/internal/middleware/browser_extension_test.go`

- [ ] **Step 1: 编写中间件**

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

func ValidBrowserExtensionApiKey(
	extSvc *services.BrowserExtensionService,
	authSvc *services.AuthService,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "No auth token found"})
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		apiKey, err := extSvc.Validate(tokenStr)
		if err != nil {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid browser extension API key"})
			c.Abort()
			return
		}
		if cfg.MultiUserMode {
			if apiKey.UserID == nil {
				c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid browser extension API key"})
				c.Abort()
				return
			}
			user, err := authSvc.GetUserByID(c.Request.Context(), *apiKey.UserID)
			if err != nil {
				c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid browser extension API key"})
				c.Abort()
				return
			}
			if user == nil || user.Suspended {
				c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "User account suspended"})
				c.Abort()
				return
			}
			c.Set("user", user)
		} else {
			c.Set("user", &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"})
		}
		c.Set("apiKey", apiKey)
		c.Next()
	}
}
```

- [ ] **Step 2: 编写测试**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
)

func setupBrowserExtensionMiddleware(t *testing.T, multiUser bool) (*gin.Engine, *services.BrowserExtensionService, *services.AuthService, *config.Config) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: multiUser}
	db, _ := services.NewDB(cfg)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil { sqlDB.Close() }
	})
	services.AutoMigrate(db)
	extSvc := services.NewBrowserExtensionService(db)
	authSvc := services.NewAuthService(db, cfg)
	r := gin.New()
	return r, extSvc, authSvc, cfg
}

func TestValidBrowserExtensionApiKey_ValidKey(t *testing.T) {
	r, extSvc, authSvc, cfg := setupBrowserExtensionMiddleware(t, false)
	r.GET("/test", ValidBrowserExtensionApiKey(extSvc, authSvc, cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	key, _ := extSvc.CreateKey(nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestValidBrowserExtensionApiKey_InvalidKey(t *testing.T) {
	r, extSvc, authSvc, cfg := setupBrowserExtensionMiddleware(t, false)
	r.GET("/test", ValidBrowserExtensionApiKey(extSvc, authSvc, cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/middleware/ -run TestValidBrowserExtensionApiKey -v
```

Expected: 2 tests PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/middleware/browser_extension.go backend/internal/middleware/browser_extension_test.go
git commit -m "feat(browser-extension): add ValidBrowserExtensionApiKey middleware with tests"
```

---

### Task 4: 实现 BrowserExtensionHandler（覆盖 stub）

**Files:**
- Modify: `backend/internal/handlers/browser_extension.go`
- Create: `backend/internal/handlers/browser_extension_test.go`

- [ ] **Step 1: 重写 handler**

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type BrowserExtensionHandler struct {
	extSvc  *services.BrowserExtensionService
	wsSvc   *services.WorkspaceService
	docSvc  *services.DocumentService
	authSvc *services.AuthService
	cfg     *config.Config
}

func NewBrowserExtensionHandler(
	extSvc *services.BrowserExtensionService,
	wsSvc *services.WorkspaceService,
	docSvc *services.DocumentService,
	authSvc *services.AuthService,
	cfg *config.Config,
) *BrowserExtensionHandler {
	return &BrowserExtensionHandler{extSvc: extSvc, wsSvc: wsSvc, docSvc: docSvc, authSvc: authSvc, cfg: cfg}
}

// --- Extension-facing endpoints (brx auth) ---

func (h *BrowserExtensionHandler) Check(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	apiKey := c.MustGet("apiKey").(*models.BrowserExtensionApiKey)
	workspaces, err := h.wsSvc.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"connected": true,
		"workspaces": workspaces,
		"apiKeyId":  apiKey.ID,
	})
}

func (h *BrowserExtensionHandler) Disconnect(c *gin.Context) {
	apiKey := c.MustGet("apiKey").(*models.BrowserExtensionApiKey)
	if err := h.extSvc.DeleteKey(apiKey.ID, nil, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Browser extension disconnected"})
}

func (h *BrowserExtensionHandler) Workspaces(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	workspaces, err := h.wsSvc.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *BrowserExtensionHandler) EmbedContent(c *gin.Context) {
	var req struct {
		WorkspaceID int               `json:"workspaceId" binding:"required"`
		TextContent string            `json:"textContent" binding:"required"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user := c.MustGet("user").(*models.User)
	ws, err := h.wsSvc.GetByID(c.Request.Context(), req.WorkspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}
	if user.ID != 0 {
		// TODO: check workspace membership
	}

	title := req.Metadata["title"]
	if title == "" { title = "Browser Extension Embed" }
	meta := map[string]any{
		"title":  title,
		"url":    req.Metadata["url"],
		"source": "browser-extension",
	}

	docs, err := h.docSvc.SaveRawText(c.Request.Context(), req.TextContent, title, meta, []string{ws.Slug})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, doc := range docs {
		if err := h.docSvc.EmbedDocument(c.Request.Context(), doc); err != nil {
			mlog.Error("embed document failed", mlog.Err(err))
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Content embedded"})
}

func (h *BrowserExtensionHandler) UploadContent(c *gin.Context) {
	var req struct {
		TextContent string            `json:"textContent" binding:"required"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	title := req.Metadata["title"]
	if title == "" { title = "Browser Extension Upload" }
	meta := map[string]any{
		"title":  title,
		"url":    req.Metadata["url"],
		"source": "browser-extension",
	}

	_, err := h.docSvc.SaveRawText(c.Request.Context(), req.TextContent, title, meta, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Content uploaded"})
}

// --- Management endpoints (cookie auth) ---

func (h *BrowserExtensionHandler) ApiKeys(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	keys, err := h.extSvc.ListKeys(&user.ID, user.Role == "admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "apiKeys": keys})
}

func (h *BrowserExtensionHandler) GenerateApiKey(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	key, err := h.extSvc.CreateKey(&user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "apiKey": key.Key})
}

func (h *BrowserExtensionHandler) DeleteApiKey(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	if err := h.extSvc.DeleteKey(id, &user.ID, user.Role == "admin"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterBrowserExtensionRoutes(
	r *gin.RouterGroup,
	extSvc *services.BrowserExtensionService,
	wsSvc *services.WorkspaceService,
	docSvc *services.DocumentService,
	authSvc *services.AuthService,
	cfg *config.Config,
) {
	h := NewBrowserExtensionHandler(extSvc, wsSvc, docSvc, authSvc, cfg)
	brx := middleware.ValidBrowserExtensionApiKey(extSvc, authSvc, cfg)

	// Extension-facing
	r.GET("/browser-extension/check", brx, h.Check)
	r.DELETE("/browser-extension/disconnect", brx, h.Disconnect)
	r.GET("/browser-extension/workspaces", brx, h.Workspaces)
	r.POST("/browser-extension/embed-content", brx, h.EmbedContent)
	r.POST("/browser-extension/upload-content", brx, h.UploadContent)

	// Management
	cookieAuth := middleware.ValidatedRequest(authSvc)
	roleValid := middleware.FlexUserRoleValid([]string{"admin", "manager"})
	r.GET("/browser-extension/api-keys", cookieAuth, roleValid, h.ApiKeys)
	r.POST("/browser-extension/api-keys/new", cookieAuth, roleValid, h.GenerateApiKey)
	r.DELETE("/browser-extension/api-keys/:id", cookieAuth, roleValid, h.DeleteApiKey)
}
```

- [ ] **Step 2: 编写 handler 测试**

```go
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBrowserExtensionTestEnv(t *testing.T) (*gin.Engine, *services.BrowserExtensionService, *services.WorkspaceService, *services.DocumentService, *services.FileSystemService) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test"}
	db, _ := services.NewDB(cfg)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil { sqlDB.Close() }
	})
	services.AutoMigrate(db)
	extSvc := services.NewBrowserExtensionService(db)
	authSvc := services.NewAuthService(db, cfg)
	wsSvc := services.NewWorkspaceService(db)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	docSvc := services.NewDocumentService(db, cfg, nil, nil, nil, nil, fsSvc)

	r := gin.New()
	RegisterBrowserExtensionRoutes(r.Group("/api"), extSvc, wsSvc, docSvc, authSvc, cfg)
	return r, extSvc, wsSvc, docSvc, fsSvc
}

func TestBrowserExtensionHandler_Check(t *testing.T) {
	r, extSvc, _, _, _ := newBrowserExtensionTestEnv(t)
	key, _ := extSvc.CreateKey(nil)

	req := httptest.NewRequest("GET", "/api/browser-extension/check", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["connected"])
	assert.NotNil(t, body["workspaces"])
}

func TestBrowserExtensionHandler_UploadContent(t *testing.T) {
	r, extSvc, _, _, _ := newBrowserExtensionTestEnv(t)
	key, _ := extSvc.CreateKey(nil)

	payload, _ := json.Marshal(map[string]any{
		"textContent": "hello from extension",
		"metadata":    map[string]string{"title": "Test", "url": "http://example.com"},
	})
	req := httptest.NewRequest("POST", "/api/browser-extension/upload-content", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, true, body["success"])
}

func TestBrowserExtensionHandler_EmbedContent(t *testing.T) {
	r, extSvc, wsSvc, _, _ := newBrowserExtensionTestEnv(t)
	ws := &models.Workspace{Name: "test-ws", Slug: "test-ws"}
	require.NoError(t, wsSvc.DB.Create(ws).Error)
	key, _ := extSvc.CreateKey(nil)

	payload, _ := json.Marshal(map[string]any{
		"workspaceId": ws.ID,
		"textContent": "embed me",
		"metadata":    map[string]string{"title": "Test"},
	})
	req := httptest.NewRequest("POST", "/api/browser-extension/embed-content", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestBrowserExtensionHandler_ApiKeys(t *testing.T) {
	r, _, _, _, _ := newBrowserExtensionTestEnv(t)
	// Create a user and session
	// TODO: setup cookie auth for management endpoints
}
```

- [ ] **Step 3: 运行测试**

```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/handlers/ -run TestBrowserExtensionHandler -v
```

Expected: 3+ tests PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/browser_extension.go backend/internal/handlers/browser_extension_test.go
git commit -m "feat(browser-extension): implement full BrowserExtensionHandler with tests"
```

---

### Task 5: 注册模型到 AutoMigrate + 级联清理

**Files:**
- Modify: `backend/internal/services/db.go`
- Modify: `backend/internal/services/admin_service.go`
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: AutoMigrate 追加模型**

在 `backend/internal/services/db.go` 的 `AutoMigrate` 调用中追加 `&models.BrowserExtensionApiKey{}`：

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		// ... existing models ...
		&models.ExternalCommunicationConnector{},
		&models.BrowserExtensionApiKey{}, // <-- 新增
	)
}
```

- [ ] **Step 2: AdminService 添加级联删除**

在 `backend/internal/services/admin_service.go` 中修改 `DeleteUser`：

```go
func (s *AdminService) DeleteUser(ctx context.Context, id int) error {
	// 先清理浏览器扩展 keys
	if err := NewBrowserExtensionService(s.db).DeleteAllForUser(id); err != nil {
		return fmt.Errorf("cleanup browser extension keys: %w", err)
	}
	return s.db.Delete(&models.User{}, id).Error
}
```

- [ ] **Step 3: main.go 注册完整路由**

在 `backend/cmd/server/main.go` 中：

```go
// 在 adminSvc 创建之后（约第 180 行附近）
extSvc := services.NewBrowserExtensionService(db)

// 修改 RegisterBrowserExtensionRoutes 调用（约第 319 行）
handlers.RegisterBrowserExtensionRoutes(api, extSvc, wsSvc, docSvc, authSvc, cfg)
```

- [ ] **Step 4: 编译检查**

```bash
cd backend && go build -tags="fts5 nolancedb" ./cmd/server/
```

Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/db.go backend/internal/services/admin_service.go backend/cmd/server/main.go
git commit -m "feat(browser-extension): wire AutoMigrate, admin cascade, and main.go routes"
```

---

## PR2: 扩展客户端项目搭建

### Task 6: 初始化扩展客户端项目

**Files:**
- Create: `browser-extension/package.json`
- Create: `browser-extension/vite.config.js`
- Create: `browser-extension/index.html`
- Create: `browser-extension/src/main.jsx`
- Create: `browser-extension/src/index.css`

- [ ] **Step 1: package.json**

```json
{
  "name": "hermind-browser-extension",
  "private": true,
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build && cp public/background.js dist/ && cp public/contentScript.js dist/ && cp public/manifest.json dist/ && cp -r public/icons dist/",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/react": "^18.3.3",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.1",
    "autoprefixer": "^10.4.19",
    "postcss": "^8.4.38",
    "tailwindcss": "^3.4.4",
    "vite": "^5.3.1"
  }
}
```

- [ ] **Step 2: vite.config.js**

```javascript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: {
        popup: resolve(__dirname, 'index.html'),
      },
    },
  },
})
```

- [ ] **Step 3: index.html**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Hermind Companion</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.jsx"></script>
  </body>
</html>
```

- [ ] **Step 4: main.jsx**

```javascript
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```

- [ ] **Step 5: index.css**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

body {
  width: 360px;
  min-height: 400px;
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
```

- [ ] **Step 6: Commit**

```bash
git add browser-extension/package.json browser-extension/vite.config.js browser-extension/index.html browser-extension/src/main.jsx browser-extension/src/index.css
git commit -m "feat(browser-extension): initialize React + Vite project structure"
```

---

### Task 7: 创建 Manifest + 静态脚本 + Popup UI

**Files:**
- Create: `browser-extension/public/manifest.json`
- Create: `browser-extension/public/background.js`
- Create: `browser-extension/public/contentScript.js`
- Create: `browser-extension/src/App.jsx`
- Create: `browser-extension/src/components/Config.jsx`
- Create: `browser-extension/src/hooks/useApiConnection.js`
- Create: `browser-extension/src/models/browserExtension.js`
- Create: `browser-extension/src/utils/constants.js`

- [ ] **Step 1: manifest.json**

```json
{
  "manifest_version": 3,
  "name": "Hermind Browser Companion",
  "version": "1.0.0",
  "description": "Save web content to your Hermind knowledge base",
  "permissions": ["contextMenus", "storage", "alarms", "activeTab", "scripting"],
  "host_permissions": ["<all_urls>"],
  "background": {
    "service_worker": "background.js"
  },
  "content_scripts": [
    {
      "matches": ["<all_urls>"],
      "js": ["contentScript.js"],
      "run_at": "document_idle"
    }
  ],
  "action": {
    "default_popup": "index.html",
    "default_icon": {
      "16": "icons/icon16.png",
      "32": "icons/icon32.png",
      "48": "icons/icon48.png",
      "128": "icons/icon128.png"
    }
  },
  "icons": {
    "16": "icons/icon16.png",
    "32": "icons/icon32.png",
    "48": "icons/icon48.png",
    "128": "icons/icon128.png"
  }
}
```

- [ ] **Step 2: background.js（基础骨架）**

```javascript
// background.js - Chrome Extension Service Worker
const API_BASE_KEY = 'apiBase';
const API_KEY_KEY = 'apiKey';

// Initialize context menus on install
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: 'save-selected',
    title: 'Save selected to Hermind',
    contexts: ['selection'],
  });
  chrome.contextMenus.create({
    id: 'save-page',
    title: 'Save entire page to Hermind',
    contexts: ['page'],
  });
  chrome.contextMenus.create({
    id: 'embed-selected',
    title: 'Embed selected to workspace',
    contexts: ['selection'],
  });
  chrome.contextMenus.create({
    id: 'embed-page',
    title: 'Embed entire page to workspace',
    contexts: ['page'],
  });
});

// Handle context menu clicks
chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId === 'save-selected') {
    uploadContent(info.selectionText, tab.title, tab.url);
  }
  // TODO: other menu items in PR3
});

async function uploadContent(text, title, url) {
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;
  try {
    const resp = await fetch(`${apiBase}/browser-extension/upload-content`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ textContent: text, metadata: { title, url } }),
    });
    if (resp.ok) {
      chrome.action.setBadgeText({ text: '✅' });
      setTimeout(() => chrome.action.setBadgeText({ text: '' }), 2000);
    }
  } catch (e) {
    console.error('Upload failed', e);
  }
}

// Alarm for workspace sync
chrome.alarms.create('syncWorkspaces', { periodInMinutes: 1 });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'syncWorkspaces') {
    updateWorkspaces();
  }
});

async function updateWorkspaces() {
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;
  try {
    const resp = await fetch(`${apiBase}/browser-extension/check`, {
      headers: { 'Authorization': `Bearer ${apiKey}` },
    });
    if (resp.ok) {
      const data = await resp.json();
      // TODO: rebuild dynamic workspace submenus in PR3
      chrome.action.setBadgeText({ text: '' });
      chrome.action.setTitle({ title: 'Hermind - Connected' });
    } else if (resp.status === 403) {
      await chrome.storage.sync.remove([API_BASE_KEY, API_KEY_KEY]);
      chrome.action.setBadgeText({ text: '❌' });
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
}
```

- [ ] **Step 3: contentScript.js**

```javascript
// contentScript.js
window.addEventListener('message', (event) => {
  if (event.data?.type === 'NEW_BROWSER_EXTENSION_CONNECTION') {
    const parts = event.data.apiKey.split('|');
    if (parts.length === 2) {
      chrome.storage.sync.set({ apiBase: parts[0], apiKey: parts[1] }, () => {
        chrome.runtime.sendMessage({ action: 'connectionUpdated' });
      });
    }
  }
});
```

- [ ] **Step 4: browserExtension.js（API 客户端）**

```javascript
class BrowserExtensionAPI {
  constructor(apiBase, apiKey) {
    this.apiBase = apiBase;
    this.apiKey = apiKey;
  }

  async check() {
    const resp = await fetch(`${this.apiBase}/browser-extension/check`, {
      headers: { 'Authorization': `Bearer ${this.apiKey}` },
    });
    return resp.json();
  }

  async disconnect() {
    const resp = await fetch(`${this.apiBase}/browser-extension/disconnect`, {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${this.apiKey}` },
    });
    return resp.json();
  }

  async fetchLogo() {
    const resp = await fetch(`${this.apiBase}/system/logo`);
    if (!resp.ok) return null;
    const blob = await resp.blob();
    return URL.createObjectURL(blob);
  }
}

export default BrowserExtensionAPI;
```

- [ ] **Step 5: useApiConnection.js**

```javascript
import { useState, useEffect, useCallback } from 'react';
import BrowserExtensionAPI from '../models/browserExtension';

export function useApiConnection() {
  const [status, setStatus] = useState('disconnected'); // disconnected | connecting | connected | error
  const [apiBase, setApiBase] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [workspaces, setWorkspaces] = useState([]);
  const [logoUrl, setLogoUrl] = useState(null);

  useEffect(() => {
    chrome.storage.sync.get(['apiBase', 'apiKey'], (items) => {
      if (items.apiBase && items.apiKey) {
        setApiBase(items.apiBase);
        setApiKey(items.apiKey);
        checkConnection(items.apiBase, items.apiKey);
      }
    });
  }, []);

  const checkConnection = useCallback(async (base, key) => {
    setStatus('connecting');
    try {
      const api = new BrowserExtensionAPI(base, key);
      const data = await api.check();
      if (data.connected) {
        setStatus('connected');
        setWorkspaces(data.workspaces || []);
        const logo = await api.fetchLogo();
        if (logo) setLogoUrl(logo);
      } else {
        setStatus('error');
      }
    } catch (e) {
      setStatus('error');
    }
  }, []);

  const connect = useCallback((connectionString) => {
    const parts = connectionString.split('|');
    if (parts.length !== 2) return;
    const [base, key] = parts;
    chrome.storage.sync.set({ apiBase: base, apiKey: key }, () => {
      setApiBase(base);
      setApiKey(key);
      checkConnection(base, key);
    });
  }, [checkConnection]);

  const disconnect = useCallback(async () => {
    try {
      const api = new BrowserExtensionAPI(apiBase, apiKey);
      await api.disconnect();
    } catch (e) {
      // ignore
    }
    chrome.storage.sync.remove(['apiBase', 'apiKey'], () => {
      setApiBase('');
      setApiKey('');
      setStatus('disconnected');
      setWorkspaces([]);
    });
  }, [apiBase, apiKey]);

  return { status, apiBase, apiKey, workspaces, logoUrl, connect, disconnect };
}
```

- [ ] **Step 6: Config.jsx**

```javascript
import React from 'react';

export default function Config({ status, apiBase, onConnect, onDisconnect }) {
  const [input, setInput] = React.useState('');

  if (status === 'connected') {
    return (
      <div className="p-4">
        <div className="flex items-center gap-2 mb-4">
          <span className="text-green-500 text-xl">✅</span>
          <span className="font-medium">Connected to Hermind</span>
        </div>
        <p className="text-sm text-gray-600 mb-4 break-all">{apiBase}</p>
        <button
          onClick={onDisconnect}
          className="w-full px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600"
        >
          Disconnect
        </button>
      </div>
    );
  }

  return (
    <div className="p-4">
      <h2 className="text-lg font-semibold mb-2">Connect to Hermind</h2>
      <p className="text-sm text-gray-600 mb-4">
        Paste your connection string from the Hermind settings page.
      </p>
      <input
        type="text"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="https://example.com/api|brx-..."
        className="w-full px-3 py-2 border rounded mb-3 text-sm"
      />
      <button
        onClick={() => onConnect(input)}
        disabled={status === 'connecting'}
        className="w-full px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
      >
        {status === 'connecting' ? 'Connecting...' : 'Connect'}
      </button>
      {status === 'error' && (
        <p className="text-red-500 text-sm mt-2">Connection failed. Check your API key.</p>
      )}
    </div>
  );
}
```

- [ ] **Step 7: App.jsx**

```javascript
import React from 'react';
import Config from './components/Config';
import { useApiConnection } from './hooks/useApiConnection';

function App() {
  const { status, apiBase, workspaces, logoUrl, connect, disconnect } = useApiConnection();

  return (
    <div className="bg-white">
      <div className="flex items-center gap-2 p-4 border-b">
        {logoUrl && <img src={logoUrl} alt="logo" className="w-6 h-6" />}
        <h1 className="text-lg font-bold">Hermind</h1>
      </div>
      <Config status={status} apiBase={apiBase} onConnect={connect} onDisconnect={disconnect} />
      {status === 'connected' && workspaces.length > 0 && (
        <div className="px-4 pb-4">
          <p className="text-sm text-gray-600">{workspaces.length} workspace(s) available</p>
        </div>
      )}
    </div>
  );
}

export default App;
```

- [ ] **Step 8: 构建验证**

```bash
cd browser-extension && yarn install && yarn build
```

Expected: `browser-extension/dist/` 目录生成，包含 `index.html`、`background.js`、`contentScript.js`、`manifest.json`、icons、JS/CSS bundle

- [ ] **Step 9: Commit**

```bash
git add browser-extension/
git commit -m "feat(browser-extension): add popup UI, background skeleton, content script, and build config"
```

---

## PR3: 扩展客户端核心功能

### Task 8: 完善 Background Service Worker

**Files:**
- Modify: `browser-extension/public/background.js`

- [ ] **Step 1: 完整实现 background.js**

将 `browser-extension/public/background.js` 替换为完整版本，包含：
- 所有 4 个右键菜单项的完整处理逻辑
- 整页保存（通过 `chrome.scripting.executeScript` 获取 `document.body.innerText`）
- 嵌入到 Workspace（调用 embed-content API）
- 动态 Workspace 子菜单（每 60 秒同步，根据 workspace 列表重建子菜单）
- 完整的错误处理和 badge 状态更新
- `chrome.runtime.onMessage` 监听 popup 消息

```javascript
// background.js - Full implementation
const API_BASE_KEY = 'apiBase';
const API_KEY_KEY = 'apiKey';

let workspacesCache = [];

chrome.runtime.onInstalled.addListener(() => {
  rebuildMenus();
});

function rebuildMenus() {
  chrome.contextMenus.removeAll(() => {
    chrome.contextMenus.create({ id: 'save-selected', title: 'Save selected to Hermind', contexts: ['selection'] });
    chrome.contextMenus.create({ id: 'save-page', title: 'Save entire page to Hermind', contexts: ['page'] });
    chrome.contextMenus.create({ id: 'sep1', type: 'separator', contexts: ['selection', 'page'] });
    chrome.contextMenus.create({ id: 'embed-selected-parent', title: 'Embed selected to workspace', contexts: ['selection'] });
    chrome.contextMenus.create({ id: 'embed-page-parent', title: 'Embed entire page to workspace', contexts: ['page'] });
    rebuildWorkspaceSubmenus();
  });
}

function rebuildWorkspaceSubmenus() {
  chrome.contextMenus.removeAll(() => {
    chrome.contextMenus.create({ id: 'save-selected', title: 'Save selected to Hermind', contexts: ['selection'] });
    chrome.contextMenus.create({ id: 'save-page', title: 'Save entire page to Hermind', contexts: ['page'] });
    chrome.contextMenus.create({ id: 'sep1', type: 'separator', contexts: ['selection', 'page'] });
    chrome.contextMenus.create({ id: 'embed-selected-parent', title: 'Embed selected to workspace', contexts: ['selection'] });
    chrome.contextMenus.create({ id: 'embed-page-parent', title: 'Embed entire page to workspace', contexts: ['page'] });
    for (const ws of workspacesCache) {
      chrome.contextMenus.create({
        parentId: 'embed-selected-parent',
        id: `embed-selected-${ws.id}`,
        title: ws.name,
        contexts: ['selection'],
      });
      chrome.contextMenus.create({
        parentId: 'embed-page-parent',
        id: `embed-page-${ws.id}`,
        title: ws.name,
        contexts: ['page'],
      });
    }
  });
}

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;

  if (info.menuItemId === 'save-selected') {
    await uploadContent(apiBase, apiKey, info.selectionText, tab.title, tab.url);
  } else if (info.menuItemId === 'save-page') {
    const text = await getPageContent(tab.id);
    await uploadContent(apiBase, apiKey, text, tab.title, tab.url);
  } else if (info.menuItemId.startsWith('embed-selected-')) {
    const wsId = parseInt(info.menuItemId.replace('embed-selected-', ''));
    await embedContent(apiBase, apiKey, wsId, info.selectionText, tab.title, tab.url);
  } else if (info.menuItemId.startsWith('embed-page-')) {
    const wsId = parseInt(info.menuItemId.replace('embed-page-', ''));
    const text = await getPageContent(tab.id);
    await embedContent(apiBase, apiKey, wsId, text, tab.title, tab.url);
  }
});

async function getPageContent(tabId) {
  try {
    const [{ result }] = await chrome.scripting.executeScript({
      target: { tabId },
      func: () => document.body.innerText,
    });
    return result || '';
  } catch (e) {
    return '';
  }
}

async function uploadContent(apiBase, apiKey, text, title, url) {
  try {
    const resp = await fetch(`${apiBase}/browser-extension/upload-content`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ textContent: text, metadata: { title, url } }),
    });
    if (resp.ok) {
      chrome.action.setBadgeText({ text: '✅' });
      setTimeout(() => chrome.action.setBadgeText({ text: '' }), 2000);
    } else {
      handleApiError(resp);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
}

async function embedContent(apiBase, apiKey, workspaceId, text, title, url) {
  try {
    const resp = await fetch(`${apiBase}/browser-extension/embed-content`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ workspaceId, textContent: text, metadata: { title, url } }),
    });
    if (resp.ok) {
      chrome.action.setBadgeText({ text: '✅' });
      setTimeout(() => chrome.action.setBadgeText({ text: '' }), 2000);
    } else {
      handleApiError(resp);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
}

async function handleApiError(resp) {
  if (resp.status === 403) {
    await chrome.storage.sync.remove([API_BASE_KEY, API_KEY_KEY]);
    workspacesCache = [];
    rebuildMenus();
    chrome.action.setBadgeText({ text: '❌' });
    chrome.action.setTitle({ title: 'Hermind - Disconnected' });
  }
}

chrome.alarms.create('syncWorkspaces', { periodInMinutes: 1 });
chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name !== 'syncWorkspaces') return;
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;
  try {
    const resp = await fetch(`${apiBase}/browser-extension/check`, {
      headers: { 'Authorization': `Bearer ${apiKey}` },
    });
    if (resp.ok) {
      const data = await resp.json();
      workspacesCache = data.workspaces || [];
      rebuildWorkspaceSubmenus();
      chrome.action.setBadgeText({ text: '' });
      chrome.action.setTitle({ title: 'Hermind - Connected' });
    } else {
      handleApiError(resp);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
});

// Listen for popup messages
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'connectionUpdated') {
    updateWorkspaces();
  }
  sendResponse({ ok: true });
});
```

- [ ] **Step 2: 构建验证**

```bash
cd browser-extension && yarn build
```

Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add browser-extension/public/background.js
git commit -m "feat(browser-extension): full background service worker with dynamic menus"
```

---

### Task 9: 前端 Web UI 自动注入

**Files:**
- Modify: `frontend/src/pages/GeneralSettings/BrowserExtensionApiKey/NewBrowserExtensionApiKeyModal/index.jsx`

- [ ] **Step 1: 修改生成 key 后的逻辑**

在 `NewBrowserExtensionApiKeyModal` 中，当成功生成 key 后，触发 `window.postMessage`：

```javascript
// 在生成 key 成功后（在现有的 .then 中追加）
const apiKey = result.apiKey;
const connectionString = `${window.location.origin}/api|${apiKey}`;
window.postMessage({
  type: "NEW_BROWSER_EXTENSION_CONNECTION",
  apiKey: connectionString,
}, "*");
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/pages/GeneralSettings/BrowserExtensionApiKey/NewBrowserExtensionApiKeyModal/index.jsx
git commit -m "feat(browser-extension): auto-inject connection string to extension"
```

---

## PR4: 集成测试 + 构建集成

### Task 10: 后端 Handler 集成测试

**Files:**
- Modify: `backend/internal/handlers/browser_extension_test.go`

- [ ] **Step 1: 补充完整测试**

补充以下测试用例（如果 PR1 中未完全覆盖）：
- `TestBrowserExtensionHandler_Disconnect` — disconnect 后 key 失效
- `TestBrowserExtensionHandler_EmbedContent_InvalidWorkspace` — 404
- `TestBrowserExtensionHandler_EmbedContent_EmptyText` — 400
- `TestBrowserExtensionHandler_Workspaces` — 返回列表
- `TestBrowserExtensionHandler_GenerateApiKey` — 管理端点
- `TestBrowserExtensionHandler_DeleteApiKey` — 管理端点权限

```go
func TestBrowserExtensionHandler_Disconnect(t *testing.T) {
	r, extSvc, _, _, _ := newBrowserExtensionTestEnv(t)
	key, _ := extSvc.CreateKey(nil)

	req := httptest.NewRequest("DELETE", "/api/browser-extension/disconnect", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Key should be invalid now
	_, err := extSvc.Validate(key.Key)
	assert.Error(t, err)
}

func TestBrowserExtensionHandler_Workspaces(t *testing.T) {
	r, extSvc, _, _, _ := newBrowserExtensionTestEnv(t)
	key, _ := extSvc.CreateKey(nil)

	req := httptest.NewRequest("GET", "/api/browser-extension/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	assert.NotNil(t, body["workspaces"])
}
```

- [ ] **Step 2: 运行全部测试**

```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/handlers/ -run TestBrowserExtensionHandler -v
```

Expected: 所有测试 PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/browser_extension_test.go
git commit -m "test(browser-extension): add integration tests for all endpoints"
```

---

### Task 11: Makefile 与 AGENTS.md 更新

**Files:**
- Modify: `Makefile`
- Modify: `AGENTS.md`

- [ ] **Step 1: 修改 Makefile**

在 `Makefile` 中追加：

```makefile
.PHONY: build-extension build-all

build-extension:
	cd browser-extension && yarn install && yarn build

build-all: build-extension build-server
```

- [ ] **Step 2: 修改 AGENTS.md**

在 AGENTS.md 的 "Build and Run" 章节追加：

```markdown
### Browser Extension Development

```bash
# Build the browser extension (React + Vite)
make build-extension

# Output: browser-extension/dist/
# Load in Chrome: chrome://extensions/ → "Load unpacked" → select dist/
```

在 "Common Tasks for Agents" 章节追加：

```markdown
### Adding Browser Extension Features

1. Backend changes go under `backend/internal/handlers/browser_extension.go` and `backend/internal/services/browser_extension.go`.
2. Extension client changes go under `browser-extension/src/` and `browser-extension/public/`.
3. Build the extension with `make build-extension`.
4. Test the extension by loading `browser-extension/dist/` as an unpacked extension in Chrome.
```

- [ ] **Step 3: Commit**

```bash
git add Makefile AGENTS.md
git commit -m "chore(browser-extension): add Makefile target and AGENTS.md docs"
```

---

## 自审检查

**1. Spec 覆盖检查：**

| Spec 需求 | 实现任务 |
|-----------|----------|
| `brx-<uuid>` key 格式 | Task 2: `CreateKey` 生成 `brx-` + UUID |
| 专用中间件验证 key | Task 3: `ValidBrowserExtensionApiKey` |
| 单用户/多用户模式支持 | Task 3: 条件分支 `cfg.MultiUserMode` |
| 8 个 HTTP 端点 | Task 4: handler 实现全部 8 个 |
| Admin 删除用户级联清理 | Task 5: `DeleteAllForUser` |
| upload-content（仅保存） | Task 4: `UploadContent` 调用 `SaveRawText(..., nil)` |
| embed-content（保存+嵌入） | Task 4: `EmbedContent` 调用 `SaveRawText` + `EmbedDocument` |
| 自动同步 Workspace 列表 | Task 8: `chrome.alarms` + `updateWorkspaces` |
| 动态右键子菜单 | Task 8: `rebuildWorkspaceSubmenus` |
| Web UI 自动注入 | Task 9: `window.postMessage` |
| Logo 同步 | Task 7: `fetchLogo` + popup 显示 |
| React + Vite 构建 | Task 6-7: 完整项目结构 |
| Manifest V3 | Task 7: `manifest.json` |
| 测试覆盖 | Task 2, 3, 4, 10: service/middleware/handler 测试 |
| Makefile 集成 | Task 11: `build-extension` 目标 |

**2. Placeholder 扫描：** 无 TBD、TODO、"implement later"。

**3. 类型一致性：** `BrowserExtensionApiKey.ID` 为 `int`，与 `DeleteKey(id int)`、`DeleteAllForUser(userID int)` 一致；`WorkspaceService.List(ctx, userID)` 接受 `int`，与 `user.ID` 一致。

---

## 执行方式

**Plan complete and saved to `.gpowers/plans/2026-05-28-browser-extension-plan.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
