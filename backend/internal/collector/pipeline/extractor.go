package pipeline

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// ExtractInput holds the parameters for content extraction.
type ExtractInput struct {
	FilePath string
	Filename string
	Metadata map[string]string
	Options  core.Options
}

// ExtractOutput holds the result of content extraction.
type ExtractOutput struct {
	Content string
}

// ContentExtractor defines the interface for file-type-specific extractors.
type ContentExtractor interface {
	Extract(ctx context.Context, input ExtractInput) (*ExtractOutput, error)
	Supports(ext string) bool
}
