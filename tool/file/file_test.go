package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() *tool.Registry {
	r := tool.NewRegistry()
	RegisterAll(r)
	return r
}

func setupTestConfig(allowedDir string) {
	SetCurrentConfig(map[string]any{
		"allowed_directories": allowedDir,
	})
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestReadFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": path})
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "hello world", decoded["content"])
}

func TestReadFileMissing(t *testing.T) {
	setupTestConfig("/tmp")
	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": "/nonexistent/path/x.txt"})
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestReadFileRejectsEmptyPath(t *testing.T) {
	setupTestConfig("/tmp")
	r := newTestRegistry()
	args := json.RawMessage(`{}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "path")
}

func TestListDirectoryHappyPath(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": dir})
	out, err := r.Dispatch(context.Background(), "list_directory", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	entries, ok := decoded["entries"].([]any)
	require.True(t, ok, "entries should be an array")
	assert.Len(t, entries, 3)

	names := map[string]bool{}
	for _, e := range entries {
		m := e.(map[string]any)
		names[m["name"].(string)] = true
	}
	assert.True(t, names["a.txt"])
	assert.True(t, names["b.txt"])
	assert.True(t, names["sub"])
}

func TestListDirectoryMissing(t *testing.T) {
	setupTestConfig("/tmp")
	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": "/nonexistent/dir"})
	out, err := r.Dispatch(context.Background(), "list_directory", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestWriteFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	path := filepath.Join(dir, "new.txt")

	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": path, "content": "written"})
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, float64(7), decoded["bytes_written"])

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "written", string(data))
}

func TestWriteFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	path := filepath.Join(dir, "a", "b", "deep.txt")
	r := newTestRegistry()
	args := mustJSON(t, map[string]any{"path": path, "content": "deep", "create_dirs": true})
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.NotContains(t, out, `"error"`)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "deep", string(data))
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	setupTestConfig("/tmp")
	r := newTestRegistry()
	args := json.RawMessage(`{"content":"nothing"}`)
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestSearchFilesHappyPath(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644))

	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"root": dir, "pattern": "*.go"})
	out, err := r.Dispatch(context.Background(), "search_files", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	matches, ok := decoded["matches"].([]any)
	require.True(t, ok)
	assert.Len(t, matches, 2)
}

func TestSearchFilesEmptyResults(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"root": dir, "pattern": "*.nope"})
	out, err := r.Dispatch(context.Background(), "search_files", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	matches, _ := decoded["matches"].([]any)
	assert.Len(t, matches, 0)
}

func TestReadFileDirectory(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": dir})
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "directory")
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	path := filepath.Join(dir, "overwrite.txt")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o644))

	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": path, "content": "updated"})
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.NotContains(t, out, `"error"`)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "updated", string(data))
}

func TestSearchFilesInvalidPattern(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644))
	r := newTestRegistry()
	// [] is an empty character class, which filepath.Match rejects
	args := mustJSON(t, map[string]string{"root": dir, "pattern": "[]"})
	out, err := r.Dispatch(context.Background(), "search_files", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestListDirectoryEmpty(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": dir})
	out, err := r.Dispatch(context.Background(), "list_directory", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	entries, _ := decoded["entries"].([]any)
	assert.Len(t, entries, 0)
}

func TestReadFileLargeFile(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	path := filepath.Join(dir, "big.txt")
	big := make([]byte, 1<<20+1)
	for i := range big {
		big[i] = 'A'
	}
	require.NoError(t, os.WriteFile(path, big, 0o644))

	r := newTestRegistry()
	args := mustJSON(t, map[string]string{"path": path})
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "too large")
}

func TestReadFileHeadAndTail(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(dir)
	path := filepath.Join(dir, "lines.txt")
	content := strings.Join([]string{"line1", "line2", "line3", "line4", "line5"}, "\n")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	r := newTestRegistry()

	// Test head
	head := 2
	args := mustJSON(t, map[string]any{"path": path, "head": head})
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, "line1")
	assert.Contains(t, out, "line2")
	assert.NotContains(t, out, "line3")

	// Test tail
	tail := 2
	args = mustJSON(t, map[string]any{"path": path, "tail": tail})
	out, err = r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, "line4")
	assert.Contains(t, out, "line5")
	assert.NotContains(t, out, "line1")

	// Test both head and tail rejected
	args = mustJSON(t, map[string]any{"path": path, "head": 1, "tail": 1})
	out, err = r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, "cannot specify both head and tail")
}
