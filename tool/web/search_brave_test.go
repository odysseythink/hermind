package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBraveProvider_HappyPath(t *testing.T) {
	var capturedQuery, capturedCount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "brv-key", r.Header.Get("X-Subscription-Token"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		capturedQuery = r.URL.Query().Get("q")
		capturedCount = r.URL.Query().Get("count")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"results": []map[string]any{
					{"title": "Go", "url": "https://go.dev", "description": "Programming language", "page_age": "2024-02-01"},
					{"title": "Docs", "url": "https://go.dev/doc", "description": "Docs"},
				},
			},
		})
	}))
	defer srv.Close()

	p := newBraveProvider("brv-key", srv.URL)
	results, err := p.Search(context.Background(), "golang", 3)
	require.NoError(t, err)
	assert.Equal(t, "golang", capturedQuery)
	assert.Equal(t, "3", capturedCount)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-02-01", results[0].PublishedDate)
	assert.Nil(t, results[0].Score, "Brave has no score")
	assert.Equal(t, "", results[1].PublishedDate, "missing page_age → empty")
}

func TestBraveProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := newBraveProvider("bad", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 403")
}

func TestBraveProvider_Configured(t *testing.T) {
	t.Setenv("BRAVE_API_KEY", "")
	assert.False(t, newBraveProvider("", "").Configured())
	assert.True(t, newBraveProvider("k", "").Configured())

	t.Setenv("BRAVE_API_KEY", "env-key")
	assert.True(t, newBraveProvider("", "").Configured())
}

func TestBraveProvider_ID(t *testing.T) {
	assert.Equal(t, "brave", newBraveProvider("k", "").ID())
}
