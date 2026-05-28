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

func setupAuthRouter(t *testing.T) (*gin.Engine, *config.Config) {
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

	r := gin.New()
	handlers.RegisterAuthRoutes(r.Group("/api"), authSvc, cfg, nil, nil)
	return r, cfg
}

func TestRegisterAndLogin(t *testing.T) {
	r, _ := setupAuthRouter(t)

	body, _ := json.Marshal(dto.RegisterRequest{Username: "alice", Password: "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var loginResp dto.LoginResponse
	body, _ = json.Marshal(dto.LoginRequest{Username: "alice", Password: "secret"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/login", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	assert.NotEmpty(t, loginResp.Token)
}
