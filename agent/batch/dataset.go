package batch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Item is a single dataset row.
type Item struct {
	// ID is a stable, unique identifier used for trajectory filenames
	// and checkpoint entries. If the row did not supply an "id" field,
	// ReadDataset assigns "line-<N>" (1-indexed).
	ID string `json:"id"`
	// Prompt is the user message fed to the agent.
	Prompt string `json:"prompt"`
	// Raw preserves the original line bytes for downstream consumers
	// that care about extra fields (e.g. ground-truth answers).
	Raw json.RawMessage `json:"-"`
}

// ReadDataset parses a JSONL file into Items. If maxItems > 0 the
// slice is truncated to that length. Blank/whitespace-only lines are
// skipped; malformed JSON returns an error (with the offending line
// number so the user can fix the file).
func ReadDataset(path string, maxItems int) ([]Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("batch: open dataset: %w", err)
	}
	defer f.Close()

	items := make([]Item, 0, 64)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<22) // allow up to 4 MiB per line
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var it Item
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("batch: dataset: line %d: %w", lineNum, err)
		}
		it.Raw = json.RawMessage(append([]byte(nil), line...))
		if it.ID == "" {
			it.ID = fmt.Sprintf("line-%d", lineNum)
		}
		items = append(items, it)
		if maxItems > 0 && len(items) >= maxItems {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("batch: dataset scan: %w", err)
	}
	return items, nil
}
