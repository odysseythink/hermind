package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "wecom",
		DisplayName: "WeCom (Enterprise WeChat Bot)",
		Summary:     "Outbound-only WeCom bot via a group bot webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true,
				Help: "Include ?key=... from the WeCom group bot settings page."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewWeCom(opts["webhook_url"]), nil
		},
	})
}
