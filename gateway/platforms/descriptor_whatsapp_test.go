package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhatsApp_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"42"}`))
	}))
	defer srv.Close()

	if err := testWhatsApp(context.Background(), "42", "abc", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/v20.0/42" || gotAuth != "Bearer abc" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}

func TestWhatsApp_MissingCreds(t *testing.T) {
	if err := testWhatsApp(context.Background(), "", "abc", "http://unused"); err == nil {
		t.Error("expected error for empty phone_id")
	}
	if err := testWhatsApp(context.Background(), "42", "", "http://unused"); err == nil {
		t.Error("expected error for empty access_token")
	}
}
