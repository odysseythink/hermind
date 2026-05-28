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

func makeTestPPTX(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.pptx")
	z, err := os.Create(path)
	require.NoError(t, err)
	defer z.Close()

	w := zip.NewWriter(z)
	defer w.Close()

	f, err := w.Create("[Content_Types].xml")
	require.NoError(t, err)
	_, err = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`))
	require.NoError(t, err)

	f, err = w.Create("ppt/slides/slide1.xml")
	require.NoError(t, err)
	_, err = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:txBody>
          <a:bodyPr/>
          <a:lstStyle/>
          <a:p>
            <a:r>
              <a:t>Hello PPTX</a:t>
            </a:r>
          </a:p>
          <a:p>
            <a:r>
              <a:t>Slide one content.</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`))
	require.NoError(t, err)

	f, err = w.Create("ppt/slides/slide2.xml")
	require.NoError(t, err)
	_, err = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:txBody>
          <a:bodyPr/>
          <a:lstStyle/>
          <a:p>
            <a:r>
              <a:t>Second slide.</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`))
	require.NoError(t, err)

	return path
}

func TestPptxExtractor_Supports(t *testing.T) {
	e := NewPptxExtractor()
	assert.True(t, e.Supports(".pptx"))
	assert.False(t, e.Supports(".ppt"))
}

func TestPptxExtractor_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := makeTestPPTX(t, tmpDir)

	e := NewPptxExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello PPTX")
	assert.Contains(t, out.Content, "Slide one content.")
	assert.Contains(t, out.Content, "Second slide.")
}
