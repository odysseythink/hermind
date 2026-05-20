package memorylayer

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/pantheon/core"
)

//go:embed prompts/taxonomy_extract.txt
var taxonomyPromptTemplate string

var _ embed.FS

type TaxonomyConfig struct {
	Enabled      bool
	MaxOutputs   int           // default 8 per boundary
	Timeout      time.Duration // default 6 * time.Second
	AllowedTypes []string      // default {"core","episode","fact","foresight"}
}

func (c *TaxonomyConfig) fill() {
	if c.MaxOutputs <= 0 {
		c.MaxOutputs = 8
	}
	if c.Timeout <= 0 {
		c.Timeout = 6 * time.Second
	}
	if len(c.AllowedTypes) == 0 {
		c.AllowedTypes = []string{"core", "episode", "fact", "foresight"}
	}
}

type TaxonomyExtractor struct {
	llm core.LanguageModel
	cfg TaxonomyConfig
}

func NewTaxonomyExtractor(llm core.LanguageModel, cfg TaxonomyConfig) *TaxonomyExtractor {
	cfg.fill()
	return &TaxonomyExtractor{llm: llm, cfg: cfg}
}

type extractedItem struct {
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	ExpiresAt  string  `json:"expires_at,omitempty"`
}

// Extract runs the LLM extractor over a Boundary and returns Memory rows
// ready for storage. Caller is responsible for SaveMemory.
func (e *TaxonomyExtractor) Extract(ctx context.Context, b *Boundary) ([]*storage.Memory, error) {
	if !e.cfg.Enabled || e.llm == nil || b == nil || len(b.Turns) == 0 {
		return nil, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
	defer cancel()

	prompt := renderTaxonomyPrompt(b)
	req := &core.Request{
		SystemPrompt: "You extract durable structured memories from conversations.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
	}
	resp, err := e.llm.Generate(callCtx, req)
	if err != nil {
		return nil, err
	}

	items := parseExtracted(extractText(resp))
	if len(items) > e.cfg.MaxOutputs {
		items = items[:e.cfg.MaxOutputs]
	}

	allowed := make(map[string]bool, len(e.cfg.AllowedTypes))
	for _, t := range e.cfg.AllowedTypes {
		allowed[t] = true
	}

	lastTurn := b.Turns[len(b.Turns)-1]
	now := time.Now().UTC()
	out := make([]*storage.Memory, 0, len(items))
	for _, it := range items {
		if !allowed[it.Type] || strings.TrimSpace(it.Content) == "" {
			continue
		}
		m := &storage.Memory{
			Content:      it.Content,
			MemType:      it.Type,
			ParentTurnID: lastTurn.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
			Status:       "active",
		}
		if it.Type == "foresight" {
			if exp, err := time.Parse(time.RFC3339, it.ExpiresAt); err == nil {
				m.ExpiresAt = exp.UTC()
			} else {
				// default: 7 days out, configurable in P3
				m.ExpiresAt = now.AddDate(0, 0, 7)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func renderTaxonomyPrompt(b *Boundary) string {
	var lines strings.Builder
	for _, t := range b.Turns {
		fmt.Fprintf(&lines, "[turn %d]\nuser: %s\nassistant: %s\n\n", t.ID, t.UserMsg, t.Assistant)
	}
	p := strings.ReplaceAll(taxonomyPromptTemplate, "{{TURN_COUNT}}", fmt.Sprintf("%d", len(b.Turns)))
	return strings.ReplaceAll(p, "{{TURNS}}", lines.String())
}

func parseExtracted(text string) []extractedItem {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "["); i >= 0 {
		text = text[i:]
	}
	if j := strings.LastIndex(text, "]"); j >= 0 {
		text = text[:j+1]
	}
	var items []extractedItem
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		return nil
	}
	return items
}
