package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type getFake struct {
	storage.Storage
	mem *storage.Memory
}

func (f *getFake) GetMemory(_ context.Context, id string) (*storage.Memory, error) {
	if f.mem == nil || f.mem.ID != id {
		return nil, storage.ErrNotFound
	}
	return f.mem, nil
}

func TestHandleMemoryGet_Found(t *testing.T) {
	fake := &getFake{mem: &storage.Memory{ID: "m1", Content: "hello", CreatedAt: time.Now().UTC()}}
	srv, err := NewServer(&ServerOpts{Config: &config.Config{}, Storage: fake})
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/memory/m1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var got storage.Memory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "m1", got.ID)
}

func TestHandleMemoryGet_NotFound(t *testing.T) {
	fake := &getFake{}
	srv, err := NewServer(&ServerOpts{Config: &config.Config{}, Storage: fake})
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/memory/nope", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
