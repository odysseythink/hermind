package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/agent/presence"
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

// presenceStub is a test-only Source used by the api package.
type presenceStub struct {
	name string
	vote presence.Vote
}

func (s presenceStub) Name() string                     { return s.name }
func (s presenceStub) Vote(_ time.Time) presence.Vote   { return s.vote }

func TestMemoryHealthIncludesPresenceBlock(t *testing.T) {
	fake := &healthFake{
		health: &storage.MemoryHealth{
			SchemaVersion: 7, FTSIntegrity: "ok",
		},
	}
	srv, err := NewServer(&ServerOpts{Config: &config.Config{}, Storage: fake})
	require.NoError(t, err)

	// Inject a Composite with two stub sources: one Absent, one Unknown.
	srv.opts.Deps.Presence = presence.NewComposite(
		presenceStub{name: "http_idle", vote: presence.Absent},
		presenceStub{name: "sleep_window", vote: presence.Unknown},
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/memory/health", nil)
	srv.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Presence *struct {
			Available bool `json:"available"`
			Sources   []struct {
				Name string `json:"name"`
				Vote string `json:"vote"`
			} `json:"sources"`
		} `json:"presence"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Presence)
	require.True(t, body.Presence.Available, "absent + unknown ⇒ available")
	require.Len(t, body.Presence.Sources, 2)
	require.Equal(t, "http_idle", body.Presence.Sources[0].Name)
	require.Equal(t, "Absent", body.Presence.Sources[0].Vote)
	require.Equal(t, "sleep_window", body.Presence.Sources[1].Name)
	require.Equal(t, "Unknown", body.Presence.Sources[1].Vote)
}
