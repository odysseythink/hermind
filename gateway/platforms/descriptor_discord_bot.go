package platforms

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "discord_bot",
		DisplayName: "Discord Bot (REST poll)",
		Summary:     "Polls a single channel for new messages and replies in-thread.",
		Fields: []FieldSpec{
			{Name: "token", Label: "Bot Token", Kind: FieldSecret, Required: true},
			{Name: "channel_id", Label: "Channel ID", Kind: FieldString, Required: true,
				Help: "Numeric Discord channel snowflake."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewDiscordBot(opts["token"], opts["channel_id"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testDiscordBot(ctx, opts["token"], "https://discord.com/api/v10")
		},
	})
}

func testDiscordBot(ctx context.Context, token, baseURL string) error {
	if token == "" {
		return fmt.Errorf("discord_bot: token is empty")
	}
	return httpProbe(ctx, "GET", baseURL+"/users/@me", map[string]string{
		"Authorization": "Bot " + token,
	})
}
