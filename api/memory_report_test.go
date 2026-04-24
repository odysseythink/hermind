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

type reportFake struct {
	storage.Storage
	events []*storage.MemoryEvent
}

func (r *reportFake) ListMemoryEvents(_ context.Context, _, _ int, _ []string) ([]*storage.MemoryEvent, error) {
	return r.events, nil
}

func TestHandleMemoryReport(t *testing.T) {
	fake := &reportFake{events: []*storage.MemoryEvent{
		{ID: 1, TS: time.Unix(1712345678, 0), Kind: "memory.consolidated", Data: []byte(`{"scanned":10}`)},
	}}
	srv, err := NewServer(&ServerOpts{Config: &config.Config{}, Storage: fake})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/memory/report?limit=5", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Events []*storage.MemoryEvent `json:"events"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Events, 1)
	assert.Equal(t, "memory.consolidated", got.Events[0].Kind)
}
