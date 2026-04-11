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

	"github.com/nousresearch/hermes-agent/gateway"
)

// DiscordBot is an outbound-only Discord bot adapter using the REST
// API. It authenticates with the "Bot <token>" header — distinct
// from the incoming-webhook URLs used by the NewDiscord adapter.
type DiscordBot struct {
	BaseURL   string // default https://discord.com/api/v10
	Token     string
	ChannelID string // default channel; OutgoingMessage.ChatID wins when set
	client    *http.Client
}

func NewDiscordBot(token, channelID string) *DiscordBot {
	return &DiscordBot{
		BaseURL:   "https://discord.com/api/v10",
		Token:     token,
		ChannelID: channelID,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DiscordBot) WithBaseURL(u string) *DiscordBot {
	d.BaseURL = strings.TrimRight(u, "/")
	return d
}

func (d *DiscordBot) Name() string { return "discord_bot" }

func (d *DiscordBot) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (d *DiscordBot) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if d.Token == "" {
		return fmt.Errorf("discord_bot: token required")
	}
	channel := d.ChannelID
	if out.ChatID != "" {
		channel = out.ChatID
	}
	if channel == "" {
		return fmt.Errorf("discord_bot: channel id required")
	}
	url := fmt.Sprintf("%s/channels/%s/messages", d.BaseURL, channel)
	buf, _ := json.Marshal(map[string]any{"content": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+d.Token)
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord_bot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord_bot: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
