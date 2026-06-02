package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

type ThreadService struct {
	db *gorm.DB
}

func NewThreadService(db *gorm.DB) *ThreadService {
	return &ThreadService{db: db}
}

func init() {
	slug.CustomSub = map[string]string{
		"+": "plus",
		"!": "bang",
		"@": "at",
		"*": "splat",
		".": "dot",
	}
}

func (s *ThreadService) Create(ctx context.Context, workspaceID int, userID *int, req dto.CreateThreadRequest) (*models.WorkspaceThread, error) {
	name := req.Name
	if name == "" {
		name = "Thread"
	}
	threadSlug := req.Slug
	if threadSlug == "" {
		threadSlug = uuid.New().String()
	} else {
		threadSlug = slug.Make(threadSlug)
		if threadSlug == "" {
			threadSlug = uuid.New().String()
		}
	}

	thread := models.WorkspaceThread{
		Name:           name,
		Slug:           threadSlug,
		WorkspaceID:    workspaceID,
		UserID:         userID,
		ParentThreadID: req.ParentThreadID,
		CreatedAt:      time.Now(),
		LastUpdatedAt:  time.Now(),
	}
	if err := s.db.Create(&thread).Error; err != nil {
		return nil, fmt.Errorf("create thread: %w", err)
	}

	// Seed child thread with parent's latest compaction summary
	if req.ParentThreadID != nil {
		if err := s.seedCompactionFromParent(ctx, workspaceID, req.ParentThreadID, &thread.ID); err != nil {
			mlog.Warning("thread handoff seeding failed: ", err)
		}
	}

	return &thread, nil
}

func (s *ThreadService) seedCompactionFromParent(ctx context.Context, workspaceID int, parentThreadID, childThreadID *int) error {
	if parentThreadID == nil || childThreadID == nil {
		return nil
	}
	var parentComp models.ThreadCompaction
	err := s.db.Where("workspace_id = ? AND thread_id = ?", workspaceID, *parentThreadID).
		Order("created_at DESC").
		First(&parentComp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // no parent compaction to seed
		}
		return err
	}
	seed := models.ThreadCompaction{
		WorkspaceID:   workspaceID,
		ThreadID:      childThreadID,
		Summary:       parentComp.Summary,
		UpToChatID:    0,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	return s.db.Create(&seed).Error
}

func (s *ThreadService) List(ctx context.Context, workspaceID int) ([]models.WorkspaceThread, error) {
	var threads []models.WorkspaceThread
	if err := s.db.Where("workspace_id = ?", workspaceID).Order("id DESC").Find(&threads).Error; err != nil {
		return nil, err
	}
	return threads, nil
}

func (s *ThreadService) GetBySlug(ctx context.Context, workspaceID int, threadSlug string) (*models.WorkspaceThread, error) {
	var thread models.WorkspaceThread
	if err := s.db.Where("slug = ? AND workspace_id = ?", threadSlug, workspaceID).First(&thread).Error; err != nil {
		return nil, err
	}
	return &thread, nil
}

func (s *ThreadService) Update(ctx context.Context, thread *models.WorkspaceThread, req dto.UpdateThreadRequest) error {
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if len(updates) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(thread).Updates(updates).Error
}

func (s *ThreadService) Delete(ctx context.Context, workspaceID int, threadSlug string) error {
	return s.db.Where("slug = ? AND workspace_id = ?", threadSlug, workspaceID).Delete(&models.WorkspaceThread{}).Error
}

func (s *ThreadService) BulkDelete(ctx context.Context, workspaceID int, slugs []string) error {
	if len(slugs) == 0 {
		return nil
	}
	return s.db.Where("workspace_id = ? AND slug IN ?", workspaceID, slugs).Delete(&models.WorkspaceThread{}).Error
}

func (s *ThreadService) GetThreadChats(ctx context.Context, threadID int) ([]map[string]any, error) {
	var chats []models.WorkspaceChat
	if err := s.db.Where("thread_id = ? AND include = true", threadID).Order("id ASC").Find(&chats).Error; err != nil {
		return nil, err
	}
	return convertToChatHistory(chats), nil
}

func convertToChatHistory(chats []models.WorkspaceChat) []map[string]any {
	result := make([]map[string]any, 0)
	for _, c := range chats {
		var data map[string]any
		if err := json.Unmarshal([]byte(c.Response), &data); err != nil {
			data = map[string]any{"text": c.Response, "type": "chart", "sources": []any{}}
		}
		text, _ := data["text"].(string)
		sources, _ := data["sources"].([]any)
		if sources == nil {
			sources = []any{}
		}
		msgType, _ := data["type"].(string)
		if msgType == "" {
			msgType = "chart"
		}
		attachments, _ := data["attachments"].([]any)
		if attachments == nil {
			attachments = []any{}
		}
		metrics, _ := data["metrics"].(map[string]any)
		if metrics == nil {
			metrics = map[string]any{}
		}
		var outputs []any
		if out, ok := data["outputs"].([]any); ok && len(out) > 0 {
			outputs = out
		}
		sentAt := c.CreatedAt.Unix()

		userMsg := map[string]any{
			"role":        "user",
			"content":     c.Prompt,
			"sentAt":      sentAt,
			"attachments": attachments,
			"chatId":      c.ID,
		}
		assistantMsg := map[string]any{
			"type":          msgType,
			"role":          "assistant",
			"content":       text,
			"sources":       sources,
			"chatId":        c.ID,
			"sentAt":        sentAt,
			"feedbackScore": c.FeedbackScore,
			"metrics":       metrics,
		}
		if len(outputs) > 0 {
			assistantMsg["outputs"] = outputs
		}
		result = append(result, userMsg, assistantMsg)
	}
	return result
}

func (s *ThreadService) DeleteThreadEditedChats(ctx context.Context, threadID int) error {
	return s.db.Where("thread_id = ? AND prompt != response", threadID).Delete(&models.WorkspaceChat{}).Error
}

func (s *ThreadService) UpdateThreadChat(ctx context.Context, chatID int, req dto.UpdateChatRequest) error {
	updates := map[string]any{}
	if req.Response != "" {
		updates["response"] = req.Response
	}
	if req.Include != nil {
		updates["include"] = *req.Include
	}
	if len(updates) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&models.WorkspaceChat{}).Where("id = ?", chatID).Updates(updates).Error
}
