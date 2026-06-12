package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAgentSkillTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{})
	require.NoError(t, err)
	return db
}

func TestAgentSkillService_Create(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "deploy-k8s",
		Description: "Deploy to Kubernetes",
		Category:    "devops",
		Content:     "## Procedure\n1. Build image\n2. Apply manifests",
		Frontmatter: "name: deploy-k8s\ndescription: Deploy to Kubernetes\n",
	})
	require.NoError(t, err)
	assert.Equal(t, "deploy-k8s", skill.Name)
	assert.Equal(t, "devops", skill.Category)
	assert.Equal(t, models.AgentSkillStatusActive, skill.Status)

	// Duplicate name in same workspace should fail
	_, err = svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "deploy-k8s",
		Description: "Duplicate",
		Content:     "...",
	})
	assert.ErrorIs(t, err, ErrSkillNameExists)

	// Same name in different workspace should succeed
	skill2, err := svc.Create(ctx, 2, dto.CreateAgentSkillRequest{
		Name:        "deploy-k8s",
		Description: "Other workspace",
		Content:     "...",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, skill2.WorkspaceID)
}

func TestAgentSkillService_Validation(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	// Invalid name
	_, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "Bad Name!",
		Content: "...",
	})
	assert.ErrorIs(t, err, ErrInvalidSkillName)

	// Missing frontmatter fields
	_, err = svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "ok-name",
		Content:     "...",
		Frontmatter: "name: ok-name\n",
	})
	assert.ErrorIs(t, err, ErrInvalidFrontmatter)

	// Frontmatter too large
	_, err = svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "ok-name",
		Content:     "...",
		Frontmatter: "name: ok-name\ndescription: " + string(make([]byte, MaxSkillFrontmatterChars+1)) + "\n",
	})
	assert.ErrorIs(t, err, ErrInvalidFrontmatter)
}

func TestAgentSkillService_Patch(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "test-skill",
		Content: "hello world",
	})
	require.NoError(t, err)

	patched, err := svc.Patch(ctx, 1, skill.Slug, dto.PatchAgentSkillRequest{
		OldString: "world",
		NewString: "universe",
	})
	require.NoError(t, err)
	assert.Equal(t, "hello universe", patched.Content)
	assert.Equal(t, 1, patched.PatchCount)

	// No match
	_, err = svc.Patch(ctx, 1, skill.Slug, dto.PatchAgentSkillRequest{
		OldString: "nonexistent",
		NewString: "...",
	})
	assert.ErrorIs(t, err, ErrPatchNoMatch)

	// Ambiguous match
	_, err = svc.Patch(ctx, 1, skill.Slug, dto.PatchAgentSkillRequest{
		OldString: "l",
		NewString: "x",
	})
	assert.ErrorIs(t, err, ErrPatchAmbiguous)
}

func TestAgentSkillService_PatchFile(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "file-patch-skill",
		Content: "...",
	})
	require.NoError(t, err)

	// Write initial file
	err = svc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/guide.md",
		Content:  "hello world foo bar",
	})
	require.NoError(t, err)

	// Patch file
	file, err := svc.PatchFile(ctx, 1, skill.Slug, dto.PatchSkillFileRequest{
		FilePath:  "references/guide.md",
		OldString: "world",
		NewString: "universe",
	})
	require.NoError(t, err)
	assert.Equal(t, "hello universe foo bar", file.Content)

	// Ambiguous patch on file
	_, err = svc.PatchFile(ctx, 1, skill.Slug, dto.PatchSkillFileRequest{
		FilePath:  "references/guide.md",
		OldString: "o",
		NewString: "x",
	})
	assert.ErrorIs(t, err, ErrPatchAmbiguous)

	// Patch nonexistent file
	_, err = svc.PatchFile(ctx, 1, skill.Slug, dto.PatchSkillFileRequest{
		FilePath:  "references/missing.md",
		OldString: "x",
		NewString: "y",
	})
	assert.ErrorIs(t, err, ErrSkillNotFound)
}

func TestAgentSkillService_Files(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "file-test",
		Content: "...",
	})
	require.NoError(t, err)

	// Write file
	err = svc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/api.md",
		Content:  "# API Docs",
	})
	require.NoError(t, err)

	// List files
	files, err := svc.ListFiles(ctx, skill.ID)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "references/api.md", files[0].FilePath)

	// Get file
	file, err := svc.GetFile(ctx, skill.ID, "references/api.md")
	require.NoError(t, err)
	assert.Equal(t, "# API Docs", file.Content)

	// Invalid path
	err = svc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "../etc/passwd",
		Content:  "bad",
	})
	assert.ErrorIs(t, err, ErrInvalidFilePath)

	// Remove file
	err = svc.RemoveFile(ctx, 1, skill.Slug, "references/api.md")
	require.NoError(t, err)

	files, err = svc.ListFiles(ctx, skill.ID)
	require.NoError(t, err)
	assert.Len(t, files, 0)
}

func TestAgentSkillService_UpdateRename(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "old-name",
		Content: "...",
	})
	require.NoError(t, err)
	assert.Equal(t, "old-name", skill.Slug)

	// Rename skill
	updated, err := svc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Name: "new-name",
	})
	require.NoError(t, err)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, "new-name", updated.Slug)

	// Old slug should not exist
	_, err = svc.GetBySlug(ctx, 1, "old-name")
	assert.ErrorIs(t, err, ErrSkillNotFound)

	// New slug should exist
	found, err := svc.GetBySlug(ctx, 1, "new-name")
	require.NoError(t, err)
	assert.Equal(t, "new-name", found.Name)
}

func TestAgentSkillService_WorkspaceIsolation(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "ws-skill",
		Content: "...",
	})
	require.NoError(t, err)

	// Same slug in workspace 2 should not find workspace 1's skill
	_, err = svc.GetBySlug(ctx, 2, skill.Slug)
	assert.ErrorIs(t, err, ErrSkillNotFound)

	// List in workspace 2 should be empty
	list, err := svc.List(ctx, 2, false)
	require.NoError(t, err)
	assert.Len(t, list, 0)

	// List in workspace 1 should find it
	list, err = svc.List(ctx, 1, false)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestAgentSkillService_CuratorTransitions(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	// Create an old active skill
	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "old-skill",
		Content: "...",
	})
	require.NoError(t, err)

	// Manually set created_at to 100 days ago
	db.Model(skill).Update("created_at", db.Raw("datetime('now', '-100 days')"))

	counts, err := svc.ApplyCuratorTransitions(ctx, 30, 90)
	require.NoError(t, err)
	assert.Equal(t, 1, counts["checked"])
	assert.Equal(t, 1, counts["archived"])

	// Verify archived
	updated, err := svc.GetBySlug(ctx, 1, skill.Slug)
	require.NoError(t, err)
	assert.Equal(t, models.AgentSkillStatusArchived, updated.Status)
}

func TestAgentSkillService_CuratorStaleAndReactivate(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	// Create a skill that is 45 days old (between 30 stale and 90 archive)
	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "aging-skill",
		Content: "...",
	})
	require.NoError(t, err)
	db.Model(skill).Update("created_at", db.Raw("datetime('now', '-45 days')"))

	counts, err := svc.ApplyCuratorTransitions(ctx, 30, 90)
	require.NoError(t, err)
	assert.Equal(t, 1, counts["checked"])
	assert.Equal(t, 1, counts["marked_stale"])
	assert.Equal(t, 0, counts["archived"])

	// Verify stale
	updated, err := svc.GetBySlug(ctx, 1, skill.Slug)
	require.NoError(t, err)
	assert.Equal(t, models.AgentSkillStatusStale, updated.Status)

	// Simulate recent activity by bumping use count (sets last_used_at)
	err = svc.BumpUse(ctx, 1, skill.Slug)
	require.NoError(t, err)

	// Run curator again — should reactivate
	counts, err = svc.ApplyCuratorTransitions(ctx, 30, 90)
	require.NoError(t, err)
	assert.Equal(t, 1, counts["checked"])
	assert.Equal(t, 1, counts["reactivated"])

	reactivated, err := svc.GetBySlug(ctx, 1, skill.Slug)
	require.NoError(t, err)
	assert.Equal(t, models.AgentSkillStatusActive, reactivated.Status)
}

func TestAgentSkillService_PinnedProtection(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-skill",
		Content: "...",
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	// Try to delete pinned skill
	err = svc.Delete(ctx, 1, skill.Slug)
	assert.ErrorContains(t, err, "pinned")
}

func boolPtr(b bool) *bool { return &b }

func TestAgentSkillService_WriteOriginDefault(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "origin-test",
		Description: "WriteOrigin default",
		Content:     "test content",
	})
	require.NoError(t, err)
	assert.Equal(t, "foreground", skill.WriteOrigin)

	// Explicit WriteOrigin
	skill2, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "origin-test2",
		Description: "Explicit origin",
		Content:     "...",
		WriteOrigin: "background_review",
	})
	require.NoError(t, err)
	assert.Equal(t, "background_review", skill2.WriteOrigin)
}
