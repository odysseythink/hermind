package platforms

import "github.com/odysseythink/hermind/gateway"

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
			return NewTelegram(opts["token"]), nil
		},
	})
}
