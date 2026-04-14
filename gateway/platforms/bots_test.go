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

	"github.com/odysseythink/hermind/gateway"
)

func TestDiscordBotSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bot tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if !strings.HasSuffix(r.URL.Path, "/channels/C1/messages") {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(body, &decoded)
		if decoded["content"] != "hi" {
			t.Errorf("content = %v", decoded["content"])
		}
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer srv.Close()

	d := NewDiscordBot("tok", "C1").WithBaseURL(srv.URL)
	err := d.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}

func TestDiscordBotMissingToken(t *testing.T) {
	d := NewDiscordBot("", "C1")
	if err := d.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"}); err == nil {
		t.Error("expected error for missing token")
	}
}

func TestMattermostBotSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if !strings.HasSuffix(r.URL.Path, "/api/v4/posts") {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(body, &decoded)
		if decoded["channel_id"] != "CH1" || decoded["message"] != "hi" {
			t.Errorf("decoded = %+v", decoded)
		}
		_, _ = w.Write([]byte(`{"id":"post1"}`))
	}))
	defer srv.Close()

	mm := NewMattermostBot(srv.URL, "tok", "CH1")
	if err := mm.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"}); err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}

func TestMattermostBotMissingConfig(t *testing.T) {
	mm := NewMattermostBot("", "", "")
	if err := mm.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"}); err == nil {
		t.Error("expected error for missing config")
	}
}
