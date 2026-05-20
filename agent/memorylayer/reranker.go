package memorylayer

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/pantheon/core"
)

//go:embed prompts/*.txt
var _ embed.FS

//go:embed prompts/rerank.txt
var rerankPromptTemplate string

type RerankerConfig struct {
	Enabled   bool
	BatchSize int           // cap candidates sent; default 20
	Timeout   time.Duration // default 1500 ms
}

func (c *RerankerConfig) fill() {
	if c.BatchSize <= 0 {
		c.BatchSize = 20
	}
	if c.Timeout <= 0 {
		c.Timeout = 1500 * time.Millisecond
	}
}

type Reranker interface {
	Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error)
}

type LLMReranker struct {
	llm core.LanguageModel
	cfg RerankerConfig
}

func NewLLMReranker(llm core.LanguageModel, cfg RerankerConfig) *LLMReranker {
	cfg.fill()
	return &LLMReranker{llm: llm, cfg: cfg}
}

func (r *LLMReranker) Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error) {
	if !r.cfg.Enabled || r.llm == nil || len(cands) <= 1 {
		return trimCandidates(cands, topN), nil
	}
	if len(cands) > r.cfg.BatchSize {
		cands = cands[:r.cfg.BatchSize]
	}

	callCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	prompt := renderRerankPrompt(query, cands)
	req := &core.Request{
		SystemPrompt: "You are a memory relevance reranker.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
	}
	resp, err := r.llm.Generate(callCtx, req)
	if err != nil {
		return trimCandidates(cands, topN), nil // degrade silently
	}
	ids := parseRerankIDs(extractText(resp))
	if len(ids) == 0 {
		return trimCandidates(cands, topN), nil
	}

	byID := make(map[string]Candidate, len(cands))
	for _, c := range cands {
		byID[c.ID] = c
	}
	ordered := make([]Candidate, 0, len(cands))
	seen := make(map[string]bool, len(cands))
	for _, id := range ids {
		if c, ok := byID[id]; ok && !seen[id] {
			c.Source = "rerank"
			ordered = append(ordered, c)
			seen[id] = true
		}
	}
	// Append any candidates the LLM dropped, in original order.
	for _, c := range cands {
		if !seen[c.ID] {
			ordered = append(ordered, c)
		}
	}
	return trimCandidates(ordered, topN), nil
}

func renderRerankPrompt(query string, cands []Candidate) string {
	var b strings.Builder
	for i, c := range cands {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, c.ID, truncate(c.Content, 200))
	}
	p := strings.ReplaceAll(rerankPromptTemplate, "{{QUERY}}", query)
	return strings.ReplaceAll(p, "{{CANDIDATES}}", b.String())
}

func parseRerankIDs(text string) []string {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "["); i >= 0 {
		text = text[i:]
	}
	if j := strings.LastIndex(text, "]"); j >= 0 {
		text = text[:j+1]
	}
	var ids []string
	if err := json.Unmarshal([]byte(text), &ids); err != nil {
		return nil
	}
	return ids
}

func trimCandidates(cs []Candidate, n int) []Candidate {
	if n <= 0 || n >= len(cs) {
		return cs
	}
	return cs[:n]
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func extractText(resp *core.Response) string {
	if resp == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range resp.Message.Content {
		if p, ok := part.(core.TextPart); ok {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
