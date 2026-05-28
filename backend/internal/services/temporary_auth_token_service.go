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
	return s.IssueWithTTL(ctx, userID, time.Hour)
}

func (s *TemporaryAuthTokenService) IssueWithTTL(ctx context.Context, userID int, ttl time.Duration) (string, error) {
	if userID == 0 {
		return "", fmt.Errorf("user ID is required")
	}
	if ttl <= 0 || ttl > time.Hour {
		return "", fmt.Errorf("ttl must be in (0, 1h]")
	}
	_ = s.InvalidateUserTokens(ctx, userID)

	token := models.TemporaryAuthToken{
		Token:     s.makeTempToken(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&token).Error; err != nil {
		return "", fmt.Errorf("create temp token: %w", err)
	}
	return token.Token, nil
}

func (s *TemporaryAuthTokenService) Validate(ctx context.Context, publicToken string) (*models.User, error) {
	if publicToken == "" {
		return nil, fmt.Errorf("public token is required")
	}

	var token models.TemporaryAuthToken
	if err := s.db.WithContext(ctx).Where("token = ?", publicToken).First(&token).Error; err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	if time.Now().After(token.ExpiresAt) {
		_ = s.db.WithContext(ctx).Delete(&token)
		return nil, fmt.Errorf("token expired")
	}

	var u models.User
	if err := s.db.WithContext(ctx).First(&u, token.UserID).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	_ = s.db.WithContext(ctx).Delete(&token) // single-use: delete after validation
	return &u, nil
}

func (s *TemporaryAuthTokenService) InvalidateUserTokens(ctx context.Context, userID int) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.TemporaryAuthToken{}).Error
}
