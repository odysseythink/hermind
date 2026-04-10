// provider/zhipu/zhipu_test.go
package zhipu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "zhipu"})
	assert.Error(t, err)
}

func TestNewRejectsMalformedKey(t *testing.T) {
	_, err := New(config.ProviderConfig{
		Provider: "zhipu",
		APIKey:   "no_dot",
		Model:    "glm-4",
	})
	assert.Error(t, err)
}

func TestSendsJWTBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = body

		resp := map[string]any{
			"id":    "c1",
			"model": "glm-4",
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
		Provider: "zhipu",
		APIKey:   "my_key.my_secret",
		BaseURL:  srv.URL,
		Model:    "glm-4",
	})
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(gotAuth, "Bearer "), "auth header should be Bearer ...")
	token := strings.TrimPrefix(gotAuth, "Bearer ")
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3, "JWT should have 3 parts")
}
