package processors

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageExtractor_Supports(t *testing.T) {
	e := NewImageExtractor(nil)
	assert.True(t, e.Supports(".png"))
	assert.True(t, e.Supports(".jpg"))
	assert.True(t, e.Supports(".jpeg"))
	assert.True(t, e.Supports(".webp"))
	assert.False(t, e.Supports(".gif"))
	assert.False(t, e.Supports(".pdf"))
}

func TestImageExtractor_Extract_NoTesseract(t *testing.T) {
	e := NewImageExtractor(nil)
	_, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: "test.png"})
	assert.ErrorIs(t, err, core.ErrOCRFailed)
}

func TestImageExtractor_Extract_NotInstalled(t *testing.T) {
	shell := utils.NewShellRunner()
	tess := external.NewTesseractAdapter("", shell)
	e := NewImageExtractor(tess)

	if tess.Available() {
		t.Skip("tesseract is installed; skipping not-installed test")
	}

	_, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: "test.png"})
	assert.ErrorIs(t, err, core.ErrOCRFailed)
}

func TestImageExtractor_ResolveLangs(t *testing.T) {
	e := NewImageExtractor(nil)

	langs := e.resolveLangs(pipeline.ExtractInput{
		Options: core.Options{OCR: core.OCROptions{LangList: "fra"}},
	})
	assert.Equal(t, []string{"fra"}, langs)

	langs = e.resolveLangs(pipeline.ExtractInput{
		Options: core.Options{},
	})
	assert.Equal(t, []string{"eng"}, langs)
}

func TestImageExtractor_Extract_WithImage(t *testing.T) {
	shell := utils.NewShellRunner()
	tess := external.NewTesseractAdapter("", shell)
	e := NewImageExtractor(tess)

	if !tess.Available() {
		t.Skip("tesseract is not installed; skipping integration test")
	}

	// Create a simple PNG image.
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	img := image.NewRGBA(image.Rect(0, 0, 100, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, image.White)
		}
	}
	f, err := os.Create(imgPath)
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, img))
	require.NoError(t, f.Close())

	_, err = e.Extract(context.Background(), pipeline.ExtractInput{FilePath: imgPath})
	// Tesseract on a blank white image may return empty text; that's acceptable.
	if err != nil {
		assert.ErrorIs(t, err, core.ErrOCRFailed)
	}
}
