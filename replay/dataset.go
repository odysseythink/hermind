package replay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/odysseythink/hermind/benchmark"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/storage"
)

// GenerateConfig controls how Generate walks state.db into ReplayItems.
type GenerateConfig struct {
	// Mode selects history extraction strategy. "cold" emits items
	// with empty History; "contextual" emits items with the preceding
	// messages as History (capped by HistoryCap).
	Mode string
	// HistoryCap caps the number of preceding messages copied into
	// each ReplayItem.History when Mode == "contextual". Default 20.
	HistoryCap int
	// UserMsgFrom is the messages.id starting offset; 0 means "all".
	UserMsgFrom int64
	// UserMsgLimit caps the number of ReplayItems produced; 0 means
	// "no cap".
	UserMsgLimit int
	// OutPath is the JSONL file Generate writes.
	OutPath string
}

// Generate walks the storage's message history and writes a ReplayItem
// per (user message, following assistant reply) pair. The output JSONL
// has a first-line meta record carrying mode + counts; subsequent
// lines are ReplayItem objects.
func Generate(ctx context.Context, store storage.Storage, cfg GenerateConfig) error {
	if cfg.Mode != "cold" && cfg.Mode != "contextual" {
		return fmt.Errorf("replay: invalid mode %q (want cold | contextual)", cfg.Mode)
	}
	if cfg.HistoryCap < 0 {
		cfg.HistoryCap = 0
	}
	if cfg.HistoryCap == 0 && cfg.Mode == "contextual" {
		cfg.HistoryCap = 20
	}

	msgs, err := store.GetHistory(ctx, 1<<20, 0)
	if err != nil {
		return fmt.Errorf("replay: get history: %w", err)
	}

	f, err := os.Create(cfg.OutPath)
	if err != nil {
		return fmt.Errorf("replay: create %s: %w", cfg.OutPath, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)

	type out struct {
		items   []ReplayItem
		skipped int
	}
	collected := out{}

	for i, m := range msgs {
		if m.Role != "user" {
			continue
		}
		if cfg.UserMsgFrom > 0 && m.ID < cfg.UserMsgFrom {
			continue
		}
		var next *storage.StoredMessage
		for j := i + 1; j < len(msgs); j++ {
			if msgs[j].Role == "assistant" && msgs[j].Content != "" {
				next = msgs[j]
				break
			}
			if msgs[j].Role == "user" {
				break
			}
		}
		if next == nil {
			collected.skipped++
			continue
		}

		var history []message.Message
		if cfg.Mode == "contextual" && i > 0 {
			startIdx := i - cfg.HistoryCap
			if startIdx < 0 {
				startIdx = 0
			}
			for _, h := range msgs[startIdx:i] {
				history = append(history, storedToMessage(h))
			}
		}

		collected.items = append(collected.items, ReplayItem{
			ID:       fmt.Sprintf("replay_%d", m.ID),
			Message:  m.Content,
			History:  history,
			Baseline: next.Content,
		})

		if cfg.UserMsgLimit > 0 && len(collected.items) >= cfg.UserMsgLimit {
			break
		}
	}

	meta := map[string]any{
		"__meta": map[string]any{
			"kind":            "replay",
			"mode":            cfg.Mode,
			"history_cap":     cfg.HistoryCap,
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
			"count":           len(collected.items),
			"skipped_orphans": collected.skipped,
			"source":          "state.db",
		},
	}
	if err := enc.Encode(meta); err != nil {
		return fmt.Errorf("replay: write meta: %w", err)
	}
	for _, item := range collected.items {
		if err := enc.Encode(item); err != nil {
			return fmt.Errorf("replay: write item %s: %w", item.ID, err)
		}
	}
	return nil
}

// storedToMessage converts a storage.StoredMessage into the in-memory
// message.Message form. Only the fields the Engine needs for context
// preload are copied; tool_calls etc. are passed through as JSON.
func storedToMessage(s *storage.StoredMessage) message.Message {
	role := message.Role(s.Role)
	return message.Message{
		Role:       role,
		Content:    message.TextContent(s.Content),
		ToolCallID: s.ToolCallID,
		ToolName:   s.ToolName,
	}
}

// LoadDataset reads a replay JSONL and returns benchmark.Items. It
// rejects datasets whose meta record indicates a non-replay kind so
// callers can't accidentally feed a synthetic dataset to the replay
// runner (or vice versa).
func LoadDataset(path string) ([]benchmark.Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("replay: open %s: %w", path, err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)

	var items []benchmark.Item
	metaSeen := false
	for s.Scan() {
		line := s.Bytes()
		if !metaSeen {
			var probe map[string]json.RawMessage
			if err := json.Unmarshal(line, &probe); err == nil {
				if metaRaw, isMeta := probe["__meta"]; isMeta {
					var meta map[string]any
					if err := json.Unmarshal(metaRaw, &meta); err == nil {
						kind, _ := meta["kind"].(string)
						if !strings.EqualFold(kind, "replay") {
							return nil, fmt.Errorf("replay: dataset kind=%q, expected \"replay\" — wrong loader for this file", kind)
						}
					}
					metaSeen = true
					continue
				}
			}
		}
		var item ReplayItem
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		if item.ID == "" {
			continue
		}
		items = append(items, item)
	}
	return items, s.Err()
}
