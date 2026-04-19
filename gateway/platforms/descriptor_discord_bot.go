package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
