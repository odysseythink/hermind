package stdio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/storage"
)

// Session is the in-memory representation of an ACP session. The
// persistent record lives in storage.Storage; this struct holds the
// runtime state (current cwd override, active prompt cancel hook).
type Session struct {
	ID        string
	Cwd       string
	Model     string
	CancelCtx context.CancelFunc // set while a prompt is running
}

// SessionManager owns the ACP session lifecycle. All methods are
// safe for concurrent use.
type SessionManager struct {
	mu       sync.Mutex
	store    storage.Storage
	sessions map[string]*Session
}

// NewSessionManager constructs a manager backed by the given storage.
func NewSessionManager(store storage.Storage) *SessionManager {
	return &SessionManager{
		store:    store,
		sessions: make(map[string]*Session),
	}
}

// Create allocates a new session, persists its metadata with
// Source="acp", and returns the runtime handle.
func (m *SessionManager) Create(ctx context.Context, cwd, model string) (*Session, error) {
	id := uuid.NewString()
	rec := &storage.Session{
		ID:        id,
		Source:    "acp",
		Model:     model,
		StartedAt: time.Now().UTC(),
	}
	if err := m.store.CreateSession(ctx, rec); err != nil {
		return nil, fmt.Errorf("acp/stdio: create session: %w", err)
	}
	s := &Session{ID: id, Cwd: cwd, Model: model}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

// Get fetches a session. If it is not in the in-memory cache, the
// manager restores it from storage (which makes a second-process
// `session/load` Just Work).
func (m *SessionManager) Get(ctx context.Context, id string) (*Session, error) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		return s, nil
	}
	rec, err := m.store.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("acp/stdio: get session %s: %w", id, err)
	}
	s = &Session{ID: rec.ID, Model: rec.Model}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

// SetCancel registers the cancel function for the session's currently
// running prompt. Cancel() will invoke it.
func (m *SessionManager) SetCancel(id string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		s.CancelCtx = cancel
	}
}

// Cancel interrupts the in-flight prompt for the given session, if any.
// It is safe to call when no prompt is active — it becomes a no-op.
func (m *SessionManager) Cancel(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil && s.CancelCtx != nil {
		s.CancelCtx()
	}
}

// AppendUserText adds a user-authored text message to the session
// history.
func (m *SessionManager) AppendUserText(ctx context.Context, id, text string) error {
	return m.store.AddMessage(ctx, id, &storage.StoredMessage{
		SessionID: id,
		Role:      string(message.RoleUser),
		Content:   text,
		Timestamp: time.Now().UTC(),
	})
}

// AppendAssistantText adds an assistant reply to the session history.
func (m *SessionManager) AppendAssistantText(ctx context.Context, id, text string) error {
	return m.store.AddMessage(ctx, id, &storage.StoredMessage{
		SessionID: id,
		Role:      string(message.RoleAssistant),
		Content:   text,
		Timestamp: time.Now().UTC(),
	})
}

// History returns the conversation history as message.Message values
// suitable for passing to provider.Provider.
func (m *SessionManager) History(ctx context.Context, id string) ([]message.Message, error) {
	stored, err := m.store.GetMessages(ctx, id, 1000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]message.Message, 0, len(stored))
	for _, sm := range stored {
		out = append(out, message.Message{
			Role:    message.Role(sm.Role),
			Content: message.TextContent(sm.Content),
		})
	}
	return out, nil
}
