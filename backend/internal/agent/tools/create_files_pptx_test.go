package tools

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unidoc/unioffice/common/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	if key := os.Getenv("UNIDOC_METERED_KEY"); key != "" {
		_ = license.SetMeteredKey(key)
	}
}

func skipIfUnlicensed(t *testing.T) {
	if !license.GetLicenseKey().IsLicensed() {
		t.Skip("unioffice license required — set UNIDOC_METERED_KEY to run this test")
	}
}

func TestWritePptxFile_SingleSlide(t *testing.T) {
	skipIfUnlicensed(t)
	dst := filepath.Join(t.TempDir(), "single.pptx")
	err := writePptxFile(context.Background(), dst, "Hello world\nBody line one", "single")
	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(1024)) // pptx zips are >1KB

	r, err := zip.OpenReader(dst)
	require.NoError(t, err)
	defer r.Close()
	var slideCount int
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
		}
	}
	assert.Equal(t, 1, slideCount)
}

func TestWritePptxFile_ThreeSlides(t *testing.T) {
	skipIfUnlicensed(t)
	dst := filepath.Join(t.TempDir(), "three.pptx")
	body := "Slide one\nBody A\n---\nSlide two\nBody B\n---\nSlide three"
	err := writePptxFile(context.Background(), dst, body, "three")
	require.NoError(t, err)

	r, err := zip.OpenReader(dst)
	require.NoError(t, err)
	defer r.Close()
	var slideCount int
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
		}
	}
	assert.Equal(t, 3, slideCount)
}

func TestWritePptxFile_EmptyContent(t *testing.T) {
	skipIfUnlicensed(t)
	dst := filepath.Join(t.TempDir(), "empty.pptx")
	err := writePptxFile(context.Background(), dst, "", "empty")
	require.NoError(t, err)
	_, err = os.Stat(dst)
	assert.NoError(t, err)
}
