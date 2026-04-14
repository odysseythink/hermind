package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestCamofoxCreateAndClose(t *testing.T) {
	var creates, closes int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/sessions" && r.Method == "POST":
			atomic.AddInt32(&creates, 1)
			_, _ = w.Write([]byte(`{"id":"sess_1","cdp_url":"ws://localhost:9222/devtools/browser/abc","vnc_url":"http://localhost:5900"}`))
		case strings.HasPrefix(r.URL.Path, "/sessions/sess_1") && r.Method == "DELETE":
			atomic.AddInt32(&closes, 1)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewCamofox(config.CamofoxConfig{BaseURL: srv.URL})
	if !p.IsConfigured() {
		t.Fatal("expected configured")
	}
	if p.Name() != "camofox" {
		t.Errorf("Name() = %q", p.Name())
	}

	ctx := context.Background()
	sess, err := p.CreateSession(ctx)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID != "sess_1" {
		t.Errorf("id = %q", sess.ID)
	}
	if sess.ConnectURL != "ws://localhost:9222/devtools/browser/abc" {
		t.Errorf("connect_url = %q", sess.ConnectURL)
	}
	if sess.LiveURL != "http://localhost:5900" {
		t.Errorf("live_url = %q", sess.LiveURL)
	}
	if sess.Provider != "camofox" {
		t.Errorf("provider = %q", sess.Provider)
	}
	if atomic.LoadInt32(&creates) != 1 {
		t.Errorf("creates = %d", creates)
	}
	if err := p.CloseSession(ctx, "sess_1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if atomic.LoadInt32(&closes) != 1 {
		t.Errorf("closes = %d", closes)
	}
}

func TestCamofoxNotConfigured(t *testing.T) {
	p := NewCamofox(config.CamofoxConfig{})
	// Default BaseURL is http://localhost:9377, so IsConfigured() is true.
	// With an empty BaseURL explicitly set, it should still default.
	if !p.IsConfigured() {
		t.Fatal("expected configured with default BaseURL")
	}
}

func TestCamofoxLiveURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/sessions/sess_1" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"vnc_url":"http://localhost:5900/sess_1"}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewCamofox(config.CamofoxConfig{BaseURL: srv.URL})
	url, err := p.LiveURL(context.Background(), "sess_1")
	if err != nil {
		t.Fatalf("LiveURL: %v", err)
	}
	if url != "http://localhost:5900/sess_1" {
		t.Errorf("url = %q", url)
	}
}

func TestCamofoxManagedPersistence(t *testing.T) {
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/sessions" && r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
			_, _ = w.Write([]byte(`{"id":"sess_1","cdp_url":"ws://x","vnc_url":""}`))
		}
	}))
	defer srv.Close()

	p := NewCamofox(config.CamofoxConfig{
		BaseURL:            srv.URL,
		ManagedPersistence: true,
	})
	_, _ = p.CreateSession(context.Background())
	if lastBody["persist"] != true {
		t.Errorf("expected persist=true, got %v", lastBody["persist"])
	}
}

// Compile-time interface check.
var _ Provider = (*CamofoxProvider)(nil)
