package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "api_server",
		DisplayName: "Generic API Server",
		Summary:     "Accepts inbound messages via HTTP POST; emits outbound via callback.",
		Fields: []FieldSpec{
			{Name: "addr", Label: "Listen Address", Kind: FieldString,
				Default: ":8080",
				Help:    `e.g. ":8080".`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			addr := opts["addr"]
			if addr == "" {
				addr = ":8080"
			}
			return NewAPIServer(addr), nil
		},
	})
}
