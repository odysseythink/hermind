package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSystemService_ListLocalFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	os.MkdirAll(filepath.Join(tmpDir, "documents"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "documents", "a.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "documents", "folder1"), 0755)

	files, err := fs.ListLocalFiles("")
	require.NoError(t, err)
	assert.Len(t, files, 2)

	names := make(map[string]string)
	for _, f := range files {
		names[f.Name] = f.Type
	}
	assert.Equal(t, "file", names["a.txt"])
	assert.Equal(t, "folder", names["folder1"])
}

func TestFileSystemService_ListLocalFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	files, err := fs.ListLocalFiles("")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestFileSystemService_CreateAndRemoveFolder(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	err := fs.CreateFolder("test-folder")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "documents", "test-folder"))
	assert.NoError(t, err)

	err = fs.RemoveFolder("test-folder")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "documents", "test-folder"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileSystemService_RemoveDocument(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "documents", "doc.txt"), []byte("content"), 0644)
	err := fs.RemoveDocument("doc.txt")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "documents", "doc.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileSystemService_AcceptedDocumentTypes(t *testing.T) {
	fs := NewFileSystemService("/tmp")
	types := fs.AcceptedDocumentTypes()
	assert.NotEmpty(t, types)
	assert.Equal(t, "text/plain", types[".txt"])
	assert.Equal(t, "application/pdf", types[".pdf"])
}

func TestFileSystemService_DetectMIME(t *testing.T) {
	fs := NewFileSystemService("/tmp")
	assert.True(t, strings.HasPrefix(fs.DetectMIME("file.txt"), "text/plain"))
	assert.Equal(t, "application/pdf", fs.DetectMIME("file.pdf"))
	assert.Equal(t, "image/png", fs.DetectMIME("file.png"))
	assert.True(t, strings.Contains(fs.DetectMIME("file.unknown"), "octet-stream"))
}

func TestFileSystemService_SaveFile(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	path, err := fs.SaveFile("sub", "test.txt", strings.NewReader("hello world"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "documents", "sub", "test.txt"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestFileSystemService_AssetOperations(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	path, err := fs.SaveAsset("test.png", strings.NewReader("fake-image"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "assets", "test.png"), path)

	found, data, size, mime, err := fs.ReadAsset(path)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, int64(10), size)
	assert.Equal(t, "image/png", mime)
	assert.Equal(t, "fake-image", string(data))

	err = fs.RemoveAsset(path)
	require.NoError(t, err)
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestFileSystemService_PfpOperations(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	path, err := fs.SavePfp("avatar.png", strings.NewReader("fake-pfp"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "assets", "pfp", "avatar.png"), path)

	found, data, size, mime, err := fs.ReadAsset(path)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, int64(8), size)
	assert.Equal(t, "image/png", mime)
	assert.Equal(t, "fake-pfp", string(data))
}

func TestFileSystemService_IsWithin(t *testing.T) {
	fs := NewFileSystemService("/tmp")
	assert.True(t, fs.IsWithin("/tmp/storage", "/tmp/storage/documents/a.txt"))
	assert.False(t, fs.IsWithin("/tmp/storage", "/etc/passwd"))
	assert.False(t, fs.IsWithin("/tmp/storage", "/tmp/storage/../etc/passwd"))
}

func TestFileSystemService_RenameAsset(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	_, err := fs.SaveAsset("old.png", strings.NewReader("data"))
	require.NoError(t, err)

	newName, err := fs.RenameAsset("old.png", "new.png")
	require.NoError(t, err)
	assert.Equal(t, "new.png", newName)

	_, err = os.Stat(filepath.Join(tmpDir, "assets", "old.png"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(tmpDir, "assets", "new.png"))
	assert.NoError(t, err)
}
