package processors

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// ODFExtractor extracts text from ODF files (.odt, .odp).
type ODFExtractor struct{}

// NewODFExtractor creates a new ODFExtractor.
func NewODFExtractor() *ODFExtractor {
	return &ODFExtractor{}
}

// Supports returns true for .odt and .odp extensions.
func (e *ODFExtractor) Supports(ext string) bool {
	return ext == ".odt" || ext == ".odp"
}

// Extract opens the ODF (a ZIP archive), reads content.xml,
// and extracts text from <text:p> nodes.
func (e *ODFExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	r, err := zip.OpenReader(input.FilePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var contentXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "content.xml" {
			contentXML, err = f.Open()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if contentXML == nil {
		return &pipeline.ExtractOutput{Content: ""}, nil
	}
	defer contentXML.Close()

	var buf strings.Builder
	decoder := xml.NewDecoder(contentXML)
	inParagraph := false
	var paraBuf strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "p" {
				inParagraph = true
				paraBuf.Reset()
			}
		case xml.EndElement:
			if se.Name.Local == "p" {
				inParagraph = false
				text := strings.TrimSpace(paraBuf.String())
				if text != "" {
					if buf.Len() > 0 {
						buf.WriteString("\n")
					}
					buf.WriteString(text)
				}
			}
		case xml.CharData:
			if inParagraph {
				paraBuf.WriteString(string(se))
			}
		}
	}

	return &pipeline.ExtractOutput{
		Content: strings.TrimSpace(buf.String()),
	}, nil
}
