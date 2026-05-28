package processors

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestODF(t *testing.T, dir string, filename string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	z, err := os.Create(path)
	require.NoError(t, err)
	defer z.Close()

	w := zip.NewWriter(z)
	defer w.Close()

	f, err := w.Create("content.xml")
	require.NoError(t, err)
	_, err = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
  <office:body>
    <office:text>
      <text:p>Hello ODF</text:p>
      <text:p>Second paragraph.</text:p>
    </office:text>
  </office:body>
</office:document-content>`))
	require.NoError(t, err)

	return path
}

func TestODFExtractor_Supports(t *testing.T) {
	e := NewODFExtractor()
	assert.True(t, e.Supports(".odt"))
	assert.True(t, e.Supports(".odp"))
	assert.False(t, e.Supports(".docx"))
}

func TestODFExtractor_Extract_ODT(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := makeTestODF(t, tmpDir, "test.odt")

	e := NewODFExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello ODF")
	assert.Contains(t, out.Content, "Second paragraph.")
}

func TestODFExtractor_Extract_ODP(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := makeTestODF(t, tmpDir, "test.odp")

	e := NewODFExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello ODF")
	assert.Contains(t, out.Content, "Second paragraph.")
}
