package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTelegramConfigService_SaveLoad(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.ExternalCommunicationConnector{}))

	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)

	svc := NewTelegramConfigService(db, enc)
	ctx := context.Background()

	cfg := &TelegramConfig{
		BotToken:          "secret-token",
		BotUsername:       "testbot",
		DefaultWorkspace:  "default",
		VoiceResponseMode: "mirror",
		ApprovedUsers: []TelegramUser{
			{ChatID: "123", Username: "alice", ActiveWorkspace: "ws1"},
		},
	}
	require.NoError(t, svc.Save(ctx, cfg))

	loaded, err := svc.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "testbot", loaded.BotUsername)
	assert.Equal(t, "ws1", loaded.ApprovedUsers[0].ActiveWorkspace)
	assert.Empty(t, loaded.BotToken) // json:"-" omits from marshal
}
