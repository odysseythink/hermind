# Agent Runtime PR-AR-3 — Tool Registry + Default Skills + MCP/AgentFlow Projection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Switch participants from raw `Model: lm` to `Agent: pantheonAgent` so tool-calling becomes possible, build a per-session `*tool.Registry` composed from four sources (default skills → MCP → AgentFlows → imported stub), filter via `disabled_agent_skills`, and land four default skills: **rag-memory**, **document-summarizer**, **web-scraping**, **rechart**. Each tool emits its own `statusResponse` frames via a session-bound emitter so the user sees per-tool progress even though `Conversation.reply` is still synchronous.

**Architecture:**
- New sub-package `internal/agent/tools/` owns the four skills + projection adapters + the `Builder`
- `Builder` is constructed per-session in `HandleWS`; it composes `tool.Entry` from sources in priority order and de-duplicates with an eventLog `agent.tool.override` line
- Each `tool.Entry.Handler` closes over a `ToolContext` struct (workspaceID/userID/lm/vectorSvc/docSvc/emit) so it has everything it needs without global state
- MCP → tool adapter wraps `mcp.ToolPlugin.Call(ctx, args map[string]any) (any, error)` to the pantheon `tool.Handler(ctx, args json.RawMessage) (string, error)` signature; result is JSON-stringified
- AgentFlow → tool adapter projects flow metadata as schema and stubs `Handler` with a `"flow execution not yet implemented"` payload (executor lands in a later PR; projection is what matters)
- `Participant.Agent` replaces `Participant.Model`; `Conversation.reply` automatically calls `Agent.Run` when present (line 295 of conversation.go) so the loop semantics in PR-AR-2 stay intact

**Tech Stack:** Go 1.25.5, `github.com/odysseythink/pantheon/agent`, `pantheon/tool`, `pantheon/core` (all already in `go.mod`), `golang.org/x/net/html` (NEW — for web-scraping). No other new deps.

**Source spec:** `.gpowers/designs/2026-05-26-agent-runtime-design.md` §3.2, §3.3, §7, §8, §14 (PR-AR-3 row).

**Reference Node implementation:**
- `server/utils/agents/aibitat/plugins/memory.js` — rag-memory search/store
- `server/utils/agents/aibitat/plugins/summarize.js` — list + summarize actions
- `server/utils/agents/aibitat/plugins/web-scraping.js` — single-URL fetch
- `server/utils/agents/aibitat/plugins/rechart.js` — return chart spec
- `server/utils/agents/defaults.js` — `DEFAULT_SKILLS` + `disabled_agent_skills` lookup
- `server/utils/MCP/index.js` `convertServerToolsToPlugins(name)` — Node's MCP→aibitat adapter

---

## Pre-task: Read this section once before starting

### What landed in PR-AR-2 (use, don't re-implement)

- `agent.Session` — fields: `conv`, `lm`, `wsConn`, `ctx`, `cancel`, `feedbackCh`, `terminated`, `muteUser`, `systemPrompt`
- `agent.Runtime.languageModelFor(ws, settings)` — caches `core.LanguageModel` per provider:model
- `agent.installEventBridges(s)` — OnMessage/OnError/OnTerminate/OnInterrupt wired to WS frames
- `agent.newSession(...)` — currently builds `Participant{Name:@agent, Model:lm}`; PR-AR-3 modifies this signature to take a `*tool.Registry` and switches to `Participant{Agent: pantheonAgent}`
- `wsConn.Send` — non-blocking; tools must tolerate `ErrSlowReader` (log + continue, don't error)

### Pantheon Agent + tool.Registry contract (verified)

```go
// pantheon/agent
func New(model core.LanguageModel, opts ...Option) *Agent
func WithRegistry(*tool.Registry) Option
func WithMaxSteps(n int) Option   // default 10
func WithCompressor(*compression.Compressor) Option

// pantheon/tool
type Handler func(ctx context.Context, args json.RawMessage) (string, error)
type CheckFunc func() bool

type Entry struct {
    Name           string
    Toolset        string
    Schema         core.ToolDefinition
    Handler        Handler
    CheckFn        CheckFunc
    RequiresEnv    []string
    IsInteractive  bool
    MaxResultChars int
    Description    string
    Emoji          string
}

func NewRegistry() *Registry
func (r *Registry) Register(*Entry)
func (r *Registry) Dispatch(ctx, name, args json.RawMessage) (string, error)
func (r *Registry) IsInteractive(name string) bool
func (r *Registry) Definitions(filter func(*Entry) bool) []core.ToolDefinition
func (r *Registry) Entries(filter func(*Entry) bool) []*Entry

// pantheon/core
type ToolDefinition struct {
    Name, Description string
    Parameters *Schema
}
```

**Key observations:**

1. `tool.Registry.Dispatch` is what `pantheon/agent.Agent.executeTool` calls when `WithRegistry` is set. The error from the Handler is **never** returned to Dispatch's caller — Dispatch always returns `(jsonString, nil)`, encoding the error as `{"error":"..."}` JSON (`tool.Error(...)`). This is critical: tool handlers can panic / return `error` and the agent loop won't bail.
2. `MaxResultChars` is enforced by Dispatch — set it per-tool to avoid drowning the LLM in scraped HTML or vector blobs.
3. `CheckFn` runs at `Definitions`/`Entries` call time, not at Dispatch time — useful for "is this tool currently available" gating (e.g., RAG only when vector DB is configured). pantheon's Agent calls `Definitions(filter)` to construct the LLM's tool list, so a `CheckFn==false` tool simply disappears from the tool catalogue.
4. Tools see only `json.RawMessage`. We unmarshal into our own typed param structs in each Handler.

### MCP `ToolPlugin` → `tool.Entry` adapter

MCP plugin's `Call(ctx, args map[string]any) (any, error)` returns arbitrary Go data; we marshal to JSON. The `InputSchema` is `json.RawMessage` (raw JSON Schema object).

```go
// internal/agent/tools/mcp.go
func mcpToolToEntry(p mcp.ToolPlugin, emit StatusEmitter) *tool.Entry {
    var params *core.Schema
    if len(p.InputSchema) > 0 {
        params = &core.Schema{Raw: p.InputSchema}  // see "core.Schema raw bytes" note below
    }
    return &tool.Entry{
        Name:        p.QualifiedName,         // "<server>-<tool>"
        Toolset:     "mcp",
        Description: p.Description,
        Schema: core.ToolDefinition{
            Name:        p.QualifiedName,
            Description: p.Description,
            Parameters:  params,
        },
        MaxResultChars: 8 * 1024,
        Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
            emit("calling MCP tool: " + p.QualifiedName)
            var args map[string]any
            if len(raw) > 0 {
                if err := json.Unmarshal(raw, &args); err != nil { return "", err }
            }
            result, err := p.Call(ctx, args)
            if err != nil { return "", err }
            b, _ := json.Marshal(result)
            return string(b), nil
        },
    }
}
```

> **`core.Schema` raw bytes** — pantheon's `core.Schema` has a `Raw json.RawMessage` field (verify in `pantheon/core/tool.go` during Task 0); if the actual field name differs, adapt accordingly. If pantheon insists on a structured schema, we marshal-unmarshal through `core.Schema`.

### New surface (this PR)

```
backend/internal/agent/
├── runtime.go                     # MODIFY — Deps gains VectorSvc, DocSvc, MCPHv, FlowSvc, EventLog, SysSvc
├── session.go                     # MODIFY — newSession signature gains *tool.Registry; Participant uses Agent
├── handler.go                     # MODIFY — HandleWS calls Builder before newSession
└── tools/
    ├── doc.go                     # package comment
    ├── context.go                 # ToolContext struct + StatusEmitter typedef
    ├── builder.go                 # Builder + Build(ctx, ws, user, settings) → *tool.Registry
    ├── builder_test.go            # disabled filter, dedup with eventLog, source priority
    ├── disabled.go                # parseDisabledSkills(rawJSON []byte) []string
    ├── disabled_test.go
    ├── rag_memory.go              # search/store
    ├── rag_memory_test.go
    ├── doc_summarizer.go          # list/summarize
    ├── doc_summarizer_test.go
    ├── web_scraping.go            # fetch + html extract
    ├── web_scraping_test.go
    ├── rechart.go                 # chart spec emit
    ├── rechart_test.go
    ├── mcp.go                     # mcpToolToEntry
    ├── mcp_test.go
    ├── flow.go                    # flowToEntry (stub handler)
    └── flow_test.go

backend/go.mod                   # MODIFY — golang.org/x/net (transitive likely already there)
backend/internal/services/system_service.go  # MAYBE MODIFY — add DisabledAgentSkills(ctx) []string convenience
```

### Methods to ship (PR-AR-3 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `agent.Deps` | gains `VectorSearchSvc *services.VectorSearchService, DocSvc *services.DocumentService, MCPHv *mcp.Hypervisor, FlowSvc *services.AgentFlowService, EventLog *services.EventLogService, SysSvc *services.SystemService` | Wiring change |
| 2 | `agent.newSession` | new param `reg *tool.Registry`; builds `pantheonAgent := agent.New(lm, agent.WithRegistry(reg), agent.WithMaxSteps(10))`; Participant uses `Agent: pantheonAgent` |
| 3 | `tools.ToolContext` | struct holding all per-session deps | passed to each skill factory |
| 4 | `tools.StatusEmitter` | `type StatusEmitter func(message string)` | wraps `wsConn.Send(statusResponse)` |
| 5 | `tools.Builder` | `func NewBuilder(deps *agent.Deps) *Builder` | |
| 6 | `tools.Builder.Build(ctx, ws, user, emit StatusEmitter, settings map[string]string) (*tool.Registry, error)` | composes sources, applies disabled filter, de-dups |
| 7 | `tools.parseDisabledSkills(raw []byte) []string` | tolerates absent/empty/malformed JSON | |
| 8 | `tools.NewRAGMemorySkill(tc *ToolContext) *tool.Entry` | search + store actions |
| 9 | `tools.NewDocSummarizerSkill(tc *ToolContext) *tool.Entry` | list + summarize actions |
| 10 | `tools.NewWebScrapingSkill(tc *ToolContext) *tool.Entry` | url param |
| 11 | `tools.NewRechartSkill(tc *ToolContext) *tool.Entry` | chart spec |
| 12 | `tools.mcpToolToEntry(p mcp.ToolPlugin, emit) *tool.Entry` | per-server projection |
| 13 | `tools.flowToEntry(f services.FlowSummary, emit, flowSvc) *tool.Entry` | stub Handler |
| 14 | `services.SystemService.DisabledAgentSkills(ctx) ([]string, error)` | convenience wrapper around `GetSetting("disabled_agent_skills")` |

### Tool naming & toolset conventions

| Skill | Tool Name (matches Node) | Toolset tag |
|---|---|---|
| rag-memory | `rag-memory` | `memory` |
| document-summarizer | `document-summarizer` | `document` |
| web-scraping | `web-scraping` | `web` |
| rechart | `rechart` | `chart` |
| MCP-projected | `<server>-<tool>` | `mcp` |
| AgentFlow-projected | `<flow.Name lowercased + slugified>` | `flow` |

> **Why `Toolset`**: pantheon `tool.Entry.Toolset` is a free-form grouping tag for observability/debug — not used by the agent loop. We populate it so future log filters / metrics can split by source.

### Disabled-skills filter

`SystemSetting` row keyed `disabled_agent_skills` stores a JSON array of tool names: `["rag-memory","web-scraping"]`. Absence/null/malformed → empty list (skill set returns to defaults). Filter is applied after **all** sources merge; users can disable MCP tools and flows by name too.

### Default skills enabled by default

Node's `DEFAULT_SKILLS = [memory, docSummarizer, webScraping]`. Note `rechart` is **NOT** in `DEFAULT_SKILLS` in Node — it's only registered when present in workspace.aibitat config. **For Go v1 simplicity we register all 4 default skills** and rely on `disabled_agent_skills` to suppress unwanted ones. This is a documented behavioural delta from Node — track in `.gpowers/decisions/2026-05-27-agent-default-skills.md` (one-line decision artefact).

### Dedup policy

Build order — later wins:
1. Default skills (rag-memory, document-summarizer, web-scraping, rechart)
2. MCP-projected (every running server's tools)
3. AgentFlow-projected (every active flow)
4. Imported plugins (v1 stub, contributes nothing)

If a later source registers a name already present, the earlier entry is replaced and `eventLog.LogEvent("agent.tool.override", {tool, from, to}, nil)` is fired. Tests must verify the eventLog call is made.

### Out of scope (explicit)

- AgentFlow **execution** — Task 7 only projects flows as tools whose Handler returns `"flow execution not yet implemented"`. A real flow executor (api-call/llm-instruction/web-scraping blocks) is a separate PR
- `Agent.RunStream` — staying with `Conversation.reply` (which calls `Agent.Run`); status updates come from each tool's emit callback, not from agent stream events
- Imported plugins (`server/utils/agents/imported.js` equivalent) — v1 stub
- gmail/outlook/google-calendar — v3 (OAuth dependencies)
- sql-agent / filesystem / create-files — PR-AR-6
- Workspace-level skill overrides (Node has per-workspace agent config) — out; we filter only via global `disabled_agent_skills`
- Per-user permission gates on tools — out (PR-AR-5 will add `AgentSkillWhitelist`)
- Tool result truncation per skill type (Node has variable limits) — use `MaxResultChars: 8*1024` uniformly; refine later

### Test setup helper additions

```go
// internal/agent/tools/test_helpers_test.go
func newToolContext(t *testing.T, env *agentTestEnv) *tools.ToolContext {
    emit := func(msg string) { t.Logf("emit: %s", msg) }
    return &tools.ToolContext{
        Ctx:           context.Background(),
        Workspace:     seedWorkspace(t, env.DB),
        User:          env.User,
        LM:            &mockLanguageModel{provider:"mock", model:"m", replies:[]string{"summary"}},
        VectorSvc:     env.VectorSearchSvc,
        DocSvc:        env.DocSvc,
        MCPHv:         env.MCPHv,
        EventLog:      env.EventLog,
        Emit:          emit,
    }
}
```

### TDD discipline

Each task lands as **one commit**. Failing test → impl → green → full suite green → commit. Tests use mocks for vectorSvc/docSvc when possible (sqlite-mem + in-memory vector store). For web-scraping and rag-memory, use `httptest.NewServer` and an in-memory vector backend respectively.

---

## Task 1: Wire Participant→Agent + Deps expansion + registry plumbing

**Files:**
- `backend/internal/agent/runtime.go` (MODIFY — Deps + Build wiring stubs)
- `backend/internal/agent/session.go` (MODIFY — newSession signature)
- `backend/internal/agent/handler.go` (MODIFY — call Builder)
- `backend/internal/agent/session_test.go` (MODIFY — verify Participant.Agent is set; backwards-compat: empty registry still works)

**Tests:**
- `TestSession_UsesAgentNotRawModel` (assert `Participant.Agent != nil && Participant.Model == nil` post-construction)
- `TestSession_EmptyRegistry_StillReplies` (mock LLM returns text, no tool calls; conversation still terminates)
- `TestSession_AgentMaxStepsIs10` (verify via direct struct introspection or behavioural test using mock that always returns tool calls — should fail with "max steps" after 10 calls)

### Steps

- [ ] Update `agent.Deps`:
  ```go
  type Deps struct {
      DB            *gorm.DB
      Cfg           *config.Config
      TempTokenSvc  *services.TemporaryAuthTokenService
      AuthSvc       *services.AuthService
      // NEW
      SysSvc        *services.SystemService
      VectorSearchSvc *services.VectorSearchService
      DocSvc        *services.DocumentService
      MCPHv         *mcp.Hypervisor
      FlowSvc       *services.AgentFlowService
      EventLog      *services.EventLogService
  }
  ```

- [ ] Update `cmd/server/main.go` to populate the new fields. **Verify each service already exists at the time of `NewRuntime` call site.** If wiring order needs adjusting, do it minimally.

- [ ] Modify `newSession` signature:
  ```go
  func newSession(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
      lm core.LanguageModel, systemPrompt string, reg *tool.Registry, conn *wsConn) *Session
  ```
  Inside, construct pantheon agent and switch to `Agent` field:
  ```go
  pAgent := agent.New(lm,
      agent.WithRegistry(reg),
      agent.WithMaxSteps(10),
  )
  s.conv.RegisterParticipant(&conversation.Participant{
      Name:  participantAgent,
      Role:  systemPrompt,
      Agent: pAgent,   // was: Model: lm
  })
  ```

- [ ] In `HandleWS`, between `lm` resolution and `newSession`, insert:
  ```go
  reg, err := buildSessionRegistry(c.Request.Context(), r.deps, &ws, user, wc)
  if err != nil {
      c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "tools: " + err.Error()})
      return
  }
  ```
  Where `buildSessionRegistry` is a thin wrapper:
  ```go
  func buildSessionRegistry(ctx context.Context, deps Deps, ws *models.Workspace, user *models.User, conn *wsConn) (*tool.Registry, error) {
      b := tools.NewBuilder(&deps)
      emit := func(msg string) { _ = conn.Send(ServerFrame{Type: FrameStatusResponse, Content: msg, Animate: true}) }
      settings, _ := deps.SysSvc.GetAllSettings(ctx)
      return b.Build(ctx, ws, user, emit, settings)
  }
  ```
  **PR-AR-3 Task 1 lands `Builder` as an empty-registry stub** — it returns `tool.NewRegistry()` with zero entries. The real composition lands in Task 2.

- [ ] Add a temporary `internal/agent/tools/builder.go` skeleton:
  ```go
  package tools

  import (
      "context"
      "github.com/odysseythink/pantheon/tool"
      "github.com/odysseythink/hermind/backend/internal/agent" // careful: import cycle? see note
      "github.com/odysseythink/hermind/backend/internal/models"
  )

  type Builder struct { deps *agent.Deps }
  func NewBuilder(deps *agent.Deps) *Builder { return &Builder{deps: deps} }

  type StatusEmitter func(msg string)

  func (b *Builder) Build(ctx context.Context, ws *models.Workspace, user *models.User, emit StatusEmitter, settings map[string]string) (*tool.Registry, error) {
      reg := tool.NewRegistry()
      // TODO Task 2: compose sources
      return reg, nil
  }
  ```

  > **Import cycle alert**: `tools` imports `agent.Deps`; `agent` imports `tools`. Resolve by **defining `agent.Deps` in `runtime.go` (already there) and making `tools.Builder` accept the dep fields explicitly**, not the struct. Refactor:
  ```go
  type BuilderDeps struct {
      VectorSearchSvc *services.VectorSearchService
      DocSvc          *services.DocumentService
      MCPHv           *mcp.Hypervisor
      FlowSvc         *services.AgentFlowService
      EventLog        *services.EventLogService
      SysSvc          *services.SystemService
      LM              core.LanguageModel  // for summarizer's secondary call
  }
  func NewBuilder(deps BuilderDeps) *Builder { ... }
  ```
  Update `buildSessionRegistry` to construct `BuilderDeps` from `deps Deps + lm` instead. **No import cycle**.

- [ ] Write & run the three tests; verify pass.

### Acceptance

- `Participant.Agent != nil` and `Model == nil` after `newSession`
- Empty registry: conversation completes with mock LLM
- MaxSteps capped at 10 (mock returns tool call 11 times → agent errors with "max steps")
- `main.go` boots, all wiring resolves
- Full test suite green

### Commit

`feat(agent): wire pantheon Agent + per-session tool.Registry (empty stub)`

---

## Task 2: Builder composition + disabled-skills filter + dedup eventLog

**Files:**
- `backend/internal/agent/tools/builder.go` (MODIFY — implement composition)
- `backend/internal/agent/tools/disabled.go` (NEW)
- `backend/internal/agent/tools/disabled_test.go` (NEW)
- `backend/internal/agent/tools/builder_test.go` (NEW)
- `backend/internal/services/system_service.go` (MODIFY — add `DisabledAgentSkills`)

**Tests:**
- `TestParseDisabledSkills_EmptyAndMalformed` (table-driven: `""`, `"[]"`, `"null"`, `"[\"foo\"]"`, `"not-json"`)
- `TestBuilder_RespectsDisabledFilter` (register two stubs; disable one; only one remains)
- `TestBuilder_DedupLastWins_FiresOverrideEventLog` (register stub named "rag-memory" in MCP source; assert eventLog called)
- `TestBuilder_DedupPreservesLater` (final Dispatch goes to the later registrant)

### Steps

- [ ] Implement `disabled.go`:
  ```go
  package tools

  import "encoding/json"

  func parseDisabledSkills(raw string) []string {
      if raw == "" || raw == "null" { return nil }
      var arr []string
      if err := json.Unmarshal([]byte(raw), &arr); err != nil { return nil }
      return arr
  }

  func isDisabled(name string, disabled []string) bool {
      for _, d := range disabled { if d == name { return true } }
      return false
  }
  ```

- [ ] Add `system_service.go` method:
  ```go
  func (s *SystemService) DisabledAgentSkills(ctx context.Context) ([]string, error) {
      raw, err := s.GetSetting(ctx, "disabled_agent_skills")
      if err != nil { return nil, err }
      return parseDisabledSkillsFromRaw(raw), nil
  }
  // helper since tools/ shouldn't be reachable from services/ either; duplicate the 4-line parser
  func parseDisabledSkillsFromRaw(raw string) []string { /* same body */ }
  ```

  Or keep `DisabledAgentSkills` returning raw string and let `tools.Builder` parse — simpler. I'll go with the latter.

- [ ] Implement `Builder.Build`:
  ```go
  func (b *Builder) Build(ctx context.Context, ws *models.Workspace, user *models.User, emit StatusEmitter, settings map[string]string) (*tool.Registry, error) {
      reg := tool.NewRegistry()
      disabled := parseDisabledSkills(settings["disabled_agent_skills"])

      tc := &ToolContext{
          Ctx: ctx, Workspace: ws, User: user, LM: b.deps.LM, Settings: settings,
          VectorSearchSvc: b.deps.VectorSearchSvc, DocSvc: b.deps.DocSvc,
          MCPHv: b.deps.MCPHv, FlowSvc: b.deps.FlowSvc, EventLog: b.deps.EventLog,
          Emit: emit,
      }

      // Source 1: defaults
      for _, e := range []*tool.Entry{
          NewRAGMemorySkill(tc),
          NewDocSummarizerSkill(tc),
          NewWebScrapingSkill(tc),
          NewRechartSkill(tc),
      } {
          if isDisabled(e.Name, disabled) { continue }
          b.add(reg, e, "default", emit)
      }

      // Source 2: MCP
      if b.deps.MCPHv != nil {
          for _, qname := range b.deps.MCPHv.ActiveServers() {  // returns "@@mcp_<server>"
              serverName := strings.TrimPrefix(qname, "@@mcp_")
              plugins, err := b.deps.MCPHv.ToolsAsPlugins(serverName)
              if err != nil {
                  mlog.Warning("agent: MCP ToolsAsPlugins(", serverName, "): ", err)
                  continue
              }
              for _, p := range plugins {
                  if isDisabled(p.QualifiedName, disabled) { continue }
                  b.add(reg, mcpToolToEntry(p, emit), "mcp:"+serverName, emit)
              }
          }
      }

      // Source 3: AgentFlow
      if b.deps.FlowSvc != nil {
          flows, err := b.deps.FlowSvc.ListFlows()
          if err == nil {
              for _, f := range flows {
                  if !f.Active { continue }
                  e := flowToEntry(f, b.deps.FlowSvc, emit)
                  if isDisabled(e.Name, disabled) { continue }
                  b.add(reg, e, "flow:"+f.UUID, emit)
              }
          }
      }

      // Source 4: imported (v1 stub, no-op)
      return reg, nil
  }

  func (b *Builder) add(reg *tool.Registry, e *tool.Entry, source string, _ StatusEmitter) {
      // Check if name already registered; if so, fire eventLog override
      if existing := reg.Entries(func(x *tool.Entry) bool { return x.Name == e.Name }); len(existing) > 0 {
          prior := existing[0]
          if b.deps.EventLog != nil {
              _ = b.deps.EventLog.LogEvent(context.Background(), "agent.tool.override",
                  map[string]any{"tool": e.Name, "from": prior.Toolset, "to": source}, nil)
          }
      }
      reg.Register(e)  // overwrites
  }
  ```

- [ ] Write `builder_test.go` with stub entries to exercise dedup and disabled filter; eventLog mock counts calls.

- [ ] Run tests; verify pass.

### Acceptance

- `parseDisabledSkills` returns `nil` for any non-array input
- `Build` registers 4 default skills when no disable + no MCP + no flows
- Disabling `rag-memory` reduces count to 3
- Registering an MCP tool with name `rag-memory` fires `agent.tool.override` event
- Final Dispatch goes to the **later** registration (verify via stub handler returning distinct strings)

### Commit

`feat(agent/tools): Builder composition + disabled-skills filter + override eventLog`

---

## Task 3: rag-memory skill (search + store)

**Files:**
- `backend/internal/agent/tools/context.go` (NEW)
- `backend/internal/agent/tools/rag_memory.go` (NEW)
- `backend/internal/agent/tools/rag_memory_test.go` (NEW)

**Tests:**
- `TestRAGMemory_Search_ReturnsTopK`
- `TestRAGMemory_Search_NoResults_ReturnsEmptyArrayString`
- `TestRAGMemory_Store_EmbedsAndPersists`
- `TestRAGMemory_InvalidAction_ReturnsError`
- `TestRAGMemory_DispatchViaRegistry` (end-to-end via `tool.Registry.Dispatch`)

### Steps

- [ ] Implement `context.go`:
  ```go
  package tools

  import (
      "context"
      "github.com/odysseythink/hermind/backend/internal/mcp"
      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/hermind/backend/internal/services"
      "github.com/odysseythink/pantheon/core"
  )

  type ToolContext struct {
      Ctx              context.Context
      Workspace        *models.Workspace
      User             *models.User
      Settings         map[string]string
      LM               core.LanguageModel
      VectorSearchSvc  *services.VectorSearchService
      DocSvc           *services.DocumentService
      MCPHv            *mcp.Hypervisor
      FlowSvc          *services.AgentFlowService
      EventLog         *services.EventLogService
      Emit             StatusEmitter
  }
  ```

- [ ] Write failing `rag_memory_test.go`:
  ```go
  func TestRAGMemory_Search_ReturnsTopK(t *testing.T) {
      tc := newToolContext(t, env)
      seedVector(t, tc.Workspace.Slug, []seed{
          {Text: "Plato wrote The Republic.", Vec: stubVec(0.9)},
          {Text: "Aristotle was Plato's student.", Vec: stubVec(0.8)},
      })
      entry := tools.NewRAGMemorySkill(tc)
      args := json.RawMessage(`{"action":"search","content":"Who wrote The Republic?"}`)
      result, err := entry.Handler(context.Background(), args)
      require.NoError(t, err)
      require.Contains(t, result, "Plato")
  }
  ```

- [ ] Implement `rag_memory.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"

      "github.com/google/uuid"
      "github.com/odysseythink/hermind/backend/internal/dto"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  func NewRAGMemorySkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "rag-memory",
          Toolset:     "memory",
          Description: "Search local documents or store information to long-term memory. Action 'search' finds relevant passages; 'store' saves content for later retrieval.",
          Emoji:       "🧠",
          MaxResultChars: 8 * 1024,
          CheckFn:     func() bool { return tc.VectorSearchSvc != nil },
          Schema: core.ToolDefinition{
              Name:        "rag-memory",
              Description: "Search or store workspace memory",
              Parameters:  ragMemorySchema(),
          },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct {
                  Action  string `json:"action"`
                  Content string `json:"content"`
              }
              if err := json.Unmarshal(raw, &args); err != nil { return "", err }
              switch args.Action {
              case "search":
                  tc.Emit("Searching memory: " + truncate(args.Content, 60))
                  results, err := tc.VectorSearchSvc.Search(ctx, tc.Workspace, dto.VectorSearchRequest{
                      Query: args.Content,
                      TopN:  4,
                  })
                  if err != nil { return tool.Error("vector search: " + err.Error()), nil }
                  if len(results) == 0 { return `{"results":[]}`, nil }
                  out := make([]map[string]any, 0, len(results))
                  for _, r := range results {
                      out = append(out, map[string]any{"text": r.Text, "score": r.Score, "source": r.SourceName})
                  }
                  b, _ := json.Marshal(map[string]any{"results": out})
                  return string(b), nil

              case "store":
                  tc.Emit("Storing to memory: " + truncate(args.Content, 60))
                  // namespace = "memory-<workspaceID>"
                  // Embed + write via VectorService — see Note below
                  return "", fmt.Errorf("store: not implemented in PR-AR-3.0 (lands as PR-AR-3.1 follow-up)")
              default:
                  return "", fmt.Errorf("unknown action %q", args.Action)
              }
          },
      }
  }

  func ragMemorySchema() *core.Schema {
      return &core.Schema{ /* JSON schema { action: enum[search,store], content: string } — see Task 0 to confirm core.Schema shape */ }
  }
  ```

  > **Note on `store` action**: Node uses a workspace-scoped vector namespace `memory-<workspaceID>`. In Go, our `VectorService.SimilaritySearch` takes `namespace string` directly. Implementing `store` cleanly requires either:
  > - extending `VectorService` with `Upsert(ctx, namespace, vec, text, metadata)` (likely already exists; verify)
  > - or marking `store` as **explicitly not implemented** in PR-AR-3, returning a clear error, and tracking via decision artefact
  >
  > **Decision for this PR**: Ship `search` fully; `store` returns `"store action will be available in a future PR"` (no error, just a polite Result string so the agent loop continues). Capture as `.gpowers/decisions/2026-05-27-ragmemory-store-deferred.md`.

- [ ] Refine Handler: replace `store`'s `return "", error` with:
  ```go
  case "store":
      tc.Emit("Memory store request acknowledged (deferred)")
      return tool.Result(map[string]any{"status": "deferred", "note": "store action is not yet implemented"}), nil
  ```

- [ ] Run tests; pass.

### Acceptance

- `search` returns formatted JSON results with text/score/source
- `search` returns `{"results":[]}` when no vectors
- `store` returns deferred-status JSON (agent loop unblocked)
- `CheckFn` returns false when `VectorSearchSvc == nil` (registry's `Definitions(filter)` excludes the tool)
- `Dispatch` via `tool.Registry` works end-to-end

### Commit

`feat(agent/tools): rag-memory skill — search action (store deferred)`

---

## Task 4: document-summarizer skill (list + summarize)

**Files:**
- `backend/internal/agent/tools/doc_summarizer.go` (NEW)
- `backend/internal/agent/tools/doc_summarizer_test.go` (NEW)

**Tests:**
- `TestDocSummarizer_List_ReturnsDocuments`
- `TestDocSummarizer_Summarize_ReturnsLLMReply`
- `TestDocSummarizer_Summarize_NonexistentFile_ReturnsError`
- `TestDocSummarizer_DispatchViaRegistry`

### Steps

- [ ] Implement `doc_summarizer.go`:
  ```go
  func NewDocSummarizerSkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "document-summarizer",
          Toolset:     "document",
          Description: "List documents in this workspace or summarize a specific document by filename.",
          MaxResultChars: 12 * 1024,
          CheckFn:     func() bool { return tc.DocSvc != nil && tc.LM != nil },
          Schema:      core.ToolDefinition{ /* action enum[list,summarize], document_filename: string */ },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct {
                  Action            string `json:"action"`
                  DocumentFilename  string `json:"document_filename,omitempty"`
              }
              if err := json.Unmarshal(raw, &args); err != nil { return "", err }
              switch args.Action {
              case "list":
                  tc.Emit("Listing workspace documents")
                  docs, err := tc.DocSvc.ListDocuments(ctx, "")  // root folder; refine for nested later
                  if err != nil { return tool.Error("list failed: " + err.Error()), nil }
                  out := make([]map[string]any, 0, len(docs))
                  for _, d := range docs {
                      out = append(out, map[string]any{
                          "filename":    d.Filename,
                          "title":       d.Title,
                          "preview":     truncate(d.PageContent, 200),
                          "docId":       d.DocID,
                      })
                  }
                  b, _ := json.Marshal(map[string]any{"documents": out})
                  return string(b), nil

              case "summarize":
                  if args.DocumentFilename == "" { return tool.Error("document_filename required"), nil }
                  tc.Emit("Summarizing " + args.DocumentFilename)
                  // 1. lookup doc by filename within workspace
                  // 2. read full text from its file (or assemble from chunks)
                  // 3. call LM.Generate with a "Summarize this:\n<text>" prompt
                  // 4. return the assistant text
                  content, err := readDocumentText(ctx, tc.DocSvc, args.DocumentFilename)
                  if err != nil { return tool.Error(err.Error()), nil }
                  resp, err := tc.LM.Generate(ctx, &core.Request{
                      SystemPrompt: "You are a concise summarizer. Output 5-10 bullet points.",
                      Messages: []core.Message{
                          core.NewTextMessage(core.MESSAGE_ROLE_USER, "Summarize:\n"+content),
                      },
                  })
                  if err != nil { return tool.Error("summarize call: " + err.Error()), nil }
                  return resp.Message.Text(), nil

              default:
                  return tool.Error("unknown action: " + args.Action), nil
              }
          },
      }
  }

  // readDocumentText reads the document file from storage and joins it. If the file
  // is too large, fall back to vector-store chunks for the document.
  func readDocumentText(ctx context.Context, docSvc *services.DocumentService, filename string) (string, error) {
      // PR-AR-3 implementation: use docSvc.ListDocuments to find by filename, then return its first 64KB.
      docs, err := docSvc.ListDocuments(ctx, "")
      if err != nil { return "", err }
      for _, d := range docs {
          if d.Filename == filename {
              if len(d.PageContent) > 64*1024 { return d.PageContent[:64*1024], nil }
              return d.PageContent, nil
          }
      }
      return "", fmt.Errorf("document %q not found", filename)
  }
  ```

  > **Note**: `WorkspaceDocument` may not have `PageContent` directly populated; if it stores a path instead, read the file. Verify field shape in `internal/models/workspace_document.go` during Task 0 of execution.

- [ ] Write tests using `tc.LM = &mockLanguageModel{replies:[]string{"- bullet one\n- bullet two"}}` for the summarize path.

- [ ] Run tests; pass.

### Acceptance

- `list` returns JSON with at least filename/title/preview/docId fields
- `summarize` calls LM and returns the assistant text
- `summarize` with bad filename returns `tool.Error("document ... not found")` — agent loop continues
- `CheckFn` false when no DocSvc or no LM
- All tests pass; full suite green

### Commit

`feat(agent/tools): document-summarizer skill — list + summarize`

---

## Task 5: web-scraping + rechart skills

**Files:**
- `backend/internal/agent/tools/web_scraping.go` (NEW)
- `backend/internal/agent/tools/web_scraping_test.go` (NEW)
- `backend/internal/agent/tools/rechart.go` (NEW)
- `backend/internal/agent/tools/rechart_test.go` (NEW)
- `backend/go.mod` (MAYBE — verify `golang.org/x/net/html` is reachable)

**Tests:**
- `TestWebScraping_FetchHTML_ExtractsArticle` (httptest.NewServer serves `<html><article>foo</article></html>`)
- `TestWebScraping_FallbackToBody`
- `TestWebScraping_404_ReturnsError`
- `TestWebScraping_RespectsMaxResultChars`
- `TestWebScraping_RejectsNonHTTPSchemes` (`ftp://`, `file://`)
- `TestRechart_BasicLineChart_ReturnsJSON`
- `TestRechart_InvalidType_ReturnsError`

### Steps

- [ ] Verify `golang.org/x/net/html` is available:
  ```bash
  cd backend && go list -m golang.org/x/net 2>/dev/null || go get golang.org/x/net/html
  ```

- [ ] Implement `web_scraping.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"
      "io"
      "net/http"
      "net/url"
      "strings"
      "time"

      "golang.org/x/net/html"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  const wsMaxBodyBytes = 1 << 20 // 1 MiB cap
  const wsHTTPTimeout = 30 * time.Second

  func NewWebScrapingSkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "web-scraping",
          Toolset:     "web",
          Description: "Fetch a URL and return its main textual content (article > main > body).",
          MaxResultChars: 8 * 1024,
          Schema:      core.ToolDefinition{ /* url: string (URI) */ },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct{ URL string `json:"url"` }
              if err := json.Unmarshal(raw, &args); err != nil { return tool.Error(err.Error()), nil }
              if args.URL == "" { return tool.Error("url is required"), nil }
              u, err := url.Parse(args.URL)
              if err != nil { return tool.Error("invalid url"), nil }
              if u.Scheme != "http" && u.Scheme != "https" {
                  return tool.Error("only http/https URLs allowed"), nil
              }
              tc.Emit("Scraping " + args.URL)

              client := &http.Client{Timeout: wsHTTPTimeout}
              req, _ := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
              req.Header.Set("User-Agent", "AnythingLLM-Agent/1.0")
              resp, err := client.Do(req)
              if err != nil { return tool.Error("fetch: " + err.Error()), nil }
              defer resp.Body.Close()
              if resp.StatusCode >= 400 { return tool.Error(fmt.Sprintf("http %d", resp.StatusCode)), nil }

              body, err := io.ReadAll(io.LimitReader(resp.Body, wsMaxBodyBytes))
              if err != nil { return tool.Error("read body: " + err.Error()), nil }

              text, title := extractMainText(body)
              return tool.Result(map[string]any{
                  "url":     args.URL,
                  "title":   title,
                  "content": text,
              }), nil
          },
      }
  }

  // extractMainText walks the HTML doc preferring <article>, then <main>, then <body>;
  // skips <script>, <style>, <nav>, <aside>.
  func extractMainText(body []byte) (text, title string) {
      doc, err := html.Parse(strings.NewReader(string(body)))
      if err != nil { return string(body), "" }

      var findTitle func(*html.Node) string
      findTitle = func(n *html.Node) string {
          if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
              return strings.TrimSpace(n.FirstChild.Data)
          }
          for c := n.FirstChild; c != nil; c = c.NextSibling {
              if t := findTitle(c); t != "" { return t }
          }
          return ""
      }
      title = findTitle(doc)

      var findRoot func(*html.Node, string) *html.Node
      findRoot = func(n *html.Node, tag string) *html.Node {
          if n.Type == html.ElementNode && n.Data == tag { return n }
          for c := n.FirstChild; c != nil; c = c.NextSibling {
              if r := findRoot(c, tag); r != nil { return r }
          }
          return nil
      }

      root := findRoot(doc, "article")
      if root == nil { root = findRoot(doc, "main") }
      if root == nil { root = findRoot(doc, "body") }
      if root == nil { return "", title }

      var sb strings.Builder
      var walk func(*html.Node)
      walk = func(n *html.Node) {
          if n.Type == html.ElementNode {
              switch n.Data {
              case "script", "style", "nav", "aside", "noscript", "iframe":
                  return
              }
          }
          if n.Type == html.TextNode {
              t := strings.TrimSpace(n.Data)
              if t != "" { sb.WriteString(t); sb.WriteByte(' ') }
          }
          for c := n.FirstChild; c != nil; c = c.NextSibling { walk(c) }
      }
      walk(root)
      return strings.TrimSpace(sb.String()), title
  }
  ```

- [ ] Implement `rechart.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"

      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  var allowedChartTypes = map[string]bool{
      "line": true, "bar": true, "pie": true, "area": true, "scatter": true,
  }

  func NewRechartSkill(tc *ToolContext) *tool.Entry {
      return &tool.Entry{
          Name:        "rechart",
          Toolset:     "chart",
          Description: "Generate a chart (line/bar/pie/area/scatter) from data. The frontend renders the returned chart spec.",
          MaxResultChars: 4 * 1024,
          Schema:      core.ToolDefinition{ /* type: enum, data: object */ },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args struct {
                  Type string         `json:"type"`
                  Data map[string]any `json:"data"`
              }
              if err := json.Unmarshal(raw, &args); err != nil { return tool.Error(err.Error()), nil }
              if !allowedChartTypes[args.Type] {
                  return tool.Error(fmt.Sprintf("chart type %q not in [line,bar,pie,area,scatter]", args.Type)), nil
              }
              if args.Data == nil { return tool.Error("data is required"), nil }
              tc.Emit("Rendering " + args.Type + " chart")
              return tool.Result(map[string]any{
                  "chart_type": args.Type,
                  "spec":       args.Data,
                  "renderable": true,
              }), nil
          },
      }
  }
  ```

  > **Note on `tc.LM.Generate` in summarizer**: ensure secondary LLM call **does not** recurse into agent step loop. We call `tc.LM.Generate` directly, not `pantheonAgent.Run`. Confirmed safe.

- [ ] Write all 7 tests; run; pass.

### Acceptance

- web-scraping returns `<article>` body when present, falls back to `<body>`
- web-scraping rejects `ftp://`/`file://` schemes
- web-scraping `MaxResultChars=8K` truncates large pages
- rechart returns JSON with `chart_type`, `spec`, `renderable:true`
- Both tools registered into `tool.Registry` and discoverable via `Dispatch`

### Commit

`feat(agent/tools): web-scraping + rechart skills`

---

## Task 6: MCP projection adapter

**Files:**
- `backend/internal/agent/tools/mcp.go` (NEW)
- `backend/internal/agent/tools/mcp_test.go` (NEW)

**Tests:**
- `TestMCPProjection_BuildsEntryFromToolPlugin`
- `TestMCPProjection_PassesArgsCorrectly` (mock ToolPlugin.Call inspects args map)
- `TestMCPProjection_PropagatesError`
- `TestMCPProjection_EmitsStatusBeforeCall`
- `TestMCPProjection_EmptyInputSchema_NilParameters`

### Steps

- [ ] Implement `mcp.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"

      "github.com/odysseythink/hermind/backend/internal/mcp"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  func mcpToolToEntry(p mcp.ToolPlugin, emit StatusEmitter) *tool.Entry {
      var params *core.Schema
      if len(p.InputSchema) > 0 {
          params = &core.Schema{Raw: p.InputSchema}
      }
      return &tool.Entry{
          Name:        p.QualifiedName,
          Toolset:     "mcp",
          Description: p.Description,
          MaxResultChars: 8 * 1024,
          Schema: core.ToolDefinition{
              Name:        p.QualifiedName,
              Description: p.Description,
              Parameters:  params,
          },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              emit("Calling MCP tool: " + p.QualifiedName)
              var args map[string]any
              if len(raw) > 0 {
                  if err := json.Unmarshal(raw, &args); err != nil {
                      return tool.Error("invalid args: " + err.Error()), nil
                  }
              }
              result, err := p.Call(ctx, args)
              if err != nil { return tool.Error(err.Error()), nil }
              b, mErr := json.Marshal(result)
              if mErr != nil { return tool.Error("marshal result: " + mErr.Error()), nil }
              return string(b), nil
          },
      }
  }
  ```

  > **Verify `core.Schema.Raw` field**: if pantheon's `core.Schema` doesn't have a `Raw` field, switch to:
  > ```go
  > var schemaStruct core.Schema
  > _ = json.Unmarshal(p.InputSchema, &schemaStruct)
  > params = &schemaStruct
  > ```
  > Either way, the LLM sees a JSON Schema-compatible parameter spec.

- [ ] Write tests using a fake `mcp.ToolPlugin` whose `Call` is a closure:
  ```go
  func TestMCPProjection_PassesArgsCorrectly(t *testing.T) {
      var got map[string]any
      p := mcp.ToolPlugin{
          ServerName: "test", ToolName: "echo", QualifiedName: "test-echo",
          Description: "Echo back args",
          InputSchema: json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
          Call: func(ctx context.Context, args map[string]any) (any, error) {
              got = args
              return map[string]any{"echoed": args["msg"]}, nil
          },
      }
      var emitted []string
      emit := func(m string) { emitted = append(emitted, m) }
      e := tools.MCPToolToEntryForTesting(p, emit)
      out, err := e.Handler(context.Background(), json.RawMessage(`{"msg":"hello"}`))
      require.NoError(t, err)
      require.Equal(t, "hello", got["msg"])
      require.Contains(t, out, `"echoed":"hello"`)
      require.Len(t, emitted, 1)
      require.Contains(t, emitted[0], "test-echo")
  }
  ```

- [ ] Run tests; pass.

### Acceptance

- MCP `ToolPlugin` projects to a working `tool.Entry`
- Args marshalled correctly through the boundary
- Emit fires exactly once per Dispatch
- Empty `InputSchema` results in `nil` Parameters (LLM still allowed to call with no args)

### Commit

`feat(agent/tools): MCP ToolPlugin → pantheon tool.Entry projection`

---

## Task 7: AgentFlow projection (stub executor) + final wiring

**Files:**
- `backend/internal/agent/tools/flow.go` (NEW)
- `backend/internal/agent/tools/flow_test.go` (NEW)
- `backend/internal/agent/handler.go` (MODIFY — final wiring)
- `backend/internal/agent/handler_test.go` (MODIFY — full-suite e2e with MCP fixture)

**Tests:**
- `TestFlowProjection_ActiveFlowsBecomeEntries`
- `TestFlowProjection_InactiveFlowsExcluded`
- `TestFlowProjection_StubHandler_ReturnsDeferred`
- `TestHandleWS_FullToolingE2E` (mock LLM returns `{"tool_calls":[{"name":"rag-memory","arguments":"{\"action\":\"search\",\"content\":\"plato\"}"}]}` once, then a final text reply, then TERMINATE; assert WS sees status frames + final chat message)

### Steps

- [ ] Implement `flow.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "strings"

      "github.com/gosimple/slug"
      "github.com/odysseythink/hermind/backend/internal/services"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  func flowToEntry(f services.FlowSummary, flowSvc *services.AgentFlowService, emit StatusEmitter) *tool.Entry {
      name := "flow-" + slug.Make(strings.ToLower(f.Name))
      desc := f.Description
      if desc == "" { desc = "User-defined agent flow: " + f.Name }
      return &tool.Entry{
          Name:        name,
          Toolset:     "flow",
          Description: desc,
          MaxResultChars: 4 * 1024,
          Schema: core.ToolDefinition{
              Name:        name,
              Description: desc,
              Parameters:  nil,  // PR-AR-3 doesn't extract flow input schema yet
          },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              emit("Invoking flow: " + f.Name)
              // Loading the full flow happens here so we can return its metadata
              loaded, err := flowSvc.LoadFlow(f.UUID)
              if err != nil {
                  return tool.Error("flow load: " + err.Error()), nil
              }
              return tool.Result(map[string]any{
                  "status":      "deferred",
                  "flow_uuid":   f.UUID,
                  "flow_name":   loaded.Name,
                  "step_count":  len(loaded.Config.Steps),
                  "note":        "flow execution is not yet implemented",
              }), nil
          },
      }
  }
  ```

- [ ] Update `handler.go` to actually pass dependencies through:
  ```go
  // inside HandleWS
  bd := tools.BuilderDeps{
      VectorSearchSvc: r.deps.VectorSearchSvc,
      DocSvc:          r.deps.DocSvc,
      MCPHv:           r.deps.MCPHv,
      FlowSvc:         r.deps.FlowSvc,
      EventLog:        r.deps.EventLog,
      SysSvc:          r.deps.SysSvc,
      LM:              lm,
  }
  builder := tools.NewBuilder(bd)
  reg, err := builder.Build(c.Request.Context(), &ws, user, emit, settings)
  ```

- [ ] Add `BuilderDeps` to `internal/agent/tools/builder.go` (moved from Task 1 stub).

- [ ] Write `TestHandleWS_FullToolingE2E`: orchestrate a mock LLM that emits one tool_call to `rag-memory` (search) then a final reply. Seed a vector entry. Assert WS receives:
  1. Welcome `statusResponse`
  2. `statusResponse` "Searching memory: ..."
  3. (internal tool result, no frame)
  4. Final chat frame from `@agent`
  5. Close on TERMINATE

  > **Mock LLM tool-call shape**: pantheon's `core.Response.Message.Content` carries `ToolCallPart` items. Mock must build a `core.Message` with `ToolCallPart{Name, Arguments, ID}` for the first reply, then a `TextPart` for the second. Verify in `pantheon/core/content.go` the exact constructor; likely `core.NewToolCallMessage(...)` or similar.

- [ ] Run full test suite; verify all PR-AR-2 tests still pass + 4 new tests.

### Acceptance

- Active flows project; inactive flows skipped
- Stub handler returns deferred-status JSON, agent loop continues
- Full tool-calling e2e works end-to-end
- No regressions in PR-AR-1 / PR-AR-2 tests
- `main.go` boots cleanly

### Commit

`feat(agent/tools): AgentFlow projection (stub executor) + full e2e wiring`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` 100% green
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] Two decision artefacts written: `.gpowers/decisions/2026-05-27-agent-default-skills.md`, `.gpowers/decisions/2026-05-27-ragmemory-store-deferred.md`
- [ ] `internal/agent/doc.go` updated to reflect tool-registry availability
- [ ] `agent.Deps` fields documented in `runtime.go` godoc
- [ ] Manual smoke: with OPEN_AI_KEY set, send `@agent search my docs for "test"` and verify status frames + chat reply
- [ ] No new TODOs without `PR-AR-N` reference

## Risk notes

| Risk | Mitigation |
|---|---|
| `core.Schema.Raw` field name differs from assumed shape | Task 0 of each tool task: verify by reading `pantheon/core/tool.go`; switch to `json.Unmarshal(p.InputSchema, &schema)` if Raw doesn't exist |
| pantheon Agent step-loop's tool-call argument shape (string vs RawMessage) | Read `pantheon/agent/agent.go executeTool`; if `tc.Arguments` is a string, our `Handler(raw json.RawMessage)` will still receive valid JSON since pantheon does `json.RawMessage(tc.Arguments)` |
| MCP `ToolPlugin.Call` deadlocks under `Conversation.reply`'s ctx if MCP server hangs | Already protected by per-server concurrency limiter from PR-D + per-call 30s timeout; agent loop sees `tool.Error` and continues |
| AgentFlow projection lists deleted flows | `ListFlows` reads disk fresh each session; deleted flows simply disappear next session |
| Tool name collisions between MCP servers (rare but possible) | Dedup eventLog warns; later wins. MCP's `QualifiedName = "<server>-<tool>"` already pre-scopes |
| `tc.LM.Generate` inside docSummarizer adds latency + token cost | Document in skill docstring; PR-AR-5 may add a separate summarizer model config |
| `wsConn.Send` returning `ErrSlowReader` from inside a tool emit drops status frames | Each emit ignores error explicitly; status frames are advisory, not control |
| Builder rebuilds per session, but MCP `ActiveServers/ToolsAsPlugins` is fast | OK — under 5ms even with 20 servers; no caching needed |

## Estimate

| Task | Hours |
|---|---|
| 1. Wire Participant→Agent + Deps expansion + empty stub | 1.5 |
| 2. Builder composition + disabled filter + dedup | 1.5 |
| 3. rag-memory skill (search; store deferred) | 1.5 |
| 4. document-summarizer skill | 2.0 |
| 5. web-scraping + rechart skills | 2.0 |
| 6. MCP projection adapter | 1.0 |
| 7. AgentFlow projection + final e2e | 2.0 |
| **Total** | **11.5** (design estimate 10-12h, mid-range ✓) |

—— end of plan
