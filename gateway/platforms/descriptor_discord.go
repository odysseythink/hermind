package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "discord",
		DisplayName: "Discord (Incoming Webhook)",
		Summary:     "Outbound-only Discord messages via a channel webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true,
				Help: "From Discord channel settings → Integrations → Webhooks."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewDiscord(opts["webhook_url"]), nil
		},
	})
}
