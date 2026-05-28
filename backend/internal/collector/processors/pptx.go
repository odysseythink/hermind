package processors

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// PptxExtractor extracts text from .pptx files.
type PptxExtractor struct{}

// NewPptxExtractor creates a new PptxExtractor.
func NewPptxExtractor() *PptxExtractor {
	return &PptxExtractor{}
}

// Supports returns true for the .pptx extension.
func (e *PptxExtractor) Supports(ext string) bool {
	return ext == ".pptx"
}

// Extract opens the PPTX (a ZIP archive), iterates over slide XML files,
// and extracts text from <a:t> nodes.
func (e *PptxExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	r, err := zip.OpenReader(input.FilePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var buf strings.Builder
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		e.extractTextFromSlide(rc, &buf)
		rc.Close()
	}

	return &pipeline.ExtractOutput{
		Content: strings.TrimSpace(buf.String()),
	}, nil
}

// extractTextFromSlide parses a slide XML reader and appends <a:t> text to buf.
func (e *PptxExtractor) extractTextFromSlide(r io.Reader, buf *strings.Builder) {
	decoder := xml.NewDecoder(r)
	inText := false
	var textBuf strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "t" {
				inText = true
				textBuf.Reset()
			}
		case xml.EndElement:
			if se.Name.Local == "t" {
				inText = false
				text := strings.TrimSpace(textBuf.String())
				if text != "" {
					if buf.Len() > 0 {
						buf.WriteString(" ")
					}
					buf.WriteString(text)
				}
			}
		case xml.CharData:
			if inText {
				textBuf.WriteString(string(se))
			}
		}
	}
}
