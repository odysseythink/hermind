package gateway

import (
	"sync"

	"github.com/nousresearch/hermes-agent/message"
)

// Session holds cached conversation state for one (platform, user).
type Session struct {
	ID       string
	Platform string
	UserID   string
	History  []message.Message
}

// SessionStore is an in-memory session cache. Plan 7b should move this
// to SQLite for durability.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// Key returns the map key used to store a session.
func Key(platform, userID string) string {
	return platform + ":" + userID
}

// GetOrCreate returns the existing session or creates a fresh one.
func (s *SessionStore) GetOrCreate(platform, userID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := Key(platform, userID)
	sess, ok := s.sessions[k]
	if !ok {
		sess = &Session{
			ID:       k,
			Platform: platform,
			UserID:   userID,
		}
		s.sessions[k] = sess
	}
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
