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

func TestTxtExtractor_Supports(t *testing.T) {
	e := NewTxtExtractor()
	assert.True(t, e.Supports(".txt"))
	assert.True(t, e.Supports(".md"))
	assert.True(t, e.Supports(".org"))
	assert.True(t, e.Supports(".adoc"))
	assert.True(t, e.Supports(".rst"))
	assert.True(t, e.Supports(".csv"))
	assert.True(t, e.Supports(".json"))
	assert.False(t, e.Supports(".html"))
	assert.False(t, e.Supports(".docx"))
}

func TestTxtExtractor_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

	e := NewTxtExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Equal(t, "hello world", out.Content)
}

func TestTxtExtractor_Extract_Markdown(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.md")
	require.NoError(t, os.WriteFile(filePath, []byte("# Hello\n\nWorld"), 0644))

	e := NewTxtExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Equal(t, "# Hello\n\nWorld", out.Content)
}
