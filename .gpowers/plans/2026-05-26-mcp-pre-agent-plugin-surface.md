# MCP Hypervisor PR-E — Agent Plugin Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose a Go-side equivalent of Node's `activeMCPServers()` + `convertServerToolsToPlugins()` so a future Go agent framework can discover and invoke MCP-backed tools as agent plugins. PR-E ships *only* the public surface + tests; there is no in-tree caller yet, by design.

**Why now:** locking the contract before the agent framework lands gives that framework a stable target to compile against. The contract is intentionally narrow (one struct, two methods) so when the framework appears it can either consume directly or adapt with minimal glue.

**Architecture:** Two new public methods on `*mcp.Hypervisor` (`ActiveServers`, `ToolsAsPlugins`) plus one new value type `mcp.ToolPlugin`. Both delegate to existing `mcps[name].tools` cache from PR-D — no new lookups, no new state. The service facade gains pass-through wrappers so `services.MCPService` stays the single integration point for any non-mcp package.

**Tech Stack:** Go 1.22+, testify. No new dependencies.

**Source spec:** `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §3.5 (Phase 2 预留), §11 step 5.

**Reference Node implementation:**
- `server/utils/MCP/index.js:17-20` (`activeMCPServers` — `@@mcp_{name}` prefix convention)
- `server/utils/MCP/index.js:28-127` (`convertServerToolsToPlugins` — closes over `mcpLayer.mcps[name]` and dispatches via `currentMcp.callTool`)
- `server/utils/agents/index.js:533-545` (the consumer side — how aibitat resolves `@@mcp_*` entries to plugins)

**Prerequisite:** PR-A, PR-B, PR-C, PR-D landed. The tool-schema cache (`activeClient.tools` populated at Boot) from PR-D is the substrate PR-E reads from.

---

## Pre-task: Read this section once before starting

### Existing Go surface (from PR-A through PR-D)

- `mcp.Hypervisor.mcps[name].tools []ToolSchema` — populated by `startServerLocked` after `Connect` succeeds. PR-E reads this; no further population needed.
- `mcp.Hypervisor.mcps[name].schemaByName map[string]json.RawMessage` — same source; PR-E can use either field (preferring `tools` since it carries `Description`).
- `mcp.ToolSchema {Name, Description, InputSchema}` — already a clean value type; PR-E's `ToolPlugin` embeds equivalent fields.
- `mcp.ServerConfig.Hermind.SuppressedTools []string` — filter source. The same filter logic used by `Servers()` in PR-C must be applied here so plugin lists match what Admin UI shows.
- `mcp.Hypervisor.CallTool(ctx, server, tool, args)` — the dispatch primitive. `ToolPlugin.Call` is a thin closure around it.
- No `services.MCPService.ActiveServers/ToolsAsPlugins` exist yet — PR-E adds them.

### Naming convention (Node parity)

- An "active MCP server" is rendered as the string `@@mcp_<server-name>`. The `@@mcp_` prefix is the marker Node's aibitat checks for to know "expand this into all of that server's tools" — see Node `server/utils/agents/index.js:539`.
- A tool's "qualified name" is `<server>-<tool>`. PR-E uses the same string in `ToolPlugin.QualifiedName` so a future Go agent framework can mirror Node's plugin map keying.

### Public API to ship (PR-E scope)

| # | Owner | Signature |
|---|---|---|
| 1 | `mcp` | `type ToolPlugin struct { ServerName, ToolName, QualifiedName, Description string; InputSchema json.RawMessage; Call func(ctx, map[string]any) (any, error) }` |
| 2 | `mcp.Hypervisor` | `ActiveServers() []string` — returns `@@mcp_<name>` for every currently-running server |
| 3 | `mcp.Hypervisor` | `ToolsAsPlugins(name string) ([]ToolPlugin, error)` — returns one server's non-suppressed tools wrapped as plugins; `ErrServerNotFound` if the server isn't running |
| 4 | `services.MCPService` | `ActiveServers() []string` — pass-through |
| 5 | `services.MCPService` | `ToolsAsPlugins(name string) ([]mcp.ToolPlugin, error)` — pass-through |

### Out of scope (explicit)

- **Agent framework integration** — there is no `Go agent` yet. PR-E ships the contract; the consumer lands in a future Phase 2 PR.
- **`ToolPlugin.Call` going through the PR-D concurrency limiter / audit log** — the HTTP-boundary hardening from PR-D is for **the REST route**, not for in-process agent invocations. The agent framework adds its own throttling and observability if/when it wants them. Document explicitly in the godoc.
- **Plugin name collisions across MCP servers** — `QualifiedName = "<server>-<tool>"` is the namespace. If two servers have a tool of the same short name, agent framework gets two distinct plugins keyed by qualified name; that's the agent framework's problem to surface.
- **Streaming tool results** — `Call` returns `(any, error)` synchronously; matches PR-D's REST handler.
- **`@agent` / `@@mcp_*` parsing logic** — that's the agent framework's job. PR-E provides only the producer side.
- **HTTP route exposure** — no new REST endpoints. This is an in-process Go API only.

### Data invariants

- `ActiveServers()` returns servers whose `mcps[name]` entry exists. A server that failed to boot (entry in `mcpLoadingResults` only, not in `mcps`) is excluded.
- The slice returned by `ActiveServers()` is sorted lexicographically by server name for deterministic agent behaviour.
- `ToolsAsPlugins(name)` returns `nil, ErrServerNotFound` if the server isn't running (vs PR-D's `GetToolSchema` which returns the same error for the same condition — keep the error type consistent).
- `ToolsAsPlugins(name)` returns `[]ToolPlugin{}` (non-nil, zero-length) if the server is running but all tools are suppressed. Callers can treat this as "server has nothing useful to offer right now" without nil-checking.
- The `Call` closure in each returned `ToolPlugin` MUST capture `name` and `toolName` by value, NOT by reference to loop variables. Otherwise all plugins from one ToolsAsPlugins call dispatch to the same (last) tool. Verify with a dedicated test (see Task 3).
- `Call` does NOT check suppression at invocation time. A consumer that holds a stale `ToolPlugin` after the operator suppresses the tool can still invoke it. (Suppression is for *discovery*, not *enforcement*; matches Node's model.)
- `Call`'s underlying transport call uses the `ctx` passed in. No internal timeout, no internal retries.

### TDD discipline

Each task follows: write failing test → run + confirm fail → implement → run + confirm pass → commit. Closure-capture bugs are the highest-risk class here — write the loop-variable-capture test before implementing the closure.

---

## Task 1: ToolPlugin type + Hypervisor.ActiveServers

**Files:** `internal/mcp/plugins.go` *(new)*, `internal/mcp/plugins_test.go` *(new)*

### Steps

- [ ] **Write failing tests** `plugins_test.go`:
  - `TestHypervisor_ActiveServers_Empty` — fresh hypervisor, never booted, `ActiveServers()` returns `[]string{}` (zero-length, non-nil).
  - `TestHypervisor_ActiveServers_OneRunning` — boot echo fixture; `ActiveServers()` returns `["@@mcp_echo-mcp"]`.
  - `TestHypervisor_ActiveServers_TwoRunning_Sorted` — boot two fixtures named e.g. `"zebra"` and `"alpha"`; assert returned order is `["@@mcp_alpha", "@@mcp_zebra"]`.
  - `TestHypervisor_ActiveServers_ExcludesFailedBoot` — seed config with one valid server + one with bogus command; Boot; `ActiveServers()` returns only the successful one.
  - `TestHypervisor_ActiveServers_ExcludesPrunedServer` — boot two; prune one via `ToggleServer`; assert returned slice has only the remaining server.
  - `TestHypervisor_ActiveServers_AutoStartFalseExcluded` — server with `autoStart:false`; Boot; `ActiveServers()` does not list it.
- [ ] Run tests — expect compile errors.
- [ ] **Implement** `internal/mcp/plugins.go`:
  ```go
  package mcp

  import (
      "context"
      "encoding/json"
      "fmt"
      "sort"
  )

  // ToolPlugin packages a single MCP-exposed tool for consumption by Go
  // agent frameworks. The Call closure dispatches to the live MCP server;
  // callers control timeouts and cancellation via the passed-in context.
  //
  // ToolPlugin values are produced by Hypervisor.ToolsAsPlugins and are
  // safe to copy. The Call closure holds an internal reference to the
  // Hypervisor, so a plugin obtained while server X was running will
  // surface an ErrServerNotFound from Call after X is toggled off.
  //
  // Note: ToolPlugin.Call does NOT route through the PR-D HTTP-boundary
  // concurrency limiter or audit log. Those are concerns of the REST
  // route; agent frameworks add their own throttling and observability.
  type ToolPlugin struct {
      // ServerName is the configured MCP server name (e.g. "docker-mcp").
      ServerName string
      // ToolName is the tool's name as advertised by the server (e.g. "list-containers").
      ToolName string
      // QualifiedName is "<ServerName>-<ToolName>". Matches Node's plugin naming.
      QualifiedName string
      // Description is the human-readable tool description from the server.
      Description string
      // InputSchema is the JSON Schema for the tool's arguments (may be empty/nil).
      InputSchema json.RawMessage
      // Call invokes the underlying MCP tool with the given arguments. The
      // result is whatever the MCP server returned (typically an object with
      // `content` array). The caller controls timeout/cancellation via ctx.
      Call func(ctx context.Context, args map[string]any) (any, error)
  }

  // ActiveServers returns the list of currently-running MCP servers tagged
  // with the "@@mcp_" prefix that agent frameworks use to mark MCP-sourced
  // plugin sets in their plugin config arrays. The list is sorted
  // lexicographically by server name for deterministic agent behaviour.
  //
  // A server appears in the list iff it is present in the running mcps
  // map — failed boots, autoStart=false servers, and pruned servers are
  // all excluded.
  func (h *Hypervisor) ActiveServers() []string {
      h.mu.RLock()
      defer h.mu.RUnlock()
      out := make([]string, 0, len(h.mcps))
      for name := range h.mcps {
          out = append(out, "@@mcp_"+name)
      }
      sort.Strings(out)
      return out
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestHypervisor_ActiveServers -v -count=1` — expect all green.
- [ ] Commit: `feat(mcp): ToolPlugin type + ActiveServers for agent integration`.

### Acceptance

- 6 ActiveServers tests pass.
- Returned slice is always sorted and non-nil (`[]string{}` for empty, never `nil`).
- `ToolPlugin` type is documented at the godoc level explaining its non-coupling to PR-D's HTTP hardening.

---

## Task 2: Hypervisor.ToolsAsPlugins (without Call closure)

**Files:** `internal/mcp/plugins.go` *(extend)*, `internal/mcp/plugins_test.go` *(extend)*

> Split into two tasks: this one ships the schema-projection logic (tool list → ToolPlugin metadata) without wiring the Call closure. Task 3 wires Call. The split makes the closure-capture failure mode trivial to test in isolation.

### Steps

- [ ] **Write failing tests** (extend `plugins_test.go`):
  - `TestHypervisor_ToolsAsPlugins_ServerNotFound` — `ToolsAsPlugins("ghost")` returns `(nil, error)` where `errors.Is(err, ErrServerNotFound)` is true.
  - `TestHypervisor_ToolsAsPlugins_PrunedServer` — boot, prune, then call; same error.
  - `TestHypervisor_ToolsAsPlugins_ReturnsAllTools` — boot echo fixture (3 tools); assert returned slice has 3 plugins with `ServerName="echo-mcp"`, `ToolName` ∈ `[echo, add, slow_echo]`, `QualifiedName="echo-mcp-<tool>"`, and a non-nil `Call` (closure presence; we'll verify it dispatches correctly in Task 3).
  - `TestHypervisor_ToolsAsPlugins_FiltersSuppressed` — seed config with `suppressedTools:["add"]`; boot; `ToolsAsPlugins` returns 2 plugins (`echo`, `slow_echo`); `add` absent.
  - `TestHypervisor_ToolsAsPlugins_AllToolsSuppressed_EmptyNotNil` — suppress all 3 tools; assert returned slice is `[]ToolPlugin{}` (length 0, non-nil).
  - `TestHypervisor_ToolsAsPlugins_CarriesInputSchema` — assert the returned `InputSchema` for `echo` is byte-for-byte the same as `Hypervisor.GetToolSchema("echo-mcp","echo").InputSchema`.
  - `TestHypervisor_ToolsAsPlugins_NoInputSchemaToolHasNilField` — if a tool has no inputSchema in the source, the plugin's `InputSchema` is nil (or `len == 0`).
- [ ] Run tests — expect compile errors.
- [ ] **Extend** `plugins.go` with the projection logic. The `Call` closure is a placeholder `func() error { panic("wired in Task 3") }` — but typed correctly so the tests compile. Actually that would fail any test that runs the closure; instead **defer the Call wiring**:
  ```go
  // ToolsAsPlugins returns each non-suppressed tool on the named running
  // server as a ToolPlugin. The Call closure on each plugin dispatches to
  // the underlying MCP server.
  //
  // Returns an error wrapping ErrServerNotFound if the server is not
  // currently running. Returns ([]ToolPlugin{}, nil) — non-nil, zero-length —
  // if the server is running but every tool is suppressed.
  func (h *Hypervisor) ToolsAsPlugins(name string) ([]ToolPlugin, error) {
      h.mu.RLock()
      client, ok := h.mcps[name]
      // Capture the data we need under the lock; build the slice without it.
      var tools []ToolSchema
      if ok {
          tools = append(tools, client.tools...)
      }
      h.mu.RUnlock()

      if !ok {
          return nil, fmt.Errorf("%w: %s", ErrServerNotFound, name)
      }

      suppressed := h.suppressionSetForServer(name)
      out := make([]ToolPlugin, 0, len(tools))
      for i := range tools {
          t := tools[i]   // copy — critical for closure capture; see Task 3
          if _, blocked := suppressed[t.Name]; blocked {
              continue
          }
          out = append(out, ToolPlugin{
              ServerName:    name,
              ToolName:      t.Name,
              QualifiedName: name + "-" + t.Name,
              Description:   t.Description,
              InputSchema:   t.InputSchema,
              // Call wired in Task 3.
          })
      }
      return out, nil
  }

  // suppressionSetForServer reads the per-server suppressed-tool list from
  // the JSON config file. Returns an empty (non-nil) set on miss.
  func (h *Hypervisor) suppressionSetForServer(name string) map[string]struct{} {
      list := h.file.GetSuppressedTools(name)
      set := make(map[string]struct{}, len(list))
      for _, s := range list { set[s] = struct{}{} }
      return set
  }
  ```
- [ ] Run `go test ./internal/mcp/ -run TestHypervisor_ToolsAsPlugins -v -count=1` — expect all green (Call closures aren't exercised yet, just metadata).
- [ ] Commit: `feat(mcp): ToolsAsPlugins projection (Call wiring pending Task 3)`.

### Acceptance

- 7 metadata tests pass.
- Suppression filter applied via `Config.GetSuppressedTools` (round-trips with the same source PR-D's `Servers()` uses).
- Server-not-found error is wrapped, detectable via `errors.Is(err, mcp.ErrServerNotFound)`.

---

## Task 3: ToolPlugin.Call closure + capture safety

**Files:** `internal/mcp/plugins.go` *(extend)*, `internal/mcp/plugins_test.go` *(extend)*

### Steps

- [ ] **Write failing tests** (extend `plugins_test.go`):
  - `TestToolPlugin_Call_RoundtripEcho` — boot echo; `plugins, _ := ToolsAsPlugins("echo-mcp")`; find `echo` plugin; `result, err := plugins[i].Call(ctx, map[string]any{"text":"hi"})`; assert no error, result string-form contains `"hi"`.
  - `TestToolPlugin_Call_RoundtripAdd` — similar with `add(a:2,b:3)`, expect result string-form contains `"sum=5"`.
  - `TestToolPlugin_Call_ServerPrunedAfterPluginObtained` — capture plugin; prune server; call returns an error wrapping `ErrServerNotFound` (verifying the closure looks up the hypervisor's *current* state, not a stale transport reference).
  - **`TestToolPlugin_Call_NoLoopVariableCaptureBug`** — the critical safety test:
    ```go
    plugins, err := hv.ToolsAsPlugins("echo-mcp")
    require.NoError(t, err)
    require.GreaterOrEqual(t, len(plugins), 2)
    // Snapshot ALL plugins first, then call each — ensures any closure-capture
    // bug surfaces as "all plugins called the last tool".
    results := make(map[string]any)
    for _, p := range plugins {
        r, callErr := p.Call(context.Background(), defaultArgsFor(t, p.ToolName))
        require.NoError(t, callErr)
        results[p.QualifiedName] = r
    }
    // Each plugin must have returned a distinct result keyed by qualifiedName.
    assert.Len(t, results, len(plugins))
    ```
    This test FAILS if `ToolName` is captured by reference (all calls would invoke the same — typically last — tool).
  - `TestToolPlugin_Call_ContextCanceled` — pass a pre-canceled ctx; assert call returns `context.Canceled` (or wrapped equivalent).
  - `TestToolPlugin_Call_ContextDeadline` — slow_echo configured to take 5s, pass 100ms ctx; assert `context.DeadlineExceeded`.
- [ ] Run tests — expect closure-capture test to fail in obvious ways (or pass coincidentally — the implementation in Task 2 was already written with proper `t := tools[i]` copy + plugin-by-value capture, so the closure should be safe by construction; this test exists to guard against future regressions).
- [ ] **Implement** Call wiring inside `ToolsAsPlugins`:
  ```go
  // ... inside the for-loop, replace the Call: nil placeholder with:
  server, tool := name, t.Name           // explicit copy into closure scope
  call := func(ctx context.Context, args map[string]any) (any, error) {
      return h.CallTool(ctx, server, tool, args)
  }
  out = append(out, ToolPlugin{
      ServerName:    server,
      ToolName:      tool,
      QualifiedName: server + "-" + tool,
      Description:   t.Description,
      InputSchema:   t.InputSchema,
      Call:          call,
  })
  ```
- [ ] Run `go test ./internal/mcp/ -run "TestToolPlugin_Call|TestHypervisor_ToolsAsPlugins" -v -count=1 -race` — expect all green.
- [ ] Run `go test ./internal/mcp/ -v -count=1 -race` — full package green.
- [ ] Commit: `feat(mcp): ToolPlugin.Call closure with capture-safe dispatch`.

### Acceptance

- 6 closure tests pass, including the loop-variable-capture guard.
- `Call` honours caller-supplied context cancellation and deadlines.
- `-race` clean across the full mcp package.

---

## Task 4: Service facade pass-through

**Files:** `internal/services/mcp_service.go` *(extend)*

**Test:** none — facade is a pure pass-through; coverage via Hypervisor tests is sufficient.

### Steps

- [ ] Append to `services/mcp_service.go`:
  ```go
  // ActiveServers returns "@@mcp_<name>" identifiers for each running MCP
  // server. Intended for consumption by a Go agent framework's plugin
  // resolver. See mcp.Hypervisor.ActiveServers for full semantics.
  func (s *MCPService) ActiveServers() []string {
      return s.hv.ActiveServers()
  }

  // ToolsAsPlugins returns each non-suppressed tool on the named running
  // server as an mcp.ToolPlugin. Returns an error wrapping
  // mcp.ErrServerNotFound if the server is not running.
  func (s *MCPService) ToolsAsPlugins(name string) ([]mcp.ToolPlugin, error) {
      return s.hv.ToolsAsPlugins(name)
  }
  ```
- [ ] Run `go build ./internal/services/...` — expect success.
- [ ] Run `go test ./internal/services/ -v -count=1` — full services green (no new tests, but verify no regression).
- [ ] Commit: `feat(mcp): expose ActiveServers/ToolsAsPlugins via service facade`.

### Acceptance

- Compiles cleanly.
- Pass-through methods take and return exactly the same types as the hypervisor methods (no transformation, no `*mcp.ToolPlugin` ↔ `services.ToolPluginDTO` indirection).

---

## Task 5: Documentation + design cross-reference

**Files:** `internal/mcp/doc.go` *(extend)*, `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` *(update §11)*

### Steps

- [ ] **Append** to `internal/mcp/doc.go` (or wherever the package doc lives):
  ```go
  // Agent plugin surface
  //
  // The hypervisor exposes ActiveServers() and ToolsAsPlugins(name) so a
  // future Go agent framework can discover and invoke MCP-backed tools.
  //
  // ActiveServers returns ["@@mcp_<name>", ...] identifiers — the same
  // convention Node's aibitat uses to mark "expand this into all of that
  // server's tools" entries in a plugin list.
  //
  // ToolsAsPlugins(name) returns one server's non-suppressed tools as
  // ToolPlugin values. Each ToolPlugin carries the tool's name,
  // description, input schema, and a Call closure that dispatches to the
  // live MCP server.
  //
  // ToolPlugin.Call does NOT route through the REST handler's HTTP-boundary
  // concurrency limiter or audit logger from PR-D — those are for the
  // /api/mcp/.../tools/.../call route only. Agent frameworks should add
  // their own throttling and observability if desired.
  //
  // The plugin surface is in-process Go API only. No new REST endpoints.
  ```
- [ ] **Update** `.gpowers/designs/2026-05-26-mcp-hypervisor-design.md` §11 (下一步) to mark PR-E delivered, e.g. cross out step 3 ("PR-E（plugin 预留）") and append a `## 12. Delivered` section listing PR-A through PR-E with one-line summaries each.
- [ ] Run `go doc ./internal/mcp/` — verify the agent-plugin surface section renders.
- [ ] Run `go vet ./...` — clean.
- [ ] Run `go test ./... -count=1 -race` — full repo green.
- [ ] Commit: `docs(mcp): document agent plugin surface; mark PR-E delivered`.

### Acceptance

- Package godoc has a discoverable "Agent plugin surface" section.
- Design doc §11 step 3 is crossed out / §12 added with PR-E summary.
- Full test suite green under `-race`.

---

## Post-PR checklist

- [ ] `go test ./internal/mcp/ -count=1 -race -v` reports ≥ 95 tests passing (PR-A ~30 + PR-B ~25 + PR-C ~30 + PR-D ~5 in mcp/ + PR-E ~13).
- [ ] `go vet ./...` clean.
- [ ] No new dependencies in `go.mod`.
- [ ] `services.MCPService` has `ActiveServers()` and `ToolsAsPlugins(name)` exported.
- [ ] `mcp.ToolPlugin` is documented + has a closure-capture safety test guarding it.
- [ ] Design doc §11/§12 updated.
- [ ] **Forward-compat smoke** (manual, optional): in a scratch branch, write a 30-line skeleton "Go agent" that calls `svc.ActiveServers()` + `svc.ToolsAsPlugins("echo-mcp")` and invokes one plugin. If it compiles and runs against PR-E's API, the contract is usable. Do not merge the skeleton.

---

## Risk notes

- **API committed without a consumer** — by design. The closest we get to a consumer is the manual forward-compat smoke. If the future Go agent framework needs richer semantics (e.g. tool-result type marshalling, plugin-level cancellation, parallel batched calls), PR-E's surface can be **extended** (additive) without breaking. **Renaming** or **changing return types** would be breaking — avoid in follow-ups.
- **Closure capture is fragile** — Go's loop variables changed semantics in 1.22 (per-iteration scoping), so a fresh codebase is less likely to hit the classic bug. The dedicated capture-safety test exists as defence-in-depth: someone may copy this code into a 1.21 module someday.
- **`ToolPlugin.Call` bypasses PR-D's concurrency limiter** — intentional, documented. If the future agent framework hammers an MCP server, that's its problem to solve. Document in the agent-framework planning doc when it's drafted.
- **Suppression cache vs. file source-of-truth** — `suppressionSetForServer` reads the JSON file fresh per call. That's deliberate: if the operator toggles a tool while an agent loop is running, the *next* `ToolsAsPlugins` call reflects the change immediately. But an agent that holds onto a stale `ToolPlugin` slice can still invoke a suppressed tool. Document.
- **Sort stability of ActiveServers** — `sort.Strings` is unstable in the rare case of duplicate keys, but server names are guaranteed unique (JSON object keys). Safe.
- **`ToolsAsPlugins` returns plugins that may outlive the server** — if the agent framework caches the returned slice and the operator toggles the server off, calls return `ErrServerNotFound`. That's the desired behaviour (fail fast); document so the framework doesn't suppress the error.

---

## Estimate

- Task 1 (ToolPlugin + ActiveServers): 1.5 h
- Task 2 (ToolsAsPlugins projection): 2 h
- Task 3 (Call closure + capture safety): 2 h
- Task 4 (service facade): 15 min
- Task 5 (docs): 30 min

**Total: ~6 hours**, well under one working day. PR-E is the smallest of the five.
