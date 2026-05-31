# Telegram Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring full-featured Telegram bot integration to Hermind's Go backend, achieving functional parity with anything-llm v1.12.0 (text chat, `@agent`, voice, photo, document, TTS, pairing approval, inline keyboard tool approval, polling retry, 401 self-cleanup).

**Architecture:** A singleton `TelegramBotService` polls the Telegram API using `go-telegram-bot-api`, routes updates through per-chat serial queues, and hands off to either `ChatService.Complete` (regular chat) or `agent.Runtime.RunAgentDirectly` (`@agent`). The agent runtime is refactored with `AgentIO` / `AgentInput` abstractions so it no longer hard-codes WebSocket. `services` does not import `agent` to avoid an import cycle; `main.go` bridges the two packages via callbacks.

**Tech Stack:** Go 1.26, Gin, GORM, `go-telegram-bot-api`, Pantheon, SQLite (memory for tests), `mlog`

---

## File Structure

### New Files

| File | Responsibility |
|---|---|
| `backend/internal/models/external_communication_connector.go` | GORM model for connector config (encrypted JSON) |
| `backend/internal/services/telegram_config.go` | `TelegramConfigService` — load/save/encrypt connector config |
| `backend/internal/services/telegram_bot.go` | `TelegramBotService` — singleton lifecycle, polling, update router |
| `backend/internal/services/telegram_queue.go` | Per-chat `messageQueue` with serial goroutine dispatch |
| `backend/internal/services/telegram_pairing.go` | Pairing code generation, pending/approved user registry |
| `backend/internal/services/telegram_commands.go` | `/start`, `/help`, `/switch`, `/history`, `/model`, `/reset` handlers |
| `backend/internal/services/telegram_chat.go` | Regular text chat → `ChatService.Complete` |
| `backend/internal/services/telegram_media.go` | Voice (STT), photo (vision), document (parse) pipelines |
| `backend/internal/services/telegram_tts.go` | TTS voice reply logic (`text_only` / `mirror` / `always_voice`) |
| `backend/internal/agent/io.go` | `AgentIO`, `AgentInput`, `InputAction` interfaces + `wsInput` adapter |
| `backend/internal/agent/telegram_io.go` | `telegramAgentIO` and `telegramInput` implementations |

### Modified Files

| File | Change |
|---|---|
| `backend/internal/services/db.go` | Add `ExternalCommunicationConnector` to `AutoMigrate` |
| `backend/internal/handlers/telegram.go` | Implement all 10 route handlers |
| `backend/internal/agent/session.go` | Replace `wsConn *wsConn` with `io AgentIO` |
| `backend/internal/agent/reader.go` | Replace `readerLoop` with `readerLoopWithInput(AgentInput)` |
| `backend/internal/agent/approval.go` | Change `wsConn.Send` → `io.Send` |
| `backend/internal/agent/handler.go` | `HandleWS` wraps `wsConn` as `AgentIO` + `wsInput`; add `RunAgentDirectly` |
| `backend/internal/agent/runtime.go` | Add `RunAgentDirectly` method |
| `backend/cmd/server/main.go` | Wire `TelegramBotService`, set agent callback, register approval handlers |

---

## PR1: Data Model + HTTP Routes

### Task 1.1: Create `ExternalCommunicationConnector` model

**Files:**
- Create: `backend/internal/models/external_communication_connector.go`

- [ ] **Step 1: Write the model file**

```go
package models

import "time"

type ExternalCommunicationConnector struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Type          string    `gorm:"unique" json:"type"`
	Config        string    `json:"config"`
	Active        bool      `gorm:"default:false" json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

- [ ] **Step 2: Register in AutoMigrate**

Edit `backend/internal/services/db.go` line 30-59. Add `&models.ExternalCommunicationConnector{}` to the `AutoMigrate` slice:

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		// ... existing models ...
		&models.Memory{},
		&models.ExternalCommunicationConnector{},
	)
}
```

- [ ] **Step 3: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile, no errors

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/external_communication_connector.go backend/internal/services/db.go
git commit -m "feat(telegram): add ExternalCommunicationConnector model"
```

---

### Task 1.2: Create `TelegramConfigService`

**Files:**
- Create: `backend/internal/services/telegram_config.go`
- Test: `backend/internal/services/telegram_config_test.go`

- [ ] **Step 1: Write config structures**

```go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type TelegramUser struct {
	ChatID          string `json:"chatId"`
	Username        string `json:"username,omitempty"`
	FirstName       string `json:"firstName,omitempty"`
	ActiveWorkspace string `json:"active_workspace,omitempty"`
	ActiveThread    string `json:"active_thread,omitempty"`
}

type TelegramConfig struct {
	BotToken          string         `json:"-"`
	BotUsername       string         `json:"bot_username"`
	DefaultWorkspace  string         `json:"default_workspace"`
	ApprovedUsers     []TelegramUser `json:"approved_users"`
	VoiceResponseMode string         `json:"voice_response_mode"`
}

type TelegramConfigService struct {
	db  *gorm.DB
	enc *utils.EncryptionManager
}

func NewTelegramConfigService(db *gorm.DB, enc *utils.EncryptionManager) *TelegramConfigService {
	return &TelegramConfigService{db: db, enc: enc}
}

func (s *TelegramConfigService) Load(ctx context.Context) (*TelegramConfig, error) {
	var conn models.ExternalCommunicationConnector
	if err := s.db.WithContext(ctx).Where("type = ?", "telegram").First(&conn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if !conn.Active {
		return nil, nil
	}
	plaintext, err := s.enc.Decrypt(conn.Config)
	if err != nil {
		return nil, fmt.Errorf("decrypt telegram config: %w", err)
	}
	var cfg TelegramConfig
	if err := json.Unmarshal([]byte(plaintext), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal telegram config: %w", err)
	}
	return &cfg, nil
}

func (s *TelegramConfigService) Save(ctx context.Context, cfg *TelegramConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	cipher, err := s.enc.Encrypt(string(raw))
	if err != nil {
		return fmt.Errorf("encrypt telegram config: %w", err)
	}
	var conn models.ExternalCommunicationConnector
	result := s.db.WithContext(ctx).Where("type = ?", "telegram").First(&conn)
	now := time.Now()
	if result.Error == gorm.ErrRecordNotFound {
		conn = models.ExternalCommunicationConnector{
			Type:          "telegram",
			Config:        cipher,
			Active:        true,
			CreatedAt:     now,
			LastUpdatedAt: now,
		}
		return s.db.WithContext(ctx).Create(&conn).Error
	}
	conn.Config = cipher
	conn.Active = true
	conn.LastUpdatedAt = now
	return s.db.WithContext(ctx).Save(&conn).Error
}

func (s *TelegramConfigService) Delete(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("type = ?", "telegram").Delete(&models.ExternalCommunicationConnector{}).Error
}
```

- [ ] **Step 2: Write the failing test**

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTelegramConfigService_SaveLoad(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&ExternalCommunicationConnector{}))

	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)

	svc := NewTelegramConfigService(db, enc)
	ctx := context.Background()

	cfg := &TelegramConfig{
		BotToken:          "secret-token",
		BotUsername:       "testbot",
		DefaultWorkspace:  "default",
		VoiceResponseMode: "mirror",
		ApprovedUsers: []TelegramUser{
			{ChatID: "123", Username: "alice", ActiveWorkspace: "ws1"},
		},
	}
	require.NoError(t, svc.Save(ctx, cfg))

	loaded, err := svc.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "testbot", loaded.BotUsername)
	assert.Equal(t, "ws1", loaded.ApprovedUsers[0].ActiveWorkspace)
	assert.Empty(t, loaded.BotToken) // json:"-" omits from marshal
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd backend && go test -tags="fts5 nolancedb" ./internal/services -run TestTelegramConfigService_SaveLoad -v`
Expected: FAIL (test file references `ExternalCommunicationConnector` which is in `models` package, not `services`)

- [ ] **Step 4: Fix the test import**

The test needs to import the model. Update the test:

```go
import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)
```

And change `db.AutoMigrate(&ExternalCommunicationConnector{})` to `db.AutoMigrate(&models.ExternalCommunicationConnector{})`.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd backend && go test -tags="fts5 nolancedb" ./internal/services -run TestTelegramConfigService_SaveLoad -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/telegram_config.go backend/internal/services/telegram_config_test.go
git commit -m "feat(telegram): add TelegramConfigService with encrypted config persistence"
```

---

### Task 1.3: Implement HTTP route handlers

**Files:**
- Modify: `backend/internal/handlers/telegram.go`

- [ ] **Step 1: Update handler struct and constructor**

```go
type TelegramHandler struct {
	cfg     *config.Config
	tgSvc   *services.TelegramBotService
}

func NewTelegramHandler(cfg *config.Config, tgSvc *services.TelegramBotService) *TelegramHandler {
	return &TelegramHandler{cfg: cfg, tgSvc: tgSvc}
}
```

- [ ] **Step 2: Implement all handlers**

Replace the entire `backend/internal/handlers/telegram.go` file:

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type TelegramHandler struct {
	cfg   *config.Config
	tgSvc *services.TelegramBotService
}

func NewTelegramHandler(cfg *config.Config, tgSvc *services.TelegramBotService) *TelegramHandler {
	return &TelegramHandler{cfg: cfg, tgSvc: tgSvc}
}

func singleUserMode(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.MultiUserMode {
			c.JSON(http.StatusForbidden, gin.H{"error": "Telegram is only available in single-user mode"})
			c.Abort()
			return
		}
		c.Next()
	}
}

type connectRequest struct {
	BotToken string `json:"bot_token" binding:"required"`
}

func (h *TelegramHandler) GetConfig(c *gin.Context) {
	cfg, err := h.tgSvc.GetConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"config": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

func (h *TelegramHandler) Connect(c *gin.Context) {
	var req connectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.Start(c.Request.Context(), req.BotToken); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "bot_username": nil})
		return
	}
	cfg, _ := h.tgSvc.GetConfig(c.Request.Context())
	username := ""
	if cfg != nil {
		username = cfg.BotUsername
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "bot_username": username})
}

func (h *TelegramHandler) Disconnect(c *gin.Context) {
	if err := h.tgSvc.Stop(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *TelegramHandler) Status(c *gin.Context) {
	active, username := h.tgSvc.Status()
	c.JSON(http.StatusOK, gin.H{"active": active, "bot_username": username})
}

func (h *TelegramHandler) PendingUsers(c *gin.Context) {
	users := h.tgSvc.PendingUsers()
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *TelegramHandler) ApprovedUsers(c *gin.Context) {
	users := h.tgSvc.ApprovedUsers()
	c.JSON(http.StatusOK, gin.H{"users": users})
}

type approveUserRequest struct {
	ChatID   string `json:"chatId" binding:"required"`
	Username string `json:"username"`
}

func (h *TelegramHandler) ApproveUser(c *gin.Context) {
	var req approveUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.ApproveUser(c.Request.Context(), req.ChatID, req.Username); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) DenyUser(c *gin.Context) {
	var req approveUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.DenyUser(req.ChatID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

type updateConfigRequest struct {
	DefaultWorkspace  string `json:"default_workspace"`
	VoiceResponseMode string `json:"voice_response_mode"`
}

func (h *TelegramHandler) UpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.UpdateConfig(c.Request.Context(), req.DefaultWorkspace, req.VoiceResponseMode); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) RevokeUser(c *gin.Context) {
	var req approveUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.RevokeUser(c.Request.Context(), req.ChatID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterTelegramRoutes(r *gin.RouterGroup, cfg *config.Config, authSvc *services.AuthService, tgSvc *services.TelegramBotService) {
	h := NewTelegramHandler(cfg, tgSvc)
	sum := singleUserMode(cfg)

	r.GET("/telegram/config", middleware.ValidatedRequest(authSvc), sum, h.GetConfig)
	r.POST("/telegram/connect", middleware.ValidatedRequest(authSvc), sum, h.Connect)
	r.POST("/telegram/disconnect", middleware.ValidatedRequest(authSvc), sum, h.Disconnect)
	r.GET("/telegram/status", middleware.ValidatedRequest(authSvc), sum, h.Status)
	r.GET("/telegram/pending-users", middleware.ValidatedRequest(authSvc), sum, h.PendingUsers)
	r.GET("/telegram/approved-users", middleware.ValidatedRequest(authSvc), sum, h.ApprovedUsers)
	r.POST("/telegram/approve-user", middleware.ValidatedRequest(authSvc), sum, h.ApproveUser)
	r.POST("/telegram/deny-user", middleware.ValidatedRequest(authSvc), sum, h.DenyUser)
	r.POST("/telegram/update-config", middleware.ValidatedRequest(authSvc), sum, h.UpdateConfig)
	r.POST("/telegram/revoke-user", middleware.ValidatedRequest(authSvc), sum, h.RevokeUser)
}
```

- [ ] **Step 3: Create minimal `TelegramBotService` stub so handlers compile**

Create `backend/internal/services/telegram_bot.go` with a stub:

```go
package services

import "context"

type TelegramBotService struct{}

func NewTelegramBotService() *TelegramBotService {
	return &TelegramBotService{}
}

func (s *TelegramBotService) Start(ctx context.Context, token string) error { return nil }
func (s *TelegramBotService) Stop(ctx context.Context) error                { return nil }
func (s *TelegramBotService) Boot(ctx context.Context) error                { return nil }
func (s *TelegramBotService) Status() (bool, string)                        { return false, "" }
func (s *TelegramBotService) GetConfig(ctx context.Context) (*TelegramConfig, error) { return nil, nil }
func (s *TelegramBotService) PendingUsers() []TelegramUser                  { return nil }
func (s *TelegramBotService) ApprovedUsers() []TelegramUser                 { return nil }
func (s *TelegramBotService) ApproveUser(ctx context.Context, chatID, username string) error { return nil }
func (s *TelegramBotService) DenyUser(chatID string) error                  { return nil }
func (s *TelegramBotService) RevokeUser(ctx context.Context, chatID string) error { return nil }
func (s *TelegramBotService) UpdateConfig(ctx context.Context, workspace, mode string) error { return nil }
```

- [ ] **Step 4: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/telegram.go backend/internal/services/telegram_bot.go
git commit -m "feat(telegram): implement HTTP route handlers with stub service"
```

---

## PR2: Bot Core Service

### Task 2.1: Create polling loop and lifecycle

**Files:**
- Modify: `backend/internal/services/telegram_bot.go`
- Create: `backend/internal/services/telegram_queue.go`

Prerequisite: Add dependency `go-telegram-bot-api`:
Run: `cd backend && go get github.com/go-telegram-bot-api/telegram-bot-api/v5`

- [ ] **Step 1: Implement `TelegramBotService` with polling**

Replace `backend/internal/services/telegram_bot.go`:

```go
package services

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

const maxActiveQueues = 1000

type TelegramBotService struct {
	db      *gorm.DB
	cfg     *config.Config
	sysSvc  *SystemService
	enc     *utils.EncryptionManager
	chatSvc *ChatService

	configSvc *TelegramConfigService
	config    *TelegramConfig
	bot       *tgbotapi.BotAPI

	mu            sync.RWMutex
	pending       sync.Map // string(chatID) → pendingPairing
	approved      sync.Map // string(chatID) → TelegramUser
	queues        sync.Map // int64(chatID) → *messageQueue
	approvalHandlers sync.Map // string(chatID) → ApprovalHandler

	stopCh chan struct{}
	wg     sync.WaitGroup
}

type pendingPairing struct {
	Code      string
	Username  string
	FirstName string
	CreatedAt time.Time
}

// ApprovalHandler is called when a user approves/denies a tool via inline keyboard.
type ApprovalHandler interface {
	HandleApproval(requestID string, approved bool)
}

// TelegramAgentCallback is the bridge to agent.Runtime. Set by main.go to avoid import cycles.
type TelegramAgentCallback func(ctx context.Context, invUUID string, chatID int64, sendText func(text string) error, sendApprovalReq func(requestID, skillName, description string, timeoutMs int) error) error

func NewTelegramBotService(db *gorm.DB, cfg *config.Config, sysSvc *SystemService, enc *utils.EncryptionManager, chatSvc *ChatService) *TelegramBotService {
	return &TelegramBotService{
		db:        db,
		cfg:       cfg,
		sysSvc:    sysSvc,
		enc:       enc,
		chatSvc:   chatSvc,
		configSvc: NewTelegramConfigService(db, enc),
		stopCh:    make(chan struct{}),
	}
}

func (s *TelegramBotService) Boot(ctx context.Context) error {
	if s.cfg.MultiUserMode {
		return nil
	}
	cfg, err := s.configSvc.Load(ctx)
	if err != nil {
		mlog.Warning("telegram boot: load config failed: ", err)
		return nil
	}
	if cfg == nil || cfg.BotToken == "" {
		return nil
	}
	return s.startWithConfig(ctx, cfg)
}

func (s *TelegramBotService) Start(ctx context.Context, token string) error {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("invalid bot token: %w", err)
	}
	bot.Debug = s.cfg.DebugMode

	cfg := &TelegramConfig{
		BotToken:          token,
		BotUsername:       bot.Self.UserName,
		VoiceResponseMode: "text_only",
	}
	if err := s.configSvc.Save(ctx, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return s.startWithConfig(ctx, cfg)
}

func (s *TelegramBotService) startWithConfig(ctx context.Context, cfg *TelegramConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.bot != nil {
		return fmt.Errorf("bot already running")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return err
	}
	bot.Debug = s.cfg.DebugMode
	s.bot = bot
	s.config = cfg

	for _, u := range cfg.ApprovedUsers {
		s.approved.Store(u.ChatID, u)
	}

	s.wg.Add(1)
	go s.pollLoop()
	return nil
}

func (s *TelegramBotService) Stop(ctx context.Context) error {
	s.mu.Lock()
	bot := s.bot
	s.bot = nil
	s.config = nil
	s.mu.Unlock()

	if bot == nil {
		return nil
	}

	close(s.stopCh)
	s.wg.Wait()
	s.stopCh = make(chan struct{})

	s.queues.Range(func(key, value any) bool {
		if q, ok := value.(*messageQueue); ok {
			q.stop()
		}
		return true
	})
	s.queues = sync.Map{}
	s.pending = sync.Map{}
	s.approvalHandlers = sync.Map{}

	return s.configSvc.Delete(ctx)
}

func (s *TelegramBotService) Status() (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.bot == nil {
		return false, ""
	}
	return true, s.config.BotUsername
}

func (s *TelegramBotService) GetConfig(ctx context.Context) (*TelegramConfig, error) {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	if cfg != nil {
		// Return a copy without token
		cpy := *cfg
		cpy.BotToken = ""
		return &cpy, nil
	}
	return s.configSvc.Load(ctx)
}

func (s *TelegramBotService) selfCleanup(ctx context.Context) {
	mlog.Warning("telegram: self-cleanup triggered (401 or multi-user mode)")
	_ = s.Stop(ctx)
}

func (s *TelegramBotService) pollLoop() {
	defer s.wg.Done()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := s.bot.GetUpdatesChan(u)

	for {
		select {
		case <-s.stopCh:
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message != nil {
				s.handleUpdate(update)
			} else if update.CallbackQuery != nil {
				s.handleCallback(update.CallbackQuery)
			}
		}
	}
}

func (s *TelegramBotService) handleUpdate(update tgbotapi.Update) {
	msg := update.Message
	chatID := msg.Chat.ID

	q, loaded := s.queues.Load(chatID)
	if !loaded {
		count := 0
		s.queues.Range(func(_, _ any) bool { count++; return true })
		if count >= maxActiveQueues {
			mlog.Warning("telegram: max active queues reached, dropping message from chat ", chatID)
			return
		}
		newQ := newMessageQueue(chatID, s)
		s.queues.Store(chatID, newQ)
		q = newQ
		go newQ.run()
	}
	q.(*messageQueue).enqueue(update)
}

func (s *TelegramBotService) handleCallback(query *tgbotapi.CallbackQuery) {
	data := query.Data
	chatID := strconv.FormatInt(query.Message.Chat.ID, 10)

	if handler, ok := s.approvalHandlers.Load(chatID); ok {
		var requestID string
		var approved bool
		if _, err := fmt.Sscanf(data, "tool:approve:%s", &requestID); err == nil {
			approved = true
		} else if _, err := fmt.Sscanf(data, "tool:deny:%s", &requestID); err == nil {
			approved = false
		} else {
			return
		}
		handler.(ApprovalHandler).HandleApproval(requestID, approved)
		cb := tgbotapi.NewCallback(query.ID, "")
		_, _ = s.bot.Request(cb)
	}
}

func (s *TelegramBotService) sendText(chatID int64, text string) error {
	_, err := s.bot.Send(tgbotapi.NewMessage(chatID, text))
	return err
}

func (s *TelegramBotService) sendApprovalReq(chatID int64, requestID, skillName, description string, timeoutMs int) error {
	text := fmt.Sprintf("🔧 *Tool Approval Required*\n\nThe agent wants to execute: `%s`\n\n%s", skillName, description)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Approve", fmt.Sprintf("tool:approve:%s", requestID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Deny", fmt.Sprintf("tool:deny:%s", requestID)),
		),
	)
	_, err := s.bot.Send(msg)
	return err
}
```

- [ ] **Step 2: Implement `messageQueue`**

Create `backend/internal/services/telegram_queue.go`:

```go
package services

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/mlog"
)

type messageQueue struct {
	chatID   int64
	svc      *TelegramBotService
	updates  chan tgbotapi.Update
	stopOnce sync.Once
	stopCh   chan struct{}
}

func newMessageQueue(chatID int64, svc *TelegramBotService) *messageQueue {
	return &messageQueue{
		chatID:  chatID,
		svc:     svc,
		updates: make(chan tgbotapi.Update, 16),
		stopCh:  make(chan struct{}),
	}
}

func (q *messageQueue) enqueue(u tgbotapi.Update) {
	select {
	case q.updates <- u:
	case <-q.stopCh:
	}
}

func (q *messageQueue) stop() {
	q.stopOnce.Do(func() {
		close(q.stopCh)
		close(q.updates)
	})
}

func (q *messageQueue) run() {
	defer q.svc.queues.Delete(q.chatID)
	for {
		select {
		case <-q.stopCh:
			return
		case u, ok := <-q.updates:
			if !ok {
				return
			}
			q.process(u)
		}
	}
}

func (q *messageQueue) process(u tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("telegram queue panic: ", r)
		}
	}()

	msg := u.Message
	chatIDStr := fmt.Sprintf("%d", q.chatID)

	// Pairing check
	if !q.isApproved(chatIDStr) {
		if msg.IsCommand() && msg.Command() == "start" {
			q.handleStart(msg)
		} else {
			_ = q.svc.sendText(q.chatID, "Please send /start to begin pairing.")
		}
		return
	}

	if msg.IsCommand() {
		q.handleCommand(msg)
		return
	}

	// Regular text / media
	q.svc.handleChatMessage(context.Background(), msg, q.chatID, chatIDStr)
}

func (q *messageQueue) isApproved(chatIDStr string) bool {
	_, ok := q.svc.approved.Load(chatIDStr)
	return ok
}
```

- [ ] **Step 3: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/telegram_bot.go backend/internal/services/telegram_queue.go
git commit -m "feat(telegram): add polling loop and per-chat message queues"
```

---

### Task 2.2: Implement pairing flow

**Files:**
- Create: `backend/internal/services/telegram_pairing.go`
- Test: `backend/internal/services/telegram_pairing_test.go`

- [ ] **Step 1: Write pairing logic**

```go
package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/mlog"
)

const maxPendingPairings = 10

func (s *TelegramBotService) handleStart(msg *tgbotapi.Message) {
	chatID := fmt.Sprintf("%d", msg.Chat.ID)

	// Check if already approved
	if _, ok := s.approved.Load(chatID); ok {
		_ = s.sendText(msg.Chat.ID, "✅ You're already approved. Send /help for available commands.")
		return
	}

	// Generate 6-digit code
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Evict oldest if at capacity
	var oldestKey string
	var oldestTime time.Time
	count := 0
	s.pending.Range(func(k, v any) bool {
		count++
		p := v.(*pendingPairing)
		if oldestKey == "" || p.CreatedAt.Before(oldestTime) {
			oldestKey = k.(string)
			oldestTime = p.CreatedAt
		}
		return true
	})
	if count >= maxPendingPairings && oldestKey != "" {
		s.pending.Delete(oldestKey)
	}

	s.pending.Store(chatID, &pendingPairing{
		Code:      code,
		Username:  msg.From.UserName,
		FirstName: msg.From.FirstName,
		CreatedAt: time.Now(),
	})

	text := fmt.Sprintf("🔐 Your pairing code is: *%s*\n\nGo to Settings → Telegram in the web UI and enter this code to approve access.", code)
	_ = s.sendText(msg.Chat.ID, text)
}

func (s *TelegramBotService) PendingUsers() []TelegramUser {
	var users []TelegramUser
	s.pending.Range(func(k, v any) bool {
		p := v.(*pendingPairing)
		users = append(users, TelegramUser{
			ChatID:    k.(string),
			Username:  p.Username,
			FirstName: p.FirstName,
		})
		return true
	})
	return users
}

func (s *TelegramBotService) ApprovedUsers() []TelegramUser {
	var users []TelegramUser
	s.approved.Range(func(k, v any) bool {
		users = append(users, v.(TelegramUser))
		return true
	})
	return users
}

func (s *TelegramBotService) ApproveUser(ctx context.Context, chatID, username string) error {
	pRaw, ok := s.pending.Load(chatID)
	if !ok {
		return fmt.Errorf("no pending pairing found for chat %s", chatID)
	}
	p := pRaw.(*pendingPairing)
	s.pending.Delete(chatID)

	user := TelegramUser{
		ChatID:    chatID,
		Username:  username,
		FirstName: p.FirstName,
	}
	s.approved.Store(chatID, user)

	// Persist
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	if cfg != nil {
		cfg.ApprovedUsers = append(cfg.ApprovedUsers, user)
		if err := s.configSvc.Save(ctx, cfg); err != nil {
			mlog.Warning("telegram: failed to persist approved user: ", err)
		}
	}

	cid, _ := fmt.ParseInt(chatID, 10, 64)
	_ = s.sendText(cid, "✅ You've been approved! Send /help to see available commands.")
	return nil
}

func (s *TelegramBotService) DenyUser(chatID string) error {
	s.pending.Delete(chatID)
	return nil
}

func (s *TelegramBotService) RevokeUser(ctx context.Context, chatID string) error {
	s.approved.Delete(chatID)

	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	if cfg != nil {
		filtered := make([]TelegramUser, 0, len(cfg.ApprovedUsers))
		for _, u := range cfg.ApprovedUsers {
			if u.ChatID != chatID {
				filtered = append(filtered, u)
			}
		}
		cfg.ApprovedUsers = filtered
		if err := s.configSvc.Save(ctx, cfg); err != nil {
			mlog.Warning("telegram: failed to persist revoked user: ", err)
		}
	}
	return nil
}

func (s *TelegramBotService) UpdateConfig(ctx context.Context, workspace, mode string) error {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	if cfg == nil {
		return fmt.Errorf("telegram not connected")
	}
	cfg.DefaultWorkspace = workspace
	if mode != "" {
		cfg.VoiceResponseMode = mode
	}
	return s.configSvc.Save(ctx, cfg)
}
```

- [ ] **Step 2: Write pairing test**

```go
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramBotService_PairingFlow(t *testing.T) {
	svc := NewTelegramBotService(nil, nil, nil, nil, nil)

	// Simulate /start
	svc.pending.Store("123", &pendingPairing{Code: "000123", Username: "alice", FirstName: "Alice"})

	users := svc.PendingUsers()
	require.Len(t, users, 1)
	assert.Equal(t, "123", users[0].ChatID)

	// Approve
	ctx := context.Background()
	err := svc.ApproveUser(ctx, "123", "alice")
	require.NoError(t, err)

	assert.Empty(t, svc.PendingUsers())
	approved := svc.ApprovedUsers()
	require.Len(t, approved, 1)
	assert.Equal(t, "alice", approved[0].Username)
}
```

- [ ] **Step 3: Run test**

Run: `cd backend && go test -tags="fts5 nolancedb" ./internal/services -run TestTelegramBotService_PairingFlow -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/telegram_pairing.go backend/internal/services/telegram_pairing_test.go
git commit -m "feat(telegram): implement pairing approval flow"
```

---

## PR3: Regular Chat Integration

### Task 3.1: Implement command handlers

**Files:**
- Create: `backend/internal/services/telegram_commands.go`

- [ ] **Step 1: Write commands**

```go
package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

func (q *messageQueue) handleCommand(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	chatIDStr := fmt.Sprintf("%d", chatID)
	cmd := msg.Command()
	args := msg.CommandArguments()

	switch cmd {
	case "help":
		text := "*Available commands:*\n/start — Begin pairing\n/help — Show this message\n/switch — Select workspace/thread\n/history [n] — Show last n messages\n/model — Show current model\n/reset — Clear current thread history"
		_ = q.svc.sendText(chatID, text)
	case "switch":
		q.handleSwitch(chatID, chatIDStr)
	case "history":
		q.handleHistory(chatID, chatIDStr, args)
	case "model":
		q.handleModel(chatID, chatIDStr)
	case "reset":
		q.handleReset(chatID, chatIDStr)
	default:
		_ = q.svc.sendText(chatID, "Unknown command. Send /help for available commands.")
	}
}

func (q *messageQueue) handleSwitch(chatID int64, chatIDStr string) {
	var workspaces []models.Workspace
	if err := q.svc.db.Find(&workspaces).Error; err != nil {
		_ = q.svc.sendText(chatID, "❌ Failed to load workspaces.")
		return
	}
	if len(workspaces) == 0 {
		_ = q.svc.sendText(chatID, "No workspaces found.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, ws := range workspaces {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(ws.Name, fmt.Sprintf("ws:%s", ws.Slug)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "Select a workspace:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, _ = q.svc.bot.Send(msg)
}

func (q *messageQueue) handleHistory(chatID int64, chatIDStr string, args string) {
	limit := 10
	if args != "" {
		if n, err := strconv.Atoi(args); err == nil && n > 0 {
			limit = n
		}
	}
	user := q.getUser(chatIDStr)
	if user == nil {
		_ = q.svc.sendText(chatID, "❌ User not found.")
		return
	}
	ws, _ := q.resolveWorkspace(user.ActiveWorkspace)
	if ws == nil {
		_ = q.svc.sendText(chatID, "❌ No workspace selected. Use /switch first.")
		return
	}

	var threadID *int
	if user.ActiveThread != "" {
		if id, err := strconv.Atoi(user.ActiveThread); err == nil {
			threadID = &id
		}
	}

	var chats []models.WorkspaceChat
	query := q.svc.db.Where("workspace_id = ? AND include = ?", ws.ID, true)
	if threadID != nil {
		query = query.Where("thread_id = ?", *threadID)
	} else {
		query = query.Where("thread_id IS NULL")
	}
	if err := query.Order("id DESC").Limit(limit).Find(&chats).Error; err != nil {
		_ = q.svc.sendText(chatID, "❌ Failed to load history.")
		return
	}

	if len(chats) == 0 {
		_ = q.svc.sendText(chatID, "No messages in this thread.")
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("*Last %d messages:*\n\n", len(chats)))
	for i := len(chats) - 1; i >= 0; i-- {
		c := chats[i]
		b.WriteString(fmt.Sprintf("*You:* %s\n", c.Prompt))
		b.WriteString(fmt.Sprintf("*Bot:* %s\n\n", c.Response))
	}
	_ = q.svc.sendText(chatID, b.String())
}

func (q *messageQueue) handleModel(chatID int64, chatIDStr string) {
	user := q.getUser(chatIDStr)
	if user == nil {
		_ = q.svc.sendText(chatID, "❌ User not found.")
		return
	}
	ws, settings := q.resolveWorkspace(user.ActiveWorkspace)
	if ws == nil {
		_ = q.svc.sendText(chatID, "❌ No workspace selected.")
		return
	}
	provider := q.svc.cfg.LLMProvider
	if ws.ChatProvider != nil && *ws.ChatProvider != "" {
		provider = *ws.ChatProvider
	}
	model := q.svc.cfg.LLMModel
	if ws.ChatModel != nil && *ws.ChatModel != "" {
		model = *ws.ChatModel
	}
	_ = q.svc.sendText(chatID, fmt.Sprintf("*Current model:* %s / %s", provider, model))
}

func (q *messageQueue) handleReset(chatID int64, chatIDStr string) {
	user := q.getUser(chatIDStr)
	if user == nil {
		_ = q.svc.sendText(chatID, "❌ User not found.")
		return
	}
	ws, _ := q.resolveWorkspace(user.ActiveWorkspace)
	if ws == nil {
		_ = q.svc.sendText(chatID, "❌ No workspace selected.")
		return
	}
	var threadID *int
	if user.ActiveThread != "" {
		if id, err := strconv.Atoi(user.ActiveThread); err == nil {
			threadID = &id
		}
	}
	query := q.svc.db.Where("workspace_id = ?", ws.ID)
	if threadID != nil {
		query = query.Where("thread_id = ?", *threadID)
	} else {
		query = query.Where("thread_id IS NULL")
	}
	if err := query.Delete(&models.WorkspaceChat{}).Error; err != nil {
		_ = q.svc.sendText(chatID, "❌ Failed to reset history.")
		return
	}
	_ = q.svc.sendText(chatID, "✅ Chat history cleared.")
}

func (q *messageQueue) getUser(chatIDStr string) *TelegramUser {
	if u, ok := q.svc.approved.Load(chatIDStr); ok {
		user := u.(TelegramUser)
		return &user
	}
	return nil
}

func (q *messageQueue) resolveWorkspace(slug string) (*models.Workspace, map[string]string) {
	var ws models.Workspace
	if slug != "" {
		if err := q.svc.db.Where("slug = ?", slug).First(&ws).Error; err == nil {
			settings, _ := q.svc.sysSvc.GetAllSettings(context.Background())
			return &ws, settings
		}
	}
	// Fallback to first workspace
	if err := q.svc.db.First(&ws).Error; err != nil {
		return nil, nil
	}
	settings, _ := q.svc.sysSvc.GetAllSettings(context.Background())
	return &ws, settings
}
```

Wait, I need to add the `context` import:

```go
import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/models"
)
```

- [ ] **Step 2: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/telegram_commands.go
git commit -m "feat(telegram): add /help /switch /history /model /reset commands"
```

---

### Task 3.2: Implement regular chat handler

**Files:**
- Create: `backend/internal/services/telegram_chat.go`
- Modify: `backend/internal/services/telegram_queue.go` (add `handleChatMessage` call)

- [ ] **Step 1: Write chat handler**

```go
package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
)

func (s *TelegramBotService) handleChatMessage(ctx context.Context, msg *tgbotapi.Message, chatID int64, chatIDStr string) {
	user := s.getApprovedUser(chatIDStr)
	if user == nil {
		_ = s.sendText(chatID, "❌ Not authorized.")
		return
	}

	ws, _ := s.resolveWorkspaceForUser(user.ActiveWorkspace)
	if ws == nil {
		_ = s.sendText(chatID, "❌ No workspace found. Use /switch to select one.")
		return
	}

	var threadID *int
	if user.ActiveThread != "" {
		if id, err := strconv.Atoi(user.ActiveThread); err == nil {
			threadID = &id
		}
	}

	// Check @agent invocation
	prompt := msg.Text
	if prompt == "" && msg.Caption != "" {
		prompt = msg.Caption
	}
	if s.isAgentInvocation(prompt) {
		s.handleAgentInvocation(ctx, ws, user, threadID, prompt, chatID)
		return
	}

	// Regular chat
	req := dto.ChatRequest{Message: prompt}
	resp, err := s.chatSvc.Complete(ctx, ws, nil, threadID, req)
	if err != nil {
		mlog.Error("telegram chat error: ", err)
		_ = s.sendText(chatID, "❌ Sorry, something went wrong.")
		return
	}
	if resp.Error != "" {
		_ = s.sendText(chatID, "❌ "+resp.Error)
		return
	}
	_ = s.sendText(chatID, resp.TextResponse)
}

func (s *TelegramBotService) getApprovedUser(chatIDStr string) *TelegramUser {
	if u, ok := s.approved.Load(chatIDStr); ok {
		user := u.(TelegramUser)
		return &user
	}
	return nil
}

func (s *TelegramBotService) resolveWorkspaceForUser(slug string) (*models.Workspace, map[string]string) {
	var ws models.Workspace
	if slug != "" {
		if err := s.db.Where("slug = ?", slug).First(&ws).Error; err == nil {
			settings, _ := s.sysSvc.GetAllSettings(context.Background())
			return &ws, settings
		}
	}
	if err := s.db.First(&ws).Error; err != nil {
		return nil, nil
	}
	settings, _ := s.sysSvc.GetAllSettings(context.Background())
	return &ws, settings
}

func (s *TelegramBotService) isAgentInvocation(message string) bool {
	msg := strings.TrimLeft(message, " \t\n\r")
	return strings.HasPrefix(msg, "@agent")
}

func (s *TelegramBotService) handleAgentInvocation(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, prompt string, chatID int64) {
	_ = s.sendText(chatID, "🤖 Agent invocation is not yet available in this PR. Coming in PR4.")
}
```

- [ ] **Step 2: Wire `handleChatMessage` into queue processor**

In `backend/internal/services/telegram_queue.go`, the `process` method already calls `q.svc.handleChatMessage`. Good.

Wait, looking at my Task 2.1 code for `telegram_queue.go`, I see `q.svc.handleChatMessage(context.Background(), msg, q.chatID, chatIDStr)` is already there. Perfect.

But I need to make sure the `handleChatMessage` signature matches. It does.

- [ ] **Step 3: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/telegram_chat.go
git commit -m "feat(telegram): integrate regular chat with ChatService.Complete"
```

---

## PR4: Agent Runtime Refactor + @agent

### Task 4.1: Extract `AgentIO` and `AgentInput` interfaces

**Files:**
- Create: `backend/internal/agent/io.go`
- Modify: `backend/internal/agent/session.go`
- Modify: `backend/internal/agent/reader.go`
- Modify: `backend/internal/agent/approval.go`

- [ ] **Step 1: Define interfaces in `agent/io.go`**

```go
package agent

import "context"

// AgentIO is the transport-neutral output sink for session events.
type AgentIO interface {
	Send(frame ServerFrame) error
	Close() error
}

// InputType classifies actions coming from the user/transport.
type InputType int

const (
	InputContinue InputType = iota
	InputAbort
	InputToolApprovalResponse
	InputSetAutoApprove
)

// InputAction is a single user action delivered to the session reader.
type InputAction struct {
	Type        InputType
	Content     string
	RequestID   string
	Approved    bool
	AutoApprove bool
}

// AgentInput is the transport-neutral input source for user actions.
type AgentInput interface {
	Read(ctx context.Context) (InputAction, error)
}

// wsInput adapts wsConn into AgentInput for the existing WebSocket path.
type wsInput struct {
	conn *wsConn
}

func (w *wsInput) Read(ctx context.Context) (InputAction, error) {
	mt, raw, err := w.conn.ReadMessage()
	if err != nil {
		return InputAction{}, err
	}
	if mt != 1 { // websocket.TextMessage
		return InputAction{}, nil
	}

	trimmed := string(raw)
	if _, ok := bailCommands[trimmed]; ok {
		return InputAction{Type: InputAbort}, nil
	}

	var f ClientFrame
	if err := json.Unmarshal(raw, &f); err != nil {
		return InputAction{}, nil
	}

	switch f.Type {
	case FrameAwaitingFeedback:
		if _, ok := bailCommands[f.Feedback]; ok {
			return InputAction{Type: InputAbort}, nil
		}
		return InputAction{Type: InputContinue, Content: f.Feedback, RequestID: f.RequestID}, nil
	case FrameToolApprovalResp:
		return InputAction{Type: InputToolApprovalResponse, RequestID: f.RequestID, Approved: f.Approved}, nil
	case FrameSetAutoApprove:
		return InputAction{Type: InputSetAutoApprove, AutoApprove: f.Enabled}, nil
	default:
		return InputAction{}, nil
	}
}
```

Wait, I need to add the `json` import since `wsInput.Read` uses `json.Unmarshal`:

```go
import (
	"context"
	"encoding/json"

	"github.com/gorilla/websocket"
)
```

Actually, `wsConn.ReadMessage()` returns `(int, []byte, error)` where `int` is the message type. The existing `reader.go` uses `websocket.TextMessage`. Let me use the constant:

```go
func (w *wsInput) Read(ctx context.Context) (InputAction, error) {
	mt, raw, err := w.conn.ReadMessage()
	if err != nil {
		return InputAction{}, err
	}
	if mt != websocket.TextMessage {
		return InputAction{}, nil
	}
	// ... rest same
}
```

- [ ] **Step 2: Refactor `Session` to use `AgentIO`**

In `backend/internal/agent/session.go`:

Replace `wsConn *wsConn` with `io AgentIO`:

```go
type Session struct {
	UUID        string
	WorkspaceID int
	UserID      *int

	conv         *conversation.Conversation
	lm           core.LanguageModel
	systemPrompt string
	pAgent       *pantheonAgent.Agent

	io         AgentIO
	ctx        context.Context
	cancel     context.CancelFunc
	feedbackCh chan feedbackMsg
	terminated chan struct{}
	muteUser   bool

	startedAt time.Time
	once      sync.Once

	approvalsMu sync.Mutex
	approvals   map[string]chan approvalResp
	autoApprove atomic.Bool
	approvalTTL time.Duration

	eventLog eventLogger
}
```

Update `newSession` signature and body:

```go
func newSession(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
	lm core.LanguageModel, systemPrompt string, reg *tool.Registry, io AgentIO, approvalTTL time.Duration, eventLog eventLogger) *Session {

	ctx, cancel := context.WithCancel(parentCtx)
	s := &Session{
		UUID:         uuid,
		WorkspaceID:  ws.ID,
		lm:           lm,
		systemPrompt: systemPrompt,
		io:           io,
		ctx:          ctx,
		cancel:       cancel,
		feedbackCh:   make(chan feedbackMsg, 1),
		terminated:   make(chan struct{}),
		muteUser:     true,
		startedAt:    time.Now(),
		approvals:    make(map[string]chan approvalResp),
		approvalTTL:  approvalTTL,
		eventLog:     eventLog,
	}
	// ... rest identical
}
```

Update `NewSessionForTesting`:

```go
func NewSessionForTesting(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
	lm core.LanguageModel, systemPrompt string, reg *tool.Registry, io AgentIO) *Session {
	return newSession(parentCtx, uuid, ws, user, lm, systemPrompt, reg, io, 2*time.Minute, nil)
}
```

Update `Abort`:

```go
func (s *Session) Abort(reason string) {
	if reason != "" {
		_ = s.io.Send(ServerFrame{Type: FrameWSSFailure, Content: reason})
	}
	s.cancelAllApprovals(reason)
	s.cancel()
}
```

- [ ] **Step 3: Refactor `reader.go` to use `AgentInput`**

Replace the entire `backend/internal/agent/reader.go`:

```go
package agent

import (
	"github.com/odysseythink/mlog"
)

var bailCommands = map[string]struct{}{
	"exit": {}, "/exit": {}, "stop": {}, "/stop": {}, "halt": {}, "/halt": {}, "/reset": {},
}

// readerLoopWithInput runs in a goroutine until Session ctx is cancelled or input errors.
func (s *Session) readerLoopWithInput(input AgentInput) {
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("agent reader panic: ", r)
			s.Abort("internal reader panic")
		}
	}()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		action, err := input.Read(s.ctx)
		if err != nil {
			s.cancel()
			return
		}

		switch action.Type {
		case InputAbort:
			s.Abort("")
			return
		case InputContinue:
			s.Continue(action.Content, nil)
		case InputToolApprovalResponse:
			if action.RequestID == "" {
				mlog.Warning("agent: toolApprovalResponse with empty requestId")
				continue
			}
			s.handleApprovalResponse(action.RequestID, action.Approved)
		case InputSetAutoApprove:
			s.SetAutoApprove(action.AutoApprove)
			mlog.Info("agent: setAutoApprove → ", action.AutoApprove)
		}
	}
}
```

- [ ] **Step 4: Update `approval.go` to use `io.Send`**

In `backend/internal/agent/approval.go`, change line 42:

```go
	if err := s.io.Send(ServerFrame{
```

- [ ] **Step 5: Update `handler.go` `HandleWS` to use new abstractions**

In `backend/internal/agent/handler.go`, change the session creation line:

```go
	sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), wc, ttl, r.deps.EventLog)
```

And change the goroutine launch:

```go
	go sess.readerLoopWithInput(&wsInput{conn: wc})
```

Also update the deferred cleanup: `wc.Close()` stays the same because `wsConn.Close()` is called, and `AgentIO.Close()` will be called too via `io.Close()` in `RunAgentDirectly`.

Actually, in `HandleWS` the defer already calls `wc.Close()`. That's fine because `wc` is `*wsConn` which implements `AgentIO`.

- [ ] **Step 6: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 7: Run existing agent tests**

Run: `cd backend && go test -tags="fts5 nolancedb" ./internal/agent -v`
Expected: Some tests may fail if they use `NewSessionForTesting` with `*wsConn`. Update them.

If `agent/session_test.go` or similar exists and uses the old signature, update it.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/agent/io.go backend/internal/agent/session.go backend/internal/agent/reader.go backend/internal/agent/approval.go backend/internal/agent/handler.go
git commit -m "refactor(agent): introduce AgentIO and AgentInput abstractions"
```

---

### Task 4.2: Add `RunAgentDirectly` and Telegram I/O

**Files:**
- Modify: `backend/internal/agent/runtime.go`
- Create: `backend/internal/agent/telegram_io.go`
- Modify: `backend/internal/services/telegram_chat.go` (wire agent)

- [ ] **Step 1: Add `RunAgentDirectly` to `Runtime`**

In `backend/internal/agent/runtime.go`, add after `LanguageModelFor`:

```go
func (r *Runtime) RunAgentDirectly(ctx context.Context, invUUID string, io AgentIO, input AgentInput) error {
	inv, err := r.GetInvocation(ctx, invUUID)
	if err != nil {
		return err
	}

	var ws models.Workspace
	if err := r.deps.DB.WithContext(ctx).First(&ws, inv.WorkspaceID).Error; err != nil {
		return err
	}
	var user *models.User
	if inv.UserID != nil {
		user = &models.User{ID: *inv.UserID}
	}

	var settings map[string]string
	if r.deps.SysSvc != nil {
		settings, _ = r.deps.SysSvc.GetAllSettings(ctx)
	}
	lm, err := r.LanguageModelFor(&ws, settings)
	if err != nil {
		_ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: "agent: " + err.Error()})
		return err
	}
	systemPrompt := resolveSystemPrompt(&ws, user)

	ttl := r.deps.Cfg.AgentToolApprovalTimeout
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	sessTTL := r.deps.Cfg.AgentSessionMaxDuration
	if sessTTL <= 0 {
		sessTTL = 30 * time.Minute
	}
	sessCtx, sessCancel := context.WithTimeout(ctx, sessTTL)
	defer sessCancel()

	sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), io, ttl, r.deps.EventLog)

	reg, err := buildSessionRegistry(ctx, r.deps, &ws, user, lm, settings, nil, sess.RequestApproval)
	if err != nil {
		_ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: "tools: " + err.Error()})
		return err
	}
	sess.pAgent = pantheonAgent.New(lm,
		pantheonAgent.WithRegistry(reg),
		pantheonAgent.WithMaxSteps(10),
	)
	sess.conv.RegisterParticipant(&conversation.Participant{
		Name:  participantAgent,
		Role:  systemPrompt,
		Agent: sess.pAgent,
	})

	r.sessions.Store(inv.UUID, sess)
	var userID *int
	if user != nil {
		userID = &user.ID
	}
	logChatStarted(r.deps.EventLog, userID, inv.UUID, ws.ID, lm.Provider(), lm.Model())

	var runErr error
	defer func() {
		duration := time.Since(sess.startedAt)
		reason := "normal"
		if runErr != nil {
			if errors.Is(runErr, context.DeadlineExceeded) {
				reason = "timeout"
			} else if errors.Is(runErr, context.Canceled) {
				reason = "cancelled"
			} else {
				reason = "error"
			}
		}
		logChatTerminated(r.deps.EventLog, userID, inv.UUID, reason, duration)
		r.sessions.Delete(inv.UUID)
		_ = r.CloseInvocation(context.Background(), inv.UUID)
		io.Close()
	}()

	_ = io.Send(ServerFrame{Type: FrameStatusResponse, Content: "@agent runtime ready"})

	go sess.readerLoopWithInput(input)
	runErr = sess.Run(inv.Prompt)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		content := runErr.Error()
		if errors.Is(runErr, context.DeadlineExceeded) || content == "context deadline exceeded" {
			content = "Session reached maximum duration (" + sessTTL.String() + "). Ending now."
		}
		_ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: content})
	}
	select {
	case <-sess.terminated:
	case <-sess.ctx.Done():
	}
	return runErr
}
```

Add the missing imports to `runtime.go` if not present:
- `errors`
- `time`
- `context`
- `github.com/odysseythink/pantheon/agent` (already imported as `pantheonAgent`)
- `github.com/odysseythink/pantheon/conversation`

- [ ] **Step 2: Create `telegramAgentIO` and `telegramInput`**

Create `backend/internal/agent/telegram_io.go`:

```go
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// TelegramAgentIO implements AgentIO for Telegram. It accumulates text chunks
// and flushes the complete message on FrameFinalizeResponseStream.
type TelegramAgentIO struct {
	sendTextFn      func(text string) error
	sendApprovalFn  func(requestID, skillName, description string, timeoutMs int) error
	mu              sync.Mutex
	builder         strings.Builder
	pendingApproval bool
}

func NewTelegramAgentIO(sendText func(text string) error, sendApproval func(requestID, skillName, description string, timeoutMs int) error) *TelegramAgentIO {
	return &TelegramAgentIO{
		sendTextFn:     sendText,
		sendApprovalFn: sendApproval,
	}
}

const telegramMaxMsgLen = 4096

func (t *TelegramAgentIO) Send(frame ServerFrame) error {
	switch frame.Type {
	case FrameStatusResponse:
		return t.sendTextFn(frame.Content)
	case FrameTextResponseChunk:
		t.mu.Lock()
		t.builder.WriteString(frame.Content)
		t.mu.Unlock()
		return nil
	case FrameFinalizeResponseStream:
		t.mu.Lock()
		text := t.builder.String()
		t.builder.Reset()
		t.mu.Unlock()
		if text == "" {
			return nil
		}
		for len(text) > 0 {
			chunk := text
			if len(chunk) > telegramMaxMsgLen {
				chunk = chunk[:telegramMaxMsgLen]
			}
			if err := t.sendTextFn(chunk); err != nil {
				return err
			}
			text = text[len(chunk):]
		}
		return nil
	case FrameToolApprovalReq:
		return t.sendApprovalFn(frame.RequestID, frame.SkillName, frame.Description, frame.TimeoutMs)
	case FrameWSSFailure:
		return t.sendTextFn("❌ " + frame.Content)
	}
	return nil
}

func (t *TelegramAgentIO) Close() error { return nil }

// TelegramInput implements AgentInput for Telegram using channels.
type TelegramInput struct {
	ch chan InputAction
}

func NewTelegramInput() *TelegramInput {
	return &TelegramInput{ch: make(chan InputAction, 4)}
}

func (t *TelegramInput) Read(ctx context.Context) (InputAction, error) {
	select {
	case <-ctx.Done():
		return InputAction{}, ctx.Err()
	case act := <-t.ch:
		return act, nil
	}
}

func (t *TelegramInput) Submit(action InputAction) {
	select {
	case t.ch <- action:
	default:
	}
}
```

- [ ] **Step 3: Wire agent callback in `TelegramBotService`**

In `backend/internal/services/telegram_bot.go`, add to the struct:

```go
	agentCallback TelegramAgentCallback
```

Add setter:

```go
func (s *TelegramBotService) SetAgentCallback(cb TelegramAgentCallback) {
	s.agentCallback = cb
}
```

In `backend/internal/services/telegram_chat.go`, replace the stub `handleAgentInvocation`:

```go
func (s *TelegramBotService) handleAgentInvocation(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, prompt string, chatID int64) {
	if s.agentCallback == nil {
		_ = s.sendText(chatID, "❌ Agent runtime is not configured.")
		return
	}

	// Create invocation
	invUUID, err := s.createAgentInvocation(ctx, ws, threadID, prompt)
	if err != nil {
		mlog.Error("telegram agent invocation failed: ", err)
		_ = s.sendText(chatID, "❌ Failed to start agent.")
		return
	}

	// Register approval handler for this chat
	input := agent.NewTelegramInput() // can't import agent here!
	// ...
}
```

Wait, we have the import cycle issue. `services` cannot import `agent`.

The solution: `main.go` creates the `TelegramInput`, registers it as an `ApprovalHandler`, and passes it to the callback. `services` only deals with the callback.

So `handleAgentInvocation` should just call the callback:

```go
func (s *TelegramBotService) handleAgentInvocation(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, prompt string, chatID int64) {
	if s.agentCallback == nil {
		_ = s.sendText(chatID, "❌ Agent runtime is not configured.")
		return
	}

	invUUID, err := s.createAgentInvocation(ctx, ws, threadID, prompt)
	if err != nil {
		mlog.Error("telegram agent invocation failed: ", err)
		_ = s.sendText(chatID, "❌ Failed to start agent.")
		return
	}

	err = s.agentCallback(ctx, invUUID, chatID,
		func(text string) error { return s.sendText(chatID, text) },
		func(requestID, skillName, description string, timeoutMs int) error {
			return s.sendApprovalReq(chatID, requestID, skillName, description, timeoutMs)
		},
	)
	if err != nil {
		mlog.Error("telegram agent execution failed: ", err)
		_ = s.sendText(chatID, "❌ Agent execution failed: "+err.Error())
	}
}

func (s *TelegramBotService) createAgentInvocation(ctx context.Context, ws *models.Workspace, threadID *int, prompt string) (string, error) {
	inv := &models.WorkspaceAgentInvocation{
		UUID:        uuid.NewString(),
		WorkspaceID: ws.ID,
		Prompt:      prompt,
	}
	if threadID != nil {
		inv.ThreadID = threadID
	}
	if err := s.db.WithContext(ctx).Create(inv).Error; err != nil {
		return "", err
	}
	return inv.UUID, nil
}
```

Wait, `uuid` is not imported in `telegram_chat.go`. Add it.

- [ ] **Step 4: Wire everything in `main.go`**

In `backend/cmd/server/main.go`, after `chatSvc` creation:

```go
	telegramSvc := services.NewTelegramBotService(db, cfg, sysSvc, enc, chatSvc)
	if err := telegramSvc.Boot(context.Background()); err != nil {
		mlog.Warning("telegram boot failed", mlog.Err(err))
	}
	// Bridge agent.Runtime → TelegramBotService without import cycle
	telegramSvc.SetAgentCallback(func(ctx context.Context, invUUID string, chatID int64, sendText func(text string) error, sendApprovalReq func(requestID, skillName, description string, timeoutMs int) error) error {
		io := agent.NewTelegramAgentIO(sendText, sendApprovalReq)
		input := agent.NewTelegramInput()
		chatIDStr := fmt.Sprintf("%d", chatID)
		telegramSvc.RegisterApprovalHandler(chatIDStr, &telegramApprovalAdapter{input: input})
		defer telegramSvc.UnregisterApprovalHandler(chatIDStr)
		return agentRuntime.RunAgentDirectly(ctx, invUUID, io, input)
	})
```

Add the helper type and methods:

In `backend/cmd/server/main.go` (before `main()`):

```go
type telegramApprovalAdapter struct {
	input *agent.TelegramInput
}

func (a *telegramApprovalAdapter) HandleApproval(requestID string, approved bool) {
	a.input.Submit(agent.InputAction{
		Type:      agent.InputToolApprovalResponse,
		RequestID: requestID,
		Approved:  approved,
	})
}
```

And add to `TelegramBotService`:

```go
func (s *TelegramBotService) RegisterApprovalHandler(chatID string, h ApprovalHandler) {
	s.approvalHandlers.Store(chatID, h)
}

func (s *TelegramBotService) UnregisterApprovalHandler(chatID string) {
	s.approvalHandlers.Delete(chatID)
}
```

Also in `main.go`, update the route registration:

```go
	handlers.RegisterTelegramRoutes(api, cfg, authSvc, telegramSvc)
```

And add to imports:
- `fmt` (already there)
- `github.com/odysseythink/hermind/backend/internal/agent`

- [ ] **Step 5: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 6: Run agent tests**

Run: `cd backend && go test -tags="fts5 nolancedb" ./internal/agent -v`
Expected: PASS (after fixing any signature changes)

- [ ] **Step 7: Commit**

```bash
git add backend/internal/agent/runtime.go backend/internal/agent/telegram_io.go backend/internal/services/telegram_bot.go backend/internal/services/telegram_chat.go backend/cmd/server/main.go
git commit -m "feat(telegram): add RunAgentDirectly and Telegram I/O adapters"
```

---

## PR5: Media + TTS

### Task 5.1: Implement media handlers

**Files:**
- Create: `backend/internal/services/telegram_media.go`
- Modify: `backend/internal/services/telegram_queue.go` (add media routing)

- [ ] **Step 1: Write media handler**

```go
package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
)

func (q *messageQueue) handleMedia(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	chatIDStr := fmt.Sprintf("%d", chatID)
	user := q.getUser(chatIDStr)
	if user == nil {
		_ = q.svc.sendText(chatID, "❌ Not authorized.")
		return
	}
	ws, _ := q.resolveWorkspace(user.ActiveWorkspace)
	if ws == nil {
		_ = q.svc.sendText(chatID, "❌ No workspace found.")
		return
	}
	var threadID *int
	if user.ActiveThread != "" {
		if id, err := strconv.Atoi(user.ActiveThread); err == nil {
			threadID = &id
		}
	}

	ctx := context.Background()
	voiceMode := false

	switch {
	case msg.Voice != nil:
		voiceMode = true
		text, err := q.svc.handleVoice(ctx, msg.Voice)
		if err != nil {
			_ = q.svc.sendText(chatID, "❌ Failed to process voice: "+err.Error())
			return
		}
		q.svc.handleChatMessage(ctx, &tgbotapi.Message{Text: text, Chat: msg.Chat}, chatID, chatIDStr)
	case msg.Photo != nil && len(msg.Photo) > 0:
		err := q.svc.handlePhoto(ctx, ws, user, threadID, msg)
		if err != nil {
			_ = q.svc.sendText(chatID, "❌ Failed to process photo: "+err.Error())
		}
	case msg.Document != nil:
		err := q.svc.handleDocument(ctx, ws, user, threadID, msg)
		if err != nil {
			_ = q.svc.sendText(chatID, "❌ Failed to process document: "+err.Error())
		}
	default:
		_ = q.svc.sendText(chatID, "❌ Unsupported message type.")
	}

	_ = voiceMode // used for TTS mirror mode later
}

func (s *TelegramBotService) handleVoice(ctx context.Context, voice *tgbotapi.Voice) (string, error) {
	file, err := s.bot.GetFile(tgbotapi.FileConfig{FileID: voice.FileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}
	data, err := s.downloadFile(file.Link(s.bot.Token))
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	mlog.Info("telegram: voice message downloaded, size=", len(data))
	// TODO: STT via Collector when available
	_ = data
	return "[Voice message transcription not yet available]", nil
}

func (s *TelegramBotService) handlePhoto(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, msg *tgbotapi.Message) error {
	// Get largest photo
	photo := msg.Photo[len(msg.Photo)-1]
	file, err := s.bot.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		return err
	}
	data, err := s.downloadFile(file.Link(s.bot.Token))
	if err != nil {
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", b64)

	prompt := msg.Caption
	if prompt == "" {
		prompt = "Describe this image."
	}
	req := dto.ChatRequest{
		Message:     prompt,
		Attachments: []string{dataURL},
	}
	resp, err := s.chatSvc.Complete(ctx, ws, nil, threadID, req)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}
	chatID, _ := strconv.ParseInt(user.ChatID, 10, 64)
	return s.sendText(chatID, resp.TextResponse)
}

func (s *TelegramBotService) handleDocument(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, msg *tgbotapi.Message) error {
	doc := msg.Document
	file, err := s.bot.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
	if err != nil {
		return err
	}
	data, err := s.downloadFile(file.Link(s.bot.Token))
	if err != nil {
		return err
	}
	mlog.Info("telegram: document downloaded, name=", doc.FileName, " size=", len(data))
	// TODO: parse via Collector when ready
	_ = data

	prompt := msg.Caption
	if prompt == "" {
		prompt = fmt.Sprintf("The user shared a document named %s.", doc.FileName)
	} else {
		prompt = fmt.Sprintf("The user shared a document named %s. User request: %s", doc.FileName, prompt)
	}
	req := dto.ChatRequest{Message: prompt}
	resp, err := s.chatSvc.Complete(ctx, ws, nil, threadID, req)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}
	chatID, _ := strconv.ParseInt(user.ChatID, 10, 64)
	return s.sendText(chatID, resp.TextResponse)
}

func (s *TelegramBotService) downloadFile(url string) ([]byte, error) {
	// SSRF guard: whitelist Telegram API domain
	if path.Dir(url) != "/" && !strings.HasPrefix(url, "https://api.telegram.org/file/") {
		return nil, fmt.Errorf("invalid file URL")
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

Wait, `path` and `strings` imports. Add them.

Actually, the URL check is wrong. `path.Dir(url)` won't work like that. Simpler:

```go
func (s *TelegramBotService) downloadFile(url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://api.telegram.org/file/") {
		return nil, fmt.Errorf("invalid file URL: not from Telegram API")
	}
	// ...
}
```

- [ ] **Step 2: Add media routing in queue processor**

In `backend/internal/services/telegram_queue.go`, update `process`:

```go
func (q *messageQueue) process(u tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("telegram queue panic: ", r)
		}
	}()

	msg := u.Message
	chatIDStr := fmt.Sprintf("%d", q.chatID)

	// Pairing check
	if !q.isApproved(chatIDStr) {
		if msg.IsCommand() && msg.Command() == "start" {
			q.handleStart(msg)
		} else {
			_ = q.svc.sendText(q.chatID, "Please send /start to begin pairing.")
		}
		return
	}

	if msg.IsCommand() {
		q.handleCommand(msg)
		return
	}

	// Media messages
	if msg.Voice != nil || (msg.Photo != nil && len(msg.Photo) > 0) || msg.Document != nil {
		q.handleMedia(msg)
		return
	}

	// Regular text
	q.svc.handleChatMessage(context.Background(), msg, q.chatID, chatIDStr)
}
```

- [ ] **Step 3: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/telegram_media.go backend/internal/services/telegram_queue.go
git commit -m "feat(telegram): add voice, photo, and document message handling"
```

---

### Task 5.2: Implement TTS voice replies

**Files:**
- Create: `backend/internal/services/telegram_tts.go`
- Modify: `backend/internal/services/telegram_chat.go` (call TTS after chat response)
- Modify: `backend/internal/services/telegram_media.go` (call TTS after photo/doc response)

- [ ] **Step 1: Add TTS service dependency**

In `backend/internal/services/telegram_bot.go`, add to the constructor:

```go
func NewTelegramBotService(db *gorm.DB, cfg *config.Config, sysSvc *SystemService, enc *utils.EncryptionManager, chatSvc *ChatService, ttsProvider tts.Provider) *TelegramBotService {
	return &TelegramBotService{
		db:          db,
		cfg:         cfg,
		sysSvc:      sysSvc,
		enc:         enc,
		chatSvc:     chatSvc,
		ttsProvider: ttsProvider,
		configSvc:   NewTelegramConfigService(db, enc),
		stopCh:      make(chan struct{}),
	}
}
```

Add to struct:

```go
	ttsProvider tts.Provider
```

Update `main.go` to pass the TTS provider:

```go
	telegramSvc := services.NewTelegramBotService(db, cfg, sysSvc, enc, chatSvc, ttsProvider)
```

- [ ] **Step 2: Write TTS helper**

Create `backend/internal/services/telegram_tts.go`:

```go
package services

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/tts"
	"github.com/odysseythink/mlog"
)

func (s *TelegramBotService) maybeSendVoiceReply(chatID int64, text string, forceVoice bool) {
	if s.ttsProvider == nil || !s.ttsProvider.Available() {
		return
	}
	if !forceVoice {
		s.mu.RLock()
		mode := ""
		if s.config != nil {
			mode = s.config.VoiceResponseMode
		}
		s.mu.RUnlock()
		if mode == "text_only" {
			return
		}
		// mirror mode: only voice if user sent voice (caller must set forceVoice)
		if mode == "mirror" && !forceVoice {
			return
		}
	}

	go func() {
		synth, err := s.ttsProvider.Synthesize(context.Background(), text)
		if err != nil {
			mlog.Warning("telegram tts failed: ", err)
			return
		}
		_ = s.sendVoice(chatID, synth.Audio, synth.ContentType)
	}()
}

func (s *TelegramBotService) sendVoice(chatID int64, audio []byte, contentType string) error {
	// Telegram voice messages require .ogg with Opus. If TTS returns MP3, we send as audio document.
	// For now, send as audio document regardless.
	_ = audio
	_ = contentType
	_ = chatID
	// TODO: use tgbotapi.NewAudioShare or tgbotapi.NewVoiceUpload when media group APIs are wired
	mlog.Info("telegram: would send voice message, length=", len(audio))
	return nil
}
```

- [ ] **Step 3: Call TTS after chat responses**

In `backend/internal/services/telegram_chat.go`, update `handleChatMessage` after sending text:

```go
	_ = s.sendText(chatID, resp.TextResponse)
	// TTS
	s.maybeSendVoiceReply(chatID, resp.TextResponse, false)
```

Similarly in `telegram_media.go`, after `handlePhoto` and `handleDocument` send text, call `s.maybeSendVoiceReply(chatID, resp.TextResponse, false)`. For voice messages, call with `true`:

```go
	case msg.Voice != nil:
		voiceMode = true
		text, err := q.svc.handleVoice(ctx, msg.Voice)
		// ...
		q.svc.handleChatMessage(ctx, &tgbotapi.Message{Text: text, Chat: msg.Chat}, chatID, chatIDStr)
		// TTS mirror: the chat handler will send text; we request voice reply
		// Actually, the voice reply should happen after the LLM response, not here.
```

Better: modify `handleChatMessage` to accept a `forceVoice` parameter:

```go
func (s *TelegramBotService) handleChatMessage(ctx context.Context, msg *tgbotapi.Message, chatID int64, chatIDStr string, forceVoice bool) {
	// ... same until after sending text
	_ = s.sendText(chatID, resp.TextResponse)
	s.maybeSendVoiceReply(chatID, resp.TextResponse, forceVoice)
}
```

And update all call sites accordingly.

- [ ] **Step 4: Build to verify**

Run: `cd backend && go build -tags="fts5 nolancedb" ./...`
Expected: clean compile

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/telegram_tts.go backend/internal/services/telegram_bot.go backend/internal/services/telegram_chat.go backend/internal/services/telegram_media.go backend/cmd/server/main.go
git commit -m "feat(telegram): add TTS voice reply support"
```

---

## Self-Review

**1. Spec coverage:**

| Design Requirement | Implementing Task |
|---|---|
| Data model (`external_communication_connectors`) | Task 1.1 |
| Config encryption (AES-GCM) | Task 1.2 |
| HTTP routes (9 routes) | Task 1.3 |
| Polling loop | Task 2.1 |
| Per-chat queue | Task 2.1 |
| Pairing flow | Task 2.2 |
| `/start`, `/help`, `/switch`, `/history`, `/model`, `/reset` | Task 3.1 |
| Regular text → `ChatService.Complete` | Task 3.2 |
| `AgentIO` / `AgentInput` abstraction | Task 4.1 |
| `RunAgentDirectly` | Task 4.2 |
| `@agent` invocation for Telegram | Task 4.2 |
| Inline keyboard tool approval | Task 2.1 (callback routing) + Task 4.2 (adapter) |
| Voice messages (STT placeholder) | Task 5.1 |
| Photo messages (vision) | Task 5.1 |
| Document messages (parse placeholder) | Task 5.1 |
| TTS voice replies | Task 5.2 |
| Single-user mode guard | `singleUserMode` middleware (existing) + runtime check |
| 401 self-cleanup | Design says polling error handling; needs to be added in Task 2.1 retry logic |

**Gap found:** 401 self-cleanup and exponential backoff retry are mentioned in the design but not explicitly implemented in the tasks above. Add a sub-task.

**2. Placeholder scan:**
- "TODO: STT via Collector" — acceptable as a graceful degradation; the plan explicitly says placeholder
- "TODO: parse via Collector" — same
- "TODO: use tgbotapi.NewAudioShare" — acceptable for Phase 1; text reply works

No other placeholders found.

**3. Type consistency:**
- `TelegramBotService` constructor signature changes in Task 5.1 (adds `ttsProvider`). Call site in `main.go` is updated.
- `handleChatMessage` signature gains `forceVoice bool` in Task 5.2. All call sites updated.
- `AgentIO.Send` takes `ServerFrame` consistently across `wsConn`, `TelegramAgentIO`, and `Session`.
- `InputAction` fields match what `readerLoopWithInput` expects.

---

## Post-Review Fix: Add Polling Retry + 401 Self-Cleanup

### Task 2.1b: Add polling resilience

**Files:**
- Modify: `backend/internal/services/telegram_bot.go`

- [ ] **Step 1: Replace `pollLoop` with retry logic**

Replace the `pollLoop` method in `telegram_bot.go`:

```go
func (s *TelegramBotService) pollLoop() {
	defer s.wg.Done()

	baseDelay := time.Second
	maxRetries := 10
	capDelay := 5 * time.Minute
	retries := 0

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		u := tgbotapi.NewUpdate(0)
		u.Timeout = 30
		updates, err := s.bot.GetUpdates(u)
		if err != nil {
			mlog.Warning("telegram polling error: ", err)
			if strings.Contains(err.Error(), "Unauthorized") {
				s.selfCleanup(context.Background())
				return
			}
			retries++
			if retries > maxRetries {
				mlog.Error("telegram: max retries exceeded, stopping polling")
				return
			}
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(retries-1)))
			if delay > capDelay {
				delay = capDelay
			}
			time.Sleep(delay)
			continue
		}
		retries = 0

		for _, update := range updates {
			if update.Message != nil {
				s.handleUpdate(update)
			} else if update.CallbackQuery != nil {
				s.handleCallback(update.CallbackQuery)
			}
		}
		if len(updates) > 0 {
			s.bot.Offset = updates[len(updates)-1].UpdateID + 1
		}
	}
}
```

Add imports: `math`, `strings`, `time` (time already there).

Wait, `tgbotapi.BotAPI.GetUpdates` takes `UpdateConfig` and returns `([]Update, error)`. This is synchronous polling, not channel-based. We should use this instead of `GetUpdatesChan` for better error handling.

Also, we need to handle startup backlog cleanup. Add at the start of `startWithConfig` or `pollLoop`:

```go
// Startup backlog cleanup: fetch pending updates with limit=100, timeout=0
// and acknowledge all of them.
func (s *TelegramBotService) clearBacklog() {
	u := tgbotapi.NewUpdate(0)
	u.Limit = 100
	u.Timeout = 0
	updates, _ := s.bot.GetUpdates(u)
	if len(updates) > 0 {
		lastID := updates[len(updates)-1].UpdateID
		s.bot.Offset = lastID + 1
	}
}
```

Call this in `startWithConfig` before starting `pollLoop`.

- [ ] **Step 2: Commit**

```bash
git add backend/internal/services/telegram_bot.go
git commit -m "feat(telegram): add exponential backoff polling retry and 401 self-cleanup"
```

---

## Execution Handoff

**Plan complete and saved to `.gpowers/plans/2026-05-28-telegram-integration.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using `executing-plans`, batch execution with checkpoints

**Which approach?**
