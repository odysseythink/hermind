# Phase 4: MTProto Telegram User Account Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable Telegram user account login alongside the existing bot API polling adapter, plus a DNS-over-HTTPS fallback transport for network-restricted environments.

**Architecture:** Use `github.com/gotd/td` for MTProto protocol handling. The adapter implements the existing `gateway.Platform` interface (Name/Run/SendReply). Session persistence via gotd's built-in file storage. The DoH transport is a standalone `http.RoundTripper` usable by both bot and user adapters.

**Tech Stack:** Go 1.25, `github.com/gotd/td` (MTProto client), stdlib `net/http`, `encoding/json`

---

## File Structure

```
hermes-agent-go/
├── gateway/
│   ├── platforms/
│   │   ├── telegram.go              # (existing bot adapter, unchanged)
│   │   ├── telegram_test.go         # (existing, unchanged)
│   │   ├── telegram_user.go         # MTProto user account adapter
│   │   ├── telegram_user_test.go    # Tests for message dispatch logic
│   │   ├── telegram_doh.go          # DNS-over-HTTPS fallback transport
│   │   └── telegram_doh_test.go     # Tests for DoH transport
```

---

### Task 1: Add gotd/td dependency

**Files:**
- Modify: `hermes-agent-go/go.mod`

- [ ] **Step 1: Add the dependency**

```bash
cd hermes-agent-go && go get github.com/gotd/td@latest
```

- [ ] **Step 2: Tidy**

```bash
cd hermes-agent-go && go mod tidy
```

- [ ] **Step 3: Verify it appears**

```bash
grep gotd hermes-agent-go/go.mod
```

Expected: `github.com/gotd/td` with a version.

- [ ] **Step 4: Run existing tests**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/go.mod hermes-agent-go/go.sum
git commit -m "feat(deps): add gotd/td MTProto library for Telegram user accounts"
```

---

### Task 2: DNS-over-HTTPS fallback transport

**Files:**
- Create: `hermes-agent-go/gateway/platforms/telegram_doh.go`
- Create: `hermes-agent-go/gateway/platforms/telegram_doh_test.go`

A standalone `http.RoundTripper` that tries alternative IPs (discovered via DoH) when the primary connection to `api.telegram.org` fails. Ported from the Python reference's `telegram_network.py`.

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/gateway/platforms/telegram_doh_test.go`:

```go
package platforms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoHResolve(t *testing.T) {
	// Mock DoH server returns one A record.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		resp := dohResponse{
			Answer: []dohAnswer{{Type: 1, Data: "1.2.3.4"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ips, err := dohResolve(context.Background(), srv.URL, "api.telegram.org")
	if err != nil {
		t.Fatalf("dohResolve: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("ips = %v", ips)
	}
}

func TestDoHResolveMergesProviders(t *testing.T) {
	// Two mock providers each return different IPs.
	var hits int32
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		json.NewEncoder(w).Encode(dohResponse{
			Answer: []dohAnswer{{Type: 1, Data: "10.0.0.1"}},
		})
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		json.NewEncoder(w).Encode(dohResponse{
			Answer: []dohAnswer{{Type: 1, Data: "10.0.0.2"}},
		})
	}))
	defer srv2.Close()

	ips := discoverFallbackIPs(context.Background(), "api.telegram.org", []string{srv1.URL, srv2.URL})
	if len(ips) < 2 {
		t.Errorf("expected >= 2 IPs, got %v", ips)
	}
}

func TestDoHTransportFallsBack(t *testing.T) {
	// Primary server fails, fallback server succeeds.
	var fallbackHits int32
	fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fallbackHits, 1)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer fallbackSrv.Close()

	transport := &DoHTransport{
		FallbackIPs: []string{fallbackSrv.Listener.Addr().String()},
		Primary:     &failingTransport{},
		Timeout:     2 * time.Second,
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get("http://api.telegram.org/test")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&fallbackHits) != 1 {
		t.Errorf("fallback hits = %d", fallbackHits)
	}
}

// failingTransport always returns a connection error.
type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestDoH -v
```

Expected: Compilation error — types undefined.

- [ ] **Step 3: Implement telegram_doh.go**

Create `hermes-agent-go/gateway/platforms/telegram_doh.go`:

```go
package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Default DoH providers.
var defaultDoHProviders = []string{
	"https://dns.google/resolve",
	"https://cloudflare-dns.com/dns-query",
}

// Seed fallback IP for api.telegram.org.
const telegramSeedIP = "149.154.167.220"

// dohResponse is the JSON-format DNS response from DoH providers.
type dohResponse struct {
	Answer []dohAnswer `json:"Answer"`
}

type dohAnswer struct {
	Type int    `json:"type"` // 1 = A record
	Data string `json:"data"`
}

// dohResolve queries a single DoH provider for A records.
func dohResolve(ctx context.Context, provider, hostname string) ([]string, error) {
	u, err := url.Parse(provider)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("name", hostname)
	q.Set("type", "A")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var doh dohResponse
	if err := json.NewDecoder(resp.Body).Decode(&doh); err != nil {
		return nil, err
	}

	var ips []string
	for _, a := range doh.Answer {
		if a.Type == 1 { // A record
			ips = append(ips, a.Data)
		}
	}
	return ips, nil
}

// discoverFallbackIPs queries multiple DoH providers and deduplicates results.
func discoverFallbackIPs(ctx context.Context, hostname string, providers []string) []string {
	if len(providers) == 0 {
		providers = defaultDoHProviders
	}

	type result struct {
		ips []string
		err error
	}

	ch := make(chan result, len(providers))
	for _, p := range providers {
		go func(provider string) {
			ips, err := dohResolve(ctx, provider, hostname)
			ch <- result{ips, err}
		}(p)
	}

	seen := make(map[string]bool)
	var all []string
	for range providers {
		r := <-ch
		if r.err != nil {
			slog.Debug("doh: provider failed", "err", r.err)
			continue
		}
		for _, ip := range r.ips {
			if !seen[ip] {
				seen[ip] = true
				all = append(all, ip)
			}
		}
	}

	// Add seed IP if no results.
	if len(all) == 0 {
		all = append(all, telegramSeedIP)
	}
	return all
}

// DoHTransport is an http.RoundTripper that falls back to alternative IPs
// when the primary transport fails with a connection error.
type DoHTransport struct {
	// FallbackIPs are tried in order when Primary fails.
	FallbackIPs []string

	// Primary is the default transport (nil = http.DefaultTransport).
	Primary http.RoundTripper

	// Timeout for fallback connections.
	Timeout time.Duration

	// Sticky IP — once a fallback succeeds, prefer it.
	mu      sync.Mutex
	stickyIP string
}

func (d *DoHTransport) primary() http.RoundTripper {
	if d.Primary != nil {
		return d.Primary
	}
	return http.DefaultTransport
}

func (d *DoHTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Try sticky IP first if set.
	d.mu.Lock()
	sticky := d.stickyIP
	d.mu.Unlock()

	if sticky != "" {
		resp, err := d.tryFallback(req, sticky)
		if err == nil {
			return resp, nil
		}
	}

	// Try primary.
	resp, err := d.primary().RoundTrip(req)
	if err == nil {
		return resp, nil
	}

	// Only fallback on connection errors.
	if !isConnError(err) {
		return nil, err
	}

	slog.Debug("doh: primary failed, trying fallbacks", "err", err)

	// Try each fallback IP.
	for _, ip := range d.FallbackIPs {
		if ip == sticky {
			continue // Already tried.
		}
		resp, err := d.tryFallback(req, ip)
		if err == nil {
			d.mu.Lock()
			d.stickyIP = ip
			d.mu.Unlock()
			return resp, nil
		}
	}

	return nil, fmt.Errorf("doh: all fallbacks exhausted: %w", err)
}

func (d *DoHTransport) tryFallback(req *http.Request, ip string) (*http.Response, error) {
	// Clone the request and rewrite the URL to use the fallback IP.
	clone := req.Clone(req.Context())
	origHost := clone.URL.Host

	// Preserve port if present, otherwise use the IP directly.
	if strings.Contains(ip, ":") {
		clone.URL.Host = ip
	} else {
		_, port, _ := net.SplitHostPort(origHost)
		if port != "" {
			clone.URL.Host = net.JoinHostPort(ip, port)
		} else {
			clone.URL.Host = ip
		}
	}

	// Preserve the original Host header for TLS SNI.
	if clone.Host == "" {
		clone.Host = origHost
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: d.Timeout}).DialContext,
	}
	return transport.RoundTrip(clone)
}

func isConnError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if ok := errorAs(err, &opErr); ok {
		return true
	}
	// Also catch URL errors wrapping net errors.
	var urlErr *url.Error
	if ok := errorAs(err, &urlErr); ok {
		return isConnError(urlErr.Err)
	}
	return false
}

// errorAs is a generic helper matching errors.As.
func errorAs[T any](err error, target *T) bool {
	for err != nil {
		if t, ok := err.(T); ok {
			*target = t
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
```

**Note:** The `errorAs` function above uses generics. If this causes issues with the `*net.OpError` pointer type, replace with `errors.As` from the `errors` package:

```go
import "errors"

func isConnError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isConnError(urlErr.Err)
	}
	return false
}
```

Remove the `errorAs` generic helper if using `errors.As`.

- [ ] **Step 4: Fix test imports**

The test needs `net` and `fmt` for the `failingTransport`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)
```

- [ ] **Step 5: Run tests**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestDoH -v
```

Expected: All 3 DoH tests pass.

- [ ] **Step 6: Run all gateway tests**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass.

- [ ] **Step 7: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/gateway/platforms/telegram_doh.go hermes-agent-go/gateway/platforms/telegram_doh_test.go
git commit -m "feat(gateway): add DNS-over-HTTPS fallback transport for Telegram"
```

---

### Task 3: Telegram user adapter — core implementation

**Files:**
- Create: `hermes-agent-go/gateway/platforms/telegram_user.go`

The adapter uses gotd/td for MTProto protocol. It implements the `gateway.Platform` interface.

- [ ] **Step 1: Create telegram_user.go**

Create `hermes-agent-go/gateway/platforms/telegram_user.go`:

```go
package platforms

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"

	"github.com/nousresearch/hermes-agent/gateway"
)

// TelegramUser is a bidirectional Telegram adapter using MTProto
// for both inbound messages and outbound replies. Requires a user
// account (phone number), not a bot token.
type TelegramUser struct {
	appID       int
	appHash     string
	phone       string
	sessionPath string // path to session file for auth persistence

	// codePrompt is called during first-time auth to get the verification code.
	// In production this could prompt the user via CLI or another channel.
	codePrompt func(ctx context.Context) (string, error)

	// passwordPrompt is called if 2FA is enabled.
	passwordPrompt func(ctx context.Context) (string, error)

	// api is set during Run for SendReply to use.
	api *tg.Client

	// sender is set during Run for SendReply to use.
	sender *message.Sender
}

// TelegramUserConfig holds construction parameters.
type TelegramUserConfig struct {
	AppID          int
	AppHash        string
	Phone          string
	SessionPath    string // default: ~/.hermes/telegram_session.json
	CodePrompt     func(ctx context.Context) (string, error)
	PasswordPrompt func(ctx context.Context) (string, error)
}

func NewTelegramUser(cfg TelegramUserConfig) *TelegramUser {
	if cfg.SessionPath == "" {
		cfg.SessionPath = "telegram_session.json"
	}
	return &TelegramUser{
		appID:          cfg.AppID,
		appHash:        cfg.AppHash,
		phone:          cfg.Phone,
		sessionPath:    cfg.SessionPath,
		codePrompt:     cfg.CodePrompt,
		passwordPrompt: cfg.PasswordPrompt,
	}
}

func (t *TelegramUser) Name() string { return "telegram_user" }

func (t *TelegramUser) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if t.appID == 0 || t.appHash == "" || t.phone == "" {
		return fmt.Errorf("telegram_user: app_id, app_hash, and phone required")
	}

	// Update dispatcher routes incoming events.
	dispatcher := tg.NewUpdateDispatcher()

	// Handle new private/group messages.
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok || msg.Message == "" {
			return nil
		}
		// Skip outgoing messages.
		if msg.Out {
			return nil
		}
		in := gateway.IncomingMessage{
			Platform:  t.Name(),
			UserID:    strconv.Itoa(t.peerID(msg)),
			ChatID:    strconv.Itoa(t.chatID(msg)),
			Text:      msg.Message,
			MessageID: strconv.Itoa(msg.ID),
		}
		gateway.DispatchAndReply(ctx, t, handler, in)
		return nil
	})

	// Gaps handler manages the update stream.
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	// Create the MTProto client.
	client := telegram.NewClient(t.appID, t.appHash, telegram.Options{
		UpdateHandler:  gaps,
		SessionStorage: &session.FileStorage{Path: t.sessionPath},
	})

	return client.Run(ctx, func(ctx context.Context) error {
		// Authenticate if necessary.
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram_user: auth status: %w", err)
		}
		if !status.Authorized {
			if err := t.authenticate(ctx, client); err != nil {
				return fmt.Errorf("telegram_user: auth: %w", err)
			}
		}

		slog.Info("telegram_user: authenticated", "phone", t.phone)

		// Store API client for SendReply.
		t.api = client.API()
		t.sender = message.NewSender(t.api)

		// Block until context is cancelled. Updates are handled
		// by the dispatcher via the gaps handler.
		<-ctx.Done()
		return ctx.Err()
	})
}

func (t *TelegramUser) authenticate(ctx context.Context, client *telegram.Client) error {
	// Send code.
	sentCode, err := client.Auth().SendCode(ctx, t.phone, telegram.SendCodeOptions{})
	if err != nil {
		return err
	}

	// Prompt for code.
	if t.codePrompt == nil {
		return fmt.Errorf("telegram_user: code prompt not configured")
	}
	code, err := t.codePrompt(ctx)
	if err != nil {
		return err
	}

	// Sign in with code.
	_, signInErr := client.Auth().SignIn(ctx, t.phone, code, sentCode.PhoneCodeHash)
	if signInErr == nil {
		return nil
	}

	// Check if 2FA is required.
	// gotd returns a specific error type when 2FA is needed.
	if t.passwordPrompt == nil {
		return fmt.Errorf("telegram_user: 2FA required but no password prompt configured")
	}
	password, err := t.passwordPrompt(ctx)
	if err != nil {
		return err
	}
	_, err = client.Auth().Password(ctx, password)
	return err
}

// peerID extracts the user ID from a message's PeerID.
func (t *TelegramUser) peerID(msg *tg.Message) int {
	if p, ok := msg.FromID.(*tg.PeerUser); ok {
		return p.UserID
	}
	if p, ok := msg.PeerID.(*tg.PeerUser); ok {
		return p.UserID
	}
	return 0
}

// chatID extracts the chat ID from a message's PeerID.
func (t *TelegramUser) chatID(msg *tg.Message) int {
	switch p := msg.PeerID.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return p.ChatID
	case *tg.PeerChannel:
		return p.ChannelID
	default:
		return 0
	}
}

func (t *TelegramUser) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if t.sender == nil {
		return fmt.Errorf("telegram_user: not connected")
	}
	if out.ChatID == "" {
		return fmt.Errorf("telegram_user: chat_id required")
	}
	chatID, err := strconv.Atoi(out.ChatID)
	if err != nil {
		return fmt.Errorf("telegram_user: invalid chat_id: %w", err)
	}

	// Try as user chat first, then as channel.
	_, sendErr := t.sender.To(&tg.InputPeerUser{UserID: int64(chatID)}).Text(ctx, out.Text)
	if sendErr != nil {
		slog.Debug("telegram_user: user send failed, trying channel", "err", sendErr)
		_, sendErr = t.sender.To(&tg.InputPeerChannel{ChannelID: int64(chatID)}).Text(ctx, out.Text)
	}
	return sendErr
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd hermes-agent-go && go build ./gateway/platforms/
```

Expected: Compiles without errors. There may be gotd/td API adjustments needed — the field names and method signatures should be verified against the actual library version downloaded. Fix any compilation errors.

- [ ] **Step 3: Run existing tests to ensure no regressions**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/gateway/platforms/telegram_user.go
git commit -m "feat(gateway): add Telegram MTProto user account adapter"
```

---

### Task 4: Telegram user adapter — unit tests

**Files:**
- Create: `hermes-agent-go/gateway/platforms/telegram_user_test.go`

Testing the full MTProto connection requires a real Telegram server. Instead, we test:
1. The `peerID` and `chatID` extraction logic
2. The Name() and config validation
3. Interface compliance

- [ ] **Step 1: Create telegram_user_test.go**

Create `hermes-agent-go/gateway/platforms/telegram_user_test.go`:

```go
package platforms

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestTelegramUserName(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	if tu.Name() != "telegram_user" {
		t.Errorf("Name() = %q", tu.Name())
	}
}

func TestTelegramUserMissingConfig(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	err := tu.Run(context.Background(), func(_ context.Context, _ gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		return nil, nil
	})
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestTelegramUserSendReplyNotConnected(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	err := tu.SendReply(context.Background(), gateway.OutgoingMessage{ChatID: "123", Text: "hi"})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestTelegramUserPeerID(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})

	msg := &tg.Message{
		FromID: &tg.PeerUser{UserID: 42},
		PeerID: &tg.PeerChat{ChatID: 99},
	}
	if id := tu.peerID(msg); id != 42 {
		t.Errorf("peerID = %d, want 42", id)
	}
}

func TestTelegramUserChatID(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})

	tests := []struct {
		name   string
		peerID tg.PeerClass
		want   int
	}{
		{"user", &tg.PeerUser{UserID: 42}, 42},
		{"chat", &tg.PeerChat{ChatID: 99}, 99},
		{"channel", &tg.PeerChannel{ChannelID: 123}, 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &tg.Message{PeerID: tt.peerID}
			if id := tu.chatID(msg); id != tt.want {
				t.Errorf("chatID = %d, want %d", id, tt.want)
			}
		})
	}
}

// Verify TelegramUser satisfies the Platform interface at compile time.
var _ gateway.Platform = (*TelegramUser)(nil)
```

- [ ] **Step 2: Run tests**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestTelegramUser -v
```

Expected: All 5 tests pass.

- [ ] **Step 3: Run all gateway tests**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/gateway/platforms/telegram_user_test.go
git commit -m "test(gateway): add Telegram user adapter unit tests"
```
