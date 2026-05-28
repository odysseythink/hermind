package services

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPromptPresetTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.PromptPreset{})
	require.NoError(t, err)
	return db
}

func TestFormatCommand(t *testing.T) {
	assert.Equal(t, "/summarize", FormatCommand("summarize"))
	assert.Equal(t, "/summarize", FormatCommand("/summarize"))
	assert.Equal(t, "/summarize", FormatCommand("SUMMARIZE"))
	assert.Equal(t, "/sum-ma-rize", FormatCommand("sum ma rize"))
	assert.Equal(t, "/sum_marize", FormatCommand("sum_marize"))
	// Length < 2 should generate random command
	random := FormatCommand("x")
	assert.True(t, len(random) >= 2)
	assert.True(t, strings.HasPrefix(random, "/"))
}

func TestIsSystemCommand(t *testing.T) {
	assert.True(t, IsSystemCommand("/reset"))
	assert.False(t, IsSystemCommand("/custom"))
}

func TestPromptPresetService_CreateAndList(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid := 1
	p1, err := svc.Create(ctx, &uid, "summarize", "Summarize the following text", "summary preset")
	require.NoError(t, err)
	assert.Equal(t, "/summarize", p1.Command)
	assert.Equal(t, "Summarize the following text", p1.Prompt)

	p2, err := svc.Create(ctx, &uid, "translate", "Translate to English", "translation preset")
	require.NoError(t, err)
	assert.Equal(t, "/translate", p2.Command)

	list, err := svc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestPromptPresetService_Create_SystemCommandConflict(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid := 1
	_, err := svc.Create(ctx, &uid, "reset", "Reset memory", "bad")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system command")
}

func TestPromptPresetService_Create_DuplicateReturnsExisting(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid := 1
	p1, err := svc.Create(ctx, &uid, "summarize", "Original prompt", "desc")
	require.NoError(t, err)

	p2, err := svc.Create(ctx, &uid, "summarize", "Different prompt", "other")
	require.NoError(t, err)
	assert.Equal(t, p1.ID, p2.ID) // should return existing
}

func TestPromptPresetService_ListByUser(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid1 := 1
	uid2 := 2
	_, err := svc.Create(ctx, &uid1, "cmd1", "prompt1", "d1")
	require.NoError(t, err)
	_, err = svc.Create(ctx, &uid2, "cmd2", "prompt2", "d2")
	require.NoError(t, err)

	list, err := svc.ListByUser(ctx, uid1)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "/cmd1", list[0].Command)
}

func TestPromptPresetService_Update(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid := 1
	p, err := svc.Create(ctx, &uid, "old", "Old prompt", "desc")
	require.NoError(t, err)

	err = svc.Update(ctx, p.ID, &uid, "new", "New prompt", "new desc")
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "/new", updated.Command)
	assert.Equal(t, "New prompt", updated.Prompt)
}

func TestPromptPresetService_Update_WrongUser(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid1 := 1
	uid2 := 2
	p, err := svc.Create(ctx, &uid1, "cmd", "prompt", "desc")
	require.NoError(t, err)

	err = svc.Update(ctx, p.ID, &uid2, "new", "New prompt", "new desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPromptPresetService_Update_SystemCommandConflict(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid := 1
	p, err := svc.Create(ctx, &uid, "cmd", "prompt", "desc")
	require.NoError(t, err)

	err = svc.Update(ctx, p.ID, &uid, "reset", "Reset", "desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system command")
}

func TestPromptPresetService_Delete(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	uid := 1
	p, err := svc.Create(ctx, &uid, "delete", "Delete me", "desc")
	require.NoError(t, err)

	err = svc.Delete(ctx, p.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, p.ID)
	assert.Error(t, err)
}
