package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "homeassistant",
		DisplayName: "Home Assistant (Notify)",
		Summary:     "Calls a Home Assistant notify service.",
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Base URL", Kind: FieldString, Required: true,
				Help: `e.g. "http://homeassistant.local:8123".`},
			{Name: "access_token", Label: "Long-Lived Access Token", Kind: FieldSecret, Required: true},
			{Name: "service", Label: "Notify Service", Kind: FieldString,
				Default: "notify",
				Help:    `Service under notify.*; e.g. "mobile_app_my_phone".`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			svc := opts["service"]
			if svc == "" {
				svc = "notify"
			}
			return NewHomeAssistant(opts["base_url"], opts["access_token"], svc), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testHomeAssistant(ctx, opts["base_url"], opts["access_token"])
		},
	})
}

func testHomeAssistant(ctx context.Context, baseURL, accessToken string) error {
	if baseURL == "" || accessToken == "" {
		return fmt.Errorf("homeassistant: base_url and access_token are required")
	}
	base := strings.TrimRight(baseURL, "/")
	return httpProbe(ctx, "GET", base+"/api/", map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
}
