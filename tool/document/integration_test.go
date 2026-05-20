package document

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegration_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateTextFileHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename":  "test",
		"extension": "txt",
		"content":   "Hello, integration test!",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))

	storageFilename := meta["storageFilename"].(string)
	p := filepath.Join(tmpDir, storageFilename)
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "Hello, integration test!", string(data))
}

func TestIntegration_PDF(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreatePDFHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "report",
		"title":    "Test Report",
		"content":  "# Summary\n\nThis is a test PDF.",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "report.pdf", meta["displayFilename"])
	require.True(t, meta["fileSize"].(float64) > 100)

	// Verify file exists
	storageFilename := meta["storageFilename"].(string)
	p := filepath.Join(tmpDir, storageFilename)
	_, err = os.Stat(p)
	require.NoError(t, err)
}

func TestIntegration_Excel(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateExcelHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "data",
		"content":  "| A | B |\n|---|---|\n| 1 | 2 |",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "data.xlsx", meta["displayFilename"])
}
