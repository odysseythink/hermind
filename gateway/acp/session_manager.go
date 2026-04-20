package acp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ManagedSession is the richer session state tracked by SessionManager.
// It extends the minimal Session struct with per-session model,
// working directory, and an append-only text transcript suitable for
// feeding back into the ACP prompt handler.
type ManagedSession struct {
	ID        string
	CreatedAt time.Time
	User      string
	Cwd       string
	Model     string
	// Messages stores the plain-text transcript as {role, text} pairs.
	// Kept tiny on purpose — richer structured content belongs in
	// storage.Session, not here.
	Messages []TranscriptEntry
}

// TranscriptEntry is a single user/assistant turn recorded in memory.
type TranscriptEntry struct {
	Role string // "user" or "assistant"
	Text string
}

// SessionManager owns the in-memory session registry used by the
// stdio-style ACP verbs (fork_session, list_sessions, set_session_model).
// It is intentionally decoupled from the existing HTTP Server.sessions
// map so adding it does not disturb pre-existing behavior.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*ManagedSession
}

// NewSessionManager constructs an empty manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: map[string]*ManagedSession{}}
}

// Create allocates and stores a fresh session scoped to cwd and model.
func (m *SessionManager) Create(_ context.Context, cwd, model string) (*ManagedSession, error) {
	s := &ManagedSession{
		ID:        "acp-" + uuid.NewString(),
		CreatedAt: time.Now().UTC(),
		Cwd:       cwd,
		Model:     model,
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s, nil
}

// Get returns the session with the given ID or a not-found error.
func (m *SessionManager) Get(_ context.Context, id string) (*ManagedSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("acp: unknown session %s", id)
	}
	return s, nil
}

// AppendUserText records a user turn onto the transcript.
func (m *SessionManager) AppendUserText(_ context.Context, id, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("acp: unknown session %s", id)
	}
	s.Messages = append(s.Messages, TranscriptEntry{Role: "user", Text: text})
	return nil
}

// AppendAssistantText records an assistant turn onto the transcript.
func (m *SessionManager) AppendAssistantText(_ context.Context, id, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("acp: unknown session %s", id)
	}
	s.Messages = append(s.Messages, TranscriptEntry{Role: "assistant", Text: text})
	return nil
}

// History returns a copy of the session's transcript entries.
func (m *SessionManager) History(_ context.Context, id string) ([]TranscriptEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("acp: unknown session %s", id)
	}
	out := make([]TranscriptEntry, len(s.Messages))
	copy(out, s.Messages)
	return out, nil
}

// Fork copies a session's transcript into a fresh session. The new
// session inherits the source's model and may override cwd; pass ""
// to keep the source's cwd.
func (m *SessionManager) Fork(ctx context.Context, srcID, cwd string) (*ManagedSession, error) {
	m.mu.Lock()
	src, ok := m.sessions[srcID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("acp: unknown session %s", srcID)
	}
	forkCwd := cwd
	if forkCwd == "" {
		forkCwd = src.Cwd
	}
	dst := &ManagedSession{
		ID:        "acp-" + uuid.NewString(),
		CreatedAt: time.Now().UTC(),
		User:      src.User,
		Cwd:       forkCwd,
		Model:     src.Model,
		Messages:  append([]TranscriptEntry(nil), src.Messages...),
	}
	m.sessions[dst.ID] = dst
	m.mu.Unlock()
	_ = ctx
	return dst, nil
}

// List returns every in-memory session in an arbitrary order.
func (m *SessionManager) List(_ context.Context) ([]*ManagedSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*ManagedSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out, nil
}

// SetModel changes a session's active model.
func (m *SessionManager) SetModel(_ context.Context, id, model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("acp: unknown session %s", id)
	}
	s.Model = model
	return nil
}

// SetMode changes a session's mode (e.g. "chat" vs "agent"). Modes
// are free-form strings; the manager only records the latest value.
func (m *SessionManager) SetMode(_ context.Context, id, mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("acp: unknown session %s", id)
	}
	// Stash mode onto the transcript as a system-style marker so it
	// survives Fork without requiring an extra field. Callers that
	// need to read the mode can scan for this marker.
	s.Messages = append(s.Messages, TranscriptEntry{Role: "mode", Text: mode})
	return nil
}
