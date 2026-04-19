package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "mattermost",
		DisplayName: "Mattermost (Incoming Webhook)",
		Summary:     "Outbound-only Mattermost messages via an incoming webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewMattermost(opts["webhook_url"]), nil
		},
	})
}
