package platforms

import (
	"context"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "acp",
		DisplayName: "ACP Server",
		Summary:     "Agent Client Protocol HTTP server.",
		Fields: []FieldSpec{
			{Name: "addr", Label: "Listen Address", Kind: FieldString, Required: true,
				Help: `e.g. ":9000".`},
			{Name: "token", Label: "Shared Token", Kind: FieldSecret, Required: true,
				Help: "Clients must present this as a bearer token."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewACP(opts["addr"], opts["token"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testListen(ctx, opts["addr"])
		},
	})
}
