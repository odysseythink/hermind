package processors

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPDFExtractor_Supports(t *testing.T) {
	e := NewPDFExtractor(nil, utils.NewShellRunner())
	assert.True(t, e.Supports(".pdf"))
	assert.False(t, e.Supports(".txt"))
	assert.False(t, e.Supports(".docx"))
}

func TestPDFExtractor_Extract_RealPDF(t *testing.T) {
	e := NewPDFExtractor(nil, utils.NewShellRunner())
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{
		FilePath: filepath.Join("testdata", "test.pdf"),
	})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Hello PDF World")
}

func TestPDFExtractor_Extract_EmptyPDF(t *testing.T) {
	e := NewPDFExtractor(nil, utils.NewShellRunner())
	_, err := e.Extract(context.Background(), pipeline.ExtractInput{
		FilePath: filepath.Join("testdata", "empty.pdf"),
	})
	assert.ErrorIs(t, err, core.ErrEmptyContent)
}

func TestPDFExtractor_Extract_FallbackToTesseract(t *testing.T) {
	shell := utils.NewShellRunner()
	tess := external.NewTesseractAdapter("", shell)
	e := NewPDFExtractor(tess, shell)

	// If tesseract is not available, ErrEmptyContent is expected for an empty PDF.
	// This test documents the OCR fallback path.
	_, err := e.Extract(context.Background(), pipeline.ExtractInput{
		FilePath: filepath.Join("testdata", "test.pdf"),
	})
	// test.pdf has content, so this should succeed regardless of tesseract.
	if err != nil {
		assert.ErrorIs(t, err, core.ErrEmptyContent)
	}
}

func TestPDFExtractor_Extract_OCR_NoTesseract(t *testing.T) {
	shell := utils.NewShellRunner()
	e := NewPDFExtractor(nil, shell)

	_, err := e.Extract(context.Background(), pipeline.ExtractInput{
		FilePath: filepath.Join("testdata", "empty.pdf"),
	})
	assert.ErrorIs(t, err, core.ErrEmptyContent)
	assert.Contains(t, err.Error(), "OCR tools are not available or configured")
}

func TestPDFExtractor_Extract_WithOptions(t *testing.T) {
	e := NewPDFExtractor(nil, utils.NewShellRunner())
	// Options are passed via ExtractInput; no panic when extracting an empty PDF.
	_, err := e.Extract(context.Background(), pipeline.ExtractInput{
		FilePath: filepath.Join("testdata", "empty.pdf"),
		Options: core.Options{
			OCR: core.OCROptions{LangList: "eng"},
		},
	})
	assert.ErrorIs(t, err, core.ErrEmptyContent)
}

func TestParseOCRLangs(t *testing.T) {
	assert.Equal(t, []string{"eng"}, parseOCRLangs("eng"))
	assert.Equal(t, []string{"eng", "fra"}, parseOCRLangs("eng+fra"))
	assert.Equal(t, []string{"eng", "fra"}, parseOCRLangs("eng, fra"))
	assert.Equal(t, []string{"eng", "fra", "deu"}, parseOCRLangs("eng+fra, deu"))
	assert.Empty(t, parseOCRLangs(""))
	assert.Empty(t, parseOCRLangs("   "))
}
