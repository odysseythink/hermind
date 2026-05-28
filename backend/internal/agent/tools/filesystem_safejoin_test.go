package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeJoin_NormalPath_ReturnsAbsolute(t *testing.T) {
	root := t.TempDir()
	result, err := safeJoin(root, "subdir/file.txt")
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(result))
	require.Contains(t, result, "subdir")
}

func TestSafeJoin_ParentTraversal_Rejected(t *testing.T) {
	root := t.TempDir()
	_, err := safeJoin(root, "../etc/passwd")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPathEscape)
}

func TestSafeJoin_AbsolutePathInUserInput_Rejected(t *testing.T) {
	root := t.TempDir()
	_, err := safeJoin(root, "/etc/passwd")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPathEscape)
}

func TestSafeJoin_SymlinkEscape_Rejected(t *testing.T) {
	root := t.TempDir()
	// Create an "outside" directory with a file inside it
	outside := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.MkdirAll(outside, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0644))

	// Create a symlink inside root that points outside
	linkPath := filepath.Join(root, "escape")
	require.NoError(t, os.Symlink(outside, linkPath))

	_, err := safeJoin(root, "escape/secret.txt")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPathEscape)
}

func TestSafeJoin_NestedPath_Allowed(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(inner, 0755))
	result, err := safeJoin(root, "a/b")
	require.NoError(t, err)
	expected, _ := filepath.EvalSymlinks(inner)
	require.Equal(t, expected, result)
}

func TestSafeJoin_EmptyPath_AllowedAsRoot(t *testing.T) {
	root := t.TempDir()
	result, err := safeJoin(root, "")
	require.NoError(t, err)
	absRoot, _ := filepath.Abs(root)
	expected, _ := filepath.EvalSymlinks(absRoot)
	if expected == "" {
		expected = absRoot
	}
	require.Equal(t, expected, result)
}
