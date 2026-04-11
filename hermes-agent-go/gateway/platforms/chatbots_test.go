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

	"github.com/nousresearch/hermes-agent/gateway"
)

type botCase struct {
	name     string
	ctor     func(url string) *WebhookBot
	containsText bool
	// optional extra assertion on decoded JSON body
	assert   func(t *testing.T, body map[string]any)
}

func TestChatbotsSendReply(t *testing.T) {
	cases := []botCase{
		{
			name:         "slack",
			ctor:         NewSlack,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["text"] != "hi" {
					t.Errorf("slack text = %v", body["text"])
				}
			},
		},
		{
			name:         "discord",
			ctor:         NewDiscord,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["content"] != "hi" {
					t.Errorf("discord content = %v", body["content"])
				}
			},
		},
		{
			name:         "mattermost",
			ctor:         NewMattermost,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["text"] != "hi" {
					t.Errorf("mattermost text = %v", body["text"])
				}
			},
		},
		{
			name:         "feishu",
			ctor:         NewFeishu,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["msg_type"] != "text" {
					t.Errorf("feishu msg_type = %v", body["msg_type"])
				}
				c, _ := body["content"].(map[string]any)
				if c == nil || c["text"] != "hi" {
					t.Errorf("feishu content = %v", body["content"])
				}
			},
		},
		{
			name:         "dingtalk",
			ctor:         NewDingTalk,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["msgtype"] != "text" {
					t.Errorf("dingtalk msgtype = %v", body["msgtype"])
				}
				txt, _ := body["text"].(map[string]any)
				if txt == nil || txt["content"] != "hi" {
					t.Errorf("dingtalk text = %v", body["text"])
				}
			},
		},
		{
			name:         "wecom",
			ctor:         NewWeCom,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["msgtype"] != "text" {
					t.Errorf("wecom msgtype = %v", body["msgtype"])
				}
				txt, _ := body["text"].(map[string]any)
				if txt == nil || txt["content"] != "hi" {
					t.Errorf("wecom text = %v", body["text"])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&hits, 1)
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "hi") {
					t.Errorf("%s: expected 'hi' in body: %s", tc.name, body)
				}
				var decoded map[string]any
				_ = json.Unmarshal(body, &decoded)
				if tc.assert != nil {
					tc.assert(t, decoded)
				}
				w.WriteHeader(200)
			}))
			defer srv.Close()

			bot := tc.ctor(srv.URL)
			if bot.Name() != tc.name {
				t.Fatalf("name = %q, want %q", bot.Name(), tc.name)
			}
			if err := bot.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"}); err != nil {
				t.Fatalf("SendReply: %v", err)
			}
			if atomic.LoadInt32(&hits) != 1 {
				t.Errorf("hits = %d", hits)
			}
		})
	}
}

func TestWebhookBotErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()

	b := NewSlack(srv.URL)
	err := b.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 500 error, got %v", err)
	}
}

func TestWebhookBotNoURL(t *testing.T) {
	b := NewSlack("")
	err := b.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}
