package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// WhatsApp is an outbound-only adapter for the WhatsApp Business
// Cloud API: POST {base}/{version}/{phone_id}/messages with a
// Bearer access token.
type WhatsApp struct {
	BaseURL     string // default https://graph.facebook.com
	APIVersion  string // default v19.0
	PhoneID     string
	AccessToken string
	client      *http.Client
}

func NewWhatsApp(phoneID, accessToken string) *WhatsApp {
	return &WhatsApp{
		BaseURL:     "https://graph.facebook.com",
		APIVersion:  "v19.0",
		PhoneID:     phoneID,
		AccessToken: accessToken,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBaseURL overrides the Graph API base URL for tests.
func (w *WhatsApp) WithBaseURL(u string) *WhatsApp {
	w.BaseURL = strings.TrimRight(u, "/")
	return w
}

func (w *WhatsApp) Name() string { return "whatsapp" }

func (w *WhatsApp) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (w *WhatsApp) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if w.PhoneID == "" || w.AccessToken == "" || out.ChatID == "" {
		return fmt.Errorf("whatsapp: phone_id/access_token/chat_id required")
	}
	url := fmt.Sprintf("%s/%s/%s/messages", w.BaseURL, w.APIVersion, w.PhoneID)
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                out.ChatID,
		"type":              "text",
		"text":              map[string]string{"body": out.Text},
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.AccessToken)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("whatsapp: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
