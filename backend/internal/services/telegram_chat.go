package services

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
)

func (s *TelegramBotService) handleChatMessage(ctx context.Context, msg *tgbotapi.Message, chatID int64, chatIDStr string, forceVoice bool) {
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
	s.maybeSendVoiceReply(chatID, resp.TextResponse, forceVoice)
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
