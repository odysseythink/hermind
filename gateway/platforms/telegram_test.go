package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

func TestTelegramPollingAndReply(t *testing.T) {
	var getUpdatesHits, sendMessageHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "getUpdates"):
			hits := atomic.AddInt32(&getUpdatesHits, 1)
			if hits == 1 {
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":{"message_id":10,"from":{"id":42,"username":"alice"},"chat":{"id":99},"text":"hello bot"}}]}`))
			} else {
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
			}
		case strings.Contains(r.URL.Path, "sendMessage"):
			atomic.AddInt32(&sendMessageHits, 1)
			body, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(body, &m)
			if m["text"] != "echo: hello bot" {
				t.Errorf("unexpected reply text: %v", m["text"])
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tg, err := NewTelegram("bot-token", "")
	if err != nil {
		t.Fatalf("NewTelegram: %v", err)
	}
	tg = tg.WithBaseURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = tg.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{
				UserID: in.UserID,
				ChatID: in.ChatID,
				Text:   "echo: " + in.Text,
			}, nil
		})
		close(done)
	}()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&sendMessageHits) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if atomic.LoadInt32(&getUpdatesHits) == 0 {
		t.Error("getUpdates never called")
	}
	if atomic.LoadInt32(&sendMessageHits) != 1 {
		t.Errorf("sendMessage hits = %d", sendMessageHits)
	}
}

func TestNewTelegramClient_Direct(t *testing.T) {
	c, err := newTelegramClient("", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", c.Timeout)
	}
	if c.Transport != http.DefaultTransport {
		t.Errorf("Transport = %T, want http.DefaultTransport", c.Transport)
	}
}

func TestNewTelegramClient_HTTP(t *testing.T) {
	const proxyURL = "http://127.0.0.1:8080"
	c, err := newTelegramClient(proxyURL, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", c.Transport)
	}
	if tr.Proxy == nil {
		t.Fatal("Transport.Proxy is nil — http proxy not wired")
	}
	req, _ := http.NewRequest("GET", "https://api.telegram.org/", nil)
	got, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy() error: %v", err)
	}
	if got == nil || got.String() != proxyURL {
		t.Errorf("Proxy() = %v, want %q", got, proxyURL)
	}
}

func TestNewTelegramClient_SOCKS5(t *testing.T) {
	const proxyURL = "socks5://127.0.0.1:1080"
	c, err := newTelegramClient(proxyURL, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", c.Transport)
	}
	if tr.DialContext == nil {
		t.Error("Transport.DialContext is nil — socks5 dialer not wired")
	}
	if tr.Proxy != nil {
		t.Error("Transport.Proxy is non-nil for socks5 — expected DialContext only")
	}
}

func TestNewTelegramClient_InvalidScheme(t *testing.T) {
	_, err := newTelegramClient("ftp://host:21", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for ftp scheme, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported proxy scheme") {
		t.Errorf("error = %q, want to contain \"unsupported proxy scheme\"", err.Error())
	}
}
