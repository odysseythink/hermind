package document

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// NewCreatePDFHandler returns a handler for create_pdf_document.
func NewCreatePDFHandler(outputDir string) tool.Handler {
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
			params.Filename = "document"
		}
		if !strings.HasSuffix(strings.ToLower(params.Filename), ".pdf") {
			params.Filename += ".pdf"
		}

		doc := fpdf.New("P", "mm", "A4", "")
		doc.SetAutoPageBreak(true, 15)
		doc.AddPage()
		doc.SetFont("Helvetica", "", 12)

		if params.Title != "" {
			doc.SetFont("Helvetica", "B", 16)
			doc.Cell(0, 10, params.Title)
			doc.Ln(12)
			doc.SetFont("Helvetica", "", 12)
		}

		lines := strings.Split(params.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				doc.Ln(6)
				continue
			}
			if strings.HasPrefix(line, "# ") {
				doc.SetFont("Helvetica", "B", 14)
				doc.Cell(0, 10, strings.TrimPrefix(line, "# "))
				doc.Ln(10)
				doc.SetFont("Helvetica", "", 12)
			} else if strings.HasPrefix(line, "## ") {
				doc.SetFont("Helvetica", "B", 13)
				doc.Cell(0, 8, strings.TrimPrefix(line, "## "))
				doc.Ln(8)
				doc.SetFont("Helvetica", "", 12)
			} else if strings.HasPrefix(line, "### ") {
				doc.SetFont("Helvetica", "B", 12)
				doc.Cell(0, 7, strings.TrimPrefix(line, "### "))
				doc.Ln(7)
				doc.SetFont("Helvetica", "", 12)
			} else if strings.HasPrefix(line, "- ") {
				doc.Cell(5, 6, "\u2022")
				doc.Cell(0, 6, strings.TrimPrefix(line, "- "))
				doc.Ln(6)
			} else if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") {
				doc.SetFont("Helvetica", "B", 12)
				text := strings.TrimSuffix(strings.TrimPrefix(line, "**"), "**")
				doc.MultiCell(0, 6, text, "", "", false)
				doc.SetFont("Helvetica", "", 12)
			} else {
				doc.MultiCell(0, 6, line, "", "", false)
			}
		}

		var buf bytes.Buffer
		if err := doc.Output(&buf); err != nil {
			return tool.ToolError(fmt.Sprintf("generate pdf: %v", err)), nil
		}

		saved, err := mgr.Save("pdf", "pdf", buf.Bytes(), params.Filename)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created PDF document '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

const CreatePDFSchema = `{
	"type": "object",
	"properties": {
		"filename": {"type": "string", "description": "The filename for the PDF. Will add .pdf if not present."},
		"title": {"type": "string", "description": "Optional document title shown at the top of the first page."},
		"content": {"type": "string", "description": "The content to render as PDF. Supports # headings, ## subheadings, - bullet lists, and **bold** text."}
	},
	"required": ["filename", "content"]
}`

func RegisterCreatePDF(reg *tool.Registry, outputDir string) {
	reg.Register(&tool.Entry{
		Name:        "create_pdf_document",
		Toolset:     "document_creation",
		Description: "Create a PDF document from plain text or Markdown content.",
		Emoji:       "📑",
		Handler:     NewCreatePDFHandler(outputDir),
		Schema: core.ToolDefinition{
			Name:        "create_pdf_document",
			Description: "Create a PDF document from plain text or Markdown content. Supports headings, bullet lists, and bold text.",
			Parameters:  core.MustSchemaFromJSON([]byte(CreatePDFSchema)),
		},
	})
}
