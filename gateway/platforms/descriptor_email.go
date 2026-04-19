package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "email",
		DisplayName: "Email (SMTP)",
		Summary:     "Sends outbound messages via an SMTP server.",
		Fields: []FieldSpec{
			{Name: "host", Label: "SMTP Host", Kind: FieldString, Required: true,
				Help: `e.g. "smtp.example.com".`},
			{Name: "port", Label: "SMTP Port", Kind: FieldString,
				Default: "587",
				Help:    `Submission port; typically 587 for STARTTLS, 465 for implicit TLS.`},
			{Name: "username", Label: "Username", Kind: FieldString},
			{Name: "password", Label: "Password", Kind: FieldSecret},
			{Name: "from", Label: "From", Kind: FieldString, Required: true,
				Help: "Sender address; must be allowed by the SMTP server."},
			{Name: "to", Label: "To", Kind: FieldString, Required: true,
				Help: "Comma-separated recipient addresses."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			port := opts["port"]
			if port == "" {
				port = "587"
			}
			return NewEmail(
				opts["host"], port,
				opts["username"], opts["password"],
				opts["from"], opts["to"],
			), nil
		},
	})
}
