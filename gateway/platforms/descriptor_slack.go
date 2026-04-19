package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "slack",
		DisplayName: "Slack (Incoming Webhook)",
		Summary:     "Outbound-only Slack messages via an incoming webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true,
				Help: "From Slack app → Incoming Webhooks."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSlack(opts["webhook_url"]), nil
		},
	})
}
