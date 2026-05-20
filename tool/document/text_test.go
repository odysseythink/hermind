package document

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTextFile(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateTextFileHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename":  "notes",
		"extension": "md",
		"content":   "# Hello\n\nWorld",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "notes.md", meta["displayFilename"])
	require.NotEmpty(t, meta["storageFilename"])
	require.NotEmpty(t, meta["downloadUrl"])

	// File should exist
	storageFilename := meta["storageFilename"].(string)
	p := filepath.Join(tmpDir, storageFilename)
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "# Hello\n\nWorld", string(data))
}
