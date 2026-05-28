package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchWorkspaces(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "searchuser", "password")
	createTestWorkspace(t, router, authSvc, "searchable-ws")

	body, _ := json.Marshal(map[string]string{"searchTerm": "searchable"})
	req := httptest.NewRequest("POST", "/api/workspace/search", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp dto.SearchResults
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Workspaces, 1)
	assert.Contains(t, resp.Workspaces[0].Slug, "searchable-ws")
}

func TestSearchWorkspaces_ShortTerm(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "searchuser2", "password")

	body, _ := json.Marshal(map[string]string{"searchTerm": "ab"})
	req := httptest.NewRequest("POST", "/api/workspace/search", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp dto.SearchResults
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Workspaces, 0)
	assert.Len(t, resp.Threads, 0)
}

func TestVectorSearch(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "vsearchuser", "password")
	ws := createTestWorkspace(t, router, authSvc, "vsearch-ws")

	body, _ := json.Marshal(dto.VectorSearchRequest{Query: "test query"})
	req := httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/vector-search", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Results []dto.VectorSearchResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// Empty results expected since no vectors exist
	assert.NotNil(t, resp.Results)
}
