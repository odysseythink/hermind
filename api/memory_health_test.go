package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type healthFake struct {
	storage.Storage
	health *storage.MemoryHealth
}

func (h *healthFake) MemoryHealth(_ context.Context) (*storage.MemoryHealth, error) {
	return h.health, nil
}

func TestHandleMemoryHealth(t *testing.T) {
	fake := &healthFake{health: &storage.MemoryHealth{
		SchemaVersion: 7, FTSIntegrity: "ok",
	}}
	srv, err := NewServer(&ServerOpts{Config: &config.Config{}, Storage: fake})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/memory/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got storage.MemoryHealth
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 7, got.SchemaVersion)
	assert.Equal(t, "ok", got.FTSIntegrity)
}
