# MCP Hypervisor PR-C — HTTP + SSE Transport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the streamable HTTP and SSE MCP transports. After PR-C, `newTransport()` dispatches all three Node-supported transports (stdio from PR-B, HTTP + SSE from PR-C). The Hypervisor lifecycle and REST routes need no further changes — they already consume the `Transport` interface generically. Operators can now configure MCP servers via `{url, type:"streamable"}`, `{url, type:"http"}`, `{url, type:"sse"}`, or `{url}` (SSE default) in `anythingllm_mcp_servers.json` and they Just Work.

**Architecture:** Two new files — `internal/mcp/transport_http.go` (streamable HTTP) and `internal/mcp/transport_sse.go` (SSE) — each implementing the `Transport` interface. Both use the MCP Go SDK chosen in PR-B's Task 0; if the SDK's HTTP/SSE client surface is incomplete, hand-roll using `net/http` (a 200-300 line cost, documented decision). Tests use `httptest.NewServer` with JSON-RPC and SSE event-stream mock handlers in a shared `internal/mcp/testutil/` package.

**Tech Stack:** Go 1.22+, `net/http`, `net/http/httptest`, MCP Go SDK (from PR-B decision), testify.

**Source spec:** `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §3-4, §6.

**Reference Node implementation:**
- `server/utils/MCP/hypervisor/index.js:422-441` (`createHttpTransport` — streamable vs SSE branching)
- `server/utils/MCP/hypervisor/index.js:344-398` (`#parseServerType`, `#validateServerDefinitionByType`)
- `@modelcontextprotocol/sdk/client/streamableHttp.js` (TS SDK, for protocol semantics)
- `@modelcontextprotocol/sdk/client/sse.js` (TS SDK, for SSE semantics)

**Prerequisite:** PR-B landed (stdio transport + hypervisor lifecycle + graceful shutdown).

---

## Pre-task: Read this section once before starting

### Existing Go surface (from PR-A/B)

- `mcp.Transport` interface — stable. PR-C adds two implementations.
- `mcp.newTransport(srv)` — currently:
  ```go
  switch parseServerType(srv) {
  case "stdio":     return newStdioTransport(srv)
  case "http","sse":return nil, ErrTransportNotImplemented
  }
  return nil, ErrInvalidServerType
  ```
  PR-C replaces both non-stdio branches with real constructors.
- `mcp.parseServerType(srv)` — returns `"stdio" | "http" | "sse" | ""`. **No changes needed**; the routing logic is correct per Node.
- `mcp.validateServerDefinition(name, srv, kind)` — PR-A stub validates `URL` presence + parseability for type-tagged HTTP. PR-C must ensure it's called *before* `newTransport` in Boot/ToggleServer (audit `hypervisor.go`).
- `mcp.Hypervisor.startServerLocked / pruneServerLocked` — generic, no changes.
- `ServerConfig.Headers map[string]string` — used by HTTP/SSE transports for auth/custom headers. Already loaded by `Config.Load`.
- `Transport.ProcessInfo()` — returns nil for HTTP/SSE (no subprocess to track).

### Two protocols at a glance

| Aspect | Streamable HTTP | SSE |
|---|---|---|
| Connect | POST `initialize` JSON-RPC to `srv.URL` | GET `srv.URL` with `Accept: text/event-stream`, wait for `event: endpoint` |
| Server→Client | Response body of each POST (JSON or SSE stream) | Persistent GET event-stream |
| Client→Server | POST to `srv.URL` (every request) | POST to endpoint URL received via SSE |
| Session ID | `Mcp-Session-Id` request/response header | URL path returned via `endpoint` event |
| Stateful | Optional (per server) | Inherently stateful |
| Close | Cancel in-flight context, close http.Client connections | Cancel SSE goroutine, close stream reader |
| Process info | nil | nil |
| Headers passthrough | `srv.Headers` on every POST | `srv.Headers` on both GET and POST |

### Methods to ship (PR-C scope)

| # | Owner | Signature |
|---|---|---|
| 1 | `mcp.httpTransport` *(new)* | Implements `Transport` via SDK's streamable HTTP client (or hand-roll) |
| 2 | `mcp.sseTransport` *(new)* | Implements `Transport` via SDK's SSE client (or hand-roll) |
| 3 | `mcp.newHTTPTransport(srv) (Transport, error)` *(new)* | Factory; chooses streamable vs SSE based on `srv.Type` |
| 4 | `mcp.newSSETransport(srv) (Transport, error)` *(new)* | Factory for plain SSE (URL only, no `type`) |
| 5 | `mcp.newTransport(srv)` *(replace HTTP/SSE branches)* | Dispatch by `parseServerType` to the new constructors |
| 6 | `testutil.NewStreamableHTTPMock(t, handlers) *Mock` *(new)* | httptest server speaking JSON-RPC over HTTP for tests |
| 7 | `testutil.NewSSEMock(t, handlers) *Mock` *(new)* | httptest server speaking SSE+POST for tests |

### Out of scope (explicit)

- **Custom CA bundles / mTLS** — PR-C uses Go default TLS verification. Custom certs are a follow-up; if a user reports trouble, document workaround via `SSL_CERT_FILE` env var.
- **HTTP/2 server push, websockets, custom MCP framings** — only HTTP and SSE per the official MCP spec.
- **Auto-reconnect on SSE disconnect** — Node doesn't either. A disconnect surfaces as `Ping() == false` and operator must reload.
- **Per-request timeout policy in the transport** — caller (Hypervisor) supplies context; transport respects it. No internal deadline beyond the 30s connect budget.
- **HTTP proxy support** — relies on `HTTPS_PROXY`/`HTTP_PROXY` env vars honoured by `http.DefaultTransport`. No explicit proxy config.

### Data invariants

- An HTTP/SSE transport's `ProcessInfo()` MUST return nil — the frontend uses `process == null` to render the "remote server" badge.
- `Headers` from config are applied as **request** headers on every outbound HTTP request, both POST (initialize, tool calls) and GET (SSE stream). Never log header *values* — only keys, since values may carry auth tokens.
- An `httptest.NewServer` lifetime must outlive the transport's: tests should `t.Cleanup(mock.Close); t.Cleanup(transport.Close)` in that order so the server isn't torn down while the SSE goroutine is still trying to read.
- `http.Client` is reused per-transport (not shared across servers) — different servers may have different cert pools / proxies in the future.
- SSE goroutine MUST exit when `Close()` is called; verify with `goleak` or manual `runtime.NumGoroutine` assertion in tests.

### TDD discipline

Each task follows: write failing test → run + confirm fail → implement → run + confirm pass → commit. New mock helpers in `testutil/` get their own micro-tests so behaviour drift is caught early (the mocks themselves are test code that becomes a dependency of other tests — they need tests).

---

## Task 0: Verify SDK HTTP/SSE client support + decision update

**File:** `.gpowers/decisions/2026-05-26-mcp-go-sdk.md` (extend)

**Test:** none (verification + decision update).

### Steps

- [ ] Re-read the SDK chosen in PR-B Task 0. Identify:
  - Does it export a streamable HTTP client? (look for `NewStreamableHTTPClient` / `NewHTTPClient` / similar)
  - Does it export an SSE client?
  - Are headers passable to the client?
  - Does it expose Close / context cancellation for the SSE goroutine?
- [ ] **If both transports are available in the SDK**, write a 10-line smoke client for each against a public MCP HTTP demo server (if accessible) or against a local mock. Confirm Initialize + ListTools roundtrip. Proceed to Task 1.
- [ ] **If one or both are missing**, decide between:
  - (a) Hand-roll the missing transport using `net/http` (~200-300 LOC each)
  - (b) Adopt a second library for the missing transport
  
  Document the decision in an "Update 2026-05-26 (PR-C)" section of `.gpowers/decisions/2026-05-26-mcp-go-sdk.md`:
  ```markdown
  ## Update 2026-05-26 (PR-C verification)
  - **HTTP client in SDK:** ✅ / ❌
  - **SSE client in SDK:** ✅ / ❌
  - **PR-C path:** wrap SDK / hand-roll / mixed
  - **If hand-rolling:** rationale + implementation file paths
  ```
- [ ] If hand-rolling, do NOT add a second MCP-specific dependency without re-running the dependency-footprint check from PR-B. `golang.org/x/net` (already transitively present) is fine.
- [ ] Run `go vet ./...` — clean.
- [ ] Commit: `chore(mcp): verify SDK HTTP/SSE support for PR-C` (decision doc update only; no code changes yet).

### Acceptance

- Decision doc has the verification update.
- Path forward (wrap vs hand-roll) is unambiguous for the next tasks.
- No code changes in this commit.

---

## Task 1: HTTP / SSE mock test fixtures

**File:** `internal/mcp/testutil/jsonrpc_mock.go`, `internal/mcp/testutil/sse_mock.go`, `internal/mcp/testutil/mock_test.go`

> Building the mocks first means Tasks 2-3 have a stable test substrate. The mocks emulate the *server* side of the MCP protocol — they are not MCP clients.

### Steps

- [ ] **Write** `testutil/jsonrpc_mock.go` providing `NewStreamableHTTPMock(t)`:
  ```go
  package testutil

  import (
      "encoding/json"
      "net/http"
      "net/http/httptest"
      "sync"
      "testing"
  )

  type ToolDef struct {
      Name        string
      Description string
      InputSchema json.RawMessage
      // Handler is called when client invokes this tool.
      Handler func(args map[string]any) (any, error)
  }

  type StreamableHTTPMock struct {
      Server     *httptest.Server
      URL        string
      SessionID  string
      Headers    map[string]string  // headers the mock requires; tests assert
      tools      map[string]ToolDef
      requestLog []*RecordedRequest
      mu         sync.Mutex
  }

  type RecordedRequest struct {
      Method  string                 // JSON-RPC method
      Params  map[string]any
      Headers http.Header
  }

  func NewStreamableHTTPMock(t *testing.T, tools []ToolDef) *StreamableHTTPMock {
      t.Helper()
      m := &StreamableHTTPMock{
          SessionID: "test-session-" + randID(),
          tools:     make(map[string]ToolDef),
      }
      for _, td := range tools { m.tools[td.Name] = td }
      m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
      m.URL = m.Server.URL
      t.Cleanup(m.Server.Close)
      return m
  }

  func (m *StreamableHTTPMock) handle(w http.ResponseWriter, r *http.Request) {
      // Parse JSON-RPC envelope
      var req struct {
          JSONRPC string          `json:"jsonrpc"`
          ID      json.RawMessage `json:"id"`
          Method  string          `json:"method"`
          Params  map[string]any  `json:"params"`
      }
      _ = json.NewDecoder(r.Body).Decode(&req)
      m.mu.Lock()
      m.requestLog = append(m.requestLog, &RecordedRequest{Method: req.Method, Params: req.Params, Headers: r.Header.Clone()})
      m.mu.Unlock()

      w.Header().Set("Content-Type", "application/json")
      w.Header().Set("Mcp-Session-Id", m.SessionID)

      switch req.Method {
      case "initialize":
          writeJSONRPCResult(w, req.ID, map[string]any{
              "protocolVersion": "2025-03-26",
              "serverInfo":      map[string]any{"name": "mock-http", "version": "1.0"},
              "capabilities":    map[string]any{"tools": map[string]any{}},
          })
      case "ping":
          writeJSONRPCResult(w, req.ID, map[string]any{})
      case "tools/list":
          tools := make([]map[string]any, 0, len(m.tools))
          for _, t := range m.tools {
              tools = append(tools, map[string]any{
                  "name":        t.Name,
                  "description": t.Description,
                  "inputSchema": rawOrEmpty(t.InputSchema),
              })
          }
          writeJSONRPCResult(w, req.ID, map[string]any{"tools": tools})
      case "tools/call":
          name, _ := req.Params["name"].(string)
          args, _ := req.Params["arguments"].(map[string]any)
          tool, ok := m.tools[name]
          if !ok {
              writeJSONRPCError(w, req.ID, -32601, "unknown tool: "+name)
              return
          }
          result, err := tool.Handler(args)
          if err != nil { writeJSONRPCError(w, req.ID, -32000, err.Error()); return }
          writeJSONRPCResult(w, req.ID, map[string]any{
              "content": []map[string]any{{"type": "text", "text": jsonString(result)}},
          })
      default:
          writeJSONRPCError(w, req.ID, -32601, "method not found: "+req.Method)
      }
  }

  func (m *StreamableHTTPMock) Requests() []*RecordedRequest {
      m.mu.Lock(); defer m.mu.Unlock()
      out := make([]*RecordedRequest, len(m.requestLog))
      copy(out, m.requestLog)
      return out
  }

  // helpers: writeJSONRPCResult, writeJSONRPCError, jsonString, randID, rawOrEmpty
  // ... (see implementation)
  ```
- [ ] **Write** `testutil/sse_mock.go` providing `NewSSEMock(t, tools)`:
  ```go
  package testutil

  // SSEMock mounts two endpoints:
  //   GET  /sse           — long-lived SSE stream; first event is `endpoint` with the POST URL
  //   POST /msg/:sessId   — receives client JSON-RPC; result written back on SSE stream
  type SSEMock struct {
      Server *httptest.Server
      URL    string  // pass to ServerConfig.URL — points to /sse
      // ... same tool registration + request log as HTTP mock
  }

  func NewSSEMock(t *testing.T, tools []ToolDef) *SSEMock {
      // ... wire two routes on a single mux, return server with .URL = baseURL + "/sse"
  }
  ```
  Critical: the SSE stream handler must call `w.(http.Flusher).Flush()` after writing each event. Use `r.Context().Done()` to detect client disconnect and clean up.
- [ ] **Write** `testutil/mock_test.go` with smoke tests for the mocks themselves:
  - `TestStreamableHTTPMock_InitializeRoundtrip` — POST `initialize`, assert JSON-RPC success response with mock's session id.
  - `TestStreamableHTTPMock_RecordsRequests` — POST `tools/list`, assert `Requests()` returns 1 entry with method `tools/list`.
  - `TestSSEMock_EndpointEventFirst` — GET `/sse`, parse SSE stream, assert first event is `event: endpoint` with `data: <url>` matching `/msg/...`.
  - `TestSSEMock_ToolCallRoundtrip` — open SSE stream, POST to endpoint, receive response event on SSE channel.
  - `TestSSEMock_DisconnectCleansUp` — open SSE, close client, assert mock server doesn't deadlock on next request.
- [ ] Run `go test ./internal/mcp/testutil/ -v -count=1 -race` — expect all green.
- [ ] Commit: `test(mcp): HTTP + SSE mock fixtures for transport tests`.

### Acceptance

- 5 mock self-tests pass under `-race`.
- Mocks are usable from external packages (exported types, `New*` constructors).
- No global state in mocks — each test gets its own instance.

---

## Task 2: HTTP transport

**File:** `internal/mcp/transport_http.go`, `internal/mcp/transport_http_test.go`

### Steps

- [ ] **Write failing tests** `transport_http_test.go`:
  - `TestHTTPTransport_Connect_Streamable` — `&ServerConfig{Type:"streamable", URL: mock.URL}`; assert `Connect(ctx)` returns nil; assert mock recorded `initialize`.
  - `TestHTTPTransport_Connect_HTTPAlias` — same with `Type:"http"`; identical behaviour.
  - `TestHTTPTransport_Connect_InvalidURL` — `URL:"://invalid"`; assert error wrapping or containing `"invalid URL"`.
  - `TestHTTPTransport_Connect_404` — mock returns 404 on any POST; assert Connect fails with error mentioning HTTP status.
  - `TestHTTPTransport_Connect_TLSError` — point to `https://expired.badssl.com/`; assert error contains TLS/certificate. (May be flaky on offline CI — guard with `if testing.Short()`.)
  - `TestHTTPTransport_ListTools` — connect to mock with 2 tools; `ListTools(ctx)` returns those tools.
  - `TestHTTPTransport_CallTool_Success` — mock has `echo(text)` returning text; assert result string contains echoed text.
  - `TestHTTPTransport_CallTool_HandlerError` — mock tool returns error; transport surfaces JSON-RPC error as Go error.
  - `TestHTTPTransport_HeadersPropagated` — `Headers: {"X-Auth":"abc"}`; after Connect, assert mock's recorded request had `X-Auth: abc`.
  - `TestHTTPTransport_Ping` — `Ping(ctx)` returns true while server up; close mock; assert next `Ping(ctx)` returns false.
  - `TestHTTPTransport_ProcessInfo_Nil` — assert `ProcessInfo()` returns nil.
  - `TestHTTPTransport_Close_Idempotent` — `Close()` twice no error.
  - `TestHTTPTransport_Close_AbortsInFlight` — start a `slow_echo` call in goroutine, call `Close()`, assert the in-flight call returns an error (context canceled).
  - `TestHTTPTransport_ConnectRespectsContext` — pass already-cancelled ctx; assert immediate error.
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `transport_http.go`:
  - Build a `*http.Client` with reasonable defaults (no global timeout; rely on ctx).
  - Apply `srv.Headers` to every outbound request via a `http.RoundTripper` wrapper or per-request injection.
  - Use the SDK's streamable HTTP client if available; otherwise hand-roll the JSON-RPC POST loop (initialize, then per-call POSTs; track `Mcp-Session-Id` if returned).
  - `ProcessInfo()` returns nil.
  - `Close()` cancels the transport's root context (which terminates in-flight requests) and is idempotent via `sync.Once`.
  - Skeleton:
    ```go
    package mcp

    import (
        "context"
        "fmt"
        "net/http"
        "net/url"
        "sync"
    )

    type httpTransport struct {
        srv       *ServerConfig
        client    *http.Client
        baseURL   *url.URL
        sessionID string

        mu        sync.RWMutex
        rootCtx   context.Context
        cancel    context.CancelFunc
        closeOnce sync.Once
    }

    func newHTTPTransport(srv *ServerConfig) (Transport, error) {
        u, err := url.Parse(srv.URL)
        if err != nil || u.Scheme == "" || u.Host == "" {
            return nil, fmt.Errorf("invalid URL %q", srv.URL)
        }
        ctx, cancel := context.WithCancel(context.Background())
        return &httpTransport{
            srv:     srv,
            client:  &http.Client{},
            baseURL: u,
            rootCtx: ctx,
            cancel:  cancel,
        }, nil
    }

    // Connect → POST initialize, store sessionID if returned.
    // ListTools → POST tools/list, parse []ToolSchema.
    // CallTool  → POST tools/call, parse result.
    // Ping      → POST ping (or short tools/list if ping not in SDK).
    // Close     → cancel rootCtx via closeOnce.
    // ProcessInfo → nil.
    ```
  - **Header injection helper:**
    ```go
    func (t *httpTransport) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
        req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL.String(), bytes.NewReader(body))
        if err != nil { return nil, err }
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Accept", "application/json, text/event-stream")
        if t.sessionID != "" { req.Header.Set("Mcp-Session-Id", t.sessionID) }
        for k, v := range t.srv.Headers {
            req.Header.Set(k, v) // user headers take precedence
        }
        return req, nil
    }
    ```
- [ ] Run `go test ./internal/mcp/ -run TestHTTPTransport -v -count=1 -race` — expect all green.
- [ ] Commit: `feat(mcp): streamable HTTP transport`.

### Acceptance

- 14 HTTP transport tests pass under `-race`.
- Header injection verified end-to-end via mock's `Requests()`.
- `Close()` aborts in-flight calls (no leaked goroutines).
- TLS error test passes when network reachable, skipped via `testing.Short()` otherwise.

---

## Task 3: SSE transport

**File:** `internal/mcp/transport_sse.go`, `internal/mcp/transport_sse_test.go`

### Steps

- [ ] **Write failing tests** `transport_sse_test.go`:
  - `TestSSETransport_Connect_EndpointDiscovery` — `&ServerConfig{URL: sseMock.URL}` (no `Type`); assert Connect returns nil; assert transport's endpoint URL matches mock's `/msg/...`.
  - `TestSSETransport_Connect_ExplicitTypeSSE` — `Type:"sse"`; identical behaviour.
  - `TestSSETransport_Connect_NoEndpointEvent` — mock that never sends `event: endpoint`; assert Connect fails with context deadline (use small ctx, e.g. 2s).
  - `TestSSETransport_Connect_MalformedEndpointEvent` — mock sends `data: not-a-url`; assert error.
  - `TestSSETransport_ListTools_Roundtrip` — full roundtrip via SSE stream + POST endpoint.
  - `TestSSETransport_CallTool_Success` — same.
  - `TestSSETransport_HeadersPropagated` — `Headers: {"X-Auth":"abc"}`; assert mock records header on both initial GET and on POST.
  - `TestSSETransport_Ping_StreamUp` — true while stream open; close stream from mock side; assert next Ping false.
  - `TestSSETransport_Close_StopsStream` — `Close()` then assert `runtime.NumGoroutine()` returns to baseline (with small slack) after 100ms.
  - `TestSSETransport_Close_Idempotent` — Close twice no error.
  - `TestSSETransport_ProcessInfo_Nil` — nil.
  - `TestSSETransport_ContextCancel_AbortsCall` — start slow_echo, cancel parent ctx, assert error.
  - `TestSSETransport_ServerDisconnect_PingFalse` — mid-session mock closes stream; next Ping false; in-flight call surfaces error.
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `transport_sse.go`:
  - On Connect:
    1. Start a goroutine that GETs `srv.URL` with `Accept: text/event-stream`, reads events into a channel.
    2. Wait (within ctx deadline, default 30s) for the first `event: endpoint` event.
    3. Resolve endpoint URL (may be relative — resolve against `srv.URL`).
    4. POST `initialize` to endpoint; wait for matching JSON-RPC response from event-stream goroutine (via correlation by `id`).
  - On ListTools/CallTool/Ping: POST JSON-RPC to endpoint; wait on response channel keyed by request id.
  - On Close: cancel root context, wait for reader goroutine to exit (with 1s safety timeout to avoid blocking forever).
  - Goroutine accounting: exactly ONE reader goroutine per transport, killed by ctx cancellation.
  - Skeleton:
    ```go
    package mcp

    import (
        "bufio"
        "context"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "sync"
    )

    type sseTransport struct {
        srv         *ServerConfig
        baseURL     *url.URL
        endpointURL *url.URL  // resolved on Connect
        client      *http.Client
        stream      io.Closer
        responses   sync.Map  // map[id]chan json.RawMessage
        nextID      atomic.Int64

        rootCtx   context.Context
        cancel    context.CancelFunc
        closeOnce sync.Once
        readerWG  sync.WaitGroup
    }

    // Reader goroutine: parses SSE events; dispatches by event type.
    // - "endpoint" event → resolve endpoint URL, signal readiness
    // - "message" event  → parse JSON-RPC, dispatch to waiting caller via id channel
    // - stream EOF       → close all pending callers with ErrStreamClosed
    ```
  - **Use `bufio.Scanner` with `ScanLines` for SSE parsing** — each event is `event: <name>\n` + `data: <payload>\n` + blank line. Standard format.
- [ ] Run `go test ./internal/mcp/ -run TestSSETransport -v -count=1 -race` — expect all green.
- [ ] Add a goroutine-leak assertion to one Close test:
  ```go
  func TestSSETransport_Close_NoGoroutineLeak(t *testing.T) {
      baseline := runtime.NumGoroutine()
      // ... connect + close ...
      time.Sleep(100 * time.Millisecond)
      after := runtime.NumGoroutine()
      assert.LessOrEqual(t, after, baseline+1, "goroutine leak detected")
  }
  ```
- [ ] Commit: `feat(mcp): SSE transport with endpoint discovery`.

### Acceptance

- 13 SSE transport tests pass under `-race`.
- Reader goroutine cleanly exits on `Close()` — verified by goroutine-count assertion.
- Endpoint URL relative-resolution works (test with `data: /msg/abc` against mock base `http://x/sse`).

---

## Task 4: Wire transport factory dispatch

**File:** `internal/mcp/transport.go`, update `internal/mcp/transport_test.go` from PR-B

### Steps

- [ ] **Update** the factory tests from PR-B:
  - `TestNewTransport_HTTPReturnsNotImplemented` → rename to `TestNewTransport_HTTPDispatched`; now asserts non-nil transport + nil error.
  - `TestNewTransport_SSEReturnsNotImplemented` → rename to `TestNewTransport_SSEDispatched`; same.
  - Add `TestNewTransport_StreamableExplicit` — `Type:"streamable"` dispatches to httpTransport.
  - Add `TestNewTransport_InvalidEmpty` — keep from PR-B.
- [ ] **Replace** `newTransport` body:
  ```go
  func newTransport(srv *ServerConfig) (Transport, error) {
      switch parseServerType(srv) {
      case "stdio":
          return newStdioTransport(srv)
      case "http":
          return newHTTPTransport(srv)
      case "sse":
          return newSSETransport(srv)
      }
      return nil, ErrInvalidServerType
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestNewTransport -v` — expect all green.
- [ ] Commit: `feat(mcp): factory dispatches HTTP + SSE transports`.

### Acceptance

- All 4 factory tests pass.
- No `ErrTransportNotImplemented` returned for any valid input.

---

## Task 5: Hypervisor end-to-end coverage for HTTP/SSE

**File:** `internal/mcp/hypervisor_test.go` (extend; no production code changes expected)

### Steps

- [ ] **Add** integration tests that drive the full Hypervisor → factory → transport → mock chain:
  - `TestHypervisor_Boot_HTTPServer` — write a config pointing at `testutil.NewStreamableHTTPMock`; Boot; assert `Servers(ctx)` shows `Running:true, Tools:[...]`, `Process:nil`.
  - `TestHypervisor_Boot_SSEServer` — same with SSE mock.
  - `TestHypervisor_CallTool_HTTPServer_E2E` — Boot → CallTool → assert returned result matches mock's tool handler output.
  - `TestHypervisor_CallTool_SSEServer_E2E` — same.
  - `TestHypervisor_ToggleServer_HTTPServer_Off_Then_On` — Toggle off, assert `Servers()` reports not-running, mock receives no further requests; Toggle on, mock receives new initialize.
  - `TestHypervisor_DeleteServer_HTTPServer` — Delete; assert config no longer contains entry; Toggle attempts after delete return `ErrServerNotFound`.
  - `TestHypervisor_Boot_MixedTransports` — config has 1 stdio + 1 HTTP + 1 SSE; Boot; assert all 3 running with correct process/tool data.
  - `TestHypervisor_Servers_HTTPSuppressionFilters` — HTTP server with `suppressedTools:["add"]`; Boot; `Servers()` returns tools sans `add`.
  - `TestHypervisor_PruneAll_HTTPCleansUp` — Boot HTTP; PruneAll; assert next Ping false; no leaked goroutines.
  - `TestHypervisor_Boot_HTTPConnectTimeout` — mock that hangs on initialize; set `connectionTimeoutForTest = 2s` from PR-B; Boot; assert `mcpLoadingResults["x"].Status == "failed"` with message containing `"deadline"` or `"timeout"`.
- [ ] Run `go test ./internal/mcp/ -run TestHypervisor -v -count=1 -race` — full lifecycle green.
- [ ] Commit: `test(mcp): end-to-end Hypervisor coverage for HTTP + SSE`.

### Acceptance

- 10 new integration tests pass under `-race`.
- Mixed-transport boot test confirms factory dispatch works for all three kinds in one Hypervisor instance.
- Goroutine-leak guard still green.

---

## Task 6: Handler-level e2e for remote transports + docs

**Files:** `internal/handlers/mcp_test.go` (extend), `internal/mcp/doc.go` (extend)

### Steps

- [ ] **Add** to `handlers/mcp_test.go`:
  - `TestMCPHandler_ListServers_HTTP_E2E` — fixture writes config for HTTP mock; GET `/api/mcp-servers/list` returns the HTTP server with `running:true, tools:[...]`, `process:null`.
  - `TestMCPHandler_CallTool_HTTPServer_E2E` — POST `/api/mcp/<name>/tools/<tool>/call` returns 200 with mock's handler result.
  - `TestMCPHandler_CallTool_SSEServer_E2E` — same with SSE mock.
- [ ] Run `go test ./internal/handlers/ -run TestMCPHandler -v -count=1` — all green (PR-A 12 + PR-B 3 + PR-C 3 = 18 tests).
- [ ] **Extend** package doc in `internal/mcp/doc.go` (create if not present in PR-A):
  ```go
  // Package mcp implements the MCP hypervisor: lifecycle, config, and transport
  // abstraction. Three transports are supported:
  //
  //   - stdio: spawn an MCP server as a child process and speak JSON-RPC
  //     over its stdin/stdout. Best for local, language-specific MCP
  //     servers (Node, Python, Rust binaries).
  //   - http (a.k.a. streamable): POST JSON-RPC to a remote URL. Best for
  //     hosted, network-resident MCP servers with stateless or session-
  //     based interaction.
  //   - sse: open a long-lived SSE event-stream to a remote URL for
  //     server→client messages; POST to a session-scoped endpoint for
  //     client→server messages.
  //
  // The Hypervisor is a process-wide singleton initialised via Instance(cfg);
  // its lifecycle is controlled by Boot(ctx) and PruneAll(). Each running MCP
  // server is exposed through a Transport interface; transports may carry no
  // process (HTTP/SSE) or a child process (stdio) — callers detect via
  // ProcessInfo().
  package mcp
  ```
- [ ] Run full suite: `go test ./... -count=1 -race` — green.
- [ ] Run `go vet ./...` — clean.
- [ ] Commit: `feat(mcp): handler e2e tests for HTTP + SSE; package docs`.

### Acceptance

- 18 handler tests pass.
- Package godoc renders cleanly (`go doc ./internal/mcp/`).
- Full repo test suite green under `-race`.

---

## Post-PR checklist

- [ ] `.gpowers/decisions/2026-05-26-mcp-go-sdk.md` updated with PR-C verification section.
- [ ] `go test ./internal/mcp/ -count=1 -race -v` reports ≥ 80 tests passing (PR-A ~30 + PR-B ~25 + PR-C ~30).
- [ ] `go test ./internal/mcp/testutil/ -count=1 -race -v` reports ≥ 5 mock self-tests passing.
- [ ] `go test ./internal/handlers/ -run TestMCPHandler -count=1 -v` reports ≥ 18 tests passing.
- [ ] `go test ./... -count=1` full repo green.
- [ ] `go vet ./...` clean.
- [ ] `internal/mcp/transport_http.go` and `transport_sse.go` do NOT import `os/exec` (compile-time guarantee they're network-only).
- [ ] Update `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §10 — mark PR-C done, cross out HTTP/SSE rows in the risk table.
- [ ] **Frontend smoke** (manual, optional): configure both an HTTP-type and an SSE-type MCP server, verify MCP settings page renders both with `running: true` and tool lists.

---

## Risk notes

- **SDK protocol drift** — if the chosen SDK lags behind the latest MCP spec (e.g. `Mcp-Session-Id` header semantics changed in 2025-03-26 spec), our hand-roll branch may be more forward-compatible than the SDK wrapping. Re-evaluate at PR-C completion: is the SDK keeping up? If not, prepare a "hand-roll all transports" follow-up.
- **SSE keepalive / proxies** — corporate proxies often drop SSE connections after 60s of silence. The mock doesn't simulate this; real servers should emit keepalive comments (`:ping\n\n` every 30s). PR-C trusts the server side to keep the connection alive; if operators report drops, add client-side reconnect as PR-C.1.
- **Concurrent CallTool on one HTTP transport** — `http.Client` is goroutine-safe; correlation by request id keeps responses on the right callers. Verify by running `TestHTTPTransport_ConcurrentCalls` (spawn 10 concurrent calls, assert all distinct results).
- **SSE goroutine leak on test panic** — if a test asserts on stream data before `Close()` and panics, the reader goroutine survives. Use `t.Cleanup` blocks (not `defer`) so cleanup runs even on FailNow.
- **TLS root CA on Alpine / minimal images** — Go's TLS dialer needs `/etc/ssl/certs/ca-certificates.crt` (Alpine has it via `ca-certificates` package). Docker base image must include it; document in Dockerfile review for PR-C deployment.
- **HTTP transport doesn't retry transient 5xx** — by design. MCP semantic is "if it failed, the caller decides whether to retry". A retrying transport would complicate idempotency assumptions.
- **No request-size or response-size cap** — large tool results (e.g. a 10MB document blob) flow through unbuffered. If memory becomes a concern, add `http.MaxBytesReader` wrapper in a follow-up.
- **`url.Parse` is lax** — accepts URLs without schemes like `localhost:8080`. `validateServerDefinition` should require `Scheme != "" && Host != ""`; verify it does (PR-A `isValidURL` already checks both).
- **`runtime.NumGoroutine` test flakiness** — Go runtime spins helper goroutines lazily. Use a baseline-capture pattern with `+1` slack, not exact equality.

---

## Estimate

- Task 0 (SDK verification): 1 h
- Task 1 (HTTP + SSE mock fixtures): 3-4 h
- Task 2 (HTTP transport + 14 tests): 4-5 h
- Task 3 (SSE transport + 13 tests): 5-6 h *(SSE is trickier due to two-channel protocol + goroutine accounting)*
- Task 4 (factory dispatch): 30 min
- Task 5 (Hypervisor e2e + 10 tests): 2-3 h
- Task 6 (handler e2e + docs): 1 h

**Total: ~16-20 hours**, i.e. 2-3 working days. Tasks 2 + 3 can run in parallel between two engineers; otherwise single-track ~3 days.
