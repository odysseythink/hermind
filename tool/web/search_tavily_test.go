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

func TestTavilyProvider_HappyPath(t *testing.T) {
	var capturedKey, capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			APIKey     string `json:"api_key"`
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		capturedKey = req.APIKey
		capturedQuery = req.Query
		assert.Equal(t, 7, req.MaxResults)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": "golang",
			"results": []map[string]any{
				{"title": "Go", "url": "https://go.dev", "content": "Programming language", "score": 0.85, "published_date": "2024-03-01"},
				{"title": "Tour", "url": "https://go.dev/tour"},
			},
		})
	}))
	defer srv.Close()

	p := newTavilyProvider("tav-key", srv.URL)
	results, err := p.Search(context.Background(), "golang", 7)
	require.NoError(t, err)
	assert.Equal(t, "tav-key", capturedKey)
	assert.Equal(t, "golang", capturedQuery)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-03-01", results[0].PublishedDate)
	require.NotNil(t, results[0].Score)
	assert.InDelta(t, 0.85, *results[0].Score, 0.001)
	assert.Nil(t, results[1].Score, "missing score must serialize as nil pointer")
}

func TestTavilyProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newTavilyProvider("bad", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 401")
}

func TestTavilyProvider_Configured(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	assert.False(t, newTavilyProvider("", "").Configured())
	assert.True(t, newTavilyProvider("k", "").Configured())

	t.Setenv("TAVILY_API_KEY", "env-key")
	assert.True(t, newTavilyProvider("", "").Configured())
}

func TestTavilyProvider_ID(t *testing.T) {
	assert.Equal(t, "tavily", newTavilyProvider("k", "").ID())
}
