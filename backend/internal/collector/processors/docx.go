package processors

import (
	"context"
	"os"
	"strings"

	"github.com/fumiama/go-docx"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// DocxExtractor extracts text from .docx files.
type DocxExtractor struct{}

// NewDocxExtractor creates a new DocxExtractor.
func NewDocxExtractor() *DocxExtractor {
	return &DocxExtractor{}
}

// Supports returns true for the .docx extension.
func (e *DocxExtractor) Supports(ext string) bool {
	return ext == ".docx"
}

// Extract opens the DOCX file and extracts text from paragraphs and tables.
func (e *DocxExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	file, err := os.Open(input.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	d, err := docx.Parse(file, info.Size())
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	for _, item := range d.Document.Body.Items {
		switch v := item.(type) {
		case *docx.Paragraph:
			text := v.String()
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteString("\n")
				}
				buf.WriteString(text)
			}
		case *docx.Table:
			text := v.String()
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteString("\n")
				}
				buf.WriteString(text)
			}
		}
	}

	content := strings.TrimSpace(buf.String())
	if content == "" {
		return nil, core.ErrEmptyContent
	}

	return &pipeline.ExtractOutput{
		Content: content,
	}, nil
}
