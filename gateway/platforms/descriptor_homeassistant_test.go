package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeAssistant_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"message":"API running."}`))
	}))
	defer srv.Close()

	if err := testHomeAssistant(context.Background(), srv.URL, "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/api/" || gotAuth != "Bearer tok" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}

func TestHomeAssistant_MissingCreds(t *testing.T) {
	if err := testHomeAssistant(context.Background(), "", "tok"); err == nil {
		t.Error("expected error for empty base_url")
	}
	if err := testHomeAssistant(context.Background(), "http://x", ""); err == nil {
		t.Error("expected error for empty access_token")
	}
}

func TestHomeAssistant_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"401"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	if err := testHomeAssistant(context.Background(), srv.URL, "bad"); err == nil {
		t.Error("expected error on 401")
	}
}
