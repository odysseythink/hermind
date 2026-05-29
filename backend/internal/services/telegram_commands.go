package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/models"
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
	ws, _ := q.resolveWorkspace(user.ActiveWorkspace)
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
