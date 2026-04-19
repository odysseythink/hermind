package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordBot_Success(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/users/@me" {
			http.Error(w, "wrong path", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	if err := testDiscordBot(context.Background(), "tok", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotAuth != "Bot tok" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bot tok")
	}
}

func TestDiscordBot_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"401: Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	if err := testDiscordBot(context.Background(), "bad", srv.URL); err == nil {
		t.Fatal("expected error")
	}
}
