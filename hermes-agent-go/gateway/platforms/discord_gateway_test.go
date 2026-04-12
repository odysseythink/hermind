package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestDiscordGatewayReceivesMessage(t *testing.T) {
	var identifyReceived int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.CloseNow()

		// Hello (op 10).
		hello := map[string]any{"op": 10, "d": map[string]any{"heartbeat_interval": 30000}}
		buf, _ := json.Marshal(hello)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		// Read Identify (op 2).
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var ident map[string]any
		_ = json.Unmarshal(data, &ident)
		if ident["op"].(float64) == 2 {
			atomic.AddInt32(&identifyReceived, 1)
		}

		// MESSAGE_CREATE dispatch.
		dispatch := map[string]any{
			"op": 0, "t": "MESSAGE_CREATE", "s": 1,
			"d": map[string]any{
				"id": "msg123", "channel_id": "chan456", "content": "hello from discord",
				"author": map[string]any{"id": "user789", "username": "alice"},
			},
		}
		buf, _ = json.Marshal(dispatch)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		<-r.Context().Done()
	}))
	defer srv.Close()

	var replyHits int32
	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&replyHits, 1)
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	dg := NewDiscordGateway("bot-token", "").WithGatewayURL(wsURL).WithBaseURL(restSrv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = dg.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			if in.Text != "hello from discord" {
				t.Errorf("unexpected text: %s", in.Text)
			}
			if in.Platform != "discord" {
				t.Errorf("unexpected platform: %s", in.Platform)
			}
			if in.UserID != "user789" {
				t.Errorf("unexpected user: %s", in.UserID)
			}
			if in.ChatID != "chan456" {
				t.Errorf("unexpected chat: %s", in.ChatID)
			}
			return &gateway.OutgoingMessage{ChatID: in.ChatID, Text: "echo: " + in.Text}, nil
		})
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&replyHits) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if atomic.LoadInt32(&identifyReceived) != 1 {
		t.Error("identify never received")
	}
	if atomic.LoadInt32(&replyHits) < 1 {
		t.Errorf("reply hits = %d, want >= 1", atomic.LoadInt32(&replyHits))
	}
}

func TestDiscordGatewaySendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bot test-tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if m["content"] != "hello" {
			t.Errorf("content = %v", m["content"])
		}
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer srv.Close()

	dg := NewDiscordGateway("test-tok", "C1").WithBaseURL(srv.URL)
	err := dg.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hello"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", atomic.LoadInt32(&hits))
	}
}
