package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// Webhook is an outbound-only adapter that POSTs gateway replies to
// a configured URL. It does not receive messages on its own; pair it
// with api_server or another inbound adapter.
type Webhook struct {
	url    string
	token  string
	client *http.Client
}

func NewWebhook(url, token string) *Webhook {
	return &Webhook{
		url:    url,
		token:  token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *Webhook) Name() string { return "webhook" }

// Run blocks until ctx is cancelled — webhook has no inbound loop.
func (w *Webhook) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

type webhookPayload struct {
	UserID string `json:"user_id"`
	ChatID string `json:"chat_id,omitempty"`
	Text   string `json:"text"`
}

func (w *Webhook) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if w.url == "" {
		return errors.New("webhook: no URL configured")
	}
	buf, err := json.Marshal(webhookPayload{
		UserID: out.UserID, ChatID: out.ChatID, Text: out.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.token != "" {
		req.Header.Set("Authorization", "Bearer "+w.token)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
