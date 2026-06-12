package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProvenanceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.SkillProvenanceLog{})
	require.NoError(t, err)
	return db
}

func TestProvenanceService_RecordOnCreate(t *testing.T) {
	db := setupProvenanceTestDB(t)
	svc := NewProvenanceService(db)
	ctx := context.Background()

	skill := &models.AgentSkill{
		ID:          1,
		WorkspaceID: 1,
		Name:        "test-skill",
		Slug:        "test-skill",
		Content:     "hello world",
		WriteOrigin: "foreground",
	}
	err := svc.Record(ctx, skill, "create", "", "agent", "")
	require.NoError(t, err)

	var logs []models.SkillProvenanceLog
	err = db.WithContext(ctx).Where("skill_id = ?", 1).Find(&logs).Error
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "create", logs[0].Action)
	assert.Equal(t, "foreground", logs[0].WriteOrigin)
	assert.Equal(t, "agent", logs[0].ActorType)
	assert.Equal(t, "hello world", logs[0].Content)
}

func TestProvenanceService_RecordOnPatch(t *testing.T) {
	db := setupProvenanceTestDB(t)
	svc := NewProvenanceService(db)
	ctx := context.Background()

	skill := &models.AgentSkill{
		ID:          2,
		WorkspaceID: 1,
		Name:        "patch-skill",
		Slug:        "patch-skill",
		Content:     "new content after patch",
		WriteOrigin: "foreground",
	}
	err := svc.Record(ctx, skill, "patch", "references/doc.md", "agent", "")
	require.NoError(t, err)

	var logs []models.SkillProvenanceLog
	err = db.WithContext(ctx).Where("skill_id = ?", 2).Find(&logs).Error
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "patch", logs[0].Action)
	assert.Equal(t, "references/doc.md", logs[0].FilePath)
}

func TestProvenanceService_MultipleRecords(t *testing.T) {
	db := setupProvenanceTestDB(t)
	svc := NewProvenanceService(db)
	ctx := context.Background()

	skill := &models.AgentSkill{
		ID:          3,
		WorkspaceID: 1,
		Name:        "multi-skill",
		Slug:        "multi-skill",
		Content:     "v3",
		WriteOrigin: "foreground",
	}
	_ = svc.Record(ctx, skill, "create", "", "agent", "")
	skill.Content = "v4"
	_ = svc.Record(ctx, skill, "edit", "", "agent", "")

	var logs []models.SkillProvenanceLog
	err := db.Where("skill_id = ?", 3).Find(&logs).Error
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}
