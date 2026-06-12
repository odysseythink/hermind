package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBackupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{})
	require.NoError(t, err)
	return db
}

func TestBackupService_SnapshotCreatesFile(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a skill with a file
	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "backup-test",
		Content: "## backup content",
	})
	require.NoError(t, err)

	err = skillSvc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/guide.md",
		Content:  "# Guide",
	})
	require.NoError(t, err)

	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	snapshotID, err := backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, snapshotID)

	// Verify file exists
	path := filepath.Join(tmpDir, "skill-backups", "1", snapshotID+".json")
	_, err = os.Stat(path)
	require.NoError(t, err, "snapshot file must exist at %s", path)
}

func TestBackupService_SnapshotPruneOld(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	// Create 11 snapshots — the oldest should be pruned (keep=10)
	for i := 0; i < 11; i++ {
		_, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
			Name:    "prune-test-" + string(rune('a'+i%26)),
			Content: "...",
		})
		require.NoError(t, err)
		_, err = backupSvc.Snapshot(ctx, 1)
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(filepath.Join(tmpDir, "skill-backups", "1"))
	require.NoError(t, err)
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	assert.LessOrEqual(t, count, 10, "should keep at most 10 snapshots")
}

func TestBackupService_RestoreIntegrity(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "restore-test",
		Content: "original content",
	})
	require.NoError(t, err)

	err = skillSvc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/doc.md",
		Content:  "file content",
	})
	require.NoError(t, err)

	// Snapshot
	snapshotID, err := backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)

	// Mutate: delete the skill and recreate something different
	err = skillSvc.Delete(ctx, 1, skill.Slug)
	require.NoError(t, err)

	_, err = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "different-skill",
		Content: "different",
	})
	require.NoError(t, err)

	// Restore
	err = backupSvc.Restore(ctx, 1, snapshotID)
	require.NoError(t, err)

	// Verify original skill is back
	restored, err := skillSvc.GetBySlug(ctx, 1, "restore-test")
	require.NoError(t, err)
	assert.Equal(t, "original content", restored.Content)

	// Verify the file is restored
	files, err := skillSvc.ListFiles(ctx, restored.ID)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "file content", files[0].Content)
	assert.Equal(t, "references/doc.md", files[0].FilePath)

	// The "different-skill" should NOT exist after restore
	_, err = skillSvc.GetBySlug(ctx, 1, "different-skill")
	assert.ErrorIs(t, err, ErrSkillNotFound)
}

func TestBackupService_RestoreInvalidSnapshot(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	err := backupSvc.Restore(ctx, 1, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot not found")
}

func TestBackupService_List(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	_, _ = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "list-1", Content: "..."})
	_, err := backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)

	_, _ = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "list-2", Content: "..."})
	_, err = backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)

	infos, err := backupSvc.List(ctx, 1)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(infos), 1)
}
