package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAPIAdmin(t *testing.T, cfg *config.Config) (*apiTestEnv, *APIAdminHandler) {
	env := newAPITestEnv(t, cfg)
	adminSvc := services.NewAdminService(env.DB)
	sysSvc := services.NewSystemService(env.DB)
	wsSvc := services.NewWorkspaceService(env.DB, env.Cfg, nil)
	wsChatSvc := services.NewWorkspaceChatService(env.DB)
	api := env.Router.Group("/api")
	RegisterAPIAdminRoutes(api, env.APIKeySvc, adminSvc, sysSvc, wsSvc, wsChatSvc, env.Cfg)
	return env, NewAPIAdminHandler(adminSvc, sysSvc, wsSvc, wsChatSvc, env.Cfg)
}

func TestAPIAdmin_IsMultiUserMode_True(t *testing.T) {
	env, _ := setupAPIAdmin(t, &config.Config{MultiUserMode: true})
	req := httptest.NewRequest("GET", "/api/v1/admin/is-multi-user-mode", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]bool
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body["isMultiUser"])
}

func TestAPIAdmin_IsMultiUserMode_False(t *testing.T) {
	env, _ := setupAPIAdmin(t, &config.Config{MultiUserMode: false})
	req := httptest.NewRequest("GET", "/api/v1/admin/is-multi-user-mode", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]bool
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.False(t, body["isMultiUser"])
}

func TestAPIAdmin_Users_List(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	require.NoError(t, env.DB.Create(&models.User{Username: utils.Ptr("alice"), Role: "admin"}).Error)
	require.NoError(t, env.DB.Create(&models.User{Username: utils.Ptr("bob"), Role: "default"}).Error)

	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Users []map[string]any `json:"users"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Users, 2)
}

func TestAPIAdmin_Users_NewSuccess(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	payload, _ := json.Marshal(map[string]string{"username": "charlie", "password": "secret", "role": "default"})
	req := httptest.NewRequest("POST", "/api/v1/admin/users/new", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotNil(t, body["user"])
	assert.Nil(t, body["error"])
}

func TestAPIAdmin_Users_NewBusinessError(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	// Empty password
	payload, _ := json.Marshal(map[string]string{"username": "charlie", "password": "", "role": "default"})
	req := httptest.NewRequest("POST", "/api/v1/admin/users/new", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Nil(t, body["user"])
	assert.NotNil(t, body["error"])
}

func TestAPIAdmin_Users_UpdateSuccess(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, env.DB.Create(u).Error)

	payload, _ := json.Marshal(map[string]string{"bio": "hello"})
	req := httptest.NewRequest("POST", "/api/v1/admin/users/"+strconv.Itoa(u.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestAPIAdmin_Users_DeleteSuccess(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, env.DB.Create(u).Error)

	req := httptest.NewRequest("DELETE", "/api/v1/admin/users/"+strconv.Itoa(u.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestAPIAdmin_Users_DeniedWithoutMultiUser(t *testing.T) {
	env, _ := setupAPIAdmin(t, &config.Config{MultiUserMode: false})
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIAdmin_Invites_List(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	// No invites yet
	req := httptest.NewRequest("GET", "/api/v1/admin/invites", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Invites []any `json:"invites"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Empty(t, body.Invites)
}

func TestAPIAdmin_Invites_CreateAndDeactivate(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	// Create
	req := httptest.NewRequest("POST", "/api/v1/admin/invite/new", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var createBody struct {
		Invite map[string]any `json:"invite"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createBody))
	require.NotNil(t, createBody.Invite)
	id := int(createBody.Invite["id"].(float64))

	// Deactivate
	req2 := httptest.NewRequest("DELETE", "/api/v1/admin/invite/"+strconv.Itoa(id), nil)
	req2.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec2 := httptest.NewRecorder()
	env.Router.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	var delBody map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &delBody))
	assert.Equal(t, true, delBody["success"])
}

func TestAPIAdmin_WorkspaceUsers_List(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, env.DB.Create(u).Error)
	require.NoError(t, env.DB.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: u.ID, Role: "admin"}).Error)

	req := httptest.NewRequest("GET", "/api/v1/admin/workspaces/"+strconv.Itoa(ws.ID)+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Users []any `json:"users"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Users, 1)
}

func TestAPIAdmin_WorkspaceUsers_Update(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	payload, _ := json.Marshal(map[string][]int{"userIds": {}})
	req := httptest.NewRequest("POST", "/api/v1/admin/workspaces/"+strconv.Itoa(ws.ID)+"/update-users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestAPIAdmin_WorkspaceUsers_Manage(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	payload, _ := json.Marshal(map[string][]int{"userIds": {}})
	req := httptest.NewRequest("POST", "/api/v1/admin/workspaces/ws/manage-users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestAPIAdmin_WorkspaceChats(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	for i := 0; i < 5; i++ {
		require.NoError(t, env.DB.Create(&models.WorkspaceChat{WorkspaceID: ws.ID, Prompt: "hi", Response: "hello", Include: true}).Error)
	}

	payload, _ := json.Marshal(map[string]int{"offset": 0})
	req := httptest.NewRequest("POST", "/api/v1/admin/workspace-chats", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Chats    []any `json:"chats"`
		HasPages bool  `json:"hasPages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Chats, 5)
	assert.False(t, body.HasPages)
}

func TestAPIAdmin_Preferences_SearchProvider_Valid(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	payload, _ := json.Marshal(map[string]string{"agent_search_provider": "serper-dot-dev"})
	req := httptest.NewRequest("POST", "/api/v1/admin/preferences", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestAPIAdmin_Preferences_SearchProvider_Invalid(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	payload, _ := json.Marshal(map[string]string{"agent_search_provider": "not-a-real-provider"})
	req := httptest.NewRequest("POST", "/api/v1/admin/preferences", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, false, body["success"])
	assert.Contains(t, body["error"], "unknown search provider")
}

func TestAPIAdmin_Preferences_SearchProvider_AllKnownKeys(t *testing.T) {
	known := []string{
		"duckduckgo-engine", "brave-search", "serpapi", "searchapi",
		"serper-dot-dev", "bing-search", "baidu-search", "serply-engine",
		"searxng-engine", "tavily-search", "exa-search", "perplexity-search",
		"crw-search",
	}
	for _, provider := range known {
		env, _ := setupAPIAdmin(t, nil)
		payload, _ := json.Marshal(map[string]string{"agent_search_provider": provider})
		req := httptest.NewRequest("POST", "/api/v1/admin/preferences", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+env.APIKey)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.Router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "provider %q should be accepted", provider)
	}
}

func TestAPIAdmin_Preferences(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	payload, _ := json.Marshal(map[string]string{"support_email": "x@y.com"})
	req := httptest.NewRequest("POST", "/api/v1/admin/preferences", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
}

func TestAPIAdmin_WorkspaceUsers_Manage_AutoCreateUser(t *testing.T) {
	env, _ := setupAPIAdmin(t, nil)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)

	// userIds contains a mix of int IDs and {username,password,role} objects.
	payload, _ := json.Marshal(map[string]any{
		"userIds": []any{
			map[string]string{"username": "newuser", "password": "secret123", "role": "default"},
		},
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/workspaces/ws/manage-users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])

	// Verify the user was created and bound to workspace.
	var users []models.User
	require.NoError(t, env.DB.Where("username = ?", "newuser").Find(&users).Error)
	require.Len(t, users, 1)
	var wus []models.WorkspaceUser
	require.NoError(t, env.DB.Where("workspace_id = ? AND user_id = ?", ws.ID, users[0].ID).Find(&wus).Error)
	require.Len(t, wus, 1)
}
