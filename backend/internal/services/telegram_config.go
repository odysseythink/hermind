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
