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

func registerThreadRoutesForTest(env *apiTestEnv) {
	threadSvc := services.NewThreadService(env.DB)
	api := env.Router.Group("/api")
	RegisterAPIThreadRoutes(api, env.APIKeySvc, threadSvc, nil, env.DB)
}

func TestAPIThread_Create(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	registerThreadRoutesForTest(env)

	payload, _ := json.Marshal(map[string]string{"name": "my-thread"})
	req := httptest.NewRequest("POST", "/api/v1/workspace/ws/thread/new", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Thread  *models.WorkspaceThread `json:"thread"`
		Message string                  `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Thread)
	assert.Equal(t, "my-thread", body.Thread.Name)
}

func TestAPIThread_Update(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	thread := &models.WorkspaceThread{WorkspaceID: ws.ID, Name: "old", Slug: "old"}
	require.NoError(t, env.DB.Create(thread).Error)
	registerThreadRoutesForTest(env)

	payload, _ := json.Marshal(map[string]string{"name": "renamed"})
	req := httptest.NewRequest("POST", "/api/v1/workspace/ws/thread/old/update", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got models.WorkspaceThread
	require.NoError(t, env.DB.Where("slug=?", "old").First(&got).Error)
	assert.Equal(t, "renamed", got.Name)
}

func TestAPIThread_Delete(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	thread := &models.WorkspaceThread{WorkspaceID: ws.ID, Name: "t", Slug: "t"}
	require.NoError(t, env.DB.Create(thread).Error)
	registerThreadRoutesForTest(env)

	req := httptest.NewRequest("DELETE", "/api/v1/workspace/ws/thread/t", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var count int64
	env.DB.Model(&models.WorkspaceThread{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAPIThread_GetChats(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	thread := &models.WorkspaceThread{WorkspaceID: ws.ID, Name: "t", Slug: "t"}
	require.NoError(t, env.DB.Create(thread).Error)
	require.NoError(t, env.DB.Create(&models.WorkspaceChat{WorkspaceID: ws.ID, ThreadID: &thread.ID, Prompt: "hi", Response: "hello"}).Error)
	registerThreadRoutesForTest(env)

	req := httptest.NewRequest("GET", "/api/v1/workspace/ws/thread/t/chats", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		History []map[string]any `json:"history"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	// Each WorkspaceChat produces 2 history entries (user + assistant).
	require.Len(t, body.History, 2)
}

func TestAPIThread_Chat_InvalidBody(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	thread := &models.WorkspaceThread{WorkspaceID: ws.ID, Name: "t", Slug: "t"}
	require.NoError(t, env.DB.Create(thread).Error)
	registerThreadRoutesForTest(env)

	req := httptest.NewRequest("POST", "/api/v1/workspace/ws/thread/t/chat", bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIThread_StreamChat_InvalidBody(t *testing.T) {
	env := newAPITestEnv(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	thread := &models.WorkspaceThread{WorkspaceID: ws.ID, Name: "t", Slug: "t"}
	require.NoError(t, env.DB.Create(thread).Error)
	registerThreadRoutesForTest(env)

	req := httptest.NewRequest("POST", "/api/v1/workspace/ws/thread/t/stream-chat", bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
