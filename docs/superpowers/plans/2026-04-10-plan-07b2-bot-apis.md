# Plan 7b.2: Bidirectional Bot APIs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Close out the Gateway platform catalog with three bot-token-based adapters:

1. **Slack Events API** — bidirectional. Inbound via `POST /slack/events` (HTTP webhook Slack pushes to), outbound via `chat.postMessage` REST call using a bot token.
2. **Discord Bot (REST)** — outbound-only via `POST /channels/{id}/messages` with the bot token in a `Bot <token>` Authorization header. Discord Gateway (WebSocket) for inbound is out of scope — operators can use `channel.id + api_server` to simulate.
3. **Mattermost Bot (REST)** — outbound-only via `POST /api/v4/posts` with a bot access token in a Bearer header. Matches the same shape as Mattermost's webhook but uses a proper bot account.

These adapters use *different* tokens than the existing webhook variants (Slack/Discord/Mattermost in Plan 7b), so they are new files with the suffix `_bot.go`.

**Non-goals:** Discord Gateway WebSocket, Mattermost WebSocket streaming, Slack Socket Mode. These need WebSocket plumbing and are deferred.

---

## Task 1: Slack Events API (bidirectional)

- [ ] Create `gateway/platforms/slack_events.go` with a `SlackEvents` struct that:
  - holds `Addr` (HTTP listen address, default `:8082`), `SigningSecret` (optional), `BotToken`, `APIBase` (default `https://slack.com`)
  - `Run(ctx, handler)` starts an HTTP server on `Addr` that accepts POST `/slack/events`
  - handles Slack's `url_verification` challenge: echo back `request.challenge`
  - for normal event envelopes (`event_callback`), extract `event.text`, build an `IncomingMessage{Platform:"slack_events", UserID: event.user, ChatID: event.channel, MessageID: event.client_msg_id, Text: event.text}`, call `handler`, and 200 the response
  - `SendReply(ctx, out)` POSTs to `{APIBase}/api/chat.postMessage` with `Authorization: Bearer BotToken` and `{channel, text}` body

- [ ] Create `gateway/platforms/slack_events_test.go`:
  - URL verification round-trip: POST `{"type":"url_verification","challenge":"xyz"}` returns `{"challenge":"xyz"}`
  - Event envelope: POST `{"type":"event_callback","event":{"type":"message","user":"U1","channel":"C1","text":"hi","client_msg_id":"m1"}}` invokes handler and replies via `chat.postMessage` (asserted via mock `APIBase`)
- [ ] Commit `feat(gateway/platforms): add Slack Events API bidirectional adapter`.

---

## Task 2: Discord Bot REST send

- [ ] Create `gateway/platforms/discord_bot.go`:

```go
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

// DiscordBot is an outbound-only Discord bot adapter using the REST API.
// It authenticates with the "Bot <token>" header (distinct from
// incoming-webhook URLs used by the NewDiscord adapter).
type DiscordBot struct {
	BaseURL   string // default https://discord.com/api/v10
	Token     string
	ChannelID string // default channel; overridden by OutgoingMessage.ChatID
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
```

- [ ] Create test with `httptest.Server` + `WithBaseURL`.
- [ ] Commit `feat(gateway/platforms): add Discord Bot REST send adapter`.

---

## Task 3: Mattermost Bot REST send

- [ ] Create `gateway/platforms/mattermost_bot.go` calling `POST /api/v4/posts` with `Authorization: Bearer <token>` and `{channel_id, message}` body.
- [ ] Test + commit.

---

## Task 4: CLI wiring

- [ ] Extend `cli/gateway.go` `buildPlatform` switch with `slack_events`, `discord_bot`, `mattermost_bot` cases.
- [ ] Commit `feat(cli): wire Slack Events / Discord Bot / Mattermost Bot into gateway builder`.

---

## Verification Checklist

- [ ] `go test ./gateway/platforms/...` passes
- [ ] Slack Events `url_verification` challenge round-trips
- [ ] Slack Events `event_callback` invokes the handler and calls `chat.postMessage`
- [ ] Discord Bot and Mattermost Bot send to their respective endpoints with correct auth
