package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExaProvider_HappyPath(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query      string `json:"query"`
			NumResults int    `json:"numResults"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		captured = req.Query
		assert.Equal(t, 5, req.NumResults)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"title": "Go", "url": "https://go.dev", "text": "Programming language", "publishedDate": "2024-01", "score": 0.9},
				{"title": "Effective Go", "url": "https://go.dev/doc/effective_go"},
			},
		})
	}))
	defer srv.Close()

	p := newExaProvider("test-key", srv.URL)
	results, err := p.Search(context.Background(), "golang", 5)
	require.NoError(t, err)
	assert.Equal(t, "golang", captured)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-01", results[0].PublishedDate)
	require.NotNil(t, results[0].Score)
	assert.InDelta(t, 0.9, *results[0].Score, 0.001)
}

func TestExaProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newExaProvider("bad-key", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 401")
}

func TestExaProvider_ConfiguredRequiresKey(t *testing.T) {
	t.Setenv("EXA_API_KEY", "")
	assert.False(t, newExaProvider("", "").Configured())
	assert.True(t, newExaProvider("k", "").Configured())
}

func TestExaProvider_ConfiguredUsesEnvFallback(t *testing.T) {
	t.Setenv("EXA_API_KEY", "env-key")
	p := newExaProvider("", "")
	assert.True(t, p.Configured())
}

func TestExaProvider_ID(t *testing.T) {
	assert.Equal(t, "exa", newExaProvider("k", "").ID())
}
