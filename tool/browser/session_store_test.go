package browser

import "testing"

func TestSessionStoreCRUD(t *testing.T) {
	s := NewSessionStore()
	s.Put(&Session{ID: "a", ConnectURL: "ws://a"})
	if got, ok := s.Get("a"); !ok || got.ConnectURL != "ws://a" {
		t.Fatalf("expected to get a, got %+v ok=%v", got, ok)
	}
	if len(s.List()) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.List()))
	}
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("expected delete to remove")
	}
}
