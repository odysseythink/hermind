package gateway

import (
	"context"
	"log/slog"
	"sync"

	"github.com/odysseythink/hermind/message"
)

// Session holds cached conversation state for one (platform, user).
type Session struct {
	ID       string
	Platform string
	UserID   string
	History  []message.Message
}

// LoadHistoryFn rehydrates session history from persistent storage.
// It is called the first time a (platform, userID) pair is requested.
type LoadHistoryFn func(ctx context.Context, platform, userID, sessionID string) ([]message.Message, error)

// SessionStore is an in-memory session cache with an optional
// persistent-storage rehydration hook.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	loader   LoadHistoryFn
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// SetLoader installs a history loader. Must be called before any
// session lookups (typically right after NewSessionStore).
func (s *SessionStore) SetLoader(fn LoadHistoryFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loader = fn
}

// Key returns the map key used to store a session.
func Key(platform, userID string) string {
	return platform + ":" + userID
}

// GetOrCreate returns the existing session or creates a fresh one.
// If a loader is installed and the session is new, the loader is
// invoked to rehydrate history.
func (s *SessionStore) GetOrCreate(ctx context.Context, platform, userID string) *Session {
	s.mu.Lock()
	k := Key(platform, userID)
	sess, ok := s.sessions[k]
	loader := s.loader
	s.mu.Unlock()
	if ok {
		return sess
	}

	sess = &Session{
		ID:       k,
		Platform: platform,
		UserID:   userID,
	}
	if loader != nil {
		history, err := loader(ctx, platform, userID, sess.ID)
		if err != nil {
			slog.WarnContext(ctx, "gateway: session load failed", "platform", platform, "user", userID, "err", err.Error())
		} else {
			sess.History = history
		}
	}

	s.mu.Lock()
	// Handle concurrent create: another goroutine may have already stored one.
	if existing, ok := s.sessions[k]; ok {
		s.mu.Unlock()
		return existing
	}
	s.sessions[k] = sess
	s.mu.Unlock()
	return sess
}

// SetHistory replaces the session history in a single atomic update.
func (s *SessionStore) SetHistory(sess *Session, history []message.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess.History = history
}

// Reset clears the session for a given key.
func (s *SessionStore) Reset(platform, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, Key(platform, userID))
}
