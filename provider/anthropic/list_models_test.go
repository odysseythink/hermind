package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicListModelsHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, defaultAPIVersion, r.Header.Get("anthropic-version"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "claude-opus-4-6"},
				{"id": "claude-sonnet-4-6"},
			},
		})
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "anthropic",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	a := p.(*Anthropic)
	got, err := a.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"claude-opus-4-6", "claude-sonnet-4-6"}, got)
}

func TestAnthropicListModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "anthropic",
		BaseURL:  srv.URL,
		APIKey:   "bad-key",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	a := p.(*Anthropic)
	_, err = a.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
