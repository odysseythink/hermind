// agent/memory_manager.go
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
)

// MemoryManager is a cheap sub-capability layer used by the agent to
// (a) observe / digest recent turns, (b) fan sync work out to registered
// external memory providers (Honcho, Mem0, Supermemory, ...), and
// (c) drive cheap LLM work (compress / summarize / retrieve) through an
// AuxClient separate from the main model.
//
// The manager is deliberately minimal: it composes existing pieces
// (Compressor, memprovider.Provider) rather than re-implementing them.
// Safe for concurrent use.
type MemoryManager struct {
	mu         sync.Mutex
	providers  []memprovider.Provider
	recent     []message.HermindMessage // bounded ring of observed turns
	limit      int
	compressor *compression.Compressor // optional — iterative compression
}

// NewMemoryManager constructs a manager. All arguments are optional:
//   - initial may be nil for "no external providers yet"; providers can
//     be added later with AddProvider.
//   - compressor may be nil, in which case Compress returns the input
//     unchanged.
func NewMemoryManager(initial []memprovider.Provider) *MemoryManager {
	return &MemoryManager{
		providers: append([]memprovider.Provider(nil), initial...),
		limit:     20,
	}
}

// SetCompressor attaches the compressor used by Compress. The engine
// typically wires its own compressor here so both paths share config.
func (m *MemoryManager) SetCompressor(c *compression.Compressor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compressor = c
}

// AddProvider registers a new memprovider backend. Providers are queried
// in insertion order.
func (m *MemoryManager) AddProvider(p memprovider.Provider) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, p)
}

// Providers returns a snapshot of registered providers.
func (m *MemoryManager) Providers() []memprovider.Provider {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]memprovider.Provider, len(m.providers))
	copy(out, m.providers)
	return out
}

// ObserveTurn records a message turn for the built-in digest.
func (m *MemoryManager) ObserveTurn(msg message.HermindMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recent = append(m.recent, msg)
	if len(m.recent) > m.limit {
		m.recent = m.recent[len(m.recent)-m.limit:]
	}
}

// BuiltinDigest returns a short, human-readable summary of the recent
// turns observed via ObserveTurn. Used as the always-on contribution
// even when no remote providers are configured. Returns "" when no
// turns have been observed.
func (m *MemoryManager) BuiltinDigest() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recent) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Recent turns:\n")
	for _, msg := range m.recent {
		role := "user"
		if msg.Role == core.MESSAGE_ROLE_ASSISTANT {
			role = "assistant"
		}
		text := msg.Text()
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		fmt.Fprintf(&sb, "- %s: %s\n", role, text)
	}
	return sb.String()
}

// SyncTurn fans a user/assistant turn out to every registered provider.
// Provider failures are collected and returned as a single composite
// error (or nil). A failing backend never blocks the others.
func (m *MemoryManager) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	m.mu.Lock()
	providers := append([]memprovider.Provider(nil), m.providers...)
	m.mu.Unlock()

	var errs []string
	for _, p := range providers {
		if err := p.SyncTurn(ctx, userMsg, assistantMsg); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Compress drives iterative compression through the attached Compressor
// using the provided token budget. When no compressor is set, the
// history is returned unchanged (passes=0, err=nil). This is the hook
// the engine calls when context is near the limit.
func (m *MemoryManager) Compress(ctx context.Context, history []message.HermindMessage, budget int) ([]message.HermindMessage, int, error) {
	m.mu.Lock()
	c := m.compressor
	m.mu.Unlock()
	if c == nil {
		return history, 0, nil
	}
	// Delegate to Compressor.Compress for a single pass; callers that want
	// the multi-pass loop can call into the compressor directly. Keeping
	// the MemoryManager surface simple avoids depending on methods that
	// may not yet exist on Compressor.
	out, err := c.Compress(ctx, history)
	if err != nil {
		return history, 0, err
	}
	if len(out) >= len(history) {
		return out, 0, nil
	}
	return out, 1, nil
}

// Summarize returns the manager's best-effort summary for the given
// text. Today this is a no-op — the compressor handles summarization.
func (m *MemoryManager) Summarize(ctx context.Context, text string) (string, error) {
	_ = ctx
	return text, nil
}

// Retrieve returns the manager's best-effort recall for the given
// query. Today this is just the built-in digest — external
// memprovider.Providers do not expose a unified recall API in hermind,
// so their contributions reach the model via tool calls instead.
func (m *MemoryManager) Retrieve(ctx context.Context, query string) (string, error) {
	_ = ctx
	_ = query
	digest := m.BuiltinDigest()
	return digest, nil
}

// BuildSystemPrompt composes the manager's contribution to the agent's
// system prompt. Today this is just the built-in digest — external
// providers contribute via tool calls during the turn loop. When no
// turns have been observed, returns "".
func (m *MemoryManager) BuildSystemPrompt(ctx context.Context, query string) (string, error) {
	_ = ctx
	_ = query
	digest := m.BuiltinDigest()
	return strings.TrimSpace(digest), nil
}

// Shutdown closes every registered provider. Errors are joined.
func (m *MemoryManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	providers := m.providers
	m.providers = nil
	m.mu.Unlock()
	var errs []string
	for _, p := range providers {
		if err := p.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory shutdown: %s", strings.Join(errs, "; "))
	}
	return nil
}
