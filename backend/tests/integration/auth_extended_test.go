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

func setupAuthExtendedRouter(t *testing.T) (*gin.Engine, *services.AuthService, *services.AdminService) {
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
	adminSvc := services.NewAdminService(db)
	sysSvc := services.NewSystemService(db)
	wsSvc := services.NewWorkspaceService(db, cfg)
	apiKeySvc := services.NewAPIKeyService(db)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, nil, nil)
	handlers.RegisterAdminRoutes(api, adminSvc, sysSvc, wsSvc, apiKeySvc, authSvc)

	return r, authSvc, adminSvc
}

func TestInviteFlow(t *testing.T) {
	r, _, adminSvc := setupAuthExtendedRouter(t)

	invite, err := adminSvc.CreateInvite(nil, 1, []int{})
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/invite/"+invite.Code, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	body, _ := json.Marshal(dto.AcceptInviteRequest{Username: "bob", Password: "secret"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/invite/"+invite.Code, bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])
}

func TestRecoverAccountNoCodes(t *testing.T) {
	r, authSvc, _ := setupAuthExtendedRouter(t)
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	body, _ := json.Marshal(dto.RecoverAccountRequest{Username: "alice", RecoveryCodes: []string{"code1", "code2"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/recover-account", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}
