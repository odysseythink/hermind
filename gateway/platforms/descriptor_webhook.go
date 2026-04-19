package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "webhook",
		DisplayName: "Generic Webhook",
		Summary:     "POSTs outgoing messages to an arbitrary URL; optional bearer token.",
		Fields: []FieldSpec{
			{Name: "url", Label: "URL", Kind: FieldString, Required: true,
				Help: "HTTPS endpoint to POST each outgoing message to."},
			{Name: "token", Label: "Bearer Token", Kind: FieldSecret,
				Help: "If set, sent as Authorization: Bearer <token>."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewWebhook(opts["url"], opts["token"]), nil
		},
	})
}
