package memorylayer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

//go:embed prompts/sufficiency_check.txt
var sufficiencyPromptTemplate string

//go:embed prompts/query_expansion.txt
var queryExpansionPromptTemplate string

type AgenticConfig struct {
	Enabled            bool
	MaxExtraRounds     int           // default 1
	ExpansionQueries   int           // default 2
	ShortcutThreshold  float64       // default 0.85
	PerTurnTokenCap    int           // default 2000
	PerSessionTokenCap int           // default 20000
	Timeout            time.Duration // default 8s
}

func (c *AgenticConfig) fill() {
	if c.MaxExtraRounds <= 0 {
		c.MaxExtraRounds = 1
	}
	if c.ExpansionQueries <= 0 {
		c.ExpansionQueries = 2
	}
	if c.ShortcutThreshold <= 0 {
		c.ShortcutThreshold = 0.85
	}
	if c.PerTurnTokenCap <= 0 {
		c.PerTurnTokenCap = 2000
	}
	if c.PerSessionTokenCap <= 0 {
		c.PerSessionTokenCap = 20000
	}
	if c.Timeout <= 0 {
		c.Timeout = 8 * time.Second
	}
}

// Agentic adds a critic-driven extra retrieval round on top of MemoryLayer.
type Agentic struct {
	base   *MemoryLayer
	llm    core.LanguageModel
	cfg    AgenticConfig
	tokens *TokenCap
}

func NewAgentic(base *MemoryLayer, llm core.LanguageModel, cfg AgenticConfig) *Agentic {
	cfg.fill()
	return &Agentic{
		base:   base,
		llm:    llm,
		cfg:    cfg,
		tokens: NewTokenCap(cfg.PerTurnTokenCap, cfg.PerSessionTokenCap),
	}
}

// Recall runs Hybrid+Rerank once; if insufficient and budget allows,
// runs one extra round of LLM-driven sub-queries and re-ranks the union.
//
// Failures degrade silently to the first-round result. The returned
// slice has at most limit items.
func (a *Agentic) Recall(ctx context.Context, query string, limit int) ([]Candidate, error) {
	if a == nil || !a.cfg.Enabled || a.base == nil {
		return nil, fmt.Errorf("agentic: not configured")
	}
	a.tokens.ResetTurn()

	callCtx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()

	m1, err := a.base.RecallCandidates(callCtx, query, limit)
	if err != nil {
		return nil, err
	}
	if len(m1) == 0 {
		return nil, nil
	}

	// Shortcut: best candidate already strong.
	if m1[0].Score >= a.cfg.ShortcutThreshold {
		return m1, nil
	}

	if a.llm == nil {
		return m1, nil
	}

	sufficient, missing := a.checkSufficiency(callCtx, query, m1)
	if sufficient {
		return m1, nil
	}

	subqueries := a.expandQueries(callCtx, query, missing, m1)
	if len(subqueries) == 0 {
		return m1, nil // degrade
	}

	extras := a.runSubqueries(callCtx, subqueries, limit)
	fused := rrfFuseCandidates(append([][]Candidate{m1}, extras...), 60)

	// Re-rank the union through the same reranker the base uses.
	final, _ := a.base.reranker.Rerank(callCtx, query, fused, limit)
	return final, nil
}

func (a *Agentic) checkSufficiency(ctx context.Context, query string, cands []Candidate) (bool, string) {
	// Token estimate: prompt + response are both small (under ~400 tokens
	// total). Charge a flat cost and bail on cap.
	if !a.tokens.Allow(400) {
		mlog.Warning("agentic: sufficiency check skipped — cap reached")
		return true, "" // treat as sufficient to skip extra round
	}
	prompt := strings.ReplaceAll(sufficiencyPromptTemplate, "{{QUERY}}", query)
	prompt = strings.ReplaceAll(prompt, "{{COUNT}}", fmt.Sprintf("%d", len(cands)))
	prompt = strings.ReplaceAll(prompt, "{{MEMORIES}}", renderCandidatesForCritic(cands))

	resp, err := a.llm.Generate(ctx, &core.Request{
		SystemPrompt: "You are a memory-retrieval critic.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
	})
	if err != nil {
		return true, ""
	} // degrade
	var out struct {
		Sufficient bool   `json:"sufficient"`
		Missing    string `json:"missing"`
	}
	if !parseJSONObject(extractText(resp), &out) {
		return true, ""
	}
	return out.Sufficient, out.Missing
}

func (a *Agentic) expandQueries(ctx context.Context, query, missing string, m1 []Candidate) []string {
	if !a.tokens.Allow(600) {
		mlog.Warning("agentic: expansion skipped — cap reached")
		return nil
	}
	prompt := strings.ReplaceAll(queryExpansionPromptTemplate, "{{QUERY}}", query)
	prompt = strings.ReplaceAll(prompt, "{{MISSING}}", missing)
	prompt = strings.ReplaceAll(prompt, "{{N}}", fmt.Sprintf("%d", a.cfg.ExpansionQueries))
	prompt = strings.ReplaceAll(prompt, "{{MEMORIES}}", renderCandidatesForCritic(m1))

	resp, err := a.llm.Generate(ctx, &core.Request{
		SystemPrompt: "You generate complementary retrieval queries.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
	})
	if err != nil {
		return nil
	}
	text := extractText(resp)
	if i := strings.Index(text, "["); i >= 0 {
		text = text[i:]
	}
	if j := strings.LastIndex(text, "]"); j >= 0 {
		text = text[:j+1]
	}
	var qs []string
	if err := json.Unmarshal([]byte(text), &qs); err != nil {
		return nil
	}
	if len(qs) > a.cfg.ExpansionQueries {
		qs = qs[:a.cfg.ExpansionQueries]
	}
	return qs
}

func (a *Agentic) runSubqueries(ctx context.Context, qs []string, limit int) [][]Candidate {
	// Each sub-recall is a hybrid hit on the local store — no LLM call,
	// no cap charge.
	out := make([][]Candidate, len(qs))
	var wg sync.WaitGroup
	for i, q := range qs {
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			cands, err := a.base.RecallCandidates(ctx, q, limit)
			if err != nil {
				return
			}
			out[i] = cands
		}(i, q)
	}
	wg.Wait()
	return out
}

func renderCandidatesForCritic(cs []Candidate) string {
	var b strings.Builder
	for i, c := range cs {
		if i >= 10 {
			break
		} // critic sees at most 10
		fmt.Fprintf(&b, "%d. %s\n", i+1, truncate(c.Content, 160))
	}
	return b.String()
}

func parseJSONObject(s string, v any) bool {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 {
		s = s[:j+1]
	}
	return json.Unmarshal([]byte(s), v) == nil
}

// rrfFuseCandidates fuses N ranked candidate lists by RRF. Candidates
// keyed by ID; score is the sum of 1/(k+rank) contributions across lists
// that contain that candidate. Source is normalized to "agentic_fused".
func rrfFuseCandidates(lists [][]Candidate, k float64) []Candidate {
	byID := make(map[string]*Candidate)
	for _, list := range lists {
		for i, c := range list {
			rank := i + 1
			contrib := 1.0 / (k + float64(rank))
			if existing, ok := byID[c.ID]; ok {
				existing.Score += contrib
			} else {
				cc := c
				cc.Score = contrib
				cc.Source = "agentic_fused"
				byID[c.ID] = &cc
			}
		}
	}
	out := make([]Candidate, 0, len(byID))
	for _, c := range byID {
		out = append(out, *c)
	}
	// Sort by score desc; reranker will rewrite ordering anyway, but
	// a deterministic input keeps reranker fallback well-behaved.
	sortCandidatesByScoreDesc(out)
	return out
}

func sortCandidatesByScoreDesc(cs []Candidate) {
	// Use the same sort.SliceStable shape as in hybrid_recaller.go.
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].Score == cs[j].Score {
			return cs[i].ID < cs[j].ID
		}
		return cs[i].Score > cs[j].Score
	})
}
