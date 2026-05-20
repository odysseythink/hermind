package document

import (
	"context"
	"os"
	"path/filepath"
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
