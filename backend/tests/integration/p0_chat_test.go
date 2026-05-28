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

func TestChat_NonStreaming(t *testing.T) {
	router, authSvc, _ := setupTestDB(t)
	token, _ := createTestUser(t, authSvc, "chatuser", "password")
	ws := createTestWorkspace(t, router, authSvc, "chat-ws")

	body, _ := json.Marshal(dto.ChatRequest{Message: "Hello"})
	req := httptest.NewRequest("POST", "/api/workspace/"+ws.Slug+"/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp dto.ChatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "textResponse", resp.Type)
	assert.True(t, resp.Close)
	assert.NotEmpty(t, resp.TextResponse)
}
