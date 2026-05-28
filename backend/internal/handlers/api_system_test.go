package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAPISystem(t *testing.T, cfg *config.Config) (*apiTestEnv, *APISystemHandler) {
	env := newAPITestEnv(t, cfg)
	sysSvc := services.NewSystemService(env.DB)
	vectorSvc := services.NewVectorService(env.Cfg)
	fsSvc := services.NewFileSystemService(env.Cfg.StorageDir)
	docSvc := services.NewDocumentService(env.DB, env.Cfg, nil, nil, nil, nil, fsSvc)
	wsChatSvc := services.NewWorkspaceChatService(env.DB)
	// SystemHandler only needs sysSvc for UpdateEnv; nil for others.
	webSysHdlr := NewSystemHandler(sysSvc, nil, nil, nil, env.Cfg, nil, nil, nil, nil, nil, nil)
	api := env.Router.Group("/api")
	RegisterAPISystemRoutes(api, env.APIKeySvc, sysSvc, vectorSvc, docSvc, wsChatSvc, webSysHdlr)
	return env, NewAPISystemHandler(sysSvc, vectorSvc, docSvc, wsChatSvc, webSysHdlr)
}

func TestAPISystem_GetAll(t *testing.T) {
	env, _ := setupAPISystem(t, nil)
	require.NoError(t, env.DB.Create(&models.SystemSetting{Key: "llm_provider", Value: strPtr("openai")}).Error)

	req := httptest.NewRequest("GET", "/api/v1/system", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Settings map[string]string `json:"settings"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "openai", body.Settings["llm_provider"])
}

func TestAPISystem_EnvDump(t *testing.T) {
	env, _ := setupAPISystem(t, nil)
	req := httptest.NewRequest("GET", "/api/v1/system/env-dump", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPISystem_VectorCount(t *testing.T) {
	env, _ := setupAPISystem(t, nil)
	req := httptest.NewRequest("GET", "/api/v1/system/vector-count", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]int64
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(0), body["vectorCount"])
}

func TestAPISystem_UpdateEnv(t *testing.T) {
	env, _ := setupAPISystem(t, nil)
	payload, _ := json.Marshal(map[string]string{"title": "My LLM"})
	req := httptest.NewRequest("POST", "/api/v1/system/update-env", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotNil(t, body["newValues"])
}

func TestAPISystem_RemoveDocuments(t *testing.T) {
	env, _ := setupAPISystem(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	payload, _ := json.Marshal(map[string][]string{"names": {"custom-documents/orphan.json"}})
	req := httptest.NewRequest("DELETE", "/api/v1/system/remove-documents", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func strPtr(s string) *string { return &s }
