package tools

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocx_HappyPath_WritesFile(t *testing.T) {
	dst := t.TempDir() + "/test.docx"
	err := writeDocxFile(t.Context(), dst, "Hello world", "Title")
	require.NoError(t, err)
	require.FileExists(t, dst)
}

func TestDocx_MultiParagraph_Content(t *testing.T) {
	dst := t.TempDir() + "/multi.docx"
	err := writeDocxFile(t.Context(), dst, "Para one.\n\nPara two.", "")
	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

func TestDocx_ReturnsValidZIP(t *testing.T) {
	dst := t.TempDir() + "/zip.docx"
	err := writeDocxFile(t.Context(), dst, "test", "")
	require.NoError(t, err)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 4)
	require.Equal(t, []byte("PK\x03\x04"), data[:4])
}

func TestDocx_WithTitle(t *testing.T) {
	dst := t.TempDir() + "/titled.docx"
	err := writeDocxFile(t.Context(), dst, "body text", "My Title")
	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}
