package platforms

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

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
		Test: func(ctx context.Context, opts map[string]string) error {
			return testWhatsApp(ctx, opts["phone_id"], opts["access_token"], "https://graph.facebook.com")
		},
	})
}

func testWhatsApp(ctx context.Context, phoneID, accessToken, baseURL string) error {
	if phoneID == "" || accessToken == "" {
		return fmt.Errorf("whatsapp: phone_id and access_token are required")
	}
	return httpProbe(ctx, "GET", baseURL+"/v20.0/"+phoneID, map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
}
