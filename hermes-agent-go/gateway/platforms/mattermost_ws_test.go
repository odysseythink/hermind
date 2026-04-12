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

func TestMattermostWSReceivesMessage(t *testing.T) {
	var authReceived int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.CloseNow()

		// Send authentication challenge.
		challenge := map[string]any{"event": "authentication_challenge", "status": "OK"}
		buf, _ := json.Marshal(challenge)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		// Read auth response.
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var auth map[string]any
		_ = json.Unmarshal(data, &auth)
		if auth["action"] == "authentication_challenge" {
			atomic.AddInt32(&authReceived, 1)
		}

		// Send a posted event.
		post := map[string]any{"channel_id": "chan1", "id": "post1", "message": "hello from mattermost", "user_id": "user1"}
		postJSON, _ := json.Marshal(post)
		event := map[string]any{"event": "posted", "data": map[string]any{"post": string(postJSON)}}
		buf, _ = json.Marshal(event)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		<-r.Context().Done()
	}))
	defer srv.Close()

	var replyHits int32
	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&replyHits, 1)
		_, _ = w.Write([]byte(`{"id":"post2"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	mm := NewMattermostWS(restSrv.URL, "mm-token", "").WithWebSocketURL(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = mm.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			if in.Text != "hello from mattermost" {
				t.Errorf("unexpected text: %s", in.Text)
			}
			if in.Platform != "mattermost" {
				t.Errorf("unexpected platform: %s", in.Platform)
			}
			if in.UserID != "user1" {
				t.Errorf("unexpected user: %s", in.UserID)
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

	if atomic.LoadInt32(&authReceived) != 1 {
		t.Error("auth never received")
	}
	if atomic.LoadInt32(&replyHits) < 1 {
		t.Errorf("reply hits = %d, want >= 1", atomic.LoadInt32(&replyHits))
	}
}

func TestMattermostWSSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer mm-tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if m["channel_id"] != "CH1" || m["message"] != "hi" {
			t.Errorf("body = %+v", m)
		}
		_, _ = w.Write([]byte(`{"id":"post1"}`))
	}))
	defer srv.Close()

	mm := NewMattermostWS(srv.URL, "mm-tok", "CH1")
	err := mm.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", atomic.LoadInt32(&hits))
	}
}

func TestMattermostWSIgnoresEmptyMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()

		// Auth challenge.
		challenge := map[string]any{"event": "authentication_challenge", "status": "OK"}
		buf, _ := json.Marshal(challenge)
		_ = c.Write(r.Context(), websocket.MessageText, buf)
		// Read auth response.
		_, _, _ = c.Read(r.Context())

		// Send an empty post.
		post := map[string]any{"id": "p1", "channel_id": "ch1", "message": "", "user_id": "u1"}
		postJSON, _ := json.Marshal(post)
		event := map[string]any{"event": "posted", "data": map[string]any{"post": string(postJSON)}}
		buf, _ = json.Marshal(event)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		<-r.Context().Done()
	}))
	defer srv.Close()

	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("SendReply should not be called for empty messages")
		_, _ = w.Write([]byte(`{"id":"p1"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	mm := NewMattermostWS(restSrv.URL, "tok", "").WithWebSocketURL(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_ = mm.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		t.Error("handler should not be called for empty messages")
		return nil, nil
	})
}
