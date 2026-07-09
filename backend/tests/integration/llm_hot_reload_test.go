package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHotReload_KeyChangeSurfacesError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir()}
	db, err := services.NewDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	require.NoError(t, services.AutoMigrate(db))

	sysSvc := services.NewSystemService(db)
	apiKeySvc := services.NewAPIKeyService(db)
	authSvc := services.NewAuthService(db, cfg, nil)
	adminSvc := services.NewAdminService(db)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)

	// Seed a working provider so we can prove it survives a bad reload.
	require.NoError(t, sysSvc.SetSetting(context.Background(), "LLMProvider", "openai"))
	require.NoError(t, sysSvc.SetSetting(context.Background(), "OpenAiModelPref", "gpt-4o-mini"))
	require.NoError(t, sysSvc.SetSetting(context.Background(), "OpenAiKey", "sk-valid"))

	dbSettings, err := sysSvc.GetAllSettings(context.Background())
	require.NoError(t, err)

	llmMgr, err := providers.NewManagedLLMProvider(cfg, sysSvc, dbSettings)
	require.NoError(t, err)
	sysSvc.RegisterObserver(llmMgr)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterSystemRoutes(api, sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, nil, nil, nil)

	// Step 1: Save a setting that triggers a reload with a non-existent provider.
	// The reload should fail but DB should persist the value.
	body, _ := json.Marshal(dto.UpdateSettingRequest{Key: "LLMProvider", Value: "nonexistent"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported LLM provider")

	// Step 2: The LLM manager should still have the old provider (openai).
	lm := llmMgr.LanguageModel()
	assert.Equal(t, "openai", lm.Provider())
	assert.Equal(t, "gpt-4o-mini", lm.Model())
}

func TestHotReload_UnrelatedSettingDoesNotTriggerReload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir()}
	db, err := services.NewDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	require.NoError(t, services.AutoMigrate(db))

	sysSvc := services.NewSystemService(db)
	apiKeySvc := services.NewAPIKeyService(db)
	authSvc := services.NewAuthService(db, cfg, nil)
	adminSvc := services.NewAdminService(db)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)

	require.NoError(t, sysSvc.SetSetting(context.Background(), "LLMProvider", "openai"))
	require.NoError(t, sysSvc.SetSetting(context.Background(), "OpenAiModelPref", "gpt-4o-mini"))
	require.NoError(t, sysSvc.SetSetting(context.Background(), "OpenAiKey", "sk-valid"))

	dbSettings, err := sysSvc.GetAllSettings(context.Background())
	require.NoError(t, err)

	llmMgr, err := providers.NewManagedLLMProvider(cfg, sysSvc, dbSettings)
	require.NoError(t, err)
	sysSvc.RegisterObserver(llmMgr)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterSystemRoutes(api, sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, nil, nil, nil)

	// Save an unrelated setting — this should NOT trigger a reload.
	body, _ := json.Marshal(dto.UpdateSettingRequest{Key: "logo_filename", Value: "test-logo.png"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Provider should still be openai.
	lm := llmMgr.LanguageModel()
	assert.Equal(t, "openai", lm.Provider())
}
