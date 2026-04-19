package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

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
		Test: func(ctx context.Context, opts map[string]string) error {
			return testSignal(ctx, opts["base_url"])
		},
	})
}

func testSignal(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("signal: base_url is required")
	}
	base := strings.TrimRight(baseURL, "/")
	return httpProbe(ctx, "GET", base+"/v1/about", nil)
}
