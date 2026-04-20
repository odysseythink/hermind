package platforms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "telegram",
		DisplayName: "Telegram Bot",
		Summary:     "Telegram Bot API — long-polling adapter.",
		Fields: []FieldSpec{
			{Name: "token", Label: "Bot Token", Kind: FieldSecret, Required: true,
				Help: "From @BotFather."},
			{Name: "proxy", Label: "Proxy URL", Kind: FieldString, Required: false,
				Help: "Optional. http://, https://, or socks5://. Leave blank for direct connection. Required in mainland China."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewTelegram(opts["token"], opts["proxy"])
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testTelegram(ctx, opts["token"], opts["proxy"], "https://api.telegram.org")
		},
	})
}

func testTelegram(ctx context.Context, token, proxyURL, baseURL string) error {
	if token == "" {
		return fmt.Errorf("telegram: token is empty")
	}
	client, err := newTelegramClient(proxyURL, 10*time.Second)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/bot"+token+"/getMe", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: probe failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram: probe returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
