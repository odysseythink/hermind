package handlers

import (
	"context"
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

func newPHHandlerTestEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.PromptHistory{}, &models.User{}))
	cfg := &config.Config{JWTSecret: "test"}
	phSvc := services.NewPromptHistoryService(db)
	wsSvc := services.NewWorkspaceService(db, cfg, phSvc)
	authSvc := services.NewAuthService(db, cfg, nil)

	r := gin.New()
	// Inject a fake user so middleware.ValidatedRequest sees an authenticated context.
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	RegisterPromptHistoryRoutes(api, phSvc, wsSvc, authSvc, db)
	return r, db
}

func TestPromptHistoryEndpoints_RoundTrip(t *testing.T) {
	r, db := newPHHandlerTestEnv(t)

	ws := models.Workspace{Name: "x", Slug: "x"}
	require.NoError(t, db.Create(&ws).Error)
	require.NoError(t, db.Create(&models.PromptHistory{WorkspaceID: ws.ID, Prompt: "p1"}).Error)
	require.NoError(t, db.Create(&models.PromptHistory{WorkspaceID: ws.ID, Prompt: "p2"}).Error)

	// LIST
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workspace/x/prompt-history", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var listBody struct{ History []models.PromptHistory }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listBody))
	assert.Len(t, listBody.History, 2)

	// DELETE ONE
	firstID := listBody.History[0].ID
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/workspace/x/prompt-history/"+strconv.Itoa(firstID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// DELETE ALL
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/workspace/x/prompt-history", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	rows, _ := services.NewPromptHistoryService(db).ListByWorkspace(context.Background(), ws.ID, 10)
	assert.Len(t, rows, 0)
}
