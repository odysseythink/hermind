// Package copilot implements provider.Provider by driving the GitHub
// Copilot CLI over its ACP stdio protocol. It assumes the user has
// already authenticated with `gh auth login` + enabled Copilot CLI.
// Hermind never touches the underlying token directly.
package copilot

import (
	"os"
	"strings"
	"sync"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

const (
	defaultCommand = "copilot"
)

// Copilot is the provider.Provider implementation. It is safe for a
// single in-flight Complete or Stream call; concurrent calls will
// queue on the writer lock.
type Copilot struct {
	command string
	args    []string

	// Subprocess state is lazy-initialized on first use.
	mu  sync.Mutex
	sub *subprocess // nil until first call

	// initialized is set after the handshake succeeds for the current
	// subprocess. Reset alongside sub in Close().
	initialized bool
	sessionID   string
}

// New constructs a Copilot provider from config.
//
// Override the command + args via env:
//
//	HERMIND_COPILOT_COMMAND=path/to/copilot
//	HERMIND_COPILOT_ARGS="--acp --stdio --other"  (space-separated)
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	cmd := os.Getenv("HERMIND_COPILOT_COMMAND")
	if cmd == "" {
		cmd = defaultCommand
	}
	args := []string{"--acp", "--stdio"}
	if v, ok := os.LookupEnv("HERMIND_COPILOT_ARGS"); ok {
		args = splitArgs(v)
	}
	_ = cfg // unused — copilot has no config knobs today
	return &Copilot{command: cmd, args: args}, nil
}

// Name returns "copilot".
func (c *Copilot) Name() string { return "copilot" }

// Available returns true when a command path is configured. We don't
// probe the subprocess eagerly; fallback layers distinguish "not
// configured" from "configured but failing" only once a request runs.
func (c *Copilot) Available() bool { return c.command != "" }

// ModelInfo returns a conservative default — Copilot CLI does not
// expose a tokenization API so these numbers are best-effort.
func (c *Copilot) ModelInfo(model string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     32_000,
		MaxOutputTokens:   4_096,
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsVision:    false,
		SupportsCaching:   false,
	}
}

// EstimateTokens uses the ~4-chars-per-token rule of thumb.
func (c *Copilot) EstimateTokens(_ string, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	return (len(text) + 3) / 4, nil
}

// Close terminates the subprocess if one is running. Callers that
// embed the provider inside a fallback chain should call Close
// during shutdown.
func (c *Copilot) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sub == nil {
		return nil
	}
	err := c.sub.Close()
	c.sub = nil
	c.initialized = false
	c.sessionID = ""
	return err
}

// splitArgs splits on spaces, ignoring runs of whitespace. This is
// deliberately naive — quoting support is explicitly out of scope
// (users with exotic needs can patch the command directly).
func splitArgs(s string) []string {
	out := make([]string, 0, 4)
	for _, p := range strings.Fields(s) {
		out = append(out, p)
	}
	return out
}
