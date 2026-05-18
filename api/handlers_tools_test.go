package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

type testTool struct {
	Name        string
	Description string
	Toolset     string
}

func buildTestRegistry(t *testing.T, tools []testTool) *tool.Registry {
	t.Helper()
	reg := tool.NewRegistry()
	for _, tt := range tools {
		reg.Register(&tool.Entry{
			Name:        tt.Name,
			Description: tt.Description,
			Toolset:     tt.Toolset,
			Handler:     func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil },
		})
	}
	return reg
}

func TestToolsList_EmptyRegistry(t *testing.T) {
	srv, err := NewServer(&ServerOpts{
		Config: &config.Config{},
		Deps:   &EngineDeps{ToolReg: nil},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Empty(t, resp.Tools)
}

func TestToolsList_WithRegistry(t *testing.T) {
	cfg := &config.Config{}
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "beta", Description: "Beta tool", Toolset: "ts1"},
				{Name: "alpha", Description: "Alpha tool", Toolset: "ts1"},
			}),
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Tools, 2)

	assert.Equal(t, "alpha", resp.Tools[0].Name)
	assert.Equal(t, "Alpha tool", resp.Tools[0].Description)
	assert.Equal(t, "ts1", resp.Tools[0].Toolset)
	assert.True(t, resp.Tools[0].Enabled)

	assert.Equal(t, "beta", resp.Tools[1].Name)
	assert.Equal(t, "Beta tool", resp.Tools[1].Description)
	assert.Equal(t, "ts1", resp.Tools[1].Toolset)
	assert.True(t, resp.Tools[1].Enabled)
}

func TestToolsList_DisabledTools(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			Disabled: []string{"beta"},
		},
	}
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "gamma", Description: "Gamma tool", Toolset: "ts2"},
				{Name: "beta", Description: "Beta tool", Toolset: "ts1"},
				{Name: "alpha", Description: "Alpha tool", Toolset: "ts1"},
			}),
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Tools, 3)

	assert.Equal(t, "alpha", resp.Tools[0].Name)
	assert.True(t, resp.Tools[0].Enabled)
	assert.Equal(t, "beta", resp.Tools[1].Name)
	assert.False(t, resp.Tools[1].Enabled)
	assert.Equal(t, "gamma", resp.Tools[2].Name)
	assert.True(t, resp.Tools[2].Enabled)
}

func TestActiveToolReg_FiltersDisabled(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			Disabled: []string{"beta"},
		},
	}
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "gamma", Description: "Gamma tool", Toolset: "ts2"},
				{Name: "beta", Description: "Beta tool", Toolset: "ts1"},
				{Name: "alpha", Description: "Alpha tool", Toolset: "ts1"},
			}),
		},
	})
	require.NoError(t, err)

	active := srv.activeToolReg()
	require.NotNil(t, active)
	entries := active.Entries(nil)
	require.Len(t, entries, 2)

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	assert.ElementsMatch(t, []string{"alpha", "gamma"}, names)
}

func TestActiveToolReg_AllDisabled(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			Disabled: []string{"alpha", "beta"},
		},
	}
	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "beta", Description: "Beta tool", Toolset: "ts1"},
				{Name: "alpha", Description: "Alpha tool", Toolset: "ts1"},
			}),
		},
	})
	require.NoError(t, err)

	active := srv.activeToolReg()
	require.NotNil(t, active)
	entries := active.Entries(nil)
	assert.Empty(t, entries)
}

func TestActiveToolReg_NilRegistry(t *testing.T) {
	srv, err := NewServer(&ServerOpts{
		Config: &config.Config{},
		Deps:   &EngineDeps{ToolReg: nil},
	})
	require.NoError(t, err)

	active := srv.activeToolReg()
	assert.Nil(t, active)
}

// TestToolsList_HotReload verifies that changes to s.opts.Config.Tools.Disabled
// are immediately reflected by handleToolsList without a server restart.
func TestToolsList_HotReload(t *testing.T) {
	cfg := &config.Config{}
	s, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "alpha"},
				{Name: "beta"},
			}),
		},
	})
	require.NoError(t, err)

	// Initially all enabled
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/tools", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Tools, 2)
	assert.True(t, resp.Tools[0].Enabled)
	assert.True(t, resp.Tools[1].Enabled)

	// Simulate hot-reload: update config directly (same as handleConfigPut does)
	s.opts.Config.Tools.Disabled = []string{"beta"}

	// Now beta should be disabled without restarting the server
	rr = httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/tools", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Tools, 2)
	byName := make(map[string]bool)
	for _, t := range resp.Tools {
		byName[t.Name] = t.Enabled
	}
	assert.True(t, byName["alpha"])
	assert.False(t, byName["beta"])
}
