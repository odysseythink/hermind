package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackEvents_Success(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if err := testSlackEvents(context.Background(), "xoxb-abc", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/auth.test" || gotAuth != "Bearer xoxb-abc" {
		t.Errorf("got method=%q path=%q auth=%q", gotMethod, gotPath, gotAuth)
	}
}

func TestSlackEvents_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"ok":false,"error":"invalid_auth"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	if err := testSlackEvents(context.Background(), "bad", srv.URL); err == nil {
		t.Fatal("expected error")
	}
}
