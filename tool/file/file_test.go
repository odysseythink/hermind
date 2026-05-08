package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestReadFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + path + `"}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "hello world", decoded["content"])
}

func TestReadFileMissing(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{"path":"/nonexistent/path/x.txt"}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "no such file")
}

func TestReadFileRejectsEmptyPath(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "path")
}

func TestListDirectoryHappyPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + dir + `"}`)
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
	r := newTestRegistry()
	args := json.RawMessage(`{"path":"/nonexistent/dir"}`)
	out, err := r.Dispatch(context.Background(), "list_directory", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestWriteFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + path + `","content":"written"}`)
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
	path := filepath.Join(dir, "a", "b", "deep.txt")
	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + path + `","content":"deep","create_dirs":true}`)
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.NotContains(t, out, `"error"`)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "deep", string(data))
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{"content":"nothing"}`)
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestSearchFilesHappyPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644))

	r := newTestRegistry()
	args := json.RawMessage(`{"root":"` + dir + `","pattern":"*.go"}`)
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
	r := newTestRegistry()
	args := json.RawMessage(`{"root":"` + dir + `","pattern":"*.nope"}`)
	out, err := r.Dispatch(context.Background(), "search_files", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	matches, _ := decoded["matches"].([]any)
	assert.Len(t, matches, 0)
}
