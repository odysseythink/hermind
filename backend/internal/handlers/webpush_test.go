package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWPHandlerEnv(t *testing.T) (*gin.Engine, *services.WebPushService, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.SystemSetting{}))
	enc, _ := utils.NewEncryptionManager(t.TempDir())
	sysSvc := services.NewSystemService(db)
	wpSvc := services.NewWebPushService(db, sysSvc, enc, services.WebPushOptions{MailTo: "mailto:t@test"})
	require.NoError(t, wpSvc.Init(t.Context()))
	authSvc := services.NewAuthService(db, &config.Config{JWTSecret: "t"}, enc)

	r := gin.New()
	api := r.Group("/api")
	RegisterWebPushRoutes(api, wpSvc, authSvc)
	return r, wpSvc, db
}

func TestWebPush_PubKeyEndpoint(t *testing.T) {
	r, _, _ := newWPHandlerEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/web-push/pubkey", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(t, body["publicKey"])
}

func TestWebPush_SubscribeEndpoint(t *testing.T) {
	r, wpSvc, db := newWPHandlerEnv(t)
	// ValidatedRequest in single-user mode sets user ID=0.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, err = sqlDB.Exec("INSERT INTO users (id, password, role, created_at, last_updated_at) VALUES (0, '', 'admin', datetime('now'), datetime('now'))")
	require.NoError(t, err)

	sub := `{"endpoint":"https://e","keys":{"p256dh":"abc","auth":"def"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/web-push/subscribe", bytes.NewReader([]byte(sub)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	_, ok := wpSvc.HasSubscription(0)
	assert.True(t, ok)
}
