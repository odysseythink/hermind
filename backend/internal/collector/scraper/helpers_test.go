package scraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsYouTubeURL(t *testing.T) {
	assert.True(t, isYouTubeURL("https://www.youtube.com/watch?v=dQw4w9WgXcQ"))
	assert.True(t, isYouTubeURL("https://youtu.be/dQw4w9WgXcQ"))
	assert.True(t, isYouTubeURL("https://youtube.com/watch?v=abc123"))
	assert.False(t, isYouTubeURL("https://example.com"))
	assert.False(t, isYouTubeURL("not-a-url"))
}

func TestValidateURL(t *testing.T) {
	assert.Equal(t, "https://example.com", validateURL("example.com"))
	assert.Equal(t, "https://example.com", validateURL("https://example.com"))
	assert.Equal(t, "http://example.com", validateURL("http://example.com"))
	assert.Equal(t, "https://example.com/path", validateURL("example.com/path"))
}

func TestValidURL(t *testing.T) {
	assert.True(t, validURL("https://example.com"))
	assert.True(t, validURL("http://example.com"))
	assert.False(t, validURL("ftp://example.com"))
	assert.False(t, validURL("example.com"))
	assert.False(t, validURL(""))
	assert.False(t, validURL("not-a-url"))
}
