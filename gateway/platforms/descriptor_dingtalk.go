package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "dingtalk",
		DisplayName: "DingTalk (Robot Webhook)",
		Summary:     "Outbound-only DingTalk robot via a custom robot webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true,
				Help: "Include ?access_token=... from the DingTalk robot settings page."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewDingTalk(opts["webhook_url"]), nil
		},
	})
}
