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

func setupPromptVariableTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.PromptVariable{})
	require.NoError(t, err)
	return db
}

func TestPromptVariableService_List(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	// Default variables should always be present.
	list, err := svc.List(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), len(DefaultPromptVariables))

	// Create a custom variable.
	_, err = svc.Create(ctx, "custom_key", "custom_value", "custom desc")
	require.NoError(t, err)

	list, err = svc.List(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), len(DefaultPromptVariables)+1)
}

func TestPromptVariableService_CreateAndGet(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	v, err := svc.Create(ctx, "my_var", "hello", "desc")
	require.NoError(t, err)
	assert.Equal(t, "my_var", v.Key)
	assert.Equal(t, "hello", *v.Value)

	byKey, err := svc.GetByKey(ctx, "my_var")
	require.NoError(t, err)
	assert.Equal(t, v.ID, byKey.ID)
}

func TestPromptVariableService_Create_InvalidKey(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	_, err := svc.Create(ctx, "", "val", "desc")
	assert.Error(t, err)

	_, err = svc.Create(ctx, "ab", "val", "desc")
	assert.Error(t, err)

	_, err = svc.Create(ctx, "user.name", "val", "desc")
	assert.Error(t, err)

	_, err = svc.Create(ctx, "system.x", "val", "desc")
	assert.Error(t, err)

	_, err = svc.Create(ctx, "has space", "val", "desc")
	assert.Error(t, err)
}

func TestPromptVariableService_Create_DuplicateKey(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	_, err := svc.Create(ctx, "unique_key", "val1", "desc")
	require.NoError(t, err)

	_, err = svc.Create(ctx, "unique_key", "val2", "desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestPromptVariableService_Update(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	v, err := svc.Create(ctx, "old_key", "old_value", "old desc")
	require.NoError(t, err)

	err = svc.Update(ctx, v.ID, "new_key", "new_value", "new desc")
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_key", updated.Key)
	assert.Equal(t, "new_value", *updated.Value)
}

func TestPromptVariableService_Update_NotFound(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	err := svc.Update(ctx, 9999, "key", "val", "desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPromptVariableService_Delete(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	v, err := svc.Create(ctx, "del_me", "val", "desc")
	require.NoError(t, err)

	err = svc.Delete(ctx, v.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, v.ID)
	assert.Error(t, err)
}
