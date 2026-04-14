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
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// MattermostWS is a bidirectional Mattermost adapter using WebSocket
// for inbound events and the REST API for outbound replies.
type MattermostWS struct {
	baseURL string // REST API base, e.g. https://mm.example.com
	token   string
	channel string // default channel
	wsURL   string // computed from baseURL if not set
	client  *http.Client
	ws      *WSConn
}

func NewMattermostWS(baseURL, token, channelID string) *MattermostWS {
	base := strings.TrimRight(baseURL, "/")
	wsBase := strings.Replace(base, "https://", "wss://", 1)
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)
	return &MattermostWS{
		baseURL: base,
		token:   token,
		channel: channelID,
		wsURL:   wsBase + "/api/v4/websocket",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MattermostWS) WithWebSocketURL(u string) *MattermostWS {
	m.wsURL = u
	return m
}

func (m *MattermostWS) Name() string { return "mattermost" }

type mmEvent struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data,omitempty"`
}

type mmPost struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	UserID    string `json:"user_id"`
}

func (m *MattermostWS) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if m.token == "" {
		return fmt.Errorf("mattermost: empty token")
	}

	m.ws = NewWSConn(WSConnConfig{
		URL: m.wsURL,
		OnMessage: func(data []byte) {
			m.handleEvent(ctx, data, handler)
		},
		ReconnectBase:   2 * time.Second,
		ReconnectMax:    60 * time.Second,
		ReconnectJitter: 0.2,
	})

	return m.ws.Run(ctx)
}

func (m *MattermostWS) handleEvent(ctx context.Context, data []byte, handler gateway.MessageHandler) {
	var ev mmEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Warn("mattermost: bad event", "err", err)
		return
	}

	switch ev.Event {
	case "authentication_challenge":
		m.sendAuth(ctx)
	case "posted":
		m.handlePosted(ctx, ev.Data, handler)
	}
}

func (m *MattermostWS) sendAuth(ctx context.Context) {
	auth := map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data":   map[string]string{"token": m.token},
	}
	if m.ws != nil {
		if err := m.ws.WriteJSON(ctx, auth); err != nil {
			slog.Warn("mattermost: auth write error", "err", err)
		}
	}
}

func (m *MattermostWS) handlePosted(ctx context.Context, data map[string]any, handler gateway.MessageHandler) {
	postStr, ok := data["post"].(string)
	if !ok {
		return
	}
	var post mmPost
	if err := json.Unmarshal([]byte(postStr), &post); err != nil {
		slog.Warn("mattermost: bad post JSON", "err", err)
		return
	}
	if post.Message == "" {
		return
	}
	in := gateway.IncomingMessage{
		Platform:  m.Name(),
		UserID:    post.UserID,
		ChatID:    post.ChannelID,
		Text:      post.Message,
		MessageID: post.ID,
	}
	gateway.DispatchAndReply(ctx, m, handler, in)
}

func (m *MattermostWS) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if m.baseURL == "" || m.token == "" {
		return fmt.Errorf("mattermost: base_url/token required")
	}
	channel := m.channel
	if out.ChatID != "" {
		channel = out.ChatID
	}
	if channel == "" {
		return fmt.Errorf("mattermost: channel id required")
	}
	url := m.baseURL + "/api/v4/posts"
	buf, _ := json.Marshal(map[string]any{"channel_id": channel, "message": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.token)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("mattermost: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mattermost: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
