// tool/web/web_test.go
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebFetchHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "hermes-agent/1.0", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch", Parameters: json.RawMessage(webFetchSchema)}},
	})

	args := json.RawMessage(`{"url":"` + srv.URL + `"}`)
	out, err := reg.Dispatch(context.Background(), "web_fetch", args)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, float64(200), result["status"])
	assert.Equal(t, "hello world", result["content"])
}

func TestWebFetchRejectsMissingURL(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch"}},
	})
	out, err := reg.Dispatch(context.Background(), "web_fetch", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "url")
}

func TestWebFetchHandlesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "not found")
	}))
	defer srv.Close()

	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch"}},
	})
	args := json.RawMessage(`{"url":"` + srv.URL + `"}`)
	out, err := reg.Dispatch(context.Background(), "web_fetch", args)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	// Non-2xx is NOT an error for the tool — the status is reported.
	assert.Equal(t, float64(404), result["status"])
	assert.Equal(t, "not found", result["content"])
}

func TestWebFetchTruncatesLargeResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write slightly more than the max
		big := make([]byte, maxFetchBytes+100)
		for i := range big {
			big[i] = 'x'
		}
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch"}},
	})
	args := json.RawMessage(`{"url":"` + srv.URL + `"}`)
	out, err := reg.Dispatch(context.Background(), "web_fetch", args)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, true, result["truncated"])
	assert.Equal(t, float64(maxFetchBytes), result["content_length"])
}

func TestWebSearchHappyPath(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		body, _ := io.ReadAll(r.Body)
		var req exaSearchRequest
		require.NoError(t, json.Unmarshal(body, &req))
		capturedQuery = req.Query

		resp := exaSearchResponse{
			Results: []exaResult{
				{Title: "Go Lang", URL: "https://go.dev", Text: "Go programming language."},
				{Title: "Effective Go", URL: "https://go.dev/doc/effective_go"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	handler := newWebSearchHandler("test-key", srv.URL)
	out, err := handler(context.Background(), json.RawMessage(`{"query":"golang"}`))
	require.NoError(t, err)

	assert.Equal(t, "golang", capturedQuery)

	var result webSearchResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "golang", result.Query)
	require.Len(t, result.Results, 2)
	assert.Equal(t, "Go Lang", result.Results[0].Title)
}

func TestWebSearchRequiresQuery(t *testing.T) {
	handler := newWebSearchHandler("test-key", "https://x")
	out, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "query")
}

func TestWebSearchRejectsMissingKey(t *testing.T) {
	t.Setenv("EXA_API_KEY", "")
	handler := newWebSearchHandler("", "https://x")
	out, err := handler(context.Background(), json.RawMessage(`{"query":"go"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "EXA_API_KEY")
}

func TestWebExtractHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		var req firecrawlRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "https://example.com", req.URL)
		assert.Contains(t, req.Formats, "markdown")

		resp := firecrawlResponse{Success: true}
		resp.Data.Markdown = "# Hello\n\nWorld."
		resp.Data.Metadata = map[string]any{"title": "Example"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	handler := newWebExtractHandler("test-key", srv.URL)
	out, err := handler(context.Background(), json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var result webExtractResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "markdown", result.Format)
	assert.Contains(t, result.Content, "# Hello")
	assert.Equal(t, "Example", result.Metadata["title"])
}

func TestWebExtractRejectsBadFormat(t *testing.T) {
	handler := newWebExtractHandler("test-key", "https://x")
	out, err := handler(context.Background(), json.RawMessage(`{"url":"https://x","format":"pdf"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "format")
}

func TestWebExtractHandlesFailureResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := firecrawlResponse{Success: false, Error: "rate limited"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	handler := newWebExtractHandler("test-key", srv.URL)
	out, err := handler(context.Background(), json.RawMessage(`{"url":"https://x"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "rate limited")
}
