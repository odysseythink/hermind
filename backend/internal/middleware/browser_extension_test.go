package middleware

import (
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
	"gorm.io/gorm"
)

func setupBrowserExtensionMiddleware(t *testing.T, multiUser bool) (*gin.Engine, *gorm.DB, *services.BrowserExtensionService, *services.AuthService, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", MultiUserMode: multiUser}
	db, err := services.NewDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	require.NoError(t, services.AutoMigrate(db))
	extSvc := services.NewBrowserExtensionService(db)
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	r := gin.New()
	return r, db, extSvc, authSvc, cfg
}

func TestValidBrowserExtensionApiKey_ValidKey(t *testing.T) {
	r, _, extSvc, authSvc, cfg := setupBrowserExtensionMiddleware(t, false)
	r.GET("/test", ValidBrowserExtensionApiKey(extSvc, authSvc, cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	key, err := extSvc.CreateKey(t.Context(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestValidBrowserExtensionApiKey_InvalidKey(t *testing.T) {
	r, _, extSvc, authSvc, cfg := setupBrowserExtensionMiddleware(t, false)
	r.GET("/test", ValidBrowserExtensionApiKey(extSvc, authSvc, cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestValidBrowserExtensionApiKey_MissingHeader(t *testing.T) {
	r, _, extSvc, authSvc, cfg := setupBrowserExtensionMiddleware(t, false)
	r.GET("/test", ValidBrowserExtensionApiKey(extSvc, authSvc, cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestValidBrowserExtensionApiKey_SuspendedUser(t *testing.T) {
	r, db, extSvc, authSvc, cfg := setupBrowserExtensionMiddleware(t, true)
	r.GET("/test", ValidBrowserExtensionApiKey(extSvc, authSvc, cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	user := &models.User{Username: utils.Ptr("suspended"), Role: "default", Suspended: 1}
	require.NoError(t, db.Create(user).Error)

	key, err := extSvc.CreateKey(t.Context(), &user.ID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}
