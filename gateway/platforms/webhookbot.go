package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// WebhookBot is the shared outbound-only bot adapter. Platform
// wrappers (Slack, Discord, Feishu, …) supply a name and a function
// that converts an OutgoingMessage into a platform-specific JSON body.
type WebhookBot struct {
	platformName string
	url          string
	buildPayload func(gateway.OutgoingMessage) any
	client       *http.Client
}

// NewWebhookBot builds a WebhookBot given a platform name, webhook URL
// and a payload builder.
func NewWebhookBot(name, url string, buildPayload func(gateway.OutgoingMessage) any) *WebhookBot {
	return &WebhookBot{
		platformName: name,
		url:          url,
		buildPayload: buildPayload,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *WebhookBot) Name() string { return b.platformName }

// Run blocks until ctx is done — webhook bots have no inbound loop.
func (b *WebhookBot) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (b *WebhookBot) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if b.url == "" {
		return fmt.Errorf("%s: no webhook URL configured", b.platformName)
	}
	buf, err := json.Marshal(b.buildPayload(out))
	if err != nil {
		return fmt.Errorf("%s: encode payload: %w", b.platformName, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", b.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", b.platformName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: status %d: %s", b.platformName, resp.StatusCode, string(body))
	}
	return nil
}
