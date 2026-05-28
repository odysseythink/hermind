package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWebScraping_HappyPath_ExtractsArticle(t *testing.T) {
	html := `<html><body><article><h1>Title</h1><p>First para.</p><p>Second para.</p></article></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteWebScraping(context.Background(), fc, map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	require.Contains(t, out, "Title")
	require.Contains(t, out, "First para")
}

func TestWebScraping_InterpolatesURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/page/99", r.URL.Path)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>page</body></html>`))
	}))
	defer srv.Close()

	fc := newFlowContext()
	fc.Variables["id"] = "99"
	_, err := ExecuteWebScraping(context.Background(), fc, map[string]any{
		"url": srv.URL + "/page/{{id}}",
	})
	require.NoError(t, err)
}

func TestWebScraping_SSRFGuard_Blocks(t *testing.T) {
	fc := &Context{
		Variables:       map[string]string{},
		Emit:            func(string) {},
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
		AllowPrivateIPs: false,
	}
	_, err := ExecuteWebScraping(context.Background(), fc, map[string]any{
		"url": "http://127.0.0.1/secret",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestWebScraping_HTTPError_Returns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	fc := newFlowContext()
	_, err := ExecuteWebScraping(context.Background(), fc, map[string]any{
		"url": srv.URL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 404")
}

func TestWebScraping_NonHTML_ReturnsRawText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("plain text response"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteWebScraping(context.Background(), fc, map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	require.Equal(t, "plain text response", out)
}
