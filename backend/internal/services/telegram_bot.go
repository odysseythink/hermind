package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type TelegramBotService struct{}

func NewTelegramBotService(db *gorm.DB, cfg *config.Config, sysSvc *SystemService, enc *utils.EncryptionManager, chatSvc *ChatService) *TelegramBotService {
	return &TelegramBotService{}
}

func (s *TelegramBotService) Start(ctx context.Context, token string) error { return nil }
func (s *TelegramBotService) Stop(ctx context.Context) error                { return nil }
func (s *TelegramBotService) Boot(ctx context.Context) error                { return nil }
func (s *TelegramBotService) Status() (bool, string)                        { return false, "" }
func (s *TelegramBotService) GetConfig(ctx context.Context) (*TelegramConfig, error) {
	return nil, nil
}
func (s *TelegramBotService) PendingUsers() []TelegramUser                  { return nil }
func (s *TelegramBotService) ApprovedUsers() []TelegramUser                 { return nil }
func (s *TelegramBotService) ApproveUser(ctx context.Context, chatID, username string) error {
	return nil
}
func (s *TelegramBotService) DenyUser(chatID string) error                  { return nil }
func (s *TelegramBotService) RevokeUser(ctx context.Context, chatID string) error { return nil }
func (s *TelegramBotService) UpdateConfig(ctx context.Context, workspace, mode string) error {
	return nil
}
