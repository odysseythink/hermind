package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type PromptHistoryService struct {
	db *gorm.DB
}

func NewPromptHistoryService(db *gorm.DB) *PromptHistoryService {
	return &PromptHistoryService{db: db}
}

// Log persists a prior prompt for a workspace. Failure is returned but should
// typically be treated as non-fatal by callers (the workspace update itself
// must not be blocked).
func (s *PromptHistoryService) Log(ctx context.Context, workspaceID int, prevPrompt string, userID *int) error {
	row := models.PromptHistory{
		WorkspaceID: workspaceID,
		Prompt:      prevPrompt,
		ModifiedBy:  userID,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// ListByWorkspace returns the most recent N rows for a workspace, newest first.
// limit <= 0 returns at most 50 rows (matching anything-llm's implicit cap).
func (s *PromptHistoryService) ListByWorkspace(ctx context.Context, workspaceID int, limit int) ([]models.PromptHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []models.PromptHistory
	err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("modified_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (s *PromptHistoryService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.PromptHistory{}, id).Error
}

func (s *PromptHistoryService) DeleteAll(ctx context.Context, workspaceID int) error {
	return s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Delete(&models.PromptHistory{}).Error
}
