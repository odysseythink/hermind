package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPProbe_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := httpProbe(context.Background(), "GET", srv.URL, nil); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestHTTPProbe_ForwardsHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := httpProbe(context.Background(), "GET", srv.URL, map[string]string{
		"Authorization": "Bearer abc",
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if gotAuth != "Bearer abc" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer abc")
	}
}

func TestHTTPProbe_NonSuccessStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()
	err := httpProbe(context.Background(), "GET", srv.URL, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestHTTPProbe_RespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := httpProbe(ctx, "GET", srv.URL, nil); err == nil {
		t.Fatal("expected error for canceled context")
	}
}
