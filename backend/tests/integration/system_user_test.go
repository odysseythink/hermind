package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupSystemUserRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test-secret", AuthToken: "test-auth-token", MultiUserMode: false}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, _ := db.DB(); sqlDB != nil {
			sqlDB.Close()
		}
	})
	assert.NoError(t, services.AutoMigrate(db))
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	sysSvc := services.NewSystemService(db)
	apiKeySvc := services.NewAPIKeyService(db)
	adminSvc := services.NewAdminService(db)
	r := gin.New()
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, nil, nil, nil)
	return r, db, authSvc, cfg
}

func seedMultiUserAdmin(t *testing.T, db *gorm.DB, cfg *config.Config, role string, suspended int) (*models.User, string) {
	t.Helper()
	hash, _ := utils.HashPassword("pw")
	u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: role, Suspended: suspended, Bio: utils.Ptr("")}
	assert.NoError(t, db.Create(u).Error)
	tok, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, time.Hour)
	assert.NoError(t, err)
	return u, tok
}

func singleUserBearer(t *testing.T, authSvc *services.AuthService) string {
	t.Helper()
	tok, err := authSvc.CreateSingleUserToken(context.Background())
	assert.NoError(t, err)
	return tok
}

func TestSystem_CheckToken_SingleUser_OK(t *testing.T) {
	r, _, authSvc, _ := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	req := httptest.NewRequest(http.MethodGet, "/api/system/check-token", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestSystem_CheckToken_MultiUser_Suspended_403(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 1)
	req := httptest.NewRequest(http.MethodGet, "/api/system/check-token", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSystem_CheckToken_MultiUser_OK(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
	req := httptest.NewRequest(http.MethodGet, "/api/system/check-token", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSystem_RefreshUser_SingleUser(t *testing.T) {
	r, _, authSvc, _ := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	req := httptest.NewRequest(http.MethodGet, "/api/system/refresh-user", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
	assert.Nil(t, body["user"])
	assert.Nil(t, body["message"])
}

func TestSystem_RefreshUser_MultiUser_OK(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
	req := httptest.NewRequest(http.MethodGet, "/api/system/refresh-user", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
	user, ok := body["user"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "alice", user["username"])
	_, hasPw := user["password"]
	assert.False(t, hasPw, "password must be stripped")
}

func TestSystem_RefreshUser_MultiUser_Suspended(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 1)
	req := httptest.NewRequest(http.MethodGet, "/api/system/refresh-user", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// suspended users still pass ValidatedRequest (which doesn't check suspended).
	// refresh-user is where the suspended check lives.
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["success"])
	assert.Equal(t, "User is suspended.", body["message"])
}

func TestSystem_UpdatePassword_MultiUserMode_Rejects(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "admin", 0)
	body := []byte(`{"usePassword": true, "newPassword": "newPassw0rd"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSystem_UpdatePassword_SingleUser_Sets(t *testing.T) {
	r, _, authSvc, cfg := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	body := []byte(`{"usePassword": true, "newPassword": "newPassw0rd"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Equal(t, "newPassw0rd", cfg.AuthToken)
	assert.NotEqual(t, "test-secret", cfg.JWTSecret)
}

func TestSystem_UpdatePassword_Disable(t *testing.T) {
	r, _, authSvc, cfg := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	body := []byte(`{"usePassword": false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", cfg.AuthToken)
	assert.Equal(t, "", cfg.JWTSecret)
}

func TestSystem_UpdatePassword_ShortPassword(t *testing.T) {
	r, _, authSvc, _ := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	body := []byte(`{"usePassword": true, "newPassword": "short"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.NotEmpty(t, resp["error"])
}

func TestSystem_EnableMultiUser_Success(t *testing.T) {
	r, _, authSvc, cfg := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	body := []byte(`{"username":"newadmin","password":"newPassw0rd"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/enable-multi-user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.True(t, cfg.MultiUserMode)
}

func TestSystem_EnableMultiUser_AlreadyOn(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "admin", 0)
	body := []byte(`{"username":"newadmin","password":"newPassw0rd"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/enable-multi-user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "Multi-user mode is already enabled.", resp["error"])
}

func TestSystem_EnableMultiUser_BadInput(t *testing.T) {
	r, _, authSvc, _ := setupSystemUserRouter(t)
	tok := singleUserBearer(t, authSvc)
	body := []byte(`{"username":"A","password":"newPassw0rd"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/enable-multi-user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Node returns 400 in the create-fail branch; matching that.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSystem_UpdateOwnUser_Bio(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
	body := []byte(`{"username":"alice","bio":"hi there"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	var reloaded models.User
	db.Where("username = ?", "alice").First(&reloaded)
	assert.Equal(t, "hi there", *reloaded.Bio)
}

func TestSystem_UpdateOwnUser_UsernameChange(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
	body := []byte(`{"username":"alice2"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var reloaded models.User
	db.Where("username = ?", "alice2").First(&reloaded)
	assert.NotZero(t, reloaded.ID)
}

func TestSystem_UpdateOwnUser_NoUpdates(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
	body := []byte(`{"username":"alice"}`) // same as current — no diff
	req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "No updates provided", resp["error"])
}

func TestSystem_UpdateOwnUser_InvalidUsername(t *testing.T) {
	r, db, _, cfg := setupSystemUserRouter(t)
	cfg.MultiUserMode = true
	_, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
	body := []byte(`{"username":"A"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.NotEmpty(t, resp["error"])
}

// json import will be used by later tests
var _ = json.Marshal
