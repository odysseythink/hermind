package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatrix_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"user_id":"@u:m"}`))
	}))
	defer srv.Close()

	if err := testMatrix(context.Background(), srv.URL, "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/_matrix/client/v3/account/whoami" || gotAuth != "Bearer tok" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}

func TestMatrix_TrimsTrailingSlashOnHomeServer(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := testMatrix(context.Background(), srv.URL+"/", "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/_matrix/client/v3/account/whoami" {
		t.Errorf("path = %q (double slash?)", gotPath)
	}
}

func TestMatrix_MissingCreds(t *testing.T) {
	if err := testMatrix(context.Background(), "", "tok"); err == nil {
		t.Error("expected error for empty home_server")
	}
	if err := testMatrix(context.Background(), "http://x", ""); err == nil {
		t.Error("expected error for empty access_token")
	}
}

func TestMatrix_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"errcode":"M_UNKNOWN_TOKEN"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	if err := testMatrix(context.Background(), srv.URL, "bad"); err == nil {
		t.Error("expected error on 401")
	}
}
