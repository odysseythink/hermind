package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

// ValidSystemCommands are commands that cannot be used by user-defined presets.
var ValidSystemCommands = map[string]bool{
	"/reset": true,
}

// FormatCommand normalizes a command string: lowercases, ensures / prefix,
// and replaces invalid characters with '-'.
func FormatCommand(command string) string {
	if len(command) < 2 {
		return "/" + strings.Split(uuid.New().String(), "-")[0]
	}
	adjusted := strings.ToLower(command)
	if !strings.HasPrefix(adjusted, "/") {
		adjusted = "/" + adjusted
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return "/" + re.ReplaceAllString(adjusted[1:], "-")
}

// IsSystemCommand checks if a command conflicts with a built-in system command.
func IsSystemCommand(command string) bool {
	return ValidSystemCommands[command]
}

type PromptPresetService struct {
	db *gorm.DB
}

func NewPromptPresetService(db *gorm.DB) *PromptPresetService {
	return &PromptPresetService{db: db}
}

func (s *PromptPresetService) List(ctx context.Context) ([]models.PromptPreset, error) {
	var presets []models.PromptPreset
	err := s.db.WithContext(ctx).Order("id desc").Find(&presets).Error
	return presets, err
}

func (s *PromptPresetService) ListByUser(ctx context.Context, userID int) ([]models.PromptPreset, error) {
	var presets []models.PromptPreset
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at asc").
		Find(&presets).Error
	return presets, err
}

func (s *PromptPresetService) GetByID(ctx context.Context, id int) (*models.PromptPreset, error) {
	var preset models.PromptPreset
	if err := s.db.WithContext(ctx).First(&preset, id).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

func (s *PromptPresetService) GetByUserAndCommand(ctx context.Context, userID *int, command string) (*models.PromptPreset, error) {
	var preset models.PromptPreset
	q := s.db.WithContext(ctx).Where("command = ?", command)
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	} else {
		q = q.Where("user_id IS NULL")
	}
	if err := q.First(&preset).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

func (s *PromptPresetService) Create(ctx context.Context, userID *int, command, prompt, description string) (*models.PromptPreset, error) {
	formatted := FormatCommand(command)

	// Check for conflict with system commands.
	if IsSystemCommand(formatted) {
		return nil, fmt.Errorf("command conflicts with a system command")
	}

	// Check if preset already exists for this user+command.
	existing, _ := s.GetByUserAndCommand(ctx, userID, formatted)
	if existing != nil {
		return existing, nil
	}

	uid := 0
	if userID != nil {
		uid = *userID
	}

	preset := models.PromptPreset{
		Command:       formatted,
		Prompt:        prompt,
		Description:   description,
		UID:           uid,
		UserID:        userID,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&preset).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

func (s *PromptPresetService) Update(ctx context.Context, id int, userID *int, command, prompt, description string) error {
	formatted := FormatCommand(command)

	// Check for conflict with system commands.
	if IsSystemCommand(formatted) {
		return fmt.Errorf("command conflicts with a system command")
	}

	// Verify ownership.
	preset, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if userID != nil && (preset.UserID == nil || *preset.UserID != *userID) {
		return fmt.Errorf("preset not found")
	}

	updates := map[string]any{
		"command":         formatted,
		"prompt":          prompt,
		"description":     description,
		"last_updated_at": time.Now(),
	}
	return s.db.WithContext(ctx).Model(&models.PromptPreset{}).Where("id = ?", id).Updates(updates).Error
}

func (s *PromptPresetService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.PromptPreset{}, id).Error
}
