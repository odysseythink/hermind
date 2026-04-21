package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestFeishuApp_StripsAtMention(t *testing.T) {
	fa := newFeishuAppForTest(nil, &fakeSender{}, "")
	var gotText string
	fa.mu.Lock()
	fa.handler = func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		gotText = in.Text
		return nil, nil
	}
	fa.mu.Unlock()

	cases := []struct{ raw, want string }{
		{`{"text":"@_user_1 hello"}`, "hello"},
		{`{"text":"hey @_user_2  how are you"}`, "hey how are you"},
		{`{"text":"@_user_1 @_user_2 hi"}`, "hi"},
		{`{"text":"plain"}`, "plain"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			gotText = ""
			evt := buildTextEvent("oc", "om", "ou", tc.raw, "text")
			if err := fa.handleEvent(context.Background(), evt); err != nil {
				t.Fatalf("handleEvent: %v", err)
			}
			if gotText != tc.want {
				t.Errorf("got %q, want %q", gotText, tc.want)
			}
		})
	}
}

func TestFeishuApp_IgnoresNonText(t *testing.T) {
	fa := newFeishuAppForTest(nil, &fakeSender{}, "")
	called := false
	fa.mu.Lock()
	fa.handler = func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		called = true
		return nil, nil
	}
	fa.mu.Unlock()

	for _, mt := range []string{"image", "file", "sticker", "post", ""} {
		mt := mt
		t.Run("msg_type="+mt, func(t *testing.T) {
			called = false
			evt := buildTextEvent("oc", "om", "ou", `{"text":"ignored"}`, mt)
			if err := fa.handleEvent(context.Background(), evt); err != nil {
				t.Fatalf("handleEvent: %v", err)
			}
			if called {
				t.Errorf("handler called for msg_type=%q; want dropped", mt)
			}
		})
	}
}

func TestFeishuApp_SendReplyToSource(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "oc_default")

	err := fa.SendReply(context.Background(), gateway.OutgoingMessage{
		ChatID: "oc_a",
		Text:   "hi",
	})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	calls := sender.recorded()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].chatID != "oc_a" {
		t.Errorf("chatID = %q, want oc_a (should NOT fall back to default)", calls[0].chatID)
	}
	if calls[0].text != "hi" {
		t.Errorf("text = %q, want hi", calls[0].text)
	}
}

func TestFeishuApp_SendReplyFallback(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "oc_default")

	err := fa.SendReply(context.Background(), gateway.OutgoingMessage{
		ChatID: "", // empty → fallback
		Text:   "push",
	})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	calls := sender.recorded()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].chatID != "oc_default" {
		t.Errorf("chatID = %q, want oc_default", calls[0].chatID)
	}
}

func TestFeishuApp_SendReplyNoTarget(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "") // no default

	err := fa.SendReply(context.Background(), gateway.OutgoingMessage{
		ChatID: "",
		Text:   "orphan",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no target") {
		t.Errorf("error should mention no target: %v", err)
	}
	if len(sender.recorded()) != 0 {
		t.Errorf("sender should not have been called")
	}
}

func TestFeishuApp_ContextCancels(t *testing.T) {
	stream := &fakeStream{startCh: make(chan struct{})}
	fa := newFeishuAppForTest(stream, &fakeSender{}, "")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- fa.Run(ctx, func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return nil, nil
		})
	}()

	<-stream.startCh // wait for stream.Start to be entered
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run should return a non-nil error on ctx cancel (ctx.Err())")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancel")
	}
}

func TestFeishuApp_HandlerErrorDoesNotKillStream(t *testing.T) {
	fa := newFeishuAppForTest(nil, &fakeSender{}, "")

	calls := 0
	fa.mu.Lock()
	fa.handler = func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("handler boom")
		}
		return nil, nil
	}
	fa.mu.Unlock()

	evt1 := buildTextEvent("oc", "om1", "ou", `{"text":"one"}`, "text")
	evt2 := buildTextEvent("oc", "om2", "ou", `{"text":"two"}`, "text")

	if err := fa.handleEvent(context.Background(), evt1); err != nil {
		t.Fatalf("handleEvent#1: %v", err)
	}
	if err := fa.handleEvent(context.Background(), evt2); err != nil {
		t.Fatalf("handleEvent#2: %v", err)
	}
	if calls != 2 {
		t.Errorf("handler called %d times, want 2", calls)
	}
}
