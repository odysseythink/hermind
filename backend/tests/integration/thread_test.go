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
)

func setupThreadTest(t *testing.T) (*gin.Engine, *services.AuthService, string, string) {
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
	threadSvc := services.NewThreadService(db)

	_, err = authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)

	ws, err := wsSvc.Create(nil, loginResp.User.(models.User).ID, dto.CreateWorkspaceRequest{Name: "Test Workspace"})
	assert.NoError(t, err)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, nil, nil)
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db, searchSvc, nil, nil, nil)
	handlers.RegisterThreadRoutes(api, threadSvc, authSvc, db)

	return r, authSvc, loginResp.Token, ws.Slug
}

func TestThreadCreateAndList(t *testing.T) {
	r, _, token, slug := setupThreadTest(t)

	body, _ := json.Marshal(dto.CreateThreadRequest{Name: "My Thread"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/"+slug+"/thread/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	assert.NotNil(t, createResp["thread"])

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/"+slug+"/threads", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
