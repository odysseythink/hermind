package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestWebScraping_FetchHTML_ExtractsArticle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Test Article</title></head><body><article><p>Hello world</p></article></body></html>`)
	}))
	defer ts.Close()

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewWebScrapingSkill(tc)
	result, err := entry.Handler(context.Background(), json.RawMessage(`{"url":"`+ts.URL+`"}`))
	require.NoError(t, err)
	require.Contains(t, result, "Hello world")
	require.Contains(t, result, "Test Article")
}

func TestWebScraping_FallbackToBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><p>Body text</p></body></html>`)
	}))
	defer ts.Close()

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewWebScrapingSkill(tc)
	result, err := entry.Handler(context.Background(), json.RawMessage(`{"url":"`+ts.URL+`"}`))
	require.NoError(t, err)
	require.Contains(t, result, "Body text")
}

func TestWebScraping_404_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewWebScrapingSkill(tc)
	result, err := entry.Handler(context.Background(), json.RawMessage(`{"url":"`+ts.URL+`"}`))
	require.NoError(t, err)
	require.Contains(t, result, "error")
	require.Contains(t, result, "404")
}

func TestWebScraping_RespectsMaxResultChars(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><p>`+strings.Repeat("word ", 20000)+`</p></body></html>`)
	}))
	defer ts.Close()

	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewWebScrapingSkill(tc)
	result, err := entry.Handler(context.Background(), json.RawMessage(`{"url":"`+ts.URL+`"}`))
	require.NoError(t, err)
	require.True(t, len(result) > 100) // should have content
}

func TestWebScraping_RejectsNonHTTPSchemes(t *testing.T) {
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewWebScrapingSkill(tc)

	for _, scheme := range []string{"ftp://example.com", "file:///etc/passwd"} {
		result, err := entry.Handler(context.Background(), json.RawMessage(`{"url":"`+scheme+`"}`))
		require.NoError(t, err)
		require.Contains(t, result, "error", "scheme %s should be rejected", scheme)
		require.Contains(t, result, "only http/https", "scheme %s should be rejected", scheme)
	}
}

func TestExtractMainText_SkipsScriptAndStyle(t *testing.T) {
	html := `<html><head><title>T</title></head><body><script>alert(1)</script><style>.x{}</style><nav>Nav</nav><aside>Side</aside><p>Keep me</p></body></html>`
	text, title := extractMainText([]byte(html))
	require.Equal(t, "T", title)
	require.Contains(t, text, "Keep me")
	require.NotContains(t, text, "alert")
	require.NotContains(t, text, "Nav")
	require.NotContains(t, text, "Side")
}
