package compression

import "testing"

func TestContextLengthFor_KnownModel(t *testing.T) {
	// OpenAI models
	if got := ContextLengthFor("gpt-4o"); got != 128000 {
		t.Errorf("ContextLengthFor(gpt-4o) = %d, want 128000", got)
	}
	if got := ContextLengthFor("gpt-4o-mini"); got != 128000 {
		t.Errorf("ContextLengthFor(gpt-4o-mini) = %d, want 128000", got)
	}
	if got := ContextLengthFor("gpt-4-turbo"); got != 128000 {
		t.Errorf("ContextLengthFor(gpt-4-turbo) = %d, want 128000", got)
	}
	if got := ContextLengthFor("gpt-4"); got != 8192 {
		t.Errorf("ContextLengthFor(gpt-4) = %d, want 8192", got)
	}
}

func TestContextLengthFor_UnknownModel(t *testing.T) {
	// Unknown models fall back to conservative default
	if got := ContextLengthFor("some-random-model-v99"); got != 8192 {
		t.Errorf("ContextLengthFor(unknown) = %d, want 8192", got)
	}
}

func TestContextLengthFor_Anthropic(t *testing.T) {
	if got := ContextLengthFor("claude-3-5-sonnet-20241022"); got != 200000 {
		t.Errorf("ContextLengthFor(claude-3-5-sonnet) = %d, want 200000", got)
	}
	if got := ContextLengthFor("claude-3-opus-20240229"); got != 200000 {
		t.Errorf("ContextLengthFor(claude-3-opus) = %d, want 200000", got)
	}
}

func TestContextLengthFor_Gemini(t *testing.T) {
	if got := ContextLengthFor("gemini-1.5-pro"); got != 2097152 {
		t.Errorf("ContextLengthFor(gemini-1.5-pro) = %d, want 2097152", got)
	}
}

func TestContextLengthFor_Ollama(t *testing.T) {
	// Ollama models vary wildly; use conservative default
	if got := ContextLengthFor("llama3.1"); got != 8192 {
		t.Errorf("ContextLengthFor(llama3.1) = %d, want 8192", got)
	}
}
