package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/go-pdf/fpdf"
)

func writePDFFile(ctx context.Context, dst string, content string, title string) error {
	for _, r := range content {
		if r > unicode.MaxASCII {
			return fmt.Errorf("PDF generation does not yet support non-ASCII text; use markdown instead")
		}
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	if title != "" {
		pdf.Cell(0, 10, title)
		pdf.Ln(12)
	}
	pdf.SetFont("Helvetica", "", 11)
	for _, line := range strings.Split(content, "\n") {
		pdf.MultiCell(0, 6, line, "", "", false)
	}
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("pdf build: %w", err)
	}
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("pdf create: %w", err)
	}
	defer f.Close()
	return pdf.Output(f)
}
