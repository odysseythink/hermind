package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

func TestSlackEventsURLVerification(t *testing.T) {
	addr := freePort(t)
	s := NewSlackEvents(addr, "xoxb-test")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return nil, nil
		})
	}()
	// Wait for server to accept.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"type":"url_verification","challenge":"xyz"}`
	resp, err := http.Post("http://"+addr+"/slack/events", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["challenge"] != "xyz" {
		t.Errorf("challenge = %q", out["challenge"])
	}
	cancel()
	<-errCh
}

func TestSlackEventsCallbackAndReply(t *testing.T) {
	// Mock the Slack API.
	var postCount int32
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&postCount, 1)
		if !strings.HasSuffix(r.URL.Path, "/api/chat.postMessage") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer xoxb-test" {
			t.Errorf("missing auth")
		}
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(body, &decoded)
		if decoded["channel"] != "C1" || !strings.Contains(decoded["text"].(string), "echo") {
			t.Errorf("unexpected body: %s", body)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer apiSrv.Close()

	addr := freePort(t)
	s := NewSlackEvents(addr, "xoxb-test").WithAPIBase(apiSrv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{UserID: in.UserID, ChatID: in.ChatID, Text: "echo: " + in.Text}, nil
		})
	}()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"type":"event_callback","event":{"type":"message","user":"U1","channel":"C1","text":"hi","client_msg_id":"m1"}}`
	resp, err := http.Post("http://"+addr+"/slack/events", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	// Wait up to 1s for the async dispatch goroutine to POST back.
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&postCount) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if atomic.LoadInt32(&postCount) != 1 {
		t.Errorf("chat.postMessage called %d times, want 1", postCount)
	}
	cancel()
	<-errCh
}

func TestSlackEventsSendReplyAPIError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	defer apiSrv.Close()
	s := NewSlackEvents(":0", "xoxb-test").WithAPIBase(apiSrv.URL)
	err := s.SendReply(context.Background(), gateway.OutgoingMessage{ChatID: "CX", Text: "hi"})
	if err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("expected channel_not_found error, got %v", err)
	}
}
