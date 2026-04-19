package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "signal",
		DisplayName: "Signal (signal-cli REST)",
		Summary:     "Requires a running signal-cli REST API.",
		Fields: []FieldSpec{
			{Name: "base_url", Label: "signal-cli Base URL", Kind: FieldString, Required: true,
				Help: `e.g. "http://localhost:8080".`},
			{Name: "account", Label: "Account", Kind: FieldString, Required: true,
				Help: "Registered phone number in E.164 form."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSignal(opts["base_url"], opts["account"]), nil
		},
	})
}
