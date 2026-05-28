package processors

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func makeTestXlsx(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.xlsx")
	f := excelize.NewFile()
	defer f.Close()

	// Sheet1
	f.SetCellValue("Sheet1", "A1", "Name")
	f.SetCellValue("Sheet1", "B1", "Age")
	f.SetCellValue("Sheet1", "A2", "Alice")
	f.SetCellValue("Sheet1", "B2", "30")
	f.SetCellValue("Sheet1", "A3", "Bob")
	f.SetCellValue("Sheet1", "B3", "25")

	// Add a second sheet
	f.NewSheet("Sheet2")
	f.SetCellValue("Sheet2", "A1", "City")
	f.SetCellValue("Sheet2", "B1", "Country")
	f.SetCellValue("Sheet2", "A2", "Paris")
	f.SetCellValue("Sheet2", "B2", "France")

	require.NoError(t, f.SaveAs(path))
	return path
}

func TestXlsxExtractor_Supports(t *testing.T) {
	e := NewXlsxExtractor()
	assert.True(t, e.Supports(".xlsx"))
	assert.False(t, e.Supports(".csv"))
}

func TestXlsxExtractor_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := makeTestXlsx(t, tmpDir)

	e := NewXlsxExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "Name\tAge")
	assert.Contains(t, out.Content, "Alice\t30")
	assert.Contains(t, out.Content, "Bob\t25")
	assert.Contains(t, out.Content, "City\tCountry")
	assert.Contains(t, out.Content, "Paris\tFrance")
}
