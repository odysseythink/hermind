package extensions

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

func TestWebsiteDepthExtension_Name(t *testing.T) {
	w := NewWebsiteDepthExtension()
	assert.Equal(t, "website-depth", w.Name())
}

func TestWebsiteDepthExtension_Handle_UnsupportedMethod(t *testing.T) {
	w := NewWebsiteDepthExtension()
	_, err := w.Handle(context.Background(), "/ext/website-depth", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestWebsiteDepthExtension_Handle_UnknownEndpoint(t *testing.T) {
	w := NewWebsiteDepthExtension()
	_, err := w.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestWebsiteDepthExtension_crawl(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Home</title></head><body>
			<p>Home page content</p>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
		</body></html>`)
	})
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Page 1</title></head><body>
			<p>Page 1 content</p>
		</body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Page 2</title></head><body>
			<p>Page 2 content</p>
		</body></html>`)
	})

	ext := NewWebsiteDepthExtensionWithClient(server.Client())
	body, _ := json.Marshal(WebsiteDepthRequest{URL: server.URL, Depth: 2, MaxLinks: 10})
	resp, err := ext.crawl(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	pages, ok := resp.Data["pages"].([]PageInfo)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(pages), 1)
	assert.Equal(t, "Home", pages[0].Title)
	assert.Contains(t, pages[0].Content, "Home page content")
}

func TestWebsiteDepthExtension_crawl_MaxLinks(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Home</title></head><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
			<a href="/page3">Page 3</a>
		</body></html>`)
	})
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Page 1</title></head><body>Page 1</body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Page 2</title></head><body>Page 2</body></html>`)
	})
	mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Page 3</title></head><body>Page 3</body></html>`)
	})

	ext := NewWebsiteDepthExtensionWithClient(server.Client())
	body, _ := json.Marshal(WebsiteDepthRequest{URL: server.URL, Depth: 2, MaxLinks: 2})
	resp, err := ext.crawl(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	pages, ok := resp.Data["pages"].([]PageInfo)
	require.True(t, ok)
	assert.LessOrEqual(t, len(pages), 2)
}

func TestWebsiteDepthExtension_crawl_InvalidBody(t *testing.T) {
	ext := NewWebsiteDepthExtension()
	_, err := ext.crawl(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestWebsiteDepthExtension_crawl_InvalidURL(t *testing.T) {
	ext := NewWebsiteDepthExtension()
	body, _ := json.Marshal(WebsiteDepthRequest{URL: "://invalid-url", Depth: 1, MaxLinks: 5})
	_, err := ext.crawl(context.Background(), body)
	assert.Error(t, err)
}

func TestWebsiteDepthExtension_crawl_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ext := NewWebsiteDepthExtensionWithClient(server.Client())
	body, _ := json.Marshal(WebsiteDepthRequest{URL: server.URL, Depth: 1, MaxLinks: 5})
	resp, err := ext.crawl(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	pages, _ := resp.Data["pages"].([]PageInfo)
	assert.Len(t, pages, 0)
}

func TestWebsiteDepthExtension_fetchPage(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Test</title></head><body>
			<p>Hello World</p>
			<a href="/other">Other</a>
			<a href="http://example.com">External</a>
		</body></html>`)
	})

	ext := NewWebsiteDepthExtensionWithClient(server.Client())
	page, links, err := ext.fetchPage(context.Background(), server.URL)
	require.NoError(t, err)
	assert.Equal(t, "Test", page.Title)
	assert.Contains(t, page.Content, "Hello World")
	assert.Len(t, links, 1) // only internal link
	assert.Equal(t, server.URL+"/other", links[0])
}

func TestVisitedLink(t *testing.T) {
	links := []string{"a", "b", "c"}
	assert.True(t, visitedLink(links, "b"))
	assert.False(t, visitedLink(links, "d"))
}
