package tools

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func boolPtr(b bool) *bool { return &b }

func setupAgentToolTestDB(t *testing.T) (*gorm.DB, *services.AgentSkillService, *services.ProvenanceService) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.SkillProvenanceLog{})
	require.NoError(t, err)
	skillSvc := services.NewAgentSkillService(db)
	provSvc := services.NewProvenanceService(db)
	return db, skillSvc, provSvc
}

func TestPinBlocksAgentEdit(t *testing.T) {
	db, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-edit",
		Content: "original",
	})
	require.NoError(t, err)

	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
		Emit:          func(msg string) {},
	}

	result, _ := skillManageEdit(ctx, tc, skillSvc, provSvc, 1, skillManageArgs{
		Name:    "pinned-edit",
		Content: "---\nname: pinned-edit\ndescription: test\n---\nnew content",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be edited")

	// Verify skill was NOT edited
	updated, _ := skillSvc.GetBySlug(ctx, 1, skill.Slug)
	assert.Equal(t, "original", updated.Content)

	_ = db // used in setup
}

func TestPinBlocksAgentPatch(t *testing.T) {
	_, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-patch",
		Content: "hello world",
	})
	require.NoError(t, err)
	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
		Emit:          func(msg string) {},
	}

	result, _ := skillManagePatch(ctx, tc, skillSvc, provSvc, 1, skillManageArgs{
		Name:      "pinned-patch",
		OldString: "world",
		NewString: "universe",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be patched")
}

func TestPinBlocksAgentWriteFile(t *testing.T) {
	_, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-write",
		Content: "...",
	})
	require.NoError(t, err)
	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
		Emit:          func(msg string) {},
	}

	result, _ := skillManageWriteFile(ctx, tc, skillSvc, provSvc, 1, skillManageArgs{
		Name:        "pinned-write",
		FilePath:    "references/test.md",
		FileContent: "new file",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be modified")
}

func TestPinBlocksAgentRemoveFile(t *testing.T) {
	_, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-rm",
		Content: "...",
	})
	require.NoError(t, err)
	err = skillSvc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/doc.md",
		Content:  "doc",
	})
	require.NoError(t, err)
	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
		Emit:          func(msg string) {},
	}

	result, _ := skillManageRemoveFile(ctx, tc, skillSvc, provSvc, 1, skillManageArgs{
		Name:     "pinned-rm",
		FilePath: "references/doc.md",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be removed")

	// Verify file is still there
	updated, _ := skillSvc.GetBySlug(ctx, 1, skill.Slug)
	files, _ := skillSvc.ListFiles(ctx, updated.ID)
	assert.Len(t, files, 1)
}

func TestAgentEditRecordsProvenance(t *testing.T) {
	db, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "prov-edit",
		Content: "before edit",
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
		Emit:          func(msg string) {},
	}

	_, _ = skillManageEdit(ctx, tc, skillSvc, provSvc, 1, skillManageArgs{
		Name:    "prov-edit",
		Content: "---\nname: prov-edit\ndescription: test\n---\nafter edit",
	})

	var logs []models.SkillProvenanceLog
	err = db.Where("skill_id = ?", skill.ID).Find(&logs).Error
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(logs), 1)
	assert.Equal(t, "edit", logs[len(logs)-1].Action)
	assert.Equal(t, "agent", logs[len(logs)-1].ActorType)
}
