package document

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
	"github.com/xuri/excelize/v2"
)

// NewCreateExcelHandler returns a handler for create_excel_spreadsheet.
func NewCreateExcelHandler(outputDir string) tool.Handler {
	mgr := NewManager(outputDir)
	return func(_ context.Context, args json.RawMessage) (string, error) {
		var params struct {
			Filename string `json:"filename"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.ToolError(fmt.Sprintf("invalid args: %v", err)), nil
		}
		if params.Filename == "" {
			params.Filename = "spreadsheet"
		}
		if !strings.HasSuffix(strings.ToLower(params.Filename), ".xlsx") {
			params.Filename += ".xlsx"
		}

		f := excelize.NewFile()
		if params.Title != "" {
			f.SetDocProps(&excelize.DocProperties{Title: params.Title})
		}

		tables := parseMarkdownTables(params.Content)
		for i, table := range tables {
			sheetName := fmt.Sprintf("Sheet%d", i+1)
			if i == 0 {
				f.SetSheetName("Sheet1", sheetName)
			} else {
				f.NewSheet(sheetName)
			}
			for rowIdx, row := range table.Rows {
				for colIdx, cell := range row {
					cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
					f.SetCellValue(sheetName, cellRef, cell)
				}
			}
		}

		buf, err := f.WriteToBuffer()
		if err != nil {
			return tool.ToolError(fmt.Sprintf("generate excel: %v", err)), nil
		}

		saved, err := mgr.Save("xlsx", "xlsx", buf.Bytes(), params.Filename)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created Excel spreadsheet '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

type markdownTable struct {
	Rows [][]string
}

var tableRowRegex = regexp.MustCompile(`^\|(.+)\|$`)

func parseMarkdownTables(content string) []markdownTable {
	lines := strings.Split(content, "\n")
	var tables []markdownTable
	var current *markdownTable

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "|---") {
			continue
		}
		match := tableRowRegex.FindStringSubmatch(line)
		if match != nil {
			if current == nil {
				current = &markdownTable{}
			}
			cells := strings.Split(match[1], "|")
			var row []string
			for _, c := range cells {
				row = append(row, strings.TrimSpace(c))
			}
			current.Rows = append(current.Rows, row)
		} else {
			if current != nil {
				tables = append(tables, *current)
				current = nil
			}
		}
	}
	if current != nil {
		tables = append(tables, *current)
	}
	return tables
}

const CreateExcelSchema = `{
	"type": "object",
	"properties": {
		"filename": {"type": "string", "description": "The filename for the spreadsheet. Will add .xlsx if not present."},
		"title": {"type": "string", "description": "Optional document title for metadata."},
		"content": {"type": "string", "description": "Markdown tables to convert to Excel sheets. Each table becomes a worksheet."}
	},
	"required": ["filename", "content"]
}`

func RegisterCreateExcel(reg *tool.Registry, outputDir string) {
	reg.Register(&tool.Entry{
		Name:        "create_excel_spreadsheet",
		Toolset:     "document_creation",
		Description: "Create an Excel spreadsheet (.xlsx) from Markdown tables.",
		Emoji:       "📊",
		Handler:     NewCreateExcelHandler(outputDir),
		Schema: core.ToolDefinition{
			Name:        "create_excel_spreadsheet",
			Description: "Create an Excel spreadsheet (.xlsx) from Markdown tables. Each table becomes a separate worksheet.",
			Parameters:  core.MustSchemaFromJSON([]byte(CreateExcelSchema)),
		},
	})
}
