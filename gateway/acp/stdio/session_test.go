package stdio

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

// openTestStore opens a fresh in-memory SQLite store with migrations
// applied. Cleanup is registered via t.Cleanup.
func openTestStore(t *testing.T) storage.Storage {
	t.Helper()
	// ":memory:" gives each connection its own DB, which is fine here
	// because sqlite.Open caps MaxOpenConns to 1.
	s, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := s.Migrate(); err != nil {
		_ = s.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSessionManager_CreateAndGet(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	s, err := sm.Create(context.Background(), "/tmp/work", "anthropic/claude-opus-4-6")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" {
		t.Fatal("expected session id")
	}
	got, err := sm.Get(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cwd != "/tmp/work" || got.Model != "anthropic/claude-opus-4-6" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestSessionManager_LoadMissing(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	if _, err := sm.Get(context.Background(), "nope"); err == nil {
		t.Error("expected error for missing session")
	}
}

func TestSessionManager_AppendAndHistory(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	s, err := sm.Create(context.Background(), "/tmp", "m")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := sm.AppendUserText(context.Background(), s.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := sm.AppendAssistantText(context.Background(), s.ID, "hi back"); err != nil {
		t.Fatal(err)
	}
	msgs, err := sm.History(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d", len(msgs))
	}
	if msgs[0].Content.Text() != "hello" || msgs[1].Content.Text() != "hi back" {
		t.Errorf("history = %+v", msgs)
	}
}

func TestSessionManager_CancelIsNoopWhenNoPrompt(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	s, _ := sm.Create(context.Background(), "/tmp", "m")
	// No panic, no data race: just returns.
	sm.Cancel(s.ID)
	sm.Cancel("does-not-exist")
}

func TestSessionManager_RestoresFromStorage(t *testing.T) {
	store := openTestStore(t)
	sm1 := NewSessionManager(store)
	s, err := sm1.Create(context.Background(), "/tmp", "anthropic/claude-opus-4-6")
	if err != nil {
		t.Fatal(err)
	}
	// Fresh manager bound to the same store — simulates a new process.
	sm2 := NewSessionManager(store)
	got, err := sm2.Get(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got.Model != "anthropic/claude-opus-4-6" {
		t.Errorf("restored model = %q", got.Model)
	}
}
