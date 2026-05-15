package pantheonadapter

import (
	"testing"

	"github.com/odysseythink/hermind/provider"
)

func TestModelInfoResolver_Lookup_ExactMatch(t *testing.T) {
	resolver := NewModelInfoResolver()
	info := resolver.Lookup("gpt-4o")

	if info == nil {
		t.Fatal("expected non-nil ModelInfo for exact match")
	}
	if info.ContextLength != 128000 {
		t.Errorf("expected ContextLength 128000, got %d", info.ContextLength)
	}
	if info.MaxOutputTokens != 16384 {
		t.Errorf("expected MaxOutputTokens 16384, got %d", info.MaxOutputTokens)
	}
	if !info.SupportsVision {
		t.Error("expected SupportsVision to be true")
	}
	if !info.SupportsTools {
		t.Error("expected SupportsTools to be true")
	}
}

func TestModelInfoResolver_Lookup_SubstringMatch(t *testing.T) {
	resolver := NewModelInfoResolver()
	info := resolver.Lookup("claude-opus-4-20250514")

	if info == nil {
		t.Fatal("expected non-nil ModelInfo for substring match")
	}
	if info.ContextLength != 200000 {
		t.Errorf("expected ContextLength 200000, got %d", info.ContextLength)
	}
	if info.MaxOutputTokens != 8192 {
		t.Errorf("expected MaxOutputTokens 8192, got %d", info.MaxOutputTokens)
	}
	if !info.SupportsVision {
		t.Error("expected SupportsVision to be true")
	}
	if !info.SupportsTools {
		t.Error("expected SupportsTools to be true")
	}
	if !info.SupportsCaching {
		t.Error("expected SupportsCaching to be true")
	}
}

func TestModelInfoResolver_Lookup_UnknownModel(t *testing.T) {
	resolver := NewModelInfoResolver()
	info := resolver.Lookup("some-unknown-model-v99")

	if info == nil {
		t.Fatal("expected non-nil ModelInfo for unknown model")
	}

	expected := provider.ModelInfo{
		ContextLength:   128000,
		MaxOutputTokens: 4096,
		SupportsTools:   true,
	}

	if *info != expected {
		t.Errorf("expected %+v, got %+v", expected, *info)
	}
}

func TestModelInfoResolver_EstimateTokens_Empty(t *testing.T) {
	resolver := NewModelInfoResolver()
	if got := resolver.EstimateTokens(""); got != 0 {
		t.Errorf("expected 0 for empty string, got %d", got)
	}
}

func TestModelInfoResolver_EstimateTokens_NonEmpty(t *testing.T) {
	resolver := NewModelInfoResolver()
	// "hello world" has 11 characters.
	// (11 + 3) / 4 = 14 / 4 = 3
	if got := resolver.EstimateTokens("hello world"); got != 3 {
		t.Errorf("expected 3 for 'hello world', got %d", got)
	}
}

func TestModelInfoResolver_Lookup_EmptyModelID(t *testing.T) {
	resolver := NewModelInfoResolver()
	info := resolver.Lookup("")
	if info == nil {
		t.Fatal("expected non-nil ModelInfo for empty model ID")
	}
	expected := provider.ModelInfo{
		ContextLength:   128000,
		MaxOutputTokens: 4096,
		SupportsTools:   true,
	}
	if *info != expected {
		t.Errorf("expected %+v, got %+v", expected, *info)
	}
}
