package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupEmbedMiddleware(t *testing.T) (*gin.Engine, *gorm.DB, *services.EmbedService) {
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
	assert.NoError(t, services.AutoMigrate(db))
	embedSvc := services.NewEmbedService(db, cfg, nil, nil, nil, nil)

	r := gin.New()
	return r, db, embedSvc
}

func TestValidEmbedConfig(t *testing.T) {
	r, db, _ := setupEmbedMiddleware(t)
	r.GET("/embed/:embedId", ValidEmbedConfig(db), func(c *gin.Context) {
		embed := c.MustGet("embedConfig").(*models.EmbedConfig)
		c.JSON(200, gin.H{"id": embed.ID})
	})

	ws := models.Workspace{Name: "W", Slug: "w"}
	db.Create(&ws)
	embed := models.EmbedConfig{UUID: "abc-123", WorkspaceID: ws.ID, Enabled: true}
	db.Create(&embed)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/embed/abc-123", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/embed/notfound", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
}
