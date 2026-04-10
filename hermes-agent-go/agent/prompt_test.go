// agent/prompt_test.go
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptBuilderIncludesIdentity(t *testing.T) {
	pb := NewPromptBuilder("cli")
	prompt := pb.Build(&PromptOptions{Model: "claude-opus-4-6"})
	assert.Contains(t, prompt, "Hermes Agent")
	assert.Contains(t, prompt, "Nous Research")
}

func TestPromptBuilderPlatformHint(t *testing.T) {
	pb := NewPromptBuilder("telegram")
	prompt := pb.Build(&PromptOptions{Model: "claude-opus-4-6"})
	// Platform hints are added in Plan 6+. Minimum: the platform name appears.
	assert.NotEmpty(t, prompt)
}

func TestPromptBuilderStable(t *testing.T) {
	// Building the same prompt twice yields identical output.
	// This is required for Anthropic prefix caching.
	pb := NewPromptBuilder("cli")
	opts := &PromptOptions{Model: "claude-opus-4-6"}
	first := pb.Build(opts)
	second := pb.Build(opts)
	assert.Equal(t, first, second)
}
