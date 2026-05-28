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

func TestOpenAITTS_HappyPath_ReturnsMPEG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/audio/speech", r.URL.Path)
		auth := r.Header.Get("Authorization")
		require.Contains(t, auth, "Bearer sk-test")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "hello", body["input"])
		require.Equal(t, "tts-1", body["model"])
		require.Equal(t, "alloy", body["voice"])
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte{0xFF, 0xFB})
	}))
	defer srv.Close()

	cfg := &config.Config{
		OpenAiKey:      "sk-test",
		OpenAITTSModel: "tts-1",
		OpenAITTSVoice: "alloy",
	}
	p := NewOpenAIProvider(cfg, nil)
	SetTestOpenAITTSBaseURL(srv.URL)
	defer SetTestOpenAITTSBaseURL("")

	out, err := p.Synthesize(context.Background(), "hello")
	require.NoError(t, err)
	require.NotEmpty(t, out.Audio)
}

func TestOpenAITTS_NoAPIKey_NotAvailable(t *testing.T) {
	p := NewOpenAIProvider(&config.Config{}, nil)
	require.False(t, p.Available())
}

func TestOpenAITTS_RequestShape(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte{0xFF})
	}))
	defer srv.Close()

	cfg := &config.Config{
		OpenAiKey:      "sk-test",
		OpenAITTSModel: "tts-1-hd",
		OpenAITTSVoice: "nova",
	}
	p := NewOpenAIProvider(cfg, nil)
	SetTestOpenAITTSBaseURL(srv.URL)
	defer SetTestOpenAITTSBaseURL("")

	_, _ = p.Synthesize(context.Background(), "say this")
	require.Equal(t, "tts-1-hd", captured["model"])
	require.Equal(t, "say this", captured["input"])
	require.Equal(t, "nova", captured["voice"])
}
