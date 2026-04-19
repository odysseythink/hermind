package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "whatsapp",
		DisplayName: "WhatsApp Cloud API",
		Summary:     "Meta's WhatsApp Cloud API, outbound only.",
		Fields: []FieldSpec{
			{Name: "phone_id", Label: "Phone Number ID", Kind: FieldString, Required: true},
			{Name: "access_token", Label: "Access Token", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewWhatsApp(opts["phone_id"], opts["access_token"]), nil
		},
	})
}
