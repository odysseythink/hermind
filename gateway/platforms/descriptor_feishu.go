package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "feishu",
		DisplayName: "Feishu / Lark (Bot Webhook)",
		Summary:     "Outbound-only Feishu/Lark bot via a custom bot webhook.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewFeishu(opts["webhook_url"]), nil
		},
	})
}
