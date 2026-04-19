package platforms

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "sms",
		DisplayName: "SMS (Twilio)",
		Summary:     "Twilio REST API — outbound SMS only.",
		Fields: []FieldSpec{
			{Name: "account_sid", Label: "Account SID", Kind: FieldSecret, Required: true,
				Help: `Twilio AC... account identifier.`},
			{Name: "auth_token", Label: "Auth Token", Kind: FieldSecret, Required: true},
			{Name: "from", Label: "From Number", Kind: FieldString, Required: true,
				Help: "E.164 phone number registered with Twilio."},
			{Name: "to", Label: "To Number", Kind: FieldString, Required: true,
				Help: "Destination phone number in E.164."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSMS(opts["account_sid"], opts["auth_token"], opts["from"], opts["to"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testSMS(ctx, opts["account_sid"], opts["auth_token"], "https://api.twilio.com")
		},
	})
}

func testSMS(ctx context.Context, sid, token, baseURL string) error {
	if sid == "" || token == "" {
		return fmt.Errorf("sms: account_sid and auth_token are required")
	}
	cred := base64.StdEncoding.EncodeToString([]byte(sid + ":" + token))
	return httpProbe(ctx, "GET", baseURL+"/2010-04-01/Accounts/"+sid+".json", map[string]string{
		"Authorization": "Basic " + cred,
	})
}
