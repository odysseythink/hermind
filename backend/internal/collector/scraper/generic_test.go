package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericScraper_Fetch_Text(t *testing.T) {
	html := `<html><head><title>Test Page</title></head><body><p>Hello World</p><script>var x=1;</script><style>body{}</style></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewGenericScraper(external.NewChromedpAdapter(nil))
	content, err := scraper.Fetch(context.Background(), server.URL, "text", nil)
	require.NoError(t, err)
	assert.Contains(t, content, "Hello World")
	assert.NotContains(t, content, "var x=1")
	assert.NotContains(t, content, "body{}")
}

func TestGenericScraper_Fetch_HTML(t *testing.T) {
	html := `<html><body><p>Hello World</p></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewGenericScraper(external.NewChromedpAdapter(nil))
	content, err := scraper.Fetch(context.Background(), server.URL, "html", nil)
	require.NoError(t, err)
	assert.Contains(t, content, "<p>Hello World</p>")
}

func TestGenericScraper_Fetch_CustomHeaders(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>OK</body></html>"))
	}))
	defer server.Close()

	scraper := NewGenericScraper(external.NewChromedpAdapter(nil))
	_, err := scraper.Fetch(context.Background(), server.URL, "text", map[string]string{"X-Custom": "value"})
	require.NoError(t, err)
	assert.Contains(t, receivedUA, "Mozilla/5.0")
}

func TestGenericScraper_Fetch_HTTPError_Fallback(t *testing.T) {
	// When HTTP returns 500, the fallback to chromedp will also likely fail
	// because there is no real browser in tests. Use a short context to
	// avoid hanging on chromedp launch.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	scraper := NewGenericScraper(external.NewChromedpAdapter(nil))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := scraper.Fetch(ctx, server.URL, "text", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generic scraper failed")
}

func TestGenericScraper_ScrapeAndSave(t *testing.T) {
	html := `<html><body><p>Hello World from link</p></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	storageDir := t.TempDir()
	scraper := NewGenericScraper(external.NewChromedpAdapter(nil))
	resp, err := scraper.ScrapeAndSave(context.Background(), server.URL, nil, map[string]string{"title": "Test Link"}, storageDir, tokenizer)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Documents, 1)
	assert.Equal(t, "Test Link", resp.Documents[0].Title)
	assert.Contains(t, resp.Documents[0].PageContent, "Hello World from link")
	assert.Greater(t, resp.Documents[0].WordCount, 0)
	assert.Greater(t, resp.Documents[0].TokenCountEstimate, 0)
	assert.NotEmpty(t, resp.Documents[0].Location)

	// Verify file was written.
	dest := filepath.Join(storageDir, "documents", "custom-documents", "test-link.json")
	_, err = os.Stat(dest)
	require.NoError(t, err)
}

func TestGenericScraper_ScrapeAndSave_NoTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Content</body></html>"))
	}))
	defer server.Close()

	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	storageDir := t.TempDir()
	scraper := NewGenericScraper(external.NewChromedpAdapter(nil))
	resp, err := scraper.ScrapeAndSave(context.Background(), server.URL, nil, nil, storageDir, tokenizer)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, server.URL, resp.Documents[0].Title)
}
