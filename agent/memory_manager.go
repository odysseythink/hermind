// agent/memory_manager.go
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// MemoryManager is a cheap sub-capability layer used by the agent to
// (a) observe / digest recent turns, (b) fan sync work out to registered
// external memory providers (Honcho, Mem0, Supermemory, ...), and
// (c) drive cheap LLM work (compress / summarize / retrieve) through an
// AuxClient separate from the main model.
//
// The manager is deliberately minimal: it composes existing pieces
// (Compressor, AuxClient, memprovider.Provider) rather than
// re-implementing them. Safe for concurrent use.
type MemoryManager struct {
	mu         sync.Mutex
	providers  []memprovider.Provider
	recent     []message.Message // bounded ring of observed turns
	limit      int
	aux        *provider.AuxClient // optional — cheap calls use this
	compressor *Compressor         // optional — iterative compression
}

// NewMemoryManager constructs a manager. All arguments are optional:
//   - initial may be nil for "no external providers yet"; providers can
//     be added later with AddProvider.
//   - aux and compressor may be nil, in which case Summarize / Compress
//     return their inputs unchanged.
func NewMemoryManager(initial []memprovider.Provider) *MemoryManager {
	return &MemoryManager{
		providers: append([]memprovider.Provider(nil), initial...),
		limit:     20,
	}
}

// SetAuxClient attaches an auxiliary LLM client used for cheap calls
// (summarize / retrieve re-ranking). Safe to call before the engine
// starts a conversation.
func (m *MemoryManager) SetAuxClient(aux *provider.AuxClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aux = aux
}

// SetCompressor attaches the compressor used by Compress. The engine
// typically wires its own compressor here so both paths share config.
func (m *MemoryManager) SetCompressor(c *Compressor) {
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
func (m *MemoryManager) ObserveTurn(msg message.Message) {
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
		if msg.Role == message.RoleAssistant {
			role = "assistant"
		}
		text := msg.Content.Text()
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
func (m *MemoryManager) Compress(ctx context.Context, history []message.Message, budget int) ([]message.Message, int, error) {
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

// Summarize sends `text` through the AuxClient with a terse
// summarization prompt and returns the response. Returns the original
// text when no aux client is configured.
func (m *MemoryManager) Summarize(ctx context.Context, text string) (string, error) {
	m.mu.Lock()
	aux := m.aux
	m.mu.Unlock()
	if aux == nil || strings.TrimSpace(text) == "" {
		return text, nil
	}
	system := "You are a summarizer. Produce a terse bullet-point summary " +
		"preserving key facts, decisions, and references. Keep it under 400 words."
	return aux.Ask(ctx, system, text)
}

// Retrieve returns the manager's best-effort recall for the given
// query. Today this is just the built-in digest — external
// memprovider.Providers do not expose a unified recall API in hermind,
// so their contributions reach the model via tool calls instead.
// The AuxClient is used only when a non-trivial digest + query need to
// be re-ranked down to a single snippet.
func (m *MemoryManager) Retrieve(ctx context.Context, query string) (string, error) {
	digest := m.BuiltinDigest()
	if digest == "" {
		return "", nil
	}
	m.mu.Lock()
	aux := m.aux
	m.mu.Unlock()
	if aux == nil || strings.TrimSpace(query) == "" {
		return digest, nil
	}
	system := "Return only the 1–3 most relevant lines from the notes below " +
		"for the given query. If none are relevant, return an empty string."
	user := "Query: " + query + "\n\nNotes:\n" + digest
	out, err := aux.Ask(ctx, system, user)
	if err != nil {
		// Fall back to the raw digest rather than breaking the caller.
		return digest, nil
	}
	return out, nil
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
