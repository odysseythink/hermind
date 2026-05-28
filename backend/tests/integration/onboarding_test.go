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
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupOnboardingRouter(t *testing.T) (*gin.Engine, *services.AuthService) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: true}
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
	sysSvc := services.NewSystemService(db)
	apiKeySvc := services.NewAPIKeyService(db)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, nil, nil)
	adminSvc := services.NewAdminService(db)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	handlers.RegisterSystemRoutes(api, sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, nil, nil, nil)

	return r, authSvc
}

func TestOnboardingFlow(t *testing.T) {
	r, authSvc := setupOnboardingRouter(t)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	token := loginResp.Token

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/onboarding", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var getResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &getResp)
	assert.Equal(t, false, getResp["onboardingComplete"])

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/onboarding", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/onboarding", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	json.Unmarshal(w.Body.Bytes(), &getResp)
	assert.Equal(t, true, getResp["onboardingComplete"])
}
