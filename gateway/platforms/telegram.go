package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/proxy"

	"github.com/odysseythink/hermind/gateway"
)

// Telegram is a Telegram Bot API adapter using long-polling.
type Telegram struct {
	token   string
	baseURL string
	client  *http.Client
	offset  int
}

func NewTelegram(token string) *Telegram {
	return &Telegram{
		token:   token,
		baseURL: "https://api.telegram.org",
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// newTelegramTransport returns an http.RoundTripper routed through proxyURL.
// Empty proxyURL → http.DefaultTransport. Supported schemes: http, https, socks5.
// Shared between the main Telegram client and DoHTransport's fallback path.
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

// newTelegramClient wraps newTelegramTransport with the standard timeout.
func newTelegramClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	t, err := newTelegramTransport(proxyURL)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: t, Timeout: timeout}, nil
}

// WithBaseURL is used by tests to point at an httptest.Server.
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

func (t *Telegram) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if t.token == "" {
		return fmt.Errorf("telegram: empty token")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		updates, err := t.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			time.Sleep(2 * time.Second)
			continue
		}
		for _, u := range updates {
			t.offset = u.UpdateID + 1
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			in := gateway.IncomingMessage{
				Platform: t.Name(),
				UserID:   strconv.Itoa(u.Message.From.ID),
				ChatID:   strconv.Itoa(u.Message.Chat.ID),
				Text:     u.Message.Text,
			}
			gateway.DispatchAndReply(ctx, t, handler, in)
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
		return nil, fmt.Errorf("telegram: api returned ok=false")
	}
	return parsed.Result, nil
}

func (t *Telegram) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	payload := map[string]any{"chat_id": out.ChatID, "text": out.Text}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL("sendMessage"), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram: send status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
