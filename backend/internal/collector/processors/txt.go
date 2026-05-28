package processors

import (
	"context"
	"os"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// TxtExtractor extracts plain-text content from text-based files.
type TxtExtractor struct {
	supported []string
}

// NewTxtExtractor creates a new TxtExtractor.
func NewTxtExtractor() *TxtExtractor {
	return &TxtExtractor{
		supported: []string{".txt", ".md", ".org", ".adoc", ".rst", ".csv", ".json"},
	}
}

// Supports returns true for supported text extensions.
func (e *TxtExtractor) Supports(ext string) bool {
	for _, s := range e.supported {
		if ext == s {
			return true
		}
	}
	return false
}

// Extract reads the file contents as UTF-8 text.
func (e *TxtExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	content, err := os.ReadFile(input.FilePath)
	if err != nil {
		return nil, err
	}
	return &pipeline.ExtractOutput{
		Content: string(content),
	}, nil
}
