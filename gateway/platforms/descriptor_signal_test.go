package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignal_Success(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"version":"0.13.0"}`))
	}))
	defer srv.Close()

	if err := testSignal(context.Background(), srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/v1/about" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestSignal_MissingBaseURL(t *testing.T) {
	if err := testSignal(context.Background(), ""); err == nil {
		t.Error("expected error")
	}
}
