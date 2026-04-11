# Plan 7b.1: Signal / WhatsApp / Matrix / Home Assistant Platforms

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Port the four remaining practical gateway platforms — Signal (via signal-cli JSON-RPC), WhatsApp (Business Cloud API), Matrix (client-server API), and Home Assistant (REST). All four are outbound-only in Plan 7b.1; pair with api_server for inbound.

**Non-goal:** `telegram_network` (MTProto user-account) requires a full MTProto implementation with TL schema, session files, and key exchange. This is hundreds of KB of Go for a very niche variant of an already-supported platform (Telegram Bot is Plan 7). It is left for a future dedicated plan when/if someone needs it.

**Architecture:** Each adapter is a new file in `gateway/platforms/`. Signal uses the existing `signal-cli daemon --http` JSON-RPC 2.0 endpoint. WhatsApp, Matrix, and Home Assistant are direct HTTP adapters. All four satisfy the existing `gateway.Platform` interface with a no-op `Run` and a real `SendReply`.

---

## Task 1: Signal adapter (signal-cli JSON-RPC over HTTP)

- [ ] Create `gateway/platforms/signal.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// Signal is an outbound-only adapter for signal-cli daemon running
// in HTTP/JSON-RPC mode (`signal-cli daemon --http 127.0.0.1:8080`).
type Signal struct {
	BaseURL   string // e.g. http://127.0.0.1:8080
	Account   string // +1234567890
	client    *http.Client
}

func NewSignal(baseURL, account string) *Signal {
	return &Signal{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Account: account,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Signal) Name() string { return "signal" }
func (s *Signal) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done(); return nil
}

type signalRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
	ID      int            `json:"id"`
}

func (s *Signal) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if s.BaseURL == "" || s.Account == "" || out.ChatID == "" {
		return fmt.Errorf("signal: base_url/account/chat_id required")
	}
	req := signalRPCRequest{
		JSONRPC: "2.0",
		Method:  "send",
		Params: map[string]any{
			"account":   s.Account,
			"recipient": []string{out.ChatID},
			"message":   out.Text,
		},
		ID: 1,
	}
	buf, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.BaseURL, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("signal: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("signal: status %d: %s", resp.StatusCode, string(body))
	}
	// JSON-RPC errors are embedded in the 200 body.
	if bytes.Contains(body, []byte(`"error"`)) {
		return fmt.Errorf("signal: rpc error: %s", string(body))
	}
	return nil
}
```

- [ ] Create `gateway/platforms/signal_test.go` with an `httptest.Server` that returns `{"jsonrpc":"2.0","id":1,"result":{"timestamp":0}}` and verifies method + params.
- [ ] Commit `feat(gateway/platforms): add Signal (signal-cli JSON-RPC) adapter`.

---

## Task 2: WhatsApp Business Cloud API

- [ ] Create `gateway/platforms/whatsapp.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// WhatsApp is an outbound-only adapter for the WhatsApp Business
// Cloud API: POST https://graph.facebook.com/v19.0/{phone-id}/messages
// with a Bearer access token.
type WhatsApp struct {
	BaseURL     string // default https://graph.facebook.com
	APIVersion  string // default v19.0
	PhoneID     string // e.g. 1234567890
	AccessToken string
	client      *http.Client
}

func NewWhatsApp(phoneID, accessToken string) *WhatsApp {
	return &WhatsApp{
		BaseURL:     "https://graph.facebook.com",
		APIVersion:  "v19.0",
		PhoneID:     phoneID,
		AccessToken: accessToken,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBaseURL overrides the Graph API base URL for tests.
func (w *WhatsApp) WithBaseURL(u string) *WhatsApp {
	w.BaseURL = strings.TrimRight(u, "/")
	return w
}

func (w *WhatsApp) Name() string { return "whatsapp" }
func (w *WhatsApp) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done(); return nil
}

func (w *WhatsApp) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if w.PhoneID == "" || w.AccessToken == "" || out.ChatID == "" {
		return fmt.Errorf("whatsapp: phone_id/access_token/chat_id required")
	}
	url := fmt.Sprintf("%s/%s/%s/messages", w.BaseURL, w.APIVersion, w.PhoneID)
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                out.ChatID,
		"type":              "text",
		"text":              map[string]string{"body": out.Text},
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.AccessToken)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("whatsapp: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] Create `gateway/platforms/whatsapp_test.go` with `httptest.Server`.
- [ ] Commit `feat(gateway/platforms): add WhatsApp Business Cloud API adapter`.

---

## Task 3: Matrix client-server API

- [ ] Create `gateway/platforms/matrix.go` using `PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}` with Bearer auth.
- [ ] Create `gateway/platforms/matrix_test.go`.
- [ ] Commit `feat(gateway/platforms): add Matrix client-server adapter`.

---

## Task 4: Home Assistant REST

- [ ] Create `gateway/platforms/homeassistant.go` calling `POST /api/services/notify/{service}` on the HA instance with Bearer auth. Payload: `{"message":"..."}`.
- [ ] Create `gateway/platforms/homeassistant_test.go`.
- [ ] Commit `feat(gateway/platforms): add Home Assistant notify adapter`.

---

## Task 5: CLI wiring

- [ ] Extend `cli/gateway.go` `buildPlatform` switch with `signal`, `whatsapp`, `matrix`, `homeassistant` cases.
- [ ] Commit `feat(cli): wire Signal/WhatsApp/Matrix/HomeAssistant into gateway builder`.

---

## Verification Checklist

- [ ] `go test ./gateway/platforms/...` passes
- [ ] `go test ./...` passes
