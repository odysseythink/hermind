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

func TestElevenLabs_HappyPath_ReturnsMPEG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("xi-api-key"))
		require.Contains(t, r.URL.Path, "/v1/text-to-speech/voice-abc")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "hello world", body["text"])
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte{0xFF, 0xFB, 0x90, 0x00}) // fake MP3 header
	}))
	defer srv.Close()

	cfg := &config.Config{
		ElevenLabsAPIKey:  "test-key",
		ElevenLabsVoiceID: "voice-abc",
		ElevenLabsModel:   "eleven_monolingual_v1",
	}
	p := NewElevenLabsProvider(cfg, nil)
	SetTestElevenLabsBaseURL(srv.URL)
	defer SetTestElevenLabsBaseURL("")

	out, err := p.Synthesize(context.Background(), "hello world")
	require.NoError(t, err)
	require.Equal(t, "audio/mpeg", out.ContentType)
	require.NotEmpty(t, out.Audio)
}

func TestElevenLabs_NoAPIKey_NotAvailable(t *testing.T) {
	p := NewElevenLabsProvider(&config.Config{}, nil)
	require.False(t, p.Available())
}

func TestElevenLabs_HTTPError_Returns5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		ElevenLabsAPIKey:  "test-key",
		ElevenLabsVoiceID: "voice-abc",
	}
	p := NewElevenLabsProvider(cfg, nil)
	SetTestElevenLabsBaseURL(srv.URL)
	defer SetTestElevenLabsBaseURL("")

	_, err := p.Synthesize(context.Background(), "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 500")
}

func TestElevenLabs_RequestBody_ContainsTextAndModel(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte{0xFF})
	}))
	defer srv.Close()

	cfg := &config.Config{
		ElevenLabsAPIKey:  "k",
		ElevenLabsVoiceID: "v",
		ElevenLabsModel:   "eleven_multilingual_v2",
	}
	p := NewElevenLabsProvider(cfg, nil)
	SetTestElevenLabsBaseURL(srv.URL)
	defer SetTestElevenLabsBaseURL("")

	_, _ = p.Synthesize(context.Background(), "test text")
	require.Equal(t, "test text", captured["text"])
	require.Equal(t, "eleven_multilingual_v2", captured["model_id"])
}

func TestElevenLabs_RequestHeaders_XAPIKey(t *testing.T) {
	var header string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("xi-api-key")
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte{0xFF})
	}))
	defer srv.Close()

	cfg := &config.Config{
		ElevenLabsAPIKey:  "secret-123",
		ElevenLabsVoiceID: "v",
	}
	p := NewElevenLabsProvider(cfg, nil)
	SetTestElevenLabsBaseURL(srv.URL)
	defer SetTestElevenLabsBaseURL("")

	_, _ = p.Synthesize(context.Background(), "x")
	require.Equal(t, "secret-123", header)
}

func TestElevenLabs_LargeResponse_CappedAt50MiB(t *testing.T) {
	big := make([]byte, 51<<20) // 51 MiB
	for i := range big {
		big[i] = byte(i % 256)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(big)
	}))
	defer srv.Close()

	cfg := &config.Config{
		ElevenLabsAPIKey:  "k",
		ElevenLabsVoiceID: "v",
	}
	p := NewElevenLabsProvider(cfg, nil)
	SetTestElevenLabsBaseURL(srv.URL)
	defer SetTestElevenLabsBaseURL("")

	out, err := p.Synthesize(context.Background(), "x")
	require.NoError(t, err)
	require.LessOrEqual(t, len(out.Audio), 50<<20)
}

