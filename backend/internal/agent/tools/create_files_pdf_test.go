package tools

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPDF_HappyPath_ASCII(t *testing.T) {
	dst := t.TempDir() + "/test.pdf"
	err := writePDFFile(t.Context(), dst, "Hello world", "Title")
	require.NoError(t, err)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.True(t, len(data) > 4)
	require.Equal(t, "%PDF-", string(data[:5]))
}

func TestPDF_NonASCII_ReturnsError(t *testing.T) {
	dst := t.TempDir() + "/fail.pdf"
	err := writePDFFile(t.Context(), dst, "Hello 世界", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-ASCII")
	_, statErr := os.Stat(dst)
	require.True(t, os.IsNotExist(statErr))
}

func TestPDF_MultiLine_Wraps(t *testing.T) {
	dst := t.TempDir() + "/multi.pdf"
	content := "Line one\nLine two\nLine three"
	err := writePDFFile(t.Context(), dst, content, "")
	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

func TestPDF_ReturnsValidPDFMagic(t *testing.T) {
	dst := t.TempDir() + "/magic.pdf"
	err := writePDFFile(t.Context(), dst, "test content", "")
	require.NoError(t, err)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 5)
	require.Equal(t, "%PDF-", string(data[:5]))
}

func TestPDF_LongContent_Paginates(t *testing.T) {
	dst := t.TempDir() + "/long.pdf"
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("This is a long line of text that should wrap or cause pagination in the PDF document. ")
	}
	err := writePDFFile(t.Context(), dst, b.String(), "")
	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}
