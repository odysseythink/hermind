package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "feishu",
		DisplayName: "Feishu / Lark (Self-built App)",
		Summary:     "Bidirectional Feishu/Lark adapter via a self-built app long-connection.",
		Fields: []FieldSpec{
			{Name: "app_id", Label: "App ID", Kind: FieldString, Required: true,
				Help: "Self-built app ID from the Feishu Open Platform console."},
			{Name: "app_secret", Label: "App Secret", Kind: FieldSecret, Required: true,
				Help: "App secret paired with App ID."},
			{Name: "domain", Label: "Domain", Kind: FieldEnum, Required: true,
				Enum: []string{"feishu", "lark"},
				Help: "feishu = feishu.cn (CN). lark = larksuite.com (overseas)."},
			{Name: "encrypt_key", Label: "Encrypt Key", Kind: FieldSecret,
				Help: "Only needed when Encrypted Push is enabled in the app console."},
			{Name: "default_chat_id", Label: "Default Chat ID", Kind: FieldString,
				Help: "Fallback chat_id for pushes with no inbound context (e.g. oc_xxxx)."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewFeishuApp(opts)
		},
	})
}
