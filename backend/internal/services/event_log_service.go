package services

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type EventLogService struct {
	db *gorm.DB

	subInit     sync.Once
	subMu       sync.RWMutex
	subscribers map[string][]eventHandler
}

func NewEventLogService(db *gorm.DB) *EventLogService {
	return &EventLogService{db: db}
}

func (s *EventLogService) LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error {
	var metaStr *string
	if len(metadata) > 0 {
		b, _ := json.Marshal(metadata)
		str := string(b)
		metaStr = &str
	}
	now := time.Now()
	log := models.EventLog{
		Event:      event,
		Metadata:   metaStr,
		UserID:     userID,
		OccurredAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		return err
	}
	s.notifySubscribers(event, metaStr, userID, now)
	return nil
}
