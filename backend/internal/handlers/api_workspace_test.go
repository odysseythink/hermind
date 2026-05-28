package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerWorkspaceRoutesForTest(env *apiTestEnv) {
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
	api := env.Router.Group("/api")
	RegisterAPIWorkspaceRoutes(api, env.APIKeySvc, wsSvc, nil, nil, nil, env.DB)
}

func TestAPIWorkspace_List(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "a", Slug: "a"}).Error)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "b", Slug: "b"}).Error)
	registerWorkspaceRoutesForTest(env)

	req := httptest.NewRequest("GET", "/api/v1/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Workspaces []models.Workspace `json:"workspaces"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Workspaces, 2)
}

func TestAPIWorkspace_Create(t *testing.T) {
	env := newAPITestEnv(t, nil)
	registerWorkspaceRoutesForTest(env)

	payload, _ := json.Marshal(map[string]string{"name": "new-ws"})
	req := httptest.NewRequest("POST", "/api/v1/workspace/new", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Workspace *models.Workspace `json:"workspace"`
		Message   string            `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Workspace)
	assert.Equal(t, "new-ws", body.Workspace.Name)
}

func TestAPIWorkspace_GetBySlug(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w-slug"}).Error)
	registerWorkspaceRoutesForTest(env)

	req := httptest.NewRequest("GET", "/api/v1/workspace/w-slug", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Workspace *models.Workspace `json:"workspace"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "w-slug", body.Workspace.Slug)
}

func TestAPIWorkspace_Update(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
	registerWorkspaceRoutesForTest(env)

	payload := []byte(`{"name":"renamed"}`)
	req := httptest.NewRequest("POST", "/api/v1/workspace/w/update", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got models.Workspace
	require.NoError(t, env.DB.Where("slug=?", "w").First(&got).Error)
	assert.Equal(t, "renamed", got.Name)
}

func TestAPIWorkspace_UpdatePin(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, env.DB.Create(ws).Error)
	f := false
	require.NoError(t, env.DB.Create(&models.WorkspaceDocument{
		DocId: "d1", Filename: "a.txt", Docpath: "custom-documents/a.json",
		WorkspaceID: ws.ID, Pinned: &f,
	}).Error)
	registerWorkspaceRoutesForTest(env)

	payload := []byte(`{"docPath":"custom-documents/a.json","pinStatus":true}`)
	req := httptest.NewRequest("POST", "/api/v1/workspace/w/update-pin", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIWorkspace_Delete(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
	registerWorkspaceRoutesForTest(env)

	req := httptest.NewRequest("DELETE", "/api/v1/workspace/w", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var count int64
	env.DB.Model(&models.Workspace{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAPIWorkspace_Chats(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, env.DB.Create(ws).Error)
	require.NoError(t, env.DB.Create(&models.WorkspaceChat{WorkspaceID: ws.ID, Prompt: "hello", Response: "hi"}).Error)
	require.NoError(t, env.DB.Create(&models.WorkspaceChat{WorkspaceID: ws.ID, Prompt: "how are you", Response: "fine"}).Error)
	registerWorkspaceRoutesForTest(env)

	req := httptest.NewRequest("GET", "/api/v1/workspace/w/chats", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		History []map[string]any `json:"history"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	// Each WorkspaceChat produces 2 history entries (user + assistant).
	require.Len(t, body.History, 4)
}

func TestAPIWorkspace_VectorSearch_NoEmbedder(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
	registerWorkspaceRoutesForTest(env)

	payload := []byte(`{"text":"test","topN":5}`)
	req := httptest.NewRequest("POST", "/api/v1/workspace/w/vector-search", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAPIWorkspace_UpdateEmbeddings(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, env.DB.Create(ws).Error)

	// Use a real DocumentService with nil embedder — UpdateEmbeddings will fail
	// on the embed step, but the handler returns 200 + error message (Node parity).
	docSvc := services.NewDocumentService(env.DB, env.Cfg, nil, nil, nil, nil, nil)
	api := env.Router.Group("/api")
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg)
	RegisterAPIWorkspaceRoutes(api, env.APIKeySvc, wsSvc, nil, nil, docSvc, env.DB)

	payload := []byte(`{"adds":[],"removes":[]}`)
	req := httptest.NewRequest("POST", "/api/v1/workspace/w/update-embeddings", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIWorkspace_Chat_InvalidBody(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
	registerWorkspaceRoutesForTest(env)

	// Invalid JSON body should return 400.
	req := httptest.NewRequest("POST", "/api/v1/workspace/w/chat", bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIWorkspace_StreamChat_InvalidBody(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
	registerWorkspaceRoutesForTest(env)

	req := httptest.NewRequest("POST", "/api/v1/workspace/w/stream-chat", bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
