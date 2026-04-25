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

type healthFake struct {
	storage.Storage
	health *storage.MemoryHealth
	gen    *storage.SkillsGeneration
}

func (h *healthFake) MemoryHealth(_ context.Context) (*storage.MemoryHealth, error) {
	return h.health, nil
}

func (h *healthFake) GetSkillsGeneration(_ context.Context) (*storage.SkillsGeneration, error) {
	return h.gen, nil
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

func TestMemoryHealthIncludesCurrentSkillsGeneration(t *testing.T) {
	now := int64(1000)
	fake := &healthFake{
		health: &storage.MemoryHealth{
			SchemaVersion: 7, FTSIntegrity: "ok",
		},
		gen: &storage.SkillsGeneration{
			Hash:      "h-2",
			Seq:       2,
			UpdatedAt: toTime(now),
		},
	}
	srv, err := NewServer(&ServerOpts{Config: &config.Config{}, Storage: fake})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/memory/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		CurrentSkillsGeneration *struct {
			Hash      string `json:"hash"`
			Seq       int64  `json:"seq"`
			UpdatedAt int64  `json:"updated_at"`
		} `json:"current_skills_generation"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.CurrentSkillsGeneration)
	assert.Equal(t, "h-2", body.CurrentSkillsGeneration.Hash)
	assert.Equal(t, int64(2), body.CurrentSkillsGeneration.Seq)
	assert.Equal(t, now, body.CurrentSkillsGeneration.UpdatedAt)
}

func toTime(unix int64) time.Time {
	return time.Unix(unix, 0).UTC()
}
