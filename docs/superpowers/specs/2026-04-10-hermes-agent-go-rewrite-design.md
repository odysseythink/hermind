# Hermes Agent Go Rewrite - Design Spec

## Overview

100% rewrite of hermes-agent (Python) to Go, using a modular monolith architecture. The Go version maintains full configuration/data compatibility with the original Python project while adopting Go-idiomatic patterns.

### Goals
- **Performance**: goroutine-based concurrency replacing Python GIL limitations
- **Deployment**: single static binary, zero runtime dependencies
- **Maintainability**: Go type system and interfaces replacing 6800+ line Python files
- **Compatibility**: read existing `~/.hermes/config.yaml`, `state.db`, and skills directory

### Scope
- All original features: agent engine, CLI, 53 tools, 21 platform adapters, 6 terminal backends, 9 memory backends, MCP, cron, ACP
- Additional LLM providers: DeepSeek, Qwen (通义千问), Zhipu (智谱GLM), Wenxin (文心), Kimi (月之暗面), MiniMax
- Dual database: SQLite (local) + PostgreSQL (multi-instance gateway)
- Go 1.24 minimum

---

## System Data Flow

### Conversation Flow (CLI)

```
┌──────────┐       ┌──────────┐       ┌──────────┐      ┌──────────┐
│   User   │──────▶│   CLI    │──────▶│  Engine  │─────▶│ Provider │
│  (term)  │       │  (REPL)  │       │          │      │  (LLM)   │
└──────────┘       └──────────┘       └────┬─────┘      └────┬─────┘
                         ▲                  │                 │
                         │                  │◀────stream──────┘
                         │                  ▼
                         │             ┌──────────┐
                         │             │   Tool   │
                         │             │ Registry │
                         │             └────┬─────┘
                         │                  │
                         │                  ▼
                         │             ┌──────────┐
                         │             │ Terminal/│
                         │             │  File/   │
                         │             │  Web/... │
                         │             └────┬─────┘
                         │                  │
                         │◀───results───────┘
                         │
                         └────────┐
                                  ▼
                           ┌──────────┐
                           │ Storage  │
                           │(SQLite/  │
                           │  PG)     │
                           └──────────┘
```

### Gateway Message Flow

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────┐
│  Telegram/  │     │   Platform   │     │   Gateway    │     │  Session │
│   Discord/  │────▶│   Adapter    │────▶│  Orchestrator│────▶│ Manager  │
│   21 total  │     │  (goroutine) │     │  (errgroup)  │     │          │
└─────────────┘     └──────────────┘     └──────┬───────┘     └────┬─────┘
                                                 │                  │
                                                 ▼                  │
                                          ┌──────────┐              │
                                          │  Engine  │◀──history────┘
                                          │(per-msg) │
                                          └────┬─────┘
                                               │
                                               ▼
                                          ┌──────────┐
                                          │ Delivery │
                                          │  Router  │
                                          └────┬─────┘
                                               │
                                               ▼
                                    ┌────────────────┐
                                    │  Back to user  │
                                    │ via platform   │
                                    └────────────────┘
```

**Concurrency model:**
- Each platform runs as an independent goroutine (via `errgroup`)
- Each message gets a fresh `Engine` instance (single-use, not thread-safe)
- Same-session messages serialized via `LiveSession.Mutex` to preserve ordering
- Tool execution within a single conversation uses a bounded `errgroup` (max 8 concurrent)

---

## Architecture: Modular Monolith

```
hermes-agent-go/
├── cmd/hermes/             # Single entry point (subcommands: run, gateway, cron)
├── agent/                  # Core engine module
├── provider/               # LLM Provider module
│   ├── openaicompat/       # Shared base for OpenAI-compatible providers
│   ├── anthropic/
│   ├── openai/
│   ├── openrouter/
│   ├── deepseek/
│   ├── qwen/
│   ├── zhipu/
│   ├── wenxin/
│   ├── kimi/
│   └── minimax/
├── tool/                   # Tool module
│   ├── terminal/           # 6 backends (local, docker, ssh, modal, daytona, singularity)
│   ├── file/
│   ├── web/
│   ├── browser/
│   ├── code/
│   ├── delegate/
│   ├── memory/
│   ├── skill/
│   ├── mcp/
│   └── vision/
├── gateway/                # Gateway module
│   └── platform/           # 21 platform adapters
├── skill/                  # Skill loading & injection
├── memory/                 # Memory Provider interface + implementations
├── storage/                # Storage interface + SQLite/PostgreSQL
├── config/                 # Configuration loading
├── cli/                    # CLI application + TUI
│   └── ui/                 # REPL, input, renderer
├── cron/                   # Scheduler
├── acp/                    # ACP protocol adapter
├── message/                # Shared message types
├── Makefile
├── Dockerfile
└── go.mod
```

---

## Section 1: Core Type System & Message Model

### Message Types (`message/`)

```go
// message/message.go

type Role string
const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
    RoleSystem    Role = "system"
)

type Message struct {
    Role            Role            `json:"role"`
    Content         Content         `json:"content"`
    ToolCalls       []ToolCall      `json:"tool_calls,omitempty"`
    ToolCallID      string          `json:"tool_call_id,omitempty"`
    ToolName        string          `json:"tool_name,omitempty"`
    Reasoning       string          `json:"reasoning,omitempty"`
    FinishReason    string          `json:"finish_reason,omitempty"`
}

// Content is a typed union of plain text and structured content blocks.
// Exactly one of text or blocks is populated. Never both.
type Content struct {
    text   string          // simple text message
    blocks []ContentBlock  // multimodal or tool_use/tool_result
}

// Constructors — forces explicit choice at creation time.
func TextContent(s string) Content {
    return Content{text: s}
}

func BlockContent(blocks []ContentBlock) Content {
    return Content{blocks: blocks}
}

// Accessors — typed, no assertions required.
func (c Content) IsText() bool       { return c.blocks == nil }
func (c Content) Text() string       { return c.text }
func (c Content) Blocks() []ContentBlock { return c.blocks }

// MarshalJSON produces the OpenAI-compatible shape: string OR array.
func (c Content) MarshalJSON() ([]byte, error) {
    if c.IsText() {
        return json.Marshal(c.text)
    }
    return json.Marshal(c.blocks)
}

// UnmarshalJSON accepts both shapes.
func (c *Content) UnmarshalJSON(data []byte) error {
    // Try string first
    var s string
    if err := json.Unmarshal(data, &s); err == nil {
        c.text = s
        return nil
    }
    // Fall back to array
    var blocks []ContentBlock
    if err := json.Unmarshal(data, &blocks); err != nil {
        return fmt.Errorf("content must be string or []ContentBlock: %w", err)
    }
    c.blocks = blocks
    return nil
}

type ContentBlock struct {
    Type     string `json:"type"`      // "text", "image_url", "tool_use", "tool_result"
    Text     string `json:"text,omitempty"`
    ImageURL *Image `json:"image_url,omitempty"`
}

type ToolCall struct {
    ID       string          `json:"id"`
    Type     string          `json:"type"`     // "function"
    Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"` // JSON string
}
```

### API Response Types

```go
// message/response.go

type StreamDelta struct {
    Content    string     `json:"content,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    Reasoning  string     `json:"reasoning,omitempty"`
}

type APIResponse struct {
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"`
    Usage        Usage   `json:"usage"`
}

type Usage struct {
    InputTokens      int `json:"input_tokens"`
    OutputTokens     int `json:"output_tokens"`
    CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
    CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
    ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}
```

### Tool Schema Types

```go
// tool/schema.go

type ToolDefinition struct {
    Type     string       `json:"type"`     // "function"
    Function FunctionDef  `json:"function"`
}

type FunctionDef struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}
```

**Design notes:**
- `Message.Content` uses `any` to support both plain text (`string`) and multimodal (`[]ContentBlock`)
- Compatible with OpenAI message format; each provider converts internally
- `json.RawMessage` for tool parameter schemas avoids over-modeling JSON Schema

---

## Section 2: Provider Interface & LLM Adapters

### Provider Interface

```go
// provider/provider.go

type Provider interface {
    Name() string
    Complete(ctx context.Context, req *Request) (*Response, error)
    Stream(ctx context.Context, req *Request) (Stream, error)
    ModelInfo(model string) *ModelInfo
    EstimateTokens(model string, text string) (int, error)
    Available() bool
}

// Error is the shared error taxonomy. Each provider converts vendor-specific
// errors to this type so the fallback chain and retry logic work consistently.
type Error struct {
    Kind       ErrorKind
    Provider   string  // "anthropic", "openai", ...
    StatusCode int     // HTTP status if available
    Message    string
    Cause      error   // wrapped original error
}

type ErrorKind int

const (
    ErrUnknown ErrorKind = iota
    ErrRateLimit      // 429: retry with backoff, eligible for fallback
    ErrAuth           // 401/403: do not retry, do not fallback (config issue)
    ErrContentFilter  // content blocked: do not retry, return to user
    ErrInvalidRequest // 400: do not retry, likely a bug
    ErrTimeout        // request timeout: retry once, then fallback
    ErrServerError    // 5xx: retry with backoff, eligible for fallback
    ErrContextTooLong // context window exceeded: trigger compression, do not fallback
)

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

// IsRetryable decides whether the fallback chain should try the next provider.
func IsRetryable(err error) bool {
    var pErr *Error
    if !errors.As(err, &pErr) {
        return false
    }
    switch pErr.Kind {
    case ErrRateLimit, ErrTimeout, ErrServerError:
        return true
    default:
        return false
    }
}

type Stream interface {
    Recv() (*StreamEvent, error)
    Close() error
}

type StreamEvent struct {
    Type     StreamEventType // EventDelta, EventDone, EventError
    Delta    *StreamDelta
    Response *Response       // Only set on EventDone
    Err      error
}

type Request struct {
    Model          string
    SystemPrompt   string
    Messages       []message.Message
    Tools          []tool.ToolDefinition
    MaxTokens      int
    Temperature    *float64
    TopP           *float64
    CacheControl   *CacheControl   // Anthropic prompt caching
    StopSequences  []string
}

type Response struct {
    Message      message.Message
    FinishReason string
    Usage        message.Usage
    Model        string
}

type ModelInfo struct {
    ContextLength    int
    MaxOutputTokens  int
    SupportsVision   bool
    SupportsTools    bool
    SupportsStreaming bool
    SupportsCaching  bool
    SupportsReasoning bool
}
// Note: token estimation moved to Provider.EstimateTokens() method above.
```

### Provider Registry

```go
// provider/registry.go

type Factory func(cfg config.ProviderConfig) (Provider, error)

var registry = map[string]Factory{}

func Register(name string, factory Factory) {
    registry[name] = factory
}

func New(name string, cfg config.ProviderConfig) (Provider, error) {
    f, ok := registry[name]
    if !ok {
        return nil, fmt.Errorf("unknown provider: %s", name)
    }
    return f(cfg)
}

// Each provider registers in init():
// provider/anthropic/init.go:   provider.Register("anthropic", New)
// provider/deepseek/init.go:    provider.Register("deepseek", New)
```

### Provider Implementation Strategy

| Provider | Protocol | Special Handling |
|----------|---------|---------|
| Anthropic | Anthropic Messages API | prompt caching, extended thinking |
| OpenAI | OpenAI Chat Completions | Standard |
| OpenRouter | OpenAI-compatible | model mapping, routing headers |
| DeepSeek | OpenAI-compatible | custom base_url |
| Qwen | OpenAI-compatible | DashScope base_url |
| Zhipu (GLM) | OpenAI-compatible | custom auth header |
| Wenxin | Baidu proprietary API | independent impl, access_token refresh |
| Kimi | OpenAI-compatible | custom base_url |
| MiniMax | OpenAI-compatible | custom base_url |

Most Chinese providers are OpenAI-compatible. Shared `openaicompat` base implementation, override only differences. Wenxin requires fully independent implementation (proprietary API + OAuth token refresh). Anthropic needs independent implementation (different message format + cache_control).

### Fallback Chain

```go
// provider/fallback.go

// FallbackChain is single-use, not thread-safe.
// Create a new chain per conversation to avoid shared mutable state.
type FallbackChain struct {
    providers []Provider
}

func NewFallbackChain(providers []Provider) *FallbackChain {
    return &FallbackChain{providers: providers}
}

func (fc *FallbackChain) Complete(ctx context.Context, req *Request) (*Response, error) {
    for i := 0; i < len(fc.providers); i++ {
        resp, err := fc.providers[i].Complete(ctx, req)
        if err == nil {
            return resp, nil
        }
        if !isRetryable(err) {
            return nil, err
        }
        // Continue to next provider in chain
    }
    return nil, ErrAllProvidersFailed
}
```

**Concurrency note:** `FallbackChain` holds no mutable state. Each `RunConversation` call in the Engine creates a fresh chain via `NewFallbackChain(...)`, matching the Python version's per-conversation lifecycle. This avoids race conditions in the gateway scenario where multiple goroutines would otherwise share a chain.

---

## Section 3: Agent Engine

### Core Structure

```go
// agent/engine.go

// Engine is single-use per conversation. NOT thread-safe.
// The gateway creates a fresh Engine per incoming message.
// The CLI creates a fresh Engine per /run invocation.
type Engine struct {
    provider    provider.Provider
    tools       *tool.Registry
    storage     storage.Storage
    skillLoader *skill.Loader
    prompt      *PromptBuilder
    config      config.AgentConfig  // value, not pointer — immutable snapshot
    
    onStreamDelta  func(delta *message.StreamDelta)
    onStep         func(step *StepEvent)
    onThinking     func(text string)
    onStatus       func(status string)
}

type ConversationResult struct {
    Response     message.Message
    Messages     []message.Message
    SessionID    string
    Usage        message.Usage
    Cost         float64
    ToolCalls    int
    Iterations   int
}
```

### Conversation Loop

```go
// agent/engine.go

func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
    session := e.initSession(opts)
    history := opts.History
    budget := NewBudget(e.config.MaxTurns) // Default 90
    
    for budget.Remaining() > 0 {
        if err := ctx.Err(); err != nil {
            return result, err
        }
        if e.shouldCompress(history) {
            history = e.compress(ctx, session, history)
        }
        req := e.buildRequest(session, history)
        stream, err := e.provider.Stream(ctx, req)
        // error handling, retry, fallback
        resp := e.collectStream(stream)
        history = append(history, resp.Message)
        budget.Consume()
        if len(resp.Message.ToolCalls) == 0 {
            break
        }
        results := e.executeTools(ctx, resp.Message.ToolCalls, budget)
        history = append(history, results...)
        if budget.Ratio() > 0.7 {
            e.injectBudgetWarning(history, budget)
        }
    }
    return e.buildResult(session, history), nil
}

type RunOptions struct {
    UserMessage   string
    SystemPrompt  string
    History       []message.Message
    SessionID     string
    Platform      string
    UserID        string
    SkipContext   bool
    ParentBudget  *Budget
}
```

### Iteration Budget (Thread-safe)

```go
// agent/budget.go

type Budget struct {
    max       int
    remaining atomic.Int32
}

func NewBudget(max int) *Budget {
    b := &Budget{max: max}
    b.remaining.Store(int32(max))
    return b
}

func (b *Budget) Consume() bool { return b.remaining.Add(-1) >= 0 }
func (b *Budget) Refund()       { b.remaining.Add(1) }
func (b *Budget) Ratio() float64 {
    return 1.0 - float64(b.remaining.Load())/float64(b.max)
}
```

### Tool Execution (Parallel/Sequential)

```go
// agent/executor.go

func (e *Engine) executeTools(ctx context.Context, calls []message.ToolCall, budget *Budget) []message.Message {
    if len(calls) == 1 || !e.canParallelize(calls) {
        return e.executeSequential(ctx, calls)
    }
    return e.executeParallel(ctx, calls)
}

func (e *Engine) canParallelize(calls []message.ToolCall) bool {
    for _, c := range calls {
        if e.tools.IsInteractive(c.Function.Name) {
            return false
        }
    }
    return !e.hasPathConflict(calls)
}

func (e *Engine) executeParallel(ctx context.Context, calls []message.ToolCall) []message.Message {
    results := make([]message.Message, len(calls))
    g, gctx := errgroup.WithContext(ctx)
    g.SetLimit(8) // Max 8 concurrent
    for i, call := range calls {
        g.Go(func() error {
            results[i] = e.executeSingle(gctx, call)
            return nil
        })
    }
    g.Wait()
    return results
}
```

### Context Compression

```go
// agent/compression.go

type CompressionConfig struct {
    Enabled     bool    // Default true
    Threshold   float64 // Default 0.5
    TargetRatio float64 // Default 0.2
    ProtectLast int     // Default 20
    MaxPasses   int     // Default 3
}

// Preserves first 3 + last ProtectLast messages.
// Summarizes middle N turns with auxiliary LLM.
// Creates new session with parent_session_id chain.
```

### Prompt Builder

```go
// agent/prompt.go

type PromptBuilder struct {
    config   *config.Config
    platform string
}

func (pb *PromptBuilder) Build(opts *PromptOptions) string {
    // Assembles: identity + context files (SOUL.md, AGENTS.md, .hermes.md)
    // + tool guidance (per model) + memory guidance + skills guidance
    // + platform hints + injection protection
}
```

**Design notes:**
- Always use streaming (even without callbacks) for health detection: 90s stale-stream timeout
- `errgroup` for parallel tool execution, max 8 concurrent
- Compression creates new session chain for audit trail
- Prompt builder is stateless

---

## Section 4: Tool System

### Tool Registry

```go
// tool/registry.go

type Handler func(ctx context.Context, args json.RawMessage) (string, error)
type CheckFunc func() bool

type Entry struct {
    Name           string
    Toolset        string          // "terminal", "web", "file", ...
    Schema         ToolDefinition
    Handler        Handler
    CheckFn        CheckFunc
    RequiresEnv    []string
    IsInteractive  bool
    MaxResultChars int
    Description    string
    Emoji          string
}

type Registry struct {
    mu      sync.RWMutex
    entries map[string]*Entry
}

func (r *Registry) Register(entry *Entry)
func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error)
func (r *Registry) Definitions(filter func(*Entry) bool) []ToolDefinition

func ToolError(msg string) string { return mustJSON(map[string]any{"error": msg}) }
func ToolResult(data any) string  { return mustJSON(data) }
```

### Built-in Tools

```
tool/
├── registry.go
├── terminal/           # local, docker, ssh, modal, daytona, singularity
├── file/               # read_file, write_file, search_files, patch_file, list_directory
├── web/                # web_search (Exa), web_extract (Firecrawl), web_fetch
├── browser/            # Browserbase integration
├── code/               # sandboxed execution
├── delegate/           # subagent spawning
├── memory/             # memory_save, memory_search, session_search
├── skill/              # skill_create, skill_manage
├── mcp/                # MCP client + bridge
└── vision/             # image analysis
```

### Terminal Backend Interface

```go
// tool/terminal/terminal.go

type Backend interface {
    Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error)
    SupportsPersistentShell() bool
    Close() error
}

type ExecOptions struct {
    Cwd     string
    Env     map[string]string
    Timeout time.Duration
    Stdin   string
}

type ExecResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Duration time.Duration
}

func NewBackend(backendType string, cfg config.TerminalConfig) (Backend, error)
```

### MCP Bridge

```go
// tool/mcp/bridge.go

type Bridge struct {
    registry *tool.Registry
    clients  map[string]*clientState  // server_name → state
    mu       sync.RWMutex
}

type clientState struct {
    client    *Client
    transport Transport
    toolNames []string  // tools registered by this server (for cleanup)
    status    ConnStatus
}

// Connects to MCP server, pulls tool list, registers into Registry.
// Namespace: "serverName__toolName" to avoid conflicts.
func (b *Bridge) Connect(serverName string, transport Transport) error

// Disconnect unregisters all tools from this server and closes the client.
func (b *Bridge) Disconnect(serverName string) error
```

### MCP Failure Handling

**Reconnection strategy:**
- On transport error (connection dropped): attempt reconnection with exponential backoff
- Backoff schedule: 1s, 2s, 4s (max 3 attempts over ~7 seconds)
- During reconnection: tool calls to that server return `McpUnavailable` error
- After max retries: unregister all tools from this server, log permanent failure, fire `onStatus` callback to notify user

**Tool call failure:**
- MCP tool handler wraps transport calls in a short timeout (default 30s)
- On timeout or transport error: return structured error via `tool.ToolError()`
- Error message format: `MCP server '<name>' unreachable: <cause>. Tools: <list>`
- Engine treats this as a normal tool error (conversation continues, LLM sees the error)

**Visible to user:**
- CLI: inline error in conversation flow using error color
- Gateway: error logged, user sees `⚠ Tool <name> failed: server unreachable` in response

**Why this matters:** MCP servers are external processes (Python scripts, Node tools, etc.) and can crash or hang. Without reconnection + graceful unregistration, a dead MCP server poisons every subsequent conversation turn with cryptic transport errors. The agent doesn't know the tool is broken, so it keeps trying.

**Design notes:**
- Unified handler signature: `func(ctx, json.RawMessage) (string, error)`
- Terminal backends use factory pattern (fixed count, require config)
- MCP tools dynamically registered via Bridge at runtime
- Registry is thread-safe for runtime registration (MCP scenario)

---

## Section 5: Storage Layer

### Storage Interface

```go
// storage/storage.go

// Storage is the root interface. Implementations must be safe for concurrent use.
type Storage interface {
    // Session
    CreateSession(ctx context.Context, session *Session) error
    GetSession(ctx context.Context, id string) (*Session, error)
    UpdateSession(ctx context.Context, id string, updates *SessionUpdate) error
    ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, error)
    
    // Messages
    AddMessage(ctx context.Context, sessionID string, msg *StoredMessage) error
    GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error)
    SearchMessages(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)
    
    // System Prompt cache (Anthropic prefix caching)
    UpdateSystemPrompt(ctx context.Context, sessionID string, prompt string) error
    
    // Usage
    UpdateUsage(ctx context.Context, sessionID string, usage *UsageUpdate) error
    
    // Transactions — group multiple operations atomically
    WithTx(ctx context.Context, fn func(tx Tx) error) error
    
    // Lifecycle
    Close() error
    Migrate() error
}

// Tx is the transaction-scoped interface. Same methods as Storage (except
// lifecycle and WithTx). Operations within a Tx are atomic: either all commit
// or all roll back.
type Tx interface {
    CreateSession(ctx context.Context, session *Session) error
    GetSession(ctx context.Context, id string) (*Session, error)
    UpdateSession(ctx context.Context, id string, updates *SessionUpdate) error
    AddMessage(ctx context.Context, sessionID string, msg *StoredMessage) error
    UpdateSystemPrompt(ctx context.Context, sessionID string, prompt string) error
    UpdateUsage(ctx context.Context, sessionID string, usage *UsageUpdate) error
}
```

**Transaction usage pattern:**

```go
// At end of conversation, save messages + usage atomically
err := storage.WithTx(ctx, func(tx Tx) error {
    for _, msg := range newMessages {
        if err := tx.AddMessage(ctx, sessionID, msg); err != nil {
            return err
        }
    }
    if err := tx.UpdateUsage(ctx, sessionID, &usage); err != nil {
        return err
    }
    return tx.UpdateSession(ctx, sessionID, &update)
})
```

**Why this matters:** Without transactions, a crash between `AddMessage` and `UpdateUsage` leaves the session with correct messages but wrong token counts forever. The Python version uses SQLite's implicit transaction on each write; Go needs explicit transaction support because Engine batches multiple writes at conversation end.

### Session & Message Types

```go
type Session struct {
    ID              string
    Source          string    // "cli", "telegram", ...
    UserID          string
    Model           string
    ModelConfig     json.RawMessage
    SystemPrompt    string
    ParentSessionID string    // compression chain
    StartedAt       time.Time
    EndedAt         *time.Time
    EndReason       string
    MessageCount    int
    ToolCallCount   int
    Usage           SessionUsage
    BillingProvider string
    BillingBaseURL  string
    EstimatedCost   float64
    ActualCost      float64
    CostStatus      string
    Title           string
}

type StoredMessage struct {
    ID               int64
    SessionID        string
    Role             string
    Content          string
    ToolCallID       string
    ToolCalls        json.RawMessage
    ToolName         string
    Timestamp        time.Time
    TokenCount       int
    FinishReason     string
    Reasoning        string
    ReasoningDetails string
}
```

### SQLite Implementation

- WAL mode, `PRAGMA foreign_keys=ON`, `busy_timeout=1000`
- Write transactions: `BEGIN IMMEDIATE` + 15 retries with random jitter
- WAL checkpoint every 50 writes (`PRAGMA wal_checkpoint(PASSIVE)`)
- FTS5 virtual table for full-text search
- Schema fully compatible with original Python `state.db`

### PostgreSQL Implementation

- `pgx/v5` + `pgxpool` for connection pooling
- `tsvector` replaces FTS5 for full-text search
- Advisory locks replace SQLite's IMMEDIATE transactions
- Same Storage interface, different implementation

### Storage Driver Selection Guide

| Scenario | Driver | Rationale |
|----------|--------|-----------|
| Single-user CLI | SQLite | Zero setup, file-based, fast for single writer |
| Single-instance gateway (< 50 msgs/sec) | SQLite | WAL mode handles light concurrency well |
| Single-instance gateway (50+ msgs/sec) | PostgreSQL | Avoid WAL write contention under load |
| Multi-instance gateway (HA, load-balanced) | PostgreSQL | SQLite can't be safely shared across processes |
| Read-heavy analytics / reporting | PostgreSQL | Better concurrent reader performance, richer query planner |
| Docker/container deployment | PostgreSQL | Avoid ephemeral filesystem issues; externalize state |

**Default:** SQLite. Users upgrade to PostgreSQL when they hit contention or deploy multiple gateway instances. Switching is a config change — no code changes required (both drivers implement the same `Storage` interface).

### Storage Factory

```go
func New(cfg *config.StorageConfig) (Storage, error) {
    switch cfg.Driver {
    case "sqlite", "":
        return sqlite.New(cfg.SQLitePath)
    case "postgres":
        return postgres.New(cfg.PostgresURL)
    }
}
```

---

## Section 6: Configuration System

### Config Structure

```go
// config/config.go

type Config struct {
    Model             string                     `yaml:"model"`
    Providers         map[string]ProviderConfig   `yaml:"providers"`
    FallbackProviders []ProviderConfig            `yaml:"fallback_providers"`
    Agent             AgentConfig                 `yaml:"agent"`
    Terminal          TerminalConfig              `yaml:"terminal"`
    Browser           BrowserConfig               `yaml:"browser"`
    Auxiliary         AuxiliaryConfig              `yaml:"auxiliary"`
    Storage           StorageConfig                `yaml:"storage"`
    Gateway           GatewayConfig                `yaml:"gateway"`
    Checkpoints       CheckpointConfig             `yaml:"checkpoints"`
}
```

### Loading Priority

```
Environment variables > config.yaml > .env > defaults
```

Fully compatible with original `~/.hermes/config.yaml` schema.

### Defaults

- Model: `anthropic/claude-opus-4-6`
- Max turns: 90
- Gateway timeout: 1800s
- Compression: enabled, threshold 0.5, target ratio 0.2, protect last 20
- Terminal: local backend, 180s timeout, persistent shell
- Storage: SQLite driver

---

## Section 7: CLI Interface

### Subcommands (cobra)

```
hermes              # Interactive REPL (default)
hermes run          # Same as above
hermes gateway      # Start messaging gateway
hermes cron         # Start scheduler
hermes setup        # Interactive config wizard
hermes session      # View/search history
hermes skill        # Manage skills
hermes model        # List/switch models
hermes config       # View/modify config
hermes version      # Version info
```

### TUI (charmbracelet ecosystem)

- `bubbletea` + `bubbles` — framework + input components
- `glamour` — markdown rendering in terminal
- `lipgloss` — styling and themes
- Multi-line input (Shift+Enter newline, Enter submit)
- Slash command autocomplete (Tab)
- History navigation (Up/Down)
- Skin/theme system (3 built-in skins, see below)

### Built-in Skins

| Skin | Color Support | Accent | Background | Use Case |
|------|--------------|--------|------------|----------|
| **Default** | Truecolor | Amber/Gold (#FFB800) | Charcoal (#1E1E1E) | Modern terminals (iTerm2, Alacritty, WezTerm) |
| **Classic** | 16-color ANSI | Yellow (ANSI 3) | Default terminal bg | SSH sessions, older terminals, tmux |
| **Minimal** | No color | None (bold/dim only) | Default terminal bg | CI/CD logs, piped output, accessibility |

**What skins control:** prompt character style, accent color for headings/highlights, error color, tool call formatting color, status bar style, spinner animation style. Skins do NOT change layout or information hierarchy.

**Auto-detection:** Default skin selected based on `$COLORTERM` (truecolor), `$TERM` (256color/16color), or pipe detection (`!isatty` → Minimal). Override via `config.yaml` → `cli.skin: "classic"`

### CLI Design Tokens

**Symbol vocabulary:**
| Symbol | Meaning | Context |
|--------|---------|---------|
| `>` | User input prompt | Conversation |
| `◆` | Agent thinking/processing | Status |
| `⚡` | Tool execution | Tool calls |
| `│` | Output continuation | Tool output lines |
| `└` | Output end | Last tool output line |
| `▊` | Streaming cursor | During LLM streaming |
| `⚠` | Warning | Budget, deprecation |
| `✗` | Error | Failed operations |
| `◈` | Active indicator | Status bar |

**Semantic colors (Default skin):**
| Color | Hex | Usage |
|-------|-----|-------|
| Accent | #FFB800 (amber) | Headings, prompt character, highlights |
| Success | #4EC9B0 (teal) | Successful tool execution, cost display |
| Warning | #E5C07B (warm yellow) | Budget warnings, deprecation |
| Error | #E06C75 (soft red) | Errors, non-zero exit codes |
| Muted | #6C7A89 (gray) | Tool commands, timestamps, metadata |
| Code | #98C379 (green) | Code blocks, file paths |

**Spacing rules:**
- 1 blank line between user message and agent response
- 0 blank lines between tool call header and tool output
- 1 blank line after tool output block before next content
- 2-space indent for tool output lines (after `│`)
- Status bar: fixed to terminal bottom, 1 line tall

### REPL Visual Hierarchy (4-zone layout)

```
┌─────────────────────────────────────────────────────┐
│  ██  HERMES AGENT  ██                               │  Zone 1: Brand banner
│  claude-opus-4-6 · ~/.hermes · session #a3f2       │  Zone 2: Context bar
├─────────────────────────────────────────────────────┤
│                                                     │
│  > user message here                                │  Zone 3: Conversation
│                                                     │
│  ◆ Thinking...                                      │
│                                                     │
│  Response with **markdown** rendering               │
│  ```go                                              │
│  func main() { ... }                                │
│  ```                                                │
│                                                     │
│  ⚡ terminal: ls -la                                │  Tool call indicator
│  │ total 48                                         │
│  │ drwxr-xr-x  12 user  staff  384 Apr 10 14:00 . │
│  └ exit 0 (0.02s)                                   │
│                                                     │
├─────────────────────────────────────────────────────┤
│  tokens: 1.2k↑ 3.4k↓  cost: $0.08  ◈ streaming    │  Zone 4: Status bar
└─────────────────────────────────────────────────────┘
```

**Startup performance target:** First prompt visible within 200ms. Config loading and skill scanning happen eagerly; MCP connections and provider health checks happen lazily (first use).

**Exit experience:** On `/exit` or Ctrl+D, show a one-line session summary: `Session #a3f2: 12 messages, 8 tool calls, $0.34 · saved to ~/.hermes/state.db`. No confirmation prompt on exit (trust the user).

- **Zone 1 (Banner):** ASCII art "HERMES" logo, shown once on startup. Compact (3-4 lines max).
- **Zone 2 (Context bar):** Model name, config directory, session ID. Updates on model/session change.
- **Zone 3 (Conversation):** Scrollable. User messages prefixed with `>`. Agent responses rendered as markdown via glamour. Tool calls shown with icon + command + collapsed output.
- **Zone 4 (Status bar):** Persistent. Shows input/output token counts, estimated cost, and current state (idle/streaming/thinking/executing).

### Interaction States

Every user-visible feature must specify what the user sees in each state:

```
FEATURE              | LOADING              | EMPTY               | ERROR                    | SUCCESS              | PARTIAL
---------------------|----------------------|---------------------|--------------------------|----------------------|------------------
REPL startup         | "Loading config..."  | N/A                 | Config parse error msg   | Banner + prompt      | N/A
LLM streaming        | "◆ Thinking..."      | N/A                 | API error + retry count  | Rendered markdown    | Partial stream shown as-is
Tool execution       | "⚡ running: <cmd>"  | N/A                 | Exit code + stderr shown | Output + duration    | Timeout: partial output + warning
Session search       | "Searching..."       | "No sessions found" | DB error message         | Formatted list       | Paginated with "more..."
Skill loading        | "Loading skills..."  | "No skills found"   | Parse error + file path  | Silent (injected)    | N/A
Context compression  | "Compressing..."     | N/A                 | Fallback to truncation   | "Compressed N→M msgs"| N/A
Provider fallback    | "Retrying with ..."  | N/A                 | "All providers failed"   | Silent provider swap | N/A
Cost display         | N/A                  | "$0.00"             | "Cost estimate unavail." | "$X.XX (estimated)"  | N/A
```

**Error display pattern:** All errors shown inline in the conversation flow, styled with red/error color. Errors include: what failed, why (if known), and what happens next (retry, fallback, abort). Never silently swallow errors.

**Streaming display:** Characters appear as received. Markdown rendering happens after stream completes (not during — avoids flicker). During streaming, raw text with a blinking cursor indicator `▊`.

**Tool call display pattern:**
- Start: `⚡ <tool_name>: <command_summary>` (dimmed/muted color)
- Running: spinner animation next to the tool line
- Complete: output indented under tool line, prefixed with `│`. Duration shown. Exit code if non-zero.
- Collapsed by default if output > 20 lines, with "[+N lines]" expand hint

**Budget warning display:**
- 70% consumed: subtle dimmed note `[budget: 27/90 remaining]`
- 90% consumed: yellow warning `⚠ [budget: 9/90 — wrapping up]`

### Terminal Responsive Behavior

| Terminal Width | Adaptation |
|---------------|-----------|
| **80+ cols** (normal) | Full layout: banner, context bar, status bar, tool output |
| **60-79 cols** (narrow) | Compact banner (single line "HERMES v1.0"), truncated context bar |
| **40-59 cols** (very narrow) | No banner, no context bar, status bar shows only cost. Tool output hard-wrapped |
| **<40 cols** | Warning on startup: "Terminal too narrow for optimal display". Minimal mode forced |

**Terminal height:** Status bar always visible. Conversation scrolls. If terminal < 10 rows, status bar hidden.

**Resize handling:** Listen for SIGWINCH. Redraw status bar and re-wrap current output on resize. No full repaint (avoid flicker).

### Accessibility

**Keyboard shortcuts:**
| Key | Action |
|-----|--------|
| Enter | Submit message |
| Shift+Enter | New line in input |
| Ctrl+C | Interrupt current operation (cancel streaming/tool, return to prompt) |
| Ctrl+D | Exit REPL (with session summary) |
| Ctrl+L | Clear screen (keep history) |
| Tab | Autocomplete slash command |
| Up/Down | Navigate input history |
| Ctrl+A / Ctrl+E | Move to start/end of input line |
| Ctrl+U / Ctrl+K | Clear line before/after cursor |

**Ctrl+C behavior (critical):**
- During idle (at prompt): no-op (don't exit, unlike typical shells)
- During LLM streaming: cancel stream, show partial response, return to prompt
- During tool execution: kill tool process, show partial output + "interrupted", return to prompt
- Double Ctrl+C within 1s: force exit (safety valve)

**Screen reader support:**
- Minimal skin auto-selected when `$TERM=dumb` or piped output
- No spinner animations in Minimal skin (replaced with static text)
- All status updates written as new lines (not in-place overwrites)
- ANSI escape sequences stripped when output is not a TTY

### Navigation Flow

```
hermes           → REPL (default, 4-zone layout)
hermes run       → REPL (same)
hermes gateway   → headless mode (structured log output, no TUI)
hermes setup     → interactive wizard (step-by-step prompts, no 4-zone)
hermes session   → list view (table) → detail view (conversation replay)
hermes skill     → list view (table) → detail view (skill content)
```

### Slash Commands

Shared registry between CLI and gateway:

```go
type SlashCommand struct {
    Name        string
    Aliases     []string
    Description string
    Handler     func(ctx context.Context, args string) error
}
```

Commands: /help, /exit, /clear, /model, /session, /skill, /config, /cost, /export, /compact

---

## Section 8: Gateway & Platform Adapters

### Gateway Architecture

```go
// gateway/gateway.go

type Gateway struct {
    config    *config.GatewayConfig
    engine    func(opts *agent.RunOptions) *agent.Engine
    storage   storage.Storage
    platforms map[string]Platform
    sessions  *SessionManager
    delivery  *DeliveryRouter
}
```

- Each platform runs in its own goroutine via `errgroup`
- Same-session messages serialized via `LiveSession.Mutex`
- Idle sessions evicted after 30 minutes

### Graceful Shutdown

```go
// cmd/hermes/gateway.go

func runGateway(cfg *config.Config) error {
    // Trap SIGTERM/SIGINT via signal.NotifyContext
    ctx, cancel := signal.NotifyContext(context.Background(), 
        syscall.SIGTERM, syscall.SIGINT)
    defer cancel()
    
    gw, err := gateway.New(cfg, store)
    if err != nil {
        return err
    }
    
    // Start gateway (blocks until ctx is cancelled)
    runErr := gw.Run(ctx)
    
    // Drain phase: stop accepting new messages, let in-flight complete
    drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer drainCancel()
    
    if err := gw.Drain(drainCtx); err != nil {
        log.Warn("drain timeout", "err", err)
    }
    
    // Close phase: platform connections, DB flush
    if err := gw.Close(); err != nil {
        log.Error("shutdown error", "err", err)
    }
    
    return runErr
}
```

**Concurrency limits (bounded worker pool):**

```go
type Gateway struct {
    // ...
    platformLimits map[string]*semaphore.Weighted  // per-platform concurrency caps
}

func (gw *Gateway) handleMessage(ctx context.Context, name string, p Platform, in *IncomingMessage) {
    sem := gw.platformLimits[name]
    if !sem.TryAcquire(1) {
        // Server busy: send throttled response to user
        gw.delivery.SendBusy(p, in)
        return
    }
    defer sem.Release(1)
    
    // ... rest of handling
}
```

- **Default concurrency:** 32 per platform (configurable via `gateway.platforms.<name>.max_concurrent`)
- **Overflow behavior:** Send "Server busy, please retry" response to user, don't block
- **Why bounded:** a spammed group could spawn 10,000 goroutines without limits, each with its own Engine + DB connection. OOM within seconds.

**Shutdown sequence:**
1. Receive SIGTERM/SIGINT → `ctx` is cancelled
2. Gateway stops dequeuing new messages from platform channels
3. In-flight conversations get 30s to complete (configurable via `agent.drain_timeout`)
4. Platform adapters close connections (send "bot going offline" messages where appropriate)
5. Database flushes pending writes (SQLite WAL checkpoint, PostgreSQL connection drain)
6. Process exits with code 0 (clean) or 1 (drain timeout)

**Why this matters:** Without graceful shutdown, deploying a new version kills in-flight conversations mid-response. Users on Telegram/Discord see no reply. With graceful shutdown, they see their current response complete, then the bot goes quiet until the new version starts.

### Platform Interface

```go
type Platform interface {
    Messages(ctx context.Context) <-chan *IncomingMessage
    Send(ctx context.Context, to string, msg *OutgoingMessage) error
    Name() string
    Close() error
}
```

### 21 Platform Adapters

telegram, discord, slack, whatsapp, signal, matrix, email, sms, dingtalk, feishu, wechat, mattermost, homeassistant, api, wecom, line, teams, rocketchat, irc, xmpp, webhook

### Platform Registry (init() pattern)

```go
// gateway/platform/registry.go

type Factory func(cfg map[string]any) (Platform, error)

var registry = map[string]Factory{}

func Register(name string, factory Factory) {
    registry[name] = factory
}

func New(name string, cfg map[string]any) (Platform, error) {
    f, ok := registry[name]
    if !ok {
        return nil, fmt.Errorf("unknown platform: %s", name)
    }
    return f(cfg)
}
```

Each platform package registers itself via `init()`:

```go
// gateway/platform/telegram/init.go
func init() { platform.Register("telegram", New) }

// gateway/platform/telegram/config.go — typed config, not map[string]any
type Config struct {
    Token       string `mapstructure:"token"`
    HomeChannel string `mapstructure:"home_channel"`
    ProxyURL    string `mapstructure:"proxy_url,omitempty"`
}

// gateway/platform/telegram/telegram.go
func New(cfg map[string]any) (platform.Platform, error) {
    var c Config
    if err := mapstructure.Decode(cfg, &c); err != nil {
        return nil, fmt.Errorf("telegram config: %w", err)
    }
    if c.Token == "" {
        return nil, errors.New("telegram: token is required")
    }
    return &TelegramAdapter{config: c}, nil
}
```

**Why typed per-platform configs matter:** The old `map[string]any` approach defers all validation to runtime. A typo in `bot_token` becomes a silent nil that surfaces only when the bot tries to connect. With typed configs, `mapstructure.Decode` fails fast with a clear error at startup.

### Delivery Router

- Auto-selects format per platform: HTML (Telegram), Markdown (Discord/Slack/Matrix), Plain (others)
- Long messages chunked by configurable chunk size (default 1500)

---

## Section 9: Skill System & Cron Scheduler

### Skill Loading

- Scans `~/.hermes/skills/` and `~/.hermes/optional-skills/`
- Parses YAML frontmatter + markdown body (same format as original)
- Keyword/regex trigger matching

### Skill Injection

- Injected as user messages (not system prompt) to preserve Anthropic prefix caching
- Format: `[Skill: name]\n<content>`

### Cron Scheduler

```go
type Job struct {
    ID       string   `yaml:"id"`
    Name     string   `yaml:"name"`
    Schedule string   `yaml:"schedule"`    // cron expression
    Prompt   string   `yaml:"prompt"`
    Model    string   `yaml:"model"`
    Delivery Delivery `yaml:"delivery"`    // platform + target
    Enabled  bool     `yaml:"enabled"`
}
```

- Minute-level ticker + cron expression evaluation
- Each job runs in independent goroutine
- Results delivered to any configured platform

### ACP Protocol Adapter

- HTTP server for IDE integration (VS Code, Zed, JetBrains)
- Endpoints: POST /v1/chat, GET /v1/tools, POST /v1/tools/{name}, GET /health

---

## Section 10: Memory System & Dependencies

### Memory Provider Interface

```go
type Provider interface {
    Save(ctx context.Context, entry *Entry) error
    Search(ctx context.Context, query string, opts *SearchOptions) ([]*Entry, error)
    Delete(ctx context.Context, id string) error
    Close() error
}
```

9 backends: honcho, mem0, builtin (SQLite FTS5), hindsight, holographic, byterover, retaindb, openviking, supermemory

### Dependencies

| Domain | Library | Rationale |
|--------|---------|-----------|
| CLI | `cobra` | Go CLI standard |
| TUI | `bubbletea` + `bubbles` + `lipgloss` | Charm ecosystem |
| Markdown | `glamour` | Terminal markdown rendering |
| HTTP | `net/http` (stdlib) | No external dependency needed |
| SSE | `bufio.Scanner` (stdlib) with 10MB buffer | SSE protocol is simple. **Critical:** must call `scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)` — default 64KB limit corrupts large LLM tool call streams |
| WebSocket | `nhooyr.io/websocket` | Pure Go, clean API |
| YAML | `gopkg.in/yaml.v3` | Standard choice |
| SQLite | `modernc.org/sqlite` | Pure Go, no CGo |
| PostgreSQL | `pgx/v5` + `pgxpool` | Fastest Go PG driver |
| Migrations | `golang-migrate/migrate` | SQLite + PG support |
| Cron | `robfig/cron/v3` | Mature and stable |
| Logging | `log/slog` (stdlib) | Go 1.21+ structured logging |
| Testing | `testing` + `testify` | Standard + assertion helpers |
| Concurrency | `errgroup` + `sync` (stdlib) | Standard library |
| Telegram | `go-telegram-bot-api/v5` | Mature |
| Discord | `bwmarrin/discordgo` | Most mature Discord Go lib |
| Slack | `slack-go/slack` | Officially recommended |
| MCP | `mark3labs/mcp-go` | Go MCP implementation |
| SSH | `golang.org/x/crypto/ssh` | Standard extension |
| Docker | `docker/docker/client` | Official SDK |

### Build

```makefile
build:
    go build -o bin/hermes ./cmd/hermes

release:
    GOOS=linux   GOARCH=amd64 go build -o bin/hermes-linux-amd64 ./cmd/hermes
    GOOS=darwin  GOARCH=arm64 go build -o bin/hermes-darwin-arm64 ./cmd/hermes
    GOOS=windows GOARCH=amd64 go build -o bin/hermes-windows-amd64.exe ./cmd/hermes
```

Module path: `github.com/nousresearch/hermes-agent`

---

## Resolved Design Decisions

1. **Gateway log format:** Structured JSON (one object per line). Machine-parseable for log aggregators (jq, Datadog, ELK). CLI mode uses slog human-readable format by default.

2. **Session view (`hermes session view <id>`):** Re-rendered conversation with full markdown rendering, tool call formatting, and the same visual treatment as live REPL. Not a plain text dump.

3. **Provider fallback visibility:** Visible switch with muted note. When fallback activates, show `⚠ Switched to <provider> (<reason>)` in muted color. Users need to know which provider is billing them.

---

## Test Strategy

### Coverage Target

- **100% of new code paths** must have tests. This is non-negotiable for a rewrite where tests prove the port preserves Python semantics.
- Quality bar: ★★★ (happy path + edge cases + error paths) for core modules; ★★ acceptable for boilerplate adapters.

### Test Types

**1. Unit tests (`*_test.go` alongside each file)**

- Pure logic: message serialization, budget arithmetic, compression rules, skin detection
- Run on every commit via `go test ./...`
- Target: sub-second per package, under 30 seconds total

**2. Race detector tests**

All concurrent-use code must have tests run under `-race`:
- `agent.Budget` (atomic counters)
- `tool.Registry` (mutex-protected map)
- `storage/sqlite.Store` (WAL concurrency)
- `gateway.Gateway` (platform goroutines)
- `gateway.SessionManager` (live session map)

CI command: `go test -race ./...`

**3. Port-fidelity tests (golden files)**

These are THE critical tests for this rewrite. They prove Go reads Python data correctly.

- **SQLite compatibility:** Bundle a recorded Python `state.db` under `testdata/python-state.db`. Load it via the Go SQLite store. Assert every session, message, and usage field matches expected values. If Python schema changes, update the golden file.
- **Skill format compatibility:** Copy all bundled Python skills under `testdata/skills/`. Load them via the Go skill loader. Assert each `Skill` struct matches the expected shape.
- **Config compatibility:** Bundle example Python `config.yaml` files under `testdata/configs/`. Load each via Go config loader. Assert all fields populate without error.

**4. Integration tests (`//go:build integration` build tag)**

Behind a build tag so they don't run on every commit:

- Provider integration: real API calls (or recorded cassettes via `github.com/dnaeon/go-vcr`)
- Terminal backends: real Docker, SSH localhost, Modal, Daytona
- Storage PostgreSQL: via `testcontainers-go`
- Platform adapters: real bot tokens (optional, per-platform env vars)

CI command: `go test -tags=integration ./...` (runs nightly or on release)

**5. Benchmarks**

Critical paths only:
- `Engine.RunConversation` with mocked provider — measure loop overhead
- `tool.Registry.Dispatch` — measure lookup + dispatch cost
- `storage/sqlite.AddMessage` — measure write throughput
- Streaming collector — measure per-token overhead

Stored in `testdata/benchmarks/baseline.txt`, checked on each release to catch regressions.

### Testing Tools

| Tool | Purpose |
|------|---------|
| `testing` (stdlib) | Test framework |
| `testify/assert` + `testify/require` | Readable assertions |
| `testify/mock` | Interface mocks (Provider, Storage, Platform) |
| `github.com/dnaeon/go-vcr` | HTTP cassette recording for provider integration tests |
| `github.com/testcontainers/testcontainers-go` | Real PostgreSQL for integration tests |
| `-race` flag | Data race detection |
| `go test -cover` | Coverage reports |
| `go-cmp` (`github.com/google/go-cmp`) | Deep struct comparison for golden file tests |

### CI Pipeline

```yaml
# .github/workflows/test.yml (referenced in Distribution section)
jobs:
  unit:
    - go test -race -cover ./...
  lint:
    - golangci-lint run
  integration:
    - go test -tags=integration ./...
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
```

---

## Distribution & Release

### GitHub Actions Workflows

**`.github/workflows/test.yml`** — runs on every push and PR:
- Go setup (1.24)
- `go build ./...`
- `go test -race -cover ./...`
- `golangci-lint run`

**`.github/workflows/release.yml`** — runs on version tag push (v*):
- Uses `goreleaser` to build all cross-platform binaries
- Uploads binaries to GitHub Releases
- Builds and pushes Docker image to `ghcr.io/nousresearch/hermes-agent`
- Generates checksums and SBOM

### goreleaser Config

```yaml
# .goreleaser.yml
project_name: hermes
builds:
  - main: ./cmd/hermes
    binary: hermes
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: 'checksums.txt'

dockers:
  - image_templates:
      - 'ghcr.io/nousresearch/hermes-agent:{{ .Version }}'
      - 'ghcr.io/nousresearch/hermes-agent:latest'
    dockerfile: Dockerfile
```

### Installation Methods

| Method | Command |
|--------|---------|
| Go install | `go install github.com/nousresearch/hermes-agent/cmd/hermes@latest` |
| Binary download | `curl -L <release-url> \| tar xz` from GitHub Releases |
| Docker | `docker run ghcr.io/nousresearch/hermes-agent:latest` |
| Homebrew (future) | `brew install nousresearch/tap/hermes` (custom tap) |

### Version Management

- Semantic versioning: v0.1.0, v0.2.0, v1.0.0
- `cmd/hermes.Version` injected at build time via `-ldflags "-X main.Version=$VERSION"`
- `hermes version` prints version, git commit, build date

---

## Implementation Priority

1. **Agent Engine** — message types, provider interface, conversation loop, budget, compression
2. **CLI** — cobra subcommands, bubbletea REPL, markdown rendering
3. **Tools** — registry, terminal (all 6 backends), file, web, MCP bridge
4. **Gateway** — platform interface, session management, delivery, all 21 adapters

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | — | — |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | CLEAN (PLAN) | 15 issues, 0 critical gaps |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | CLEAN (FULL) | score: 4/10 → 9/10, 3 decisions |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

- **UNRESOLVED:** 0
- **VERDICT:** ENG + DESIGN CLEARED — ready to implement

