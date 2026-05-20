package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagerSaveGeneratedFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	buf := []byte("hello world")
	result, err := m.Save("text", "txt", buf, "notes.txt")
	require.NoError(t, err)
	require.NotEmpty(t, result.Filename)
	require.True(t, len(result.Filename) > 40) // type-uuid.ext
	require.Equal(t, "notes.txt", result.DisplayFilename)
	require.Equal(t, int64(11), result.FileSize)
	require.True(t, filepath.IsAbs(result.StoragePath))

	// File should exist
	_, err = os.Stat(result.StoragePath)
	require.NoError(t, err)
}

func TestManagerGetGeneratedFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	buf := []byte("test content")
	saved, _ := m.Save("pdf", "pdf", buf, "report.pdf")

	got, err := m.Get(saved.Filename)
	require.NoError(t, err)
	require.Equal(t, buf, got.Buffer)
	require.Equal(t, saved.StoragePath, got.StoragePath)
}

func TestManagerGetInvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	got, err := m.Get("../../../etc/passwd")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestManagerMimeType(t *testing.T) {
	m := NewManager("")
	require.Equal(t, "application/pdf", m.MimeType("pdf"))
	require.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", m.MimeType("docx"))
	require.Equal(t, "application/vnd.openxmlformats-officedocument.presentationml.presentation", m.MimeType("pptx"))
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", m.MimeType("xlsx"))
	require.Equal(t, "text/plain; charset=utf-8", m.MimeType("txt"))
}
