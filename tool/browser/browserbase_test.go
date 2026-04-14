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

func TestBrowserbaseCreateAndClose(t *testing.T) {
	var creates, releases, debugCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BB-API-Key") != "k" {
			t.Errorf("missing api key header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/sessions" && r.Method == "POST":
			atomic.AddInt32(&creates, 1)
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["projectId"] != "proj" {
				t.Errorf("expected projectId=proj, got %v", body["projectId"])
			}
			_, _ = w.Write([]byte(`{"id":"sess_1","connectUrl":"wss://c/sess_1"}`))
		case strings.HasSuffix(r.URL.Path, "/debug") && r.Method == "GET":
			atomic.AddInt32(&debugCalls, 1)
			_, _ = w.Write([]byte(`{"debuggerFullscreenUrl":"https://live/sess_1"}`))
		case r.URL.Path == "/v1/sessions/sess_1" && r.Method == "POST":
			atomic.AddInt32(&releases, 1)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("BROWSERBASE_API_KEY", "k")
	t.Setenv("BROWSERBASE_PROJECT_ID", "proj")
	p := NewBrowserbase(config.BrowserbaseConfig{BaseURL: srv.URL})
	if !p.IsConfigured() {
		t.Fatal("expected configured")
	}

	ctx := context.Background()
	sess, err := p.CreateSession(ctx)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID != "sess_1" || sess.ConnectURL != "wss://c/sess_1" {
		t.Errorf("bad session: %+v", sess)
	}
	if sess.LiveURL != "https://live/sess_1" {
		t.Errorf("expected live url to be populated, got %q", sess.LiveURL)
	}
	if atomic.LoadInt32(&creates) != 1 {
		t.Errorf("creates = %d", creates)
	}
	if atomic.LoadInt32(&debugCalls) != 1 {
		t.Errorf("debugCalls = %d", debugCalls)
	}
	if err := p.CloseSession(ctx, "sess_1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if atomic.LoadInt32(&releases) != 1 {
		t.Errorf("releases = %d", releases)
	}
}

func TestBrowserbaseNotConfigured(t *testing.T) {
	t.Setenv("BROWSERBASE_API_KEY", "")
	t.Setenv("BROWSERBASE_PROJECT_ID", "")
	p := NewBrowserbase(config.BrowserbaseConfig{})
	if p.IsConfigured() {
		t.Fatal("expected not configured")
	}
	_, err := p.CreateSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
