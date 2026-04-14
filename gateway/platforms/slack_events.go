package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

// SlackEvents is a bidirectional Slack adapter.
//
// Inbound: runs an HTTP server on Addr that accepts POST /slack/events.
// Slack sends both url_verification (one-time challenge) and
// event_callback (real events). The adapter echoes the challenge and
// forwards message events to the gateway handler.
//
// Outbound: calls {APIBase}/api/chat.postMessage with a bot token.
type SlackEvents struct {
	Addr     string // e.g. ":8082"
	APIBase  string // default https://slack.com
	BotToken string
	client   *http.Client
}

func NewSlackEvents(addr, botToken string) *SlackEvents {
	if addr == "" {
		addr = ":8082"
	}
	return &SlackEvents{
		Addr:     addr,
		APIBase:  "https://slack.com",
		BotToken: botToken,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// WithAPIBase overrides the Slack API base URL for tests.
func (s *SlackEvents) WithAPIBase(u string) *SlackEvents {
	s.APIBase = strings.TrimRight(u, "/")
	return s
}

func (s *SlackEvents) Name() string { return "slack_events" }

type slackEventEnvelope struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge,omitempty"`
	Event     *struct {
		Type        string `json:"type"`
		User        string `json:"user"`
		Channel     string `json:"channel"`
		Text        string `json:"text"`
		ClientMsgID string `json:"client_msg_id"`
	} `json:"event,omitempty"`
}

func (s *SlackEvents) Run(ctx context.Context, handler gateway.MessageHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var env slackEventEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		switch env.Type {
		case "url_verification":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"challenge": env.Challenge})
			return
		case "event_callback":
			if env.Event == nil || env.Event.Type != "message" || env.Event.Text == "" {
				w.WriteHeader(200)
				return
			}
			in := gateway.IncomingMessage{
				Platform:  s.Name(),
				UserID:    env.Event.User,
				ChatID:    env.Event.Channel,
				Text:      env.Event.Text,
				MessageID: env.Event.ClientMsgID,
			}
			// Slack requires a fast 200, so dispatch to handler and
			// let gateway.DispatchAndReply fire the reply back via
			// chat.postMessage. Use the Run ctx (not r.Context) so
			// the goroutine outlives the HTTP response lifetime.
			go gateway.DispatchAndReply(ctx, s, handler, in)
			w.WriteHeader(200)
			return
		default:
			w.WriteHeader(200)
		}
	})

	srv := &http.Server{Addr: s.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("slack_events: %w", err)
	}
}

func (s *SlackEvents) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if s.BotToken == "" || out.ChatID == "" {
		return fmt.Errorf("slack_events: bot_token and chat_id required")
	}
	url := s.APIBase + "/api/chat.postMessage"
	buf, _ := json.Marshal(map[string]any{"channel": out.ChatID, "text": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.BotToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack_events: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack_events: status %d: %s", resp.StatusCode, string(body))
	}
	// Slack returns 200 with {"ok":false,"error":"..."} on logical errors.
	var apiResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &apiResp); err == nil && !apiResp.OK && apiResp.Error != "" {
		return fmt.Errorf("slack_events: api error: %s", apiResp.Error)
	}
	return nil
}
