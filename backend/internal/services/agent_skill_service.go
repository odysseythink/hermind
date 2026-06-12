package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

var (
	ErrSkillNotFound        = errors.New("skill not found")
	ErrSkillNameExists      = errors.New("skill name already exists in this workspace")
	ErrInvalidSkillName     = errors.New("invalid skill name")
	ErrInvalidCategory      = errors.New("invalid category")
	ErrInvalidFrontmatter   = errors.New("invalid SKILL.md frontmatter")
	ErrSkillContentTooLarge = errors.New("skill content exceeds maximum size")
	ErrInvalidFilePath      = errors.New("invalid file path")
	ErrPatchNoMatch         = errors.New("patch old_string not found")
	ErrPatchAmbiguous       = errors.New("patch old_string matches multiple locations")
)

const (
	MaxSkillNameLength        = 64
	MaxSkillDescriptionLength = 1024
	MaxSkillContentChars      = 100_000
	MaxSkillFrontmatterChars  = 10_000
	MaxSkillFileBytes         = 1_048_576 // 1 MiB
)

var (
	validNameRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	allowedSubdirs = map[string]bool{
		"references": true,
		"templates":  true,
		"scripts":    true,
		"assets":     true,
	}
)

func init() {
	slug.CustomSub = map[string]string{
		"+": "plus",
		"!": "bang",
		"@": "at",
		"*": "splat",
		".": "dot",
	}
}

// AgentSkillManager is the interface exposed to agent tools and handlers.
type AgentSkillManager interface {
	Create(ctx context.Context, workspaceID int, req dto.CreateAgentSkillRequest) (*models.AgentSkill, error)
	GetBySlug(ctx context.Context, workspaceID int, slug string) (*models.AgentSkill, error)
	GetByID(ctx context.Context, id int) (*models.AgentSkill, error)
	List(ctx context.Context, workspaceID int, includeArchived bool) ([]models.AgentSkill, error)
	ListActiveByWorkspace(ctx context.Context, workspaceID int) ([]models.AgentSkill, error)
	Update(ctx context.Context, workspaceID int, skillSlug string, req dto.UpdateAgentSkillRequest) (*models.AgentSkill, error)
	Patch(ctx context.Context, workspaceID int, skillSlug string, req dto.PatchAgentSkillRequest) (*models.AgentSkill, error)
	PatchFile(ctx context.Context, workspaceID int, skillSlug string, req dto.PatchSkillFileRequest) (*models.AgentSkillFile, error)
	Delete(ctx context.Context, workspaceID int, skillSlug string) error
	WriteFile(ctx context.Context, workspaceID int, skillSlug string, req dto.WriteSkillFileRequest) error
	RemoveFile(ctx context.Context, workspaceID int, skillSlug string, filePath string) error
	GetFile(ctx context.Context, skillID int, filePath string) (*models.AgentSkillFile, error)
	ListFiles(ctx context.Context, skillID int) ([]models.AgentSkillFile, error)
	BumpUse(ctx context.Context, workspaceID int, skillSlug string) error
	BumpView(ctx context.Context, workspaceID int, skillSlug string) error
	BumpPatch(ctx context.Context, workspaceID int, skillSlug string) error
	ApplyCuratorTransitions(ctx context.Context, staleDays, archiveDays int) (map[string]int, error)
}

// Ensure AgentSkillService implements AgentSkillManager.
var _ AgentSkillManager = (*AgentSkillService)(nil)

type AgentSkillService struct {
	db *gorm.DB
}

func NewAgentSkillService(db *gorm.DB) *AgentSkillService {
	return &AgentSkillService{db: db}
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func validateSkillName(name string) error {
	if name == "" {
		return ErrInvalidSkillName
	}
	if len(name) > MaxSkillNameLength {
		return fmt.Errorf("%w: exceeds %d characters", ErrInvalidSkillName, MaxSkillNameLength)
	}
	if !validNameRE.MatchString(name) {
		return fmt.Errorf("%w: must be lowercase alphanumeric with hyphens, dots, underscores", ErrInvalidSkillName)
	}
	return nil
}

func validateCategory(category string) error {
	if category == "" {
		return nil
	}
	if strings.Contains(category, "/") || strings.Contains(category, "\\") {
		return fmt.Errorf("%w: cannot contain path separators", ErrInvalidCategory)
	}
	if len(category) > MaxSkillNameLength {
		return fmt.Errorf("%w: exceeds %d characters", ErrInvalidCategory, MaxSkillNameLength)
	}
	if !validNameRE.MatchString(category) {
		return fmt.Errorf("%w: must be lowercase alphanumeric with hyphens, dots, underscores", ErrInvalidCategory)
	}
	return nil
}

func validateFrontmatter(frontmatter string) (map[string]any, error) {
	if frontmatter == "" {
		return nil, fmt.Errorf("%w: frontmatter is required", ErrInvalidFrontmatter)
	}
	if len(frontmatter) > MaxSkillFrontmatterChars {
		return nil, fmt.Errorf("%w: exceeds %d characters", ErrInvalidFrontmatter, MaxSkillFrontmatterChars)
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFrontmatter, err)
	}
	name, ok := fm["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: 'name' field is required", ErrInvalidFrontmatter)
	}
	desc, ok := fm["description"].(string)
	if !ok || strings.TrimSpace(desc) == "" {
		return nil, fmt.Errorf("%w: 'description' field is required", ErrInvalidFrontmatter)
	}
	if len(desc) > MaxSkillDescriptionLength {
		return nil, fmt.Errorf("%w: description exceeds %d characters", ErrInvalidFrontmatter, MaxSkillDescriptionLength)
	}
	return fm, nil
}

func validateContentSize(content string) error {
	if len(content) > MaxSkillContentChars {
		return fmt.Errorf("%w: %d characters (limit: %d)", ErrSkillContentTooLarge, len(content), MaxSkillContentChars)
	}
	return nil
}

func validateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("%w: file_path is required", ErrInvalidFilePath)
	}
	parts := strings.Split(filePath, "/")
	if len(parts) < 2 {
		return fmt.Errorf("%w: must be under references/, templates/, scripts/, or assets/", ErrInvalidFilePath)
	}
	if !allowedSubdirs[parts[0]] {
		return fmt.Errorf("%w: must be under references/, templates/, scripts/, or assets/", ErrInvalidFilePath)
	}
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("%w: path traversal not allowed", ErrInvalidFilePath)
	}
	return nil
}

func makeSkillSlug(name string) string {
	s := slug.Make(name)
	if s == "" {
		s = slug.Make("skill")
	}
	return s
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

func (s *AgentSkillService) Create(ctx context.Context, workspaceID int, req dto.CreateAgentSkillRequest) (*models.AgentSkill, error) {
	if err := validateSkillName(req.Name); err != nil {
		return nil, err
	}
	if err := validateCategory(req.Category); err != nil {
		return nil, err
	}
	if err := validateContentSize(req.Content); err != nil {
		return nil, err
	}

	// Parse frontmatter if provided, or build minimal one
	var frontmatter string
	var description string
	if req.Frontmatter != "" {
		fm, err := validateFrontmatter(req.Frontmatter)
		if err != nil {
			return nil, err
		}
		frontmatter = req.Frontmatter
		description = getString(fm, "description")
	} else {
		// Build minimal frontmatter
		frontmatter = fmt.Sprintf("name: %s\ndescription: %s\n", req.Name, req.Description)
		description = req.Description
	}

	skillSlug := makeSkillSlug(req.Name)

	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = models.AgentSkillCreatedByAgent
	}
	writeOrigin := req.WriteOrigin
	if writeOrigin == "" {
		writeOrigin = "foreground"
	}

	skill := models.AgentSkill{
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Slug:        skillSlug,
		Description: description,
		Category:    req.Category,
		Content:     req.Content,
		Frontmatter: frontmatter,
		Status:      models.AgentSkillStatusActive,
		CreatedBy:   createdBy,
		WriteOrigin: writeOrigin,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&skill).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSkillNameExists
		}
		return nil, fmt.Errorf("create skill: %w", err)
	}
	return &skill, nil
}

func (s *AgentSkillService) GetBySlug(ctx context.Context, workspaceID int, skillSlug string) (*models.AgentSkill, error) {
	var skill models.AgentSkill
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND slug = ?", workspaceID, skillSlug).
		First(&skill).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return &skill, nil
}

func (s *AgentSkillService) GetByID(ctx context.Context, id int) (*models.AgentSkill, error) {
	var skill models.AgentSkill
	if err := s.db.WithContext(ctx).First(&skill, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return &skill, nil
}

func (s *AgentSkillService) List(ctx context.Context, workspaceID int, includeArchived bool) ([]models.AgentSkill, error) {
	var skills []models.AgentSkill
	q := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if !includeArchived {
		q = q.Where("status != ?", models.AgentSkillStatusArchived)
	}
	if err := q.Order("category ASC, name ASC").Find(&skills).Error; err != nil {
		return nil, err
	}
	return skills, nil
}

func (s *AgentSkillService) ListActiveByWorkspace(ctx context.Context, workspaceID int) ([]models.AgentSkill, error) {
	var skills []models.AgentSkill
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ?", workspaceID, models.AgentSkillStatusActive).
		Order("category ASC, name ASC").
		Find(&skills).Error; err != nil {
		return nil, err
	}
	return skills, nil
}

func (s *AgentSkillService) Update(ctx context.Context, workspaceID int, skillSlug string, req dto.UpdateAgentSkillRequest) (*models.AgentSkill, error) {
	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	finalSlug := skillSlug
	if req.Name != "" {
		if err := validateSkillName(req.Name); err != nil {
			return nil, err
		}
		updates["name"] = req.Name
		newSlug := makeSkillSlug(req.Name)
		if newSlug != skill.Slug {
			var existing int64
			if err := s.db.WithContext(ctx).Model(&models.AgentSkill{}).
				Where("workspace_id = ? AND slug = ? AND id != ?", workspaceID, newSlug, skill.ID).
				Count(&existing).Error; err != nil {
				return nil, err
			}
			if existing > 0 {
				return nil, ErrSkillNameExists
			}
			updates["slug"] = newSlug
			finalSlug = newSlug
		}
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Category != "" {
		if err := validateCategory(req.Category); err != nil {
			return nil, err
		}
		updates["category"] = req.Category
	}
	if req.Content != "" {
		if err := validateContentSize(req.Content); err != nil {
			return nil, err
		}
		updates["content"] = req.Content
	}
	if req.Frontmatter != "" {
		if _, err := validateFrontmatter(req.Frontmatter); err != nil {
			return nil, err
		}
		updates["frontmatter"] = req.Frontmatter
	}
	if req.Status != "" {
		if req.Status != models.AgentSkillStatusActive &&
			req.Status != models.AgentSkillStatusStale &&
			req.Status != models.AgentSkillStatusArchived {
			return nil, errors.New("invalid status")
		}
		updates["status"] = req.Status
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}

	if len(updates) == 0 {
		return skill, nil
	}
	updates["updated_at"] = time.Now()

	if err := s.db.WithContext(ctx).Model(skill).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetBySlug(ctx, workspaceID, finalSlug)
}

func (s *AgentSkillService) Patch(ctx context.Context, workspaceID int, skillSlug string, req dto.PatchAgentSkillRequest) (*models.AgentSkill, error) {
	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return nil, err
	}

	if req.OldString == "" {
		return nil, errors.New("old_string is required")
	}

	content := skill.Content
	newContent, err := patchContent(content, req.OldString, req.NewString, req.ReplaceAll)
	if err != nil {
		return nil, err
	}

	if err := validateContentSize(newContent); err != nil {
		return nil, err
	}

	now := time.Now()
	updates := map[string]any{
		"content":         newContent,
		"updated_at":      now,
		"patch_count":     gorm.Expr("patch_count + 1"),
		"last_patched_at": now,
	}
	if err := s.db.WithContext(ctx).Model(skill).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetBySlug(ctx, workspaceID, skillSlug)
}

func (s *AgentSkillService) PatchFile(ctx context.Context, workspaceID int, skillSlug string, req dto.PatchSkillFileRequest) (*models.AgentSkillFile, error) {
	if err := validateFilePath(req.FilePath); err != nil {
		return nil, err
	}
	if req.OldString == "" {
		return nil, errors.New("old_string is required")
	}

	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return nil, err
	}

	file, err := s.GetFile(ctx, skill.ID, req.FilePath)
	if err != nil {
		return nil, err
	}

	newContent, err := patchContent(file.Content, req.OldString, req.NewString, req.ReplaceAll)
	if err != nil {
		return nil, err
	}
	if len(newContent) > MaxSkillFileBytes {
		return nil, fmt.Errorf("%w: %d bytes (limit: %d)", ErrSkillContentTooLarge, len(newContent), MaxSkillFileBytes)
	}

	now := time.Now()
	file.Content = newContent
	file.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(file).Error; err != nil {
		return nil, err
	}

	// Bump skill patch count since this is a content mutation
	_ = s.db.WithContext(ctx).Model(skill).Updates(map[string]any{
		"updated_at":      now,
		"patch_count":     gorm.Expr("patch_count + 1"),
		"last_patched_at": now,
	}).Error

	return file, nil
}

func patchContent(content, oldStr, newStr string, replaceAll bool) (string, error) {
	if replaceAll {
		return strings.ReplaceAll(content, oldStr, newStr), nil
	}
	count := strings.Count(content, oldStr)
	if count == 0 {
		return "", ErrPatchNoMatch
	}
	if count > 1 {
		return "", ErrPatchAmbiguous
	}
	return strings.Replace(content, oldStr, newStr, 1), nil
}

func (s *AgentSkillService) Delete(ctx context.Context, workspaceID int, skillSlug string) error {
	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return err
	}
	if skill.Pinned {
		return errors.New("cannot delete a pinned skill; unpin it first")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("skill_id = ?", skill.ID).Delete(&models.AgentSkillFile{}).Error; err != nil {
			return err
		}
		return tx.Delete(skill).Error
	})
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

func (s *AgentSkillService) WriteFile(ctx context.Context, workspaceID int, skillSlug string, req dto.WriteSkillFileRequest) error {
	if err := validateFilePath(req.FilePath); err != nil {
		return err
	}
	if len(req.Content) > MaxSkillFileBytes {
		return fmt.Errorf("%w: %d bytes (limit: %d)", ErrSkillContentTooLarge, len(req.Content), MaxSkillFileBytes)
	}

	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.AgentSkillFile
		err := tx.Where("skill_id = ? AND file_path = ?", skill.ID, req.FilePath).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			file := models.AgentSkillFile{
				SkillID:   skill.ID,
				FilePath:  req.FilePath,
				Content:   req.Content,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
		} else {
			existing.Content = req.Content
			existing.UpdatedAt = now
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
		}

		// Bump patch count since this is a content mutation
		return tx.Model(skill).Updates(map[string]any{
			"updated_at":      now,
			"patch_count":     gorm.Expr("patch_count + 1"),
			"last_patched_at": now,
		}).Error
	})
}

func (s *AgentSkillService) RemoveFile(ctx context.Context, workspaceID int, skillSlug string, filePath string) error {
	if err := validateFilePath(filePath); err != nil {
		return err
	}

	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Where("skill_id = ? AND file_path = ?", skill.ID, filePath).
		Delete(&models.AgentSkillFile{}).Error; err != nil {
		return err
	}

	return s.db.WithContext(ctx).Model(skill).Update("updated_at", time.Now()).Error
}

func (s *AgentSkillService) GetFile(ctx context.Context, skillID int, filePath string) (*models.AgentSkillFile, error) {
	var file models.AgentSkillFile
	if err := s.db.WithContext(ctx).Where("skill_id = ? AND file_path = ?", skillID, filePath).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return &file, nil
}

func (s *AgentSkillService) ListFiles(ctx context.Context, skillID int) ([]models.AgentSkillFile, error) {
	var files []models.AgentSkillFile
	if err := s.db.WithContext(ctx).Where("skill_id = ?", skillID).Order("file_path ASC").Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

// ---------------------------------------------------------------------------
// Telemetry
// ---------------------------------------------------------------------------

func (s *AgentSkillService) BumpUse(ctx context.Context, workspaceID int, skillSlug string) error {
	return s.db.WithContext(ctx).Model(&models.AgentSkill{}).
		Where("workspace_id = ? AND slug = ?", workspaceID, skillSlug).
		Updates(map[string]any{
			"use_count":    gorm.Expr("use_count + 1"),
			"last_used_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error
}

func (s *AgentSkillService) BumpView(ctx context.Context, workspaceID int, skillSlug string) error {
	return s.db.WithContext(ctx).Model(&models.AgentSkill{}).
		Where("workspace_id = ? AND slug = ?", workspaceID, skillSlug).
		Updates(map[string]any{
			"view_count":     gorm.Expr("view_count + 1"),
			"last_viewed_at": time.Now(),
			"updated_at":     time.Now(),
		}).Error
}

func (s *AgentSkillService) BumpPatch(ctx context.Context, workspaceID int, skillSlug string) error {
	return s.db.WithContext(ctx).Model(&models.AgentSkill{}).
		Where("workspace_id = ? AND slug = ?", workspaceID, skillSlug).
		Updates(map[string]any{
			"patch_count":     gorm.Expr("patch_count + 1"),
			"last_patched_at": time.Now(),
			"updated_at":      time.Now(),
		}).Error
}

// ---------------------------------------------------------------------------
// Curator transitions
// ---------------------------------------------------------------------------

func (s *AgentSkillService) ApplyCuratorTransitions(ctx context.Context, staleDays, archiveDays int) (map[string]int, error) {
	counts := map[string]int{"marked_stale": 0, "archived": 0, "reactivated": 0, "checked": 0}

	staleCutoff := time.Now().AddDate(0, 0, -staleDays)
	archiveCutoff := time.Now().AddDate(0, 0, -archiveDays)

	batchSize := 500
	offset := 0

	for {
		var batch []models.AgentSkill
		if err := s.db.WithContext(ctx).Where("status IN ?", []string{models.AgentSkillStatusActive, models.AgentSkillStatusStale}).
			Limit(batchSize).Offset(offset).Find(&batch).Error; err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		offset += len(batch)

		for _, skill := range batch {
			counts["checked"]++
			if skill.Pinned {
				continue
			}

			// Activity anchor: last_used_at > last_viewed_at > last_patched_at > created_at
			anchor := skill.CreatedAt
			if skill.LastUsedAt != nil && skill.LastUsedAt.After(anchor) {
				anchor = *skill.LastUsedAt
			}
			if skill.LastViewedAt != nil && skill.LastViewedAt.After(anchor) {
				anchor = *skill.LastViewedAt
			}
			if skill.LastPatchedAt != nil && skill.LastPatchedAt.After(anchor) {
				anchor = *skill.LastPatchedAt
			}

			switch skill.Status {
			case models.AgentSkillStatusActive:
				if anchor.Before(archiveCutoff) {
					if err := s.db.WithContext(ctx).Model(&skill).Updates(map[string]any{
						"status":     models.AgentSkillStatusArchived,
						"updated_at": time.Now(),
					}).Error; err != nil {
						return counts, err
					}
					counts["archived"]++
				} else if anchor.Before(staleCutoff) {
					if err := s.db.WithContext(ctx).Model(&skill).Updates(map[string]any{
						"status":     models.AgentSkillStatusStale,
						"updated_at": time.Now(),
					}).Error; err != nil {
						return counts, err
					}
					counts["marked_stale"]++
				}
			case models.AgentSkillStatusStale:
				if anchor.Before(archiveCutoff) {
					if err := s.db.WithContext(ctx).Model(&skill).Updates(map[string]any{
						"status":     models.AgentSkillStatusArchived,
						"updated_at": time.Now(),
					}).Error; err != nil {
						return counts, err
					}
					counts["archived"]++
				} else if anchor.After(staleCutoff) {
					if err := s.db.WithContext(ctx).Model(&skill).Updates(map[string]any{
						"status":     models.AgentSkillStatusActive,
						"updated_at": time.Now(),
					}).Error; err != nil {
						return counts, err
					}
					counts["reactivated"]++
				}
			}
		}
	}

	return counts, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func getString(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "constraint failed")
}
