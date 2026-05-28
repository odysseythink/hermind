package integration

import (
	"bytes"
	"context"
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

func setupEmbedIntegrationRouter(t *testing.T) (*gin.Engine, *services.EmbedService, *services.AuthService, *gorm.DB) {
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
	assert.NoError(t, services.AutoMigrate(db))
	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	assert.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	embedSvc := services.NewEmbedService(db, cfg, nil, nil, nil, nil)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterEmbedRoutes(api, embedSvc, db)
	handlers.RegisterEmbedManagementRoutes(api, embedSvc, authSvc, db)
	return r, embedSvc, authSvc, db
}

func TestEmbedIntegration_FullFlow(t *testing.T) {
	r, _, authSvc, db := setupEmbedIntegrationRouter(t)
	ctx := context.Background()

	// Create admin user
	_, _ = authSvc.Register(ctx, dto.RegisterRequest{Username: "admin", Password: "admin123"})
	loginResp, _ := authSvc.Login(ctx, dto.LoginRequest{Username: "admin", Password: "admin123"})
	token := loginResp.Token

	// Seed workspace
	ws := models.Workspace{Name: "Test", Slug: "test"}
	db.Create(&ws)

	// 1. Create embed via admin
	body, _ := json.Marshal(dto.CreateEmbedConfigRequest{WorkspaceSlug: "test"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/embeds/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var createResp struct{ Embed models.EmbedConfig }
	json.Unmarshal(w.Body.Bytes(), &createResp)
	assert.NotEmpty(t, createResp.Embed.UUID)

	// 2. Get session history (empty)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/embed/"+createResp.Embed.UUID+"/sess-1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var histResp dto.EmbedHistoryResponse
	json.Unmarshal(w.Body.Bytes(), &histResp)
	assert.Empty(t, histResp.History)

	// 3. Delete session
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/embed/"+createResp.Embed.UUID+"/sess-1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// 4. List embeds via admin
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/embeds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var listResp dto.EmbedConfigListResponse
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.Len(t, listResp.Embeds, 1)
}
