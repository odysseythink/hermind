# Phase 2: WebSocket (Discord Gateway + Mattermost Streaming) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Discord and Mattermost from outbound-only REST adapters to fully bidirectional real-time gateways using WebSocket connections.

**Architecture:** Add `nhooyr.io/websocket` as the single new dependency. Build a small shared `wsconn` helper for connect/read/reconnect, then use it in two new adapters (`discord_gateway.go`, `mattermost_ws.go`) that implement the existing `gateway.Platform` interface. The existing outbound-only adapters remain unchanged — the new adapters replace them when bidirectional messaging is desired.

**Tech Stack:** Go 1.25, `nhooyr.io/websocket`, stdlib `encoding/json`, `net/http`, `sync`, `log/slog`

---

## File Structure

```
hermes-agent-go/
├── gateway/
│   ├── platforms/
│   │   ├── wsconn.go              # Shared WebSocket connection helper
│   │   ├── wsconn_test.go         # Tests for wsconn
│   │   ├── discord_gateway.go     # Discord Gateway WebSocket adapter
│   │   ├── discord_gateway_test.go
│   │   ├── mattermost_ws.go       # Mattermost WebSocket adapter
│   │   ├── mattermost_ws_test.go
│   │   ├── discord_bot.go         # (existing, unchanged)
│   │   └── mattermost_bot.go      # (existing, unchanged)
```

**Why `wsconn` lives in `platforms/`:** It's only used by adapters in this package. No need for an `internal/` package — keep files that change together in the same package.

---

### Task 1: Add nhooyr.io/websocket dependency

**Files:**
- Modify: `hermes-agent-go/go.mod`

- [ ] **Step 1: Add the dependency**

```bash
cd hermes-agent-go && go get nhooyr.io/websocket@latest
```

- [ ] **Step 2: Tidy**

```bash
cd hermes-agent-go && go mod tidy
```

- [ ] **Step 3: Verify it appears in go.mod**

```bash
grep nhooyr hermes-agent-go/go.mod
```

Expected: A line like `nhooyr.io/websocket v1.x.x`

- [ ] **Step 4: Run existing tests to ensure nothing breaks**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All existing gateway tests pass.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/go.mod hermes-agent-go/go.sum
git commit -m "feat(deps): add nhooyr.io/websocket for gateway WebSocket support"
```

---

### Task 2: Build the shared wsconn helper

**Files:**
- Create: `hermes-agent-go/gateway/platforms/wsconn.go`
- Create: `hermes-agent-go/gateway/platforms/wsconn_test.go`

This helper encapsulates: connect → read JSON frames in a loop → write JSON frames (thread-safe) → reconnect with exponential backoff. Both Discord and Mattermost adapters will use it.

- [ ] **Step 1: Write the failing test for Conn read loop**

Create `hermes-agent-go/gateway/platforms/wsconn_test.go`:

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

	"nhooyr.io/websocket"
)

func TestWSConnReadLoop(t *testing.T) {
	// Server sends one JSON frame, then closes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.CloseNow()
		msg := map[string]string{"type": "hello"}
		buf, _ := json.Marshal(msg)
		_ = c.Write(r.Context(), websocket.MessageText, buf)
		c.Close(websocket.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	var received int32
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wsURL := "ws" + srv.URL[len("http"):]
	conn := NewWSConn(WSConnConfig{
		URL:            wsURL,
		OnMessage:      func(data []byte) { atomic.AddInt32(&received, 1) },
		ReconnectBase:  50 * time.Millisecond,
		ReconnectMax:   200 * time.Millisecond,
		ReconnectJitter: 0,
	})

	err := conn.Run(ctx)
	// Run returns nil or context-cancelled after server closes + reconnect fails.
	_ = err

	if atomic.LoadInt32(&received) < 1 {
		t.Errorf("received = %d, want >= 1", atomic.LoadInt32(&received))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestWSConnReadLoop -v
```

Expected: Compilation error — `NewWSConn`, `WSConnConfig`, etc. are undefined.

- [ ] **Step 3: Implement wsconn.go**

Create `hermes-agent-go/gateway/platforms/wsconn.go`:

```go
package platforms

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// WSConnConfig configures a managed WebSocket connection.
type WSConnConfig struct {
	// URL is the WebSocket endpoint (wss://...).
	URL string

	// Headers are sent on every connect/reconnect (e.g. auth).
	Headers map[string]string

	// OnMessage is called for every text frame received.
	// Called from a single goroutine — no need for internal locking.
	OnMessage func(data []byte)

	// OnConnect is called after each successful connection.
	// Use it to send auth/identify frames. Return error to abort.
	OnConnect func(conn *WSConn) error

	// ReconnectBase is the initial backoff duration (default 2s).
	ReconnectBase time.Duration
	// ReconnectMax is the maximum backoff duration (default 60s).
	ReconnectMax time.Duration
	// ReconnectJitter as a fraction 0.0–1.0 (default 0.2 = ±20%).
	ReconnectJitter float64
}

// WSConn is a managed WebSocket connection with automatic reconnection.
type WSConn struct {
	cfg  WSConnConfig
	mu   sync.Mutex
	conn *websocket.Conn
}

// NewWSConn creates a managed WebSocket connection.
func NewWSConn(cfg WSConnConfig) *WSConn {
	if cfg.ReconnectBase == 0 {
		cfg.ReconnectBase = 2 * time.Second
	}
	if cfg.ReconnectMax == 0 {
		cfg.ReconnectMax = 60 * time.Second
	}
	if cfg.ReconnectJitter == 0 {
		cfg.ReconnectJitter = 0.2
	}
	return &WSConn{cfg: cfg}
}

// WriteJSON marshals v as JSON and sends it as a text frame.
// Thread-safe.
func (w *WSConn) WriteJSON(ctx context.Context, v any) error {
	w.mu.Lock()
	c := w.conn
	w.mu.Unlock()
	if c == nil {
		return context.Canceled
	}
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, buf)
}

// Run connects and reads messages until ctx is cancelled.
// On disconnect it reconnects with exponential backoff.
func (w *WSConn) Run(ctx context.Context) error {
	backoff := w.cfg.ReconnectBase
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := w.connectAndRead(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("wsconn: disconnected, reconnecting",
			"url", w.cfg.URL, "err", err, "backoff", backoff)

		jittered := w.jitter(backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jittered):
		}
		backoff = min(backoff*2, w.cfg.ReconnectMax)
	}
}

func (w *WSConn) connectAndRead(ctx context.Context) error {
	opts := &websocket.DialOptions{}
	if len(w.cfg.Headers) > 0 {
		h := make(http.Header)
		for k, v := range w.cfg.Headers {
			h.Set(k, v)
		}
		opts.HTTPHeader = h
	}
	c, _, err := websocket.Dial(ctx, w.cfg.URL, opts)
	if err != nil {
		return err
	}
	// Allow large messages (Discord payloads can be big).
	c.SetReadLimit(1 << 20)

	w.mu.Lock()
	w.conn = c
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.conn = nil
		w.mu.Unlock()
		c.CloseNow()
	}()

	if w.cfg.OnConnect != nil {
		if err := w.cfg.OnConnect(w); err != nil {
			return err
		}
	}

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return err
		}
		if w.cfg.OnMessage != nil {
			w.cfg.OnMessage(data)
		}
	}
}

func (w *WSConn) jitter(d time.Duration) time.Duration {
	if w.cfg.ReconnectJitter <= 0 {
		return d
	}
	j := float64(d) * w.cfg.ReconnectJitter
	return d + time.Duration((rand.Float64()*2-1)*j)
}
```

Note: this file needs an import for `"encoding/json"` and `"net/http"` — add them to the import block:

```go
import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestWSConnReadLoop -v
```

Expected: PASS

- [ ] **Step 5: Write test for WriteJSON**

Add to `wsconn_test.go`:

```go
func TestWSConnWriteJSON(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		// Read one frame the client sends.
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var msg map[string]string
		_ = json.Unmarshal(data, &msg)
		if msg["type"] == "ping" {
			atomic.AddInt32(&received, 1)
		}
		c.Close(websocket.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wsURL := "ws" + srv.URL[len("http"):]
	connected := make(chan struct{})
	conn := NewWSConn(WSConnConfig{
		URL: wsURL,
		OnConnect: func(c *WSConn) error {
			close(connected)
			return nil
		},
		OnMessage:      func(data []byte) {},
		ReconnectBase:  50 * time.Millisecond,
		ReconnectMax:   200 * time.Millisecond,
		ReconnectJitter: 0,
	})

	go conn.Run(ctx)

	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("never connected")
	}

	err := conn.WriteJSON(ctx, map[string]string{"type": "ping"})
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Wait for server to process.
	time.Sleep(200 * time.Millisecond)
	cancel()

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("server received = %d, want 1", atomic.LoadInt32(&received))
	}
}
```

- [ ] **Step 6: Run test**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestWSConn -v
```

Expected: Both TestWSConnReadLoop and TestWSConnWriteJSON pass.

- [ ] **Step 7: Run all gateway tests**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add hermes-agent-go/gateway/platforms/wsconn.go hermes-agent-go/gateway/platforms/wsconn_test.go
git commit -m "feat(gateway): add shared WebSocket connection helper with reconnect"
```

---

### Task 3: Discord Gateway WebSocket adapter

**Files:**
- Create: `hermes-agent-go/gateway/platforms/discord_gateway.go`
- Create: `hermes-agent-go/gateway/platforms/discord_gateway_test.go`

The Discord Gateway protocol: connect to `wss://gateway.discord.gg/?v=10&encoding=json`, receive Hello (op 10) with heartbeat_interval, send Identify (op 2) with bot token, then receive Dispatch events (op 0) with `t: "MESSAGE_CREATE"`. Must send Heartbeat (op 1) at the requested interval and handle Resume on reconnect.

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/gateway/platforms/discord_gateway_test.go`:

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

	"nhooyr.io/websocket"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestDiscordGatewayReceivesMessage(t *testing.T) {
	// Fake Discord Gateway server.
	var identifyReceived int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.CloseNow()

		// Send Hello (op 10) with 30s heartbeat (won't fire in test).
		hello := map[string]any{
			"op": 10,
			"d":  map[string]any{"heartbeat_interval": 30000},
		}
		buf, _ := json.Marshal(hello)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		// Read Identify (op 2) from client.
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var ident map[string]any
		_ = json.Unmarshal(data, &ident)
		if ident["op"].(float64) == 2 {
			atomic.AddInt32(&identifyReceived, 1)
		}

		// Send a MESSAGE_CREATE dispatch (op 0).
		dispatch := map[string]any{
			"op": 0,
			"t":  "MESSAGE_CREATE",
			"s":  1,
			"d": map[string]any{
				"id":         "msg123",
				"channel_id": "chan456",
				"content":    "hello from discord",
				"author": map[string]any{
					"id":       "user789",
					"username": "alice",
				},
			},
		}
		buf, _ = json.Marshal(dispatch)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		// Keep connection open until context done.
		<-r.Context().Done()
	}))
	defer srv.Close()

	// Outbound reply mock — Discord REST API for SendReply.
	var replyHits int32
	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&replyHits, 1)
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	dg := NewDiscordGateway("bot-token", "").
		WithGatewayURL(wsURL).
		WithBaseURL(restSrv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = dg.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			if in.Text != "hello from discord" {
				t.Errorf("unexpected text: %s", in.Text)
			}
			if in.Platform != "discord" {
				t.Errorf("unexpected platform: %s", in.Platform)
			}
			if in.UserID != "user789" {
				t.Errorf("unexpected user: %s", in.UserID)
			}
			if in.ChatID != "chan456" {
				t.Errorf("unexpected chat: %s", in.ChatID)
			}
			return &gateway.OutgoingMessage{
				ChatID: in.ChatID,
				Text:   "echo: " + in.Text,
			}, nil
		})
		close(done)
	}()

	// Wait for reply to be sent.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&replyHits) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if atomic.LoadInt32(&identifyReceived) != 1 {
		t.Error("identify never received")
	}
	if atomic.LoadInt32(&replyHits) < 1 {
		t.Errorf("reply hits = %d, want >= 1", atomic.LoadInt32(&replyHits))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestDiscordGateway -v
```

Expected: Compilation error — `NewDiscordGateway` is undefined.

- [ ] **Step 3: Implement discord_gateway.go**

Create `hermes-agent-go/gateway/platforms/discord_gateway.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// DiscordGateway is a bidirectional Discord adapter using the Gateway
// WebSocket for inbound messages and the REST API for outbound replies.
type DiscordGateway struct {
	token      string
	channelID  string // default channel for SendReply
	gatewayURL string // wss://gateway.discord.gg/?v=10&encoding=json
	baseURL    string // REST API base, default https://discord.com/api/v10
	client     *http.Client
	ws         *WSConn // set during Run

	// Resume state.
	mu        sync.Mutex
	sessionID string
	seq       int
}

func NewDiscordGateway(token, channelID string) *DiscordGateway {
	return &DiscordGateway{
		token:      token,
		channelID:  channelID,
		gatewayURL: "wss://gateway.discord.gg/?v=10&encoding=json",
		baseURL:    "https://discord.com/api/v10",
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DiscordGateway) WithGatewayURL(u string) *DiscordGateway {
	d.gatewayURL = u
	return d
}

func (d *DiscordGateway) WithBaseURL(u string) *DiscordGateway {
	d.baseURL = strings.TrimRight(u, "/")
	return d
}

func (d *DiscordGateway) Name() string { return "discord" }

// discordPayload is the generic Gateway payload envelope.
type discordPayload struct {
	Op   int             `json:"op"`
	D    json.RawMessage `json:"d,omitempty"`
	S    *int            `json:"s,omitempty"`
	T    string          `json:"t,omitempty"`
}

// discordHello is op 10 data.
type discordHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"` // milliseconds
}

// discordMessageCreate is the MESSAGE_CREATE dispatch data (subset).
type discordMessageCreate struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

// discordReady is the READY dispatch data (subset).
type discordReady struct {
	SessionID string `json:"session_id"`
}

func (d *DiscordGateway) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if d.token == "" {
		return fmt.Errorf("discord: empty token")
	}

	d.ws = NewWSConn(WSConnConfig{
		URL: d.gatewayURL,
		OnMessage: func(data []byte) {
			d.handlePayload(ctx, data, handler)
		},
		ReconnectBase:   2 * time.Second,
		ReconnectMax:    60 * time.Second,
		ReconnectJitter: 0.2,
	})

	return d.ws.Run(ctx)
}

func (d *DiscordGateway) handlePayload(ctx context.Context, data []byte, handler gateway.MessageHandler) {
	var p discordPayload
	if err := json.Unmarshal(data, &p); err != nil {
		slog.Warn("discord: bad payload", "err", err)
		return
	}

	// Track sequence number for resume.
	if p.S != nil {
		d.mu.Lock()
		d.seq = *p.S
		d.mu.Unlock()
	}

	switch p.Op {
	case 10: // Hello — start heartbeat + identify.
		var hello discordHello
		_ = json.Unmarshal(p.D, &hello)
		d.handleHello(ctx, hello)

	case 0: // Dispatch.
		d.handleDispatch(ctx, p.T, p.D, handler)

	case 11: // Heartbeat ACK — no action needed.

	case 1: // Server requests heartbeat.
		d.sendHeartbeat(ctx)

	case 7: // Reconnect — wsconn will reconnect on read error.
		slog.Info("discord: server requested reconnect")

	case 9: // Invalid session — clear session for fresh identify.
		d.mu.Lock()
		d.sessionID = ""
		d.seq = 0
		d.mu.Unlock()
	}
}

func (d *DiscordGateway) handleHello(ctx context.Context, hello discordHello) {
	// Start heartbeat goroutine.
	interval := time.Duration(hello.HeartbeatInterval) * time.Millisecond
	go d.heartbeatLoop(ctx, interval)

	// Send Identify or Resume.
	d.mu.Lock()
	sid := d.sessionID
	seq := d.seq
	d.mu.Unlock()

	if sid != "" {
		// Resume.
		d.sendResume(ctx, sid, seq)
	} else {
		d.sendIdentify(ctx)
	}
}

func (d *DiscordGateway) sendIdentify(ctx context.Context) {
	identify := map[string]any{
		"op": 2,
		"d": map[string]any{
			"token":   d.token,
			"intents": 512 | 32768, // GUILD_MESSAGES | MESSAGE_CONTENT
			"properties": map[string]string{
				"os":      "linux",
				"browser": "hermes-agent",
				"device":  "hermes-agent",
			},
		},
	}
	d.writeJSON(ctx, identify)
}

func (d *DiscordGateway) sendResume(ctx context.Context, sessionID string, seq int) {
	resume := map[string]any{
		"op": 6,
		"d": map[string]any{
			"token":      d.token,
			"session_id": sessionID,
			"seq":        seq,
		},
	}
	d.writeJSON(ctx, resume)
}

func (d *DiscordGateway) sendHeartbeat(ctx context.Context) {
	d.mu.Lock()
	seq := d.seq
	d.mu.Unlock()
	hb := map[string]any{"op": 1, "d": seq}
	d.writeJSON(ctx, hb)
}

func (d *DiscordGateway) heartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sendHeartbeat(ctx)
		}
	}
}

func (d *DiscordGateway) writeJSON(ctx context.Context, v any) {
	if d.ws == nil {
		return
	}
	if err := d.ws.WriteJSON(ctx, v); err != nil {
		slog.Warn("discord: write error", "err", err)
	}
}

func (d *DiscordGateway) handleDispatch(ctx context.Context, eventType string, data json.RawMessage, handler gateway.MessageHandler) {
	switch eventType {
	case "READY":
		var ready discordReady
		_ = json.Unmarshal(data, &ready)
		d.mu.Lock()
		d.sessionID = ready.SessionID
		d.mu.Unlock()
		slog.Info("discord: ready", "session_id", ready.SessionID)

	case "MESSAGE_CREATE":
		var msg discordMessageCreate
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("discord: bad MESSAGE_CREATE", "err", err)
			return
		}
		// Ignore bot messages.
		if msg.Author.Bot {
			return
		}
		if msg.Content == "" {
			return
		}
		in := gateway.IncomingMessage{
			Platform:  d.Name(),
			UserID:    msg.Author.ID,
			ChatID:    msg.ChannelID,
			Text:      msg.Content,
			MessageID: msg.ID,
		}
		gateway.DispatchAndReply(ctx, d, handler, in)
	}
}

func (d *DiscordGateway) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if d.token == "" {
		return fmt.Errorf("discord: token required")
	}
	channel := d.channelID
	if out.ChatID != "" {
		channel = out.ChatID
	}
	if channel == "" {
		return fmt.Errorf("discord: channel id required")
	}
	url := fmt.Sprintf("%s/channels/%s/messages", d.baseURL, channel)
	buf, _ := json.Marshal(map[string]any{"content": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+d.token)
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestDiscordGateway -v
```

Expected: PASS

- [ ] **Step 5: Write test for SendReply**

Add to `discord_gateway_test.go`:

```go
func TestDiscordGatewaySendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bot test-tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if m["content"] != "hello" {
			t.Errorf("content = %v", m["content"])
		}
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer srv.Close()

	dg := NewDiscordGateway("test-tok", "C1").WithBaseURL(srv.URL)
	err := dg.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hello"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", atomic.LoadInt32(&hits))
	}
}
```

- [ ] **Step 6: Run all Discord tests**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestDiscordGateway -v
```

Expected: Both tests pass.

- [ ] **Step 7: Run all gateway tests**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass (including existing tests).

- [ ] **Step 8: Commit**

```bash
git add hermes-agent-go/gateway/platforms/discord_gateway.go hermes-agent-go/gateway/platforms/discord_gateway_test.go
git commit -m "feat(gateway): add Discord Gateway WebSocket adapter"
```

---

### Task 4: Mattermost WebSocket adapter

**Files:**
- Create: `hermes-agent-go/gateway/platforms/mattermost_ws.go`
- Create: `hermes-agent-go/gateway/platforms/mattermost_ws_test.go`

The Mattermost WebSocket protocol: connect to `{base}/api/v4/websocket`, send `authentication_challenge` response with token, then receive `posted` events. Simpler than Discord — no heartbeat, no resume, just auth + read events.

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/gateway/platforms/mattermost_ws_test.go`:

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

	"nhooyr.io/websocket"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestMattermostWSReceivesMessage(t *testing.T) {
	var authReceived int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.CloseNow()

		// Send authentication challenge.
		challenge := map[string]any{
			"event":    "authentication_challenge",
			"status":   "OK",
		}
		buf, _ := json.Marshal(challenge)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		// Read auth response.
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var auth map[string]any
		_ = json.Unmarshal(data, &auth)
		if auth["action"] == "authentication_challenge" {
			atomic.AddInt32(&authReceived, 1)
		}

		// Send a posted event.
		post := map[string]any{
			"channel_id":   "chan1",
			"id":           "post1",
			"message":      "hello from mattermost",
			"user_id":      "user1",
		}
		postJSON, _ := json.Marshal(post)
		event := map[string]any{
			"event": "posted",
			"data":  map[string]any{"post": string(postJSON)},
		}
		buf, _ = json.Marshal(event)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		<-r.Context().Done()
	}))
	defer srv.Close()

	// REST API for SendReply.
	var replyHits int32
	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&replyHits, 1)
		_, _ = w.Write([]byte(`{"id":"post2"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	mm := NewMattermostWS(restSrv.URL, "mm-token", "").
		WithWebSocketURL(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = mm.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			if in.Text != "hello from mattermost" {
				t.Errorf("unexpected text: %s", in.Text)
			}
			if in.Platform != "mattermost" {
				t.Errorf("unexpected platform: %s", in.Platform)
			}
			if in.UserID != "user1" {
				t.Errorf("unexpected user: %s", in.UserID)
			}
			return &gateway.OutgoingMessage{
				ChatID: in.ChatID,
				Text:   "echo: " + in.Text,
			}, nil
		})
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&replyHits) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if atomic.LoadInt32(&authReceived) != 1 {
		t.Error("auth never received")
	}
	if atomic.LoadInt32(&replyHits) < 1 {
		t.Errorf("reply hits = %d, want >= 1", atomic.LoadInt32(&replyHits))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestMattermostWS -v
```

Expected: Compilation error — `NewMattermostWS` is undefined.

- [ ] **Step 3: Implement mattermost_ws.go**

Create `hermes-agent-go/gateway/platforms/mattermost_ws.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// MattermostWS is a bidirectional Mattermost adapter using WebSocket
// for inbound events and the REST API for outbound replies.
type MattermostWS struct {
	baseURL string // REST API base, e.g. https://mm.example.com
	token   string
	channel string // default channel
	wsURL   string // computed from baseURL if not set
	client  *http.Client
	ws      *WSConn
}

func NewMattermostWS(baseURL, token, channelID string) *MattermostWS {
	base := strings.TrimRight(baseURL, "/")
	// Compute WebSocket URL from base: https → wss, http → ws.
	wsBase := strings.Replace(base, "https://", "wss://", 1)
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)
	return &MattermostWS{
		baseURL: base,
		token:   token,
		channel: channelID,
		wsURL:   wsBase + "/api/v4/websocket",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MattermostWS) WithWebSocketURL(u string) *MattermostWS {
	m.wsURL = u
	return m
}

func (m *MattermostWS) Name() string { return "mattermost" }

// mmEvent is the Mattermost WebSocket event envelope.
type mmEvent struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data,omitempty"`
}

// mmPost is the nested post JSON within a "posted" event.
type mmPost struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	UserID    string `json:"user_id"`
}

func (m *MattermostWS) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if m.token == "" {
		return fmt.Errorf("mattermost: empty token")
	}

	m.ws = NewWSConn(WSConnConfig{
		URL: m.wsURL,
		OnConnect: func(c *WSConn) error {
			// Server sends authentication_challenge; we respond.
			return nil
		},
		OnMessage: func(data []byte) {
			m.handleEvent(ctx, data, handler)
		},
		ReconnectBase:   2 * time.Second,
		ReconnectMax:    60 * time.Second,
		ReconnectJitter: 0.2,
	})

	return m.ws.Run(ctx)
}

func (m *MattermostWS) handleEvent(ctx context.Context, data []byte, handler gateway.MessageHandler) {
	var ev mmEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Warn("mattermost: bad event", "err", err)
		return
	}

	switch ev.Event {
	case "authentication_challenge":
		m.sendAuth(ctx)

	case "posted":
		m.handlePosted(ctx, ev.Data, handler)
	}
}

func (m *MattermostWS) sendAuth(ctx context.Context) {
	auth := map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data":   map[string]string{"token": m.token},
	}
	if m.ws != nil {
		if err := m.ws.WriteJSON(ctx, auth); err != nil {
			slog.Warn("mattermost: auth write error", "err", err)
		}
	}
}

func (m *MattermostWS) handlePosted(ctx context.Context, data map[string]any, handler gateway.MessageHandler) {
	postStr, ok := data["post"].(string)
	if !ok {
		return
	}
	var post mmPost
	if err := json.Unmarshal([]byte(postStr), &post); err != nil {
		slog.Warn("mattermost: bad post JSON", "err", err)
		return
	}
	if post.Message == "" {
		return
	}
	in := gateway.IncomingMessage{
		Platform:  m.Name(),
		UserID:    post.UserID,
		ChatID:    post.ChannelID,
		Text:      post.Message,
		MessageID: post.ID,
	}
	gateway.DispatchAndReply(ctx, m, handler, in)
}

func (m *MattermostWS) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if m.baseURL == "" || m.token == "" {
		return fmt.Errorf("mattermost: base_url/token required")
	}
	channel := m.channel
	if out.ChatID != "" {
		channel = out.ChatID
	}
	if channel == "" {
		return fmt.Errorf("mattermost: channel id required")
	}
	url := m.baseURL + "/api/v4/posts"
	buf, _ := json.Marshal(map[string]any{"channel_id": channel, "message": out.Text})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.token)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("mattermost: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mattermost: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestMattermostWS -v
```

Expected: PASS

- [ ] **Step 5: Write test for SendReply**

Add to `mattermost_ws_test.go`:

```go
func TestMattermostWSSendReply(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer mm-tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if m["channel_id"] != "CH1" || m["message"] != "hi" {
			t.Errorf("body = %+v", m)
		}
		_, _ = w.Write([]byte(`{"id":"post1"}`))
	}))
	defer srv.Close()

	mm := NewMattermostWS(srv.URL, "mm-tok", "CH1")
	err := mm.SendReply(context.Background(), gateway.OutgoingMessage{Text: "hi"})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("hits = %d", atomic.LoadInt32(&hits))
	}
}
```

- [ ] **Step 6: Run all Mattermost tests**

```bash
cd hermes-agent-go && go test ./gateway/platforms/ -run TestMattermost -v
```

Expected: All Mattermost tests pass (including existing `TestMattermostBotSendReply`).

- [ ] **Step 7: Run all gateway tests**

```bash
cd hermes-agent-go && go test ./gateway/...
```

Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add hermes-agent-go/gateway/platforms/mattermost_ws.go hermes-agent-go/gateway/platforms/mattermost_ws_test.go
git commit -m "feat(gateway): add Mattermost WebSocket adapter"
```

---

### Task 5: Integration test — full round trip

**Files:**
- Modify: `hermes-agent-go/gateway/platforms/discord_gateway_test.go`
- Modify: `hermes-agent-go/gateway/platforms/mattermost_ws_test.go`

Add tests that verify the bot-ignore behavior for Discord and empty-message filtering for both.

- [ ] **Step 1: Add Discord bot-message-ignore test**

Add to `discord_gateway_test.go`:

```go
func TestDiscordGatewayIgnoresBotMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()

		// Hello.
		hello := map[string]any{"op": 10, "d": map[string]any{"heartbeat_interval": 30000}}
		buf, _ := json.Marshal(hello)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		// Read Identify.
		_, _, _ = c.Read(r.Context())

		// Send a bot message (should be ignored).
		dispatch := map[string]any{
			"op": 0, "t": "MESSAGE_CREATE", "s": 1,
			"d": map[string]any{
				"id": "msg1", "channel_id": "ch1", "content": "bot says hi",
				"author": map[string]any{"id": "bot1", "username": "mybot", "bot": true},
			},
		}
		buf, _ = json.Marshal(dispatch)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		<-r.Context().Done()
	}))
	defer srv.Close()

	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("SendReply should not be called for bot messages")
		_, _ = w.Write([]byte(`{"id":"msg1"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	dg := NewDiscordGateway("tok", "").WithGatewayURL(wsURL).WithBaseURL(restSrv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_ = dg.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		t.Error("handler should not be called for bot messages")
		return nil, nil
	})
}
```

- [ ] **Step 2: Add Mattermost empty-message-ignore test**

Add to `mattermost_ws_test.go`:

```go
func TestMattermostWSIgnoresEmptyMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()

		// Auth challenge.
		challenge := map[string]any{"event": "authentication_challenge", "status": "OK"}
		buf, _ := json.Marshal(challenge)
		_ = c.Write(r.Context(), websocket.MessageText, buf)
		// Read auth response.
		_, _, _ = c.Read(r.Context())

		// Send an empty post.
		post := map[string]any{"id": "p1", "channel_id": "ch1", "message": "", "user_id": "u1"}
		postJSON, _ := json.Marshal(post)
		event := map[string]any{"event": "posted", "data": map[string]any{"post": string(postJSON)}}
		buf, _ = json.Marshal(event)
		_ = c.Write(r.Context(), websocket.MessageText, buf)

		<-r.Context().Done()
	}))
	defer srv.Close()

	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("SendReply should not be called for empty messages")
		_, _ = w.Write([]byte(`{"id":"p1"}`))
	}))
	defer restSrv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	mm := NewMattermostWS(restSrv.URL, "tok", "").WithWebSocketURL(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_ = mm.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		t.Error("handler should not be called for empty messages")
		return nil, nil
	})
}
```

- [ ] **Step 3: Run all tests**

```bash
cd hermes-agent-go && go test ./gateway/... -v
```

Expected: All pass, including the new bot-ignore and empty-message tests.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/gateway/platforms/discord_gateway_test.go hermes-agent-go/gateway/platforms/mattermost_ws_test.go
git commit -m "test(gateway): add edge case tests for Discord bot-ignore and Mattermost empty-message"
```
