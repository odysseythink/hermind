package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodedError_RoundtripJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondCodedError(c, mcp.CodeToolNotFound, "tool x not found on y", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Nil(t, resp["result"])
	assert.Equal(t, "tool x not found on y", resp["error"])
	assert.Equal(t, "TOOL_NOT_FOUND", resp["errorCode"])
}

func TestCodedError_DetailsAreNotLeaked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with details", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		respondCodedError(c, mcp.CodeArgsSchemaMismatch, "bad args", map[string]any{"field": "text"})
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Contains(t, resp, "details")
		assert.Equal(t, map[string]any{"field": "text"}, resp["details"])
	})

	t.Run("nil details omitted", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		respondCodedError(c, mcp.CodeInvalidBody, "bad body", nil)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotContains(t, resp, "details")
	})
}

func TestCodedError_AllCodesMap(t *testing.T) {
	cases := []struct {
		code   mcp.ErrorCode
		status int
	}{
		{mcp.CodeInvalidBody, http.StatusBadRequest},
		{mcp.CodeInvalidParams, http.StatusUnprocessableEntity},
		{mcp.CodeServerNotFound, http.StatusNotFound},
		{mcp.CodeToolNotFound, http.StatusNotFound},
		{mcp.CodeArgsSchemaMismatch, http.StatusUnprocessableEntity},
		{mcp.CodeBodyTooLarge, http.StatusRequestEntityTooLarge},
		{mcp.CodeConcurrencyLimit, http.StatusTooManyRequests},
		{mcp.CodeCallTimeout, http.StatusGatewayTimeout},
		{mcp.CodeTransportError, http.StatusBadGateway},
		{mcp.CodeInternalError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			assert.Equal(t, tc.status, codeToStatus(tc.code))
		})
	}
}

func TestCodedError_RespectsContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondCodedError(c, mcp.CodeInternalError, "oops", nil)
	ct := w.Header().Get("Content-Type")
	assert.Contains(t, ct, "application/json")
	assert.Contains(t, ct, "charset=utf-8")
}
