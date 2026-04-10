// provider/wenxin/oauth_test.go
package wenxin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAccessTokenHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/2.0/token", r.URL.Path)
		assert.Equal(t, "client_credentials", r.URL.Query().Get("grant_type"))
		assert.Equal(t, "my_api_key", r.URL.Query().Get("client_id"))
		assert.Equal(t, "my_secret", r.URL.Query().Get("client_secret"))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test_token_xyz",
			"expires_in":   2592000,
		})
	}))
	defer srv.Close()

	w := &Wenxin{
		apiKey:    "my_api_key",
		secretKey: "my_secret",
		oauthURL:  srv.URL + "/oauth/2.0/token",
		http:      &http.Client{},
	}

	token, err := w.fetchAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test_token_xyz", token)
}

func TestGetAccessTokenCachesUntilExpiry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(wr).Encode(map[string]any{
			"access_token": "cached_token",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	w := &Wenxin{
		apiKey:    "my_api_key",
		secretKey: "my_secret",
		oauthURL:  srv.URL + "/oauth/2.0/token",
		http:      &http.Client{},
	}

	// First call hits the server
	tok1, err := w.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached_token", tok1)
	assert.Equal(t, 1, calls)

	// Second call uses cache
	tok2, err := w.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached_token", tok2)
	assert.Equal(t, 1, calls, "should not re-fetch cached token")

	// Manually expire the cache and verify re-fetch
	w.tokenMu.Lock()
	w.tokenExp = time.Now().Add(-time.Minute)
	w.tokenMu.Unlock()

	tok3, err := w.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached_token", tok3)
	assert.Equal(t, 2, calls, "should re-fetch after expiry")
}

func TestGetAccessTokenHandlesOAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(wr).Encode(map[string]any{
			"error":             "invalid_client",
			"error_description": "unknown client id",
		})
	}))
	defer srv.Close()

	w := &Wenxin{
		apiKey:    "bad",
		secretKey: "bad",
		oauthURL:  srv.URL + "/oauth/2.0/token",
		http:      &http.Client{},
	}
	_, err := w.getAccessToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_client")
}
