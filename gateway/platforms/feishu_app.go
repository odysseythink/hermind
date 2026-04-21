package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/odysseythink/hermind/gateway"
)

var feishuMentionRE = regexp.MustCompile(`@_user_\d+\s*`)

// feishuEventStream is the inbound seam. Production wraps *larkws.Client.
// Start blocks until ctx is cancelled and delivers events via the handler
// registered at construction of the concrete implementation.
type feishuEventStream interface {
	Start(ctx context.Context) error
}

// feishuMessageSender is the outbound seam. Production wraps
// *lark.Client.Im.Message.Create.
type feishuMessageSender interface {
	Create(ctx context.Context, chatID, text string) error
}

// FeishuApp is the bidirectional Feishu / Lark adapter backed by a
// self-built app long-connection.
type FeishuApp struct {
	appID         string
	appSecret     string
	domain        string // "feishu" or "lark"
	encryptKey    string
	defaultChatID string

	stream feishuEventStream
	sender feishuMessageSender

	mu      sync.Mutex
	handler gateway.MessageHandler
}

// NewFeishuApp constructs the adapter using production SDK-backed stream
// and sender. Returns an error when required options are missing or when
// legacy webhook_url is present without app_id (migration guard).
func NewFeishuApp(opts map[string]string) (*FeishuApp, error) {
	appID := opts["app_id"]
	appSecret := opts["app_secret"]
	if appID == "" && opts["webhook_url"] != "" {
		return nil, fmt.Errorf("feishu: webhook_url is no longer supported; migrate to a self-built app (see CHANGELOG)")
	}
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("feishu: missing app_id or app_secret")
	}
	fa := &FeishuApp{
		appID:         appID,
		appSecret:     appSecret,
		domain:        opts["domain"],
		encryptKey:    opts["encrypt_key"],
		defaultChatID: opts["default_chat_id"],
	}
	return fa, nil
}

// newFeishuAppForTest builds an adapter with injected seams. Unexported
// so tests in this package are the only callers.
func newFeishuAppForTest(stream feishuEventStream, sender feishuMessageSender, defaultChatID string) *FeishuApp {
	return &FeishuApp{
		stream:        stream,
		sender:        sender,
		defaultChatID: defaultChatID,
	}
}

// Name returns the canonical platform name.
func (fa *FeishuApp) Name() string { return "feishu" }

// Run starts the long-connection event loop. It blocks until ctx is
// cancelled or the stream errors out.
func (fa *FeishuApp) Run(ctx context.Context, h gateway.MessageHandler) error {
	return fmt.Errorf("feishu: Run not implemented yet")
}

// SendReply posts a text message to the target chat. When out.ChatID is
// empty, falls back to the adapter's default_chat_id.
func (fa *FeishuApp) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	target := out.ChatID
	if target == "" {
		target = fa.defaultChatID
	}
	if target == "" {
		return fmt.Errorf("feishu: no target chat_id (out.ChatID empty and default_chat_id not set)")
	}
	if fa.sender == nil {
		return fmt.Errorf("feishu: sender not initialised")
	}
	return fa.sender.Create(ctx, target, out.Text)
}

// handleEvent is invoked by the event stream for each inbound event.
func (fa *FeishuApp) handleEvent(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
	if evt == nil || evt.Event == nil || evt.Event.Message == nil {
		return nil
	}
	msg := evt.Event.Message
	if stringPtrValue(msg.MessageType) != "text" {
		return nil
	}

	var content struct {
		Text string `json:"text"`
	}
	raw := stringPtrValue(msg.Content)
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return nil // malformed content — drop, don't kill the stream
	}

	openID := ""
	if evt.Event.Sender != nil && evt.Event.Sender.SenderId != nil {
		openID = stringPtrValue(evt.Event.Sender.SenderId.OpenId)
	}

	cleaned := strings.TrimSpace(feishuMentionRE.ReplaceAllString(content.Text, ""))
	// collapse double spaces left behind between two mentions
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	in := gateway.IncomingMessage{
		Platform:  "feishu",
		UserID:    openID,
		ChatID:    stringPtrValue(msg.ChatId),
		Text:      cleaned,
		MessageID: stringPtrValue(msg.MessageId),
	}

	fa.mu.Lock()
	h := fa.handler
	fa.mu.Unlock()
	if h == nil {
		return nil
	}
	out, err := h(ctx, in)
	if err != nil {
		return nil // handler errors logged at gateway layer; don't kill stream
	}
	if out == nil {
		return nil
	}
	_ = fa.SendReply(ctx, *out) // SendReply errors also don't kill stream
	return nil
}

// stringPtrValue dereferences a *string safely.
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
