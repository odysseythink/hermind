package document

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestCreateExcelFile(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateExcelHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "report",
		"title":    "Q1 Report",
		"content":  "## Sheet1\n\n| Name | Value |\n|------|-------|\n| A    | 10    |\n| B    | 20    |",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "report.xlsx", meta["displayFilename"])

	// Open and verify
	storageFilename := meta["storageFilename"].(string)
	f, err := excelize.OpenFile(tmpDir + "/" + storageFilename)
	require.NoError(t, err)
	defer f.Close()

	val, _ := f.GetCellValue("Sheet1", "A1")
	require.Equal(t, "Name", val)
}
