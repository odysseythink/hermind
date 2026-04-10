// provider/openrouter/openrouter_test.go
package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "openrouter"})
	assert.Error(t, err)
}

func TestSendsRoutingHeaders(t *testing.T) {
	var gotReferer, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		body, _ := io.ReadAll(r.Body)
		_ = body

		resp := map[string]any{
			"id":    "c1",
			"model": "openai/gpt-4o",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "hi"},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "openrouter",
		APIKey:   "or-test",
		BaseURL:  srv.URL,
		Model:    "openai/gpt-4o",
	})
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/nousresearch/hermes-agent", gotReferer)
	assert.Equal(t, "hermes-agent", gotTitle)
}
