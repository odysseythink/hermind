package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

func init() {
	RegisterBuilder("telegram", func(name string, opts map[string]string) (Platform, error) {
		token := opts["bot_token"]
		proxyURL := opts["proxy_url"]
		slog.Info("gateway: building telegram adapter",
			"name", name,
			"has_token", token != "",
			"has_proxy", proxyURL != "",
		)
		return NewTelegram(name, token, proxyURL)
	})
}

// Telegram is a Telegram Bot API adapter using long-polling.
type Telegram struct {
	name     string
	token    string
	baseURL  string
	client   *http.Client
	proxyURL string
	offset   int
}

// NewTelegram creates a Telegram adapter.
func NewTelegram(name, token, proxyURL string) (*Telegram, error) {
	client, err := newTelegramClient(proxyURL, 60*time.Second)
	if err != nil {
		return nil, err
	}
	return &Telegram{
		name:     name,
		token:    token,
		baseURL:  "https://api.telegram.org",
		client:   client,
		proxyURL: proxyURL,
	}, nil
}

func newTelegramTransport(proxyURL string) (http.RoundTripper, error) {
	if proxyURL == "" {
		return http.DefaultTransport, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("telegram: invalid proxy %q: %w", proxyURL, err)
	}
	switch u.Scheme {
	case "http", "https":
		return &http.Transport{Proxy: http.ProxyURL(u)}, nil
	case "socks5":
		dialer, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("telegram: socks5 dial: %w", err)
		}
		cd, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("telegram: socks5 dialer does not support context")
		}
		return &http.Transport{DialContext: cd.DialContext}, nil
	default:
		return nil, fmt.Errorf("telegram: unsupported proxy scheme %q (want http/https/socks5)", u.Scheme)
	}
}

func newTelegramClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	t, err := newTelegramTransport(proxyURL)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: t, Timeout: timeout}, nil
}

func (t *Telegram) WithBaseURL(u string) *Telegram { t.baseURL = u; return t }

func (t *Telegram) Name() string { return "telegram" }

type tgUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
}

type tgUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

func (t *Telegram) apiURL(method string) string {
	return t.baseURL + "/bot" + t.token + "/" + method
}

func (t *Telegram) Run(ctx context.Context, handler MessageHandler) error {
	if t.token == "" {
		return fmt.Errorf("telegram[%s]: empty bot_token", t.name)
	}
	slog.InfoContext(ctx, "telegram: starting long-poll loop", "name", t.name)
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		updates, err := t.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.WarnContext(ctx, "telegram: getUpdates error, retrying in 2s",
				"name", t.name, "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}
		slog.DebugContext(ctx, "telegram: polled updates",
			"name", t.name, "count", len(updates))
		for _, u := range updates {
			t.offset = u.UpdateID + 1
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			in := IncomingMessage{
				Platform:  t.Name(),
				UserID:    strconv.Itoa(u.Message.From.ID),
				ChatID:    strconv.Itoa(u.Message.Chat.ID),
				Text:      u.Message.Text,
				MessageID: strconv.Itoa(u.Message.MessageID),
			}
			slog.InfoContext(ctx, "telegram: dispatching message",
				"name", t.name,
				"from", u.Message.From.Username,
				"chat_id", in.ChatID,
				"text_preview", truncate(in.Text, 60),
			)
			DispatchAndReply(ctx, t, handler, in)
		}
	}
}

func (t *Telegram) getUpdates(ctx context.Context) ([]tgUpdate, error) {
	q := url.Values{}
	q.Set("timeout", "25")
	q.Set("offset", strconv.Itoa(t.offset))
	req, err := http.NewRequestWithContext(ctx, "GET", t.apiURL("getUpdates")+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("telegram: status %d: %s", resp.StatusCode, string(body))
	}
	var parsed tgUpdatesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.OK {
		return nil, fmt.Errorf("telegram: api returned ok=false: %s", string(body))
	}
	return parsed.Result, nil
}

func (t *Telegram) SendReply(ctx context.Context, out OutgoingMessage) error {
	payload := map[string]any{"chat_id": out.ChatID, "text": out.Text}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL("sendMessage"), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram[%s]: send: %w", t.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram[%s]: send status %d: %s", t.name, resp.StatusCode, string(body))
	}
	slog.InfoContext(ctx, "telegram: reply sent", "name", t.name, "chat_id", out.ChatID)
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
