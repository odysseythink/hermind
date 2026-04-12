package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// DiscordGateway is a bidirectional Discord adapter using the Gateway
// WebSocket for inbound messages and the REST API for outbound replies.
type DiscordGateway struct {
	token      string
	channelID  string // default channel for SendReply
	gatewayURL string // wss://gateway.discord.gg/?v=10&encoding=json
	baseURL    string // REST API base, default https://discord.com/api/v10
	client     *http.Client
	ws         *WSConn // set during Run

	// Resume state.
	mu        sync.Mutex
	sessionID string
	seq       int
}

func NewDiscordGateway(token, channelID string) *DiscordGateway {
	return &DiscordGateway{
		token:      token,
		channelID:  channelID,
		gatewayURL: "wss://gateway.discord.gg/?v=10&encoding=json",
		baseURL:    "https://discord.com/api/v10",
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DiscordGateway) WithGatewayURL(u string) *DiscordGateway {
	d.gatewayURL = u
	return d
}

func (d *DiscordGateway) WithBaseURL(u string) *DiscordGateway {
	d.baseURL = strings.TrimRight(u, "/")
	return d
}

func (d *DiscordGateway) Name() string { return "discord" }

// discordPayload is the generic Gateway payload envelope.
type discordPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *int            `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type discordHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type discordMessageCreate struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

type discordReady struct {
	SessionID string `json:"session_id"`
}

func (d *DiscordGateway) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if d.token == "" {
		return fmt.Errorf("discord: empty token")
	}

	d.ws = NewWSConn(WSConnConfig{
		URL: d.gatewayURL,
		OnMessage: func(data []byte) {
			d.handlePayload(ctx, data, handler)
		},
		ReconnectBase:   2 * time.Second,
		ReconnectMax:    60 * time.Second,
		ReconnectJitter: 0.2,
	})

	return d.ws.Run(ctx)
}

func (d *DiscordGateway) handlePayload(ctx context.Context, data []byte, handler gateway.MessageHandler) {
	var p discordPayload
	if err := json.Unmarshal(data, &p); err != nil {
		slog.Warn("discord: bad payload", "err", err)
		return
	}

	if p.S != nil {
		d.mu.Lock()
		d.seq = *p.S
		d.mu.Unlock()
	}

	switch p.Op {
	case 10: // Hello
		var hello discordHello
		_ = json.Unmarshal(p.D, &hello)
		d.handleHello(ctx, hello)
	case 0: // Dispatch
		d.handleDispatch(ctx, p.T, p.D, handler)
	case 11: // Heartbeat ACK — no action
	case 1: // Server requests heartbeat
		d.sendHeartbeat(ctx)
	case 7: // Reconnect
		slog.Info("discord: server requested reconnect")
	case 9: // Invalid session
		d.mu.Lock()
		d.sessionID = ""
		d.seq = 0
		d.mu.Unlock()
	}
}

func (d *DiscordGateway) handleHello(ctx context.Context, hello discordHello) {
	interval := time.Duration(hello.HeartbeatInterval) * time.Millisecond
	go d.heartbeatLoop(ctx, interval)

	d.mu.Lock()
	sid := d.sessionID
	seq := d.seq
	d.mu.Unlock()

	if sid != "" {
		d.sendResume(ctx, sid, seq)
	} else {
		d.sendIdentify(ctx)
	}
}

func (d *DiscordGateway) sendIdentify(ctx context.Context) {
	identify := map[string]any{
		"op": 2,
		"d": map[string]any{
			"token":   d.token,
			"intents": 512 | 32768, // GUILD_MESSAGES | MESSAGE_CONTENT
			"properties": map[string]string{
				"os":      "linux",
				"browser": "hermes-agent",
				"device":  "hermes-agent",
			},
		},
	}
	d.writeJSON(ctx, identify)
}

func (d *DiscordGateway) sendResume(ctx context.Context, sessionID string, seq int) {
	resume := map[string]any{
		"op": 6,
		"d": map[string]any{
			"token":      d.token,
			"session_id": sessionID,
			"seq":        seq,
		},
	}
	d.writeJSON(ctx, resume)
}

func (d *DiscordGateway) sendHeartbeat(ctx context.Context) {
	d.mu.Lock()
	seq := d.seq
	d.mu.Unlock()
	d.writeJSON(ctx, map[string]any{"op": 1, "d": seq})
}

func (d *DiscordGateway) heartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sendHeartbeat(ctx)
		}
	}
}

func (d *DiscordGateway) writeJSON(ctx context.Context, v any) {
	if d.ws == nil {
		return
	}
	if err := d.ws.WriteJSON(ctx, v); err != nil {
		slog.Warn("discord: write error", "err", err)
	}
}

func (d *DiscordGateway) handleDispatch(ctx context.Context, eventType string, data json.RawMessage, handler gateway.MessageHandler) {
	switch eventType {
	case "READY":
		var ready discordReady
		_ = json.Unmarshal(data, &ready)
		d.mu.Lock()
		d.sessionID = ready.SessionID
		d.mu.Unlock()
		slog.Info("discord: ready", "session_id", ready.SessionID)
	case "MESSAGE_CREATE":
		var msg discordMessageCreate
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("discord: bad MESSAGE_CREATE", "err", err)
			return
		}
		if msg.Author.Bot {
			return
		}
		if msg.Content == "" {
			return
		}
		in := gateway.IncomingMessage{
			Platform:  d.Name(),
			UserID:    msg.Author.ID,
			ChatID:    msg.ChannelID,
			Text:      msg.Content,
			MessageID: msg.ID,
		}
		gateway.DispatchAndReply(ctx, d, handler, in)
	}
}

func (d *DiscordGateway) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if d.token == "" {
		return fmt.Errorf("discord: token required")
	}
	channel := d.channelID
	if out.ChatID != "" {
		channel = out.ChatID
	}
	if channel == "" {
		return fmt.Errorf("discord: channel id required")
	}
	url := fmt.Sprintf("%s/channels/%s/messages", d.baseURL, channel)
	buf, _ := json.Marshal(map[string]any{"content": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+d.token)
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
