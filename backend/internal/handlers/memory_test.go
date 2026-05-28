package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMemHandlerEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Memory{}, &models.Workspace{}, &models.User{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	memSvc := services.NewMemoryService(db)
	wsSvc := services.NewWorkspaceService(db, &config.Config{}, nil)
	authSvc := services.NewAuthService(db, &config.Config{JWTSecret: "t"}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	RegisterMemoryRoutes(api, memSvc, wsSvc, authSvc)
	return r, db
}

func TestMemory_CreateListDelete(t *testing.T) {
	r, db := newMemHandlerEnv(t)
	ws := models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, db.Create(&ws).Error)

	// Create
	body, _ := json.Marshal(map[string]any{"scope": "workspace", "workspaceId": ws.ID, "content": "fact"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/memory", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created struct{ Memory models.Memory }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// List for slug
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memory/workspace/w", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/memory/"+strconv.Itoa(created.Memory.ID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMemory_LimitReached(t *testing.T) {
	r, db := newMemHandlerEnv(t)
	ws := models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, db.Create(&ws).Error)

	for i := 0; i < models.GlobalMemoryLimit+1; i++ {
		body, _ := json.Marshal(map[string]any{"scope": "global", "content": "fact"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/memory", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i < models.GlobalMemoryLimit {
			require.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusConflict, w.Code)
		}
	}
}
