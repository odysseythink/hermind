package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// LLMJudge is the default ConversationJudge backed by an aux LLM call.
type LLMJudge struct {
	llm provider.Provider
}

// NewLLMJudge constructs an LLMJudge backed by the given provider.
// A nil provider yields a judge that always returns Verdict{Outcome: "unknown"}.
func NewLLMJudge(llm provider.Provider) *LLMJudge {
	return &LLMJudge{llm: llm}
}

// Run implements ConversationJudge. Any failure is folded into a
// "unknown" verdict; the error return is reserved for future use.
func (j *LLMJudge) Run(ctx context.Context, in JudgeInput) (*Verdict, error) {
	if j.llm == nil {
		return &Verdict{Outcome: "unknown"}, nil
	}
	prompt := buildJudgePrompt(in)
	resp, err := j.llm.Complete(ctx, &provider.Request{
		SystemPrompt: judgeSystemPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(prompt)},
		},
	})
	if err != nil {
		return &Verdict{Outcome: "unknown"}, nil
	}

	raw := strings.TrimSpace(resp.Message.Content.Text())
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		Outcome         string       `json:"outcome"`
		MemoriesUsed    []string     `json:"memories_used"`
		SkillsToExtract []SkillDraft `json:"skills_to_extract"`
		Reasoning       string       `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return &Verdict{Outcome: "unknown"}, nil
	}
	if parsed.Outcome == "" {
		parsed.Outcome = "unknown"
	}
	return &Verdict{
		Outcome:         parsed.Outcome,
		MemoriesUsed:    parsed.MemoriesUsed,
		SkillsToExtract: parsed.SkillsToExtract,
		Reasoning:       parsed.Reasoning,
	}, nil
}

const judgeSystemPrompt = `You are a post-conversation judge. Given the transcript and the memories/skills injected into the system prompt, produce a JSON verdict.

Rules:
- outcome: "success" if the user's request was resolved cleanly; "struggle" if the agent retried, backtracked, or the user had to restate; "failure" if unresolved or wrong.
- memories_used: the subset of injected memory IDs that materially influenced the assistant's reply. Exclude memories the agent clearly ignored.
- skills_to_extract: only populate when outcome != "success". Each skill must be reusable beyond this conversation.

Reply ONLY with JSON, no fences.`

func buildJudgePrompt(in JudgeInput) string {
	var b strings.Builder
	b.WriteString("# Transcript\n")
	for _, m := range in.History {
		text := m.Content.Text()
		if len(text) > 2000 {
			text = text[:2000] + "…"
		}
		fmt.Fprintf(&b, "%s: %s\n", m.Role, text)
	}
	b.WriteString("\n# Injected memories\n")
	for _, m := range in.InjectedMemories {
		fmt.Fprintf(&b, "- id=%s content=%q\n", m.ID, m.Content)
	}
	if len(in.InjectedSkills) > 0 {
		b.WriteString("\n# Injected skills\n")
		for _, s := range in.InjectedSkills {
			fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
		}
	}
	return b.String()
}
