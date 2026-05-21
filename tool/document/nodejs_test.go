package document

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeJSWrapper_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a mock script
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "bin", "generate-doc.js")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	mockScript := `const fs = require('fs');
const data = JSON.parse(require('fs').readFileSync(0, 'utf8'));
const out = data.outputDir + '/docx-test-12345678-1234-1234-1234-123456789abc.docx';
fs.writeFileSync(out, 'fake docx content');
console.log(out);`
	require.NoError(t, os.WriteFile(scriptPath, []byte(mockScript), 0o755))

	w := NewNodeJSWrapper(scriptDir, tmpDir)
	result, err := w.Generate(context.Background(), "docx", map[string]interface{}{
		"filename": "report.docx",
		"content":  "# Hello",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.True(t, filepath.IsAbs(result))
}

func TestNodeJSWrapper_ScriptNotFound(t *testing.T) {
	w := NewNodeJSWrapper("/nonexistent", "/tmp")
	_, err := w.Generate(context.Background(), "docx", map[string]interface{}{})
	require.Error(t, err)
}

func TestFindNodeExecutable_BundledNextToExe(t *testing.T) {
	tmpDir := t.TempDir()
	nodeName := "node"
	if runtime.GOOS == "windows" {
		nodeName = "node.exe"
	}
	fakeNode := filepath.Join(tmpDir, nodeName)
	require.NoError(t, os.WriteFile(fakeNode, []byte("fake node"), 0o755))

	exePath := filepath.Join(tmpDir, "hermind-desktop.exe")
	result := findNodeExecutableFrom(exePath)
	require.NotEmpty(t, result)
	require.Equal(t, fakeNode, result)
}

func TestFindNodeExecutable_BundledInParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	childDir := filepath.Join(tmpDir, "child")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	nodeName := "node"
	if runtime.GOOS == "windows" {
		nodeName = "node.exe"
	}
	fakeNode := filepath.Join(tmpDir, nodeName)
	require.NoError(t, os.WriteFile(fakeNode, []byte("fake node"), 0o755))

	exePath := filepath.Join(childDir, "hermind-desktop.exe")
	result := findNodeExecutableFrom(exePath)
	require.NotEmpty(t, result)
	require.Equal(t, fakeNode, result)
}

func TestFindNodeExecutable_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "hermind-desktop.exe")
	result := findNodeExecutableFrom(exePath)
	// It may fall back to a real node on PATH, which is fine.
	// Just make sure it does not return a path inside our empty temp dir.
	if result != "" {
		require.NotContains(t, result, tmpDir)
	}
}
