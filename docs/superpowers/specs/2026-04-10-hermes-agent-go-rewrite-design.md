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
    Content         any             `json:"content"`          // string or []ContentBlock
    ToolCalls       []ToolCall      `json:"tool_calls,omitempty"`
    ToolCallID      string          `json:"tool_call_id,omitempty"`
    ToolName        string          `json:"tool_name,omitempty"`
    Reasoning       string          `json:"reasoning,omitempty"`
    FinishReason    string          `json:"finish_reason,omitempty"`
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
    Available() bool
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
    TokenEstimator   func(text string) int
}
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

type FallbackChain struct {
    providers []Provider
    current   int
}

func (fc *FallbackChain) Complete(ctx context.Context, req *Request) (*Response, error) {
    for i := fc.current; i < len(fc.providers); i++ {
        resp, err := fc.providers[i].Complete(ctx, req)
        if err == nil {
            fc.current = 0 // Restore primary on next round
            return resp, nil
        }
        if !isRetryable(err) {
            return nil, err
        }
    }
    return nil, ErrAllProvidersFailed
}
```

---

## Section 3: Agent Engine

### Core Structure

```go
// agent/engine.go

type Engine struct {
    provider    provider.Provider
    tools       *tool.Registry
    storage     storage.Storage
    skillLoader *skill.Loader
    prompt      *PromptBuilder
    config      *config.AgentConfig
    
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
    clients  map[string]*Client
}

// Connects to MCP server, pulls tool list, registers into Registry
// Namespace: "serverName__toolName" to avoid conflicts
func (b *Bridge) Connect(serverName string, transport Transport) error
```

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
    
    // Lifecycle
    Close() error
    Migrate() error
}
```

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
| SSE | `bufio.Scanner` (stdlib) | SSE protocol is simple |
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

## Implementation Priority

1. **Agent Engine** — message types, provider interface, conversation loop, budget, compression
2. **CLI** — cobra subcommands, bubbletea REPL, markdown rendering
3. **Tools** — registry, terminal (all 6 backends), file, web, MCP bridge
4. **Gateway** — platform interface, session management, delivery, all 21 adapters
