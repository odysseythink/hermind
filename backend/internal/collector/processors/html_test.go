package processors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLExtractor_Supports(t *testing.T) {
	e := NewHTMLExtractor()
	assert.True(t, e.Supports(".html"))
	assert.False(t, e.Supports(".htm"))
	assert.False(t, e.Supports(".txt"))
}

func TestHTMLExtractor_Extract(t *testing.T) {
	e := NewHTMLExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: "testdata/test.html"})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello World")
	assert.Contains(t, out.Content, "This is a paragraph.")
	assert.NotContains(t, out.Content, "var x = 1")
	assert.NotContains(t, out.Content, "color: black")
}

func TestHTMLExtractor_Extract_Simple(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "simple.html")
	require.NoError(t, os.WriteFile(filePath, []byte("<html><body><p>Hello World</p></body></html>"), 0644))

	e := NewHTMLExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Equal(t, "Hello World", out.Content)
}
