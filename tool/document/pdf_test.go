package document

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreatePDF(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreatePDFHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "report",
		"title":    "Annual Report",
		"content":  "# Introduction\n\nThis is the annual report.\n\n## Section 1\n\nSome details here.",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "report.pdf", meta["displayFilename"])
	require.True(t, meta["fileSize"].(float64) > 100)
}
