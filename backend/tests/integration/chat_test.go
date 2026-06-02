package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// mockLLM is a test LLM provider that returns predefined chunks.
type mockLLM struct {
	chunks []providers.LLMChunk
}

func (m *mockLLM) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan providers.LLMChunk, error) {
	out := make(chan providers.LLMChunk, len(m.chunks))
	for _, c := range m.chunks {
		out <- c
	}
	close(out)
	return out, nil
}

func (m *mockLLM) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	var result strings.Builder
	for _, c := range m.chunks {
		result.WriteString(c.TextDelta)
	}
	return result.String(), nil
}
func (m *mockLLM) LanguageModel() core.LanguageModel { return nil }

func setupChatTest(t *testing.T, llm providers.LLMProvider) (*gin.Engine, *services.AuthService, *services.WorkspaceService, *gorm.DB) {
	cfg := &config.Config{
		StorageDir:    t.TempDir(),
		JWTSecret:     "test-secret",
		MultiUserMode: true,
	}
	db, err := services.NewDB(cfg)
	require.NoError(t, err)
	err = services.AutoMigrate(db)
	require.NoError(t, err)

	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	require.NoError(t, err)
	authSvc := services.NewAuthService(db, cfg, enc)
	wsSvc := services.NewWorkspaceService(db, cfg, nil)
	searchSvc := services.NewSearchService(db)
	vectorSvc := services.NewVectorService(cfg)
	chatSvc := services.NewChatService(db, cfg, vectorSvc, llm, nil, nil, nil, nil, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, nil, nil)
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db, searchSvc, nil, nil, nil)
	handlers.RegisterChatRoutes(api, chatSvc, authSvc, db)

	return r, authSvc, wsSvc, db
}

func registerUser(t *testing.T, r *gin.Engine, username, password string) {
	body, _ := json.Marshal(dto.RegisterRequest{Username: username, Password: password})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func loginUser(t *testing.T, r *gin.Engine, username, password string) string {
	body, _ := json.Marshal(dto.LoginRequest{Username: username, Password: password})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp dto.LoginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token
}

func createWorkspace(t *testing.T, r *gin.Engine, token string) *models.Workspace {
	body, _ := json.Marshal(dto.CreateWorkspaceRequest{Name: "Test WS"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp struct {
		Workspace models.Workspace `json:"workspace"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return &resp.Workspace
}

func TestChatStreamWithMockLLM(t *testing.T) {
	llm := &mockLLM{
		chunks: []providers.LLMChunk{
			{TextDelta: "Hello"},
			{TextDelta: " world"},
			{FinishReason: "stop"},
		},
	}
	r, authSvc, _, db := setupChatTest(t, llm)
	_ = authSvc

	registerUser(t, r, "alice", "secret")
	token := loginUser(t, r, "alice", "secret")
	ws := createWorkspace(t, r, token)

	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/workspace/%s/stream-chat", ws.Slug), strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Hello")
	assert.Contains(t, body, "world")
}

func TestChatStreamAbortOnError(t *testing.T) {
	llm := &mockLLM{
		chunks: []providers.LLMChunk{
			{Err: fmt.Errorf("mock llm error")},
		},
	}
	r, authSvc, _, db := setupChatTest(t, llm)
	_ = authSvc

	registerUser(t, r, "bob", "secret")
	token := loginUser(t, r, "bob", "secret")
	ws := createWorkspace(t, r, token)

	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/workspace/%s/stream-chat", ws.Slug), strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "abort")
}
