package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModelsHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/models", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "model-a"},
				{"id": "model-b"},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "model-a",
	})
	require.NoError(t, err)

	got, err := c.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"model-a", "model-b"}, got)
}

func TestListModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "model-a",
	})
	require.NoError(t, err)

	_, err = c.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestListModelsIncludesExtraHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "hermind-test", r.Header.Get("X-Foo"))
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{}})
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL:      srv.URL,
		APIKey:       "test-key",
		Model:        "model-a",
		ExtraHeaders: map[string]string{"X-Foo": "hermind-test"},
	})
	require.NoError(t, err)

	_, err = c.ListModels(context.Background())
	require.NoError(t, err)
}
