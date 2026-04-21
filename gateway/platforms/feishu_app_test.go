package platforms

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/odysseythink/hermind/gateway"
)

// --- test fakes ---

type fakeStream struct {
	startCh chan struct{}
	err     error
}

func (f *fakeStream) Start(ctx context.Context) error {
	if f.startCh != nil {
		close(f.startCh)
	}
	<-ctx.Done()
	return ctx.Err()
}

type fakeSender struct {
	mu    sync.Mutex
	calls []sendCall
	err   error
}

type sendCall struct {
	chatID string
	text   string
}

func (f *fakeSender) Create(ctx context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sendCall{chatID: chatID, text: text})
	return f.err
}

func (f *fakeSender) recorded() []sendCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sendCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// buildTextEvent returns a minimally populated P2MessageReceiveV1 with
// msg_type=text and the given content payload.
func buildTextEvent(chatID, messageID, openID, contentJSON, msgType string) *larkim.P2MessageReceiveV1 {
	cid := chatID
	mid := messageID
	oid := openID
	mt := msgType
	content := contentJSON
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{OpenId: &oid},
			},
			Message: &larkim.EventMessage{
				MessageId:   &mid,
				ChatId:      &cid,
				MessageType: &mt,
				Content:     &content,
			},
		},
	}
}

func TestFeishuApp_MissingCreds(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]string
	}{
		{"no app_id", map[string]string{"app_secret": "s"}},
		{"no app_secret", map[string]string{"app_id": "a"}},
		{"both empty", map[string]string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fa, err := NewFeishuApp(tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if fa != nil {
				t.Errorf("expected nil FeishuApp, got %v", fa)
			}
			if !strings.Contains(err.Error(), "app_id") && !strings.Contains(err.Error(), "app_secret") {
				t.Errorf("error should mention app_id or app_secret: %v", err)
			}
		})
	}
}

func TestFeishuApp_WebhookURLSurfaced(t *testing.T) {
	fa, err := NewFeishuApp(map[string]string{
		"webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
	})
	if err == nil {
		t.Fatal("expected migration error, got nil")
	}
	if fa != nil {
		t.Errorf("expected nil FeishuApp on migration error, got %v", fa)
	}
	if !strings.Contains(err.Error(), "webhook_url is no longer supported") {
		t.Errorf("error should mention migration: %v", err)
	}
}

func TestFeishuApp_IncomingText(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "")

	var got gateway.IncomingMessage
	var gotCalled bool
	handler := func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		got = in
		gotCalled = true
		return nil, nil
	}
	fa.mu.Lock()
	fa.handler = handler
	fa.mu.Unlock()

	content, _ := json.Marshal(map[string]string{"text": "hello world"})
	evt := buildTextEvent("oc_abc", "om_123", "ou_xyz", string(content), "text")

	if err := fa.handleEvent(context.Background(), evt); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}
	if !gotCalled {
		t.Fatal("handler not called")
	}
	if got.Platform != "feishu" {
		t.Errorf("Platform = %q, want feishu", got.Platform)
	}
	if got.ChatID != "oc_abc" {
		t.Errorf("ChatID = %q, want oc_abc", got.ChatID)
	}
	if got.UserID != "ou_xyz" {
		t.Errorf("UserID = %q, want ou_xyz", got.UserID)
	}
	if got.MessageID != "om_123" {
		t.Errorf("MessageID = %q, want om_123", got.MessageID)
	}
	if got.Text != "hello world" {
		t.Errorf("Text = %q, want hello world", got.Text)
	}
}
