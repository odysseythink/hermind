package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"foo/bar", "foo/bar"},
		{"../foo/bar", "foo/bar"},
		{"../../foo/bar", "foo/bar"},
		{"..", ""},
		{".", ""},
		{"/", ""},
		{"  /foo/bar  ", "/foo/bar"},
	}
	for _, tt := range tests {
		got := NormalizePath(tt.input)
		// On Windows separators differ, so compare using filepath.Clean
		assert.Equal(t, filepath.Clean(tt.expected), filepath.Clean(got), "input: %s", tt.input)
	}
}

func TestIsWithin(t *testing.T) {
	assert.True(t, IsWithin("/home/user", "/home/user/docs"))
	assert.False(t, IsWithin("/home/user", "/home/user"))
	assert.False(t, IsWithin("/home/user", "/other/path"))
	assert.False(t, IsWithin("/home/user", "../home/user/docs"))
}

func TestSanitizeFileName(t *testing.T) {
	assert.Equal(t, "hello", SanitizeFileName("<he>llo"))
	assert.Equal(t, "file", SanitizeFileName(`fi\"le`))
	assert.Equal(t, "name", SanitizeFileName(`na\me`))
	assert.Equal(t, "", SanitizeFileName(""))
}

func TestSlugifyFilename(t *testing.T) {
	assert.Equal(t, "hello-world", SlugifyFilename("Hello World"))
	assert.Equal(t, "file-name", SlugifyFilename("file name!"))
}

func TestTrashFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trash-me.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("bye"), 0644))

	TrashFile(tmpFile)
	_, err := os.Stat(tmpFile)
	assert.True(t, os.IsNotExist(err))
}

func TestWriteToServerDocuments(t *testing.T) {
	tmpDir := t.TempDir()
	doc := &core.Document{
		Title:       "Test Doc",
		PageContent: "hello world",
	}

	result, err := WriteToServerDocuments(tmpDir, doc, "my-file", false)
	require.NoError(t, err)
	assert.True(t, result.IsDirectUpload == false)
	assert.Contains(t, result.Location, "custom-documents/my-file.json")

	// Verify file was written
	writtenPath := filepath.Join(tmpDir, "documents", "custom-documents", "my-file.json")
	_, err = os.Stat(writtenPath)
	require.NoError(t, err)
}

func TestCreatedDate(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("hello"), 0644))

	date := CreatedDate(tmpFile)
	assert.NotEqual(t, "unknown", date)
	// Verify format matches "2006-01-02 15:04:05"
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, date)
}

func TestCreatedDate_MissingFile(t *testing.T) {
	assert.Equal(t, "unknown", CreatedDate("/nonexistent/path/file.txt"))
}

func TestWriteToServerDocuments_ParseOnly(t *testing.T) {
	tmpDir := t.TempDir()
	doc := &core.Document{
		Title:       "Test Doc",
		PageContent: "hello world",
	}

	result, err := WriteToServerDocuments(tmpDir, doc, "my-file", true)
	require.NoError(t, err)
	assert.True(t, result.IsDirectUpload)
	assert.Contains(t, result.Location, "direct-uploads/my-file.json")

	writtenPath := filepath.Join(tmpDir, "direct-uploads", "my-file.json")
	_, err = os.Stat(writtenPath)
	require.NoError(t, err)
}
