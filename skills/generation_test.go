package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComputeLibraryHashEmptyDir(t *testing.T) {
	dir := t.TempDir()
	h, err := computeLibraryHash(dir)
	require.NoError(t, err)
	require.NotEmpty(t, h, "even an empty dir produces a deterministic hash")
}

func TestComputeLibraryHashDeterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("world"), 0o644))
	h1, err := computeLibraryHash(dir)
	require.NoError(t, err)
	h2, err := computeLibraryHash(dir)
	require.NoError(t, err)
	require.Equal(t, h1, h2)
}

func TestComputeLibraryHashContentSensitive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))
	h1, _ := computeLibraryHash(dir)
	require.NoError(t, os.WriteFile(p, []byte("hello!"), 0o644))
	h2, _ := computeLibraryHash(dir)
	require.NotEqual(t, h1, h2, "1-byte content change must change hash")
}

func TestComputeLibraryHashMtimeInvariant(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))
	h1, _ := computeLibraryHash(dir)
	// touch — change mtime without changing content
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(p, future, future))
	h2, _ := computeLibraryHash(dir)
	require.Equal(t, h1, h2, "mtime change must not affect hash")
}

func TestComputeLibraryHashOnlyMd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o644))
	h1, _ := computeLibraryHash(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("y"), 0o644))
	h2, _ := computeLibraryHash(dir)
	require.Equal(t, h1, h2, "non-.md files must be ignored")
}
