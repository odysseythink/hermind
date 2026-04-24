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

type testFakeStorage struct {
	storage.Storage
	stats *storage.MemoryStats
}

func (f *testFakeStorage) MemoryStats(_ context.Context) (*storage.MemoryStats, error) {
	return f.stats, nil
}

func TestHandleMemoryStats(t *testing.T) {
	fake := &testFakeStorage{stats: &storage.MemoryStats{
		Total:    5,
		ByType:   map[string]int{"episodic": 3, "semantic": 2},
		ByStatus: map[string]int{"active": 5},
	}}
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Storage: fake,
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/memory/stats", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got storage.MemoryStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 5, got.Total)
	assert.Equal(t, 3, got.ByType["episodic"])
}
