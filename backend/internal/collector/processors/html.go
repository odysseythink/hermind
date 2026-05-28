package processors

import (
	"context"
	"os"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"golang.org/x/net/html"
)

// HTMLExtractor extracts visible text content from HTML files.
type HTMLExtractor struct{}

// NewHTMLExtractor creates a new HTMLExtractor.
func NewHTMLExtractor() *HTMLExtractor {
	return &HTMLExtractor{}
}

// Supports returns true for the .html extension.
func (e *HTMLExtractor) Supports(ext string) bool {
	return ext == ".html"
}

// Extract parses the HTML file and returns visible body text.
func (e *HTMLExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	data, err := os.ReadFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	e.extractText(doc, &buf)

	return &pipeline.ExtractOutput{
		Content: strings.TrimSpace(buf.String()),
	}, nil
}

// extractText recursively walks the HTML node tree, collecting text from
// visible elements and skipping script, style, and noscript tags.
func (e *HTMLExtractor) extractText(n *html.Node, buf *strings.Builder) {
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

	// Skip invisible elements.
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
