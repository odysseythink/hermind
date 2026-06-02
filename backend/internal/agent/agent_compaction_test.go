package agent

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildCompressor_EnabledWorkspace_ReturnsNonNil(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))

	ws := &models.Workspace{Name: "Test", Slug: "test"}
	require.NoError(t, db.Create(ws).Error)
	trueVal := true
	ws.CompressEnabled = &trueVal
	require.NoError(t, db.Save(ws).Error)

	lm := &mockLanguageModel{provider: "openai", model: "gpt-4o-mini"}
	comp := buildCompressor(db, ws, lm, nil, nil)
	require.NotNil(t, comp, "expected compressor to be non-nil when workspace compression is enabled")
}

func TestBuildCompressor_DisabledWorkspace_ReturnsNil(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))

	ws := &models.Workspace{Name: "Test", Slug: "test"}
	require.NoError(t, db.Create(ws).Error)

	lm := &mockLanguageModel{provider: "openai", model: "gpt-4o-mini"}
	comp := buildCompressor(db, ws, lm, nil, nil)
	require.Nil(t, comp, "expected compressor to be nil when workspace compression is disabled")
}

func TestRunAgentDirectly_WithCompressionEnabled_Completes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))

	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	rt := NewRuntime(Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
	})

	ws := &models.Workspace{Name: "Test", Slug: "test"}
	require.NoError(t, db.Create(ws).Error)
	trueVal := true
	ws.CompressEnabled = &trueVal
	require.NoError(t, db.Save(ws).Error)

	uid, err := rt.CreateInvocation(context.Background(), ws, nil, nil, "@agent do work")
	require.NoError(t, err)

	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		replies: []string{"Done!", "TERMINATE"},
	}
	rt.SetTestLanguageModelOverride(mock)

	io := NewTelegramAgentIO(func(string) error { return nil }, nil)
	input := NewTelegramInput()

	err = rt.RunAgentDirectly(context.Background(), uid, io, input)
	require.NoError(t, err)
}
