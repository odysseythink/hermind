package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// GenerateConfig parameterizes dataset generation.
type GenerateConfig struct {
	Count    int
	Seed     int64
	OutPath  string
	Provider provider.Provider
	Model    string
}

const generateSystemPrompt = `Generate a JSON array of diverse user messages testing an AI agent's ability to solve coding problems, explain concepts, use tools, and follow multi-step instructions. Return ONLY the JSON array, no fences or commentary.`

// Generate asks the aux provider for a JSON array of input items and
// writes them to a JSONL file with a meta first line.
func Generate(ctx context.Context, cfg GenerateConfig) error {
	prompt := fmt.Sprintf(`Produce %d items. Each item: {"id": "gen_NNN", "category": "...", "message": "..."}. Use deterministic ordering based on seed=%d.`, cfg.Count, cfg.Seed)

	resp, err := cfg.Provider.Complete(ctx, &provider.Request{
		SystemPrompt: generateSystemPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(prompt)},
		},
	})
	if err != nil {
		return fmt.Errorf("benchmark: generate llm: %w", err)
	}

	raw := strings.TrimSpace(resp.Message.Content.Text())
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var items []InputItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return fmt.Errorf("benchmark: generate parse: %w (raw=%q)", err, raw)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.OutPath), 0o755); err != nil {
		return fmt.Errorf("benchmark: mkdir: %w", err)
	}
	f, err := os.Create(cfg.OutPath)
	if err != nil {
		return fmt.Errorf("benchmark: create: %w", err)
	}
	defer f.Close()

	meta := map[string]DatasetMeta{"__meta": {
		Seed:        cfg.Seed,
		Model:       cfg.Model,
		GeneratedAt: time.Now().UTC(),
		Count:       len(items),
	}}
	enc := json.NewEncoder(f)
	if err := enc.Encode(meta); err != nil {
		return fmt.Errorf("benchmark: write meta: %w", err)
	}
	for _, it := range items {
		if strings.TrimSpace(it.Message) == "" {
			continue
		}
		if err := enc.Encode(it); err != nil {
			return fmt.Errorf("benchmark: write item: %w", err)
		}
	}
	return nil
}
