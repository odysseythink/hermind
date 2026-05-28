package extensions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYouTubeExtension_Name(t *testing.T) {
	y := NewYouTubeExtension()
	assert.Equal(t, "youtube-transcript", y.Name())
}

func TestYouTubeExtension_Handle_UnsupportedMethod(t *testing.T) {
	y := NewYouTubeExtension()
	_, err := y.Handle(context.Background(), "/ext/youtube-transcript", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestYouTubeExtension_Handle_UnknownEndpoint(t *testing.T) {
	y := NewYouTubeExtension()
	_, err := y.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestYouTubeExtension_extractTranscript_InvalidBody(t *testing.T) {
	y := NewYouTubeExtension()
	_, err := y.extractTranscript(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestYouTubeExtension_extractTranscript_MissingURL(t *testing.T) {
	y := NewYouTubeExtension()
	body, _ := json.Marshal(YouTubeRequest{})
	_, err := y.extractTranscript(context.Background(), body)
	assert.Error(t, err)
}

func TestYouTubeExtension_extractTranscript_InvalidURL(t *testing.T) {
	y := NewYouTubeExtension()
	body, _ := json.Marshal(YouTubeRequest{URL: "https://youtube.com/watch"})
	_, err := y.extractTranscript(context.Background(), body)
	assert.Error(t, err)
}
