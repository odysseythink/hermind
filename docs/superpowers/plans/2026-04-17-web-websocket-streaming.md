# Web WebSocket Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Depends on:** Plan E (Web REST API) — the `api/` package with `Server`, `NewAuthMiddleware`, and `ServerOpts` must already exist.

**Goal:** Add a `/api/chat/stream` WebSocket endpoint so the browser can push a prompt and receive live streaming deltas (content chunks, tool_call frames, done, error) while hermind runs the agent. Also add a lightweight `/api/events` Server-Sent-Events endpoint for observability dashboards that don't want a bidirectional channel. Token auth piggybacks on the Bearer-token mechanism from Plan E; we accept the token via query parameter because browsers can't set headers on a WebSocket handshake.

**Architecture:** A new `api/ws.go` houses the WebSocket upgrade path and the per-connection writer loop. `api/stream.go` holds the JSON envelope types the client parses. A `chatRunner` owns the agent-engine invocation, calls `provider.Stream()`, and forwards each `StreamEvent` as a JSON frame. Writes to the socket go through a per-connection channel to keep writes serialized. On cancel / disconnect the context is cancelled and the stream closed cleanly. The SSE endpoint re-uses the same envelope so the frontend code is shape-compatible.

**Tech Stack:** Go 1.21+, `github.com/coder/websocket` (Context-aware, CSRF-safe Accept options), existing `api/`, `agent`, `provider`, `provider/factory` packages. No new chi dependency — the chi router from Plan E is already present.

---

## File Structure

- Modify: `go.mod` — add `github.com/coder/websocket`
- Create: `api/stream.go` — JSON envelope types (`StreamFrame` + constants)
- Create: `api/stream_test.go`
- Create: `api/ws.go` — `/api/chat/stream` handler + writer loop
- Create: `api/ws_test.go`
- Create: `api/sse.go` — `/api/events` Server-Sent-Events handler
- Create: `api/sse_test.go`
- Modify: `api/server.go` — register the two new routes; accept `?t=<token>` on the upgrade path
- Modify: `api/auth.go` — teach the middleware to fall back to `?t=<token>` for the streaming paths only

---

## Task 1: Add coder/websocket dependency

- [ ] **Step 1: Add it**

Run: `go get github.com/coder/websocket@latest`

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add coder/websocket for streaming api"
```

---

## Task 2: StreamFrame envelope

**Files:**
- Create: `api/stream.go`
- Create: `api/stream_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/stream_test.go`:

```go
package api

import (
	"encoding/json"
	"testing"
)

func TestStreamFrame_DeltaMarshal(t *testing.T) {
	f := StreamFrame{Type: FrameDelta, Data: map[string]string{"content": "hi"}}
	data, _ := json.Marshal(f)
	want := `{"type":"delta","data":{"content":"hi"}}`
	if string(data) != want {
		t.Errorf("got %s\nwant %s", data, want)
	}
}

func TestStreamFrame_ErrorMarshal(t *testing.T) {
	f := StreamFrame{Type: FrameError, Error: &StreamError{Code: "provider", Message: "boom"}}
	data, _ := json.Marshal(f)
	if !contains(string(data), `"code":"provider"`) {
		t.Errorf("missing code: %s", data)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestStreamFrame -v`
Expected: FAIL — `StreamFrame` undefined.

- [ ] **Step 3: Implement StreamFrame**

Create `api/stream.go`:

```go
package api

// FrameType discriminates streaming envelope payloads.
type FrameType string

const (
	FrameDelta    FrameType = "delta"     // incremental content chunk
	FrameToolCall FrameType = "tool_call" // model invoked a tool
	FrameToolResult FrameType = "tool_result" // tool finished executing
	FrameDone     FrameType = "done"      // stream finished normally
	FrameError    FrameType = "error"     // stream aborted
	FrameUsage    FrameType = "usage"     // token accounting
	FramePing     FrameType = "ping"      // keepalive
)

// StreamFrame is the JSON envelope the client parses. Exactly one
// of Data or Error is populated.
type StreamFrame struct {
	Type  FrameType   `json:"type"`
	Data  interface{} `json:"data,omitempty"`
	Error *StreamError `json:"error,omitempty"`
}

// StreamError is the error payload inside a FrameError frame.
type StreamError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChatRequest is the client → server message that kicks off a stream.
type ChatRequest struct {
	SessionID string `json:"session_id,omitempty"` // empty → new anonymous session
	Prompt    string `json:"prompt"`
	Model     string `json:"model,omitempty"` // override server default
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run TestStreamFrame -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/stream.go api/stream_test.go
git commit -m "feat(api): streaming frame envelope types"
```

---

## Task 3: Extend auth middleware for query-token fallback

**Files:**
- Modify: `api/auth.go`
- Modify: `api/auth_test.go`

- [ ] **Step 1: Write the failing test**

Append to `api/auth_test.go`:

```go
func TestAuthMiddleware_AcceptsQueryTokenForUpgradePaths(t *testing.T) {
	mw := NewAuthMiddlewareWithQueryPaths("secret", nil, []string{"/api/chat/stream"})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	req := httptest.NewRequest("GET", "/api/chat/stream?t=secret", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Errorf("code = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_RejectsQueryTokenForRESTPaths(t *testing.T) {
	mw := NewAuthMiddlewareWithQueryPaths("secret", nil, []string{"/api/chat/stream"})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not run") })

	// /api/sessions is a REST path — query token must NOT be accepted.
	req := httptest.NewRequest("GET", "/api/sessions?t=secret", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("code = %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestAuthMiddleware_AcceptsQuery -v`
Expected: FAIL — `NewAuthMiddlewareWithQueryPaths` undefined.

- [ ] **Step 3: Extend auth.go**

Append to `api/auth.go`:

```go
// NewAuthMiddlewareWithQueryPaths extends NewAuthMiddleware to accept
// "?t=<token>" on the given queryPaths (typically WebSocket/SSE
// endpoints where browsers cannot set the Authorization header).
// Other paths continue to require a Bearer header.
func NewAuthMiddlewareWithQueryPaths(token string, publicPaths, queryPaths []string) func(http.Handler) http.Handler {
	publicSet := make(map[string]struct{}, len(publicPaths))
	for _, p := range publicPaths {
		publicSet[p] = struct{}{}
	}
	querySet := make(map[string]struct{}, len(queryPaths))
	for _, p := range queryPaths {
		querySet[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := publicSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := querySet[r.URL.Path]; ok {
				q := r.URL.Query().Get("t")
				if q != "" && subtle.ConstantTimeCompare([]byte(q), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			got := auth[len(prefix):]
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Update server.go to use it**

In `api/server.go`, inside `buildRouter`, replace the existing auth middleware construction:

```go
	public := []string{"/api/status", "/api/model/info"}
	queryAuth := []string{"/api/chat/stream", "/api/events"}
	auth := NewAuthMiddlewareWithQueryPaths(s.opts.Token, public, queryAuth)
```

(The `NewAuthMiddleware` function remains as a backward-compatible alias — callers that don't need query auth can keep using it.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./api/ -run TestAuth -v`
Expected: PASS — all existing tests + new query-token tests.

- [ ] **Step 6: Commit**

```bash
git add api/auth.go api/auth_test.go api/server.go
git commit -m "feat(api): accept ?t=token for websocket/sse endpoints"
```

---

## Task 4: WebSocket handler

**Files:**
- Create: `api/ws.go`
- Create: `api/ws_test.go`
- Modify: `api/server.go` — register the route

- [ ] **Step 1: Write the failing test**

Create `api/ws_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWS_StreamsDeltaAndDone(t *testing.T) {
	s, _ := newTestServerWithStore(t)

	// Wire a stub agent runner so the test doesn't need a real provider.
	s.ChatRunner = func(ctx context.Context, req ChatRequest, emit func(StreamFrame)) error {
		emit(StreamFrame{Type: FrameDelta, Data: map[string]string{"content": "he"}})
		emit(StreamFrame{Type: FrameDelta, Data: map[string]string{"content": "llo"}})
		emit(StreamFrame{Type: FrameDone, Data: map[string]string{"finish_reason": "end_turn"}})
		return nil
	}

	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/stream?t=t"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	req := ChatRequest{Prompt: "hi"}
	body, _ := json.Marshal(req)
	if err := conn.Write(ctx, websocket.MessageText, body); err != nil {
		t.Fatalf("write: %v", err)
	}

	var got []StreamFrame
	for i := 0; i < 3; i++ {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read[%d]: %v", i, err)
		}
		var f StreamFrame
		_ = json.Unmarshal(data, &f)
		got = append(got, f)
	}
	if got[0].Type != FrameDelta || got[1].Type != FrameDelta {
		t.Errorf("first two frames not delta: %+v", got)
	}
	if got[2].Type != FrameDone {
		t.Errorf("final frame not done: %+v", got[2])
	}
}

func TestWS_RejectsMissingToken(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/stream"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != 401 {
		t.Errorf("expected 401, got %v", resp)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestWS -v`
Expected: FAIL — `Server.ChatRunner` undefined, `/api/chat/stream` not registered.

- [ ] **Step 3: Implement the handler**

Create `api/ws.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// ChatRunner runs a chat turn and emits frames through emit. It must
// return when the stream is complete or ctx is cancelled. Errors
// returned by ChatRunner are turned into a FrameError before the
// connection is closed.
type ChatRunner func(ctx context.Context, req ChatRequest, emit func(StreamFrame)) error

// handleChatStream serves /api/chat/stream. See the package-level
// description for protocol details.
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// localhost-only server, but be explicit.
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "bye")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read the single ChatRequest frame.
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	_, payload, err := conn.Read(readCtx)
	readCancel()
	if err != nil {
		return
	}
	var req ChatRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		_ = writeFrame(ctx, conn, StreamFrame{
			Type:  FrameError,
			Error: &StreamError{Code: "bad_request", Message: err.Error()},
		})
		conn.Close(websocket.StatusUnsupportedData, "bad request")
		return
	}

	runner := s.ChatRunner
	if runner == nil {
		runner = s.defaultChatRunner
	}

	emit := func(f StreamFrame) {
		_ = writeFrame(ctx, conn, f)
	}
	if err := runner(ctx, req, emit); err != nil {
		_ = writeFrame(ctx, conn, StreamFrame{
			Type:  FrameError,
			Error: &StreamError{Code: "runner", Message: err.Error()},
		})
		conn.Close(websocket.StatusInternalError, "runner error")
		return
	}
	conn.Close(websocket.StatusNormalClosure, "")
}

func writeFrame(ctx context.Context, conn *websocket.Conn, f StreamFrame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, data)
}
```

Now extend `Server` in `api/server.go` with the new field + default runner:

```go
type Server struct {
	opts     *ServerOpts
	router   chi.Router
	bootedAt time.Time

	// ChatRunner is the streaming hook. Tests override this with a
	// stub that emits canned frames. Production wiring in cli/web.go
	// installs a runner that constructs an agent.Engine per request.
	ChatRunner ChatRunner
}
```

Add a no-op default:

```go
func (s *Server) defaultChatRunner(ctx context.Context, req ChatRequest, emit func(StreamFrame)) error {
	emit(StreamFrame{
		Type:  FrameError,
		Error: &StreamError{Code: "not_configured", Message: "hermind web: ChatRunner not configured"},
	})
	return nil
}
```

And register the route inside `buildRouter`:

```go
	r.Route("/api", func(r chi.Router) {
		// ... existing routes ...
		r.Get("/chat/stream", s.handleChatStream)
	})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run TestWS -v -race`
Expected: PASS (both sub-tests, no races).

- [ ] **Step 5: Commit**

```bash
git add api/ws.go api/ws_test.go api/server.go
git commit -m "feat(api): /api/chat/stream websocket endpoint"
```

---

## Task 5: Default ChatRunner wired to provider.Stream

**Files:**
- Modify: `api/ws.go` — replace `defaultChatRunner` with one that actually calls the provider
- Modify: `api/server.go` — accept a `ProviderFactory` opt
- Modify: `cli/web.go` — pass in the factory

- [ ] **Step 1: Write the failing test**

Append to `api/ws_test.go`:

```go
func TestDefaultChatRunner_UsesStubProvider(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	// Install a stub ProviderFactory that returns a canned stream.
	s.ProviderFactory = func(model string) (provider.Provider, error) {
		return &fakeStreamProvider{chunks: []string{"a", "b", "c"}}, nil
	}
	// Leave ChatRunner nil so the default path runs.

	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/stream?t=t"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	body, _ := json.Marshal(ChatRequest{Prompt: "hi", Model: "stub/model"})
	_ = conn.Write(ctx, websocket.MessageText, body)

	gotContent := ""
	sawDone := false
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		var f StreamFrame
		_ = json.Unmarshal(data, &f)
		switch f.Type {
		case FrameDelta:
			if m, ok := f.Data.(map[string]interface{}); ok {
				if c, _ := m["content"].(string); c != "" {
					gotContent += c
				}
			}
		case FrameDone:
			sawDone = true
		}
		if sawDone {
			break
		}
	}
	if gotContent != "abc" {
		t.Errorf("content = %q, want %q", gotContent, "abc")
	}
	if !sawDone {
		t.Error("did not see FrameDone")
	}
}
```

Add a test-local stub provider (inside `ws_test.go` or a shared `helpers_test.go`):

```go
type fakeStreamProvider struct{ chunks []string }

func (f *fakeStreamProvider) Name() string               { return "fake" }
func (f *fakeStreamProvider) Available() bool            { return true }
func (f *fakeStreamProvider) ModelInfo(string) *provider.ModelInfo {
	return &provider.ModelInfo{ContextLength: 1000, SupportsStreaming: true}
}
func (f *fakeStreamProvider) EstimateTokens(_, t string) (int, error) { return len(t) / 4, nil }
func (f *fakeStreamProvider) Complete(context.Context, *provider.Request) (*provider.Response, error) {
	return nil, errors.New("not implemented in fake")
}

type fakeStream struct {
	chunks []string
	i      int
}

func (f *fakeStream) Recv() (*provider.StreamEvent, error) {
	if f.i >= len(f.chunks) {
		return &provider.StreamEvent{Type: provider.EventDone, Response: &provider.Response{FinishReason: "end_turn"}}, nil
	}
	c := f.chunks[f.i]
	f.i++
	return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: c}}, nil
}

func (f *fakeStream) Close() error { return nil }

func (f *fakeStreamProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return &fakeStream{chunks: f.chunks}, nil
}
```

Make sure imports include `"errors"` and `"github.com/odysseythink/hermind/provider"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestDefaultChatRunner -v`
Expected: FAIL — `s.ProviderFactory` undefined, default runner still emits `not_configured`.

- [ ] **Step 3: Implement**

In `api/server.go`, extend `ServerOpts` and `Server`:

```go
// ProviderFactory resolves a "provider/model" ref to a ready-to-use
// provider.Provider. cli/web.go builds one that calls factory.New
// against the loaded config.
type ProviderFactory func(modelRef string) (provider.Provider, error)

type ServerOpts struct {
	Config          *config.Config
	ConfigPath      string
	Storage         storage.Storage
	Token           string
	Version         string
	ProviderFactory ProviderFactory // optional; without it streaming returns "not_configured"
}

type Server struct {
	opts            *ServerOpts
	router          chi.Router
	bootedAt        time.Time
	ChatRunner      ChatRunner
	ProviderFactory ProviderFactory // mirrors opts for test convenience
}
```

Inside `NewServer`, after validating opts:

```go
s.ProviderFactory = opts.ProviderFactory
```

Replace `defaultChatRunner` in `api/ws.go` with a provider-backed version:

```go
func (s *Server) defaultChatRunner(ctx context.Context, req ChatRequest, emit func(StreamFrame)) error {
	factory := s.ProviderFactory
	if factory == nil {
		emit(StreamFrame{
			Type:  FrameError,
			Error: &StreamError{Code: "not_configured", Message: "hermind web: ProviderFactory not configured"},
		})
		return nil
	}
	model := req.Model
	if model == "" {
		model = s.opts.Config.Model
	}
	prov, err := factory(model)
	if err != nil {
		emit(StreamFrame{
			Type:  FrameError,
			Error: &StreamError{Code: "provider_factory", Message: err.Error()},
		})
		return nil
	}

	stream, err := prov.Stream(ctx, &provider.Request{
		Model: model,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(req.Prompt)},
		},
		MaxTokens: 4096,
	})
	if err != nil {
		emit(StreamFrame{
			Type:  FrameError,
			Error: &StreamError{Code: "provider", Message: err.Error()},
		})
		return nil
	}
	defer stream.Close()

	for {
		ev, err := stream.Recv()
		if err != nil {
			emit(StreamFrame{
				Type:  FrameError,
				Error: &StreamError{Code: "stream_recv", Message: err.Error()},
			})
			return nil
		}
		switch ev.Type {
		case provider.EventDelta:
			if ev.Delta != nil && ev.Delta.Content != "" {
				emit(StreamFrame{Type: FrameDelta, Data: map[string]string{"content": ev.Delta.Content}})
			}
		case provider.EventDone:
			finish := ""
			if ev.Response != nil {
				finish = ev.Response.FinishReason
			}
			emit(StreamFrame{Type: FrameDone, Data: map[string]string{"finish_reason": finish}})
			return nil
		case provider.EventError:
			emit(StreamFrame{
				Type:  FrameError,
				Error: &StreamError{Code: "provider_stream", Message: ev.Err.Error()},
			})
			return nil
		}
	}
}
```

Make sure `api/ws.go` imports `"github.com/odysseythink/hermind/message"` and `"github.com/odysseythink/hermind/provider"`.

- [ ] **Step 4: Wire ProviderFactory from cli/web.go**

In `cli/web.go`, extend the `NewServer` call:

```go
srv, err := api.NewServer(&api.ServerOpts{
	Config:     app.Config,
	ConfigPath: app.ConfigPath,
	Storage:    app.Storage,
	Token:      token,
	Version:    Version,
	ProviderFactory: func(modelRef string) (provider.Provider, error) {
		cfg := resolveProviderConfig(app.Config, modelRef)
		return factory.New(cfg)
	},
})
```

Add imports `"github.com/odysseythink/hermind/provider"` and `"github.com/odysseythink/hermind/provider/factory"` if not already present. If `resolveProviderConfig` was defined by Plan D (cli/acp.go), keep using that; otherwise copy it in.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./api/ -run TestDefaultChatRunner -v -race`
Expected: PASS.

Full suite: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/server.go api/ws.go api/ws_test.go cli/web.go
git commit -m "feat(api): default ChatRunner backed by provider.Stream()"
```

---

## Task 6: Server-Sent Events (`/api/events`)

**Files:**
- Create: `api/sse.go`
- Create: `api/sse_test.go`
- Modify: `api/server.go` — register `GET /api/events`

- [ ] **Step 1: Write the failing test**

Create `api/sse_test.go`:

```go
package api

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE_EmitsFrames(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	// Install a one-shot test hook that pushes a single event.
	s.EventSource = func() <-chan StreamFrame {
		ch := make(chan StreamFrame, 1)
		ch <- StreamFrame{Type: FramePing}
		close(ch)
		return ch
	}
	httpSrv := httptest.NewServer(s.Router())
	defer httpSrv.Close()

	req, _ := http.NewRequest("GET", httpSrv.URL+"/api/events?t=t", nil)
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Errorf("content-type = %q", got)
	}

	// Read the first SSE frame.
	reader := bufio.NewReader(resp.Body)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line, "data: ") {
		t.Errorf("unexpected first line: %q", line)
	}
	if !strings.Contains(line, `"type":"ping"`) {
		t.Errorf("missing ping: %q", line)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestSSE -v`
Expected: FAIL — `Server.EventSource` undefined.

- [ ] **Step 3: Implement SSE**

Create `api/sse.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EventSource produces a fresh channel of frames per request. The
// server range-reads until the channel closes or the client
// disconnects. Typical wiring produces frames from a pub/sub bus
// subscribed on request, unsubscribed on disconnect.
type EventSource func() <-chan StreamFrame

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.EventSource == nil {
		http.Error(w, "events not configured", http.StatusNotImplemented)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	ch := s.EventSource()

	// Heartbeat every 30s so proxies don't time the connection out.
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(f)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-tick.C:
			fmt.Fprint(w, "data: {\"type\":\"ping\"}\n\n")
			flusher.Flush()
		}
	}
}
```

Add to `api/server.go`:

```go
type Server struct {
	// ... existing fields ...
	EventSource EventSource
}
```

and register the route:

```go
		r.Get("/events", s.handleEvents)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run TestSSE -v -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/sse.go api/sse_test.go api/server.go
git commit -m "feat(api): /api/events server-sent events endpoint"
```

---

## Task 7: Manual smoke test

- [ ] **Step 1: Build**

```bash
go build -o /tmp/hermind ./cmd/hermind
```

- [ ] **Step 2: Start the server**

```bash
/tmp/hermind web --no-browser --addr 127.0.0.1:9120 &
WEB_PID=$!
sleep 1
```

Capture the token from the stdout (`token: ...`).

- [ ] **Step 3: Drive the WebSocket with a tiny client**

```bash
TOKEN=<paste here>
python3 - <<PY
import json, websocket
ws = websocket.WebSocket()
ws.connect(f"ws://127.0.0.1:9120/api/chat/stream?t=$TOKEN")
ws.send(json.dumps({"prompt":"say hi in one word","model":"anthropic/claude-opus-4-6"}))
while True:
    msg = ws.recv()
    print(msg)
    if '"type":"done"' in msg or '"type":"error"' in msg:
        break
ws.close()
PY
```

Expected: multiple `{"type":"delta","data":{"content":"..."}}` frames, then `{"type":"done", ...}`.

- [ ] **Step 4: Cleanup**

```bash
kill $WEB_PID
rm /tmp/hermind
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - StreamFrame envelope ↔ Task 2 ✓
   - Query-token auth for upgrade paths ↔ Task 3 ✓
   - `/api/chat/stream` WebSocket with stub + real runner ↔ Tasks 4 + 5 ✓
   - `/api/events` SSE with heartbeat ↔ Task 6 ✓
   - Origin allowlist for WS upgrades ↔ Task 4 (`OriginPatterns`) ✓
   - Streaming of `provider.Stream()` deltas ↔ Task 5 ✓

2. **Placeholders:** Task 5 notes the possibility that `resolveProviderConfig` was defined by an earlier plan — copy it locally if not. No TBD content.

3. **Type consistency:**
   - `StreamFrame{Type, Data, Error}` shape stable across Tasks 2, 4, 5, 6.
   - `ChatRunner(ctx, req, emit)` signature stable across Tasks 4, 5.
   - `ServerOpts{ProviderFactory}` stable between server + cli wiring.
   - `EventSource = func() <-chan StreamFrame` stable in Task 6.

4. **Gaps (later work):**
   - Multi-turn history (the MVP sends the prompt as a single user message; history round-tripping through `session_id` is a follow-up once the frontend supports it).
   - Tool-call frame shapes (`tool_call`, `tool_result`) carry only `interface{}` payloads for now — tighter schema when tools are wired through the streaming path.
   - Client reconnect / resume-by-cursor — the MVP is a fresh stream per WebSocket.

---

## Definition of Done

- `go test ./api/... -race` passes, including the WS and SSE tests.
- `hermind web` serves `/api/chat/stream` (WebSocket) and `/api/events` (SSE) with the same Bearer token.
- Manual smoke test (WebSocket client → 3+ delta frames + done) succeeds against a real provider.
- Origin check rejects a WebSocket dial from outside `localhost` / `127.0.0.1`.
