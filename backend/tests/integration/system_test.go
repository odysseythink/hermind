package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir()}
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
	sysSvc := services.NewSystemService(db)
	apiKeySvc := services.NewAPIKeyService(db)
	authSvc := services.NewAuthService(db, cfg, nil)

	r := gin.New()
	adminSvc := services.NewAdminService(db)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ping", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}
