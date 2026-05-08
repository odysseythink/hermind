package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
)

func TestSuggestions(t *testing.T) {
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Streams: NewMemoryStreamHub(),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	suggestions, ok := body["suggestions"].([]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(suggestions), 1)
}
