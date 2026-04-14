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

// MattermostBot is an outbound-only Mattermost bot adapter. It calls
// POST {base}/api/v4/posts with a Bearer token — the bot access
// token created under System Console → Integrations → Bot Accounts.
type MattermostBot struct {
	BaseURL   string // e.g. https://mm.example.com
	Token     string
	ChannelID string // default channel; OutgoingMessage.ChatID wins when set
	client    *http.Client
}

func NewMattermostBot(baseURL, token, channelID string) *MattermostBot {
	return &MattermostBot{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		ChannelID: channelID,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MattermostBot) Name() string { return "mattermost_bot" }

func (m *MattermostBot) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (m *MattermostBot) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if m.BaseURL == "" || m.Token == "" {
		return fmt.Errorf("mattermost_bot: base_url/token required")
	}
	channel := m.ChannelID
	if out.ChatID != "" {
		channel = out.ChatID
	}
	if channel == "" {
		return fmt.Errorf("mattermost_bot: channel id required")
	}
	url := m.BaseURL + "/api/v4/posts"
	buf, _ := json.Marshal(map[string]any{"channel_id": channel, "message": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.Token)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("mattermost_bot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mattermost_bot: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
