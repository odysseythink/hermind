package platforms

import (
	"context"
	"fmt"
	"sync"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/odysseythink/hermind/gateway"
)

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
	return fmt.Errorf("feishu: SendReply not implemented yet")
}

// handleEvent is invoked by the event stream for each inbound event.
func (fa *FeishuApp) handleEvent(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
	return nil
}

// stringPtrValue dereferences a *string safely.
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
