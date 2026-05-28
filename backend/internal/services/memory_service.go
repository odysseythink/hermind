package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrMemoryLimitReached = errors.New("memory limit reached")
	ErrMemoryNotFound     = errors.New("memory not found")
)

type MemoryService struct {
	db *gorm.DB
}

func NewMemoryService(db *gorm.DB) *MemoryService {
	return &MemoryService{db: db}
}

type ExtractedAction struct {
	Action   string // "create" or "update"
	Scope    string // "WORKSPACE" or "GLOBAL" (uppercase from LLM)
	Content  string
	UpdateID *int // populated when Action == "update"
}

type ApplyResult struct {
	WS, Global, Updated int
}

func (s *MemoryService) countForScope(ctx context.Context, userID *int, workspaceID *int, scope string) (int64, error) {
	return countForScopeTx(s.db.WithContext(ctx), userID, workspaceID, scope)
}

func countForScopeTx(tx *gorm.DB, userID *int, workspaceID *int, scope string) (int64, error) {
	q := tx.Model(&models.Memory{}).Where("scope = ?", scope)
	q = applyUser(q, userID)
	if scope == models.MemoryScopeWorkspace {
		q = applyWorkspace(q, workspaceID)
	}
	var count int64
	return count, q.Count(&count).Error
}

func applyUser(q *gorm.DB, userID *int) *gorm.DB {
	if userID == nil {
		return q.Where("user_id IS NULL")
	}
	return q.Where("user_id = ?", *userID)
}

func applyWorkspace(q *gorm.DB, wsID *int) *gorm.DB {
	if wsID == nil {
		return q.Where("workspace_id IS NULL")
	}
	return q.Where("workspace_id = ?", *wsID)
}

func (s *MemoryService) Create(ctx context.Context, userID *int, workspaceID *int, scope, content string) (*models.Memory, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("content cannot be empty")
	}
	if scope != models.MemoryScopeWorkspace && scope != models.MemoryScopeGlobal {
		return nil, errors.New("invalid scope")
	}
	limit := models.WorkspaceMemoryLimit
	if scope == models.MemoryScopeGlobal {
		limit = models.GlobalMemoryLimit
	}
	count, err := s.countForScope(ctx, userID, workspaceID, scope)
	if err != nil {
		return nil, err
	}
	if count >= int64(limit) {
		return nil, ErrMemoryLimitReached
	}
	m := &models.Memory{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Scope:       scope,
		Content:     content,
	}
	if scope == models.MemoryScopeGlobal {
		m.WorkspaceID = nil
	}
	if err := s.db.WithContext(ctx).Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MemoryService) Update(ctx context.Context, id int, content string) (*models.Memory, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("content cannot be empty")
	}
	if err := s.db.WithContext(ctx).Model(&models.Memory{}).
		Where("id = ?", id).
		Updates(map[string]any{"content": content, "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *MemoryService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.Memory{}, id).Error
}

func (s *MemoryService) Get(ctx context.Context, id int) (*models.Memory, error) {
	var m models.Memory
	if err := s.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemoryNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (s *MemoryService) ListWorkspace(ctx context.Context, userID *int, workspaceID int) ([]models.Memory, error) {
	var rows []models.Memory
	q := s.db.WithContext(ctx).Where("workspace_id = ? AND scope = ?", workspaceID, models.MemoryScopeWorkspace).
		Order("created_at DESC")
	q = applyUser(q, userID)
	err := q.Find(&rows).Error
	return rows, err
}

func (s *MemoryService) ListGlobal(ctx context.Context, userID *int) ([]models.Memory, error) {
	var rows []models.Memory
	q := s.db.WithContext(ctx).Where("scope = ?", models.MemoryScopeGlobal).Order("created_at DESC")
	q = applyUser(q, userID)
	err := q.Find(&rows).Error
	return rows, err
}

func (s *MemoryService) PromoteToGlobal(ctx context.Context, id int) (*models.Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Scope == models.MemoryScopeGlobal {
		return m, nil
	}
	count, err := s.countForScope(ctx, m.UserID, nil, models.MemoryScopeGlobal)
	if err != nil {
		return nil, err
	}
	if count >= int64(models.GlobalMemoryLimit) {
		return nil, ErrMemoryLimitReached
	}
	if err := s.db.WithContext(ctx).Model(&models.Memory{}).Where("id = ?", id).
		Updates(map[string]any{
			"scope":        models.MemoryScopeGlobal,
			"workspace_id": nil,
			"updated_at":   time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *MemoryService) DemoteToWorkspace(ctx context.Context, id, workspaceID int) (*models.Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Scope == models.MemoryScopeWorkspace {
		return m, nil
	}
	count, err := s.countForScope(ctx, m.UserID, &workspaceID, models.MemoryScopeWorkspace)
	if err != nil {
		return nil, err
	}
	if count >= int64(models.WorkspaceMemoryLimit) {
		return nil, ErrMemoryLimitReached
	}
	if err := s.db.WithContext(ctx).Model(&models.Memory{}).Where("id = ?", id).
		Updates(map[string]any{
			"scope":        models.MemoryScopeWorkspace,
			"workspace_id": workspaceID,
			"updated_at":   time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *MemoryService) UpdateLastUsed(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&models.Memory{}).
		Where("id IN ?", ids).Update("last_used_at", time.Now()).Error
}

// ReplaceWorkspace transactionally deletes all workspace-scoped memories for
// (user, workspace) and inserts up to WorkspaceMemoryLimit new contents.
func (s *MemoryService) ReplaceWorkspace(ctx context.Context, userID *int, workspaceID int, contents []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("workspace_id = ? AND scope = ?", workspaceID, models.MemoryScopeWorkspace)
		q = applyUser(q, userID)
		if err := q.Delete(&models.Memory{}).Error; err != nil {
			return err
		}
		for i, c := range contents {
			if i >= models.WorkspaceMemoryLimit {
				break
			}
			if err := tx.Create(&models.Memory{
				UserID: userID, WorkspaceID: &workspaceID,
				Scope: models.MemoryScopeWorkspace, Content: c,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyExtracted atomically applies a batch of Observer/Reflector decisions.
// It re-counts inside the transaction to guard against concurrent changes.
func (s *MemoryService) ApplyExtracted(ctx context.Context, userID *int, workspaceID int, actions []ExtractedAction, globalSlots int) (ApplyResult, error) {
	var result ApplyResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Re-count inside transaction to guard against concurrent changes.
		wsCount, err := countForScopeTx(tx, userID, &workspaceID, models.MemoryScopeWorkspace)
		if err != nil {
			return err
		}
		glCount, err := countForScopeTx(tx, userID, nil, models.MemoryScopeGlobal)
		if err != nil {
			return err
		}
		actualGlobalSlots := globalSlots
		if actualGlobalSlots > models.GlobalMemoryLimit-int(glCount) {
			actualGlobalSlots = models.GlobalMemoryLimit - int(glCount)
		}
		if actualGlobalSlots < 0 {
			actualGlobalSlots = 0
		}
		actualWSSlots := models.WorkspaceMemoryLimit - int(wsCount)
		if actualWSSlots < 0 {
			actualWSSlots = 0
		}

		wsCreates, glCreates, updates := splitActions(actions, actualGlobalSlots)
		if len(wsCreates) > actualWSSlots {
			wsCreates = wsCreates[:actualWSSlots]
		}

		for _, a := range wsCreates {
			if err := tx.Create(&models.Memory{
				UserID: userID, WorkspaceID: &workspaceID,
				Scope: models.MemoryScopeWorkspace, Content: a.Content,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range glCreates {
			if err := tx.Create(&models.Memory{
				UserID: userID, Scope: models.MemoryScopeGlobal, Content: a.Content,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range updates {
			if err := tx.Model(&models.Memory{}).Where("id = ?", *a.UpdateID).
				Updates(map[string]any{"content": a.Content, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		result.WS = len(wsCreates)
		result.Global = len(glCreates)
		result.Updated = len(updates)
		return nil
	})
	if err != nil {
		result = ApplyResult{} // zero on rollback
	}
	return result, err
}

func splitActions(actions []ExtractedAction, globalSlots int) (ws, gl, upd []ExtractedAction) {
	for _, a := range actions {
		switch {
		case a.Action == "update" && a.UpdateID != nil:
			upd = append(upd, a)
		case a.Action == "create" && a.Scope == "WORKSPACE":
			if len(ws) < models.WorkspaceMemoryLimit {
				ws = append(ws, a)
			}
		case a.Action == "create" && a.Scope == "GLOBAL":
			if len(gl) < globalSlots {
				gl = append(gl, a)
			}
		}
	}
	return
}
