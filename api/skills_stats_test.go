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

type skillsFake struct {
	storage.Storage
	stats *storage.SkillsStats
}

func (s *skillsFake) SkillsStats(_ context.Context, _ string) (*storage.SkillsStats, error) {
	return s.stats, nil
}

func TestHandleSkillsStats(t *testing.T) {
	fake := &skillsFake{stats: &storage.SkillsStats{Total: 3, ByCategory: map[string]int{"coding": 3}}}
	srv, err := NewServer(&ServerOpts{
		Config:       &config.Config{},
		Storage:      fake,
		InstanceRoot: "/tmp/hermind-test",
	})
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/skills/stats", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var got storage.SkillsStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 3, got.Total)
}
