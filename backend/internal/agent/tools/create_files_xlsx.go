package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xuri/excelize/v2"
)

type xlsxContent struct {
	Sheets []xlsxSheet `json:"sheets"`
}

type xlsxSheet struct {
	Name string     `json:"name"`
	Rows [][]string `json:"rows"`
}

func writeXLSXFile(ctx context.Context, dst string, contentRaw any) error {
	blob, err := json.Marshal(contentRaw)
	if err != nil {
		return fmt.Errorf("content marshal: %w", err)
	}
	var c xlsxContent
	if err := json.Unmarshal(blob, &c); err != nil {
		return fmt.Errorf("content must have shape {sheets: [{name, rows}]}: %w", err)
	}
	if len(c.Sheets) == 0 {
		return fmt.Errorf("at least one sheet required")
	}

	f := excelize.NewFile()
	defer f.Close()

	defaultIdx, _ := f.GetSheetIndex("Sheet1")

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return fmt.Errorf("style create: %w", err)
	}

	for i, sh := range c.Sheets {
		name := sh.Name
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		if i == 0 && defaultIdx >= 0 {
			f.SetSheetName("Sheet1", name)
		} else {
			if _, err := f.NewSheet(name); err != nil {
				return fmt.Errorf("create sheet %s: %w", name, err)
			}
		}
		for rowIdx, row := range sh.Rows {
			for colIdx, cell := range row {
				axis, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				if err != nil {
					return err
				}
				if err := f.SetCellValue(name, axis, cell); err != nil {
					return err
				}
				if rowIdx == 0 {
					f.SetCellStyle(name, axis, axis, headerStyle)
				}
			}
		}
	}

	f.SetActiveSheet(0)
	return f.SaveAs(dst)
}
