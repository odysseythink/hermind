// skills/evolver.go
package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Evolver extracts reusable skill snippets from completed conversations
// and persists them as Markdown files in skillDir.
type Evolver struct {
	llm      provider.Provider
	skillDir string
}

// NewEvolver constructs an Evolver.
// llm may be nil — in that case Extract is a no-op.
// skillDir is the directory where .md skill files are written.
func NewEvolver(llm provider.Provider, skillDir string) *Evolver {
	return &Evolver{llm: llm, skillDir: skillDir}
}

// Extract analyses the conversation history and persists skills.
// When verdict is non-nil, directly persists SkillsToExtract from the judge.
// When verdict is nil, falls back to the legacy LLM-extraction path.
// Always ensures skillDir exists.
func (ev *Evolver) Extract(ctx context.Context, turns []message.Message, verdict *agent.Verdict) error {
	if err := os.MkdirAll(ev.skillDir, 0o755); err != nil {
		return fmt.Errorf("evolver: mkdir %s: %w", ev.skillDir, err)
	}

	if verdict != nil {
		for _, d := range verdict.SkillsToExtract {
			body := strings.TrimSpace(d.Body)
			if body == "" {
				continue
			}
			slug := makeSlug(d.Name)
			if slug == "skill" {
				slug = makeSlug(body)
			}
			filename := fmt.Sprintf("%s-%s.md", time.Now().UTC().Format("20060102-150405"), slug)
			path := filepath.Join(ev.skillDir, filename)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return fmt.Errorf("evolver: write %s: %w", path, err)
			}
		}
		return nil
	}

	// Legacy path: no judge, full LLM extraction.
	if ev.llm == nil || len(turns) == 0 {
		return nil
	}

	conversation := formatTurns(turns)
	prompt := fmt.Sprintf(`You are a skill extraction assistant. Review this conversation and determine if it contains a reusable operational pattern, technique, or workflow that would be helpful in future conversations.

If you find a reusable skill, respond with a Markdown snippet in EXACTLY this format:
---
## <title>
**When to use:** <one-sentence trigger condition>

<step-by-step instructions>
---

If there is no reusable skill in this conversation, respond with exactly: NONE

Conversation:
%s`, conversation)

	resp, err := ev.llm.Complete(ctx, &provider.Request{
		SystemPrompt: "You extract reusable skill patterns from conversations. Reply only as instructed.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(prompt)},
		},
	})
	if err != nil {
		return nil // extraction is best-effort
	}

	raw := strings.TrimSpace(resp.Message.Content.Text())
	if raw == "NONE" || raw == "" {
		return nil
	}

	slug := makeSlug(raw)
	filename := fmt.Sprintf("%s-%s.md", time.Now().UTC().Format("20060102-150405"), slug)
	path := filepath.Join(ev.skillDir, filename)
	return os.WriteFile(path, []byte(raw), 0o644)
}

func formatTurns(turns []message.Message) string {
	var sb strings.Builder
	for _, t := range turns {
		sb.WriteString(string(t.Role))
		sb.WriteString(": ")
		sb.WriteString(t.Content.Text())
		sb.WriteString("\n")
	}
	return sb.String()
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func makeSlug(content string) string {
	lines := strings.SplitN(content, "\n", 5)
	title := ""
	for _, l := range lines {
		l = strings.TrimPrefix(l, "## ")
		l = strings.TrimPrefix(l, "# ")
		l = strings.TrimPrefix(l, "---")
		l = strings.TrimSpace(l)
		if l != "" {
			title = l
			break
		}
	}
	if title == "" {
		title = content
	}
	if len(title) > 40 {
		title = title[:40]
	}
	slug := nonAlnum.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "skill"
	}
	return slug
}
