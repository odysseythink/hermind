package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupP4dRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test-secret", AuthToken: "test-auth-token", MultiUserMode: true}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, _ := db.DB(); sqlDB != nil {
			sqlDB.Close()
		}
	})
	assert.NoError(t, services.AutoMigrate(db))
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	sysSvc := services.NewSystemService(db)
	apiKeySvc := services.NewAPIKeyService(db)
	adminSvc := services.NewAdminService(db)
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	promptPresetSvc := services.NewPromptPresetService(db)
	promptVariableSvc := services.NewPromptVariableService(db)
	wsChatSvc := services.NewWorkspaceChatService(db)

	r := gin.New()
	handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, promptPresetSvc, promptVariableSvc, wsChatSvc)
	return r, db, authSvc, cfg
}

func seedWorkspaceChat(t *testing.T, db *gorm.DB) {
	var count int64
	db.Model(&models.Workspace{}).Count(&count)
	slug := fmt.Sprintf("test-ws-%d", count)
	ws := models.Workspace{Name: "Test WS", Slug: slug}
	assert.NoError(t, db.Create(&ws).Error)
	resp, _ := json.Marshal(map[string]string{"text": "hello"})
	chat := models.WorkspaceChat{
		WorkspaceID: ws.ID,
		Prompt:      "hi",
		Response:    string(resp),
		Include:     true,
	}
	assert.NoError(t, db.Create(&chat).Error)
}

// ---------- Prompt Presets ----------

func TestListSlashCommandPresets(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/slash-command-presets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotNil(t, body["presets"])
}

func TestCreateSlashCommandPreset(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	payload := map[string]string{"command": "summarize", "prompt": "Summarize this", "description": "desc"}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/slash-command-presets", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 201, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotNil(t, body["preset"])
}

func TestCreateSlashCommandPreset_SystemCommandConflict(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	payload := map[string]string{"command": "reset", "prompt": "Reset", "description": "desc"}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/slash-command-presets", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestUpdateSlashCommandPreset(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	// Create first
	create := map[string]string{"command": "old", "prompt": "Old", "description": "d"}
	b, _ := json.Marshal(create)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/slash-command-presets", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 201, w.Code)

	var createBody map[string]any
	json.Unmarshal(w.Body.Bytes(), &createBody)
	preset := createBody["preset"].(map[string]any)
	id := int(preset["id"].(float64))

	// Update
	update := map[string]string{"command": "new", "prompt": "New prompt", "description": "new desc"}
	b, _ = json.Marshal(update)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/system/slash-command-presets/"+strconv.Itoa(id), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	updatedPreset := body["preset"].(map[string]any)
	assert.Equal(t, "/new", updatedPreset["command"])
}

func TestDeleteSlashCommandPreset(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	// Create
	create := map[string]string{"command": "del", "prompt": "Del", "description": "d"}
	b, _ := json.Marshal(create)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/slash-command-presets", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 201, w.Code)

	var createBody map[string]any
	json.Unmarshal(w.Body.Bytes(), &createBody)
	preset := createBody["preset"].(map[string]any)
	id := int(preset["id"].(float64))

	// Delete
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/system/slash-command-presets/"+strconv.Itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 204, w.Code)
}

// ---------- Prompt Variables ----------

func TestListPromptVariables(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/prompt-variables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	vars := body["variables"].([]any)
	assert.GreaterOrEqual(t, len(vars), 1) // at least defaults
}

func TestCreatePromptVariable(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	payload := map[string]string{"key": "custom_var", "value": "hello", "description": "desc"}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/prompt-variables", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["success"].(bool))
}

func TestUpdatePromptVariable(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	// Create
	create := map[string]string{"key": "old_key", "value": "old", "description": "d"}
	b, _ := json.Marshal(create)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/prompt-variables", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var createBody map[string]any
	json.Unmarshal(w.Body.Bytes(), &createBody)
	v := createBody["variable"].(map[string]any)
	id := int(v["id"].(float64))

	// Update
	update := map[string]string{"key": "new_key", "value": "new", "description": "nd"}
	b, _ = json.Marshal(update)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/system/prompt-variables/"+strconv.Itoa(id), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["success"].(bool))
}

func TestDeletePromptVariable(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	// Create
	create := map[string]string{"key": "del_key", "value": "val", "description": "d"}
	b, _ := json.Marshal(create)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/prompt-variables", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var createBody map[string]any
	json.Unmarshal(w.Body.Bytes(), &createBody)
	v := createBody["variable"].(map[string]any)
	id := int(v["id"].(float64))

	// Delete
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/system/prompt-variables/"+strconv.Itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

// ---------- Workspace Chats ----------

func TestWorkspaceChats(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)
	seedWorkspaceChat(t, db)

	payload := map[string]int{"offset": 0, "limit": 20}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/workspace-chats", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotNil(t, body["chats"])
	assert.NotNil(t, body["totalChats"])
}

func TestDeleteWorkspaceChat(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)
	seedWorkspaceChat(t, db)

	var chat models.WorkspaceChat
	assert.NoError(t, db.First(&chat).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/system/workspace-chats/"+strconv.Itoa(chat.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["success"].(bool))
}

func TestDeleteAllWorkspaceChats(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)
	seedWorkspaceChat(t, db)
	seedWorkspaceChat(t, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/system/workspace-chats/-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var count int64
	db.Model(&models.WorkspaceChat{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

// ---------- Export Chats ----------

func TestExportChats_CSV(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)
	seedWorkspaceChat(t, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/export-chats?type=csv", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "id,workspace,prompt,response,sent_at,username,rating")
}

func TestExportChats_JSONL(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)
	seedWorkspaceChat(t, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/export-chats?type=jsonl", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "application/jsonl", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "messages")
}

// ---------- Validate SQL Connection ----------

func TestValidateSQLConnection_SQLite(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	dbPath := cfg.StorageDir + "/test.db"
	payload := map[string]string{"engine": "sqlite", "connectionString": dbPath}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/validate-sql-connection", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["success"].(bool))
}

func TestValidateSQLConnection_MissingParams(t *testing.T) {
	r, db, _, cfg := setupP4dRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	payload := map[string]string{"engine": ""}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/validate-sql-connection", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

// utils.Itoa helper
func init() {}
