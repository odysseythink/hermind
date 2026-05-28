package processors

import (
	"archive/zip"
	"context"
	"path"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"golang.org/x/net/html"
)

// EPubExtractor extracts visible text from EPUB files.
type EPubExtractor struct{}

// NewEPubExtractor creates a new EPubExtractor.
func NewEPubExtractor() *EPubExtractor {
	return &EPubExtractor{}
}

// Supports returns true for the .epub extension.
func (e *EPubExtractor) Supports(ext string) bool {
	return ext == ".epub"
}

// Extract opens the EPUB (a ZIP archive), finds XHTML/HTML files,
// and extracts visible text from each.
func (e *EPubExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	r, err := zip.OpenReader(input.FilePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var buf strings.Builder
	for _, f := range r.File {
		ext := strings.ToLower(path.Ext(f.Name))
		if ext != ".xhtml" && ext != ".html" && ext != ".htm" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		doc, err := html.Parse(rc)
		rc.Close()
		if err != nil {
			continue
		}

		e.extractText(doc, &buf)
	}

	return &pipeline.ExtractOutput{
		Content: strings.TrimSpace(buf.String()),
	}, nil
}

// extractText recursively walks the HTML node tree, collecting text from
// visible elements and skipping script, style, and noscript tags.
func (e *EPubExtractor) extractText(n *html.Node, buf *strings.Builder) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
		return
	}

	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript", "iframe", "canvas":
			return
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.extractText(c, buf)
	}
}
