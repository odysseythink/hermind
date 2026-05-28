package oauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector"
)

const MaxAttachmentBytes = 10 << 20 // 10 MiB

// Attachment represents a file attachment sent by the LLM.
type Attachment struct {
	Filename   string `json:"filename"`
	DataBase64 string `json:"data_base64"`
}

// ParseAttachments decodes and parses each attachment via the collector,
// returning concatenated "--- Attached file: X ---\n<text>" blocks.
// Empty input returns empty string with nil error.
func ParseAttachments(ctx context.Context, coll *collector.Client, atts []Attachment) (string, error) {
	if len(atts) == 0 {
		return "", nil
	}
	if coll == nil {
		return "", fmt.Errorf("collector not available for attachment parsing")
	}
	var out strings.Builder
	for _, a := range atts {
		if a.Filename == "" {
			return "", fmt.Errorf("attachment missing filename")
		}
		data, err := base64.StdEncoding.DecodeString(a.DataBase64)
		if err != nil {
			return "", fmt.Errorf("decode %s: %w", a.Filename, err)
		}
		if len(data) > MaxAttachmentBytes {
			return "", fmt.Errorf("attachment %s exceeds %d bytes", a.Filename, MaxAttachmentBytes)
		}
		parsed, err := coll.ParseInMemory(ctx, a.Filename, data)
		if err != nil {
			return "", fmt.Errorf("parse %s: %w", a.Filename, err)
		}
		fmt.Fprintf(&out, "\n\n--- Attached file: %s ---\n%s\n", a.Filename, parsed)
	}
	return out.String(), nil
}
