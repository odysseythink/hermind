package platforms

import (
	"context"
	"fmt"
	"net"

	"github.com/odysseythink/hermind/gateway"
)

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
		Test: func(ctx context.Context, opts map[string]string) error {
			addr := opts["addr"]
			if addr == "" {
				addr = ":8080"
			}
			return testListen(ctx, addr)
		},
	})
}

// testListen opens a TCP listener on addr and closes it immediately.
// A successful bind proves the address is syntactically valid and
// not already in use.
func testListen(ctx context.Context, addr string) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return ln.Close()
}
