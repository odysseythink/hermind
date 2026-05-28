package services

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type ScheduledJobContinueService struct {
	db    *gorm.DB
	sjSvc *ScheduledJobService
}

func NewScheduledJobContinueService(db *gorm.DB, sjSvc *ScheduledJobService) *ScheduledJobContinueService {
	return &ScheduledJobContinueService{db: db, sjSvc: sjSvc}
}

func (s *ScheduledJobContinueService) ContinueInThread(ctx context.Context, runID int) (*models.Workspace, *models.WorkspaceThread, error) {
	run, err := s.sjSvc.GetRun(ctx, runID)
	if err != nil {
		return nil, nil, err
	}

	var parsed struct {
		Text    string `json:"text"`
		Sources []any  `json:"sources,omitempty"`
		Outputs []any  `json:"outputs,omitempty"`
	}
	_ = json.Unmarshal([]byte(run.Result), &parsed)
	if parsed.Text == "" {
		parsed.Text = "No response was generated."
	}

	// Upsert workspace by slug "scheduled-jobs"
	var ws models.Workspace
	if err := s.db.WithContext(ctx).Where("slug = ?", "scheduled-jobs").First(&ws).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, nil, err
		}
		ws = models.Workspace{
			Name:          "Scheduled Jobs",
			Slug:          "scheduled-jobs",
			ChatMode:      strPtr("automatic"),
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}
		if err := s.db.WithContext(ctx).Create(&ws).Error; err != nil {
			return nil, nil, err
		}
	}

	thr := models.WorkspaceThread{
		WorkspaceID:   ws.ID,
		Slug:          uuid.NewString()[:8],
		Name:          "Run #" + strconv.Itoa(run.ID),
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&thr).Error; err != nil {
		return nil, nil, err
	}

	resp := map[string]any{
		"text": parsed.Text, "sources": parsed.Sources, "outputs": parsed.Outputs, "type": "chat",
	}
	respJSON, _ := json.Marshal(resp)
	chat := models.WorkspaceChat{
		WorkspaceID:   ws.ID,
		ThreadID:      &thr.ID,
		Prompt:        run.Job.Prompt,
		Response:      string(respJSON),
		Include:       true,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&chat).Error; err != nil {
		return nil, nil, err
	}
	return &ws, &thr, nil
}
