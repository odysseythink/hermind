package middleware

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
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWSAuthTestEnv(t *testing.T, cfg *config.Config) (*gin.Engine, *services.AuthService, *services.TemporaryAuthTokenService, *gorm.DB, *models.User) {
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
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)

	u := &models.User{Username: utils.Ptr("alice"), Role: "default"}
	require.NoError(t, db.Create(u).Error)

	r := gin.New()
	return r, authSvc, tempTokenSvc, db, u
}

func TestWSValidatedRequest_NoToken_401(t *testing.T) {
	r, authSvc, tempTokenSvc, _, _ := newWSAuthTestEnv(t, nil)
	r.GET("/probe", WSValidatedRequest(authSvc, tempTokenSvc), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/probe", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWSValidatedRequest_InvalidToken_403(t *testing.T) {
	cfg := &config.Config{StorageDir: t.TempDir(), AuthToken: "secret", JWTSecret: "jwt-secret"}
	r, authSvc, tempTokenSvc, _, _ := newWSAuthTestEnv(t, cfg)
	r.GET("/probe", WSValidatedRequest(authSvc, tempTokenSvc), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/probe?token=bogus", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestWSValidatedRequest_ValidToken_SetsUser(t *testing.T) {
	cfg := &config.Config{StorageDir: t.TempDir(), AuthToken: "secret", JWTSecret: "jwt-secret"}
	r, authSvc, tempTokenSvc, _, u := newWSAuthTestEnv(t, cfg)
	r.GET("/probe", WSValidatedRequest(authSvc, tempTokenSvc), func(c *gin.Context) {
		user := c.MustGet("user").(*models.User)
		c.JSON(200, gin.H{"id": user.ID})
	})

	tok, err := tempTokenSvc.IssueWithTTL(context.Background(), u.ID, time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/probe?token=%s", tok), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), fmt.Sprintf(`"id":%d`, u.ID))
}

func TestWSValidatedRequest_TokenIsSingleUse(t *testing.T) {
	cfg := &config.Config{StorageDir: t.TempDir(), AuthToken: "secret", JWTSecret: "jwt-secret"}
	r, authSvc, tempTokenSvc, _, u := newWSAuthTestEnv(t, cfg)
	r.GET("/probe", WSValidatedRequest(authSvc, tempTokenSvc), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	tok, err := tempTokenSvc.IssueWithTTL(context.Background(), u.ID, time.Hour)
	require.NoError(t, err)

	// First use succeeds
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", fmt.Sprintf("/probe?token=%s", tok), nil)
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Second use fails (token consumed)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", fmt.Sprintf("/probe?token=%s", tok), nil)
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusForbidden, w2.Code)
}

func TestWSValidatedRequest_AuthDisabled_AcceptsBypassSentinel(t *testing.T) {
	r, authSvc, tempTokenSvc, _, _ := newWSAuthTestEnv(t, nil)
	r.GET("/probe", WSValidatedRequest(authSvc, tempTokenSvc), func(c *gin.Context) {
		user := c.MustGet("user").(*models.User)
		c.JSON(200, gin.H{"id": user.ID, "role": user.Role})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/probe?token=%s", AuthDisabledBypassToken), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"id":0`)
	require.Contains(t, w.Body.String(), `"role":"admin"`)
}
