package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
)

func TestTTS(t *testing.T) {
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Streams: NewMemoryStreamHub(),
	})
	require.NoError(t, err)

	body := strings.NewReader(`{"text":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tts", body)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "audio_url")
}

func TestTTS_EmptyText(t *testing.T) {
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Streams: NewMemoryStreamHub(),
	})
	require.NoError(t, err)

	body := strings.NewReader(`{"text":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tts", body)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
