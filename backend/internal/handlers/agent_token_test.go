package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAgentTokenTestEnv(t *testing.T, cfg *config.Config) (*gin.Engine, *services.TemporaryAuthTokenService, *services.AuthService, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))
	if cfg == nil {
		cfg = &config.Config{StorageDir: t.TempDir()}
	}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	r := gin.New()
	api := r.Group("/api")
	RegisterAgentTokenRoutes(api, tempTokenSvc, authSvc)
	return r, tempTokenSvc, authSvc, db
}

func TestAgentTokenHandler_HappyPath_200WithToken(t *testing.T) {
	r, _, _, db := newAgentTokenTestEnv(t, nil)
	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/workspace/%s/agent-token", ws.Slug), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Success          bool   `json:"success"`
		Token            string `json:"token"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.True(t, body.Success)
	require.NotEmpty(t, body.Token)
	require.Equal(t, middleware.AuthDisabledBypassToken, body.Token)
	require.Equal(t, 180, body.ExpiresInSeconds)
}

func TestAgentTokenHandler_AuthEnabled_Unauthenticated_401(t *testing.T) {
	cfg := &config.Config{
		StorageDir:    t.TempDir(),
		AuthToken:     "secret",
		JWTSecret:     "jwt-secret",
		MultiUserMode: true,
	}
	r, _, _, db := newAgentTokenTestEnv(t, cfg)
	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/workspace/%s/agent-token", ws.Slug), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentTokenHandler_AuthEnabled_Authenticated_200WithRealToken(t *testing.T) {
	cfg := &config.Config{
		StorageDir:    t.TempDir(),
		AuthToken:     "secret",
		JWTSecret:     "jwt-secret",
		MultiUserMode: true,
	}
	r, _, _, db := newAgentTokenTestEnv(t, cfg)
	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)

	u := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, db.Create(u).Error)
	token, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, 24*time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/workspace/%s/agent-token", ws.Slug), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Success          bool   `json:"success"`
		Token            string `json:"token"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.True(t, body.Success)
	require.NotEmpty(t, body.Token)
	require.NotEqual(t, middleware.AuthDisabledBypassToken, body.Token)
	require.Equal(t, 180, body.ExpiresInSeconds)
}
