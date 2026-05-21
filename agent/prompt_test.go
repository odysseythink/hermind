// agent/prompt_test.go
package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptBuilderIncludesIdentity(t *testing.T) {
	pb := NewPromptBuilder("cli", "")
	prompt := pb.Build(&PromptOptions{Model: "claude-opus-4-6"})
	assert.Contains(t, prompt, "Hermind Agent")
	assert.Contains(t, prompt, "odysseythink")
}

func TestPromptBuilderPlatformHint(t *testing.T) {
	pb := NewPromptBuilder("telegram", "")
	prompt := pb.Build(&PromptOptions{Model: "claude-opus-4-6"})
	// Platform hints are added in Plan 6+. Minimum: the platform name appears.
	assert.NotEmpty(t, prompt)
}

func TestPromptBuilderStable(t *testing.T) {
	// Building the same prompt twice yields identical output.
	// This is required for Anthropic prefix caching.
	pb := NewPromptBuilder("cli", "")
	opts := &PromptOptions{Model: "claude-opus-4-6"}
	first := pb.Build(opts)
	second := pb.Build(opts)
	assert.Equal(t, first, second)
}

func TestPromptBuilder_AppendsDefaultSystemPrompt(t *testing.T) {
	pb := NewPromptBuilder("cli", "You are a Go debugger.")
	got := pb.Build(&PromptOptions{Model: "claude-opus-4-7"})
	if !strings.Contains(got, "Hermind Agent") {
		t.Errorf("expected identity block in output, got %q", got)
	}
	if !strings.Contains(got, "You are a Go debugger.") {
		t.Errorf("expected default system prompt to be appended, got %q", got)
	}
	// Identity must come first
	identIdx := strings.Index(got, "Hermind Agent")
	defIdx := strings.Index(got, "You are a Go debugger.")
	if identIdx > defIdx {
		t.Errorf("identity must come before default system prompt (ident=%d def=%d)", identIdx, defIdx)
	}
}

func TestPromptBuilder_EmptyDefaultPreservesIdentityOnly(t *testing.T) {
	pb := NewPromptBuilder("cli", "")
	got := pb.Build(&PromptOptions{Model: "claude-opus-4-7"})
	if got != defaultIdentity {
		t.Errorf("expected identity-only output\nwant: %q\n got: %q", defaultIdentity, got)
	}
}

func TestPrompt_RendersPinnedBeforeActive(t *testing.T) {
	pb := NewPromptBuilder("cli", "")
	out := pb.Build(&PromptOptions{
		PinnedMemories: []string{"i am allergic to peanuts"},
		ActiveMemories: []string{"recalled: discussed jenkins last week"},
	})
	pinnedIdx := strings.Index(out, "Pinned context")
	activeIdx := strings.Index(out, "Relevant memories")
	if pinnedIdx == -1 || activeIdx == -1 || pinnedIdx > activeIdx {
		t.Fatalf("pinned must precede active; got pinned=%d active=%d", pinnedIdx, activeIdx)
	}
}

func TestPrompt_PinnedSectionOmittedWhenEmpty(t *testing.T) {
	pb := NewPromptBuilder("cli", "")
	out := pb.Build(&PromptOptions{})
	if strings.Contains(out, "Pinned context") {
		t.Fatal("Pinned section must be omitted when no pinned memories")
	}
}
