package gateway

import (
	"testing"

	"github.com/nousresearch/hermes-agent/message"
)

func TestSessionStoreGetOrCreateAndSet(t *testing.T) {
	s := NewSessionStore()
	a := s.GetOrCreate("tg", "u1")
	if a == nil || a.UserID != "u1" {
		t.Fatalf("unexpected session: %+v", a)
	}
	b := s.GetOrCreate("tg", "u1")
	if a != b {
		t.Errorf("expected same pointer on repeat call")
	}
	s.SetHistory(a, []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("hi")},
	})
	if len(a.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(a.History))
	}
	s.Reset("tg", "u1")
	c := s.GetOrCreate("tg", "u1")
	if c == a {
		t.Errorf("expected fresh session after reset")
	}
}
