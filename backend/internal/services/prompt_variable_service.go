package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

// DefaultPromptVariables are built-in variables available to all users.
// They mirror the Node.js SystemPromptVariables.DEFAULT_VARIABLES.
var DefaultPromptVariables = []models.PromptVariable{
	{Key: "time", Value: strPtr("[Current time]"), Description: strPtr("Current time"), Type: "system"},
	{Key: "date", Value: strPtr("[Current date]"), Description: strPtr("Current date"), Type: "system"},
	{Key: "datetime", Value: strPtr("[Current date and time]"), Description: strPtr("Current date and time"), Type: "system"},
	{Key: "user.id", Value: strPtr("[User ID]"), Description: strPtr("Current user's ID"), Type: "user"},
	{Key: "user.name", Value: strPtr("[User name]"), Description: strPtr("Current user's username"), Type: "user"},
	{Key: "user.bio", Value: strPtr("[User bio]"), Description: strPtr("Current user's bio field from their profile"), Type: "user"},
	{Key: "workspace.id", Value: strPtr("[Workspace ID]"), Description: strPtr("Current workspace's ID"), Type: "workspace"},
	{Key: "workspace.name", Value: strPtr("[Workspace name]"), Description: strPtr("Current workspace's name"), Type: "workspace"},
}

type PromptVariableService struct {
	db *gorm.DB
}

func NewPromptVariableService(db *gorm.DB) *PromptVariableService {
	return &PromptVariableService{db: db}
}

// List returns default variables plus user-defined variables from the database.
// If userID is 0, user-scoped variables are still returned (front-end filters as needed).
func (s *PromptVariableService) List(ctx context.Context) ([]models.PromptVariable, error) {
	var vars []models.PromptVariable
	if err := s.db.WithContext(ctx).Order("id desc").Find(&vars).Error; err != nil {
		return nil, err
	}
	// Merge defaults first, then DB vars.
	return append(DefaultPromptVariables, vars...), nil
}

func (s *PromptVariableService) GetByID(ctx context.Context, id int) (*models.PromptVariable, error) {
	var v models.PromptVariable
	if err := s.db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *PromptVariableService) GetByKey(ctx context.Context, key string) (*models.PromptVariable, error) {
	var v models.PromptVariable
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *PromptVariableService) Create(ctx context.Context, key, value, description string) (*models.PromptVariable, error) {
	if err := validateVariableKey(key, true, s, ctx); err != nil {
		return nil, err
	}
	v := models.PromptVariable{
		Key:         key,
		Value:       &value,
		Description: &description,
		Type:        "static",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *PromptVariableService) Update(ctx context.Context, id int, key, value, description string) error {
	if err := validateVariableKey(key, false, s, ctx); err != nil {
		return err
	}
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("variable not found")
	}
	updates := map[string]any{
		"key":         key,
		"value":       value,
		"description": description,
		"updated_at":  time.Now(),
	}
	// Preserve existing type.
	if existing.Type != "" {
		updates["type"] = existing.Type
	}
	return s.db.WithContext(ctx).Model(&models.PromptVariable{}).Where("id = ?", id).Updates(updates).Error
}

func (s *PromptVariableService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.PromptVariable{}, id).Error
}

func validateVariableKey(key string, checkExisting bool, s *PromptVariableService, ctx context.Context) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if len(key) > 255 {
		return fmt.Errorf("key must be less than 255 characters")
	}
	if len(key) < 3 {
		return fmt.Errorf("key must be at least 3 characters")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(key) {
		return fmt.Errorf("key must contain only letters, numbers and underscores")
	}
	if strings.HasPrefix(key, "user.") {
		return fmt.Errorf("key cannot start with 'user.'")
	}
	if strings.HasPrefix(key, "system.") {
		return fmt.Errorf("key cannot start with 'system.'")
	}
	if checkExisting {
		_, err := s.GetByKey(ctx, key)
		if err == nil {
			return fmt.Errorf("system prompt variable with this key already exists")
		}
	}
	return nil
}

// strPtr helper is provided by db.go in the same package.
