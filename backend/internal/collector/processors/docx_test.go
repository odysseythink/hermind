package processors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fumiama/go-docx"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestDocx(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.docx")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	d := docx.New()
	p := d.AddParagraph()
	p.AddText("Hello Docx")
	p = d.AddParagraph()
	p.AddText("Second paragraph.")

	_, err = d.WriteTo(file)
	require.NoError(t, err)
	return path
}

func TestDocxExtractor_Supports(t *testing.T) {
	e := NewDocxExtractor()
	assert.True(t, e.Supports(".docx"))
	assert.False(t, e.Supports(".odt"))
}

func TestDocxExtractor_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := makeTestDocx(t, tmpDir)

	e := NewDocxExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello Docx")
	assert.Contains(t, out.Content, "Second paragraph.")
}

func TestDocxExtractor_Extract_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.docx")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	d := docx.New()
	_, err = d.WriteTo(file)
	require.NoError(t, err)

	e := NewDocxExtractor()
	_, err = e.Extract(context.Background(), pipeline.ExtractInput{FilePath: path})
	require.ErrorIs(t, err, core.ErrEmptyContent)
}
