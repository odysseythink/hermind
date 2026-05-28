package services

import (
	"fmt"
	"path/filepath"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewDB(cfg *config.Config) (*gorm.DB, error) {
	if cfg.DatabaseURL != "" {
		db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		return db, nil
	}
	dbPath := filepath.Join(cfg.StorageDir, "anythingllm.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Invite{},
		&models.APIKey{},
		&models.PasswordResetToken{},
		&models.RecoveryCode{},
		&models.Workspace{},
		&models.WorkspaceUser{},
		&models.WorkspaceChat{},
		&models.WorkspaceDocument{},
		&models.DocumentVector{},
		&models.WorkspaceThread{},
		&models.SystemSetting{},
		&models.EmbedConfig{},
		&models.EmbedChat{},
		&models.PromptPreset{},
		&models.PromptVariable{},
		&models.EventLog{},
		&models.TemporaryAuthToken{},
		&models.WorkspaceAgentInvocation{},
		&models.WorkspaceParsedFile{},
		&models.DocumentSyncQueue{},
		&models.OutlookOAuthToken{},
	)
}

func SeedDefaults(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.SystemSetting{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaults := []models.SystemSetting{
		{Key: "setup_complete", Value: strPtr("false")},
		{Key: "llm_provider", Value: strPtr("openai")},
		{Key: "vector_db", Value: strPtr("lancedb")},
	}
	for _, s := range defaults {
		if err := db.Create(&s).Error; err != nil {
			return err
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }
