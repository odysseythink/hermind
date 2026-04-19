package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
