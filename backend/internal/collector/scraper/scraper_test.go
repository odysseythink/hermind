package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Scrape_Generic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><p>Generic page content</p></body></html>"))
	}))
	defer server.Close()

	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	resp, err := manager.Scrape(context.Background(), server.URL, "text", nil, false, nil, "")
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Documents, 1)
	assert.Contains(t, resp.Documents[0].PageContent, "Generic page content")
	assert.Empty(t, resp.Documents[0].Location)
}

func TestManager_Scrape_Generic_Save(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><p>Save me</p></body></html>"))
	}))
	defer server.Close()

	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	storageDir := t.TempDir()
	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	resp, err := manager.Scrape(context.Background(), server.URL, "text", nil, true, map[string]string{"title": "Saved"}, storageDir)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Documents[0].Location)

	dest := filepath.Join(storageDir, "documents", "custom-documents", "saved.json")
	_, err = os.Stat(dest)
	require.NoError(t, err)
}

func TestManager_Scrape_InvalidURL(t *testing.T) {
	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	_, err = manager.Scrape(context.Background(), "http://", "text", nil, false, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestManager_Scrape_YouTubeRouting(t *testing.T) {
	// Since we can't mock youtube.com DNS, we verify routing logic at the
	// helper level in helpers_test. Here we verify that an invalid YouTube
	// ID results in an error when trying to fetch the real YouTube page.
	// Use a short context to avoid hanging on network/browser launch.
	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = manager.Scrape(ctx, "https://youtu.be/invalidvideo123456", "text", nil, false, nil, "")
	require.Error(t, err)
}

func TestManager_GetLinkText_Generic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Link text content</body></html>"))
	}))
	defer server.Close()

	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	resp, err := manager.GetLinkText(context.Background(), server.URL, "text")
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Content, "Link text content")
	assert.Equal(t, server.URL, resp.URL)
}

func TestManager_GetLinkText_HTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Raw HTML</body></html>"))
	}))
	defer server.Close()

	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	resp, err := manager.GetLinkText(context.Background(), server.URL, "html")
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Content, "<html>")
}

func TestManager_GetLinkText_InvalidURL(t *testing.T) {
	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	manager := NewManager(external.NewChromedpAdapter(nil), tokenizer)
	_, err = manager.GetLinkText(context.Background(), "http://", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestEnrichDocument(t *testing.T) {
	tokenizer, err := utils.NewTokenizer()
	require.NoError(t, err)

	doc := &core.Document{}
	enrichDocument(doc, "Hello world this is a test", tokenizer)
	assert.Equal(t, 6, doc.WordCount)
	assert.Greater(t, doc.TokenCountEstimate, 0)
}
