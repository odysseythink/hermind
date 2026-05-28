package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWhitelistHandlerTestEnv(t *testing.T, cfg *config.Config) (*gin.Engine, *services.AgentSkillWhitelistService, *services.AuthService, *gorm.DB) {
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
	sysSvc := services.NewSystemService(db)
	whitelistSvc := services.NewAgentSkillWhitelistService(sysSvc)

	r := gin.New()
	api := r.Group("/api")
	RegisterAgentSkillWhitelistRoutes(api, NewAgentSkillWhitelistHandler(whitelistSvc), authSvc)
	return r, whitelistSvc, authSvc, db
}

func TestWhitelistHandler_GET_ReturnsCallerList(t *testing.T) {
	r, svc, _, _ := newWhitelistHandlerTestEnv(t, nil)
	require.NoError(t, svc.Add(nil, nil, "rag-memory"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/agent-skill-whitelist", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Skills []string `json:"skills"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Equal(t, []string{"rag-memory"}, body.Skills)
}

func TestWhitelistHandler_POST_AddsSkill(t *testing.T) {
	r, svc, _, _ := newWhitelistHandlerTestEnv(t, nil)

	w := httptest.NewRecorder()
	payload, _ := json.Marshal(map[string]string{"skill": "web-scraping"})
	req, _ := http.NewRequest("POST", "/api/agent-skill-whitelist", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	list, _ := svc.Get(nil, nil)
	require.Equal(t, []string{"web-scraping"}, list)
}

func TestWhitelistHandler_DELETE_RemovesSkill(t *testing.T) {
	r, svc, _, _ := newWhitelistHandlerTestEnv(t, nil)
	require.NoError(t, svc.Add(nil, nil, "keep"))
	require.NoError(t, svc.Add(nil, nil, "remove"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/agent-skill-whitelist/remove", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	list, _ := svc.Get(nil, nil)
	require.Equal(t, []string{"keep"}, list)
}

func TestWhitelistHandler_Admin_GetForUser_NonAdmin_Forbidden(t *testing.T) {
	cfg := &config.Config{
		StorageDir:    t.TempDir(),
		AuthToken:     "secret",
		JWTSecret:     "jwt-secret",
		MultiUserMode: true,
	}
	r, _, _, db := newWhitelistHandlerTestEnv(t, cfg)
	u := &models.User{Username: utils.Ptr("alice"), Role: "default"}
	require.NoError(t, db.Create(u).Error)
	token, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, 24*time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/admin/agent-skill-whitelist/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestWhitelistHandler_Admin_GetForUser_Admin_OK(t *testing.T) {
	cfg := &config.Config{
		StorageDir:    t.TempDir(),
		AuthToken:     "secret",
		JWTSecret:     "jwt-secret",
		MultiUserMode: true,
	}
	r, svc, _, db := newWhitelistHandlerTestEnv(t, cfg)
	target := &models.User{Username: utils.Ptr("bob")}
	require.NoError(t, db.Create(target).Error)
	require.NoError(t, svc.Add(nil, &target.ID, "mcp-tool"))

	admin := &models.User{Username: utils.Ptr("alice"), Role: "admin"}
	require.NoError(t, db.Create(admin).Error)
	token, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": admin.ID}, 24*time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/agent-skill-whitelist/%d", target.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Skills []string `json:"skills"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Equal(t, []string{"mcp-tool"}, body.Skills)
}
