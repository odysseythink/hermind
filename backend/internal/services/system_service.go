package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"gorm.io/gorm"
)

type SystemService struct {
	db        *gorm.DB
	cache     *sync.Map
	observers []providers.SettingObserver
	obsMu     sync.RWMutex
}

func NewSystemService(db *gorm.DB) *SystemService {
	return &SystemService{db: db, cache: &sync.Map{}}
}

func (s *SystemService) RegisterObserver(o providers.SettingObserver) {
	s.obsMu.Lock()
	defer s.obsMu.Unlock()
	s.observers = append(s.observers, o)
}

func (s *SystemService) notifyObservers(ctx context.Context, key, value string) error {
	s.obsMu.RLock()
	defer s.obsMu.RUnlock()
	for _, o := range s.observers {
		if err := o.OnSettingChanged(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *SystemService) GetSetting(ctx context.Context, key string) (string, error) {
	if val, ok := s.cache.Load(key); ok {
		return val.(string), nil
	}
	var setting models.SystemSetting
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return "", fmt.Errorf("setting not found: %w", err)
	}
	if setting.Value == nil {
		return "", nil
	}
	s.cache.Store(key, *setting.Value)
	return *setting.Value, nil
}

func (s *SystemService) SetSetting(ctx context.Context, key, value string) error {
	var setting models.SystemSetting
	result := s.db.Where("key = ?", key).First(&setting)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			setting = models.SystemSetting{Key: key, Value: &value}
			if err := s.db.Create(&setting).Error; err != nil {
				return err
			}
		} else {
			return result.Error
		}
	} else {
		setting.Value = &value
		if err := s.db.Save(&setting).Error; err != nil {
			return err
		}
	}
	s.cache.Store(key, value)
	if err := s.notifyObservers(ctx, key, value); err != nil {
		return err
	}
	return nil
}

func (s *SystemService) IsSetupComplete(ctx context.Context) bool {
	val, err := s.GetSetting(ctx, "setup_complete")
	return err == nil && val == "true"
}

func (s *SystemService) GetAllSettings(ctx context.Context) (map[string]string, error) {
	var settings []models.SystemSetting
	if err := s.db.Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		if s.Value != nil {
			result[s.Key] = *s.Value
		} else {
			result[s.Key] = ""
		}
	}
	return result, nil
}

func (s *SystemService) MemoriesEnabled(ctx context.Context) bool {
	v, err := s.GetSetting(ctx, "memories_enabled")
	// Default: true. Explicit "false" disables.
	return err == nil && v != "false"
}

func (s *SystemService) GetOnboardingStatus(ctx context.Context) (bool, error) {
	val, err := s.GetSetting(ctx, "onboarding_complete")
	if err != nil {
		return false, nil
	}
	return val == "true", nil
}

func (s *SystemService) CompleteOnboarding(ctx context.Context) error {
	return s.SetSetting(ctx, "onboarding_complete", "true")
}
