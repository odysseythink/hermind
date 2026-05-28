package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupSystemMiscRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
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

	r := gin.New()
	handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, nil, nil, nil, nil, nil)
	return r, db, authSvc, cfg
}

func seedAdminUser(t *testing.T, db *gorm.DB, cfg *config.Config) (*models.User, string) {
	t.Helper()
	hash, _ := utils.HashPassword("pw")
	u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "admin", Suspended: 0, Bio: utils.Ptr("")}
	assert.NoError(t, db.Create(u).Error)
	tok, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, time.Hour)
	assert.NoError(t, err)
	return u, tok
}

func TestMigrate(t *testing.T) {
	r, _, _, _ := setupSystemMiscRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/migrate", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestEnvDump(t *testing.T) {
	r, _, _, _ := setupSystemMiscRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/env-dump", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestLocalFiles(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	// Seed some files
	os.MkdirAll(filepath.Join(cfg.StorageDir, "documents"), 0755)
	os.WriteFile(filepath.Join(cfg.StorageDir, "documents", "a.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(cfg.StorageDir, "documents", "folder1"), 0755)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/local-files", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "a.txt")
	assert.Contains(t, w.Body.String(), "folder1")
}

func TestCustomAppName(t *testing.T) {
	r, _, _, _ := setupSystemMiscRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/custom-app-name", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestUpdateDefaultSystemPrompt(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	body := `{"defaultSystemPrompt":"new prompt"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/default-system-prompt", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestSystemVectors_NoProvider(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/system-vectors", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "vectorCount")
}

func TestDocumentProcessingStatus_NoCollector(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/document-processing-status", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 503, w.Code)
}

func TestRemoveDocument(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	os.WriteFile(filepath.Join(cfg.StorageDir, "documents", "doc.txt"), []byte("data"), 0644)

	body := `{"name":"doc.txt"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/system/remove-document", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	_, err := os.Stat(filepath.Join(cfg.StorageDir, "documents", "doc.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveDocuments(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	os.WriteFile(filepath.Join(cfg.StorageDir, "documents", "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(cfg.StorageDir, "documents", "b.txt"), []byte("b"), 0644)

	body := `{"names":["a.txt","b.txt"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/system/remove-documents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestRemoveFolder(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	os.MkdirAll(filepath.Join(cfg.StorageDir, "documents", "folder1"), 0755)
	os.WriteFile(filepath.Join(cfg.StorageDir, "documents", "folder1", "file.txt"), []byte("data"), 0644)

	body := `{"name":"folder1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/system/remove-folder", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	_, err := os.Stat(filepath.Join(cfg.StorageDir, "documents", "folder1"))
	assert.True(t, os.IsNotExist(err))
}

func TestLogo_NoCustom(t *testing.T) {
	r, _, _, _ := setupSystemMiscRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/logo", nil)
	r.ServeHTTP(w, req)
	// No default logo file exists, so 204
	assert.Equal(t, 204, w.Code)
}

func TestPfp_NoPfp(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	user, token := seedAdminUser(t, db, cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/system/pfp/%d", user.ID), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 204, w.Code)
}

func TestUploadAndRemovePfp(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	user, token := seedAdminUser(t, db, cfg)

	// Upload PFP
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, _ := writer.CreateFormFile("file", "avatar.png")
	part.Write([]byte("fake-png-data"))
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/upload-pfp", &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "uploaded successfully")

	// Verify DB updated
	var updated models.User
	db.First(&updated, user.ID)
	assert.NotNil(t, updated.PfpFilename)
	assert.NotEmpty(t, *updated.PfpFilename)

	// Remove PFP
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/api/system/remove-pfp", nil)
	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w2.Body.String(), "removed successfully")
}

func TestUploadAndRemoveLogo(t *testing.T) {
	r, db, _, cfg := setupSystemMiscRouter(t)
	_, token := seedAdminUser(t, db, cfg)

	// Upload logo
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, _ := writer.CreateFormFile("file", "logo.png")
	part.Write([]byte("fake-logo-data"))
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/system/upload-logo", &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "uploaded successfully")

	// Remove logo
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/system/remove-logo", nil)
	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w2.Body.String(), "removed successfully")
}
