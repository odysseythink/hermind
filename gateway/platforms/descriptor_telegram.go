package platforms

import (
	"context"
	"fmt"

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
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewTelegram(opts["token"], opts["proxy"])
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testTelegram(ctx, opts["token"], "https://api.telegram.org")
		},
	})
}

func testTelegram(ctx context.Context, token, baseURL string) error {
	if token == "" {
		return fmt.Errorf("telegram: token is empty")
	}
	return httpProbe(ctx, "GET", baseURL+"/bot"+token+"/getMe", nil)
}
