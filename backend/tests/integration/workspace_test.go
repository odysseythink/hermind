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

func setupWorkspaceRouter(t *testing.T) (*gin.Engine, string) {
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
	wsSvc := services.NewWorkspaceService(db, cfg, nil)
	searchSvc := services.NewSearchService(db)

	authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	loginResp, _ := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db, searchSvc, nil, nil, nil)
	return r, loginResp.Token
}

func TestCreateAndListWorkspace(t *testing.T) {
	r, token := setupWorkspaceRouter(t)

	body, _ := json.Marshal(dto.CreateWorkspaceRequest{Name: "My Workspace"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
