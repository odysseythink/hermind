// agent/prompt.go
package agent

import (
	"sort"
	"strconv"
	"strings"
)

// defaultIdentity is the base personality/identity block.
// Ported from the Python hermes agent/prompt_builder.py DEFAULT_AGENT_IDENTITY.
const defaultIdentity = `You are Hermind Agent, created by odysseythink.

You are a helpful, knowledgeable AI assistant. You are direct and efficient.
You respond with markdown formatting when it aids clarity.

You have access to a set of tools. When the user's request requires
information you do not have, current/real-time data, or actions beyond
text generation, you MUST use the appropriate tool rather than guessing
or making up information. For example, use web_search to find current
information from the internet, use web_fetch to read a specific URL,
and use file tools to read or write files.

IMPORTANT: After you call a tool and receive its result, you MUST
immediately answer the user's question based on that result. Do NOT
call the same tool again with a different query unless the result was
clearly insufficient. Avoid looping — if you have search results,
synthesize them into a helpful answer right away.

You are running inside the "hermind" CLI. Skill packages for hermind live at
<instance-root>/skills (defaults to ./.hermind/skills; override with
$HERMIND_HOME). When the user asks you to install, add, or write a skill,
place the SKILL.md under that path — never under ~/.openclaw, ~/.claude,
or any other tool's directory.`

// ActiveSkill is a minimal view of a skill that PromptBuilder needs.
// Defined here so agent does not import the skills package.
type ActiveSkill struct {
	Name        string
	Description string
	Body        string
}

// PromptOptions parameterize prompt generation.
type PromptOptions struct {
	Model          string
	SkipContext    bool
	ActiveSkills   []ActiveSkill // prepended under a stable header
	ActiveMemories []string      // recalled memory snippets, one per line
	ObsidianCtx    *ObsidianContext
}

// PromptBuilder assembles system prompts for the agent engine.
// Immutable after construction — safe to share a single instance across conversations.
type PromptBuilder struct {
	platform            string
	defaultSystemPrompt string
}

// NewPromptBuilder creates a PromptBuilder for a specific platform.
// Valid platforms: "cli", "telegram", "discord", etc.
// defaultSystemPrompt is appended after the identity block when non-empty.
func NewPromptBuilder(platform, defaultSystemPrompt string) *PromptBuilder {
	return &PromptBuilder{platform: platform, defaultSystemPrompt: defaultSystemPrompt}
}

// Build assembles the system prompt. The output is stable for equivalent
// inputs — this is required for Anthropic prefix caching to work. Skill
// order is normalized so the same active set always produces the same
// prefix regardless of map iteration order upstream.
func (pb *PromptBuilder) Build(opts *PromptOptions) string {
	var parts []string
	parts = append(parts, defaultIdentity)
	if strings.TrimSpace(pb.defaultSystemPrompt) != "" {
		parts = append(parts, pb.defaultSystemPrompt)
	}
	if opts != nil && len(opts.ActiveSkills) > 0 {
		parts = append(parts, renderActiveSkills(opts.ActiveSkills))
	}
	if opts != nil && len(opts.ActiveMemories) > 0 {
		parts = append(parts, renderActiveMemories(opts.ActiveMemories))
	}
	if opts != nil && opts.ObsidianCtx != nil {
		parts = append(parts, renderObsidianContext(opts.ObsidianCtx))
	}
	return strings.Join(parts, "\n\n")
}

func renderActiveMemories(mems []string) string {
	var b strings.Builder
	b.WriteString("# Relevant memories\n\n")
	b.WriteString("Recalled from prior conversations. Treat as background context, not ground truth.\n")
	for _, m := range mems {
		trimmed := strings.TrimSpace(m)
		if trimmed == "" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(trimmed)
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderActiveSkills(active []ActiveSkill) string {
	sorted := make([]ActiveSkill, len(active))
	copy(sorted, active)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var b strings.Builder
	b.WriteString("# Active skills\n\n")
	b.WriteString("The following skills have been activated by the user. Follow their guidance for the rest of the conversation unless deactivated.\n")
	for _, s := range sorted {
		b.WriteString("\n## ")
		b.WriteString(s.Name)
		if s.Description != "" {
			b.WriteString(" — ")
			b.WriteString(s.Description)
		}
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(s.Body))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderObsidianContext(ctx *ObsidianContext) string {
	var b strings.Builder
	b.WriteString("# Obsidian Context\n\n")
	b.WriteString("The user is currently working in an Obsidian vault. Use the following context to ground your answers. When you need to read or write notes, use the obsidian_* tools.\n")
	b.WriteString("\n- Vault path: ")
	b.WriteString(ctx.VaultPath)
	if ctx.CurrentNote != "" {
		b.WriteString("\n- Active note: ")
		b.WriteString(ctx.CurrentNote)
	}
	if ctx.SelectedText != "" {
		b.WriteString("\n- Selected text: ")
		b.WriteString(ctx.SelectedText)
	}
	if ctx.CursorLine > 0 {
		b.WriteString("\n- Cursor line: ")
		b.WriteString(strconv.Itoa(ctx.CursorLine))
	}
	return b.String()
}
