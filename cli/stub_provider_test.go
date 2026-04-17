package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

func TestStubProvider_CompleteReturnsInstructiveError(t *testing.T) {
	s := newStubProvider("qwen")
	_, err := s.Complete(context.Background(), &provider.Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "qwen") || !strings.Contains(msg, "hermind config") {
		t.Errorf("error message should name the provider and point at `hermind config`: %q", msg)
	}
}

func TestStubProvider_StreamReturnsInstructiveError(t *testing.T) {
	s := newStubProvider("")
	_, err := s.Stream(context.Background(), &provider.Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("empty provider name should render as \"unknown\": %q", err.Error())
	}
}

func TestStubProvider_EstimateTokensNoError(t *testing.T) {
	s := newStubProvider("anthropic")
	n, err := s.EstimateTokens("any-model", "hello world")
	if err != nil {
		t.Fatalf("EstimateTokens: %v", err)
	}
	if n <= 0 {
		t.Errorf("expected positive estimate, got %d", n)
	}
}

func TestStubProvider_Available(t *testing.T) {
	s := newStubProvider("anthropic")
	if s.Available() {
		t.Error("stub provider must report Available() == false")
	}
}

func TestStubProvider_SatisfiesInterface(t *testing.T) {
	var _ provider.Provider = newStubProvider("anthropic")
}

func TestBuildPrimaryProvider_MissingKeyReturnsSentinel(t *testing.T) {
	// No API key, no env var — must produce the sentinel so runREPL knows
	// to fall back to the stub.
	t.Setenv("ANTHROPIC_API_KEY", "")
	cfg := &config.Config{Model: "qwen/qwen-flash"}
	_, name, err := buildPrimaryProvider(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errMissingAPIKey) {
		t.Errorf("expected errMissingAPIKey sentinel, got %v", err)
	}
	if name != "qwen" {
		t.Errorf("expected primary name \"qwen\", got %q", name)
	}
}
