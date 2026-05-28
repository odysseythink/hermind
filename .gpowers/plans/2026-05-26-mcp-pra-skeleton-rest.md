# MCP Hypervisor PR-A — Skeleton + REST Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the `mcp` package skeleton + Node-compatible JSON config read/write + service facade + 6 admin REST routes (5 Node-parity + 1 Go-private tool-call proxy). All transport-backed operations return `error: "MCP transport not implemented"`; transport itself lands in PR-B/C. Config-only operations (list/reload/delete/toggle-tool) are fully functional from day 1.

**Architecture:** New `internal/mcp/` package owns the hypervisor + transport interface; `services/mcp_service.go` is a thin facade exposed to handlers. Existing `handlers/mcp.go` is rewritten end-to-end. Singleton initialised once in `main.go`. Tests use the standard `apiTestEnv` pattern + a private `setupMCPTest` helper that supplies a tempdir-backed config file.

**Tech Stack:** Go 1.22+, Gin, GORM (only via existing test scaffolding), sqlite (test), testify, `os.WriteFile`/`os.Rename` atomic JSON writes.

**Source spec:** `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §3-5, §7.

**Reference Node implementation:**
- `server/endpoints/mcpServers.js` (5 admin routes, JSON response shape)
- `server/utils/MCP/index.js` (servers/toggleServerStatus/deleteServer/toggleToolSuppression)
- `server/utils/MCP/hypervisor/index.js` (config file, parseServerType, validateServerDefinitionByType, updateSuppressedTools, removeMCPServerFromConfig, getSuppressedTools)

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `handlers/mcp.go` (60 lines, stub) — to be **completely rewritten** in Task 5. All 5 current handlers return hard-coded empty JSON. RegisterMCPRoutes signature is `(api *gin.RouterGroup, authSvc *services.AuthService)`; this must change to also accept `*services.MCPService`.
- `middleware.ValidatedRequest(authSvc)` (`middleware/auth.go:14`) — sets `c.Set("user", *models.User)`. In single-user mode (`AuthToken == "" || JWTSecret == ""`) auto-admin bypass; tests can leave config zero-valued.
- `middleware.FlexUserRoleValid([]string{"admin"})` (`middleware/rbac.go:12`) — gates by `user.Role`. Auto-bypass admin user from `ValidatedRequest` satisfies this.
- `config.Config.StorageDir` (`config/config.go:12`) — defaults `./storage`, ensures `os.MkdirAll` at load time. MCP config lives at `<StorageDir>/plugins/hermind_mcp_servers.json`.
- Test scaffolding `apiTestEnv` (`handlers/api_setup_test.go:15`) is API-key oriented; MCP routes are session-auth not API-key. **Do not reuse `apiTestEnv` for MCP tests** — write a small purpose-built helper (see §Test setup).
- `dto.ErrorResponse` (`dto/`) — `{ "error": "..." }`. Node MCP returns richer shape `{ success: bool, error: string|null, ... }`; **do not** use `dto.ErrorResponse` here.

### New package layout (this PR)

```
backend/internal/mcp/
├── types.go                # ServerConfig, ServerView, ToolSchema, LoadResult, ProcessInfo, errors
├── transport.go            # Transport interface + factory stub (returns ErrTransportNotImplemented)
├── config.go               # JSON read/write, suppression toggle, remove, ensure
├── hypervisor.go           # Hypervisor struct + Servers/Toggle/Delete/ToggleTool/CallTool + Boot/PruneAll no-ops
├── singleton.go            # Instance(cfg) — sync.Once
├── config_test.go
└── hypervisor_test.go

backend/internal/services/
└── mcp_service.go          # thin facade

backend/internal/handlers/
├── mcp.go                  # REWRITTEN — 6 routes
└── mcp_test.go             # NEW — handler-level tests
```

### Methods to ship (PR-A scope)

| # | Owner | Signature | Backed by |
|---|---|---|---|
| 1 | `mcp.Config` | `Load() ([]ServerConfig, error)` | JSON file |
| 2 | `mcp.Config` | `Write([]ServerConfig) error` | atomic write |
| 3 | `mcp.Config` | `Ensure() error` | mkdir + seed `{"mcpServers":{}}` |
| 4 | `mcp.Config` | `RemoveServer(name string) (bool, error)` | rewrites JSON |
| 5 | `mcp.Config` | `UpdateSuppressedTools(serverName, toolName string, enabled bool) ([]string, error)` | rewrites JSON |
| 6 | `mcp.Config` | `GetSuppressedTools(serverName string) []string` | reads JSON |
| 7 | `mcp.Hypervisor` | `Servers(ctx) ([]ServerView, error)` | returns config-derived view; `running=false`, `error="MCP transport not implemented"` |
| 8 | `mcp.Hypervisor` | `Reload(ctx) ([]ServerView, error)` | identical to Servers in PR-A |
| 9 | `mcp.Hypervisor` | `ToggleServer(ctx, name) (bool, error)` | returns `ErrTransportNotImplemented` |
| 10 | `mcp.Hypervisor` | `DeleteServer(ctx, name) (bool, error)` | delegates to `Config.RemoveServer` (transport-free) |
| 11 | `mcp.Hypervisor` | `ToggleTool(ctx, serverName, toolName, enabled) ([]string, error)` | delegates to `Config.UpdateSuppressedTools` |
| 12 | `mcp.Hypervisor` | `CallTool(ctx, serverName, toolName, args) (any, error)` | returns `ErrTransportNotImplemented` |
| 13 | `mcp.Hypervisor` | `Boot(ctx) error` | no-op in PR-A (logs `"MCP transport not implemented — skipping boot"` once) |
| 14 | `mcp.Hypervisor` | `PruneAll() error` | no-op |
| 15 | `services.MCPService` | thin pass-through to all 8 hypervisor methods | — |
| 16 | `handlers` | 6 routes — see §5 of design | — |

### Response shape (Node parity)

| Route | Success body | Error body |
|---|---|---|
| `GET /api/mcp-servers/list` | `{success: true, servers: [...]}` | `{success: false, error: "..."}` |
| `GET /api/mcp-servers/force-reload` | `{success: true, error: null, servers: [...]}` | `{success: false, error: "...", servers: []}` |
| `POST /api/mcp-servers/toggle` | `{success: true, error: null}` *(N/A in PR-A — always 500)* | `{success: false, error: "MCP transport not implemented"}` |
| `POST /api/mcp-servers/delete` | `{success: true, error: null}` | `{success: false, error: "..."}` |
| `POST /api/mcp-servers/toggle-tool` | `{success: true, error: null, suppressedTools: [...]}` | `{success: false, error: "...", suppressedTools: []}` |
| `POST /api/mcp/:name/tools/:tool/call` *(NEW)* | `{success: true, result: <any>, error: null}` *(N/A in PR-A — always 502)* | `{success: false, result: null, error: "MCP transport not implemented"}` |

**HTTP status convention** (matches Node):
- Success: `200`
- Application error (server not in config, malformed input): `200` with `success: false` (Node's pattern — never raise non-200 for these)
- Internal error (file IO, JSON parse): `500`
- `CallTool` is Go-private, use proper REST codes: `404` (server/tool not found), `422` (bad args), `502` (transport error or not implemented)

### ServerView for PR-A (no transport)

```json
{
  "name": "echo",
  "config": {
    "command": "node",
    "args": ["echo-server.js"],
    "hermind": {"autoStart": true, "suppressedTools": []}
  },
  "running": false,
  "tools": [],
  "error": "MCP transport not implemented",
  "process": null
}
```

> The `error` field stays `"MCP transport not implemented"` for every server until PR-B lands stdio transport. Frontend already handles per-server error state.

### Out of scope (explicit)

- **Transport implementations** — stdio/HTTP/SSE all land in PR-B/PR-C.
- **`ToolPlugin` interface / `ActiveServers()` / `ToolsAsPlugins()`** — PR-E. Do NOT add stubs in PR-A even as exported names; keep `mcp.Hypervisor` surface tight.
- **MCP SDK dependency** — DO NOT add `github.com/modelcontextprotocol/go-sdk` or `mark3labs/mcp-go` to `go.mod` in PR-A. SDK selection is the very first step of PR-B's spike (see design §6).
- **Shell env patching (`patchShellEnvironmentPath`)** — PR-B.
- **Frontend changes** — none. The existing UI already calls the 5 admin routes; tool-call proxy is debug-only.

### Data invariants

- Config file path: `filepath.Join(cfg.StorageDir, "plugins", "hermind_mcp_servers.json")`. **Do not** read `NODE_ENV`; Go has no equivalent — always use `StorageDir`.
- JSON top-level shape: `{"mcpServers": { "<name>": { ... } }}`. An empty file (zero bytes) is treated as `{"mcpServers":{}}` (Node has same fallback via `safeJsonParse`).
- A server entry name is its JSON object key (e.g. `"docker-mcp"`); the `Name` field on `ServerConfig` is set by us at load time, never serialised.
- `hermind.suppressedTools` is the only field we write back besides server-add/delete. Other fields (command/args/env/url/headers/type/hermind.autoStart) are **owned by the user** — never modify them.
- Atomic write convention: `os.WriteFile(path+".tmp", data, 0644)` then `os.Rename(path+".tmp", path)`. On Windows, `os.Rename` over an existing file fails — use `os.WriteFile` directly there (acceptable race, no concurrent Node writer in dev).
- Reserved names: none. Any name in the JSON is a valid server; `DeleteServer("not-found")` returns `(false, nil)` *with no error* (Node returns `{success:false, error:"MCP server X not found in config file."}`).

### TDD discipline

Each task follows: write failing test → run + confirm fail → implement → run + confirm pass → commit. Each commit message: `feat(mcp): <task description>` or `test(mcp): <task description>`. Do NOT bundle multiple tasks into one commit.

### Test setup helper (handlers/mcp_test.go private helper)

```go
type mcpTestEnv struct {
    Router  *gin.Engine
    Storage string                  // tempdir, set as cfg.StorageDir
    Cfg     *config.Config
    Hyp     *mcp.Hypervisor
    Svc     *services.MCPService
    AuthSvc *services.AuthService
}

func newMCPTestEnv(t *testing.T) *mcpTestEnv {
    t.Helper()
    tmp := t.TempDir()
    cfg := &config.Config{StorageDir: tmp}     // AuthToken == "" → IsAuthEnabled() == false → bypass auth
    db := openMemDB(t)                          // reuse pattern from api_setup_test.go
    enc, _ := utils.NewEncryptionManager(cfg.SigKey, cfg.SigSalt)
    authSvc := services.NewAuthService(db, cfg, enc)
    hyp := mcp.NewHypervisorForTesting(cfg)     // bypass singleton — see Task 3
    svc := services.NewMCPService(hyp)

    gin.SetMode(gin.TestMode)
    r := gin.New()
    api := r.Group("/api")
    handlers.RegisterMCPRoutes(api, authSvc, svc)
    return &mcpTestEnv{Router: r, Storage: tmp, Cfg: cfg, Hyp: hyp, Svc: svc, AuthSvc: authSvc}
}

func (e *mcpTestEnv) writeRawConfig(t *testing.T, body string) {
    t.Helper()
    path := filepath.Join(e.Storage, "plugins", "hermind_mcp_servers.json")
    require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
    require.NoError(t, os.WriteFile(path, []byte(body), 0644))
}
```

> `NewHypervisorForTesting(cfg)` is required because the production `Instance(cfg)` is a `sync.Once` singleton — re-using it across tests pollutes state. Task 3 ships this exported test-only constructor (no build tag, just a `_for_testing` suffix to discourage non-test callers).

---

## Task 1: Package skeleton + types + transport interface

**File:** `backend/internal/mcp/types.go`, `backend/internal/mcp/transport.go`

**Test:** none yet — pure type declarations. (Tests come in Task 2.)

### Steps

- [ ] Create `internal/mcp/` directory.
- [ ] Write `types.go` with:
  ```go
  package mcp

  import (
      "context"
      "encoding/json"
      "errors"
  )

  var (
      ErrTransportNotImplemented = errors.New("MCP transport not implemented")
      ErrInvalidServerType       = errors.New("MCP server type is invalid")
      ErrServerNotFound          = errors.New("MCP server not found in config file")
      ErrServerNameRequired      = errors.New("MCP server name is required")
  )

  type ServerConfig struct {
      Name        string              `json:"-"`
      Command     string              `json:"command,omitempty"`
      Args        []string            `json:"args,omitempty"`
      Env         map[string]string   `json:"env,omitempty"`
      URL         string              `json:"url,omitempty"`
      Type        string              `json:"type,omitempty"`
      Headers     map[string]string   `json:"headers,omitempty"`
      Hermind *HermindOptions `json:"hermind,omitempty"`
  }

  type HermindOptions struct {
      AutoStart       *bool    `json:"autoStart,omitempty"`
      SuppressedTools []string `json:"suppressedTools,omitempty"`
  }

  type ServerView struct {
      Name    string        `json:"name"`
      Config  *ServerConfig `json:"config"`
      Running bool          `json:"running"`
      Tools   []ToolSchema  `json:"tools"`
      Error   *string       `json:"error"`
      Process *ProcessInfo  `json:"process"`
  }

  type ToolSchema struct {
      Name        string          `json:"name"`
      Description string          `json:"description"`
      InputSchema json.RawMessage `json:"inputSchema"`
  }

  type ProcessInfo struct {
      PID int    `json:"pid"`
      Cmd string `json:"cmd,omitempty"`
  }

  type LoadResult struct {
      Status  string // "success" | "failed"
      Message string
  }
  ```
- [ ] Write `transport.go` with:
  ```go
  package mcp

  import "context"

  type Transport interface {
      Connect(ctx context.Context) error
      Close() error
      Ping(ctx context.Context) bool
      ListTools(ctx context.Context) ([]ToolSchema, error)
      CallTool(ctx context.Context, name string, args map[string]any) (any, error)
      ProcessInfo() *ProcessInfo
  }

  // newTransport is the factory used by Hypervisor to spawn a transport
  // for a configured server. PR-A stub — returns ErrTransportNotImplemented
  // for every input. Real implementations land in PR-B (stdio) and PR-C (HTTP/SSE).
  func newTransport(srv *ServerConfig) (Transport, error) {
      return nil, ErrTransportNotImplemented
  }
  ```
- [ ] Run `go build ./internal/mcp/...` — expect success (no test runner yet).
- [ ] Run `gofmt -l internal/mcp/` — expect zero output.
- [ ] Commit: `feat(mcp): add package types + transport interface`.

### Acceptance

- `internal/mcp/types.go` compiles cleanly.
- `internal/mcp/transport.go` exports `Transport` interface and `newTransport` returning `ErrTransportNotImplemented`.
- No new dependencies in `go.mod`.

---

## Task 2: Config read/write (Node-compatible JSON)

**File:** `internal/mcp/config.go`, `internal/mcp/config_test.go`

### Steps

- [ ] **Write failing test** `config_test.go` with cases:
  - `TestConfig_Ensure_CreatesFileIfMissing` — fresh tempdir, `Ensure()` creates `plugins/hermind_mcp_servers.json` with `{"mcpServers":{}}`.
  - `TestConfig_Ensure_NoopIfPresent` — pre-write garbage; `Ensure()` does not overwrite.
  - `TestConfig_Load_EmptyFile` — pre-write `""`; `Load()` returns `[]ServerConfig{}` no error.
  - `TestConfig_Load_MalformedJSON` — pre-write `"{bad"`; `Load()` returns `[]ServerConfig{}` no error (Node parity via `safeJsonParse`).
  - `TestConfig_Load_StdioServer` — pre-write valid JSON with one stdio server; assert `ServerConfig{Name:"echo", Command:"node", Args:[...]}` round-trip.
  - `TestConfig_Load_HTTPServer` — pre-write valid JSON with `{url, type:"streamable", headers}`; assert fields populated.
  - `TestConfig_Write_RoundTrip` — load → mutate → write → reload → assert equal.
  - `TestConfig_RemoveServer_Found` — assert `(true, nil)` + JSON no longer contains key.
  - `TestConfig_RemoveServer_NotFound` — assert `(false, nil)`.
  - `TestConfig_UpdateSuppressedTools_AddNew` — enabled=false on a server with empty/no suppressedTools → array `["toolX"]`.
  - `TestConfig_UpdateSuppressedTools_RemoveExisting` — enabled=true on a server whose suppressedTools=["toolX"] → array `[]`.
  - `TestConfig_UpdateSuppressedTools_Idempotent_Add` — suppress same tool twice → still single entry.
  - `TestConfig_UpdateSuppressedTools_Idempotent_Remove` — unsuppress an already-enabled tool → no change.
  - `TestConfig_UpdateSuppressedTools_ServerNotFound` — returns `[]` + non-nil error wrapping `ErrServerNotFound`.
  - `TestConfig_GetSuppressedTools_Default` — server with no hermind block → empty slice.
- [ ] Run `go test ./internal/mcp/ -run TestConfig` — expect compile errors (Config type not defined yet).
- [ ] **Implement** `config.go`:
  ```go
  package mcp

  import (
      "encoding/json"
      "errors"
      "fmt"
      "os"
      "path/filepath"
      "sync"
  )

  type Config struct {
      Path string
      mu   sync.Mutex
  }

  func NewConfig(storageDir string) *Config {
      return &Config{Path: filepath.Join(storageDir, "plugins", "hermind_mcp_servers.json")}
  }

  type rawFile struct {
      MCPServers map[string]*ServerConfig `json:"mcpServers"`
  }

  func (c *Config) Ensure() error {
      if _, err := os.Stat(c.Path); err == nil {
          return nil
      } else if !errors.Is(err, os.ErrNotExist) {
          return err
      }
      if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err != nil {
          return err
      }
      return os.WriteFile(c.Path, []byte(`{"mcpServers":{}}`), 0644)
  }

  func (c *Config) Load() ([]ServerConfig, error) {
      c.mu.Lock()
      defer c.mu.Unlock()
      return c.loadLocked()
  }

  func (c *Config) loadLocked() ([]ServerConfig, error) {
      data, err := os.ReadFile(c.Path)
      if err != nil {
          if errors.Is(err, os.ErrNotExist) {
              return []ServerConfig{}, nil
          }
          return nil, err
      }
      if len(data) == 0 {
          return []ServerConfig{}, nil
      }
      var raw rawFile
      if err := json.Unmarshal(data, &raw); err != nil {
          // Node parity: tolerate malformed JSON, treat as empty
          return []ServerConfig{}, nil
      }
      out := make([]ServerConfig, 0, len(raw.MCPServers))
      for name, srv := range raw.MCPServers {
          if srv == nil {
              continue
          }
          srv.Name = name
          out = append(out, *srv)
      }
      return out, nil
  }

  func (c *Config) writeLocked(servers []ServerConfig) error {
      raw := rawFile{MCPServers: make(map[string]*ServerConfig, len(servers))}
      for i := range servers {
          name := servers[i].Name
          srv := servers[i]
          srv.Name = "" // never serialise
          raw.MCPServers[name] = &srv
      }
      data, err := json.MarshalIndent(raw, "", "  ")
      if err != nil {
          return err
      }
      tmp := c.Path + ".tmp"
      if err := os.WriteFile(tmp, data, 0644); err != nil {
          return err
      }
      if err := os.Rename(tmp, c.Path); err != nil {
          // Windows: rename over existing fails — fall back to direct write
          return os.WriteFile(c.Path, data, 0644)
      }
      return nil
  }

  func (c *Config) Write(servers []ServerConfig) error {
      c.mu.Lock()
      defer c.mu.Unlock()
      return c.writeLocked(servers)
  }

  func (c *Config) RemoveServer(name string) (bool, error) {
      c.mu.Lock()
      defer c.mu.Unlock()
      servers, err := c.loadLocked()
      if err != nil {
          return false, err
      }
      idx := -1
      for i, s := range servers {
          if s.Name == name {
              idx = i
              break
          }
      }
      if idx < 0 {
          return false, nil
      }
      servers = append(servers[:idx], servers[idx+1:]...)
      if err := c.writeLocked(servers); err != nil {
          return false, err
      }
      return true, nil
  }

  func (c *Config) UpdateSuppressedTools(serverName, toolName string, enabled bool) ([]string, error) {
      c.mu.Lock()
      defer c.mu.Unlock()
      servers, err := c.loadLocked()
      if err != nil {
          return nil, err
      }
      for i := range servers {
          if servers[i].Name != serverName {
              continue
          }
          if servers[i].Hermind == nil {
              servers[i].Hermind = &HermindOptions{}
          }
          suppressed := servers[i].Hermind.SuppressedTools
          if enabled {
              suppressed = removeString(suppressed, toolName)
          } else if !containsString(suppressed, toolName) {
              suppressed = append(suppressed, toolName)
          }
          servers[i].Hermind.SuppressedTools = suppressed
          if err := c.writeLocked(servers); err != nil {
              return nil, err
          }
          return suppressed, nil
      }
      return nil, fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
  }

  func (c *Config) GetSuppressedTools(serverName string) []string {
      servers, err := c.Load()
      if err != nil {
          return nil
      }
      for _, s := range servers {
          if s.Name != serverName {
              continue
          }
          if s.Hermind == nil {
              return nil
          }
          return s.Hermind.SuppressedTools
      }
      return nil
  }

  func containsString(xs []string, x string) bool {
      for _, v := range xs {
          if v == x { return true }
      }
      return false
  }

  func removeString(xs []string, x string) []string {
      out := xs[:0]
      for _, v := range xs {
          if v != x { out = append(out, v) }
      }
      return out
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestConfig -v` — expect all green.
- [ ] Commit: `feat(mcp): config read/write (Node-compatible JSON)`.

### Acceptance

- All 14 unit tests pass.
- `os.WriteFile(path+".tmp") + os.Rename` atomic write.
- Malformed JSON is silently treated as empty (Node parity).
- Empty `hermind` block is materialised on first suppression write.

---

## Task 3: Hypervisor + singleton (no transport)

**File:** `internal/mcp/hypervisor.go`, `internal/mcp/singleton.go`, `internal/mcp/hypervisor_test.go`

### Steps

- [ ] **Write failing test** `hypervisor_test.go`:
  - `TestHypervisor_Servers_EmptyConfig` — fresh tempdir, `Servers(ctx)` returns `[]ServerView{}` no error, no file created (Ensure not called eagerly... actually call Ensure first; assert returns `[]`).
  - `TestHypervisor_Servers_OneServer` — seed config with one stdio server; `Servers(ctx)` returns 1 view with `Running:false, Error:Ptr("MCP transport not implemented"), Tools:[], Process:nil`.
  - `TestHypervisor_Servers_TwoServers_OrderStable` — seed 2 servers; assert returned slice length 2 (order may be map-iteration-random, so check by name, not by index).
  - `TestHypervisor_Reload_SameAsServers` — assert `Reload(ctx)` returns equal payload to `Servers(ctx)`.
  - `TestHypervisor_ToggleServer_ReturnsTransportError` — assert `(false, ErrTransportNotImplemented)` regardless of whether the server exists.
  - `TestHypervisor_DeleteServer_Found` — seed 1 server; `DeleteServer(ctx, "echo")` returns `(true, nil)`; assert file no longer contains key.
  - `TestHypervisor_DeleteServer_NotFound` — seed no servers; `DeleteServer(ctx, "echo")` returns `(false, nil)`.
  - `TestHypervisor_ToggleTool_Suppress` — seed 1 server with no suppression; `ToggleTool(ctx, "echo", "danger", false)` returns `["danger"]`; reload config; assert persisted.
  - `TestHypervisor_ToggleTool_Unsuppress` — seed 1 server with `suppressedTools:["danger"]`; `ToggleTool(ctx, "echo", "danger", true)` returns `[]`.
  - `TestHypervisor_ToggleTool_ServerNotFound` — returns `nil, error` wrapping `ErrServerNotFound`.
  - `TestHypervisor_CallTool_ReturnsTransportError` — `CallTool(ctx, "echo", "do", nil)` returns `(nil, ErrTransportNotImplemented)`.
  - `TestHypervisor_Boot_Noop` — `Boot(ctx)` returns nil, leaves state empty.
  - `TestHypervisor_PruneAll_Noop` — `PruneAll()` returns nil.
  - `TestHypervisor_ParseServerType` (table-driven):
    | input | expected |
    |---|---|
    | `{Type:"sse"}` | `"http"` |
    | `{Type:"streamable"}` | `"http"` |
    | `{Type:"http"}` | `"http"` |
    | `{Command:"node"}` | `"stdio"` |
    | `{URL:"http://x"}` | `"sse"` *(Node fallback when no `type` field but has `url`)* |
    | `{}` | `""` *(invalid)* |
  - `TestHypervisor_ValidateServerDefinition_StdioArgsMustBeArray` — `Args` is a `[]string` in Go so this can't fail at runtime; **skip this test** but include a comment explaining Node's check is JS-only.
  - `TestHypervisor_ValidateServerDefinition_HTTPUrlRequired` — `{Type:"http"}` with no URL returns error containing `"missing required"`.
  - `TestHypervisor_ValidateServerDefinition_HTTPUrlMalformed` — `{Type:"http", URL:"://invalid"}` returns error containing `"invalid URL"`.
- [ ] Run `go test ./internal/mcp/ -run TestHypervisor -v` — expect compile errors.
- [ ] **Implement** `hypervisor.go`:
  ```go
  package mcp

  import (
      "context"
      "errors"
      "fmt"
      "net/url"
      "sync"

      "github.com/odysseythink/hermind/backend/internal/config"
  )

  type Hypervisor struct {
      mu     sync.RWMutex
      cfg    *config.Config
      file   *Config
      mcps   map[string]*activeClient // empty in PR-A; populated in PR-B
      booted bool
  }

  type activeClient struct {
      transport Transport
      process   *ProcessInfo
  }

  func newHypervisor(cfg *config.Config) *Hypervisor {
      return &Hypervisor{
          cfg:  cfg,
          file: NewConfig(cfg.StorageDir),
          mcps: make(map[string]*activeClient),
      }
  }

  // NewHypervisorForTesting bypasses the singleton for test isolation.
  // Do not call from production code.
  func NewHypervisorForTesting(cfg *config.Config) *Hypervisor {
      return newHypervisor(cfg)
  }

  func (h *Hypervisor) Servers(ctx context.Context) ([]ServerView, error) {
      if err := h.file.Ensure(); err != nil {
          return nil, err
      }
      configs, err := h.file.Load()
      if err != nil {
          return nil, err
      }
      transportErr := ErrTransportNotImplemented.Error()
      out := make([]ServerView, 0, len(configs))
      for i := range configs {
          srv := configs[i]
          out = append(out, ServerView{
              Name:    srv.Name,
              Config:  &srv,
              Running: false,
              Tools:   []ToolSchema{},
              Error:   &transportErr,
              Process: nil,
          })
      }
      return out, nil
  }

  func (h *Hypervisor) Reload(ctx context.Context) ([]ServerView, error) {
      return h.Servers(ctx) // PR-A: no transport state to flush
  }

  func (h *Hypervisor) ToggleServer(ctx context.Context, name string) (bool, error) {
      return false, ErrTransportNotImplemented
  }

  func (h *Hypervisor) DeleteServer(ctx context.Context, name string) (bool, error) {
      ok, err := h.file.RemoveServer(name)
      if err != nil {
          return false, err
      }
      if !ok {
          return false, fmt.Errorf("%w: %s", ErrServerNotFound, name)
      }
      return true, nil
  }

  func (h *Hypervisor) ToggleTool(ctx context.Context, serverName, toolName string, enabled bool) ([]string, error) {
      return h.file.UpdateSuppressedTools(serverName, toolName, enabled)
  }

  func (h *Hypervisor) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
      return nil, ErrTransportNotImplemented
  }

  func (h *Hypervisor) Boot(ctx context.Context) error {
      h.mu.Lock()
      defer h.mu.Unlock()
      if h.booted {
          return nil
      }
      h.booted = true
      // PR-A: no-op. PR-B will iterate configs and start transports here.
      return nil
  }

  func (h *Hypervisor) PruneAll() error {
      return nil
  }

  // parseServerType inspects a ServerConfig and returns one of "stdio" | "http" | "sse" | "".
  func parseServerType(srv *ServerConfig) string {
      switch srv.Type {
      case "sse", "streamable", "http":
          return "http"
      }
      if srv.Command != "" {
          return "stdio"
      }
      if srv.URL != "" {
          return "sse"
      }
      return ""
  }

  func validateServerDefinition(name string, srv *ServerConfig, kind string) error {
      switch srv.Type {
      case "sse", "streamable", "http":
          if srv.URL == "" {
              return fmt.Errorf("MCP server %q: missing required %q for %s transport", name, "url", srv.Type)
          }
          if _, err := url.Parse(srv.URL); err != nil || !isValidURL(srv.URL) {
              return fmt.Errorf("MCP server %q: invalid URL %q", name, srv.URL)
          }
          return nil
      }
      switch kind {
      case "stdio":
          return nil // Args type-checked by Go
      case "http":
          if srv.Type != "sse" && srv.Type != "streamable" {
              return errors.New("MCP server type must be sse or streamable")
          }
      }
      return nil
  }

  func isValidURL(s string) bool {
      u, err := url.Parse(s)
      return err == nil && u.Scheme != "" && u.Host != ""
  }
  ```
- [ ] **Implement** `singleton.go`:
  ```go
  package mcp

  import (
      "sync"

      "github.com/odysseythink/hermind/backend/internal/config"
  )

  var (
      instance *Hypervisor
      once     sync.Once
  )

  // Instance returns the process-wide MCP hypervisor singleton. First call
  // initialises it from the provided config; subsequent calls return the same
  // instance regardless of argument.
  func Instance(cfg *config.Config) *Hypervisor {
      once.Do(func() { instance = newHypervisor(cfg) })
      return instance
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestHypervisor -v` — expect all green.
- [ ] Run `go test ./internal/mcp/ -v` — full package green (Config + Hypervisor).
- [ ] Commit: `feat(mcp): hypervisor + singleton (no-transport mode)`.

### Acceptance

- 16+ hypervisor unit tests pass.
- `Instance(cfg)` returns same `*Hypervisor` on repeated calls.
- `NewHypervisorForTesting(cfg)` returns fresh instance bypassing the singleton.
- `Servers()` reads from disk every call (no caching, since config can be edited by Node or external tooling).

---

## Task 4: Service facade

**File:** `internal/services/mcp_service.go`

**Test:** none — the facade is a pure pass-through; Hypervisor tests + Handler tests provide coverage end-to-end.

### Steps

- [ ] Write `mcp_service.go`:
  ```go
  package services

  import (
      "context"

      "github.com/odysseythink/hermind/backend/internal/mcp"
  )

  type MCPService struct {
      hv *mcp.Hypervisor
  }

  func NewMCPService(hv *mcp.Hypervisor) *MCPService {
      return &MCPService{hv: hv}
  }

  func (s *MCPService) Servers(ctx context.Context) ([]mcp.ServerView, error) {
      return s.hv.Servers(ctx)
  }

  func (s *MCPService) Reload(ctx context.Context) ([]mcp.ServerView, error) {
      return s.hv.Reload(ctx)
  }

  func (s *MCPService) ToggleServer(ctx context.Context, name string) (bool, error) {
      return s.hv.ToggleServer(ctx, name)
  }

  func (s *MCPService) DeleteServer(ctx context.Context, name string) (bool, error) {
      return s.hv.DeleteServer(ctx, name)
  }

  func (s *MCPService) ToggleTool(ctx context.Context, serverName, toolName string, enabled bool) ([]string, error) {
      return s.hv.ToggleTool(ctx, serverName, toolName, enabled)
  }

  func (s *MCPService) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
      return s.hv.CallTool(ctx, serverName, toolName, args)
  }
  ```
- [ ] Run `go build ./internal/services/...` — expect success.
- [ ] Commit: `feat(mcp): MCPService facade`.

### Acceptance

- Compiles cleanly.
- No test file (facade is too thin; coverage via handler tests).

---

## Task 5: Rewrite handlers + tool-call route

**File:** `internal/handlers/mcp.go` (REWRITTEN), `internal/handlers/mcp_test.go` (NEW)

### Steps

- [ ] **Write failing test** `mcp_test.go` with `setupMCPTest` helper (per §Pre-task Test setup) and cases:
  - `TestMCPHandler_ListServers_Empty` — fresh env, GET `/api/mcp-servers/list` → `200`, body `{success:true, servers:[]}`.
  - `TestMCPHandler_ListServers_OneServer` — seed file via `env.writeRawConfig`, GET list → `200`, body has 1 server with `running:false, error:"MCP transport not implemented", tools:[]`.
  - `TestMCPHandler_ForceReload_Success` — fresh env, GET `/api/mcp-servers/force-reload` → `200`, body `{success:true, error:null, servers:[]}`.
  - `TestMCPHandler_ToggleServer_TransportNotImplemented` — POST `/api/mcp-servers/toggle` body `{"name":"echo"}` → `200`, body `{success:false, error:"MCP transport not implemented"}`.
  - `TestMCPHandler_ToggleServer_MissingNameBody` — POST with empty body → `200`, body `{success:false, error:"..."}` containing `"name"` (input validation, no transport touched).
  - `TestMCPHandler_DeleteServer_Found` — seed 1 server, POST `/api/mcp-servers/delete` body `{"name":"echo"}` → `200`, body `{success:true, error:null}`; assert file no longer contains key (read disk).
  - `TestMCPHandler_DeleteServer_NotFound` — POST against empty config → `200`, body `{success:false, error:"...not found..."}`.
  - `TestMCPHandler_ToggleTool_Suppress` — seed 1 server, POST `/api/mcp-servers/toggle-tool` body `{"serverName":"echo","toolName":"danger","enabled":false}` → `200`, body `{success:true, error:null, suppressedTools:["danger"]}`.
  - `TestMCPHandler_ToggleTool_Unsuppress` — seed with `suppressedTools:["danger"]`, enabled=true → suppressedTools `[]`.
  - `TestMCPHandler_ToggleTool_ServerNotFound` — POST against empty config → `200`, body `{success:false, error:"...", suppressedTools:[]}`.
  - `TestMCPHandler_CallTool_TransportNotImplemented` — POST `/api/mcp/echo/tools/do/call` body `{"arguments":{}}` → `502`, body `{success:false, result:null, error:"MCP transport not implemented"}`.
  - `TestMCPHandler_CallTool_BadJSON` — POST with invalid JSON body → `422` or `400` (decide; **prefer 422** for Go-private route consistency), body has `error` containing `"arguments"`.
- [ ] Run `go test ./internal/handlers/ -run TestMCPHandler -v` — expect compile errors.
- [ ] **Rewrite** `handlers/mcp.go` (delete the existing file content, replace with):
  ```go
  package handlers

  import (
      "errors"
      "net/http"

      "github.com/gin-gonic/gin"
      "github.com/odysseythink/hermind/backend/internal/mcp"
      "github.com/odysseythink/hermind/backend/internal/middleware"
      "github.com/odysseythink/hermind/backend/internal/services"
  )

  type MCPHandler struct {
      svc *services.MCPService
  }

  func NewMCPHandler(svc *services.MCPService) *MCPHandler {
      return &MCPHandler{svc: svc}
  }

  func (h *MCPHandler) ListServers(c *gin.Context) {
      servers, err := h.svc.Servers(c.Request.Context())
      if err != nil {
          c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
          return
      }
      c.JSON(http.StatusOK, gin.H{"success": true, "servers": servers})
  }

  func (h *MCPHandler) ForceReload(c *gin.Context) {
      servers, err := h.svc.Reload(c.Request.Context())
      if err != nil {
          c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "servers": []any{}})
          return
      }
      c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "servers": servers})
  }

  type mcpNameBody struct {
      Name string `json:"name"`
  }

  func (h *MCPHandler) ToggleServer(c *gin.Context) {
      var body mcpNameBody
      if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
          c.JSON(http.StatusOK, gin.H{"success": false, "error": "name is required"})
          return
      }
      _, err := h.svc.ToggleServer(c.Request.Context(), body.Name)
      if err != nil {
          c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
          return
      }
      c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
  }

  func (h *MCPHandler) DeleteServer(c *gin.Context) {
      var body mcpNameBody
      if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
          c.JSON(http.StatusOK, gin.H{"success": false, "error": "name is required"})
          return
      }
      _, err := h.svc.DeleteServer(c.Request.Context(), body.Name)
      if err != nil {
          c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
          return
      }
      c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
  }

  type mcpToggleToolBody struct {
      ServerName string `json:"serverName"`
      ToolName   string `json:"toolName"`
      Enabled    bool   `json:"enabled"`
  }

  func (h *MCPHandler) ToggleTool(c *gin.Context) {
      var body mcpToggleToolBody
      if err := c.ShouldBindJSON(&body); err != nil || body.ServerName == "" || body.ToolName == "" {
          c.JSON(http.StatusOK, gin.H{
              "success":         false,
              "error":           "serverName and toolName are required",
              "suppressedTools": []string{},
          })
          return
      }
      suppressed, err := h.svc.ToggleTool(c.Request.Context(), body.ServerName, body.ToolName, body.Enabled)
      if err != nil {
          c.JSON(http.StatusOK, gin.H{
              "success":         false,
              "error":           err.Error(),
              "suppressedTools": []string{},
          })
          return
      }
      if suppressed == nil {
          suppressed = []string{}
      }
      c.JSON(http.StatusOK, gin.H{
          "success":         true,
          "error":           nil,
          "suppressedTools": suppressed,
      })
  }

  type mcpCallToolBody struct {
      Arguments map[string]any `json:"arguments"`
  }

  func (h *MCPHandler) CallTool(c *gin.Context) {
      name := c.Param("name")
      tool := c.Param("tool")
      if name == "" || tool == "" {
          c.JSON(http.StatusBadRequest, gin.H{"success": false, "result": nil, "error": "name and tool are required"})
          return
      }
      var body mcpCallToolBody
      if err := c.ShouldBindJSON(&body); err != nil {
          c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "result": nil, "error": "invalid arguments payload: " + err.Error()})
          return
      }
      result, err := h.svc.CallTool(c.Request.Context(), name, tool, body.Arguments)
      if err != nil {
          status := http.StatusBadGateway
          if errors.Is(err, mcp.ErrServerNotFound) {
              status = http.StatusNotFound
          }
          c.JSON(status, gin.H{"success": false, "result": nil, "error": err.Error()})
          return
      }
      c.JSON(http.StatusOK, gin.H{"success": true, "result": result, "error": nil})
  }

  // RegisterMCPRoutes wires the 5 Node-parity admin routes + 1 Go-private
  // tool-call proxy under the supplied router group. All routes require an
  // authenticated admin user.
  func RegisterMCPRoutes(r *gin.RouterGroup, authSvc *services.AuthService, svc *services.MCPService) {
      h := NewMCPHandler(svc)
      admin := []gin.HandlerFunc{
          middleware.ValidatedRequest(authSvc),
          middleware.FlexUserRoleValid([]string{"admin"}),
      }

      r.GET("/mcp-servers/force-reload", append(admin, h.ForceReload)...)
      r.GET("/mcp-servers/list",          append(admin, h.ListServers)...)
      r.POST("/mcp-servers/toggle",       append(admin, h.ToggleServer)...)
      r.POST("/mcp-servers/delete",       append(admin, h.DeleteServer)...)
      r.POST("/mcp-servers/toggle-tool",  append(admin, h.ToggleTool)...)

      // Go-private tool-call proxy (not Node-compat)
      r.POST("/mcp/:name/tools/:tool/call", append(admin, h.CallTool)...)
  }
  ```
- [ ] Run `go test ./internal/handlers/ -run TestMCPHandler -v` — expect all green.
- [ ] Run `go test ./internal/handlers/ -v` — full handlers package green (no regression).
- [ ] Commit: `feat(mcp): rewrite handlers + add tool-call REST proxy`.

### Acceptance

- 12+ handler tests pass.
- Existing handler tests still pass (`go test ./internal/handlers/`).
- Node-parity JSON shape verified by assertion on response body.
- `RegisterMCPRoutes` signature now takes `(api, authSvc, svc)`.

---

## Task 6: Wire singleton into main.go

**File:** `cmd/server/main.go`

**Test:** none directly (the rewired call is type-checked by the compiler; tests already exercise the handler layer).

### Steps

- [ ] Read `cmd/server/main.go` and locate two anchor points:
  1. Service construction block (where `authSvc`, `adminSvc` etc. are built).
  2. `handlers.RegisterMCPRoutes(api, authSvc)` line (~line 156).
- [ ] Add MCP service construction near the other service constructors:
  ```go
  mcpHyp := mcp.Instance(cfg)
  if err := mcpHyp.Boot(context.Background()); err != nil {
      log.Printf("mcp boot warning: %v", err)
  }
  mcpSvc := services.NewMCPService(mcpHyp)
  ```
  (Add imports: `"github.com/odysseythink/hermind/backend/internal/mcp"` and `"context"` if not already present.)
- [ ] Update the call to:
  ```go
  handlers.RegisterMCPRoutes(api, authSvc, mcpSvc)
  ```
- [ ] Run `go build ./cmd/server/` — expect success.
- [ ] Run `go test ./...` — full test suite green.
- [ ] Run `go vet ./...` — clean.
- [ ] Manual smoke (optional, only if dev env wired):
  ```bash
  STORAGE_DIR=/tmp/mcp-test go run ./cmd/server &
  curl -s http://localhost:3001/api/mcp-servers/list | jq .
  # Expect: {"success":true,"servers":[]}
  ls /tmp/mcp-test/plugins/hermind_mcp_servers.json
  # Expect: file exists with content {"mcpServers":{}}
  ```
- [ ] Commit: `feat(mcp): wire singleton + service into main`.

### Acceptance

- `go build ./...` succeeds.
- `go test ./...` all green.
- `go vet ./...` clean.
- Cold-start of backend creates `<StorageDir>/plugins/hermind_mcp_servers.json` with `{"mcpServers":{}}`.
- `GET /api/mcp-servers/list` returns `{success:true, servers:[]}`.

---

## Post-PR checklist

- [ ] Verify no new dependencies in `go.mod` (no `mcp-go`, no `modelcontextprotocol/go-sdk` yet — those land in PR-B).
- [ ] Verify `go test ./internal/mcp/ -v` reports ≥ 30 tests passing (14 config + 16 hypervisor).
- [ ] Verify `go test ./internal/handlers/ -run TestMCPHandler -v` reports ≥ 12 tests passing.
- [ ] Verify `internal/mcp/` is documented at the package level — add `// Package mcp implements the MCP hypervisor: lifecycle, config, and transport abstraction. PR-A ships skeleton + REST only; transports land in PR-B/C.` to top of `types.go`.
- [ ] Update `.gpowers/designs/2026-05-25-api-v1-layer-design.md` §9 to mark `mcp-servers/*` as "PR-A landed, transport pending".

---

## Risk notes

- **JSON map iteration order** in `Servers()` — Go maps are unordered. Tests must assert by name, not by index. If frontend depends on deterministic order, add sort in Task 3 (`sort.Slice(out, func(i,j int){ return out[i].Name < out[j].Name })`).
- **Windows `os.Rename` over existing** — the fallback to `os.WriteFile` accepts a race window. Acceptable for PR-A; PR-B may revisit if concurrent writers become a concern.
- **Singleton in test environment** — `NewHypervisorForTesting` exists precisely so test files never touch `Instance(cfg)`. Reviewers should reject any test that calls `Instance(...)`.
- **`error: "MCP transport not implemented"` is observable in production** — make sure frontend gracefully handles per-server `error` strings. The existing UI already displays these (Node's failed-boot states use the same field).

---

## Estimate

- Task 1: 30 min
- Task 2: 2-3 h (14 tests + implementation)
- Task 3: 3-4 h (16 tests + implementation)
- Task 4: 15 min
- Task 5: 2-3 h (12 tests + rewrite)
- Task 6: 30 min

**Total: ~9-11 hours single-track**, or 1 working day.
