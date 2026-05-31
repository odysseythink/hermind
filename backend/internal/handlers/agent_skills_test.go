package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAgentSkillsHandler(t *testing.T) (*AgentSkillsHandler, *gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.Workspace{}, &models.User{})
	require.NoError(t, err)

	// Seed a workspace
	db.Create(&models.Workspace{Slug: "test-ws", Name: "Test Workspace"})

	svc := services.NewAgentSkillService(db)
	h := NewAgentSkillsHandler(svc)
	authSvc := services.NewAuthService(db, &config.Config{JWTSecret: "t"}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	RegisterAgentSkillsRoutes(api, svc, authSvc, db)

	return h, r, db
}

func TestAgentSkillsHandler_CRUD(t *testing.T) {
	_, r, _ := setupAgentSkillsHandler(t)

	// Create
	body, _ := json.Marshal(dto.CreateAgentSkillRequest{
		Name:        "test-skill",
		Description: "A test skill",
		Content:     "## Hello",
		Frontmatter: "name: test-skill\ndescription: A test skill\n",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/test-ws/agent-skills", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var createRes struct {
		Success bool                `json:"success"`
		Skill   models.AgentSkill   `json:"skill"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &createRes)
	require.NoError(t, err)
	assert.True(t, createRes.Success)
	assert.Equal(t, "test-skill", createRes.Skill.Name)

	// List
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/test-ws/agent-skills", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var listRes dto.AgentSkillListResponse
	err = json.Unmarshal(w.Body.Bytes(), &listRes)
	require.NoError(t, err)
	assert.Equal(t, 1, listRes.Count)

	// Get
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/test-ws/agent-skills/test-skill", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Update
	updateBody, _ := json.Marshal(dto.UpdateAgentSkillRequest{
		Description: "Updated description",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/workspace/test-ws/agent-skills/test-skill", bytes.NewReader(updateBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Patch
	patchBody, _ := json.Marshal(dto.PatchAgentSkillRequest{
		OldString: "Hello",
		NewString: "World",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PATCH", "/api/workspace/test-ws/agent-skills/test-skill", bytes.NewReader(patchBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/workspace/test-ws/agent-skills/test-skill", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify deleted
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/test-ws/agent-skills/test-skill", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentSkillsHandler_Files(t *testing.T) {
	_, r, _ := setupAgentSkillsHandler(t)

	// Create skill first
	body, _ := json.Marshal(dto.CreateAgentSkillRequest{
		Name:        "file-skill",
		Content:     "...",
		Frontmatter: "name: file-skill\ndescription: d\n",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/workspace/test-ws/agent-skills", bytes.NewReader(body))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Write file
	fileBody, _ := json.Marshal(dto.WriteSkillFileRequest{
		FilePath: "references/guide.md",
		Content:  "# Guide",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/workspace/test-ws/agent-skills/file-skill/files", bytes.NewReader(fileBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// List files
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/test-ws/agent-skills/file-skill/files", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get file
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/workspace/test-ws/agent-skills/file-skill/files/references/guide.md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete file
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/workspace/test-ws/agent-skills/file-skill/files/references/guide.md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
