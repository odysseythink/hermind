# MCP Hypervisor PR-B — Stdio Transport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the stdio transport for MCP servers and wire it into the Hypervisor lifecycle. After PR-B, `GET /api/mcp-servers/list` returns real `running`/`tools`/`process.pid` data; `POST /api/mcp-servers/toggle` actually starts/stops subprocesses; `POST /api/mcp/:name/tools/:tool/call` dispatches to live MCP servers. HTTP and SSE transports remain `ErrTransportNotImplemented` until PR-C.

**Architecture:** `internal/mcp/transport_stdio.go` wraps an MCP Go SDK's stdio client with our `Transport` interface. `internal/mcp/shell_env.go` ports Node's `patchShellEnvironmentPath`. `Hypervisor.Boot/Servers/Toggle/Delete/CallTool/PruneAll` graduate from stubs to real implementations. A self-built Go echo MCP server fixture (`internal/mcp/testdata/echo-mcp/`) ships in-repo so CI is hermetic — no npm or external services. `main.go` gains a graceful shutdown handler that calls `mcpHyp.PruneAll()` before exit.

**Tech Stack:** Go 1.22+, `os/exec`, MCP Go SDK (selected in Task 0), syscall signals (POSIX), testify.

**Source spec:** `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §3-4, §6, §10.

**Reference Node implementation:**
- `server/utils/MCP/hypervisor/index.js` (#startMCPServer, pruneMCPServer, #buildMCPServerENV, #parseServerType, #validateServerDefinitionByType, bootMCPServers, 30s connection timeout)
- `server/utils/helpers/shell.js` (patchShellEnvironmentPath — `$SHELL -ic env`)
- `server/utils/MCP/index.js` (servers(), toggleServerStatus(), deleteServer())

**Prerequisite:** PR-A landed (`mcp` package skeleton + REST + handler + service facade).

---

## Pre-task: Read this section once before starting

### Existing Go surface (from PR-A)

- `mcp.Transport` interface (`mcp/transport.go`) — already defined: `Connect/Close/Ping/ListTools/CallTool/ProcessInfo`. PR-B implements stdio behind it.
- `mcp.newTransport(srv) (Transport, error)` (`mcp/transport.go`) — currently returns `ErrTransportNotImplemented`. PR-B dispatches by `parseServerType()`: `"stdio"` → stdio, others → still `ErrTransportNotImplemented`.
- `mcp.Hypervisor` (`mcp/hypervisor.go`) — `mcps map[string]*activeClient` exists but is unused; PR-B populates and consumes it.
- `mcp.ServerView.Process *ProcessInfo` — populated by stdio transport's `ProcessInfo()`.
- `mcp.Config.GetSuppressedTools(name) []string` — call in `Servers()` after `ListTools` to filter (Node parity).
- `services.MCPService` — no signature changes; it remains a thin facade.
- `handlers/mcp.go` — no changes; the route handlers already invoke `svc.Servers/Toggle/Delete/ToggleTool/CallTool` which now return real data.

### Mental model

Stdio MCP server = a child process spawned via `exec.Command(command, args...)` with `Stdin`/`Stdout` connected to an MCP JSON-RPC framing codec (handled by the SDK). The Go-side client writes JSON-RPC requests to stdin and reads responses from stdout. Stderr is line-buffered to our logger for debugging. SIGTERM with 3s grace + SIGKILL for cleanup.

### Methods to ship (PR-B scope)

| # | Owner | Behaviour change |
|---|---|---|
| 1 | `mcp.shellEnv(ctx) map[string]string` *(new)* | Runs `$SHELL -ic env` (5s timeout), parses, falls back to `os.Environ()` on error / Windows / docker-sans-shell |
| 2 | `mcp.buildServerEnv(srv, baseEnv) []string` *(new)* | Layered merge: shell env → docker overrides (if `ANYTHING_LLM_RUNTIME=docker`) → user's `srv.Env` (highest priority); returns `KEY=VAL` slice for `exec.Cmd.Env` |
| 3 | `mcp.stdioTransport` *(new struct)* | Implements `Transport` interface via chosen MCP SDK |
| 4 | `mcp.newTransport(srv)` *(replace)* | Dispatch by `parseServerType()` — stdio is real, others still `ErrTransportNotImplemented` |
| 5 | `mcp.Hypervisor.Boot(ctx)` *(replace)* | Iterate configs, skip `autoStart=false`, start each with 30s timeout, store results in `mcpLoadingResults` |
| 6 | `mcp.Hypervisor.Servers(ctx)` *(replace)* | For each entry in `mcpLoadingResults`: if running, `Ping` + `ListTools` (filtered by suppression); else return failed view |
| 7 | `mcp.Hypervisor.ToggleServer(ctx, name)` *(replace)* | If online: prune. If offline: start. Returns `(success bool, error)` |
| 8 | `mcp.Hypervisor.DeleteServer(ctx, name)` *(replace)* | If running, prune first; always remove from config |
| 9 | `mcp.Hypervisor.CallTool(ctx, srv, tool, args)` *(replace)* | Dispatch to running transport; error if server not booted |
| 10 | `mcp.Hypervisor.PruneAll()` *(replace)* | SIGTERM each child, 3s grace, then SIGKILL; close transports; clear `mcps` |
| 11 | `mcp.Hypervisor.startServer(ctx, srv)` *(new private)* | Build transport, `Connect` with 30s context, store in `mcps`, record `LoadResult` |
| 12 | `mcp.Hypervisor.pruneServer(name)` *(new private)* | Close one server's transport with grace; update `mcpLoadingResults` |
| 13 | `main.go` graceful shutdown *(new)* | `signal.Notify(SIGINT, SIGTERM)` → `mcpHyp.PruneAll()` → `srv.Shutdown(ctx)` |

### Out of scope (explicit)

- **HTTP / SSE transports** — PR-C. `newTransport()` returns `ErrTransportNotImplemented` for non-stdio kinds.
- **`ToolPlugin` interface / `ActiveServers()` / `ToolsAsPlugins()`** — PR-E.
- **Per-tool ACL beyond suppression** — out of scope; admin can call any tool via `CallTool`.
- **Streaming tool results** — MCP tool calls in PR-B are request/response only; long-running streaming tools (if SDK supports them) get fully buffered and returned synchronously.
- **Concurrent boot of multiple servers** — Node boots them sequentially via `await`; Go can do the same. **Do not** introduce goroutine fan-out yet (debuggability > throughput for now).
- **Per-server CPU/RAM limits / cgroups** — operator concern, out of scope.

### Data invariants

- `Hypervisor.mcps` and `Hypervisor.results` are guarded by `Hypervisor.mu`. Public methods must lock; private `*Locked` variants assume held lock.
- `mcps[name]` presence == "this server has been booted at least once and not pruned". Use `len(h.mcps) > 0` as the "already booted" signal (drop the `booted bool` from PR-A).
- Once a server is in `mcps`, its `transport` is guaranteed non-nil. `Close()` is idempotent.
- `mcpLoadingResults[name]` may be set without `mcps[name]` being set (failed boot). Conversely, every entry in `mcps` has a corresponding `mcpLoadingResults[name]` with status="success".
- Server log lines from stdin/stdout/stderr go to `mlog` with `mlog.String("mcp", name)` field — never `fmt.Println`.
- SIGTERM is the only graceful kill signal we send to children. SIGKILL is reserved for the 3-second grace fallback.
- The 30-second connection timeout is `context.WithTimeout` on `Connect`; do not introduce a second timer.
- **Race-free shutdown**: after `signal.Notify` fires, the order is (1) stop accepting new HTTP, (2) PruneAll, (3) Shutdown the HTTP server. Reverse order risks new tool-call requests hitting half-killed transports.

### TDD discipline

Each task follows: write failing test → run + confirm fail → implement → run + confirm pass → commit. Integration tests that spawn the echo fixture must use `t.Cleanup` to call `pruneServer` so test failures don't leak processes. Echo fixture build happens once per test run via `TestMain`.

### Test fixture infrastructure

```go
// internal/mcp/testdata/echo-mcp/main.go — built by TestMain
// Implements MCP server-side protocol via the chosen SDK.
// Exposes three tools:
//   echo(text string) -> text                       — golden path
//   add(a int, b int) -> a+b                        — typed args
//   slow_echo(text string, delay_ms int) -> text    — for timeout/cancel tests

// internal/mcp/testmain_test.go
func TestMain(m *testing.M) {
    // Build echo-mcp binary once into a temp path
    binPath := buildEchoMCP() // go build into t.TempDir()
    os.Setenv("MCP_ECHO_BIN", binPath)
    code := m.Run()
    os.Exit(code)
}
```

Tests reference the binary via `os.Getenv("MCP_ECHO_BIN")` and pass it as `ServerConfig.Command`.

---

## Task 0: SDK selection spike + go.mod update

**Files:** `.gpowers/decisions/2026-05-26-mcp-go-sdk.md` (new), `go.mod`/`go.sum` (updated)

**Test:** none (research + decision artefact).

### Steps

- [ ] Read the two candidate SDKs' Go docs and recent commits:
  - Candidate A: `github.com/modelcontextprotocol/go-sdk` (official; pkg.go.dev for current API)
  - Candidate B: `github.com/mark3labs/mcp-go` (community)
- [ ] In a scratch worktree, write a 30-line stdio client using each SDK that:
  1. Spawns `node -e 'console.log("not actually a mcp server, just verifying lib compiles")'`
  2. Compiles cleanly
- [ ] Score each candidate on:
  - **Client side completeness**: stdio + HTTP + SSE transports?
  - **Server side completeness**: needed for echo fixture
  - **API stability**: pre-v1 churn risk
  - **Maintenance cadence**: commits in last 90 days
  - **Dependency footprint**: number of transitive deps added
- [ ] Pick one. Record decision in `.gpowers/decisions/2026-05-26-mcp-go-sdk.md`:
  ```markdown
  # MCP Go SDK Selection (2026-05-26)
  **Chosen:** github.com/<owner>/<pkg> v<version>
  **Considered:**
    - github.com/modelcontextprotocol/go-sdk v<version>
    - github.com/mark3labs/mcp-go v<version>
  **Why:** <2-3 bullet points covering completeness / stability / fit>
  **Tradeoffs accepted:** <e.g. "API may break in 0.x → pin to exact version, revisit at 1.0">
  **Fallback plan:** <which alternative we'd swap to and at what trigger>
  ```
- [ ] Add the chosen SDK to `go.mod` with an exact version pin (no `^` ranges):
  ```bash
  go get github.com/<owner>/<pkg>@v<exact>
  go mod tidy
  ```
- [ ] Run `go build ./...` — expect success.
- [ ] Commit: `chore(mcp): adopt <pkg> v<version> for stdio transport`. Reference the decision doc in the commit body.

### Acceptance

- `.gpowers/decisions/2026-05-26-mcp-go-sdk.md` exists with all sections filled in.
- `go.mod` has the chosen SDK with exact version.
- `go build ./...` and `go vet ./...` pass.
- No other code changes in this commit.

---

## Task 1: Shell environment patch

**File:** `internal/mcp/shell_env.go`, `internal/mcp/shell_env_test.go`

### Steps

- [ ] **Write failing test** `shell_env_test.go`:
  - `TestShellEnv_FallbackOnEmptyShellVar` — clear `SHELL` env, assert returns `os.Environ()` map non-empty.
  - `TestShellEnv_FallbackOnShellError` — set `SHELL=/nonexistent/bin`, assert returns `os.Environ()` non-empty (no panic).
  - `TestShellEnv_Timeout` — set `SHELL` to a script that sleeps 10s (`testdata/slow-shell.sh`), assert returns within 6s with fallback env.
  - `TestParseEnvOutput_Standard` — feed `"PATH=/bin\nFOO=bar\n"`, assert map `{"PATH":"/bin", "FOO":"bar"}`.
  - `TestParseEnvOutput_MultilineValue` — env can produce multi-line values via `\n` escape; for now assert the simple case (single `=` per line).
  - `TestParseEnvOutput_SkipEmpty` — input with blank lines is tolerated.
  - `TestBuildServerEnv_UserEnvOverridesShell` — shell env `PATH=/usr/bin`, server `Env={"PATH":"/custom/bin"}`, assert returned slice has `PATH=/custom/bin`.
  - `TestBuildServerEnv_DockerOverrides` — `ANYTHING_LLM_RUNTIME=docker`, shell env empty, assert `PATH` defaulted to `/usr/local/bin:/usr/bin:/bin` and `NODE_PATH=/usr/local/lib/node_modules`.
  - `TestBuildServerEnv_PassthroughOSEnv` — server has no `Env`, assert returned slice contains all caller's env vars (transitively from `os.Environ`).
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `shell_env.go`:
  ```go
  package mcp

  import (
      "context"
      "os"
      "os/exec"
      "runtime"
      "strings"
      "time"
  )

  // shellEnv runs "$SHELL -ic env" to capture the user's interactive shell
  // environment (PATH, NODE_PATH, language paths, etc) so subprocess MCP
  // servers see the same toolchain a human shell would. On error or unsupported
  // platforms, falls back to os.Environ().
  func shellEnv(parent context.Context) map[string]string {
      if runtime.GOOS == "windows" {
          return osEnvMap()
      }
      shell := os.Getenv("SHELL")
      if shell == "" {
          return osEnvMap()
      }
      ctx, cancel := context.WithTimeout(parent, 5*time.Second)
      defer cancel()
      cmd := exec.CommandContext(ctx, shell, "-ic", "env")
      out, err := cmd.Output()
      if err != nil {
          return osEnvMap()
      }
      return parseEnvOutput(string(out))
  }

  func osEnvMap() map[string]string {
      m := make(map[string]string, len(os.Environ()))
      for _, kv := range os.Environ() {
          if i := strings.IndexByte(kv, '='); i > 0 {
              m[kv[:i]] = kv[i+1:]
          }
      }
      return m
  }

  func parseEnvOutput(s string) map[string]string {
      m := make(map[string]string)
      for _, line := range strings.Split(s, "\n") {
          if line == "" {
              continue
          }
          i := strings.IndexByte(line, '=')
          if i <= 0 {
              continue
          }
          m[line[:i]] = line[i+1:]
      }
      return m
  }

  // buildServerEnv produces the KEY=VAL slice for exec.Cmd.Env, layering:
  // 1. base shell environment (or os.Environ on fallback)
  // 2. docker hardcoded defaults (if ANYTHING_LLM_RUNTIME=docker)
  // 3. user-specified server.Env (highest priority)
  func buildServerEnv(srv *ServerConfig) []string {
      base := shellEnv(context.Background())
      if base["PATH"] == "" {
          base["PATH"] = "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
      }
      if base["NODE_PATH"] == "" {
          base["NODE_PATH"] = "/usr/local/lib/node_modules"
      }
      if os.Getenv("ANYTHING_LLM_RUNTIME") == "docker" {
          if base["NODE_PATH"] == "" {
              base["NODE_PATH"] = "/usr/local/lib/node_modules"
          }
          if base["PATH"] == "" {
              base["PATH"] = "/usr/local/bin:/usr/bin:/bin"
          }
      }
      for k, v := range srv.Env {
          base[k] = v
      }
      out := make([]string, 0, len(base))
      for k, v := range base {
          out = append(out, k+"="+v)
      }
      return out
  }
  ```
- [ ] Create `internal/mcp/testdata/slow-shell.sh`:
  ```bash
  #!/bin/sh
  # Used by TestShellEnv_Timeout to verify 5s budget
  sleep 10
  echo "should-not-be-reached=true"
  ```
  Mark executable: `chmod +x internal/mcp/testdata/slow-shell.sh`.
- [ ] Run `go test ./internal/mcp/ -run TestShellEnv -v` — expect all green.
- [ ] Commit: `feat(mcp): shell environment patch for stdio subprocess env`.

### Acceptance

- 9 unit tests pass on Linux/macOS; Windows skips `TestShellEnv_*` (use `t.Skip("posix-only")` guarded by `runtime.GOOS == "windows"`).
- 5s timeout is respected — test takes <6s.
- Env layering order (shell → docker → user) verified by table-driven test.

---

## Task 2: Echo MCP server fixture

**File:** `internal/mcp/testdata/echo-mcp/main.go`, `internal/mcp/testdata/echo-mcp/go.mod` (optional), `internal/mcp/testmain_test.go`

### Steps

- [ ] Decide based on Task 0's SDK choice whether to:
  - (a) Use the chosen SDK's server-side helpers if available — preferred (≤80 lines).
  - (b) Hand-roll a minimal JSON-RPC server over stdio — fallback (~150 lines).
- [ ] **Implement** `testdata/echo-mcp/main.go` exposing three tools:
  - `echo(text string) -> {content: [{type:"text", text}]}` — returns input verbatim
  - `add(a int, b int) -> {content: [{type:"text", text: "sum=<n>"}]}` — exercises typed args
  - `slow_echo(text string, delay_ms int) -> {content: ...}` — sleeps then echoes; for timeout/cancel tests
  
  Keep server name `"echo-mcp"`, version `"0.0.1"`. Log nothing on stdout (only stdin/stdout for protocol); stderr OK for diagnostics.

  ⚠️ If the SDK requires its module path, place under `internal/mcp/testdata/echo-mcp/` with own `go.mod` to avoid coupling test fixtures to the main module's dependency graph. But if the chosen SDK is already a direct dependency, the fixture can be a plain `package main` under the same module — simpler.

- [ ] **Write** `internal/mcp/testmain_test.go`:
  ```go
  package mcp_test  // or `package mcp` if tests live in-package

  import (
      "os"
      "os/exec"
      "path/filepath"
      "testing"
  )

  func TestMain(m *testing.M) {
      tmp, err := os.MkdirTemp("", "mcp-echo-bin-")
      if err != nil { os.Exit(1) }
      defer os.RemoveAll(tmp)

      binPath := filepath.Join(tmp, "echo-mcp")
      cmd := exec.Command("go", "build", "-o", binPath, "./testdata/echo-mcp")
      cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
      if err := cmd.Run(); err != nil {
          os.Exit(1)
      }
      os.Setenv("MCP_ECHO_BIN", binPath)
      os.Exit(m.Run())
  }
  ```
- [ ] **Smoke test** the fixture standalone (manual, not in CI):
  ```bash
  go build -o /tmp/echo-mcp ./internal/mcp/testdata/echo-mcp
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}' | /tmp/echo-mcp
  # Should print a valid initialize response
  ```
- [ ] Run `go test ./internal/mcp/ -run XXX_NoMatch -v` to verify `TestMain` builds the binary without running any test (exit code 0 expected). Then run a placeholder test that exercises `os.Getenv("MCP_ECHO_BIN")` to confirm the path is propagated.
- [ ] Commit: `test(mcp): add echo MCP server fixture for stdio integration tests`.

### Acceptance

- `go test ./internal/mcp/ -count=1 -run XXX_NoMatch` exits 0.
- `/tmp/echo-mcp` (or temp path) responds to `initialize` request with valid MCP response.
- Fixture takes <300ms to build.
- No external network or npm dependency.

---

## Task 3: Stdio transport

**File:** `internal/mcp/transport_stdio.go`, `internal/mcp/transport_stdio_test.go`

### Steps

- [ ] **Write failing test** `transport_stdio_test.go`:
  - `TestStdioTransport_Connect_Echo` — spawn `MCP_ECHO_BIN`, assert `Connect(ctx)` returns nil within 5s.
  - `TestStdioTransport_Connect_NonexistentCommand` — `Command:"/nonexistent"`, assert error within 1s.
  - `TestStdioTransport_Connect_Timeout` — `Command:"sleep", Args:["10"]` (sleep never initialises MCP), use 1s ctx, assert ctx.DeadlineExceeded.
  - `TestStdioTransport_ListTools` — connect echo, `ListTools(ctx)` returns 3 tools with expected names `[echo, add, slow_echo]`.
  - `TestStdioTransport_CallTool_Echo` — call `echo({text:"hi"})`, assert result contains `"hi"`.
  - `TestStdioTransport_CallTool_Add` — call `add({a:2,b:3})`, assert result contains `"sum=5"`.
  - `TestStdioTransport_CallTool_UnknownTool` — call `nonexistent({})`, assert error.
  - `TestStdioTransport_Ping` — connect echo, `Ping(ctx)` returns true; then `Close()`; assert next `Ping(ctx)` returns false.
  - `TestStdioTransport_ProcessInfo` — connect, assert `ProcessInfo()` non-nil with `PID > 0` and `Cmd` containing `"echo-mcp"`.
  - `TestStdioTransport_Close_Idempotent` — `Close()` twice returns nil both times; child is reaped.
  - `TestStdioTransport_Close_GracefulSIGTERM` — connect, capture child PID, `Close()`, assert PID gone from process table within 4s (3s grace + slack).
  - `TestStdioTransport_Close_KillsRunaway` — use a fixture that ignores SIGTERM (`testdata/sig-ignorer/main.go`); assert SIGKILL fires after 3s and child is reaped.
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `transport_stdio.go`:
  ```go
  package mcp

  import (
      "context"
      "errors"
      "fmt"
      "os/exec"
      "strings"
      "sync"
      "syscall"
      "time"

      sdkclient "github.com/<owner>/<pkg>/client" // placeholder — exact import from Task 0
  )

  type stdioTransport struct {
      cmd       *exec.Cmd
      client    *sdkclient.Client // SDK type — adjust to the chosen SDK's API
      pid       int
      cmdStr    string
      closeOnce sync.Once
      closed    chan struct{}
      mu        sync.RWMutex
  }

  func newStdioTransport(srv *ServerConfig) (Transport, error) {
      if srv.Command == "" {
          return nil, errors.New("stdio transport: missing command")
      }
      return &stdioTransport{
          cmdStr: strings.Join(append([]string{srv.Command}, srv.Args...), " "),
          closed: make(chan struct{}),
          // command + env built lazily in Connect to honour caller's context for env probe
      }, nil
  }

  func (t *stdioTransport) Connect(ctx context.Context) error {
      // Build exec.Cmd
      // NB: use exec.Command (not CommandContext) — we manage lifetime explicitly
      // because CommandContext sends SIGKILL on cancel which bypasses grace.
      cmd := exec.Command(t.srvCommand(), t.srvArgs()...)  // see Note A below
      cmd.Env = buildServerEnv(t.srv())                     // see Note A below
      // Hook stdin/stdout to the SDK's stdio transport
      // ... SDK-specific glue here ...
      t.cmd = cmd
      t.pid = cmd.Process.Pid

      // Wait for MCP handshake within ctx deadline
      done := make(chan error, 1)
      go func() {
          done <- t.client.Initialize(ctx) // SDK-specific call
      }()
      select {
      case err := <-done:
          if err != nil { t.kill(); return err }
          return nil
      case <-ctx.Done():
          t.kill()
          return ctx.Err()
      }
  }

  func (t *stdioTransport) Close() error {
      var err error
      t.closeOnce.Do(func() {
          close(t.closed)
          err = t.gracefulKill()
      })
      return err
  }

  func (t *stdioTransport) gracefulKill() error {
      if t.cmd == nil || t.cmd.Process == nil { return nil }
      _ = t.cmd.Process.Signal(syscall.SIGTERM)
      done := make(chan struct{})
      go func() { _ = t.cmd.Wait(); close(done) }()
      select {
      case <-done:
      case <-time.After(3 * time.Second):
          _ = t.cmd.Process.Signal(syscall.SIGKILL)
          <-done
      }
      return nil
  }

  func (t *stdioTransport) kill() {
      if t.cmd != nil && t.cmd.Process != nil {
          _ = t.cmd.Process.Kill()
      }
  }

  func (t *stdioTransport) Ping(ctx context.Context) bool {
      select {
      case <-t.closed:
          return false
      default:
      }
      // SDK-specific ping (or no-op tool list)
      return t.client.Ping(ctx) == nil
  }

  func (t *stdioTransport) ListTools(ctx context.Context) ([]ToolSchema, error) {
      raw, err := t.client.ListTools(ctx) // SDK type
      if err != nil { return nil, err }
      out := make([]ToolSchema, 0, len(raw))
      for _, r := range raw {
          out = append(out, ToolSchema{
              Name:        r.Name,
              Description: r.Description,
              InputSchema: r.InputSchema, // already json.RawMessage from SDK or marshal explicitly
          })
      }
      return out, nil
  }

  func (t *stdioTransport) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
      return t.client.CallTool(ctx, name, args) // SDK API
  }

  func (t *stdioTransport) ProcessInfo() *ProcessInfo {
      if t.pid == 0 { return nil }
      return &ProcessInfo{PID: t.pid, Cmd: t.cmdStr}
  }
  ```
  > **Note A:** the snippet above is a sketch — the precise SDK glue (stdio reader/writer, Initialize/Ping/CallTool calls, tool schema fields) must follow the SDK chosen in Task 0. Keep the public `Transport` interface stable; all SDK-specific code lives inside `transport_stdio.go`.

- [ ] Create `testdata/sig-ignorer/main.go` — a binary that traps SIGTERM and runs forever (for the SIGKILL fallback test):
  ```go
  package main
  import ("os/signal"; "syscall"; "time")
  func main() {
      c := make(chan os.Signal, 1)
      signal.Notify(c, syscall.SIGTERM)
      go func() { for range c {} }() // swallow
      time.Sleep(60 * time.Second)
  }
  ```
  Build via `TestMain` alongside echo (extend Task 2's TestMain).
- [ ] Run `go test ./internal/mcp/ -run TestStdioTransport -v -count=1` — expect all green.
- [ ] Run `go test ./internal/mcp/ -v -count=1 -race` — full package green under race detector.
- [ ] Commit: `feat(mcp): stdio transport with SIGTERM+grace lifecycle`.

### Acceptance

- 12 stdio transport tests pass.
- `-race` clean.
- SIGKILL fallback verified to fire <4s after Close().
- `ProcessInfo()` returns sensible PID + cmd string.

---

## Task 4: Wire transport factory

**File:** `internal/mcp/transport.go`

### Steps

- [ ] **Write failing test** in `transport_test.go` (NEW):
  - `TestNewTransport_Stdio` — `&ServerConfig{Command:"/usr/bin/true"}` returns non-nil transport + nil error.
  - `TestNewTransport_HTTPReturnsNotImplemented` — `&ServerConfig{Type:"http", URL:"http://x"}` returns `(nil, ErrTransportNotImplemented)`.
  - `TestNewTransport_SSEReturnsNotImplemented` — `&ServerConfig{URL:"http://x"}` (no type, has URL) returns `(nil, ErrTransportNotImplemented)`.
  - `TestNewTransport_InvalidEmpty` — `&ServerConfig{}` returns `(nil, ErrInvalidServerType)`.
- [ ] **Modify** `newTransport`:
  ```go
  func newTransport(srv *ServerConfig) (Transport, error) {
      switch parseServerType(srv) {
      case "stdio":
          return newStdioTransport(srv)
      case "http", "sse":
          return nil, ErrTransportNotImplemented // PR-C wires these
      }
      return nil, ErrInvalidServerType
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestNewTransport -v` — expect all green.
- [ ] Commit: `feat(mcp): transport factory dispatches stdio (HTTP/SSE pending PR-C)`.

### Acceptance

- 4 factory tests pass.
- HTTP/SSE inputs still produce `ErrTransportNotImplemented` (PR-C will replace).

---

## Task 5: Hypervisor lifecycle (Boot, Servers, Toggle, Delete, CallTool, PruneAll)

**Files:** `internal/mcp/hypervisor.go` (HEAVILY MODIFIED), `internal/mcp/hypervisor_test.go` (extend with integration tests)

### Steps

- [ ] **Remove** the `booted bool` field added in PR-A; use `len(h.mcps) > 0` as the boot signal.
- [ ] **Write failing integration tests** in `hypervisor_test.go`:
  - `TestHypervisor_Boot_StartsAutoStartServer` — seed config with `{command: $MCP_ECHO_BIN}`; `Boot(ctx)` → `len(h.mcps)==1`, `mcpLoadingResults["echo"].Status=="success"`.
  - `TestHypervisor_Boot_SkipsAutoStartFalse` — seed `{anythingllm:{autoStart:false}}`; `Boot(ctx)` → `mcps` empty, `mcpLoadingResults["echo"].Status=="failed"` with message containing `"autoStart"`.
  - `TestHypervisor_Boot_FailureContinuesOthers` — seed two servers, one with bogus command; `Boot(ctx)` returns nil; one success, one failed in results.
  - `TestHypervisor_Boot_30sTimeoutOnHangingServer` — seed a `sleep 60` server; use modified timeout (or wait 30s if test budget allows; otherwise inject a smaller timeout via a test hook).
  - `TestHypervisor_Servers_AfterBoot` — boot echo, `Servers(ctx)` returns view with `Running:true, Tools:[echo, add, slow_echo], Process.PID > 0, Error: nil`.
  - `TestHypervisor_Servers_FiltersSuppressedTools` — seed `{anythingllm:{suppressedTools:["add"]}}`; boot; `Servers(ctx)` returns tools `[echo, slow_echo]` (add filtered).
  - `TestHypervisor_ToggleServer_Off` — boot echo, `ToggleServer(ctx, "echo")` returns `(true, nil)`, child process gone, `mcps["echo"]` absent.
  - `TestHypervisor_ToggleServer_On` — start with autoStart=false, `ToggleServer(ctx, "echo")` returns `(true, nil)`, child running.
  - `TestHypervisor_ToggleServer_UnknownName` — `ToggleServer(ctx, "ghost")` returns `(false, error)` wrapping `ErrServerNotFound`.
  - `TestHypervisor_DeleteServer_KillsAndRemoves` — boot, `DeleteServer(ctx, "echo")` returns `(true, nil)`, child gone, config file no longer contains key.
  - `TestHypervisor_CallTool_Echo` — boot, `CallTool(ctx, "echo", "echo", {"text":"hi"})` returns result containing `"hi"`.
  - `TestHypervisor_CallTool_ServerNotRunning` — config exists but autoStart=false, `CallTool` returns error wrapping `ErrServerNotFound` (Node parity treats this the same).
  - `TestHypervisor_CallTool_UnknownServer` — returns `ErrServerNotFound`.
  - `TestHypervisor_PruneAll_KillsEverything` — boot 2 servers, capture PIDs, `PruneAll()` → both child processes gone within 4s.
  - `TestHypervisor_Reload_RestartsAfterPrune` — boot, prune, `Boot(ctx)` again → fresh start, `mcps` repopulated.
- [ ] Run tests — expect compile errors.
- [ ] **Replace** `Boot`, `Servers`, `ToggleServer`, `DeleteServer`, `CallTool`, `PruneAll`. Add private `startServerLocked(ctx, srv)`, `pruneServerLocked(name)`.

  Key snippets:
  ```go
  const connectionTimeout = 30 * time.Second

  func (h *Hypervisor) Boot(ctx context.Context) error {
      h.mu.Lock()
      defer h.mu.Unlock()
      if len(h.mcps) > 0 {
          return nil // already booted
      }
      if err := h.file.Ensure(); err != nil { return err }
      servers, err := h.file.Load()
      if err != nil { return err }
      for i := range servers {
          srv := &servers[i]
          if srv.AnythingLLM != nil && srv.AnythingLLM.AutoStart != nil && !*srv.AnythingLLM.AutoStart {
              h.results[srv.Name] = LoadResult{
                  Status:  "failed",
                  Message: fmt.Sprintf("MCP server %s has anythingllm.autoStart=false, boot skipped", srv.Name),
              }
              continue
          }
          h.startServerLocked(ctx, srv)
      }
      return nil
  }

  func (h *Hypervisor) startServerLocked(parent context.Context, srv *ServerConfig) {
      ctx, cancel := context.WithTimeout(parent, connectionTimeout)
      defer cancel()
      transport, err := newTransport(srv)
      if err != nil {
          h.results[srv.Name] = LoadResult{Status: "failed", Message: err.Error()}
          return
      }
      if err := transport.Connect(ctx); err != nil {
          _ = transport.Close()
          h.results[srv.Name] = LoadResult{Status: "failed", Message: err.Error()}
          return
      }
      h.mcps[srv.Name] = &activeClient{transport: transport, process: transport.ProcessInfo()}
      h.results[srv.Name] = LoadResult{Status: "success", Message: fmt.Sprintf("Successfully connected to MCP server: %s", srv.Name)}
  }

  func (h *Hypervisor) Servers(ctx context.Context) ([]ServerView, error) {
      if err := h.file.Ensure(); err != nil { return nil, err }
      configs, err := h.file.Load()
      if err != nil { return nil, err }
      h.mu.RLock()
      defer h.mu.RUnlock()
      out := make([]ServerView, 0, len(configs))
      for i := range configs {
          srv := configs[i]
          view := ServerView{Name: srv.Name, Config: &srv, Tools: []ToolSchema{}}
          result, hasResult := h.results[srv.Name]
          client, hasClient := h.mcps[srv.Name]
          switch {
          case hasResult && result.Status == "failed":
              msg := result.Message
              view.Error = &msg
              view.Running = false
          case hasClient:
              online := client.transport.Ping(ctx)
              view.Running = online
              view.Process = client.process
              if online {
                  if tools, err := client.transport.ListTools(ctx); err == nil {
                      suppressed := stringSet(srv.SuppressedTools())
                      for _, t := range tools {
                          if !suppressed[t.Name] {
                              view.Tools = append(view.Tools, t)
                          }
                      }
                  }
              }
          default:
              // Never booted (e.g. cold list before Boot)
              transportErr := ErrTransportNotImplemented.Error()
              view.Error = &transportErr
          }
          out = append(out, view)
      }
      return out, nil
  }

  func (h *Hypervisor) ToggleServer(ctx context.Context, name string) (bool, error) {
      h.mu.Lock()
      defer h.mu.Unlock()
      servers, err := h.file.Load()
      if err != nil { return false, err }
      var found *ServerConfig
      for i := range servers {
          if servers[i].Name == name { found = &servers[i]; break }
      }
      if found == nil { return false, fmt.Errorf("%w: %s", ErrServerNotFound, name) }
      if _, on := h.mcps[name]; on {
          h.pruneServerLocked(name)
          return true, nil
      }
      h.startServerLocked(ctx, found)
      result := h.results[name]
      if result.Status != "success" {
          return false, errors.New(result.Message)
      }
      return true, nil
  }

  func (h *Hypervisor) DeleteServer(ctx context.Context, name string) (bool, error) {
      h.mu.Lock()
      h.pruneServerLocked(name)
      h.mu.Unlock()
      ok, err := h.file.RemoveServer(name)
      if err != nil { return false, err }
      if !ok { return false, fmt.Errorf("%w: %s", ErrServerNotFound, name) }
      return true, nil
  }

  func (h *Hypervisor) CallTool(ctx context.Context, name, tool string, args map[string]any) (any, error) {
      h.mu.RLock()
      client, ok := h.mcps[name]
      h.mu.RUnlock()
      if !ok {
          return nil, fmt.Errorf("%w: %s (not running)", ErrServerNotFound, name)
      }
      return client.transport.CallTool(ctx, tool, args)
  }

  func (h *Hypervisor) PruneAll() error {
      h.mu.Lock()
      defer h.mu.Unlock()
      for name := range h.mcps {
          h.pruneServerLocked(name)
      }
      return nil
  }

  func (h *Hypervisor) pruneServerLocked(name string) {
      client, ok := h.mcps[name]
      if !ok { return }
      _ = client.transport.Close()
      delete(h.mcps, name)
      h.results[name] = LoadResult{Status: "failed", Message: "Server was stopped manually by the administrator."}
  }

  func stringSet(xs []string) map[string]struct{} {
      m := make(map[string]struct{}, len(xs))
      for _, x := range xs { m[x] = struct{}{} }
      return m
  }
  ```
  > Add a `SuppressedTools()` helper on `ServerConfig` returning `c.AnythingLLM.SuppressedTools` (nil-safe) — keeps `Servers()` readable.

- [ ] **Make the 30s timeout injectable** for tests:
  ```go
  var connectionTimeoutForTest = 0 * time.Second
  func effectiveConnectionTimeout() time.Duration {
      if connectionTimeoutForTest > 0 { return connectionTimeoutForTest }
      return connectionTimeout
  }
  ```
  Tests that want a 2s timeout for the hanging-server case set `connectionTimeoutForTest = 2 * time.Second` and reset in `t.Cleanup`.
- [ ] Run `go test ./internal/mcp/ -v -count=1 -race` — expect all green.
- [ ] Update `handlers/mcp_test.go` (from PR-A) to add 3 new tests that exercise the now-real toggle/call-tool paths via HTTP:
  - `TestMCPHandler_ToggleServer_StartsRealProcess`
  - `TestMCPHandler_CallTool_EchoTool_E2E`
  - `TestMCPHandler_DeleteServer_KillsRunningProcess`
- [ ] Run `go test ./internal/handlers/ -run TestMCPHandler -v` — all green including the new e2e tests.
- [ ] Commit: `feat(mcp): hypervisor lifecycle backed by stdio transport`.

### Acceptance

- 15 hypervisor integration tests + 3 handler e2e tests pass.
- `-race` clean.
- Connection timeout overrideable in tests.
- Tool suppression filtering verified end-to-end.
- `PruneAll` reaps all children within 4s.

---

## Task 6: Graceful shutdown in main.go

**File:** `cmd/server/main.go`

### Steps

- [ ] **Replace** the trailing `r.Run(addr)` block with a graceful-shutdown pattern:
  ```go
  srv := &http.Server{
      Addr:    addr,
      Handler: r,
  }
  go func() {
      mlog.Info("server starting", mlog.String("addr", addr))
      if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
          mlog.Fatal("server failed", mlog.Err(err))
      }
  }()

  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
  sig := <-sigCh
  mlog.Info("shutdown signal received", mlog.String("signal", sig.String()))

  // Order matters: stop accepting requests → drain MCP children → exit.
  shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
  defer cancel()
  if err := srv.Shutdown(shutdownCtx); err != nil {
      mlog.Warn("http shutdown error", mlog.Err(err))
  }
  if err := mcpHyp.PruneAll(); err != nil {
      mlog.Warn("mcp prune error", mlog.Err(err))
  }
  mlog.Info("server shutdown complete")
  ```
- [ ] Add imports: `"errors"`, `"net/http"`, `"os/signal"`, `"syscall"`, `"context"`, `"time"`.
- [ ] **Manual smoke test** (cannot be automated in unit tests easily):
  ```bash
  # Terminal 1
  STORAGE_DIR=/tmp/mcp-smoke go run ./cmd/server
  # Wait for "server starting"
  # In another terminal, seed a config:
  cat > /tmp/mcp-smoke/plugins/anythingllm_mcp_servers.json <<'EOF'
  {"mcpServers": {"echo": {"command": "/path/to/echo-mcp"}}}
  EOF
  # Trigger a reload via API to boot it
  curl -X GET http://localhost:3001/api/mcp-servers/force-reload
  # Verify child PID via:
  pgrep -f echo-mcp
  # Send SIGTERM to backend (Ctrl-C in Terminal 1)
  # Verify echo-mcp child is gone:
  pgrep -f echo-mcp # should print nothing within 4s
  ```
- [ ] Run `go build ./cmd/server/` — expect success.
- [ ] Run `go test ./... -count=1 -race` — full suite green.
- [ ] Commit: `feat(mcp): graceful shutdown reaps MCP children on SIGTERM`.

### Acceptance

- `go build ./cmd/server/` and `go vet ./...` clean.
- Manual smoke confirms child MCP processes die within 4s of SIGTERM.
- Existing HTTP routes still serve correctly (no regression).

---

## Post-PR checklist

- [ ] `.gpowers/decisions/2026-05-26-mcp-go-sdk.md` exists and is referenced from `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §6.
- [ ] `go test ./internal/mcp/ -count=1 -race -v` reports ≥ 55 tests passing (PR-A ~30 + PR-B ~25 added).
- [ ] `go test ./internal/handlers/ -run TestMCPHandler -count=1 -v` reports ≥ 15 tests passing (PR-A 12 + 3 new e2e).
- [ ] `go test ./... -count=1` full repo green.
- [ ] `go vet ./...` clean.
- [ ] No `fmt.Println`, `log.Println`, or bare `panic` in `internal/mcp/` — all logging via `mlog`.
- [ ] Update `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §10 "PR-B 已完成" + cross out the stdio risk row in §10.1.
- [ ] `frontend` smoke: load the MCP settings page in dev, verify a configured stdio server shows `running: true` with tool list (manual; no automated check).

---

## Risk notes

- **SDK API churn** — pin to exact version in Task 0. If the SDK breaks between minor versions, the impact is contained to `transport_stdio.go` + the echo fixture (none of `hypervisor.go` / `handlers/mcp.go` should import the SDK directly).
- **Zombie processes on test panic** — `t.Cleanup` blocks ensure `pruneServer` runs even on assertion failure; verify by deliberately panicking in a test and asserting no `echo-mcp` left in `pgrep` after the test exits.
- **`exec.Cmd.Wait()` deadlock** — if the child writes more than the OS pipe buffer (~64KB on Linux) to stdout and we're not draining it (because the SDK reader stopped), Wait() blocks forever. The SDK should drain stdout into its message reader continuously; verify in Task 3 with a stress test that calls `echo` with a 100KB payload.
- **30s timeout interaction with `Boot()`** — booting N servers sequentially with hung ones could spend up to N×30s. Acceptable for v1; consider parallel boot in a future PR if operators report slow startup.
- **`$SHELL -ic env` slow on first invocation in some shells** (e.g. fish/zsh with heavy rcfiles) — capped at 5s with fallback; document in user troubleshooting.
- **Windows stdio transport** — `os/exec` works but signal semantics differ (no SIGTERM; Windows uses `Process.Kill()` directly which is SIGKILL-equivalent). Tag stdio transport Windows behaviour with a TODO + skip the grace test on Windows. Full Windows parity is not a v1 goal.
- **Suppressed tool calls via `CallTool`** — by design, admin can call suppressed tools through the REST proxy (suppression filters agent exposure, not direct admin invocation). Document in route docs.
- **Concurrent ToggleServer** — two admins toggling the same server simultaneously: `Hypervisor.mu` serialises them; the second call sees the updated state and either starts or stops accordingly. No data race; the user may see surprising on/off ordering — acceptable.

---

## Estimate

- Task 0 (SDK spike): 3-4 h
- Task 1 (shell env): 2 h
- Task 2 (echo fixture + TestMain): 3 h
- Task 3 (stdio transport + 12 tests): 4-5 h
- Task 4 (factory dispatch): 30 min
- Task 5 (hypervisor lifecycle + 18 tests): 5-6 h
- Task 6 (graceful shutdown): 1 h

**Total: ~18-22 hours**, i.e. 3 working days. Tasks 1–4 can run in parallel with Task 5 prep if two engineers are available; otherwise single-track ~3 days.
