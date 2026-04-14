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

// HomeAssistant is an outbound-only adapter for a Home Assistant
// instance using the REST /api/services/notify/{service} endpoint.
// Inbound via HA webhooks is deferred — pair with api_server.
type HomeAssistant struct {
	BaseURL     string // e.g. http://homeassistant.local:8123
	AccessToken string // Long-lived token
	Service     string // notify service name, e.g. "mobile_app_phone"
	client      *http.Client
}

func NewHomeAssistant(baseURL, accessToken, service string) *HomeAssistant {
	if service == "" {
		service = "notify"
	}
	return &HomeAssistant{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		AccessToken: accessToken,
		Service:     service,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (h *HomeAssistant) Name() string { return "homeassistant" }

func (h *HomeAssistant) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (h *HomeAssistant) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if h.BaseURL == "" || h.AccessToken == "" {
		return fmt.Errorf("homeassistant: base_url/access_token required")
	}
	url := fmt.Sprintf("%s/api/services/notify/%s", h.BaseURL, h.Service)
	payload := map[string]any{"message": out.Text}
	if out.ChatID != "" {
		payload["title"] = out.ChatID
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.AccessToken)
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("homeassistant: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("homeassistant: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
