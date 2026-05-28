package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUploadToWorkspace(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "docuser", "password")
	ws := createTestWorkspace(t, router, authSvc, "doc-ws")

	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/upload", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestUploadAndEmbed(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "embuser", "password")
	ws := createTestWorkspace(t, router, authSvc, "emb-ws")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "embed.txt")
	part.Write([]byte("embed me"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/upload-and-embed", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestUpdateEmbeddings(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "upduser", "password")
	ws := createTestWorkspace(t, router, authSvc, "upd-ws")

	// Upload a document first
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "update.txt")
	part.Write([]byte("update embeddings"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/upload-and-embed", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var uploadResp struct {
		Document struct {
			DocId string `json:"docId"`
		} `json:"document"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)

	// Now update embeddings
	updBody, _ := json.Marshal(map[string]any{
		"adds":    []string{uploadResp.Document.DocId},
		"removes": []string{},
	})
	req = httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/update-embeddings", bytes.NewReader(updBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestRemoveAndUnembed(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "remuser", "password")
	ws := createTestWorkspace(t, router, authSvc, "rem-ws")

	// Upload a document first
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "remove.txt")
	part.Write([]byte("remove me"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/upload-and-embed", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var uploadResp struct {
		Document struct {
			DocId string `json:"docId"`
		} `json:"document"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)

	// Now remove it
	req = httptest.NewRequest("DELETE", "/api/workspace/"+ws.Slug+"/remove-and-unembed?docId="+uploadResp.Document.DocId, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestCreateFolder(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "folderuser", "password")

	body, _ := json.Marshal(map[string]string{"name": "test-folder"})
	req := httptest.NewRequest("POST", "/api/document/create-folder", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestListDocuments(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "listuser", "password")

	req := httptest.NewRequest("GET", "/api/documents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "documents")
}

func TestCreateFolder_TraversalBlocked(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "travuser", "password")

	body, _ := json.Marshal(map[string]string{"name": "../../../etc"})
	req := httptest.NewRequest("POST", "/api/document/create-folder", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetDocument_NotFound(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "getuser", "password")

	req := httptest.NewRequest("GET", "/api/document/nonexistent-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
