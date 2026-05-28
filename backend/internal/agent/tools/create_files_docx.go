package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fumiama/go-docx"
)

func writeDocxFile(ctx context.Context, dst string, content string, title string) error {
	doc := docx.New()
	doc.WithA4Page()

	if title != "" {
		p := doc.AddParagraph()
		p.AddText(title)
	}

	for _, para := range strings.Split(content, "\n\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		p := doc.AddParagraph()
		p.AddText(para)
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("docx create file: %w", err)
	}
	defer f.Close()
	if _, err := doc.WriteTo(f); err != nil {
		return fmt.Errorf("docx write: %w", err)
	}
	return nil
}
