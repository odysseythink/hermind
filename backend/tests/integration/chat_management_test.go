package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func setupChatMgmtTest(t *testing.T) (*gin.Engine, string, string, int) {
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
	chatSvc := services.NewChatService(db, cfg, nil, nil, nil, nil, nil)

	_, err = authSvc.Register(nil, dto.RegisterRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: "alice", Password: "secret"})
	assert.NoError(t, err)
	user := loginResp.User.(models.User)

	ws, _ := wsSvc.Create(nil, user.ID, dto.CreateWorkspaceRequest{Name: "Test"})

	chat := models.WorkspaceChat{WorkspaceID: ws.ID, UserID: &user.ID, Prompt: "hi", Response: "hello", Include: true}
	db.Create(&chat)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, nil, nil)
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db, searchSvc, nil, nil, nil)
	handlers.RegisterChatRoutes(api, chatSvc, authSvc, db)

	return r, loginResp.Token, ws.Slug, chat.ID
}

func TestDeleteWorkspaceChats(t *testing.T) {
	r, token, slug, _ := setupChatMgmtTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/workspace/"+slug+"/delete-chats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestUpdateChatFeedback(t *testing.T) {
	r, token, slug, chatID := setupChatMgmtTest(t)
	score := true
	body, _ := json.Marshal(map[string]any{"feedbackScore": score})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/"+slug+"/chat-feedback/"+strconv.Itoa(chatID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
