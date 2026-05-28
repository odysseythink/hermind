package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIAuth_Authenticated(t *testing.T) {
	env := newAPITestEnv(t, nil)
	api := env.Router.Group("/api")
	RegisterAPIAuthRoutes(api, env.APIKeySvc)

	req := httptest.NewRequest("GET", "/api/v1/auth", nil)
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["authenticated"])
}

func TestAPIAuth_RejectsInvalidKey(t *testing.T) {
	env := newAPITestEnv(t, nil)
	api := env.Router.Group("/api")
	RegisterAPIAuthRoutes(api, env.APIKeySvc)

	req := httptest.NewRequest("GET", "/api/v1/auth", nil)
	req.Header.Set("Authorization", "Bearer bogus")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
