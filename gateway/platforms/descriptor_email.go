package platforms

import (
	"context"
	"fmt"
	"net"
	"net/smtp"

	"github.com/odysseythink/hermind/gateway"
)

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
		Test: func(ctx context.Context, opts map[string]string) error {
			port := opts["port"]
			if port == "" {
				port = "587"
			}
			return testEmail(ctx, opts["host"], port, opts["username"], opts["password"])
		},
	})
}

// testEmail dials the SMTP submission port, runs EHLO, attempts
// AUTH LOGIN when credentials are supplied, and hangs up. No message
// is sent. Respects ctx for the dial deadline.
func testEmail(ctx context.Context, host, port, user, pass string) error {
	if host == "" {
		return fmt.Errorf("email: host is required")
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Quit()

	if err := client.Hello("hermind.test"); err != nil {
		return fmt.Errorf("ehlo: %w", err)
	}
	if user == "" && pass == "" {
		return nil
	}
	auth := smtp.PlainAuth("", user, pass, host)
	if err := client.Auth(auth); err != nil {
		// Fall back to AUTH LOGIN for servers that don't advertise PLAIN.
		if err2 := client.Auth(loginAuth(user, pass)); err2 != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	return nil
}

// loginAuth returns an smtp.Auth implementation that performs the
// AUTH LOGIN exchange (not RFC 4954 SASL PLAIN). Some servers only
// advertise LOGIN, so we use this as a fallback.
func loginAuth(user, pass string) smtp.Auth { return &authLogin{user: user, pass: pass} }

type authLogin struct{ user, pass string }

func (a *authLogin) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}
func (a *authLogin) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.user), nil
	case "Password:":
		return []byte(a.pass), nil
	}
	return nil, nil
}
