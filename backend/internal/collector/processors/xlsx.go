package processors

import (
	"context"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/xuri/excelize/v2"
)

// XlsxExtractor extracts text from .xlsx files.
type XlsxExtractor struct{}

// NewXlsxExtractor creates a new XlsxExtractor.
func NewXlsxExtractor() *XlsxExtractor {
	return &XlsxExtractor{}
}

// Supports returns true for the .xlsx extension.
func (e *XlsxExtractor) Supports(ext string) bool {
	return ext == ".xlsx"
}

// Extract opens the XLSX file, reads all sheets and cells,
// and formats them as tab-separated columns, newline-separated rows.
func (e *XlsxExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	f, err := excelize.OpenFile(input.FilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf strings.Builder

	sheets := f.GetSheetList()
	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}
		for _, row := range rows {
			for i, cell := range row {
				if i > 0 {
					buf.WriteString("\t")
				}
				buf.WriteString(cell)
			}
			buf.WriteString("\n")
		}
	}

	return &pipeline.ExtractOutput{
		Content: strings.TrimSpace(buf.String()),
	}, nil
}
