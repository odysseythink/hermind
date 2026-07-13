package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWSDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func TestWorkspaceService_UpdatePin(t *testing.T) {
	db := setupWSDB(t)
	svc := NewWorkspaceService(db, &config.Config{}, nil)

	ws := &models.Workspace{Name: "ws1", Slug: "ws1"}
	require.NoError(t, db.Create(ws).Error)
	f := false
	doc := &models.WorkspaceDocument{
		DocId:       "doc-1",
		Filename:    "a.txt",
		Docpath:     "custom-documents/a.txt-xyz.json",
		WorkspaceID: ws.ID,
		Pinned:      &f,
	}
	require.NoError(t, db.Create(doc).Error)

	// Pin
	err := svc.UpdatePin(context.Background(), ws.ID, "custom-documents/a.txt-xyz.json", true)
	require.NoError(t, err)

	var got models.WorkspaceDocument
	require.NoError(t, db.First(&got, doc.ID).Error)
	require.NotNil(t, got.Pinned)
	assert.True(t, *got.Pinned)

	// Unpin
	err = svc.UpdatePin(context.Background(), ws.ID, "custom-documents/a.txt-xyz.json", false)
	require.NoError(t, err)
	require.NoError(t, db.First(&got, doc.ID).Error)
	assert.False(t, *got.Pinned)
}

func TestWorkspaceService_UpdatePin_NotFound(t *testing.T) {
	db := setupWSDB(t)
	svc := NewWorkspaceService(db, &config.Config{}, nil)

	err := svc.UpdatePin(context.Background(), 9999, "missing.json", true)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestWorkspaceService_Update_PromptHistoryHook(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.PromptHistory{}))
	phSvc := NewPromptHistoryService(db)
	svc := NewWorkspaceService(db, &config.Config{}, phSvc)

	def := svc.defaultPrompt
	old := "Custom prompt v1"
	newP := "Custom prompt v2"

	cases := []struct {
		name        string
		prevPrompt  *string
		newPrompt   *string
		expectRow   bool
	}{
		{"both set, distinct, not default", &old, &newP, true},
		{"prev is default", &def, &newP, false},
		{"prev is nil", nil, &newP, false},
		{"prev equals new", &old, &old, false},
		{"new is empty string", &old, strPtrFunc(""), false},
		{"new is nil", &old, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := models.Workspace{
				Name: "t", Slug: "s-" + tc.name, OpenAiPrompt: tc.prevPrompt,
			}
			require.NoError(t, db.Create(&ws).Error)
			uid := 1
			err := svc.Update(context.Background(), ws.Slug,
				dto.UpdateWorkspaceRequest{OpenAiPrompt: tc.newPrompt}, &uid)
			require.NoError(t, err)
			var count int64
			db.Model(&models.PromptHistory{}).Where("workspace_id = ?", ws.ID).Count(&count)
			if tc.expectRow {
				assert.Equal(t, int64(1), count, "expected one history row")
			} else {
				assert.Equal(t, int64(0), count, "expected no history row")
			}
		})
	}
}

func strPtrFunc(s string) *string { return &s }

func TestWorkspaceService_Update_AgentFields(t *testing.T) {
	db := setupWSDB(t)
	svc := NewWorkspaceService(db, &config.Config{}, nil)

	ws := &models.Workspace{Name: "agent-ws", Slug: "agent-ws"}
	require.NoError(t, db.Create(ws).Error)

	uid := 1
	provider := "openai"
	model := "gpt-4o-mini"
	err := svc.Update(context.Background(), ws.Slug, dto.UpdateWorkspaceRequest{
		AgentProvider: &provider,
		AgentModel:    &model,
	}, &uid)
	require.NoError(t, err)

	var got models.Workspace
	require.NoError(t, db.First(&got, ws.ID).Error)
	require.NotNil(t, got.AgentProvider)
	assert.Equal(t, provider, *got.AgentProvider)
	require.NotNil(t, got.AgentModel)
	assert.Equal(t, model, *got.AgentModel)
}
