package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearXNGProvider_HappyPath(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		assert.Equal(t, "json", r.URL.Query().Get("format"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": "golang",
			"results": []map[string]any{
				{"title": "Go", "url": "https://go.dev", "content": "Programming language", "publishedDate": "2024-03-01"},
				{"title": "Docs", "url": "https://go.dev/doc", "content": "Documentation"},
			},
		})
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	results, err := p.Search(context.Background(), "golang", 7)
	require.NoError(t, err)
	assert.Equal(t, "golang", capturedQuery)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-03-01", results[0].PublishedDate)
	assert.Equal(t, "Docs", results[1].Title)
	assert.Equal(t, "", results[1].PublishedDate)
}

func TestSearXNGProvider_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	results, err := p.Search(context.Background(), "xyz nonsense", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearXNGProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 500")
}

func TestSearXNGProvider_Configured(t *testing.T) {
	assert.False(t, newSearXNGProvider("").Configured())
	assert.True(t, newSearXNGProvider("http://localhost:8080").Configured())
}

func TestSearXNGProvider_ID(t *testing.T) {
	assert.Equal(t, "searxng", newSearXNGProvider("").ID())
}

func TestSearXNGProvider_RespectsNResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		results := make([]map[string]any, 10)
		for i := 0; i < 10; i++ {
			results[i] = map[string]any{"title": fmt.Sprintf("R%d", i), "url": fmt.Sprintf("https://example.com/%d", i), "content": fmt.Sprintf("C%d", i)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	res, err := p.Search(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, res, 3)
	assert.Equal(t, "R0", res[0].Title)
	assert.Equal(t, "R1", res[1].Title)
	assert.Equal(t, "R2", res[2].Title)
}

func TestSearXNGProvider_TrailingSlash(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{}})
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL + "/")
	_, _ = p.Search(context.Background(), "q", 5)
	assert.Equal(t, "/search", capturedPath, "trailing slash in baseURL should not produce double slash")
}
