package gateway

import (
	"context"
	"testing"

	"github.com/nousresearch/hermes-agent/message"
)

func TestSessionStoreGetOrCreateAndSet(t *testing.T) {
	ctx := context.Background()
	s := NewSessionStore()
	a := s.GetOrCreate(ctx, "tg", "u1")
	if a == nil || a.UserID != "u1" {
		t.Fatalf("unexpected session: %+v", a)
	}
	b := s.GetOrCreate(ctx, "tg", "u1")
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
	c := s.GetOrCreate(ctx, "tg", "u1")
	if c == a {
		t.Errorf("expected fresh session after reset")
	}
}

func TestSessionStoreLoader(t *testing.T) {
	ctx := context.Background()
	s := NewSessionStore()
	called := 0
	s.SetLoader(func(ctx context.Context, platform, userID, sessionID string) ([]message.Message, error) {
		called++
		return []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("prior turn")},
		}, nil
	})
	a := s.GetOrCreate(ctx, "tg", "u2")
	if len(a.History) != 1 {
		t.Errorf("expected loader-supplied history, got %d entries", len(a.History))
	}
	if called != 1 {
		t.Errorf("loader called %d times, want 1", called)
	}
	// Second call should not invoke the loader (cache hit).
	_ = s.GetOrCreate(ctx, "tg", "u2")
	if called != 1 {
		t.Errorf("loader called %d times on cache hit, want 1", called)
	}
}
