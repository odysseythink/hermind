package services

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupEmbedService(t *testing.T) (*EmbedService, *gorm.DB) {
	cfg := &config.Config{StorageDir: t.TempDir()}
	db, err := NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	assert.NoError(t, AutoMigrate(db))
	return NewEmbedService(db, cfg, nil, nil, nil, nil), db
}

func TestEmbedService_Create(t *testing.T) {
	svc, db := setupEmbedService(t)
	ctx := context.Background()

	ws := models.Workspace{Name: "Test", Slug: "test"}
	assert.NoError(t, db.Create(&ws).Error)

	req := dto.CreateEmbedConfigRequest{
		WorkspaceSlug:    "test",
		ChatMode:         "chat",
		AllowlistDomains: []string{"https://example.com"},
	}
	creatorID := 1
	embed, err := svc.Create(ctx, req, &creatorID)
	assert.NoError(t, err)
	assert.NotEmpty(t, embed.UUID)
	assert.Equal(t, "chat", embed.ChatMode)
	assert.True(t, embed.Enabled)
}

func TestEmbedService_Update(t *testing.T) {
	svc, db := setupEmbedService(t)
	ctx := context.Background()

	ws := models.Workspace{Name: "Test", Slug: "test"}
	assert.NoError(t, db.Create(&ws).Error)

	embed, _ := svc.Create(ctx, dto.CreateEmbedConfigRequest{WorkspaceSlug: "test"}, nil)

	enabled := false
	err := svc.Update(ctx, embed.ID, dto.UpdateEmbedConfigRequest{Enabled: &enabled})
	assert.NoError(t, err)

	updated, _ := svc.GetByID(ctx, embed.ID)
	assert.False(t, updated.Enabled)
}

func TestEmbedService_GetByUUID(t *testing.T) {
	svc, db := setupEmbedService(t)
	ctx := context.Background()

	ws := models.Workspace{Name: "Test", Slug: "test"}
	assert.NoError(t, db.Create(&ws).Error)

	embed, _ := svc.Create(ctx, dto.CreateEmbedConfigRequest{WorkspaceSlug: "test"}, nil)

	found, err := svc.GetByUUID(ctx, embed.UUID)
	assert.NoError(t, err)
	assert.Equal(t, embed.ID, found.ID)
}

func TestEmbedService_MarkHistoryInvalid(t *testing.T) {
	svc, db := setupEmbedService(t)
	ctx := context.Background()

	ws := models.Workspace{Name: "Test", Slug: "test"}
	assert.NoError(t, db.Create(&ws).Error)
	embed, _ := svc.Create(ctx, dto.CreateEmbedConfigRequest{WorkspaceSlug: "test"}, nil)

	chat := models.EmbedChat{EmbedID: embed.ID, SessionID: "sess-1", Prompt: "hi", Response: "{}", Include: true}
	assert.NoError(t, db.Create(&chat).Error)

	assert.NoError(t, svc.MarkHistoryInvalid(ctx, embed.ID, "sess-1"))

	var updated models.EmbedChat
	db.First(&updated, chat.ID)
	assert.False(t, updated.Include)
}

func TestEmbedService_CountRecentChats(t *testing.T) {
	svc, db := setupEmbedService(t)
	ctx := context.Background()

	ws := models.Workspace{Name: "Test", Slug: "test"}
	assert.NoError(t, db.Create(&ws).Error)
	embed, _ := svc.Create(ctx, dto.CreateEmbedConfigRequest{WorkspaceSlug: "test"}, nil)

	chat := models.EmbedChat{EmbedID: embed.ID, SessionID: "sess-1", Prompt: "hi", Response: "{}"}
	assert.NoError(t, db.Create(&chat).Error)

	since := time.Now().Add(-24 * time.Hour)
	assert.Equal(t, int64(1), svc.CountRecentChats(ctx, embed.ID, since))
	assert.Equal(t, int64(1), svc.CountRecentSessionChats(ctx, embed.ID, "sess-1", since))
	assert.Equal(t, int64(0), svc.CountRecentSessionChats(ctx, embed.ID, "sess-2", since))
}

func TestEmbedService_StreamChat_Placeholder(t *testing.T) {
	svc, db := setupEmbedService(t)
	ctx := context.Background()

	ws := models.Workspace{Name: "Test", Slug: "test"}
	assert.NoError(t, db.Create(&ws).Error)
	embed, _ := svc.Create(ctx, dto.CreateEmbedConfigRequest{WorkspaceSlug: "test"}, nil)

	req := &dto.EmbedStreamChatRequest{SessionID: "550e8400-e29b-41d4-a716-446655440000", Message: "hello"}
	conn := &dto.ConnectionMeta{Host: "https://example.com", IP: "127.0.0.1"}

	stream, err := svc.StreamChat(ctx, embed, req, conn)
	assert.NoError(t, err)

	var chunks []dto.StreamChatResponse
	for ch := range stream {
		chunks = append(chunks, ch)
	}
	// With no LLM provider, it should return an abort or fallback
	assert.True(t, len(chunks) > 0)
}
