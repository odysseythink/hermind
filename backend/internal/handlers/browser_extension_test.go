package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBrowserExtensionTestEnv(t *testing.T) (*gin.Engine, *gorm.DB, *services.BrowserExtensionService, *services.WorkspaceService, *services.DocumentService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test"}
	db, err := services.NewDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	require.NoError(t, services.AutoMigrate(db))

	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	extSvc := services.NewBrowserExtensionService(db)
	wsSvc := services.NewWorkspaceService(db, cfg, nil)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	docSvc := services.NewDocumentService(db, cfg, nil, nil, nil, nil, fsSvc)

	r := gin.New()
	api := r.Group("/api")
	RegisterBrowserExtensionRoutes(api, extSvc, wsSvc, docSvc, authSvc, cfg)
	return r, db, extSvc, wsSvc, docSvc
}

func TestBrowserExtensionHandler_Check(t *testing.T) {
	r, _, extSvc, _, _ := newBrowserExtensionTestEnv(t)
	key, err := extSvc.CreateKey(t.Context(), nil)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/browser-extension/check", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["connected"])
	assert.NotNil(t, body["workspaces"])
	assert.Equal(t, float64(key.ID), body["apiKeyId"])
}

func TestBrowserExtensionHandler_Workspaces(t *testing.T) {
	r, _, extSvc, wsSvc, _ := newBrowserExtensionTestEnv(t)
	key, err := extSvc.CreateKey(t.Context(), nil)
	require.NoError(t, err)

	ws, err := wsSvc.Create(t.Context(), 0, dto.CreateWorkspaceRequest{Name: "Test WS"})
	require.NoError(t, err)
	require.NotNil(t, ws)

	req := httptest.NewRequest("GET", "/api/browser-extension/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	workspaces, ok := body["workspaces"].([]any)
	require.True(t, ok)
	assert.Len(t, workspaces, 1)
}

func TestBrowserExtensionHandler_UploadContent(t *testing.T) {
	r, _, extSvc, _, _ := newBrowserExtensionTestEnv(t)
	key, err := extSvc.CreateKey(t.Context(), nil)
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]any{
		"textContent": "hello from extension",
		"metadata": map[string]string{
			"title": "Test Title",
			"url":   "http://example.com",
		},
	})
	req := httptest.NewRequest("POST", "/api/browser-extension/upload-content", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestBrowserExtensionHandler_EmbedContent(t *testing.T) {
	r, _, extSvc, wsSvc, _ := newBrowserExtensionTestEnv(t)
	key, err := extSvc.CreateKey(t.Context(), nil)
	require.NoError(t, err)

	ws, err := wsSvc.Create(t.Context(), 0, dto.CreateWorkspaceRequest{Name: "Embed WS"})
	require.NoError(t, err)
	require.NotNil(t, ws)

	payload, _ := json.Marshal(map[string]any{
		"workspaceId": ws.ID,
		"textContent": "embed me",
		"metadata": map[string]string{
			"title": "Embed Title",
			"url":   "http://example.com",
		},
	})
	req := httptest.NewRequest("POST", "/api/browser-extension/embed-content", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestBrowserExtensionHandler_Disconnect(t *testing.T) {
	r, _, extSvc, _, _ := newBrowserExtensionTestEnv(t)
	key, err := extSvc.CreateKey(t.Context(), nil)
	require.NoError(t, err)

	// Disconnect
	req := httptest.NewRequest("DELETE", "/api/browser-extension/disconnect", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify key is now invalid
	req = httptest.NewRequest("GET", "/api/browser-extension/check", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}
