// agent/prompt.go
package agent

import "strings"

// defaultIdentity is the base personality/identity block.
// Ported from the Python hermes agent/prompt_builder.py DEFAULT_AGENT_IDENTITY.
const defaultIdentity = `You are Hermes Agent, created by Nous Research.

You are a helpful, knowledgeable AI assistant. You are direct and efficient.
You respond with markdown formatting when it aids clarity.`

// PromptOptions parameterize prompt generation.
// In later plans this will expand to include memory, skills, context files, etc.
type PromptOptions struct {
	Model       string
	SkipContext bool
}

// PromptBuilder assembles system prompts for the agent engine.
// Stateless — safe to share a single instance across conversations.
type PromptBuilder struct {
	platform string
}

// NewPromptBuilder creates a PromptBuilder for a specific platform.
// Valid platforms: "cli", "telegram", "discord", etc.
func NewPromptBuilder(platform string) *PromptBuilder {
	return &PromptBuilder{platform: platform}
}

// Build assembles the system prompt. The output is stable for equivalent
// inputs — this is required for Anthropic prefix caching to work.
func (pb *PromptBuilder) Build(opts *PromptOptions) string {
	var parts []string
	parts = append(parts, defaultIdentity)

	// Context files, memory guidance, skills guidance, platform hints,
	// and injection protection are added in later plans.
	// For Plan 1 we just want a stable minimal prompt.

	return strings.Join(parts, "\n\n")
}
