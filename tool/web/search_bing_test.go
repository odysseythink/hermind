package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBingProvider_HappyPath(t *testing.T) {
	var capturedQuery, capturedCount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:125.0) Gecko/20100101 Firefox/125.0", r.Header.Get("User-Agent"))
		capturedQuery = r.URL.Query().Get("q")
		capturedCount = r.URL.Query().Get("count")

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<body>
<ol id="b_results">
<li class="b_algo">
<h2><a href="https://go.dev">The Go Programming Language</a></h2>
<div class="b_caption"><p>Go is an open source programming language.</p></div>
</li>
<li class="b_algo">
<h2><a href="https://go.dev/tour">A Tour of Go</a></h2>
<div class="b_caption"><p>Welcome to a tour of the Go programming language.</p></div>
</li>
</ol>
</body>
</html>`))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	results, err := p.Search(context.Background(), "golang", 3)
	require.NoError(t, err)
	assert.Equal(t, "golang", capturedQuery)
	assert.Equal(t, "3", capturedCount)
	require.Len(t, results, 2)
	assert.Equal(t, "The Go Programming Language", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "Go is an open source programming language.", results[0].Snippet)
	assert.Equal(t, "A Tour of Go", results[1].Title)
}

func TestBingProvider_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><ol id="b_results"></ol></body></html>`))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	results, err := p.Search(context.Background(), "xyz nonsense", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBingProvider_CAPTCHAError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div id="captcha">Please solve this CAPTCHA</div></body></html>`))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "captcha")
}

func TestBingProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 503")
}

func TestBingProvider_Configured(t *testing.T) {
	assert.True(t, newBingProvider("", "").Configured())
}

func TestBingProvider_ID(t *testing.T) {
	assert.Equal(t, "bing", newBingProvider("", "").ID())
}

func TestBingProvider_RespectsNResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		body := `<!DOCTYPE html><html><body><ol id="b_results">`
		for i := 0; i < 10; i++ {
			body += `<li class="b_algo"><h2><a href="https://example.com/` + fmt.Sprintf("%d", i) + `">Result ` + fmt.Sprintf("%d", i) + `</a></h2><div class="b_caption"><p>Snippet ` + fmt.Sprintf("%d", i) + `</p></div></li>`
		}
		body += `</ol></body></html>`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	results, err := p.Search(context.Background(), "test", 3)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "Result 0", results[0].Title)
	assert.Equal(t, "Result 1", results[1].Title)
	assert.Equal(t, "Result 2", results[2].Title)
}

func TestBingProvider_MarketParam(t *testing.T) {
	var capturedMarket string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMarket = r.URL.Query().Get("setmkt")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><ol id="b_results"></ol></body></html>`))
	}))
	defer srv.Close()

	p := newBingProvider("zh-CN", srv.URL)
	_, _ = p.Search(context.Background(), "q", 5)
	assert.Equal(t, "zh-CN", capturedMarket)
}

func TestDecodeBingURL(t *testing.T) {
	// Direct URL — unchanged
	assert.Equal(t, "https://go.dev", decodeBingURL("https://go.dev"))

	// Bing redirect with base64 "u" parameter
	assert.Equal(t, "https://go.dev", decodeBingURL("https://www.bing.com/ck/a?!&&p=abc&u=aHR0cHM6Ly9nby5kZXY"))

	// Non-Bing host with "u" param — unchanged
	assert.Equal(t, "https://example.com/?u=aHR0cHM6Ly9nby5kZXY", decodeBingURL("https://example.com/?u=aHR0cHM6Ly9nby5kZXY"))

	// Bing link without "u" param — unchanged
	assert.Equal(t, "https://www.bing.com/search?q=test", decodeBingURL("https://www.bing.com/search?q=test"))
}
