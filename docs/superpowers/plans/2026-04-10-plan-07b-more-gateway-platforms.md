# Plan 7b: Remaining Gateway Platforms Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Extend the Gateway with eight more platform adapters: Discord, Slack, Mattermost, Feishu, DingTalk, WeCom, Email (SMTP), and SMS (Twilio). Six of these share the "incoming webhook bot" pattern (POST JSON to a bot URL, reply-only, no inbound channel), so we factor their behavior into a shared helper. Email and SMS each use their own transport (`net/smtp` and Twilio REST).

**Architecture:**
- A shared `gateway/platforms/webhookbot.go` helper provides a generic "send to webhook URL with a platform-specific payload builder". Each bot platform is then a tiny wrapper that defines (1) a name, (2) an env/config key for the URL, and (3) a `payload(OutgoingMessage) any` function.
- Email uses `net/smtp` with config for host, port, username, password, from, to.
- SMS uses the Twilio REST API (`POST /2010-04-01/Accounts/{sid}/Messages.json`) with Basic Auth.
- All eight platforms are **outbound-only**: `Run` blocks on `ctx.Done()`, `SendReply` does the actual work. For inbound, the operator pairs one of these with api_server.

**Tech Stack:** Go 1.25 stdlib (`net/http`, `net/smtp`, `encoding/base64`). No new deps.

**Deliverable at end of plan:**
```yaml
gateway:
  platforms:
    slack:
      enabled: true
      type: slack
      options:
        webhook_url: https://hooks.slack.com/services/XXX/YYY/ZZZ
    email:
      enabled: true
      type: email
      options:
        host: smtp.gmail.com
        port: "587"
        username: bot@example.com
        password: app-password
        from: bot@example.com
        to: me@example.com
    sms:
      enabled: true
      type: sms
      options:
        account_sid: ACxxxx
        auth_token: yyyy
        from: "+15551234567"
        to: "+15557654321"
```

**Non-goals (deferred to Plan 7b.1):**
- Signal (signal-cli wrapper)
- WhatsApp Business API webhook
- Matrix client-server protocol
- Home Assistant REST/WebSocket
- telegram_network (MTProto user-account variant)
- Bidirectional bot API support (Discord Bot, Slack RTM, Mattermost Bot) — Plan 7b.2

---

## Task 1: webhookbot helper

- [ ] Create `gateway/platforms/webhookbot.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// WebhookBot is the shared outbound-only bot adapter. Platform
// wrappers (Slack, Discord, Feishu, …) supply a name and a function
// that converts an OutgoingMessage into a platform-specific JSON body.
type WebhookBot struct {
	platformName string
	url          string
	buildPayload func(gateway.OutgoingMessage) any
	client       *http.Client
}

// NewWebhookBot builds a WebhookBot given a platform name, webhook URL
// and a payload builder.
func NewWebhookBot(name, url string, buildPayload func(gateway.OutgoingMessage) any) *WebhookBot {
	return &WebhookBot{
		platformName: name,
		url:          url,
		buildPayload: buildPayload,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *WebhookBot) Name() string { return b.platformName }

// Run blocks until ctx is done — webhook bots have no inbound loop.
func (b *WebhookBot) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

func (b *WebhookBot) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if b.url == "" {
		return fmt.Errorf("%s: no webhook URL configured", b.platformName)
	}
	buf, err := json.Marshal(b.buildPayload(out))
	if err != nil {
		return fmt.Errorf("%s: encode payload: %w", b.platformName, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", b.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", b.platformName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: status %d: %s", b.platformName, resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] Commit `feat(gateway/platforms): add shared WebhookBot helper`.

---

## Task 2: six webhook bot wrappers

- [ ] Create `gateway/platforms/slack.go`:

```go
package platforms

import "github.com/nousresearch/hermes-agent/gateway"

func NewSlack(url string) *WebhookBot {
	return NewWebhookBot("slack", url, func(out gateway.OutgoingMessage) any {
		return map[string]string{"text": out.Text}
	})
}
```

- [ ] Create `gateway/platforms/discord.go`:

```go
package platforms

import "github.com/nousresearch/hermes-agent/gateway"

func NewDiscord(url string) *WebhookBot {
	return NewWebhookBot("discord", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{"content": out.Text}
	})
}
```

- [ ] Create `gateway/platforms/mattermost.go`:

```go
package platforms

import "github.com/nousresearch/hermes-agent/gateway"

func NewMattermost(url string) *WebhookBot {
	return NewWebhookBot("mattermost", url, func(out gateway.OutgoingMessage) any {
		return map[string]string{"text": out.Text}
	})
}
```

- [ ] Create `gateway/platforms/feishu.go`:

```go
package platforms

import "github.com/nousresearch/hermes-agent/gateway"

// NewFeishu builds a Feishu / Lark incoming webhook bot.
// Feishu expects: {"msg_type":"text","content":{"text":"..."}}.
func NewFeishu(url string) *WebhookBot {
	return NewWebhookBot("feishu", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": out.Text},
		}
	})
}
```

- [ ] Create `gateway/platforms/dingtalk.go`:

```go
package platforms

import "github.com/nousresearch/hermes-agent/gateway"

// NewDingTalk builds a DingTalk incoming webhook bot.
// Expected shape: {"msgtype":"text","text":{"content":"..."}}.
func NewDingTalk(url string) *WebhookBot {
	return NewWebhookBot("dingtalk", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": out.Text},
		}
	})
}
```

- [ ] Create `gateway/platforms/wecom.go`:

```go
package platforms

import "github.com/nousresearch/hermes-agent/gateway"

// NewWeCom builds a WeCom (enterprise WeChat) incoming webhook bot.
// Expected shape: {"msgtype":"text","text":{"content":"..."}}.
func NewWeCom(url string) *WebhookBot {
	return NewWebhookBot("wecom", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": out.Text},
		}
	})
}
```

- [ ] Create `gateway/platforms/webhookbot_test.go` exercising a shared `httptest.Server` with each of the six wrappers and asserting:
  - the correct request path is hit
  - the body contains the `out.Text`
  - 200 returns nil error
  - 500 returns a wrapped error
- [ ] Run `go test ./gateway/platforms/...` — PASS.
- [ ] Commit `feat(gateway/platforms): add Slack, Discord, Mattermost, Feishu, DingTalk, WeCom webhook bots`.

---

## Task 3: Email (SMTP) adapter

- [ ] Create `gateway/platforms/email.go`:

```go
package platforms

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/nousresearch/hermes-agent/gateway"
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

// NewEmail constructs an Email adapter. Only host, from, and to are
// strictly required; username/password are used if the server
// requires auth.
func NewEmail(host, port, username, password, from, to string) *Email {
	if port == "" {
		port = "587"
	}
	return &Email{
		Host: host, Port: port,
		Username: username, Password: password,
		From: from, To: to,
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
		subject = "hermes: " + out.ChatID
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
```

- [ ] Create `gateway/platforms/email_test.go` using an injected `sendMail`:

```go
func TestEmailSendReply(t *testing.T) {
    var captured struct {
        addr string
        from string
        to   []string
        body string
    }
    e := NewEmail("smtp.example.com", "587", "u", "p", "bot@x", "me@x")
    e.sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
        captured.addr = addr
        captured.from = from
        captured.to = to
        captured.body = string(msg)
        return nil
    }
    err := e.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi", ChatID: "topic"})
    // assertions...
}
```

- [ ] Run tests — PASS.
- [ ] Commit `feat(gateway/platforms): add Email (SMTP) adapter`.

---

## Task 4: SMS (Twilio) adapter

- [ ] Create `gateway/platforms/sms.go` talking to `https://api.twilio.com/2010-04-01/Accounts/{sid}/Messages.json` with Basic Auth (`sid:token`). Use `application/x-www-form-urlencoded` body with `From`, `To`, `Body`.
- [ ] Create `gateway/platforms/sms_test.go` using `httptest.Server` + `WithBaseURL` seam to redirect the client.
- [ ] Run tests — PASS.
- [ ] Commit `feat(gateway/platforms): add SMS (Twilio) adapter`.

---

## Task 5: CLI wiring

- [ ] Extend `cli/gateway.go` `buildPlatform` switch with eight new cases:

```go
case "slack":
    return platforms.NewSlack(pc.Options["webhook_url"]), nil
case "discord":
    return platforms.NewDiscord(pc.Options["webhook_url"]), nil
case "mattermost":
    return platforms.NewMattermost(pc.Options["webhook_url"]), nil
case "feishu":
    return platforms.NewFeishu(pc.Options["webhook_url"]), nil
case "dingtalk":
    return platforms.NewDingTalk(pc.Options["webhook_url"]), nil
case "wecom":
    return platforms.NewWeCom(pc.Options["webhook_url"]), nil
case "email":
    return platforms.NewEmail(
        pc.Options["host"], pc.Options["port"],
        pc.Options["username"], pc.Options["password"],
        pc.Options["from"], pc.Options["to"],
    ), nil
case "sms":
    return platforms.NewSMS(
        pc.Options["account_sid"], pc.Options["auth_token"],
        pc.Options["from"], pc.Options["to"],
    ), nil
```

- [ ] `go build ./... && go test ./...` — PASS.
- [ ] Commit `feat(cli): wire eight new platforms into the gateway buildPlatform switch`.

---

## Verification Checklist

- [ ] All eight new adapters listed in `reg.platforms` when enabled
- [ ] `go test ./gateway/platforms/...` covers each new adapter
- [ ] Twilio adapter works against a local `httptest.Server` under the BaseURL override
