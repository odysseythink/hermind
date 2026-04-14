package platforms

import (
	"context"
	"net/smtp"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/gateway"
)

func TestEmailSendReplyWithInjectedSendMail(t *testing.T) {
	var (
		capturedAddr string
		capturedFrom string
		capturedTo   []string
		capturedBody string
	)
	e := NewEmail("smtp.example.com", "587", "u", "p", "bot@x", "me@x")
	e.sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		capturedAddr = addr
		capturedFrom = from
		capturedTo = to
		capturedBody = string(msg)
		return nil
	}
	err := e.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi", ChatID: "t1"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if capturedAddr != "smtp.example.com:587" {
		t.Errorf("addr = %q", capturedAddr)
	}
	if capturedFrom != "bot@x" || len(capturedTo) != 1 || capturedTo[0] != "me@x" {
		t.Errorf("from/to = %q / %v", capturedFrom, capturedTo)
	}
	if !strings.Contains(capturedBody, "Subject: hermind: t1") {
		t.Errorf("missing subject: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "\r\n\r\nhi") {
		t.Errorf("missing body: %s", capturedBody)
	}
}

func TestEmailSendReplyMissingConfig(t *testing.T) {
	e := NewEmail("", "", "", "", "", "")
	err := e.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}
