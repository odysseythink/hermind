package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDocumentTestEnv(t *testing.T) (*apiTestEnv, *services.DocumentService, *services.FileSystemService) {
	t.Helper()
	env := newAPITestEnv(t, nil)
	tmpDir := t.TempDir()
	env.Cfg.StorageDir = tmpDir
	fsSvc := services.NewFileSystemService(tmpDir)
	docSvc := services.NewDocumentService(env.DB, env.Cfg, nil, nil, nil, nil, fsSvc)
	return env, docSvc, fsSvc
}

func registerDocumentRoutesForTest(env *apiTestEnv, docSvc *services.DocumentService, fsSvc *services.FileSystemService) {
	api := env.Router.Group("/api")
	RegisterAPIDocumentRoutes(api, env.APIKeySvc, docSvc, fsSvc, nil)
}

// ---------- D4: Raw Text ----------

func TestAPIDocument_RawText_Success(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, env.DB.Create(ws).Error)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	payload, _ := json.Marshal(map[string]any{
		"textContent":     "hello world",
		"metadata":        map[string]any{"title": "my-doc"},
		"addToWorkspaces": "ws",
	})
	req := httptest.NewRequest("POST", "/api/v1/document/raw-text", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Success   bool  `json:"success"`
		Documents []any `json:"documents"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.NotEmpty(t, body.Documents)
}

func TestAPIDocument_RawText_MissingTitle(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	payload, _ := json.Marshal(map[string]any{
		"textContent": "hello world",
		"metadata":    map[string]any{},
	})
	req := httptest.NewRequest("POST", "/api/v1/document/raw-text", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestAPIDocument_RawText_EmptyContent(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	payload, _ := json.Marshal(map[string]any{
		"textContent": "",
		"metadata":    map[string]any{"title": "x"},
	})
	req := httptest.NewRequest("POST", "/api/v1/document/raw-text", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// ---------- D1-D3: Upload error paths ----------

func TestAPIDocument_Upload_MissingFile(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	req := httptest.NewRequest("POST", "/api/v1/document/upload", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIDocument_Upload_NoWorkspaces(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	// Build multipart form with file but no addToWorkspaces.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("file", "test.txt")
	fw.Write([]byte("hello"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/document/upload", &b)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.False(t, body.Success)
}

func TestAPIDocument_UploadLink_MissingWorkspaces(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	payload, _ := json.Marshal(map[string]string{"link": "http://example.com"})
	req := httptest.NewRequest("POST", "/api/v1/document/upload-link", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.False(t, body.Success)
}

// ---------- D5-D6, D12: Folder ops ----------

func TestAPIDocument_CreateFolder(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	payload, _ := json.Marshal(map[string]string{"name": "my-folder"})
	req := httptest.NewRequest("POST", "/api/v1/document/create-folder", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body.Success)

	// Verify folder exists on disk.
	_, err := os.Stat(filepath.Join(env.Cfg.StorageDir, "documents", "my-folder"))
	assert.NoError(t, err)
}

func TestAPIDocument_RemoveFolder_Reserved(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	payload, _ := json.Marshal(map[string]string{"name": "custom-documents"})
	req := httptest.NewRequest("DELETE", "/api/v1/document/remove-folder", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ---------- D7-D9: Listing ----------

func TestAPIDocument_ListAll(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	req := httptest.NewRequest("GET", "/api/v1/documents", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		LocalFiles []any `json:"localFiles"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	// Empty storage dir returns empty list.
	assert.NotNil(t, body.LocalFiles)
}

func TestAPIDocument_GetByDocName_NotFound(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	req := httptest.NewRequest("GET", "/api/v1/document/nope.json", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------- D10-D11: Hardcoded responses ----------

func TestAPIDocument_MetadataSchema(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	req := httptest.NewRequest("GET", "/api/v1/document/metadata-schema", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Schema map[string]any `json:"schema"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotNil(t, body.Schema)
	assert.Equal(t, "string", body.Schema["title"])
}

func TestAPIDocument_AcceptedFileTypes(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	req := httptest.NewRequest("GET", "/api/v1/document/accepted-file-types", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Types map[string]string `json:"types"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotEmpty(t, body.Types)
}

func TestAPIDocument_DocNameDoesNotShadow(t *testing.T) {
	env, docSvc, fsSvc := newDocumentTestEnv(t)
	registerDocumentRoutesForTest(env, docSvc, fsSvc)

	// GET /v1/document/metadata-schema should return schema, NOT 404 from GetByDocName.
	req := httptest.NewRequest("GET", "/api/v1/document/metadata-schema", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Schema map[string]any `json:"schema"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotNil(t, body.Schema)
}
