# Agent Runtime PR-AR-6 — sql-agent + filesystem-agent + create-files-agent Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining v1 default-skill gap by landing the three local skills that don't need OAuth or third-party providers: **sql-agent** (list-databases / list-tables / get-table-schema / run-query against admin-configured connections), **filesystem-agent** (list/read/write/edit/move/copy/search files within a sandboxed root), and **create-files-agent** (generate txt/md/docx/pdf/pptx/xlsx files into `STORAGE_DIR/generated-files/`). All three are wrapped by PR-AR-5's approval gate. After PR-AR-6 lands, the Go default-skill set matches Node's exactly (except for OAuth-bound gmail/outlook/google-calendar, which are v3).

**Architecture:**
- Each skill is a single `tool.Entry` registered in `tools/builder.go` Source 1 (default skills) — same shape as PR-AR-3's four existing skills. Skill registration is **gated by `CheckFn`** so an unconfigured environment doesn't expose the tool to the LLM.
- **sql-agent** runs as **one fat tool** with `action` enum (`list_databases`/`list_tables`/`get_schema`/`query`) — same as rag-memory's two-action shape. Connections come from `SystemSetting{key: "agent_sql_connections"}` (Node-compatible JSON). Drivers reuse already-imported `lib/pq`, `mattn/go-sqlite3`; **MSSQL via new `github.com/microsoft/go-mssqldb`** (the only new dep).
- **filesystem-agent** runs as one fat tool with `action` enum (`list_dir`/`read_file`/`write_file`/`edit_file`/`move_file`/`copy_file`/`search_files`/`get_info`/`create_dir`). Sandboxed under `STORAGE_DIR/anythingllm-fs/` by default; root is overridable via `cfg.AgentFilesystemRoot`. **All paths go through a `safeJoin(root, userPath)` guard** that defends against `..` traversal + symlink escape.
- **create-files-agent** runs as one tool with `format` enum (`txt`/`md`/`docx`/`pdf`/`pptx`/`xlsx`). `txt`/`md` are stdlib; the rest pull in **`github.com/unidoc/unioffice`** (commercial but free for AGPL — already used by some Go AnythingLLM forks; needs decision artefact) **OR** stub three formats to "coming-in-PR-AR-6.1" and ship only `txt`/`md` in this PR. **Decision deferred to Task 0**; recommendation = ship `txt`/`md` only, defer the four binary formats to a follow-up.
- All three skills are tagged `approval-required` in `Builder.addWithApproval` — even though they're "default skills" (not MCP/Flow), the destructive nature (DB writes inadvertently allowed, file writes, generated artifacts) warrants the gate. **Or**: only `sql-query` mutations and `filesystem.write/edit/move/copy/create_dir` + all `create-files` actions get the gate; read-only actions bypass. **Pick at Task 1 (the second, finer-grained approach is cleaner).**

**Tech Stack:** Go 1.25.5; new deps `github.com/microsoft/go-mssqldb` (sql-server driver). Stdlib only for filesystem-agent. `txt`/`md` only for create-files in this PR (other formats deferred). All pantheon + gorilla deps already in `go.mod`.

**Source spec:** `.gpowers/designs/2026-05-26-agent-runtime-design.md` §8 (skill list), §14 (PR-AR-6 row, 10-12h estimate).

**Reference Node implementation:**
- `server/utils/agents/aibitat/plugins/sql-agent/{index.js, list-database.js, list-table.js, get-table-schema.js, query.js, SQLConnectors/}`
- `server/utils/agents/aibitat/plugins/filesystem/{index.js, lib.js, list-directory.js, read-text-file.js, write-text-file.js, edit-file.js, move-file.js, copy-file.js, search-files.js, get-file-info.js, create-directory.js, read-multiple-files.js}`
- `server/utils/agents/aibitat/plugins/create-files/{index.js, lib.js, text/, docx/, pdf/, pptx/, xlsx/}`
- `server/utils/agents/defaults.js:21-30` — `SKILL_FILTER_CONFIG` showing which skills need `isToolAvailable()` gating

---

## Pre-task: Read this section once before starting

### What landed in PR-AR-1 to PR-AR-5 (use, don't re-implement)

- `internal/agent/tools/context.go` — `ToolContext` holds Workspace/User/LM/VectorSearchSvc/DocSvc/MCPHv/FlowSvc/EventLog/Emit + Settings. PR-AR-6 **does not extend ToolContext**; SQL/FS-specific deps come from `BuilderDeps` directly.
- `internal/agent/tools/builder.go:39` `Build(ctx, ws, user, emit, settings)` — current Source 1 default-skill list:
  ```go
  for _, e := range []*tool.Entry{
      NewRAGMemorySkill(tc),
      NewDocSummarizerSkill(tc),
      NewWebScrapingSkill(tc),
      NewRechartSkill(tc),
  } { ... }
  ```
  PR-AR-6 appends **3 more entries**, each behind a `CheckFn` gate.
- `internal/agent/tools/builder.go:111` `addWithApproval(reg, seen, e, source, requiresApproval, globalAutoApprove)` — single registration funnel. PR-AR-6 calls with `requiresApproval=true` for the destructive actions.
- `internal/agent/tools/helpers.go` — `truncate(s, max)` used by other skills for bounded output. Reuse.
- `internal/config/config.go` — `StorageDir` already defined and `MkdirAll`'d at boot. PR-AR-6 adds `AgentFilesystemRoot` + `AgentCreateFilesDir` knobs (both default-derived from `StorageDir`).
- `internal/services/system_service.go:21` `GetSetting(ctx, key)` — used to read `agent_sql_connections` (Node-compat JSON).
- PR-AR-5's `Builder.addWithApproval` and per-session `RequestApproval` are the gate primitives — PR-AR-6 just sets the `requiresApproval` bool correctly per skill.

### Skill availability gates (CheckFn)

Mirrors Node's `SKILL_FILTER_CONFIG`:

| Skill | CheckFn returns true iff |
|---|---|
| sql-agent | `len(connections from agent_sql_connections SystemSetting) > 0` |
| filesystem-agent | Always **except** when `cfg.AgentFilesystemEnabled == false` (default true under dev/docker; we don't mirror Node's docker-only restriction — security relies on the sandbox guard instead) |
| create-files-agent | `cfg.AgentCreateFilesEnabled != false` (default true) |

> **Decision artefact** `.gpowers/decisions/2026-05-27-fs-skill-no-docker-restriction.md`: We do NOT replicate Node's docker-only filesystem-agent restriction. Reason: the sandbox `safeJoin` guard is strong enough that the docker restriction is belt-and-suspenders; users running outside docker also need the feature; admins who want to disable it can set `AGENT_FILESYSTEM_ENABLED=false`.

### `BuilderDeps` extensions

```go
// builder.go (additions)
type BuilderDeps struct {
    // ... existing fields ...
    Cfg *config.Config  // NEW — needed for AgentFilesystemRoot / AgentCreateFilesDir / enable flags
}
```

Why on `BuilderDeps` rather than `ToolContext`: these are static config, not per-session.

### New surface (this PR)

```
backend/internal/agent/tools/
├── sql_agent.go              # NEW — single fat tool with action enum
├── sql_agent_test.go         # NEW
├── sql_connections.go        # NEW — load + parse agent_sql_connections from SystemSetting
├── sql_connections_test.go   # NEW
├── sql_drivers.go            # NEW — driver dispatch (postgresql/mysql/sqlite/mssql)
├── sql_drivers_test.go       # NEW
├── filesystem_agent.go       # NEW — single fat tool with action enum
├── filesystem_agent_test.go  # NEW
├── filesystem_safejoin.go    # NEW — path-traversal guard
├── filesystem_safejoin_test.go # NEW
├── create_files_agent.go     # NEW — txt/md only (other formats deferred)
├── create_files_agent_test.go # NEW
└── builder.go                # MODIFY — append 3 skills to default-skill list with CheckFn

backend/internal/config/
└── config.go                 # MODIFY — add AgentFilesystemEnabled, AgentFilesystemRoot, AgentCreateFilesEnabled, AgentCreateFilesDir

backend/go.mod              # MODIFY — add github.com/microsoft/go-mssqldb
```

### Methods to ship (PR-AR-6 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `tools.NewSQLAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry` | one fat tool, action enum | |
| 2 | `tools.loadSQLConnections(sysSvc) ([]SQLConnection, error)` | parses `agent_sql_connections` JSON | |
| 3 | `tools.dispatchSQLDriver(engine, connString) (*sql.DB, error)` | switch over postgresql/mysql/sqlite/mssql | |
| 4 | `tools.NewFilesystemAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry` | one fat tool, action enum | |
| 5 | `tools.safeJoin(root, userPath string) (string, error)` | absolute-path + symlink-resolved containment check | |
| 6 | `tools.NewCreateFilesAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry` | format enum (`txt`/`md` only in this PR) | |

### Action enums per skill (mirrors Node tool names)

```
sql-agent (Tool name: "sql-agent"):
  - action="list_databases"          (read-only)  → bypass approval
  - action="list_tables"             (read-only)  → bypass approval
  - action="get_schema"              (read-only)  → bypass approval
  - action="query"                   (write?)     → APPROVAL REQUIRED (LLM may issue mutations despite "SELECT only" prompt)

filesystem-agent (Tool name: "filesystem-agent"):
  - action="list_dir"                (read-only)  → bypass approval
  - action="read_file"               (read-only)  → bypass approval
  - action="get_info"                (read-only)  → bypass approval
  - action="search_files"            (read-only)  → bypass approval
  - action="write_file"              (write)      → APPROVAL REQUIRED
  - action="edit_file"               (write)      → APPROVAL REQUIRED
  - action="move_file"               (write)      → APPROVAL REQUIRED
  - action="copy_file"               (write)      → APPROVAL REQUIRED
  - action="create_dir"              (write)      → APPROVAL REQUIRED

create-files-agent (Tool name: "create-files-agent"):
  - format="txt"                     (write)      → APPROVAL REQUIRED
  - format="md"                      (write)      → APPROVAL REQUIRED
  - format ∈ {docx,pdf,pptx,xlsx}    → return tool.Error("format X not yet implemented; PR-AR-6.1") — no file generated
```

### Approval gate granularity (action-level not tool-level)

PR-AR-5's `addWithApproval` operates at **tool** granularity (`requiresApproval bool`). For PR-AR-6 we need **action** granularity because read actions should bypass.

**Implementation choice**: Don't change `addWithApproval`. Instead, the **Handler closure itself** invokes `tc.Emit` and then calls into a private helper that decides whether to consult approval. Each skill Handler is responsible for routing to `b.deps.Approval(ctx, "<tool>:<action>", args, desc)` only on destructive actions.

To keep this clean, declare a helper on each Handler:

```go
// inside a Handler body for sql-agent:
if isDestructiveSQLAction(args.Action) && tc.Approval != nil {
    if approved, reason := tc.Approval(ctx, "sql-agent:"+args.Action, args, "Run SQL query on " + args.DatabaseID); !approved {
        return tool.Error("rejected: " + reason), nil
    }
}
```

> Requires exposing `Approval ApprovalFn` via `ToolContext` (currently lives on `BuilderDeps.Approval` from PR-AR-5). Task 1 step: **plumb `Approval` from `BuilderDeps.Approval` into `ToolContext.Approval` in `Build`**. This is a 1-line addition in builder.go.

### Out of scope (explicit)

- docx/pdf/pptx/xlsx file generation — deferred to **PR-AR-6.1** (separate plan; needs unioffice/gofpdf decision and license artefact)
- SQL connection **management UI** routes (add/edit/delete connections) — Node has `server/utils/helpers/admin/agents.js` for this; out of scope; admins write JSON directly via SystemSettings until a follow-up
- Read-multiple-files action — Node bundles it; Go LLM can issue multiple `read_file` calls
- File diff display (Node uses `diff` lib for `edit_file` preview) — Go returns raw before/after content; UI handles display
- `humanFileSize` formatting helper — keep raw bytes in the response; LLM formats if asked
- Cross-database `JOIN` across multiple `agent_sql_connections` — each query targets one DB
- Streaming SQL results — bounded to 100 rows per response
- Connection pool — open + close per query for simplicity; this is fine for the agent use case (low QPS)

### TDD discipline

Each task lands as **one commit**. Failing test → impl → green → full suite green → commit.

---

## Task 0: Decision artefacts + go.mod + ToolContext.Approval plumbing

**Files:**
- `.gpowers/decisions/2026-05-27-fs-skill-no-docker-restriction.md` (NEW)
- `.gpowers/decisions/2026-05-27-create-files-binary-formats-deferred.md` (NEW)
- `backend/go.mod` (MODIFY)
- `backend/internal/agent/tools/context.go` (MODIFY — add Approval field)
- `backend/internal/agent/tools/builder.go` (MODIFY — pass Approval through, add Cfg)
- `backend/internal/config/config.go` (MODIFY — 4 new knobs)

**Tests:** none yet (wiring + decision docs); compilation is the gate.

### Steps

- [ ] Add MSSQL driver:
  ```bash
  cd backend && go get github.com/microsoft/go-mssqldb@latest
  ```

- [ ] Add config knobs to `config.go`:
  ```go
  // === Agent skills ===
  AgentFilesystemEnabled  bool   `env:"AGENT_FILESYSTEM_ENABLED" envDefault:"true"`
  AgentFilesystemRoot     string `env:"AGENT_FILESYSTEM_ROOT"` // empty → <StorageDir>/anythingllm-fs
  AgentCreateFilesEnabled bool   `env:"AGENT_CREATE_FILES_ENABLED" envDefault:"true"`
  AgentCreateFilesDir     string `env:"AGENT_CREATE_FILES_DIR"` // empty → <StorageDir>/generated-files
  ```

  In `Load()`, after `MkdirAll(cfg.StorageDir)`:
  ```go
  if cfg.AgentFilesystemRoot == "" {
      cfg.AgentFilesystemRoot = filepath.Join(cfg.StorageDir, "anythingllm-fs")
  }
  if cfg.AgentCreateFilesDir == "" {
      cfg.AgentCreateFilesDir = filepath.Join(cfg.StorageDir, "generated-files")
  }
  _ = os.MkdirAll(cfg.AgentFilesystemRoot, 0o755)
  _ = os.MkdirAll(cfg.AgentCreateFilesDir, 0o755)
  ```

- [ ] Extend `ToolContext` with `Approval`:
  ```go
  // tools/context.go
  type ToolContext struct {
      // ... existing ...
      Approval ApprovalFn  // nil = no gate (test default)
      Cfg      *config.Config
  }
  ```

- [ ] Extend `BuilderDeps.Cfg` if not already present (PR-AR-5 may have added it; verify):
  ```go
  type BuilderDeps struct {
      // ... existing ...
      Cfg *config.Config
  }
  ```

- [ ] In `Build()`, populate `tc.Approval` + `tc.Cfg`:
  ```go
  tc := &ToolContext{
      // ... existing ...
      Approval: b.deps.Approval,
      Cfg:      b.deps.Cfg,
  }
  ```

- [ ] Write `.gpowers/decisions/2026-05-27-fs-skill-no-docker-restriction.md`:
  ```markdown
  # Filesystem Agent — No Docker-Only Restriction

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: Node's `filesystem-agent.isToolAvailable()` returns true only when `NODE_ENV=development` or `ANYTHING_LLM_RUNTIME=docker`. We don't replicate this.

  **Decision**: Go skill is enabled in any deployment, gated by `cfg.AgentFilesystemEnabled` (default true) and a strict `safeJoin` sandbox under `cfg.AgentFilesystemRoot`.

  **Rationale**: The safeJoin guard (absolute-path + symlink resolution + prefix check) is the real security boundary. The docker-only check is belt-and-suspenders. Admins who want the Node behaviour can set `AGENT_FILESYSTEM_ENABLED=false`.
  ```

- [ ] Write `.gpowers/decisions/2026-05-27-create-files-binary-formats-deferred.md`:
  ```markdown
  # Create-Files Skill — Binary Formats Deferred to PR-AR-6.1

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: Node's create-files-agent supports txt/md/docx/pdf/pptx/xlsx. Binary formats need unioffice or similar — non-trivial new dep + license discussion.

  **Decision**: PR-AR-6 ships txt/md only. docx/pdf/pptx/xlsx return `tool.Error("format X not yet implemented; PR-AR-6.1")`.

  **Rationale**: txt/md is 90% of agent file-generation use cases (memos, reports). The four binary formats are a separate concern that justifies its own license + dependency review.
  ```

- [ ] `go vet ./...` + `go build ./...` clean.

### Acceptance

- `go.mod` has `github.com/microsoft/go-mssqldb`
- 4 new config knobs parse cleanly with defaults
- `ToolContext.Approval` + `ToolContext.Cfg` populated from BuilderDeps
- Both decision artefacts present
- Full suite still passes

### Commit

`feat(agent/tools): plumb Approval+Cfg into ToolContext; add fs/createFiles knobs + MSSQL driver`

---

## Task 1: sql-agent — load connections + driver dispatch

**Files:**
- `backend/internal/agent/tools/sql_connections.go` (NEW)
- `backend/internal/agent/tools/sql_connections_test.go` (NEW)
- `backend/internal/agent/tools/sql_drivers.go` (NEW)
- `backend/internal/agent/tools/sql_drivers_test.go` (NEW)

**Tests:**
- `TestLoadSQLConnections_EmptySetting_ReturnsEmpty`
- `TestLoadSQLConnections_MalformedJSON_ReturnsErrorWithoutPanic`
- `TestLoadSQLConnections_ValidJSON_ReturnsParsedConnections`
- `TestLoadSQLConnections_FiltersByDatabaseID` (helper `findConnection`)
- `TestDispatchSQLDriver_Postgresql_OpensSuccessfully` (in-process sqlite via `mattn/go-sqlite3` registered as alias; use `:memory:` for real test)
- `TestDispatchSQLDriver_UnknownEngine_ReturnsError`
- `TestDispatchSQLDriver_SQLite_OpensSuccessfully`

### Steps

- [ ] Write failing `sql_connections_test.go`:
  ```go
  func TestLoadSQLConnections_ValidJSON_ReturnsParsedConnections(t *testing.T) {
      sysSvc := newMockSysSvc(map[string]string{
          "agent_sql_connections": `[
              {"database_id":"prod","engine":"postgresql","connectionString":"postgres://u:p@h/db"},
              {"database_id":"local","engine":"sqlite","connectionString":"file:./local.db"}
          ]`,
      })
      conns, err := tools.LoadSQLConnectionsForTesting(context.Background(), sysSvc)
      require.NoError(t, err)
      require.Len(t, conns, 2)
      require.Equal(t, "prod", conns[0].DatabaseID)
  }
  ```

- [ ] Implement `sql_connections.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/services"
  )

  type SQLConnection struct {
      DatabaseID       string `json:"database_id"`
      Engine           string `json:"engine"` // "postgresql"|"mysql"|"sqlite"|"sql-server"
      ConnectionString string `json:"connectionString"`
  }

  func loadSQLConnections(ctx context.Context, sysSvc *services.SystemService) ([]SQLConnection, error) {
      if sysSvc == nil { return nil, nil }
      raw, err := sysSvc.GetSetting(ctx, "agent_sql_connections")
      if err != nil { return nil, fmt.Errorf("get agent_sql_connections: %w", err) }
      raw = strings.TrimSpace(raw)
      if raw == "" || raw == "null" { return nil, nil }
      var out []SQLConnection
      if err := json.Unmarshal([]byte(raw), &out); err != nil {
          return nil, fmt.Errorf("parse agent_sql_connections: %w", err)
      }
      return out, nil
  }

  func findConnection(conns []SQLConnection, dbID string) (*SQLConnection, bool) {
      for i := range conns {
          if conns[i].DatabaseID == dbID { return &conns[i], true }
      }
      return nil, false
  }

  func LoadSQLConnectionsForTesting(ctx context.Context, sysSvc *services.SystemService) ([]SQLConnection, error) {
      return loadSQLConnections(ctx, sysSvc)
  }
  ```

- [ ] Implement `sql_drivers.go`:
  ```go
  package tools

  import (
      "database/sql"
      "fmt"

      _ "github.com/lib/pq"                 // postgres
      _ "github.com/mattn/go-sqlite3"       // sqlite
      _ "github.com/microsoft/go-mssqldb"   // sql-server
      // mysql driver: import "github.com/go-sql-driver/mysql"; ALREADY transitively present? verify.
  )

  func dispatchSQLDriver(engine, connString string) (*sql.DB, error) {
      switch engine {
      case "postgresql", "postgres":
          return sql.Open("postgres", connString)
      case "mysql":
          return sql.Open("mysql", connString)
      case "sqlite", "sqlite3":
          return sql.Open("sqlite3", connString)
      case "sql-server", "mssql":
          return sql.Open("sqlserver", connString)
      default:
          return nil, fmt.Errorf("unsupported SQL engine: %q", engine)
      }
  }

  func DispatchSQLDriverForTesting(engine, connString string) (*sql.DB, error) {
      return dispatchSQLDriver(engine, connString)
  }
  ```

  > **mysql driver check**: `cd backend && go list -m github.com/go-sql-driver/mysql`. If not present, `go get github.com/go-sql-driver/mysql` and add the blank-import.

- [ ] Run tests; verify pass.

### Acceptance

- All 7 tests pass
- Unknown engines fail-fast with clear error
- `sqlite ":memory:"` opens + closes cleanly in tests
- Malformed JSON does not panic
- Connection-string parse failures bubble as wrapped errors

### Commit

`feat(agent/tools): sql-agent connection loader + driver dispatch`

---

## Task 2: sql-agent skill (4 actions)

**Files:**
- `backend/internal/agent/tools/sql_agent.go` (NEW)
- `backend/internal/agent/tools/sql_agent_test.go` (NEW)
- `backend/internal/agent/tools/builder.go` (MODIFY — append `NewSQLAgentSkill` to Source 1)

**Tests:**
- `TestSQLAgent_ListDatabases_ReturnsConfiguredConnections`
- `TestSQLAgent_ListDatabases_NoConnections_ReturnsEmptyArray`
- `TestSQLAgent_ListTables_Sqlite_ReturnsTableNames`
- `TestSQLAgent_GetSchema_Sqlite_ReturnsColumnInfo`
- `TestSQLAgent_Query_Sqlite_SelectStatement_ReturnsRows`
- `TestSQLAgent_Query_BoundedTo100Rows`
- `TestSQLAgent_Query_RequiresApprovalForDestructive` (mock Approval; verify called)
- `TestSQLAgent_Query_ApprovalRejected_ReturnsToolError`
- `TestSQLAgent_UnknownDatabaseID_ReturnsToolError`
- `TestSQLAgent_CheckFn_FalseWhenNoConnections`
- `TestSQLAgent_DispatchViaRegistry` (e2e via tool.Registry.Dispatch)

### Steps

- [ ] Write failing tests using a sqlite `:memory:` connection seeded with a `users` table.

- [ ] Implement `sql_agent.go`:
  ```go
  package tools

  import (
      "context"
      "database/sql"
      "encoding/json"
      "fmt"
      "strings"

      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  const sqlMaxRows = 100

  func NewSQLAgentSkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "sql-agent",
          Toolset:     "sql",
          Description: "Inspect and query admin-configured SQL databases. Actions: list_databases, list_tables, get_schema, query.",
          Emoji:       "🗄",
          MaxResultChars: 16 * 1024,
          CheckFn: func() bool {
              if tc.Settings == nil { return false }
              if v, ok := tc.Settings["agent_sql_connections"]; ok && strings.TrimSpace(v) != "" && v != "[]" {
                  return true
              }
              return false
          },
          Schema: core.ToolDefinition{
              Name:        "sql-agent",
              Description: "Inspect or query SQL databases",
              Parameters:  sqlAgentSchema(),
          },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct {
                  Action     string `json:"action"`
                  DatabaseID string `json:"database_id,omitempty"`
                  SQLQuery   string `json:"sql_query,omitempty"`
                  Table      string `json:"table,omitempty"`
              }
              if err := json.Unmarshal(raw, &args); err != nil { return tool.Error(err.Error()), nil }

              conns, err := loadSQLConnections(ctx, tc.SysSvcAsConcrete())
              // ^ small helper because ToolContext SysSvc is via interface
              if err != nil { return tool.Error(err.Error()), nil }

              switch args.Action {
              case "list_databases":
                  tc.Emit("Listing configured SQL databases")
                  out := make([]map[string]any, 0, len(conns))
                  for _, c := range conns { out = append(out, map[string]any{"database_id": c.DatabaseID, "engine": c.Engine}) }
                  return tool.Result(map[string]any{"databases": out}), nil

              case "list_tables":
                  conn, ok := findConnection(conns, args.DatabaseID)
                  if !ok { return tool.Error("unknown database_id: " + args.DatabaseID), nil }
                  tc.Emit("Listing tables in " + args.DatabaseID)
                  db, err := dispatchSQLDriver(conn.Engine, conn.ConnectionString)
                  if err != nil { return tool.Error("connect: " + err.Error()), nil }
                  defer db.Close()
                  tables, err := listTables(ctx, db, conn.Engine)
                  if err != nil { return tool.Error("list tables: " + err.Error()), nil }
                  return tool.Result(map[string]any{"tables": tables}), nil

              case "get_schema":
                  conn, ok := findConnection(conns, args.DatabaseID)
                  if !ok { return tool.Error("unknown database_id: " + args.DatabaseID), nil }
                  if args.Table == "" { return tool.Error("table is required"), nil }
                  tc.Emit(fmt.Sprintf("Inspecting %s.%s", args.DatabaseID, args.Table))
                  db, err := dispatchSQLDriver(conn.Engine, conn.ConnectionString)
                  if err != nil { return tool.Error("connect: " + err.Error()), nil }
                  defer db.Close()
                  cols, err := tableSchema(ctx, db, conn.Engine, args.Table)
                  if err != nil { return tool.Error("schema: " + err.Error()), nil }
                  return tool.Result(map[string]any{"table": args.Table, "columns": cols}), nil

              case "query":
                  conn, ok := findConnection(conns, args.DatabaseID)
                  if !ok { return tool.Error("unknown database_id: " + args.DatabaseID), nil }
                  if args.SQLQuery == "" { return tool.Error("sql_query is required"), nil }

                  // Approval gate (destructive action)
                  if tc.Approval != nil {
                      desc := fmt.Sprintf("Run SQL on %s: %s", args.DatabaseID, truncate(args.SQLQuery, 200))
                      approved, reason := tc.Approval(ctx, "sql-agent:query", args, desc)
                      if !approved { return tool.Error("rejected: " + reason), nil }
                  }

                  tc.Emit("Running SQL query on " + args.DatabaseID)
                  db, err := dispatchSQLDriver(conn.Engine, conn.ConnectionString)
                  if err != nil { return tool.Error("connect: " + err.Error()), nil }
                  defer db.Close()
                  rows, err := runQuery(ctx, db, args.SQLQuery, sqlMaxRows)
                  if err != nil { return tool.Error("query: " + err.Error()), nil }
                  return tool.Result(map[string]any{
                      "rows": rows,
                      "row_count": len(rows),
                      "limit_hit": len(rows) == sqlMaxRows,
                  }), nil

              default:
                  return tool.Error("unknown action: " + args.Action), nil
              }
          },
      }
  }

  // listTables uses information_schema or sqlite_master depending on engine
  func listTables(ctx context.Context, db *sql.DB, engine string) ([]string, error) {
      var q string
      switch engine {
      case "postgresql", "postgres":
          q = "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name"
      case "mysql":
          q = "SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE() ORDER BY table_name"
      case "sqlite", "sqlite3":
          q = "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
      case "sql-server", "mssql":
          q = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME"
      default:
          return nil, fmt.Errorf("unsupported engine: %s", engine)
      }
      rows, err := db.QueryContext(ctx, q)
      if err != nil { return nil, err }
      defer rows.Close()
      var out []string
      for rows.Next() {
          var name string
          if err := rows.Scan(&name); err != nil { return nil, err }
          out = append(out, name)
      }
      return out, rows.Err()
  }

  // tableSchema returns column metadata for the named table.
  func tableSchema(ctx context.Context, db *sql.DB, engine, table string) ([]map[string]any, error) {
      var q string
      switch engine {
      case "postgresql", "postgres":
          q = "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name=$1 ORDER BY ordinal_position"
      case "mysql":
          q = "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name=? AND table_schema=DATABASE() ORDER BY ordinal_position"
      case "sqlite", "sqlite3":
          // Use PRAGMA table_info('<table>')
          return sqliteTableSchema(ctx, db, table)
      case "sql-server", "mssql":
          q = "SELECT column_name, data_type, is_nullable FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME=@p1"
      default:
          return nil, fmt.Errorf("unsupported engine: %s", engine)
      }
      rows, err := db.QueryContext(ctx, q, table)
      if err != nil { return nil, err }
      defer rows.Close()
      var out []map[string]any
      for rows.Next() {
          var name, dtype, nullable string
          if err := rows.Scan(&name, &dtype, &nullable); err != nil { return nil, err }
          out = append(out, map[string]any{"name": name, "type": dtype, "nullable": nullable == "YES"})
      }
      return out, rows.Err()
  }

  // sqliteTableSchema uses PRAGMA table_info(<table>) — table name interpolated (safe: table name from prior list_tables call).
  func sqliteTableSchema(ctx context.Context, db *sql.DB, table string) ([]map[string]any, error) {
      // Validate name (alphanumeric + underscore only) to avoid injection.
      for _, r := range table {
          if !( (r>='a'&&r<='z') || (r>='A'&&r<='Z') || (r>='0'&&r<='9') || r=='_' ) {
              return nil, fmt.Errorf("invalid table name: %q", table)
          }
      }
      rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
      if err != nil { return nil, err }
      defer rows.Close()
      var out []map[string]any
      for rows.Next() {
          var cid int
          var name, ctype string
          var notnull, pk int
          var dflt sql.NullString
          if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil { return nil, err }
          out = append(out, map[string]any{"name": name, "type": ctype, "nullable": notnull == 0, "primary_key": pk == 1})
      }
      return out, rows.Err()
  }

  // runQuery executes a query and returns up to maxRows results.
  func runQuery(ctx context.Context, db *sql.DB, query string, maxRows int) ([]map[string]any, error) {
      rows, err := db.QueryContext(ctx, query)
      if err != nil { return nil, err }
      defer rows.Close()
      cols, err := rows.Columns()
      if err != nil { return nil, err }
      var out []map[string]any
      for rows.Next() && len(out) < maxRows {
          values := make([]any, len(cols))
          ptrs := make([]any, len(cols))
          for i := range values { ptrs[i] = &values[i] }
          if err := rows.Scan(ptrs...); err != nil { return nil, err }
          row := make(map[string]any, len(cols))
          for i, c := range cols { row[c] = values[i] }
          out = append(out, row)
      }
      return out, rows.Err()
  }
  ```

  > **`tc.SysSvcAsConcrete()` is not a thing**: ToolContext's SysSvc field type — verify what it is in `context.go`. It might be a concrete `*services.SystemService` or wrapped in an interface. If interface, add a small helper that type-asserts; if concrete, use directly. (Looking at the earlier read: ToolContext has `Cfg *config.Config` after Task 0 but the SQL connections live in `Settings map[string]string` already populated at session start. **Use `tc.Settings["agent_sql_connections"]` directly instead of re-reading via sysSvc** — simpler and no service interface dance.)

  Refactor `loadSQLConnections` to take the raw string instead:
  ```go
  func parseSQLConnections(raw string) ([]SQLConnection, error) {
      // (same body, take raw string)
  }
  ```
  And in Handler: `parseSQLConnections(tc.Settings["agent_sql_connections"])`.

- [ ] Add to Builder Source 1:
  ```go
  // builder.go
  for _, e := range []*tool.Entry{
      NewRAGMemorySkill(tc),
      NewDocSummarizerSkill(tc),
      NewWebScrapingSkill(tc),
      NewRechartSkill(tc),
      NewSQLAgentSkill(tc),  // NEW
  } { ... }
  ```

- [ ] Write all 11 tests, including the registry-dispatch e2e. Use sqlite `:memory:` for live SQL; pre-create tables in test fixtures.

- [ ] Run; full suite green.

### Acceptance

- All 11 tests pass
- `list_databases` returns `[]` (empty array, not null) when no connections configured
- `query` honors `sqlMaxRows=100` cap
- Approval gate invoked on `query` only, NOT on read actions
- CheckFn returns false when `agent_sql_connections` is empty/null/missing — verified by adding a test that asserts `Definitions(filter)` excludes sql-agent in that case
- SQL injection guard on PRAGMA path
- Dispatching via `tool.Registry` works end-to-end

### Commit

`feat(agent/tools): sql-agent skill — list/schema/query with approval gate`

---

## Task 3: filesystem-agent — safeJoin + read-only actions

**Files:**
- `backend/internal/agent/tools/filesystem_safejoin.go` (NEW)
- `backend/internal/agent/tools/filesystem_safejoin_test.go` (NEW)
- `backend/internal/agent/tools/filesystem_agent.go` (NEW — read-only half: list_dir/read_file/get_info/search_files)
- `backend/internal/agent/tools/filesystem_agent_test.go` (NEW)

**Tests:**
- `TestSafeJoin_NormalPath_ReturnsAbsolute`
- `TestSafeJoin_ParentTraversal_Rejected` (`../etc/passwd`)
- `TestSafeJoin_AbsolutePathInUserInput_Rejected` (`/etc/passwd`)
- `TestSafeJoin_SymlinkEscape_Rejected` (create symlink outside root + try to read through it)
- `TestSafeJoin_NestedPath_Allowed` (`subdir/file.txt`)
- `TestSafeJoin_EmptyPath_AllowedAsRoot`
- `TestFilesystem_ListDir_ReturnsFilesAndDirs`
- `TestFilesystem_ListDir_PathNotFound_ReturnsToolError`
- `TestFilesystem_ReadFile_HappyPath`
- `TestFilesystem_ReadFile_DirectoryReturnsToolError`
- `TestFilesystem_ReadFile_SizeCap_Truncates` (file > 1MiB → truncated)
- `TestFilesystem_GetInfo_ReturnsStat`
- `TestFilesystem_SearchFiles_MatchesGlob`

### Steps

- [ ] Implement `filesystem_safejoin.go`:
  ```go
  package tools

  import (
      "errors"
      "fmt"
      "path/filepath"
      "strings"
  )

  var ErrPathEscape = errors.New("path escapes filesystem root")

  // safeJoin resolves userPath against root, defending against:
  //   - parent-directory traversal (../)
  //   - absolute paths in user input
  //   - symlinks escaping the root after resolution
  // Returns the absolute, symlink-resolved path that is guaranteed to be within root.
  func safeJoin(root, userPath string) (string, error) {
      if filepath.IsAbs(userPath) {
          return "", fmt.Errorf("%w: absolute paths not allowed", ErrPathEscape)
      }
      absRoot, err := filepath.Abs(root)
      if err != nil { return "", err }
      // Resolve any symlinks in root once
      resolvedRoot, err := filepath.EvalSymlinks(absRoot)
      if err != nil {
          // Root doesn't exist yet — use absRoot as-is (filesystem ops will fail later with a clearer error)
          resolvedRoot = absRoot
      }

      joined := filepath.Join(resolvedRoot, userPath)
      cleaned := filepath.Clean(joined)
      if !strings.HasPrefix(cleaned, resolvedRoot+string(filepath.Separator)) && cleaned != resolvedRoot {
          return "", fmt.Errorf("%w: %s outside %s", ErrPathEscape, cleaned, resolvedRoot)
      }
      // Resolve symlinks on the final path if it exists
      if final, err := filepath.EvalSymlinks(cleaned); err == nil {
          if !strings.HasPrefix(final, resolvedRoot+string(filepath.Separator)) && final != resolvedRoot {
              return "", fmt.Errorf("%w: symlink target %s outside %s", ErrPathEscape, final, resolvedRoot)
          }
          return final, nil
      }
      return cleaned, nil
  }
  ```

- [ ] Write failing safeJoin tests with `t.TempDir()` for root + `os.Symlink` for escape test.

- [ ] Implement read-only filesystem skill in `filesystem_agent.go` (write actions in Task 4):
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"
      "io"
      "io/fs"
      "os"
      "path/filepath"
      "strings"

      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  const fsMaxReadBytes = 1 << 20 // 1 MiB per read

  func NewFilesystemAgentSkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "filesystem-agent",
          Toolset:     "filesystem",
          Description: "Read, write, search, and organize files within the agent's sandboxed workspace.",
          Emoji:       "📂",
          MaxResultChars: 16 * 1024,
          CheckFn: func() bool {
              return tc.Cfg != nil && tc.Cfg.AgentFilesystemEnabled
          },
          Schema: core.ToolDefinition{Name: "filesystem-agent", Description: "Sandboxed filesystem operations", Parameters: filesystemAgentSchema()},
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct {
                  Action      string `json:"action"`
                  Path        string `json:"path,omitempty"`
                  Source      string `json:"source,omitempty"`
                  Destination string `json:"destination,omitempty"`
                  Content     string `json:"content,omitempty"`
                  Pattern     string `json:"pattern,omitempty"`
                  // edit_file specifics
                  OldString string `json:"old_string,omitempty"`
                  NewString string `json:"new_string,omitempty"`
              }
              if err := json.Unmarshal(raw, &args); err != nil { return tool.Error(err.Error()), nil }
              if tc.Cfg == nil { return tool.Error("filesystem not configured"), nil }
              root := tc.Cfg.AgentFilesystemRoot

              // Approval-required actions
              destructive := map[string]bool{
                  "write_file": true, "edit_file": true, "move_file": true, "copy_file": true, "create_dir": true,
              }
              if destructive[args.Action] && tc.Approval != nil {
                  desc := fmt.Sprintf("Filesystem %s: %s", args.Action, args.Path+args.Source)
                  if approved, reason := tc.Approval(ctx, "filesystem-agent:"+args.Action, args, desc); !approved {
                      return tool.Error("rejected: " + reason), nil
                  }
              }

              switch args.Action {
              case "list_dir":
                  return fsListDir(tc, root, args.Path)
              case "read_file":
                  return fsReadFile(tc, root, args.Path)
              case "get_info":
                  return fsGetInfo(tc, root, args.Path)
              case "search_files":
                  return fsSearchFiles(tc, root, args.Path, args.Pattern)
              // write actions are added in Task 4 (return stub here so all action names map cleanly)
              case "write_file", "edit_file", "move_file", "copy_file", "create_dir":
                  return tool.Error("write actions land in Task 4 of PR-AR-6"), nil
              default:
                  return tool.Error("unknown action: " + args.Action), nil
              }
          },
      }
  }

  func fsListDir(tc *ToolContext, root, path string) (string, error) {
      abs, err := safeJoin(root, path)
      if err != nil { return tool.Error(err.Error()), nil }
      tc.Emit("Listing " + path)
      entries, err := os.ReadDir(abs)
      if err != nil { return tool.Error(err.Error()), nil }
      out := make([]map[string]any, 0, len(entries))
      for _, e := range entries {
          info, _ := e.Info()
          item := map[string]any{"name": e.Name(), "is_dir": e.IsDir()}
          if info != nil { item["size"] = info.Size() }
          out = append(out, item)
      }
      return tool.Result(map[string]any{"path": path, "entries": out}), nil
  }

  func fsReadFile(tc *ToolContext, root, path string) (string, error) {
      abs, err := safeJoin(root, path)
      if err != nil { return tool.Error(err.Error()), nil }
      tc.Emit("Reading " + path)
      info, err := os.Stat(abs)
      if err != nil { return tool.Error(err.Error()), nil }
      if info.IsDir() { return tool.Error("path is a directory: " + path), nil }
      f, err := os.Open(abs)
      if err != nil { return tool.Error(err.Error()), nil }
      defer f.Close()
      body, err := io.ReadAll(io.LimitReader(f, fsMaxReadBytes))
      if err != nil { return tool.Error(err.Error()), nil }
      truncated := info.Size() > fsMaxReadBytes
      return tool.Result(map[string]any{
          "path": path, "content": string(body),
          "size": info.Size(), "truncated": truncated,
      }), nil
  }

  func fsGetInfo(tc *ToolContext, root, path string) (string, error) {
      abs, err := safeJoin(root, path)
      if err != nil { return tool.Error(err.Error()), nil }
      info, err := os.Stat(abs)
      if err != nil { return tool.Error(err.Error()), nil }
      return tool.Result(map[string]any{
          "path": path, "size": info.Size(), "is_dir": info.IsDir(), "modified": info.ModTime(),
      }), nil
  }

  func fsSearchFiles(tc *ToolContext, root, path, pattern string) (string, error) {
      if pattern == "" { return tool.Error("pattern is required"), nil }
      abs, err := safeJoin(root, path)
      if err != nil { return tool.Error(err.Error()), nil }
      tc.Emit(fmt.Sprintf("Searching for %q under %s", pattern, path))
      var matches []string
      filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
          if err != nil { return nil }
          if !d.IsDir() {
              ok, _ := filepath.Match(pattern, d.Name())
              if ok {
                  rel, _ := filepath.Rel(abs, p)
                  matches = append(matches, filepath.Join(path, rel))
              }
          }
          if len(matches) >= 200 { return filepath.SkipAll }
          return nil
      })
      return tool.Result(map[string]any{"pattern": pattern, "matches": matches, "limit_hit": len(matches) == 200}), nil
  }
  ```

- [ ] Add to Builder Source 1 (after sql-agent):
  ```go
  NewFilesystemAgentSkill(tc),
  ```

- [ ] Run all 13 tests; full suite green.

### Acceptance

- All 13 tests pass
- safeJoin defends against `..`, absolute paths, and symlinks (verified by 4 test cases)
- File read >1MiB returns `truncated: true`
- CheckFn returns false when `cfg.AgentFilesystemEnabled == false`
- Write actions return a clean placeholder error (Task 4 will replace)
- Dispatch via `tool.Registry` works for all 4 read actions

### Commit

`feat(agent/tools): filesystem-agent — safeJoin + read-only actions`

---

## Task 4: filesystem-agent write actions

**Files:**
- `backend/internal/agent/tools/filesystem_agent.go` (MODIFY — implement 5 write actions)
- `backend/internal/agent/tools/filesystem_agent_test.go` (MODIFY — add write tests)

**Tests:**
- `TestFilesystem_WriteFile_HappyPath`
- `TestFilesystem_WriteFile_CreatesMissingParents`
- `TestFilesystem_EditFile_ReplacesOldString`
- `TestFilesystem_EditFile_OldStringNotFound_ReturnsToolError`
- `TestFilesystem_EditFile_AmbiguousMatch_ReturnsToolError` (multiple occurrences)
- `TestFilesystem_MoveFile_HappyPath`
- `TestFilesystem_CopyFile_HappyPath`
- `TestFilesystem_CreateDir_HappyPath`
- `TestFilesystem_CreateDir_AlreadyExists_Idempotent`
- `TestFilesystem_Write_TriggersApprovalGate`
- `TestFilesystem_Write_ApprovalRejected_NoFileWritten`

### Steps

- [ ] Replace the write-action placeholder branches with real impls:
  ```go
  case "write_file":
      return fsWriteFile(tc, root, args.Path, args.Content)
  case "edit_file":
      return fsEditFile(tc, root, args.Path, args.OldString, args.NewString)
  case "move_file":
      return fsMoveFile(tc, root, args.Source, args.Destination)
  case "copy_file":
      return fsCopyFile(tc, root, args.Source, args.Destination)
  case "create_dir":
      return fsCreateDir(tc, root, args.Path)
  ```

- [ ] Implement the 5 helpers:
  ```go
  func fsWriteFile(tc *ToolContext, root, path, content string) (string, error) {
      abs, err := safeJoin(root, path)
      if err != nil { return tool.Error(err.Error()), nil }
      if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil { return tool.Error(err.Error()), nil }
      if err := os.WriteFile(abs, []byte(content), 0o644); err != nil { return tool.Error(err.Error()), nil }
      tc.Emit("Wrote " + path)
      return tool.Result(map[string]any{"path": path, "bytes": len(content)}), nil
  }

  func fsEditFile(tc *ToolContext, root, path, oldStr, newStr string) (string, error) {
      if oldStr == "" { return tool.Error("old_string is required"), nil }
      abs, err := safeJoin(root, path)
      if err != nil { return tool.Error(err.Error()), nil }
      body, err := os.ReadFile(abs)
      if err != nil { return tool.Error(err.Error()), nil }
      count := strings.Count(string(body), oldStr)
      switch count {
      case 0: return tool.Error("old_string not found in file"), nil
      case 1:
          newBody := strings.Replace(string(body), oldStr, newStr, 1)
          if err := os.WriteFile(abs, []byte(newBody), 0o644); err != nil { return tool.Error(err.Error()), nil }
          tc.Emit("Edited " + path)
          return tool.Result(map[string]any{"path": path, "replacements": 1}), nil
      default:
          return tool.Error(fmt.Sprintf("old_string is ambiguous (%d occurrences); provide more context", count)), nil
      }
  }

  func fsMoveFile(tc *ToolContext, root, src, dst string) (string, error) {
      absSrc, err := safeJoin(root, src); if err != nil { return tool.Error(err.Error()), nil }
      absDst, err := safeJoin(root, dst); if err != nil { return tool.Error(err.Error()), nil }
      if err := os.MkdirAll(filepath.Dir(absDst), 0o755); err != nil { return tool.Error(err.Error()), nil }
      if err := os.Rename(absSrc, absDst); err != nil { return tool.Error(err.Error()), nil }
      tc.Emit("Moved " + src + " → " + dst)
      return tool.Result(map[string]any{"from": src, "to": dst}), nil
  }

  func fsCopyFile(tc *ToolContext, root, src, dst string) (string, error) {
      absSrc, err := safeJoin(root, src); if err != nil { return tool.Error(err.Error()), nil }
      absDst, err := safeJoin(root, dst); if err != nil { return tool.Error(err.Error()), nil }
      if err := os.MkdirAll(filepath.Dir(absDst), 0o755); err != nil { return tool.Error(err.Error()), nil }
      data, err := os.ReadFile(absSrc); if err != nil { return tool.Error(err.Error()), nil }
      if err := os.WriteFile(absDst, data, 0o644); err != nil { return tool.Error(err.Error()), nil }
      tc.Emit("Copied " + src + " → " + dst)
      return tool.Result(map[string]any{"from": src, "to": dst, "bytes": len(data)}), nil
  }

  func fsCreateDir(tc *ToolContext, root, path string) (string, error) {
      abs, err := safeJoin(root, path); if err != nil { return tool.Error(err.Error()), nil }
      if err := os.MkdirAll(abs, 0o755); err != nil { return tool.Error(err.Error()), nil }
      tc.Emit("Created dir " + path)
      return tool.Result(map[string]any{"path": path}), nil
  }
  ```

- [ ] Write all 11 tests, including approval gate verification:
  ```go
  func TestFilesystem_Write_TriggersApprovalGate(t *testing.T) {
      var calledWith string
      tc := newToolContext(t)
      tc.Cfg = &config.Config{AgentFilesystemEnabled: true, AgentFilesystemRoot: t.TempDir()}
      tc.Approval = func(_ context.Context, name string, _ any, _ string) (bool, string) {
          calledWith = name; return true, ""
      }
      e := tools.NewFilesystemAgentSkill(tc)
      _, _ = e.Handler(context.Background(), json.RawMessage(`{"action":"write_file","path":"hello.txt","content":"hi"}`))
      require.Equal(t, "filesystem-agent:write_file", calledWith)
  }
  ```

- [ ] Run; full suite green.

### Acceptance

- All 11 tests pass
- `edit_file` with 0 or >1 occurrences returns clean tool.Error, no file mutation
- Approval gate fires for every write action; rejected → no file mutation
- Move/copy create missing destination directories
- `create_dir` is idempotent (MkdirAll semantics)

### Commit

`feat(agent/tools): filesystem-agent — write/edit/move/copy/create_dir`

---

## Task 5: create-files-agent (txt/md only)

**Files:**
- `backend/internal/agent/tools/create_files_agent.go` (NEW)
- `backend/internal/agent/tools/create_files_agent_test.go` (NEW)
- `backend/internal/agent/tools/builder.go` (MODIFY — append `NewCreateFilesAgentSkill`)

**Tests:**
- `TestCreateFiles_Txt_HappyPath`
- `TestCreateFiles_Md_HappyPath`
- `TestCreateFiles_BinaryFormats_ReturnDeferredError`
- `TestCreateFiles_TriggersApprovalGate`
- `TestCreateFiles_ApprovalRejected_NoFileWritten`
- `TestCreateFiles_SafeFilename` (filename sanitisation — strip `..`, `/`, etc)
- `TestCreateFiles_GeneratesUniqueFilename` (UUID prefix)
- `TestCreateFiles_CheckFn_FalseWhenDisabled`

### Steps

- [ ] Implement `create_files_agent.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"
      "os"
      "path/filepath"
      "strings"
      "time"

      "github.com/google/uuid"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  func NewCreateFilesAgentSkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "create-files-agent",
          Toolset:     "create-files",
          Description: "Generate a file (txt or md) and save it to the workspace's generated-files folder. Returns the saved path.",
          Emoji:       "📝",
          MaxResultChars: 2 * 1024,
          CheckFn: func() bool {
              return tc.Cfg != nil && tc.Cfg.AgentCreateFilesEnabled
          },
          Schema: core.ToolDefinition{Name: "create-files-agent", Description: "Create txt/md files", Parameters: createFilesSchema()},
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct {
                  Format   string `json:"format"`            // txt|md|docx|pdf|pptx|xlsx
                  Filename string `json:"filename"`
                  Content  string `json:"content"`
              }
              if err := json.Unmarshal(raw, &args); err != nil { return tool.Error(err.Error()), nil }
              if tc.Cfg == nil { return tool.Error("create-files not configured"), nil }

              switch args.Format {
              case "txt", "md":
                  // ok
              case "docx", "pdf", "pptx", "xlsx":
                  return tool.Error(fmt.Sprintf("format %q not yet implemented (deferred to PR-AR-6.1)", args.Format)), nil
              default:
                  return tool.Error("unknown format: " + args.Format), nil
              }

              // Approval gate
              if tc.Approval != nil {
                  desc := fmt.Sprintf("Create %s file: %s", args.Format, args.Filename)
                  if approved, reason := tc.Approval(ctx, "create-files-agent:"+args.Format, args, desc); !approved {
                      return tool.Error("rejected: " + reason), nil
                  }
              }

              base := sanitiseFilename(args.Filename)
              if base == "" { base = "untitled" }
              uniqueName := fmt.Sprintf("%s-%s.%s", time.Now().Format("20060102-150405"), uuid.NewString()[:8], args.Format)
              if base != "" && base != "untitled" {
                  uniqueName = base + "-" + uniqueName
              }
              dst := filepath.Join(tc.Cfg.AgentCreateFilesDir, uniqueName)
              if err := os.WriteFile(dst, []byte(args.Content), 0o644); err != nil {
                  return tool.Error(err.Error()), nil
              }
              tc.Emit("Created " + uniqueName)
              return tool.Result(map[string]any{
                  "saved_path": dst,
                  "filename":   uniqueName,
                  "bytes":      len(args.Content),
              }), nil
          },
      }
  }

  // sanitiseFilename strips path separators and parent-traversal sequences.
  func sanitiseFilename(name string) string {
      // Allow letters, digits, underscore, hyphen, period — replace others with empty.
      var b strings.Builder
      for _, r := range name {
          switch {
          case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_':
              b.WriteRune(r)
          }
      }
      out := b.String()
      if len(out) > 64 { out = out[:64] }
      return out
  }
  ```

- [ ] Add to Builder:
  ```go
  NewCreateFilesAgentSkill(tc),
  ```

- [ ] Write 8 tests; verify each:
  - `txt` + `md` write into `AgentCreateFilesDir` with UUID-suffixed name
  - Binary formats return `tool.Error("format \"docx\" not yet implemented...")`
  - Filename sanitiser strips `../`, `/`, `\`, special chars
  - Approval gate fires; rejection → no file on disk
  - CheckFn false when `AgentCreateFilesEnabled=false`

- [ ] Run; full suite green.

### Acceptance

- All 8 tests pass
- File names are deterministically unique (timestamp + UUID8)
- Sanitiser is bidi/unicode-aware (test with `t.Run("../etc/passwd")`)
- Approval fires before file is written

### Commit

`feat(agent/tools): create-files-agent — txt/md (binary formats deferred)`

---

## Task 6: Builder e2e + documentation refresh

**Files:**
- `backend/internal/agent/tools/builder_test.go` (MODIFY — add 3 new default-skill registration tests)
- `backend/internal/agent/doc.go` (MODIFY — note PR-AR-6 lands sql/fs/createFiles)
- `.gpowers/designs/2026-05-26-agent-runtime-design.md` (MODIFY — mark PR-AR-6 row as ✅)

**Tests:**
- `TestBuilder_SQLAgent_RegisteredWhenConnectionsConfigured`
- `TestBuilder_FilesystemAgent_RegisteredWhenEnabled`
- `TestBuilder_CreateFilesAgent_RegisteredWhenEnabled`
- `TestBuilder_SQLAgent_NotRegisteredWhenNoConnections` (verify via `Definitions(nil)` excludes "sql-agent")
- `TestBuilder_AllSevenDefaultSkills_PresentInRegistry`

### Steps

- [ ] Augment `builder_test.go` to assert the 7 default skills present (rag-memory, document-summarizer, web-scraping, rechart, sql-agent[gated], filesystem-agent[gated], create-files-agent[gated]).

- [ ] Update `internal/agent/doc.go`:
  ```go
  // PR-AR-6 lands sql-agent / filesystem-agent / create-files-agent skills.
  // Binary file formats (docx/pdf/pptx/xlsx) deferred to PR-AR-6.1.
  ```

- [ ] Mark design doc §14 PR-AR-6 row as ✅ (one-line edit).

- [ ] Manual smoke procedure documented in `doc.go`:
  ```bash
  # Set up a sqlite test DB
  sqlite3 /tmp/test.db "CREATE TABLE users (id INTEGER, name TEXT); INSERT INTO users VALUES (1, 'alice');"

  # Configure SQL connection
  curl -X POST http://localhost:3001/api/system/setting \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"key":"agent_sql_connections","value":"[{\"database_id\":\"test\",\"engine\":\"sqlite\",\"connectionString\":\"/tmp/test.db\"}]"}'

  # In a chat: "@agent list databases"  → expect ["test"]
  # "@agent show me users from test"  → approval prompt → approve → rows returned
  # "@agent write hello.txt with content 'hi from agent'"  → approval prompt → approve → file created
  ```

- [ ] `go test ./...` — full suite green.

### Acceptance

- All 5 builder tests pass
- 7 default skills enumerable from registry
- Manual smoke succeeds
- Design doc reflects PR-AR-6 ✅
- No `TODO(PR-AR-6)` markers remain in source

### Commit

`feat(agent/tools): finalise PR-AR-6 — register sql/fs/createFiles + docs`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` 100% green
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] 2 decision artefacts in `.gpowers/decisions/` (no-docker-restriction, binary-formats-deferred)
- [ ] All 6 actions in filesystem-agent path-checked via safeJoin (verify via grep)
- [ ] sql-agent's `query` action uses approval gate (verify via grep `tc.Approval`)
- [ ] create-files-agent's filename sanitiser strips `../` (verify via test)
- [ ] `internal/agent/doc.go` updated
- [ ] No new TODOs without `PR-AR-6.1` or later reference
- [ ] PR-AR-6.1 plan stub created or tracked (binary file formats)

## Risk notes

| Risk | Mitigation |
|---|---|
| SQL query bypasses "SELECT only" hint via prompt injection → mutation | Approval gate on every `query` action; default-approve is per-session opt-in. Risk is bounded to what the configured DB user can do (admin should give read-only DB creds) |
| safeJoin race condition (TOCTOU): check path → symlink swap → open file | `filepath.EvalSymlinks` runs after Clean, mitigating most cases. For absolute safety, use `os.OpenFile` with `O_NOFOLLOW` on Unix — but cross-platform complexity. Acceptable for v1; doc the caveat |
| `edit_file` with `\r\n` vs `\n` mismatch returns "not found" on Windows-edited files | Document; LLM can read first, edit by exact content slice |
| sql driver registration is package-init side effect — affects unrelated tests | Already true for `lib/pq` and `sqlite3` elsewhere; one more is fine |
| `AGENT_FILESYSTEM_ROOT` pointing at an existing directory with user data | Default is `<StorageDir>/anythingllm-fs`; admins overriding must opt-in consciously. Document in `.env.example` |
| `create-files-agent` filename collisions | Timestamp + UUID8 suffix; collision prob ~0 |
| 100-row SQL cap silently truncates important queries | `limit_hit` field in response signals truncation; LLM can paginate via LIMIT/OFFSET if needed |
| MSSQL driver adds 4MB to binary | Acceptable; gated by `agent_sql_connections` having an mssql entry |
| `safeJoin` rejects empty path → can't list root | Empty path → resolved root, which is a valid dir. Test covers this |

## Estimate

| Task | Hours |
|---|---|
| 0. Wiring + decision artefacts + go.mod | 1.0 |
| 1. SQL connections + driver dispatch | 1.5 |
| 2. sql-agent skill (4 actions) | 2.5 |
| 3. filesystem read-only + safeJoin | 2.0 |
| 4. filesystem write actions | 1.5 |
| 5. create-files-agent (txt/md) | 1.5 |
| 6. Builder e2e + docs | 1.0 |
| **Total** | **11.0** (design estimate 10-12h, mid-range ✓) |

—— end of plan
