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

func makeTestEPUB(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.epub")
	z, err := os.Create(path)
	require.NoError(t, err)
	defer z.Close()

	w := zip.NewWriter(z)
	defer w.Close()

	f, err := w.Create("mimetype")
	require.NoError(t, err)
	_, err = f.Write([]byte("application/epub+zip"))
	require.NoError(t, err)

	f, err = w.Create("OEBPS/content.opf")
	require.NoError(t, err)
	_, err = f.Write([]byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <manifest><item id="p1" href="page1.xhtml" media-type="application/xhtml+xml"/></manifest>
</package>`))
	require.NoError(t, err)

	f, err = w.Create("OEBPS/page1.xhtml")
	require.NoError(t, err)
	_, err = f.Write([]byte(`<html xmlns="http://www.w3.org/1999/xhtml">
<body><h1>Hello EPUB</h1><p>Paragraph one.</p><p>Paragraph two.</p></body>
</html>`))
	require.NoError(t, err)

	return path
}

func TestEPubExtractor_Supports(t *testing.T) {
	e := NewEPubExtractor()
	assert.True(t, e.Supports(".epub"))
	assert.False(t, e.Supports(".txt"))
}

func TestEPubExtractor_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := makeTestEPUB(t, tmpDir)

	e := NewEPubExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello EPUB")
	assert.Contains(t, out.Content, "Paragraph one.")
	assert.Contains(t, out.Content, "Paragraph two.")
}
