package processors

import (
	"context"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// ImageExtractor extracts text from image files using OCR.
type ImageExtractor struct {
	tesseract *external.TesseractAdapter
}

// NewImageExtractor creates a new ImageExtractor.
func NewImageExtractor(tesseract *external.TesseractAdapter) *ImageExtractor {
	return &ImageExtractor{
		tesseract: tesseract,
	}
}

// Supports returns true for supported image extensions.
func (e *ImageExtractor) Supports(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}

// Extract runs OCR on the image file and returns the recognized text.
func (e *ImageExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	if e.tesseract == nil || !e.tesseract.Available() {
		return nil, core.ErrOCRFailed
	}

	langs := e.resolveLangs(input)
	content, err := e.tesseract.Run(ctx, input.FilePath, langs)
	if err != nil {
		return nil, core.ErrOCRFailed
	}

	return &pipeline.ExtractOutput{
		Content: content,
	}, nil
}

func (e *ImageExtractor) resolveLangs(input pipeline.ExtractInput) []string {
	if lang := strings.TrimSpace(input.Options.OCR.LangList); lang != "" {
		return parseOCRLangs(lang)
	}
	// Default to English.
	return []string{"eng"}
}
