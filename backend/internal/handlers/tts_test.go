package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/internal/tts"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockTTSProvider struct {
	name    string
	avail   bool
	synthFn func(ctx context.Context, text string) (*tts.Synthesis, error)
}

func (m *mockTTSProvider) Synthesize(ctx context.Context, text string) (*tts.Synthesis, error) {
	if m.synthFn != nil {
		return m.synthFn(ctx, text)
	}
	return &tts.Synthesis{Audio: []byte("fake-audio-" + text), ContentType: "audio/mpeg"}, nil
}
func (m *mockTTSProvider) Available() bool { return m.avail }
func (m *mockTTSProvider) Name() string    { return m.name }

func newTTSTestEnv(t *testing.T, cfg *config.Config, mockTTS tts.Provider) (*gin.Engine, *services.ChatService, *services.AuthService, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))
	if cfg == nil {
		cfg = &config.Config{StorageDir: t.TempDir()}
	}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	chatSvc := services.NewChatService(db, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	r := gin.New()
	api := r.Group("/api")
	RegisterTTSRoutes(api, NewTTSHandler(chatSvc, mockTTS), authSvc)
	return r, chatSvc, authSvc, db
}

func TestTTSHandler_HappyPath_200WithAudio(t *testing.T) {
	mock := &mockTTSProvider{name: "mock", avail: true}
	r, _, _, db := newTTSTestEnv(t, nil, mock)

	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)
	chat := models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "hello",
		Response:    `{"text":"hi there","sources":[]}`,
		Include:     true,
		CreatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&chat).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/workspace/test-ws/tts/%d", chat.ID), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), "fake-audio-hi there")
}

func TestTTSHandler_ChatNotFound_404(t *testing.T) {
	mock := &mockTTSProvider{name: "mock", avail: true}
	r, _, _, _ := newTTSTestEnv(t, nil, mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workspace/test-ws/tts/99999", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTTSHandler_NoAssistantText_422(t *testing.T) {
	mock := &mockTTSProvider{name: "mock", avail: true}
	r, _, _, db := newTTSTestEnv(t, nil, mock)

	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)
	chat := models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "hello",
		Response:    ``,
		Include:     true,
		CreatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&chat).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/workspace/test-ws/tts/%d", chat.ID), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestTTSHandler_NativeProvider_ReturnsError(t *testing.T) {
	mock := &mockTTSProvider{
		name:  "native",
		avail: true,
		synthFn: func(ctx context.Context, text string) (*tts.Synthesis, error) {
			return nil, fmt.Errorf("native TTS is handled by the browser")
		},
	}
	r, _, _, db := newTTSTestEnv(t, nil, mock)

	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)
	chat := models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "hello",
		Response:    `{"text":"hi there","sources":[]}`,
		Include:     true,
		CreatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&chat).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/workspace/test-ws/tts/%d", chat.ID), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "native TTS")
}

func TestTTSHandler_RequiresAuth_401(t *testing.T) {
	mock := &mockTTSProvider{name: "mock", avail: true}
	cfg := &config.Config{
		StorageDir:    t.TempDir(),
		AuthToken:     "secret",
		JWTSecret:     "jwt-secret",
		MultiUserMode: true,
	}
	r, _, _, db := newTTSTestEnv(t, cfg, mock)

	ws := &models.Workspace{Name: "Test", Slug: "test-ws"}
	require.NoError(t, db.Create(ws).Error)
	chat := models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "hello",
		Response:    `{"text":"hi","sources":[]}`,
		Include:     true,
		CreatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&chat).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/workspace/test-ws/tts/%d", chat.ID), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
