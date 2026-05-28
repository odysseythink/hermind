package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/chunker"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockVectorDB struct{}

func (m *mockVectorDB) Name() string                      { return "mock" }
func (m *mockVectorDB) Connect(ctx context.Context) error { return nil }
func (m *mockVectorDB) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"status": "ok"}, nil
}
func (m *mockVectorDB) Tables(ctx context.Context) ([]string, error)    { return nil, nil }
func (m *mockVectorDB) TotalVectors(ctx context.Context) (int64, error) { return 0, nil }
func (m *mockVectorDB) AddVectors(ctx context.Context, namespace string, chunks []vectordb.VectorChunk) error {
	return nil
}
func (m *mockVectorDB) DeleteVectors(ctx context.Context, namespace string, docIds []string) error {
	return nil
}
func (m *mockVectorDB) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts vectordb.SearchOptions) ([]vectordb.SearchResult, error) {
	return nil, nil
}
func (m *mockVectorDB) DeleteNamespace(ctx context.Context, namespace string) error { return nil }
func (m *mockVectorDB) CountVectors(ctx context.Context, namespace string) (int64, error) {
	return 0, nil
}

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	vec := make([]float32, 3)
	return [][]float32{vec}, nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return make([]float32, 3), nil
}

func (m *mockEmbedder) Dimensions() int { return 3 }

func setupTestDB(t *testing.T) (*gin.Engine, *services.AuthService, *gorm.DB) {
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
	mockVDB := &mockVectorDB{}
	vectorSvc.SetProvider(mockVDB)
	emb := &mockEmbedder{}
	vectorSearchSvc := services.NewVectorSearchService(vectorSvc, emb, nil)

	llm := &mockLLM{
		chunks: []providers.LLMChunk{
			{TextDelta: "Hello from mock LLM"},
			{FinishReason: "stop"},
		},
	}
	chatSvc := services.NewChatService(db, cfg, vectorSvc, llm, nil, nil, nil, nil)

	ch := chunker.NewChunker(1000, 20, "")
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	docSvc := services.NewDocumentService(db, cfg, nil, emb, ch, mockVDB, fsSvc)
	progressMgr := services.NewEmbeddingProgressManager()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterAuthRoutes(api, authSvc, cfg, nil, nil)
	handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db, searchSvc, vectorSearchSvc, docSvc, progressMgr)
	handlers.RegisterChatRoutes(api, chatSvc, authSvc, db)
	handlers.RegisterDocumentRoutes(api, docSvc, authSvc)

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})

	return r, authSvc, db
}

func createTestUser(t *testing.T, authSvc *services.AuthService, username, password string) (string, *models.User) {
	_, err := authSvc.Register(nil, dto.RegisterRequest{Username: username, Password: password})
	require.NoError(t, err)
	loginResp, err := authSvc.Login(nil, dto.LoginRequest{Username: username, Password: password})
	require.NoError(t, err)
	return loginResp.Token, nil
}

func createTestWorkspace(t *testing.T, router *gin.Engine, authSvc *services.AuthService, name string) *models.Workspace {
	token, _ := createTestUser(t, authSvc, name+"-owner", "password")
	body, _ := json.Marshal(dto.CreateWorkspaceRequest{Name: name})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Workspace models.Workspace `json:"workspace"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return &resp.Workspace
}
