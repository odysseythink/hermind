package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegram_Success(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"testbot"}}`))
	}))
	defer srv.Close()

	err := testTelegram(context.Background(), "12345:abcdef", "", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/bot12345:abcdef/getMe"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestTelegram_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"ok":false,"description":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := testTelegram(context.Background(), "bad", "", srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTelegram_ClosureIsRegistered(t *testing.T) {
	d, ok := Get("telegram")
	if !ok || d.Test == nil {
		t.Fatal("telegram descriptor missing Test closure")
	}
}
