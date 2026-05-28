package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGeneric_CustomEndpoint_OverridesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/audio/speech", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "custom-model", body["model"])
		w.Header().Set("Content-Type", "audio/mp3")
		w.Write([]byte{0xFF})
	}))
	defer srv.Close()

	cfg := &config.Config{
		TTSOpenAICompatEndpoint: srv.URL,
		TTSOpenAICompatKey:      "key-123",
		TTSOpenAICompatModel:    "custom-model",
		TTSOpenAICompatVoice:    "echo",
	}
	p := NewOpenAIGenericProvider(cfg, nil)

	out, err := p.Synthesize(context.Background(), "hello")
	require.NoError(t, err)
	require.NotEmpty(t, out.Audio)
}

func TestOpenAIGeneric_NoEndpoint_NotAvailable(t *testing.T) {
	p := NewOpenAIGenericProvider(&config.Config{}, nil)
	require.False(t, p.Available())
}

func TestOpenAIGeneric_RequestHeaders_Authorization(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte{0xFF})
	}))
	defer srv.Close()

	cfg := &config.Config{
		TTSOpenAICompatEndpoint: srv.URL,
		TTSOpenAICompatKey:      "bearer-key",
	}
	p := NewOpenAIGenericProvider(cfg, nil)
	SetTestOpenAIGenericBaseURL(srv.URL)
	defer SetTestOpenAIGenericBaseURL("")

	_, _ = p.Synthesize(context.Background(), "x")
	require.Contains(t, authHeader, "bearer-key")
}
