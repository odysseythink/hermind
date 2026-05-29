package services

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	offset    int

	mu               sync.RWMutex
	pending          sync.Map // string(chatID) → *pendingPairing
	approved         sync.Map // string(chatID) → TelegramUser
	queues           sync.Map // int64(chatID) → *messageQueue
	approvalHandlers sync.Map // string(chatID) → ApprovalHandler

	agentCallback TelegramAgentCallback

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

func (s *TelegramBotService) SetAgentCallback(cb TelegramAgentCallback) {
	s.agentCallback = cb
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

	// Startup backlog cleanup
	s.clearBacklog()

	s.wg.Add(1)
	go s.pollLoop()
	return nil
}

func (s *TelegramBotService) clearBacklog() {
	u := tgbotapi.NewUpdate(0)
	u.Limit = 100
	u.Timeout = 0
	updates, _ := s.bot.GetUpdates(u)
	if len(updates) > 0 {
		s.offset = updates[len(updates)-1].UpdateID + 1
	}
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

		u := tgbotapi.NewUpdate(s.offset)
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
			s.offset = updates[len(updates)-1].UpdateID + 1
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
	if s.bot == nil {
		return nil
	}
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

func (s *TelegramBotService) RegisterApprovalHandler(chatID string, h ApprovalHandler) {
	s.approvalHandlers.Store(chatID, h)
}

func (s *TelegramBotService) UnregisterApprovalHandler(chatID string) {
	s.approvalHandlers.Delete(chatID)
}

func (s *TelegramBotService) createAgentInvocation(ctx context.Context, ws *models.Workspace, threadID *int, prompt string) (string, error) {
	inv := &models.WorkspaceAgentInvocation{
		UUID:        fmt.Sprintf("%d", rand.Int63()),
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

// ---- Pairing & User Management (PR2 Task 2.2) ----

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

	cid, _ := strconv.ParseInt(chatID, 10, 64)
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
