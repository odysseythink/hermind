package processors

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// MboxExtractor extracts body text from MBOX files.
type MboxExtractor struct{}

// NewMboxExtractor creates a new MboxExtractor.
func NewMboxExtractor() *MboxExtractor {
	return &MboxExtractor{}
}

// Supports returns true for the .mbox extension.
func (e *MboxExtractor) Supports(ext string) bool {
	return ext == ".mbox"
}

// Extract reads an MBOX file, splits messages by "From " lines,
// skips headers, and concatenates body text.
func (e *MboxExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	file, err := os.Open(input.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var bodies []string
	var currentBody strings.Builder
	inHeaders := true

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// New message starts with "From " at the beginning of a line.
		if strings.HasPrefix(line, "From ") {
			if currentBody.Len() > 0 {
				bodies = append(bodies, strings.TrimSpace(currentBody.String()))
			}
			currentBody.Reset()
			inHeaders = true
			continue
		}

		if inHeaders {
			if line == "" {
				inHeaders = false
			}
			continue
		}

		currentBody.WriteString(line)
		currentBody.WriteString("\n")
	}

	if currentBody.Len() > 0 {
		bodies = append(bodies, strings.TrimSpace(currentBody.String()))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	content := strings.Join(bodies, "\n\n")
	return &pipeline.ExtractOutput{
		Content: content,
	}, nil
}
