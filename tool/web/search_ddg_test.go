package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ddgHTMLFixture = `
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
    <a class="result__a" rel="nofollow" href="/l/?uddg=https%3A%2F%2Fgo.dev&rut=xx">Go programming language</a>
  </h2>
  <a class="result__snippet" href="/l/?uddg=https%3A%2F%2Fgo.dev">The Go programming language is an open source project.</a>
</div>
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
    <a class="result__a" rel="nofollow" href="/l/?uddg=https%3A%2F%2Fgo.dev%2Fdoc&rut=yy">Go documentation</a>
  </h2>
  <a class="result__snippet">Documentation for the Go programming language.</a>
</div>
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
    <a class="result__a" rel="nofollow" href="https://example.com/direct">Direct absolute URL</a>
  </h2>
  <a class="result__snippet">Already absolute.</a>
</div>
`

func TestDDGProvider_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "golang", form.Get("q"))

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(ddgHTMLFixture))
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	results, err := p.Search(context.Background(), "golang", 10)
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "Go programming language", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Contains(t, results[0].Snippet, "open source project")
	assert.Nil(t, results[0].Score)

	assert.Equal(t, "Go documentation", results[1].Title)
	assert.Equal(t, "https://go.dev/doc", results[1].URL)

	assert.Equal(t, "https://example.com/direct", results[2].URL)
}

func TestDDGProvider_CAPTCHAReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div class="anomaly-modal">Sorry, please verify.</div></body></html>`))
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	_, err := p.Search(context.Background(), "q", 10)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "rate limited")
}

func TestDDGProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	_, err := p.Search(context.Background(), "q", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 502")
}

func TestDDGProvider_AlwaysConfigured(t *testing.T) {
	assert.True(t, newDDGProvider("").Configured())
}

func TestDDGProvider_ID(t *testing.T) {
	assert.Equal(t, "ddg", newDDGProvider("").ID())
}

func TestDDGProvider_RespectsNResults(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(`
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title"><a class="result__a" href="https://x/`)
		sb.WriteString("a")
		sb.WriteString(`">T</a></h2>
  <a class="result__snippet">S</a>
</div>`)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sb.String()))
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	results, err := p.Search(context.Background(), "q", 3)
	require.NoError(t, err)
	assert.Len(t, results, 3, "should clip to n=3")
}
