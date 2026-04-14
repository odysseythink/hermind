package platforms

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

// Email sends replies via an SMTP server with PLAIN auth. Inbound is
// not supported in Plan 7b — pair with api_server.
type Email struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	To       string
	// sendMail is indirected so tests can substitute a fake.
	sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewEmail constructs an Email adapter. Host, from, and to are
// strictly required at SendReply time; username/password are used
// only if provided.
func NewEmail(host, port, username, password, from, to string) *Email {
	if port == "" {
		port = "587"
	}
	return &Email{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		To:       to,
		sendMail: smtp.SendMail,
	}
}

func (e *Email) Name() string { return "email" }

func (e *Email) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (e *Email) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if e.Host == "" || e.From == "" || e.To == "" {
		return fmt.Errorf("email: host/from/to are required")
	}
	subject := "hermes reply"
	if out.ChatID != "" {
		subject = "hermind: " + out.ChatID
	}
	msg := []byte(strings.Join([]string{
		"From: " + e.From,
		"To: " + e.To,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		out.Text,
	}, "\r\n"))

	var auth smtp.Auth
	if e.Username != "" {
		auth = smtp.PlainAuth("", e.Username, e.Password, e.Host)
	}
	addr := net.JoinHostPort(e.Host, e.Port)
	return e.sendMail(addr, auth, e.From, []string{e.To}, msg)
}
