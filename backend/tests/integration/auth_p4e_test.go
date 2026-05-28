package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupAuthP4ERouter(t *testing.T, cfg *config.Config) (*gin.Engine, *services.AuthService, *services.EventLogService, *services.TemporaryAuthTokenService, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	if cfg.StorageDir == "" {
		cfg.StorageDir = t.TempDir()
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "test"
	}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	err = services.AutoMigrate(db)
	assert.NoError(t, err)
	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	assert.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	eventLogSvc := services.NewEventLogService(db)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	r := gin.New()
	handlers.RegisterAuthRoutes(r.Group("/api"), authSvc, cfg, eventLogSvc, tempTokenSvc)
	return r, authSvc, eventLogSvc, tempTokenSvc, db
}

func TestRequestTokenSingleUser(t *testing.T) {
	cfg := &config.Config{AuthToken: "single-user-token", MultiUserMode: false}
	r, _, _, _, _ := setupAuthP4ERouter(t, cfg)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "admin", Password: "admin"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.NotEmpty(t, resp.Token)
}

func TestRequestTokenMultiUserFirstLogin(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, authSvc, _, _, _ := setupAuthP4ERouter(t, cfg)

	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.NotEmpty(t, resp.Token)
	assert.Len(t, resp.RecoveryCodes, 4)
}

func TestRequestTokenMultiUserSecondLogin(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, authSvc, _, _, _ := setupAuthP4ERouter(t, cfg)

	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	// First login to generate recovery codes
	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// Second login should not return recovery codes
	body, _ = json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.NotEmpty(t, resp.Token)
	assert.Empty(t, resp.RecoveryCodes)
}

func TestRequestTokenMultiUserInvalidPassword(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, authSvc, _, _, _ := setupAuthP4ERouter(t, cfg)

	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "wrong"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Valid)
	assert.NotNil(t, resp.Message)
	assert.Contains(t, *resp.Message, "invalid credentials")
}

func TestRequestTokenMultiUserSSOLoginDisabled(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true, SimpleSSOEnabled: true, SimpleSSONoLogin: true}
	r, _, _, _, _ := setupAuthP4ERouter(t, cfg)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "alice", Password: "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Valid)
	assert.NotNil(t, resp.Message)
	assert.Contains(t, *resp.Message, "disabled")
}

func TestSSOSimpleSuccess(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, authSvc, _, tempTokenSvc, _ := setupAuthP4ERouter(t, cfg)

	loginResp, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	userID := loginResp.User.(models.User).ID

	tempToken, err := tempTokenSvc.Issue(nil, userID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/request-token/sso/simple?token="+tempToken, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Valid)
	assert.NotEmpty(t, resp.Token)
	assert.NotNil(t, resp.User)
}

func TestSSOSimpleInvalidToken(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, _, _, _, _ := setupAuthP4ERouter(t, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/request-token/sso/simple?token=invalid-token", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Valid)
	assert.NotNil(t, resp.Message)
}

func TestSSOSimpleSingleUserMode(t *testing.T) {
	cfg := &config.Config{MultiUserMode: false, AuthToken: "token"}
	r, _, _, _, _ := setupAuthP4ERouter(t, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/request-token/sso/simple?token=abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
}

func TestRecoverAccountSingleUserMode(t *testing.T) {
	cfg := &config.Config{MultiUserMode: false, AuthToken: "token"}
	r, _, _, _, _ := setupAuthP4ERouter(t, cfg)

	body, _ := json.Marshal(dto.RecoverAccountRequest{Username: "alice", RecoveryCodes: []string{"a", "b"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/recover-account", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
}

func TestRecoverAccountNoCodesMultiUser(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, authSvc, _, _, _ := setupAuthP4ERouter(t, cfg)

	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RecoverAccountRequest{Username: "alice", RecoveryCodes: []string{"code1", "code2"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/recover-account", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestResetPasswordSingleUserMode(t *testing.T) {
	cfg := &config.Config{MultiUserMode: false, AuthToken: "token"}
	r, _, _, _, _ := setupAuthP4ERouter(t, cfg)

	body, _ := json.Marshal(dto.ResetPasswordRequest{Token: "abc", NewPassword: "new", ConfirmPassword: "new"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/reset-password", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
}

func TestEventLogFailedLoginInvalidUsername(t *testing.T) {
	cfg := &config.Config{MultiUserMode: true}
	r, _, _, _, db := setupAuthP4ERouter(t, cfg)

	body, _ := json.Marshal(dto.RequestTokenMultiUserRequest{Username: "nobody", Password: "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/request-token", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp dto.RequestTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Valid)

	// Verify event log was created
	var logs []models.EventLog
	err := db.Find(&logs).Error
	assert.NoError(t, err)
	found := false
	for _, log := range logs {
		if log.Event == "failed_login_invalid_username" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected failed_login_invalid_username event log")
}
