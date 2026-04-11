package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
	"github.com/nousresearch/hermes-agent/tool"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestPermissions(t *testing.T) {
	p := NewPermissions()
	p.Grant("t1", "messages:send", "tools:*")
	if !p.Allow("t1", "messages:send") {
		t.Error("expected allow")
	}
	if !p.Allow("t1", "tools:execute") {
		t.Error("wildcard should allow")
	}
	if p.Allow("t1", "admin:wipe") {
		t.Error("unrelated action should be denied")
	}
	if p.Allow("t2", "anything") {
		t.Error("unknown token should be denied")
	}
	p.Revoke("t1")
	if p.Allow("t1", "messages:send") {
		t.Error("revoke failed")
	}
}

func TestEventBusPubSub(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()
	bus.Publish(Event{Type: "hello", Data: "world"})
	select {
	case ev := <-ch:
		if ev.Type != "hello" || ev.Data != "world" {
			t.Errorf("bad event: %+v", ev)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
	bus.Unsubscribe(ch)
	if _, open := <-ch; open {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestServerFullFlow(t *testing.T) {
	addr := freePort(t)
	reg := tool.NewRegistry()
	perms := NewPermissions()
	perms.Grant("tok", "*")

	handler := func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		return &gateway.OutgoingMessage{Text: "echo: " + in.Text, UserID: in.UserID, ChatID: in.ChatID}, nil
	}

	srv := NewServer(addr, "hermes-test", reg, handler, perms)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Run(ctx) }()

	// Wait for server to accept.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	base := "http://" + addr
	postAuthed := func(path, body string) (*http.Response, error) {
		req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer tok")
		return http.DefaultClient.Do(req)
	}

	// .well-known/agent.json (no auth needed).
	resp, err := http.Get(base + "/.well-known/agent.json")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("well-known: %v %v", err, resp)
	}
	var wellKnown map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&wellKnown)
	resp.Body.Close()
	if wellKnown["name"] != "hermes-test" {
		t.Errorf("name = %v", wellKnown["name"])
	}

	// Create session.
	resp, err = postAuthed("/acp/sessions", `{"user":"alice"}`)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("create session: %v %v", err, resp)
	}
	var sessResp createSessionResponse
	_ = json.NewDecoder(resp.Body).Decode(&sessResp)
	resp.Body.Close()
	if sessResp.SessionID == "" {
		t.Fatal("missing session id")
	}

	// Send message.
	msgBody, _ := json.Marshal(messageRequest{SessionID: sessResp.SessionID, Text: "hi"})
	resp, err = postAuthed("/acp/messages", string(msgBody))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("send message: %v %v", err, resp)
	}
	var msgResp messageResponse
	_ = json.NewDecoder(resp.Body).Decode(&msgResp)
	resp.Body.Close()
	if !strings.Contains(msgResp.Text, "echo: hi") {
		t.Errorf("reply = %q", msgResp.Text)
	}

	// Missing auth is rejected.
	req, _ := http.NewRequest("POST", base+"/acp/messages", bytes.NewBufferString(`{}`))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}
