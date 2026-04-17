package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/odysseythink/hermind/provider"
)

// errMissingAPIKey is returned by buildPrimaryProvider when the primary
// provider has no API key. runREPL treats it as "start in degraded mode"
// rather than a hard failure.
var errMissingAPIKey = errors.New("hermind: primary provider has no api_key")

// stubProvider is a provider.Provider that does nothing successfully — it
// lets the REPL boot even when the user hasn't configured a real LLM yet.
// Any Complete/Stream call returns a clear instruction pointing at the
// config TUI.
type stubProvider struct {
	name string // provider name the user *tried* to use, for the error message
}

func newStubProvider(name string) *stubProvider {
	if name == "" {
		name = "unknown"
	}
	return &stubProvider{name: name}
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return nil, s.notConfiguredErr()
}

func (s *stubProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return nil, s.notConfiguredErr()
}

func (s *stubProvider) ModelInfo(_ string) *provider.ModelInfo { return nil }

func (s *stubProvider) EstimateTokens(_ string, text string) (int, error) {
	// Rough heuristic so token-budget code elsewhere doesn't crash.
	return len(text) / 4, nil
}

func (s *stubProvider) Available() bool { return false }

func (s *stubProvider) notConfiguredErr() error {
	return fmt.Errorf(
		"no LLM configured (provider %q has no api_key). "+
			"Run `hermind config` or open the web UI with `hermind config --web` to add one.",
		s.name,
	)
}
