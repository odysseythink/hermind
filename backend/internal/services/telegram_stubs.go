package services

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/models"
)

// Stubs for methods implemented in later PRs. These are overwritten by
// telegram_pairing.go, telegram_commands.go, telegram_chat.go, telegram_media.go.

func (q *messageQueue) handleStart(msg *tgbotapi.Message) {}

func (q *messageQueue) handleCommand(msg *tgbotapi.Message) {}

func (q *messageQueue) handleMedia(msg *tgbotapi.Message) {}

func (s *TelegramBotService) handleChatMessage(ctx context.Context, msg *tgbotapi.Message, chatID int64, chatIDStr string) {}

func (q *messageQueue) getUser(chatIDStr string) *TelegramUser         { return nil }
func (q *messageQueue) resolveWorkspace(slug string) (*models.Workspace, map[string]string) {
	return nil, nil
}
