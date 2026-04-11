# Plan 7: Gateway + Platform Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Introduce a gateway service that receives messages from one or more platforms, routes each to a per-user agent Engine, and delivers the reply back over the same platform. Ship with three concrete platform adapters — **HTTP API** (generic request/response), **webhook** (outbound POST-based), and **Telegram Bot** (polling) — plus a clear `Platform` interface so the remaining 18 platforms can be added incrementally without touching the Gateway core.

**Architecture:** The `gateway.Gateway` struct owns a `map[string]Platform` of registered adapters and a `SessionStore` that caches per-user conversation state. On `Start`, it calls `Platform.Run(ctx, handler)` for each adapter in its own goroutine; the handler is a `MessageHandler func(ctx, IncomingMessage) error` provided by the Gateway itself. When an `IncomingMessage` arrives, the Gateway builds or looks up a session, spawns a fresh `agent.Engine`, runs one turn, and calls `Platform.SendReply`. Graceful shutdown is a `context.Context` cancellation. Errors are logged to stderr (structured logging is deferred to Plan 8).

**Tech Stack:** Go 1.25, stdlib `net/http`, existing `agent`, `provider`, `tool`, `storage`, `config`. Telegram uses Bot API via `net/http` (no SDK dep).

**Deliverable at end of plan:**
```
$ hermes gateway
2026-04-11 12:00:00 gateway: starting platforms: [api_server, telegram, webhook]
2026-04-11 12:00:00 api_server: listening on :8080
2026-04-11 12:00:00 telegram: polling started
```

```
$ curl -X POST http://localhost:8080/message \
    -d '{"user_id":"alice","text":"hello"}'
{"reply":"Hi alice! How can I help?"}
```

**Non-goals for this plan (deferred):**
- Discord, Slack, WhatsApp, Signal, Matrix, Feishu, DingTalk, WeCom, Email, SMS, Mattermost, Webhook-inbound, Home Assistant — interface is in place, adapters deferred to Plan 7b
- Delivery retry / idempotency — deferred
- Per-user rate limiting — deferred
- Streaming replies to platforms that support them — initial version is non-streaming
- Multi-tenant session persistence to SQLite — in-memory only for Plan 7 (Plan 7b)

**Plans 1-6e dependencies this plan touches:**
- `config/config.go` — add `GatewayConfig`
- `gateway/` — NEW package: gateway.go, platform.go, session.go, handler.go, tests
- `gateway/platforms/api_server.go` — NEW
- `gateway/platforms/webhook.go` — NEW
- `gateway/platforms/telegram.go` — NEW
- `cli/root.go` — add `gateway` subcommand
- `cli/gateway.go` — NEW

---

## File Structure

```
hermes-agent-go/
├── config/config.go                            # MODIFIED: add GatewayConfig
├── gateway/
│   ├── gateway.go                              # Gateway struct + Run/Stop
│   ├── gateway_test.go
│   ├── platform.go                             # Platform interface + types
│   ├── session.go                              # SessionStore
│   ├── session_test.go
│   ├── handler.go                              # Dispatch incoming → Engine → reply
│   └── platforms/
│       ├── api_server.go                       # HTTP JSON server adapter
│       ├── api_server_test.go
│       ├── webhook.go                          # Outbound webhook adapter
│       ├── webhook_test.go
│       ├── telegram.go                         # Telegram Bot API polling adapter
│       └── telegram_test.go
├── cli/
│   ├── root.go                                 # MODIFIED: add gateway subcommand
│   └── gateway.go                              # newGatewayCmd
```

---

## Task 1: Config block

- [ ] **Step 1:** Add to `config/config.go`:

```go
// GatewayConfig controls the multi-platform gateway.
type GatewayConfig struct {
	Platforms map[string]PlatformConfig `yaml:"platforms,omitempty"`
}

// PlatformConfig is an untyped configuration blob passed to the
// platform adapter. Known keys depend on the adapter.
type PlatformConfig struct {
	Enabled bool              `yaml:"enabled"`
	Type    string            `yaml:"type"` // "api_server", "webhook", "telegram", ...
	Options map[string]string `yaml:"options,omitempty"`
}
```

Add `Gateway GatewayConfig `yaml:"gateway,omitempty"`` to `Config`.

- [ ] **Step 2:** `go test ./config/...` — PASS.
- [ ] **Step 3:** Commit `feat(config): add GatewayConfig block`.

---

## Task 2: gateway package core

- [ ] **Step 1:** Create `gateway/platform.go`:

```go
// Package gateway is the multi-platform messaging front end.
// It receives messages from one or more Platform adapters, routes
// each message to a per-user agent.Engine conversation, and sends
// replies back. Adapters are registered in main and driven by a
// shared Gateway instance.
package gateway

import "context"

// IncomingMessage is the platform-agnostic shape every adapter emits.
type IncomingMessage struct {
	Platform string // "telegram", "api_server", etc.
	UserID   string // platform-specific user identifier
	ChatID   string // optional group/channel identifier
	Text     string // user text (may be empty for media-only)
	// Extra holds raw platform fields (JSON-serializable) for debugging.
	Extra map[string]any
}

// OutgoingMessage is what the Gateway hands back to the adapter.
type OutgoingMessage struct {
	UserID string
	ChatID string
	Text   string
}

// MessageHandler is the callback adapters invoke for each new message.
// Adapters are expected to call this from a goroutine per message; the
// Gateway takes care of session isolation.
type MessageHandler func(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error)

// Platform is the interface every adapter implements.
type Platform interface {
	// Name returns the canonical platform name.
	Name() string

	// Run starts the adapter loop (polling, HTTP server, websocket,
	// …). It must block until ctx is cancelled. Incoming messages
	// are delivered via handler. Errors returned shut the Gateway
	// down.
	Run(ctx context.Context, handler MessageHandler) error

	// SendReply pushes a message back to the user via the platform.
	// Called by the Gateway after the Engine produces a response.
	SendReply(ctx context.Context, out OutgoingMessage) error
}
```

- [ ] **Step 2:** Create `gateway/session.go`:

```go
package gateway

import (
	"sync"

	"github.com/nousresearch/hermes-agent/message"
)

// Session holds cached conversation state for one (platform, user).
type Session struct {
	ID       string
	Platform string
	UserID   string
	History  []message.Message
}

// SessionStore is an in-memory session cache. Plan 7b should move
// this to SQLite for durability.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// Key returns the map key used to store a session.
func Key(platform, userID string) string {
	return platform + ":" + userID
}

// GetOrCreate returns the existing session, or creates a fresh one
// if none exists. Safe for concurrent use.
func (s *SessionStore) GetOrCreate(platform, userID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := Key(platform, userID)
	sess, ok := s.sessions[k]
	if !ok {
		sess = &Session{
			ID:       k,
			Platform: platform,
			UserID:   userID,
		}
		s.sessions[k] = sess
	}
	return sess
}

// Append adds a turn to the session history.
func (s *SessionStore) Append(sess *Session, msgs ...message.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess.History = append(sess.History, msgs...)
}

// Reset clears the session history for a given key.
func (s *SessionStore) Reset(platform, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, Key(platform, userID))
}
```

- [ ] **Step 3:** Create `gateway/session_test.go`:

```go
package gateway

import (
	"testing"

	"github.com/nousresearch/hermes-agent/message"
)

func TestSessionStoreGetOrCreateAndAppend(t *testing.T) {
	s := NewSessionStore()
	a := s.GetOrCreate("tg", "u1")
	if a == nil || a.UserID != "u1" {
		t.Fatalf("unexpected session: %+v", a)
	}
	b := s.GetOrCreate("tg", "u1")
	if a != b {
		t.Errorf("expected same pointer on repeat call")
	}
	s.Append(a, message.Message{Role: message.RoleUser, Content: message.TextContent("hi")})
	if len(a.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(a.History))
	}
	s.Reset("tg", "u1")
	c := s.GetOrCreate("tg", "u1")
	if c == a {
		t.Errorf("expected fresh session after reset")
	}
}
```

- [ ] **Step 4:** Create `gateway/gateway.go`:

```go
package gateway

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// Gateway routes messages from one or more Platform adapters into
// per-user agent.Engine conversations.
type Gateway struct {
	cfg       config.Config
	provider  provider.Provider
	aux       provider.Provider
	storage   storage.Storage
	tools     *tool.Registry
	platforms map[string]Platform
	sessions  *SessionStore
}

// NewGateway builds a Gateway with the given dependencies. Platforms
// are added separately via Register.
func NewGateway(cfg config.Config, p, aux provider.Provider, s storage.Storage, reg *tool.Registry) *Gateway {
	return &Gateway{
		cfg:       cfg,
		provider:  p,
		aux:       aux,
		storage:   s,
		tools:     reg,
		platforms: make(map[string]Platform),
		sessions:  NewSessionStore(),
	}
}

// Register adds a platform adapter. Duplicate names replace prior entries.
func (g *Gateway) Register(p Platform) {
	g.platforms[p.Name()] = p
}

// Start runs all registered platforms in their own goroutines and
// blocks until ctx is done or any Platform.Run returns a non-nil error.
func (g *Gateway) Start(ctx context.Context) error {
	if len(g.platforms) == 0 {
		return fmt.Errorf("gateway: no platforms registered")
	}
	errCh := make(chan error, len(g.platforms))
	var wg sync.WaitGroup

	for name, p := range g.platforms {
		wg.Add(1)
		go func(name string, p Platform) {
			defer wg.Done()
			log.Printf("gateway: starting platform %s", name)
			if err := p.Run(ctx, g.handleMessage); err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf("gateway: %s: %w", name, err)
			}
		}(name, p)
	}

	// Wait for either ctx cancel or a platform error.
	select {
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// handleMessage is the MessageHandler passed to each platform. It
// looks up the session, runs one Engine turn, and returns the reply.
func (g *Gateway) handleMessage(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	sess := g.sessions.GetOrCreate(in.Platform, in.UserID)

	eng := agent.NewEngineWithToolsAndAux(
		g.provider, g.aux, g.storage, g.tools,
		g.cfg.Agent, in.Platform,
	)
	result, err := eng.RunConversation(ctx, &agent.RunOptions{
		UserMessage: in.Text,
		History:     sess.History,
		SessionID:   sess.ID,
		UserID:      in.UserID,
		Model:       modelFromCfg(g.cfg),
	})
	if err != nil {
		return nil, err
	}
	// Update the session history with the new turns.
	g.sessions.Append(sess, result.Messages[len(sess.History):]...)
	return &OutgoingMessage{
		UserID: in.UserID,
		ChatID: in.ChatID,
		Text:   result.Response.Content.Text(),
	}, nil
}

// modelFromCfg extracts "model name only" from cfg.Model (strip provider/).
func modelFromCfg(cfg config.Config) string {
	m := cfg.Model
	for i := 0; i < len(m); i++ {
		if m[i] == '/' {
			return m[i+1:]
		}
	}
	return m
}
```

- [ ] **Step 5:** Create `gateway/handler.go` with a helper the adapters use to wrap MessageHandler + SendReply:

```go
package gateway

import (
	"context"
	"log"
)

// DispatchAndReply is a convenience that adapters can call inside
// their event loop: it runs the MessageHandler and calls SendReply
// if it succeeds. Errors are logged.
func DispatchAndReply(ctx context.Context, p Platform, handler MessageHandler, in IncomingMessage) {
	out, err := handler(ctx, in)
	if err != nil {
		log.Printf("gateway: %s: handler error: %v", p.Name(), err)
		return
	}
	if out == nil {
		return
	}
	if err := p.SendReply(ctx, *out); err != nil {
		log.Printf("gateway: %s: send reply error: %v", p.Name(), err)
	}
}
```

- [ ] **Step 6:** Create `gateway/gateway_test.go` with a fake platform + fake provider:

```go
package gateway

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/tool"
)

// fakePlatform sends one canned message, records replies, and blocks
// until ctx is cancelled.
type fakePlatform struct {
	name    string
	send    IncomingMessage
	replies []OutgoingMessage
	mu      sync.Mutex
	started chan struct{}
}

func (f *fakePlatform) Name() string { return f.name }
func (f *fakePlatform) Run(ctx context.Context, h MessageHandler) error {
	close(f.started)
	DispatchAndReply(ctx, f, h, f.send)
	<-ctx.Done()
	return nil
}
func (f *fakePlatform) SendReply(ctx context.Context, out OutgoingMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, out)
	return nil
}

type echoProvider struct{}

func (echoProvider) Name() string { return "echo" }
func (echoProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	last := req.Messages[len(req.Messages)-1]
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent("echo: " + last.Content.Text()),
		},
		FinishReason: "stop",
	}, nil
}
func (echoProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return nil, errors.New("no stream")
}
func (echoProvider) ModelInfo(string) *provider.ModelInfo       { return &provider.ModelInfo{ContextLength: 8000} }
func (echoProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (echoProvider) Available() bool                            { return true }

func TestGatewayRoutesMessageAndReplies(t *testing.T) {
	fp := &fakePlatform{
		name:    "fake",
		send:    IncomingMessage{Platform: "fake", UserID: "u1", Text: "hello"},
		started: make(chan struct{}),
	}
	cfg := config.Config{
		Model: "anthropic/claude-opus-4-6",
		Agent: config.AgentConfig{MaxTurns: 3},
	}
	g := NewGateway(cfg, echoProvider{}, nil, nil, tool.NewRegistry())
	g.Register(fp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- g.Start(ctx) }()
	<-fp.started
	// Give the dispatch goroutine time to send a reply.
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-errCh

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(fp.replies))
	}
	if fp.replies[0].Text != "echo: hello" {
		t.Errorf("unexpected reply: %q", fp.replies[0].Text)
	}
}
```

- [ ] **Step 7:** Run `go test ./gateway/...` — PASS.
- [ ] **Step 8:** Commit `feat(gateway): add core Gateway, SessionStore, and Platform interface`.

---

## Task 3: api_server platform

A generic HTTP JSON endpoint for local testing and integration with custom clients.

- [ ] **Step 1:** Create `gateway/platforms/api_server.go`:

```go
// Package platforms contains platform adapters for the gateway.
package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// APIServer is a simple HTTP JSON adapter. POST /message with a JSON
// body {"user_id":"...", "text":"..."} returns the reply synchronously.
// Suitable for local development and programmatic integration.
type APIServer struct {
	addr string
	mu   sync.Mutex
	// handler is captured during Run so SendReply can't be used from
	// outside the request/response lifecycle. This adapter replies
	// inline, so SendReply is a no-op.
	pending map[string]chan gateway.OutgoingMessage
	srv     *http.Server
}

// NewAPIServer builds an APIServer listening on addr (e.g. ":8080").
func NewAPIServer(addr string) *APIServer {
	if addr == "" {
		addr = ":8080"
	}
	return &APIServer{addr: addr, pending: make(map[string]chan gateway.OutgoingMessage)}
}

func (a *APIServer) Name() string { return "api_server" }

type apiRequest struct {
	UserID string `json:"user_id"`
	ChatID string `json:"chat_id,omitempty"`
	Text   string `json:"text"`
}

type apiReply struct {
	Reply string `json:"reply"`
}

func (a *APIServer) Run(ctx context.Context, handler gateway.MessageHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req apiRequest
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.UserID == "" || req.Text == "" {
			http.Error(w, "user_id and text are required", http.StatusBadRequest)
			return
		}
		in := gateway.IncomingMessage{
			Platform: a.Name(),
			UserID:   req.UserID,
			ChatID:   req.ChatID,
			Text:     req.Text,
		}
		out, err := handler(r.Context(), in)
		if err != nil {
			http.Error(w, "handler error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		text := ""
		if out != nil {
			text = out.Text
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiReply{Reply: text})
	})

	srv := &http.Server{Addr: a.addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	a.mu.Lock()
	a.srv = srv
	a.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("api_server: %w", err)
	}
}

// SendReply is unused — the api_server replies inline via the HTTP
// response writer. It is implemented to satisfy the interface.
func (a *APIServer) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	return nil
}
```

- [ ] **Step 2:** Create `gateway/platforms/api_server_test.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestAPIServerRoundTrip(t *testing.T) {
	addr := freePort(t)
	srv := NewAPIServer(addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{UserID: in.UserID, Text: "echo: " + in.Text}, nil
		})
	}()

	// Wait up to 500ms for the server to accept.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"user_id":"u1","text":"ping"}`
	req, _ := http.NewRequest("POST", "http://"+addr+"/message", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out apiReply
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.Reply, "echo: ping") {
		t.Errorf("unexpected reply: %q", out.Reply)
	}

	cancel()
	<-errCh
}
```

- [ ] **Step 3:** `go test ./gateway/platforms/... -run TestAPIServer` — PASS.
- [ ] **Step 4:** Commit `feat(gateway/platforms): add api_server HTTP adapter`.

---

## Task 4: webhook platform

An outbound-only adapter: the Gateway POSTs replies to a configured URL.
Inbound messages for webhook are expected to arrive via the api_server
adapter. This task exercises the `SendReply` path separately.

- [ ] **Step 1:** Create `gateway/platforms/webhook.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// Webhook is an outbound-only adapter that POSTs gateway replies to a
// configured URL. It does not receive messages on its own; pair it
// with api_server or another inbound adapter.
type Webhook struct {
	url    string
	token  string
	client *http.Client
}

func NewWebhook(url, token string) *Webhook {
	return &Webhook{
		url:    url,
		token:  token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *Webhook) Name() string { return "webhook" }

// Run blocks until ctx is cancelled — webhook has no inbound loop.
func (w *Webhook) Run(ctx context.Context, handler gateway.MessageHandler) error {
	<-ctx.Done()
	return nil
}

type webhookPayload struct {
	UserID string `json:"user_id"`
	ChatID string `json:"chat_id,omitempty"`
	Text   string `json:"text"`
}

func (w *Webhook) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if w.url == "" {
		return errors.New("webhook: no URL configured")
	}
	buf, err := json.Marshal(webhookPayload{
		UserID: out.UserID, ChatID: out.ChatID, Text: out.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.token != "" {
		req.Header.Set("Authorization", "Bearer "+w.token)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] **Step 2:** Create `gateway/platforms/webhook_test.go`:

```go
package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestWebhookSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hi there") {
			t.Errorf("unexpected body: %s", body)
		}
		var p webhookPayload
		_ = json.Unmarshal(body, &p)
		if p.UserID != "u1" {
			t.Errorf("user id = %q", p.UserID)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wb := NewWebhook(srv.URL, "tok")
	err := wb.SendReply(context.Background(), gateway.OutgoingMessage{
		UserID: "u1", Text: "hi there",
	})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", hits)
	}
}

func TestWebhookSendReplyNoURL(t *testing.T) {
	wb := NewWebhook("", "")
	err := wb.SendReply(context.Background(), gateway.OutgoingMessage{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3:** `go test ./gateway/platforms/... -run TestWebhook` — PASS.
- [ ] **Step 4:** Commit `feat(gateway/platforms): add outbound webhook adapter`.

---

## Task 5: Telegram polling platform

Uses the Telegram Bot API via plain `net/http`. Long-polling (`getUpdates`)
is simpler than webhooks for development.

- [ ] **Step 1:** Create `gateway/platforms/telegram.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// Telegram is a Telegram Bot API adapter using long-polling. It calls
// getUpdates in a loop and delivers each message text to the gateway
// handler. Replies are sent via sendMessage.
type Telegram struct {
	token   string
	baseURL string
	client  *http.Client
	offset  int
}

func NewTelegram(token string) *Telegram {
	return &Telegram{
		token:   token,
		baseURL: "https://api.telegram.org",
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// withBaseURL allows tests to point at an httptest.Server.
func (t *Telegram) withBaseURL(u string) *Telegram { t.baseURL = u; return t }

func (t *Telegram) Name() string { return "telegram" }

type tgUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
}

type tgUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

func (t *Telegram) apiURL(method string) string {
	return t.baseURL + "/bot" + t.token + "/" + method
}

func (t *Telegram) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if t.token == "" {
		return fmt.Errorf("telegram: empty token")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		updates, err := t.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Back off briefly on errors.
			time.Sleep(2 * time.Second)
			continue
		}
		for _, u := range updates {
			t.offset = u.UpdateID + 1
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			in := gateway.IncomingMessage{
				Platform: t.Name(),
				UserID:   strconv.Itoa(u.Message.From.ID),
				ChatID:   strconv.Itoa(u.Message.Chat.ID),
				Text:     u.Message.Text,
			}
			gateway.DispatchAndReply(ctx, t, handler, in)
		}
	}
}

func (t *Telegram) getUpdates(ctx context.Context) ([]tgUpdate, error) {
	q := url.Values{}
	q.Set("timeout", "25")
	q.Set("offset", strconv.Itoa(t.offset))
	req, err := http.NewRequestWithContext(ctx, "GET", t.apiURL("getUpdates")+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("telegram: status %d: %s", resp.StatusCode, string(body))
	}
	var parsed tgUpdatesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.OK {
		return nil, fmt.Errorf("telegram: api returned ok=false")
	}
	return parsed.Result, nil
}

func (t *Telegram) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	payload := map[string]any{"chat_id": out.ChatID, "text": out.Text}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL("sendMessage"), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram: send status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] **Step 2:** Create `gateway/platforms/telegram_test.go`:

```go
package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestTelegramPollingAndReply(t *testing.T) {
	var getUpdatesHits, sendMessageHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "getUpdates"):
			hits := atomic.AddInt32(&getUpdatesHits, 1)
			if hits == 1 {
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":{"message_id":10,"from":{"id":42,"username":"alice"},"chat":{"id":99},"text":"hello bot"}}]}`))
			} else {
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
			}
		case strings.Contains(r.URL.Path, "sendMessage"):
			atomic.AddInt32(&sendMessageHits, 1)
			body, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(body, &m)
			if m["text"] != "echo: hello bot" {
				t.Errorf("unexpected reply text: %v", m["text"])
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tg := NewTelegram("bot-token").withBaseURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = tg.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{
				UserID: in.UserID,
				ChatID: in.ChatID,
				Text:   "echo: " + in.Text,
			}, nil
		})
		close(done)
	}()

	// Wait up to 800ms for both hits.
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&sendMessageHits) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if atomic.LoadInt32(&getUpdatesHits) == 0 {
		t.Error("getUpdates never called")
	}
	if atomic.LoadInt32(&sendMessageHits) != 1 {
		t.Errorf("sendMessage hits = %d", sendMessageHits)
	}
}
```

- [ ] **Step 3:** `go test ./gateway/platforms/... -run TestTelegram -timeout 10s` — PASS.
- [ ] **Step 4:** Commit `feat(gateway/platforms): add Telegram Bot API long-polling adapter`.

---

## Task 6: CLI `gateway` subcommand

- [ ] **Step 1:** Create `cli/gateway.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/gateway"
	"github.com/nousresearch/hermes-agent/gateway/platforms"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/factory"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/nousresearch/hermes-agent/tool/file"
	"github.com/nousresearch/hermes-agent/tool/terminal"
	"github.com/spf13/cobra"
)

var _ = agent.NewEngineWithToolsAndAux // keep linker happy if unused by this file

// newGatewayCmd creates the "hermes gateway" subcommand.
func newGatewayCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "gateway",
		Short: "Run the multi-platform gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGateway(cmd.Context(), app)
		},
	}
}

func runGateway(ctx context.Context, app *App) error {
	// Storage (optional, reused from REPL helper).
	if err := ensureStorage(app); err != nil {
		return err
	}

	// Build providers.
	primary, _, err := buildPrimaryProvider(app.Config)
	if err != nil {
		return err
	}
	var aux provider.Provider
	if app.Config.Auxiliary.APIKey != "" || app.Config.Auxiliary.Provider != "" {
		auxCfg := config.ProviderConfig{
			Provider: app.Config.Auxiliary.Provider,
			BaseURL:  app.Config.Auxiliary.BaseURL,
			APIKey:   app.Config.Auxiliary.APIKey,
			Model:    app.Config.Auxiliary.Model,
		}
		if auxCfg.Provider == "" {
			auxCfg.Provider = "anthropic"
		}
		if p, err := factory.New(auxCfg); err == nil {
			aux = p
		}
	}

	// Build the tool registry: file + terminal only for the gateway,
	// since many other tools would be surprising in a messaging
	// context. Later plans can expand this.
	reg := tool.NewRegistry()
	file.RegisterAll(reg)
	termBackend, err := terminal.New("local", terminal.Config{})
	if err == nil {
		terminal.RegisterShellExecute(reg, termBackend)
		defer termBackend.Close()
	}

	g := gateway.NewGateway(*app.Config, primary, aux, app.Storage, reg)

	// Register configured platforms.
	for name, pc := range app.Config.Gateway.Platforms {
		if !pc.Enabled {
			continue
		}
		plat, err := buildPlatform(name, pc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gateway: skipping %s: %v\n", name, err)
			continue
		}
		g.Register(plat)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	return g.Start(ctx)
}

// buildPlatform instantiates a platform adapter from its config entry.
func buildPlatform(name string, pc config.PlatformConfig) (gateway.Platform, error) {
	t := strings.ToLower(pc.Type)
	if t == "" {
		t = strings.ToLower(name)
	}
	switch t {
	case "api_server":
		return platforms.NewAPIServer(pc.Options["addr"]), nil
	case "webhook":
		return platforms.NewWebhook(pc.Options["url"], pc.Options["token"]), nil
	case "telegram":
		return platforms.NewTelegram(pc.Options["token"]), nil
	default:
		return nil, fmt.Errorf("unknown platform type %q", t)
	}
}
```

- [ ] **Step 2:** In `cli/root.go`, add `newGatewayCmd(app)` to the `AddCommand` call.

- [ ] **Step 3:** `go build ./... && go test ./...` — PASS.
- [ ] **Step 4:** Commit `feat(cli): add hermes gateway subcommand wired to Gateway.Start`.

---

## Verification Checklist

- [ ] `go test ./gateway/...` passes
- [ ] `hermes gateway` starts when at least one platform is enabled in config, exits with clear error otherwise
- [ ] `curl -X POST http://localhost:8080/message -d '{"user_id":"u1","text":"hi"}'` round-trips when api_server is enabled
