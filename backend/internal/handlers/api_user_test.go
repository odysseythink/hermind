package handlers

import (
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

func TestAPIUsers_Lists(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.User{Username: utils.Ptr("alice"), Role: "admin"}).Error)
	require.NoError(t, env.DB.Create(&models.User{Username: utils.Ptr("bob"), Role: "default"}).Error)

	adminSvc := services.NewAdminService(env.DB)
	tempSvc := services.NewTemporaryAuthTokenService(env.DB)
	api := env.Router.Group("/api")
	RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Users []map[string]any `json:"users"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Users, 2)
	for _, u := range body.Users {
		_, hasPassword := u["password"]
		assert.False(t, hasPassword)
		_, hasUsername := u["username"]
		assert.True(t, hasUsername)
		_, hasRole := u["role"]
		assert.True(t, hasRole)
	}
}

func TestAPIUsers_DeniedWhenNotMultiUser(t *testing.T) {
	env := newAPITestEnv(t, &config.Config{MultiUserMode: false})
	adminSvc := services.NewAdminService(env.DB)
	tempSvc := services.NewTemporaryAuthTokenService(env.DB)
	api := env.Router.Group("/api")
	RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIUsers_IssueAuthToken(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true, SimpleSSOEnabled: true}
	env := newAPITestEnv(t, cfg)
	u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, env.DB.Create(u).Error)

	adminSvc := services.NewAdminService(env.DB)
	tempSvc := services.NewTemporaryAuthTokenService(env.DB)
	api := env.Router.Group("/api")
	RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/users/"+strconv.Itoa(u.ID)+"/issue-auth-token", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Token     string `json:"token"`
		LoginPath string `json:"loginPath"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotEmpty(t, body.Token)
	assert.Contains(t, body.LoginPath, "/sso/simple?token=")
}

func TestAPIUsers_IssueAuthToken_RequiresSimpleSSO(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true, SimpleSSOEnabled: false}
	env := newAPITestEnv(t, cfg)
	u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, env.DB.Create(u).Error)

	adminSvc := services.NewAdminService(env.DB)
	tempSvc := services.NewTemporaryAuthTokenService(env.DB)
	api := env.Router.Group("/api")
	RegisterAPIUserRoutes(api, env.APIKeySvc, adminSvc, tempSvc, env.Cfg)

	req := httptest.NewRequest("GET", "/api/v1/users/"+strconv.Itoa(u.ID)+"/issue-auth-token", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
