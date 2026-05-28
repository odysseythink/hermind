# MCP Hypervisor PR-D — Tool-Call REST Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the existing `POST /api/mcp/:name/tools/:tool/call` route (landed in PR-A, dispatch wired in PR-B/C) for production use. Add audit logging, input-schema validation, per-call timeout policy, request-body size cap, per-server concurrency limit, and stable error codes. After PR-D this route is safe to expose to lightly trusted callers (agent integrations, internal automation) without worrying about resource exhaustion, malformed payloads silently corrupting an MCP server, or untraceable invocations.

**Architecture:** Changes concentrated in `handlers/mcp.go` (the route's logic gains pre-flight checks and post-call telemetry) and three small additions inside `internal/mcp/`: a tool-schema cache on `activeClient`, a per-server semaphore in a new `concurrency.go`, and a small `errors.go` with stable error codes. `MCPService` gets one new helper (`GetToolSchema`) — its existing `CallTool` signature stays untouched so PR-E and downstream consumers don't need changes.

**Tech Stack:** Go 1.22+, `github.com/xeipuuv/gojsonschema` (already in `go.sum` transitively — promote to direct dep), `golang.org/x/sync/semaphore` (likely already transitive; verify), testify.

**Source spec:** `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §5.2 + design note "Phase 2 可改为按 workspace 权限" — PR-D does the production-readiness slice; workspace-scoped ACL stays for a later PR.

**Scope clarification vs. original design:**

The original design listed PR-D as "tool-call REST 代理 + 错误码 (404/422/502) + 集成测试". Those bullet items already landed: the route + 404/422/502 codes shipped in PR-A; e2e tests across all three transports shipped in PR-B/C. **PR-D re-scopes to production-grade hardening** of the same route (audit log, schema validation, timeouts, size caps, concurrency limits, stable error codes). User confirmed scope on 2026-05-26.

**Prerequisite:** PR-A, PR-B, PR-C landed. The handler, transports, and Hypervisor lifecycle are all functional.

---

## Pre-task: Read this section once before starting

### Existing Go surface (from PR-A/B/C)

- `handlers/mcp.go:CallTool` — current handler:
  ```go
  func (h *MCPHandler) CallTool(c *gin.Context) {
      name := c.Param("name")
      tool := c.Param("tool")
      // ... parse body, dispatch to svc.CallTool, return {success, result, error} ...
  }
  ```
  All hardening logic lives in this function and supporting helpers in `handlers/mcp_validation.go` (new).
- `services.MCPService.CallTool(ctx, server, tool, args) (any, error)` — keep this signature; do not gate the service layer with concerns that belong to the HTTP boundary.
- `mcp.Hypervisor.mcps[name] *activeClient` — currently `{transport Transport, process *ProcessInfo}`. PR-D adds `tools []ToolSchema, schemaByName map[string]json.RawMessage` populated at boot time so the handler can look up `inputSchema` without re-listing tools every call.
- `services.EventLogService.LogEvent(ctx, event string, metadata map[string]any, userID *int) error` (`services/event_log_service.go:20`) — used as-is for audit log.
- `models.User` is set into Gin context by `middleware.ValidatedRequest`; cast via `c.MustGet("user").(*models.User)` to get the actor for audit logs.
- `gojsonschema v1.2.0` is already in `go.sum` transitively. Task 3 promotes to direct require.

### New helpers (this PR)

| # | File | Symbol | Purpose |
|---|---|---|---|
| 1 | `internal/mcp/errors.go` *(new)* | `ErrorCode` enum + `CodedError` type | Stable error codes returned to clients |
| 2 | `internal/mcp/concurrency.go` *(new)* | `concurrencyLimiter` + `acquire(name)/release(name)` | Per-server semaphore, default 4, override via config |
| 3 | `internal/mcp/hypervisor.go` *(modify)* | `activeClient.tools []ToolSchema` + `activeClient.schemaByName map[string]json.RawMessage` | Cache tool list at boot; refresh on ToggleServer |
| 4 | `internal/mcp/hypervisor.go` *(new)* | `Hypervisor.GetToolSchema(server, tool) (*ToolSchema, error)` | Lookup cached schema; returns `ErrServerNotFound` / `ErrToolNotFound` |
| 5 | `services/mcp_service.go` *(new method)* | `MCPService.GetToolSchema(server, tool)` | Thin pass-through |
| 6 | `handlers/mcp_validation.go` *(new)* | `validateArgsAgainstSchema(args, inputSchema) error` | gojsonschema wrapper |
| 7 | `handlers/mcp_validation.go` *(new)* | `parseTimeoutParam(s string) (time.Duration, error)` | Parse `?timeout=30s`, bound to `[1s, 300s]` |
| 8 | `handlers/mcp.go` *(modify)* | `CallTool` rewritten with the new pipeline | — |

### New response shape

**Success** (unchanged from PR-A):
```json
{ "success": true, "result": <any>, "error": null }
```

**Error** — adds `errorCode` field; `error` stays a human string for back-compat with PR-A consumers:
```json
{
  "success": false,
  "result": null,
  "error": "tool 'xyz' not found on server 'echo-mcp'",
  "errorCode": "TOOL_NOT_FOUND"
}
```

### Stable error codes

| Code | HTTP | Reason |
|---|---|---|
| `INVALID_BODY` | 400 | Body isn't valid JSON |
| `INVALID_PARAMS` | 422 | Query params invalid (e.g. `timeout` out of range) |
| `SERVER_NOT_FOUND` | 404 | `name` doesn't exist in config or isn't running |
| `TOOL_NOT_FOUND` | 404 | `tool` not exposed by the named server |
| `ARGS_SCHEMA_MISMATCH` | 422 | `arguments` failed JSON Schema validation against tool's `inputSchema` |
| `BODY_TOO_LARGE` | 413 | Body exceeded the 10 MiB cap |
| `CONCURRENCY_LIMIT` | 429 | Per-server in-flight cap reached, retry later |
| `CALL_TIMEOUT` | 504 | Tool call exceeded the deadline |
| `TRANSPORT_ERROR` | 502 | Underlying MCP transport returned an error |
| `INTERNAL_ERROR` | 500 | Unexpected server-side fault |

> Codes are stable string identifiers. Adding a new code is non-breaking; renaming an existing one is breaking. Treat them like enum values.

### Configuration knobs

Add **only** these two fields to `config.Config`:

| Field | Env | Default | Notes |
|---|---|---|---|
| `MCPCallTimeoutDefault` | `MCP_CALL_TIMEOUT_DEFAULT` | `30s` | Default per-call deadline; query param can override within bounds |
| `MCPCallConcurrencyPerServer` | `MCP_CALL_CONCURRENCY_PER_SERVER` | `4` | Per-server semaphore size |

Per-server override lives in the existing JSON config under `hermind.maxConcurrency`:
```json
{
  "mcpServers": {
    "heavy-server": { "command": "...", "hermind": { "maxConcurrency": 1 } }
  }
}
```

Body-size cap is hard-coded at **10 MiB**. If an operator needs more, they file a follow-up.

Query-param `?timeout=N` accepts Go duration strings (`30s`, `2m`); bounded to `[1s, 300s]`.

### Out of scope (explicit)

- **Workspace-scoped ACL** (Phase 2 from design §5.2). PR-D stays admin-only via session auth.
- **API-key auth on this route**. Still session-only in PR-D.
- **Streaming tool responses**. Synchronous request/response only.
- **Tool result schema validation**. Only `arguments` (input) is validated.
- **Cross-server rate limiting** (global cap). Only per-server limits.
- **OpenTelemetry/distributed tracing**. Audit log via `EventLogService` is the only telemetry channel in PR-D.
- **Retry on transient `TRANSPORT_ERROR`**. Caller's responsibility.

### Data invariants

- `activeClient.tools` is the snapshot taken right after `Connect` succeeds. It is **never** refreshed during the lifetime of an `activeClient`. To refresh, the operator toggles the server off then on, or calls `force-reload`.
- `activeClient.schemaByName[toolName]` is `tools[i].InputSchema`. If the server didn't ship an `inputSchema` for a tool, the entry is omitted (lookup returns `nil` schema → handler skips validation, dispatches with the raw args).
- Audit log events are best-effort: a failure to write to `event_logs` MUST NOT block the tool call. Log the failure to `mlog` and proceed.
- Concurrency-limit failures (`CONCURRENCY_LIMIT`) are NOT counted as `TRANSPORT_ERROR`s in the audit log — they are a separate `mcp.call.rejected` event.
- The structured error response is **the only place** the new `errorCode` field appears. The 5 Node-parity admin routes keep their plain `{success, error}` shape — PR-D does NOT touch them.

### TDD discipline

Each task follows: write failing test → run + confirm fail → implement → run + confirm pass → commit. Tests live next to handler/service code. **Do not** add timing-dependent tests for concurrency (those go in Task 5 with explicit synchronisation primitives, not `time.Sleep`).

---

## Task 1: Stable error codes + structured handler error helper

**Files:** `internal/mcp/errors.go` *(new)*, `internal/handlers/mcp_errors.go` *(new)*, `internal/handlers/mcp_errors_test.go` *(new)*

### Steps

- [ ] **Write failing test** `handlers/mcp_errors_test.go`:
  - `TestCodedError_RoundtripJSON` — `respondCodedError(c, CodeToolNotFound, "tool x not found on y", nil)` writes `{success:false, result:null, error:"tool x not found on y", errorCode:"TOOL_NOT_FOUND"}` with HTTP 404.
  - `TestCodedError_DetailsAreNotLeaked` — passing a `details` map writes it as a sibling `details` field; passing nil omits the field (don't render `"details":null`).
  - `TestCodedError_AllCodesMap` — table-driven test over all 10 codes, asserts each maps to its documented HTTP status.
  - `TestCodedError_RespectsContentType` — response `Content-Type: application/json; charset=utf-8`.
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `internal/mcp/errors.go`:
  ```go
  package mcp

  // ErrorCode is a stable identifier for tool-call failures returned to clients.
  type ErrorCode string

  const (
      CodeInvalidBody        ErrorCode = "INVALID_BODY"
      CodeInvalidParams      ErrorCode = "INVALID_PARAMS"
      CodeServerNotFound     ErrorCode = "SERVER_NOT_FOUND"
      CodeToolNotFound       ErrorCode = "TOOL_NOT_FOUND"
      CodeArgsSchemaMismatch ErrorCode = "ARGS_SCHEMA_MISMATCH"
      CodeBodyTooLarge       ErrorCode = "BODY_TOO_LARGE"
      CodeConcurrencyLimit   ErrorCode = "CONCURRENCY_LIMIT"
      CodeCallTimeout        ErrorCode = "CALL_TIMEOUT"
      CodeTransportError     ErrorCode = "TRANSPORT_ERROR"
      CodeInternalError      ErrorCode = "INTERNAL_ERROR"
  )

  // ErrToolNotFound is returned by Hypervisor.GetToolSchema when the named
  // tool is not present in the running server's tool list.
  var ErrToolNotFound = errors.New("MCP tool not found")
  ```
- [ ] **Implement** `internal/handlers/mcp_errors.go`:
  ```go
  package handlers

  import (
      "net/http"

      "github.com/gin-gonic/gin"
      "github.com/odysseythink/hermind/backend/internal/mcp"
  )

  func codeToStatus(c mcp.ErrorCode) int {
      switch c {
      case mcp.CodeInvalidBody:        return http.StatusBadRequest
      case mcp.CodeInvalidParams:      return http.StatusUnprocessableEntity
      case mcp.CodeServerNotFound:     return http.StatusNotFound
      case mcp.CodeToolNotFound:       return http.StatusNotFound
      case mcp.CodeArgsSchemaMismatch: return http.StatusUnprocessableEntity
      case mcp.CodeBodyTooLarge:       return http.StatusRequestEntityTooLarge
      case mcp.CodeConcurrencyLimit:   return http.StatusTooManyRequests
      case mcp.CodeCallTimeout:        return http.StatusGatewayTimeout
      case mcp.CodeTransportError:     return http.StatusBadGateway
      }
      return http.StatusInternalServerError
  }

  func respondCodedError(c *gin.Context, code mcp.ErrorCode, msg string, details map[string]any) {
      body := gin.H{
          "success":   false,
          "result":    nil,
          "error":     msg,
          "errorCode": string(code),
      }
      if len(details) > 0 {
          body["details"] = details
      }
      c.JSON(codeToStatus(code), body)
  }

  func respondCallSuccess(c *gin.Context, result any) {
      c.JSON(http.StatusOK, gin.H{
          "success": true,
          "result":  result,
          "error":   nil,
      })
  }
  ```
- [ ] Run `go test ./internal/handlers/ -run TestCodedError -v` — expect all green.
- [ ] Commit: `feat(mcp): stable error codes + structured error responder`.

### Acceptance

- 4 helper tests pass.
- Every `ErrorCode` constant maps to a documented HTTP status via `codeToStatus`.
- The handler error helper writes the documented JSON shape exactly.

---

## Task 2: Tool-schema cache + GetToolSchema lookup

**Files:** `internal/mcp/hypervisor.go` *(modify)*, `internal/mcp/hypervisor_test.go` *(extend)*, `internal/services/mcp_service.go` *(extend)*

### Steps

- [ ] **Write failing tests** in `hypervisor_test.go`:
  - `TestHypervisor_BootCachesTools` — boot echo fixture (3 tools); inspect `activeClient.tools` length is 3 and `schemaByName["echo"]` is a non-empty JSON object.
  - `TestHypervisor_GetToolSchema_Found` — boot echo; `GetToolSchema("echo-mcp", "echo")` returns non-nil `*ToolSchema` with `Name=="echo"`.
  - `TestHypervisor_GetToolSchema_ServerNotFound` — `GetToolSchema("ghost", "x")` returns `(nil, ErrServerNotFound)`.
  - `TestHypervisor_GetToolSchema_ToolNotFound` — `GetToolSchema("echo-mcp", "nope")` returns `(nil, ErrToolNotFound)`.
  - `TestHypervisor_GetToolSchema_ToolWithoutInputSchema` — fixture tool that doesn't expose inputSchema; `GetToolSchema` returns non-nil `*ToolSchema` with `InputSchema == nil` (caller must handle nil to skip validation).
  - `TestHypervisor_GetToolSchema_AfterToggleOff` — boot then toggle off; `GetToolSchema(...)` returns `ErrServerNotFound` (cache wiped on prune).
- [ ] Run tests — expect compile errors.
- [ ] **Modify** `activeClient` struct in `hypervisor.go`:
  ```go
  type activeClient struct {
      transport    Transport
      process      *ProcessInfo
      tools        []ToolSchema
      schemaByName map[string]json.RawMessage
  }
  ```
- [ ] **Modify** `startServerLocked` to populate the cache right after `Connect` succeeds:
  ```go
  if err := transport.Connect(ctx); err != nil {
      _ = transport.Close()
      h.results[srv.Name] = LoadResult{Status: "failed", Message: err.Error()}
      return
  }
  tools, err := transport.ListTools(ctx)
  if err != nil {
      // Tools list failure is non-fatal — server is up but schema cache stays empty.
      // Operator can refresh by toggling.
      mlog.Warn("mcp tool list failed", mlog.String("server", srv.Name), mlog.Err(err))
      tools = nil
  }
  schemaByName := make(map[string]json.RawMessage, len(tools))
  for _, t := range tools {
      if len(t.InputSchema) > 0 {
          schemaByName[t.Name] = t.InputSchema
      }
  }
  h.mcps[srv.Name] = &activeClient{
      transport:    transport,
      process:      transport.ProcessInfo(),
      tools:        tools,
      schemaByName: schemaByName,
  }
  ```
- [ ] **Add** to `hypervisor.go`:
  ```go
  // GetToolSchema returns the cached tool definition for (server, tool). The
  // returned ToolSchema may have a zero-length InputSchema if the upstream
  // server didn't advertise one; callers should treat that as "skip
  // validation" rather than as an error.
  func (h *Hypervisor) GetToolSchema(server, tool string) (*ToolSchema, error) {
      h.mu.RLock()
      defer h.mu.RUnlock()
      client, ok := h.mcps[server]
      if !ok {
          return nil, fmt.Errorf("%w: %s", ErrServerNotFound, server)
      }
      for i := range client.tools {
          if client.tools[i].Name == tool {
              t := client.tools[i]
              return &t, nil
          }
      }
      return nil, fmt.Errorf("%w: %s on server %s", ErrToolNotFound, tool, server)
  }
  ```
- [ ] **Update** `Servers()` to use the cached `client.tools` instead of calling `ListTools` again. The PR-C version calls `ListTools(ctx)` on every list — that's wasteful and adds latency. Replace with `client.tools` filtered by suppression.
- [ ] **Update** `pruneServerLocked` — no code change needed (it already `delete(h.mcps, name)` which removes the cache).
- [ ] **Add** to `services/mcp_service.go`:
  ```go
  func (s *MCPService) GetToolSchema(server, tool string) (*mcp.ToolSchema, error) {
      return s.hv.GetToolSchema(server, tool)
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestHypervisor -v -count=1 -race` — expect all green.
- [ ] Run `go test ./internal/mcp/ -v -count=1 -race` — full package green.
- [ ] Commit: `feat(mcp): cache tool schemas at boot + GetToolSchema lookup`.

### Acceptance

- 6 new hypervisor tests pass.
- `Servers()` no longer calls `ListTools` for cached entries (verify by counting mock requests in HTTP transport test).
- Cache is automatically dropped when the server is pruned (no stale schemas).

---

## Task 3: Input schema validation

**Files:** `go.mod` *(promote gojsonschema)*, `internal/handlers/mcp_validation.go` *(new)*, `internal/handlers/mcp_validation_test.go` *(new)*

### Steps

- [ ] Promote `gojsonschema` to direct dependency:
  ```bash
  go get github.com/xeipuuv/gojsonschema@v1.2.0
  go mod tidy
  ```
- [ ] **Write failing test** `mcp_validation_test.go`:
  - `TestValidateArgs_NoSchema` — passing `inputSchema=nil`, any args return nil error (no schema → skip).
  - `TestValidateArgs_EmptySchema` — passing `inputSchema=json.RawMessage("{}")`, any args return nil (empty schema accepts everything).
  - `TestValidateArgs_RequiredFieldMissing` — schema `{"type":"object","required":["text"],"properties":{"text":{"type":"string"}}}`, args `{}`, returns error containing `"text"` and `"required"`.
  - `TestValidateArgs_WrongType` — schema requires `text:string`, args `{"text":123}`, returns error mentioning `"string"`.
  - `TestValidateArgs_AdditionalProperties` — schema disallows extras, args have extra key, returns error.
  - `TestValidateArgs_MultipleErrorsAggregated` — args missing 2 required fields; error message contains both field names.
  - `TestValidateArgs_NestedObject` — schema with nested `properties.foo.properties.bar`, args missing `foo.bar`, returns error.
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `internal/handlers/mcp_validation.go`:
  ```go
  package handlers

  import (
      "encoding/json"
      "fmt"
      "strings"

      "github.com/xeipuuv/gojsonschema"
  )

  // validateArgsAgainstSchema returns nil if args satisfies inputSchema, or an
  // error whose Error() contains the aggregated mismatch messages. A nil or
  // empty schema returns nil unconditionally.
  func validateArgsAgainstSchema(args map[string]any, inputSchema json.RawMessage) error {
      if len(inputSchema) == 0 {
          return nil
      }
      schemaLoader := gojsonschema.NewBytesLoader(inputSchema)
      argsBytes, err := json.Marshal(args)
      if err != nil {
          return fmt.Errorf("marshal args: %w", err)
      }
      docLoader := gojsonschema.NewBytesLoader(argsBytes)
      result, err := gojsonschema.Validate(schemaLoader, docLoader)
      if err != nil {
          return fmt.Errorf("schema validate: %w", err)
      }
      if result.Valid() {
          return nil
      }
      msgs := make([]string, 0, len(result.Errors()))
      for _, e := range result.Errors() {
          msgs = append(msgs, e.String())
      }
      return fmt.Errorf("schema validation failed: %s", strings.Join(msgs, "; "))
  }
  ```
- [ ] Run `go test ./internal/handlers/ -run TestValidateArgs -v -count=1` — expect all green.
- [ ] Commit: `feat(mcp): input schema validation via gojsonschema`.

### Acceptance

- 7 validation tests pass.
- `gojsonschema` is a direct dependency in `go.mod`.
- Errors aggregate **all** mismatches (not just the first).

---

## Task 4: Per-call timeout + body-size cap

**Files:** `internal/config/config.go` *(extend)*, `internal/handlers/mcp_validation.go` *(extend)*, `internal/handlers/mcp_validation_test.go` *(extend)*

### Steps

- [ ] **Add** two fields to `config.Config`:
  ```go
  MCPCallTimeoutDefault       time.Duration `env:"MCP_CALL_TIMEOUT_DEFAULT" envDefault:"30s"`
  MCPCallConcurrencyPerServer int           `env:"MCP_CALL_CONCURRENCY_PER_SERVER" envDefault:"4"`
  ```
  (Add `import "time"` if not present.)
- [ ] **Write failing tests** in `mcp_validation_test.go`:
  - `TestParseTimeoutParam_Empty` — empty string returns `(0, nil)` (caller uses default).
  - `TestParseTimeoutParam_Valid_30s` — `"30s"` returns `30*time.Second`.
  - `TestParseTimeoutParam_Valid_2m` — `"2m"` returns `2*time.Minute`.
  - `TestParseTimeoutParam_TooSmall` — `"500ms"` returns error containing `"out of range"`.
  - `TestParseTimeoutParam_TooLarge` — `"301s"` returns error containing `"out of range"`.
  - `TestParseTimeoutParam_NotDuration` — `"abc"` returns error containing `"invalid"`.
- [ ] **Implement** `parseTimeoutParam`:
  ```go
  const (
      minTimeout = 1 * time.Second
      maxTimeout = 300 * time.Second
  )

  // parseTimeoutParam parses a ?timeout=<duration> query value. An empty string
  // returns (0, nil) — callers substitute their default. Out-of-range or
  // malformed values return an error.
  func parseTimeoutParam(s string) (time.Duration, error) {
      if s == "" { return 0, nil }
      d, err := time.ParseDuration(s)
      if err != nil { return 0, fmt.Errorf("invalid timeout %q: %w", s, err) }
      if d < minTimeout || d > maxTimeout {
          return 0, fmt.Errorf("timeout %s out of range [%s, %s]", d, minTimeout, maxTimeout)
      }
      return d, nil
  }
  ```
- [ ] Run `go test ./internal/handlers/ -run TestParseTimeoutParam -v` — expect all green.
- [ ] Commit: `feat(mcp): per-call timeout policy + config knobs`.

### Acceptance

- 6 timeout-parser tests pass.
- Default timeout is 30s; ceiling is 300s.
- Config struct has both new fields with env-var bindings.

---

## Task 5: Per-server concurrency limit

**Files:** `internal/mcp/concurrency.go` *(new)*, `internal/mcp/concurrency_test.go` *(new)*, `internal/mcp/hypervisor.go` *(integrate)*

### Steps

- [ ] **Write failing tests** `concurrency_test.go`:
  - `TestConcurrencyLimiter_AcquireRelease` — acquire 4, release 4, acquire 4 again — all succeed.
  - `TestConcurrencyLimiter_FifthBlocks_NonBlockingTry` — acquire 4 then `TryAcquire(name)` returns `false` immediately.
  - `TestConcurrencyLimiter_PerServerIsolation` — server A's limit doesn't affect server B (both at default 4).
  - `TestConcurrencyLimiter_PerServerOverride` — set `maxConcurrency=1` for `"heavy"`; first acquire succeeds, second `TryAcquire("heavy")` returns false; meanwhile `"light"` still allows 4.
  - `TestConcurrencyLimiter_ReleaseUnknown_Noop` — releasing a name never acquired is a no-op (don't panic).
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `internal/mcp/concurrency.go`:
  ```go
  package mcp

  import "sync"

  // concurrencyLimiter is a per-MCP-server in-flight call limiter. Each server
  // has its own semaphore sized to either its config override or the global
  // default. Acquire is non-blocking (TryAcquire): if the slot is taken,
  // callers should fail fast with CONCURRENCY_LIMIT rather than queueing.
  type concurrencyLimiter struct {
      mu             sync.Mutex
      defaultLimit   int
      overrides      map[string]int     // serverName → limit
      inFlight       map[string]int     // serverName → current count
  }

  func newConcurrencyLimiter(defaultLimit int) *concurrencyLimiter {
      if defaultLimit < 1 { defaultLimit = 1 }
      return &concurrencyLimiter{
          defaultLimit: defaultLimit,
          overrides:    make(map[string]int),
          inFlight:     make(map[string]int),
      }
  }

  func (l *concurrencyLimiter) SetOverride(name string, limit int) {
      l.mu.Lock(); defer l.mu.Unlock()
      if limit < 1 { delete(l.overrides, name); return }
      l.overrides[name] = limit
  }

  func (l *concurrencyLimiter) ClearOverride(name string) {
      l.mu.Lock(); defer l.mu.Unlock()
      delete(l.overrides, name)
      delete(l.inFlight, name)
  }

  func (l *concurrencyLimiter) TryAcquire(name string) bool {
      l.mu.Lock(); defer l.mu.Unlock()
      limit, ok := l.overrides[name]
      if !ok { limit = l.defaultLimit }
      if l.inFlight[name] >= limit {
          return false
      }
      l.inFlight[name]++
      return true
  }

  func (l *concurrencyLimiter) Release(name string) {
      l.mu.Lock(); defer l.mu.Unlock()
      if l.inFlight[name] > 0 { l.inFlight[name]-- }
  }
  ```
- [ ] **Wire into Hypervisor**:
  - Add `limiter *concurrencyLimiter` field on `Hypervisor`.
  - In `newHypervisor(cfg)`: `h.limiter = newConcurrencyLimiter(cfg.MCPCallConcurrencyPerServer)`.
  - In `startServerLocked` after success: if `srv.Hermind != nil && srv.Hermind.MaxConcurrency != nil`, call `h.limiter.SetOverride(srv.Name, *srv.Hermind.MaxConcurrency)`.
  - In `pruneServerLocked`: `h.limiter.ClearOverride(name)`.
  - Add to `HermindOptions`:
    ```go
    MaxConcurrency *int `json:"maxConcurrency,omitempty"`
    ```
  - Expose to handler via a tiny method:
    ```go
    func (h *Hypervisor) TryAcquireCall(server string) bool { return h.limiter.TryAcquire(server) }
    func (h *Hypervisor) ReleaseCall(server string)         { h.limiter.Release(server) }
    ```
  - Service facade:
    ```go
    func (s *MCPService) TryAcquireCall(server string) bool { return s.hv.TryAcquireCall(server) }
    func (s *MCPService) ReleaseCall(server string)         { s.hv.ReleaseCall(server) }
    ```
- [ ] Run `go test ./internal/mcp/ -run TestConcurrencyLimiter -v -count=1 -race` — green.
- [ ] Run `go test ./internal/mcp/ -v -count=1 -race` — full package green.
- [ ] Commit: `feat(mcp): per-server concurrency limiter`.

### Acceptance

- 5 limiter tests pass under `-race`.
- Override field `maxConcurrency` round-trips through config JSON.
- `Hypervisor.TryAcquireCall / ReleaseCall` exposed via service facade.

---

## Task 6: CallTool handler rewrite

**Files:** `internal/handlers/mcp.go` *(modify)*, `internal/handlers/mcp_test.go` *(extend)*, `cmd/server/main.go` *(pass eventLogSvc)*

### Steps

- [ ] **Update** `MCPHandler` struct to carry the dependencies:
  ```go
  type MCPHandler struct {
      svc      *services.MCPService
      eventLog *services.EventLogService
      cfg      *config.Config
  }

  func NewMCPHandler(svc *services.MCPService, eventLog *services.EventLogService, cfg *config.Config) *MCPHandler {
      return &MCPHandler{svc: svc, eventLog: eventLog, cfg: cfg}
  }
  ```
  Update `RegisterMCPRoutes` signature:
  ```go
  func RegisterMCPRoutes(r *gin.RouterGroup, authSvc *services.AuthService, svc *services.MCPService, eventLog *services.EventLogService, cfg *config.Config) {
      h := NewMCPHandler(svc, eventLog, cfg)
      // ... unchanged routes ...
  }
  ```
- [ ] **Update** `main.go` to pass the new args:
  ```go
  handlers.RegisterMCPRoutes(api, authSvc, mcpSvc, eventLogSvc, cfg)
  ```
- [ ] **Write failing tests** (extend `mcp_test.go`):
  - `TestMCPHandler_CallTool_BodyTooLarge` — send 11 MiB body, expect 413 + `errorCode: "BODY_TOO_LARGE"`.
  - `TestMCPHandler_CallTool_MalformedJSON` — body `"not json"`, expect 400 + `errorCode: "INVALID_BODY"`.
  - `TestMCPHandler_CallTool_ServerNotRunning` — server in config but autoStart=false, expect 404 + `errorCode: "SERVER_NOT_FOUND"`.
  - `TestMCPHandler_CallTool_ToolNotOnServer` — call `unknown-tool`, expect 404 + `errorCode: "TOOL_NOT_FOUND"`.
  - `TestMCPHandler_CallTool_SchemaMismatch` — call `echo` with `{}` (missing required `text`), expect 422 + `errorCode: "ARGS_SCHEMA_MISMATCH"` + `details.errors` listing the violation.
  - `TestMCPHandler_CallTool_TimeoutQueryOutOfRange` — `?timeout=999s`, expect 422 + `errorCode: "INVALID_PARAMS"`.
  - `TestMCPHandler_CallTool_TimeoutExceeded` — tool `slow_echo` configured to take 5s, query `?timeout=1s`, expect 504 + `errorCode: "CALL_TIMEOUT"`.
  - `TestMCPHandler_CallTool_ConcurrencyLimit` — set per-server override to 1, fire 2 concurrent calls, expect one to return 429 + `errorCode: "CONCURRENCY_LIMIT"`. Use a `sync.WaitGroup` + tool that blocks on a channel for deterministic timing — no `time.Sleep`.
  - `TestMCPHandler_CallTool_TransportError` — mock returns JSON-RPC error, expect 502 + `errorCode: "TRANSPORT_ERROR"`.
  - `TestMCPHandler_CallTool_AuditLog_Success` — successful call writes an `event_logs` row with event=`mcp.call.success` and metadata containing `server`, `tool`, `duration_ms`, `user_id`.
  - `TestMCPHandler_CallTool_AuditLog_Failure` — failed call writes `mcp.call.failed` with `errorCode` in metadata.
  - `TestMCPHandler_CallTool_AuditLog_BestEffort` — inject a failing event-log service (DB closed), assert tool call still returns 200 with success body, and a warning is emitted (skip if your test harness doesn't capture logs — make it a `t.Log` smoke).
- [ ] Run tests — expect compile errors.
- [ ] **Rewrite** `CallTool` handler:
  ```go
  const maxCallBodyBytes = 10 << 20 // 10 MiB

  type mcpCallToolBody struct {
      Arguments map[string]any `json:"arguments"`
  }

  func (h *MCPHandler) CallTool(c *gin.Context) {
      server := c.Param("name")
      tool := c.Param("tool")
      if server == "" || tool == "" {
          respondCodedError(c, mcp.CodeInvalidParams, "server name and tool are required", nil)
          return
      }

      // 1. Body size cap
      c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCallBodyBytes)
      var body mcpCallToolBody
      if err := c.ShouldBindJSON(&body); err != nil {
          if strings.Contains(err.Error(), "http: request body too large") {
              respondCodedError(c, mcp.CodeBodyTooLarge, "request body exceeds 10 MiB cap", nil)
              return
          }
          respondCodedError(c, mcp.CodeInvalidBody, "invalid JSON body: "+err.Error(), nil)
          return
      }
      if body.Arguments == nil {
          body.Arguments = map[string]any{}
      }

      // 2. Per-call timeout
      timeout, err := parseTimeoutParam(c.Query("timeout"))
      if err != nil {
          respondCodedError(c, mcp.CodeInvalidParams, err.Error(), nil)
          return
      }
      if timeout == 0 {
          timeout = h.cfg.MCPCallTimeoutDefault
          if timeout == 0 { timeout = 30 * time.Second }
      }

      // 3. Schema lookup & validation
      toolSchema, err := h.svc.GetToolSchema(server, tool)
      if err != nil {
          switch {
          case errors.Is(err, mcp.ErrServerNotFound):
              respondCodedError(c, mcp.CodeServerNotFound, err.Error(), nil)
          case errors.Is(err, mcp.ErrToolNotFound):
              respondCodedError(c, mcp.CodeToolNotFound, err.Error(), nil)
          default:
              respondCodedError(c, mcp.CodeInternalError, err.Error(), nil)
          }
          return
      }
      if err := validateArgsAgainstSchema(body.Arguments, toolSchema.InputSchema); err != nil {
          respondCodedError(c, mcp.CodeArgsSchemaMismatch, err.Error(), gin.H{
              "schema_url": fmt.Sprintf("inputSchema of tool %s/%s", server, tool),
          })
          return
      }

      // 4. Concurrency gate
      if !h.svc.TryAcquireCall(server) {
          h.logAuditAsync(c, "mcp.call.rejected", server, tool, 0, mcp.CodeConcurrencyLimit, nil)
          respondCodedError(c, mcp.CodeConcurrencyLimit, "per-server in-flight cap reached, retry later", nil)
          return
      }
      defer h.svc.ReleaseCall(server)

      // 5. Dispatch with deadline
      ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
      defer cancel()
      start := time.Now()
      result, err := h.svc.CallTool(ctx, server, tool, body.Arguments)
      dur := time.Since(start)

      if err != nil {
          if errors.Is(err, context.DeadlineExceeded) {
              h.logAuditAsync(c, "mcp.call.failed", server, tool, dur, mcp.CodeCallTimeout, nil)
              respondCodedError(c, mcp.CodeCallTimeout, fmt.Sprintf("call exceeded %s", timeout), nil)
              return
          }
          h.logAuditAsync(c, "mcp.call.failed", server, tool, dur, mcp.CodeTransportError, gin.H{
              "transport_error": err.Error(),
          })
          respondCodedError(c, mcp.CodeTransportError, err.Error(), nil)
          return
      }

      h.logAuditAsync(c, "mcp.call.success", server, tool, dur, "", nil)
      respondCallSuccess(c, result)
  }

  func (h *MCPHandler) logAuditAsync(c *gin.Context, event, server, tool string, dur time.Duration, code mcp.ErrorCode, extra gin.H) {
      if h.eventLog == nil { return }
      userVal, _ := c.Get("user")
      var userID *int
      if u, ok := userVal.(*models.User); ok && u != nil { userID = &u.ID }
      meta := gin.H{
          "server":      server,
          "tool":        tool,
          "duration_ms": dur.Milliseconds(),
      }
      if code != "" { meta["error_code"] = string(code) }
      for k, v := range extra { meta[k] = v }
      go func() {
          // Detach from request ctx so a client disconnect doesn't drop the log.
          ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
          defer cancel()
          if err := h.eventLog.LogEvent(ctx, event, meta, userID); err != nil {
              mlog.Warn("mcp audit log failed", mlog.String("event", event), mlog.Err(err))
          }
      }()
  }
  ```
- [ ] Run `go test ./internal/handlers/ -run TestMCPHandler_CallTool -v -count=1 -race` — expect all green.
- [ ] Run `go test ./... -count=1 -race` — full repo green.
- [ ] Commit: `feat(mcp): production-grade CallTool with audit + validation + limits`.

### Acceptance

- 12 new handler tests pass under `-race`.
- All 10 documented error codes are exercised by at least one test.
- Audit log entries materialise in `event_logs` rows; failure to write doesn't block the call.
- Concurrency test uses channel-synchronised tool handler — no `time.Sleep` flake.

---

## Post-PR checklist

- [ ] `go test ./... -count=1 -race` reports all green.
- [ ] `go vet ./...` clean.
- [ ] `go.mod` has `github.com/xeipuuv/gojsonschema v1.2.0` as a direct `require`.
- [ ] `config.Config` has `MCPCallTimeoutDefault` and `MCPCallConcurrencyPerServer` with documented env vars.
- [ ] All 10 `ErrorCode` constants are present, tested, and documented in `internal/mcp/errors.go`.
- [ ] Update `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` — append a §12 "PR-D delivered: tool-call hardening" with a bullet list of the 6 new capabilities.
- [ ] Update the README/operator notes (or this design's §10) with the documented env vars + per-server `maxConcurrency` field example.
- [ ] No goroutine leaks: confirm via the existing `runtime.NumGoroutine` guard in transport tests.
- [ ] Audit log table receives entries during a manual smoke test (`SELECT event, metadata FROM event_logs WHERE event LIKE 'mcp.%' ORDER BY occurred_at DESC LIMIT 10`).

---

## Risk notes

- **Audit log write amplification** — every tool call writes one row. A chatty agent hammering MCP at 100 rps generates 8.6M rows/day. If this becomes a problem, add sampling (e.g. log only failures + 1% of successes). Out of scope for PR-D.
- **`gojsonschema` v1.2.0 last updated 2020** — library is mature, no known bugs blocking us, but it's effectively maintenance-mode. If we hit a JSON Schema draft 2020-12 limitation, swap for `santhosh-tekuri/jsonschema/v5` in a follow-up (drop-in API similar).
- **Concurrency limiter is process-local** — multi-replica deployments won't share the cap. Acceptable until horizontal scaling appears; then move to a Redis-backed limiter.
- **Schema cache is stale by design** — a server that hot-swaps its tool catalog at runtime won't surface new tools until the operator toggles it. Document in operator notes.
- **`http.MaxBytesReader` bounds the JSON parser's input** — but if the JSON parser doesn't read to the end of the body, the cap is silently never enforced. The Gin binder reads to EOF, so we're fine; verify with the 11 MiB body test.
- **Per-call timeout doesn't kill the upstream MCP server** — it cancels our context, which propagates to the transport. For HTTP/SSE transports the underlying HTTP request is aborted; for stdio the in-flight JSON-RPC call is cancelled but the child process keeps running. This matches Node semantics.
- **`go func() { ... LogEvent ... }()` in `logAuditAsync`** — fire-and-forget goroutine. Bound at 2s deadline so a DB outage doesn't accumulate goroutines indefinitely. Worst-case under outage: O(reqs/2s) leaked → bounded.
- **`details` field naming** — clients may type-pun against `details.errors` etc. Keep the keys stable; document any breaking change in release notes.
- **Concurrent CallTool ordering vs. audit log ordering** — audit log goroutines are unordered; if strict ordering matters (it usually doesn't for telemetry), the caller should sort by `occurred_at` desc. Document.

---

## Estimate

- Task 1 (error codes + responder): 1 h
- Task 2 (schema cache + GetToolSchema): 2 h
- Task 3 (gojsonschema validation): 1.5 h
- Task 4 (timeout parser + config knobs): 1 h
- Task 5 (concurrency limiter): 2 h
- Task 6 (handler rewrite + 12 tests): 4-5 h

**Total: ~11-13 hours**, i.e. 1.5 working days. Single-track delivery is the natural path (handler rewrite consumes everything that came before).
