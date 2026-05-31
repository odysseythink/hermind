package services

import (
	"context"
	"strings"
	"time"

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

func (s *BrowserExtensionService) CreateKey(ctx context.Context, userID *int) (*models.BrowserExtensionApiKey, error) {
	key := &models.BrowserExtensionApiKey{
		Key:           "brx-" + uuid.NewString(),
		UserID:        userID,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(key).Error; err != nil {
		return nil, err
	}
	return key, nil
}

func (s *BrowserExtensionService) ListKeys(ctx context.Context, userID *int, isAdmin bool) ([]models.BrowserExtensionApiKey, error) {
	var keys []models.BrowserExtensionApiKey
	q := s.db.WithContext(ctx).Order("id desc")
	if !isAdmin {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *BrowserExtensionService) DeleteKey(ctx context.Context, id int, userID *int, isAdmin bool) error {
	if !isAdmin {
		var count int64
		if err := s.db.WithContext(ctx).
			Model(&models.BrowserExtensionApiKey{}).
			Where("id = ? AND user_id = ?", id, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	q := s.db.WithContext(ctx).Delete(&models.BrowserExtensionApiKey{}, id)
	if err := q.Error; err != nil {
		return err
	}
	if q.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *BrowserExtensionService) Validate(ctx context.Context, key string) (*models.BrowserExtensionApiKey, error) {
	if !strings.HasPrefix(key, "brx-") {
		return nil, gorm.ErrRecordNotFound
	}
	var record models.BrowserExtensionApiKey
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *BrowserExtensionService) DeleteAllForUser(ctx context.Context, userID int) error {
	return s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.BrowserExtensionApiKey{}).Error
}
