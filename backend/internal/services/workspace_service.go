package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type WorkspaceService struct {
	db            *gorm.DB
	cfg           *config.Config
	phSvc         *PromptHistoryService
	defaultPrompt string
}

func NewWorkspaceService(db *gorm.DB, cfg *config.Config, phSvc *PromptHistoryService) *WorkspaceService {
	return &WorkspaceService{
		db:            db,
		cfg:           cfg,
		phSvc:         phSvc,
		defaultPrompt: "Given the following conversation, relevant context, and a follow up question, reply with an answer to the current question the user is asking.",
	}
}

func (s *WorkspaceService) Create(ctx context.Context, userID int, req dto.CreateWorkspaceRequest) (*models.Workspace, error) {
	slug := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-")) + "-" + uuid.New().String()[:8]
	ws := models.Workspace{
		Name:          req.Name,
		Slug:          slug,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&ws).Error; err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	wu := models.WorkspaceUser{
		WorkspaceID:   ws.ID,
		UserID:        userID,
		Role:          "admin",
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&wu).Error; err != nil {
		return nil, fmt.Errorf("create workspace user: %w", err)
	}
	return &ws, nil
}

func (s *WorkspaceService) List(ctx context.Context, userID int) ([]models.Workspace, error) {
	// API context (userID == 0) returns ALL workspaces (Node parity).
	if userID == 0 {
		var workspaces []models.Workspace
		if err := s.db.Find(&workspaces).Error; err != nil {
			return nil, err
		}
		return workspaces, nil
	}
	var wus []models.WorkspaceUser
	if err := s.db.Where("user_id = ?", userID).Preload("Workspace").Find(&wus).Error; err != nil {
		return nil, err
	}
	workspaces := make([]models.Workspace, 0, len(wus))
	for _, wu := range wus {
		workspaces = append(workspaces, wu.Workspace)
	}
	return workspaces, nil
}

func (s *WorkspaceService) GetBySlug(ctx context.Context, slug string) (*models.Workspace, error) {
	var ws models.Workspace
	if err := s.db.Where("slug = ?", slug).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

func (s *WorkspaceService) Update(ctx context.Context, slug string, req dto.UpdateWorkspaceRequest, userID *int) error {
	var ws models.Workspace
	if err := s.db.Where("slug = ?", slug).First(&ws).Error; err != nil {
		return err
	}

	// Mirror anything-llm's 4-condition prompt-history hook (server/models/workspace.js:526–532):
	// fires when new prompt is non-empty AND prev was non-empty AND prev != defaultPrompt AND prev != new.
	if req.OpenAiPrompt != nil && *req.OpenAiPrompt != "" &&
		ws.OpenAiPrompt != nil && *ws.OpenAiPrompt != "" &&
		*ws.OpenAiPrompt != s.defaultPrompt &&
		*ws.OpenAiPrompt != *req.OpenAiPrompt {
		if s.phSvc != nil {
			// Non-fatal — log on failure but never block the update.
			_ = s.phSvc.Log(ctx, ws.ID, *ws.OpenAiPrompt, userID)
		}
	}

	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.OpenAiTemp != nil {
		updates["open_ai_temp"] = *req.OpenAiTemp
	}
	if req.OpenAiHistory > 0 {
		updates["open_ai_history"] = req.OpenAiHistory
	}
	if req.OpenAiPrompt != nil {
		updates["open_ai_prompt"] = *req.OpenAiPrompt
	}
	if req.SimilarityThreshold != nil {
		updates["similarity_threshold"] = *req.SimilarityThreshold
	}
	if req.ChatProvider != nil {
		updates["chat_provider"] = *req.ChatProvider
	}
	if req.ChatModel != nil {
		updates["chat_model"] = *req.ChatModel
	}
	if req.TopN != nil {
		updates["top_n"] = *req.TopN
	}
	if req.ChatMode != nil {
		updates["chat_mode"] = *req.ChatMode
	}
	if req.QueryRefusalResponse != nil {
		updates["query_refusal_response"] = *req.QueryRefusalResponse
	}
	if req.CompressEnabled != nil {
		switch *req.CompressEnabled {
		case "true":
			updates["compress_enabled"] = true
		case "false":
			updates["compress_enabled"] = false
		default:
			updates["compress_enabled"] = nil
		}
	}
	if req.CompressThreshold != nil {
		if *req.CompressThreshold == "" || *req.CompressThreshold == "default" {
			updates["compress_threshold"] = nil
		} else {
			if v, err := strconv.ParseFloat(*req.CompressThreshold, 64); err == nil {
				updates["compress_threshold"] = v
			}
		}
	}
	if req.CompressContextLen != nil {
		if *req.CompressContextLen == "" || *req.CompressContextLen == "default" {
			updates["compress_context_len"] = nil
		} else {
			if v, err := strconv.Atoi(*req.CompressContextLen); err == nil {
				updates["compress_context_len"] = v
			}
		}
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&ws).Updates(updates).Error
}

func (s *WorkspaceService) Delete(ctx context.Context, slug string) error {
	return s.db.Where("slug = ?", slug).Delete(&models.Workspace{}).Error
}

type WorkspaceUserInfo struct {
	UserID        int       `json:"userId"`
	Username      string    `json:"username"`
	Role          string    `json:"role"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

func (s *WorkspaceService) ListWorkspaceUsers(ctx context.Context, workspaceID int) ([]WorkspaceUserInfo, error) {
	var wus []models.WorkspaceUser
	if err := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&wus).Error; err != nil {
		return nil, err
	}
	if len(wus) == 0 {
		return []WorkspaceUserInfo{}, nil
	}
	userIDs := make([]int, 0, len(wus))
	for _, wu := range wus {
		userIDs = append(userIDs, wu.UserID)
	}
	var users []models.User
	if err := s.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	byID := map[int]models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	out := make([]WorkspaceUserInfo, 0, len(wus))
	for _, wu := range wus {
		u, ok := byID[wu.UserID]
		if !ok {
			continue
		}
		username := ""
		if u.Username != nil {
			username = *u.Username
		}
		out = append(out, WorkspaceUserInfo{
			UserID:        u.ID,
			Username:      username,
			Role:          u.Role,
			LastUpdatedAt: wu.LastUpdatedAt,
		})
	}
	return out, nil
}

func (s *WorkspaceService) UpdateUsers(ctx context.Context, workspaceID int, userIDs []int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.WorkspaceUser{}).Error; err != nil {
			return err
		}
		now := time.Now()
		rows := make([]models.WorkspaceUser, 0, len(userIDs))
		for _, uid := range userIDs {
			rows = append(rows, models.WorkspaceUser{
				WorkspaceID:   workspaceID,
				UserID:        uid,
				Role:          "default",
				CreatedAt:     now,
				LastUpdatedAt: now,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (s *WorkspaceService) DeleteByID(ctx context.Context, id int) (bool, error) {
	var ws models.Workspace
	if err := s.db.WithContext(ctx).First(&ws, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceChat{}).Error; err != nil {
			return err
		}
		// document_vectors has no workspace_id column — join via workspace_documents.doc_id
		var docIDs []string
		if err := tx.Model(&models.WorkspaceDocument{}).Where("workspace_id = ?", id).Pluck("doc_id", &docIDs).Error; err != nil {
			return err
		}
		if len(docIDs) > 0 {
			if err := tx.Where("doc_id IN ?", docIDs).Delete(&models.DocumentVector{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceDocument{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceUser{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Workspace{}, id).Error
	})
}

// Slug exposes the workspace slug for vector-namespace lookup.
func (s *WorkspaceService) GetByID(ctx context.Context, id int) (*models.Workspace, error) {
	var ws models.Workspace
	if err := s.db.WithContext(ctx).First(&ws, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ws, nil
}

// UpdatePin sets workspace_documents.pinned for the (workspaceID, docPath) row.
// Returns gorm.ErrRecordNotFound when no row matches.
func (s *WorkspaceService) UpdatePin(ctx context.Context, workspaceID int, docPath string, pinned bool) error {
	res := s.db.WithContext(ctx).
		Model(&models.WorkspaceDocument{}).
		Where("workspace_id = ? AND docpath = ?", workspaceID, docPath).
		Update("pinned", pinned)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *WorkspaceService) GetChats(ctx context.Context, workspaceID int) ([]map[string]any, error) {
	var chats []models.WorkspaceChat
	if err := s.db.Where("workspace_id = ? AND thread_id IS NULL", workspaceID).Order("id ASC").Find(&chats).Error; err != nil {
		return nil, err
	}
	return convertToChatHistory(chats), nil
}
