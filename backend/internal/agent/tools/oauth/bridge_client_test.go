package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/stretchr/testify/require"
)

func TestBridgeClient_HappyPath_ReturnsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
		require.Equal(t, "my-key", reqBody["key"])
		require.Equal(t, "getToken", reqBody["action"])
		require.Equal(t, "hello", reqBody["query"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   map[string]any{"token": "abc123"},
		})
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	data, err := bc.Call(context.Background(), "DEP_ID", "my-key", "getToken", map[string]any{"query": "hello"})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, "abc123", parsed["token"])
}

func TestBridgeClient_ErrorEnvelope_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "quota exceeded",
		})
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(context.Background(), "DEP_ID", "key", "action", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "apps script error: quota exceeded")
}

func TestBridgeClient_HTTPError_PropagatesStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(context.Background(), "DEP_ID", "key", "action", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bridge HTTP 500")
}

func TestBridgeClient_Timeout_ReturnsContextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(ctx, "DEP_ID", "key", "action", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context deadline exceeded")
}

func TestBridgeClient_LargeResponseCapped_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Write more than 4 MiB so LimitReader allows reading maxBridgeRespBytes+1
		_, _ = w.Write(make([]byte, 4<<20+100))
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(context.Background(), "DEP_ID", "key", "action", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bridge response exceeds 4194304 bytes")
}

func TestBridgeClient_NonJSONResponse_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>error</body></html>"))
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(context.Background(), "DEP_ID", "key", "action", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bridge response not JSON")
}

func TestBridgeClient_SetsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "AnythingLLM-Agent-Go/1.0", r.Header.Get("X-AnythingLLM-UA"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": nil})
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(context.Background(), "DEP_ID", "key", "action", nil)
	require.NoError(t, err)
}

func TestBridgeClient_BodyShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))

		require.Contains(t, reqBody, "key")
		require.Contains(t, reqBody, "action")
		require.Equal(t, "my-key", reqBody["key"])
		require.Equal(t, "my-action", reqBody["action"])
		require.Equal(t, "val1", reqBody["extra1"])
		require.Equal(t, "val2", reqBody["extra2"])

		// Ensure no nested params object; everything is at top level.
		require.NotContains(t, reqBody, "params")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": nil})
	}))
	defer srv.Close()
	oauth.SetTestBaseURL(srv.URL)
	defer oauth.SetTestBaseURL("")

	bc := oauth.NewBridgeClient(5 * time.Second)
	_, err := bc.Call(context.Background(), "DEP_ID", "my-key", "my-action", map[string]any{
		"extra1": "val1",
		"extra2": "val2",
	})
	require.NoError(t, err)
}
