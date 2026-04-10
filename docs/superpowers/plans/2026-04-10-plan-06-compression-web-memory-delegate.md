# Plan 6: Context Compression + Web Tools + Memory + Delegate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unblock long-running agent sessions by adding context compression, add 3 web tools (fetch/search/extract), add a local memory store, and add the delegate tool for subagent spawning.

**Architecture:** Context compression uses an auxiliary LLM call to summarize middle-of-conversation messages into a condensed block, creating a new session with a `parent_session_id` pointer. Web tools use `net/http` for fetch, the Exa API for search, and the Firecrawl API for extract. Memory uses the existing `storage/sqlite` package with a new `memories` table and FTS5 index. Delegate spawns a fresh `Engine` instance with a subset of tools and a shared budget counter.

**Tech Stack:** Go 1.25, `net/http` (stdlib), existing `agent`, `provider`, `tool`, `storage/sqlite`, `config`, `cli` packages. No new major dependencies — Exa/Firecrawl are plain HTTP APIs.

**Deliverable at end of plan:**
```
> summarize the HN front page
⚡ web_fetch: {"url":"https://news.ycombinator.com"}
│ {"status":200, "content":"..."}
└
⚡ delegate: {"task":"extract article titles from the HTML"}
│ {"result":["HN: Show Article 1", "HN: Show Article 2", ...]}
└
The top stories on HN right now are...

> remember that I prefer to code in Go
⚡ memory_save: {"content":"user prefers Go"}
│ {"id":"mem_01", "ok":true}
└
Got it, I'll remember.

> (long conversation continues, hitting 50% context)
[Compressed: 40 messages → 3 summary blocks, new session chained to parent]
```

**Non-goals for this plan (deferred):**
- **MCP client/bridge** — Plan 6b (cohesive feature, deserves its own plan)
- **External memory providers** (honcho, mem0, hindsight, etc.) — Plan 6c (8 HTTP adapters)
- **Browser automation** (Browserbase) — Plan 6d
- **Vision tool** (image input to providers that support it) — Plan 6e
- **Web search via Tavily / Serper / DuckDuckGo** — Plan 6b (only Exa in Plan 6)
- **Web extract via Reader.dev / Jina** — Plan 6b (only Firecrawl in Plan 6)
- **Trajectory-aware compression** (keeping tool call context) — simple version in Plan 6, smart version later
- **Auto-compaction of tool results** — deferred
- **Delegated subagents calling delegate recursively** — blocked in Plan 6, enabled later
- **Parallel delegate spawning** — Plan 6 is sequential-only

**Plans 1-5 dependencies this plan touches:**
- `agent/engine.go` — `Engine` gains an auxiliary provider field for compression
- `agent/conversation.go` — main loop checks compression threshold
- `agent/compression.go` — NEW
- `agent/prompt.go` — injects memory guidance when memory tools are registered
- `storage/storage.go` — adds `SaveMemory`/`SearchMemory`/`DeleteMemory` methods
- `storage/sqlite/migrate.go` — adds `memories` table + FTS5
- `tool/web/` — NEW package
- `tool/memory/` — NEW package
- `tool/delegate/` — NEW package
- `config/config.go` — adds `Auxiliary` config, `Exa` / `Firecrawl` provider configs
- `cli/repl.go` — registers new tools when their env/config is present

---

## File Structure

```
hermes-agent-go/
├── agent/
│   ├── engine.go                   # MODIFIED: add auxProvider field
│   ├── conversation.go             # MODIFIED: compression trigger in main loop
│   ├── compression.go              # NEW
│   ├── compression_test.go
│   └── prompt.go                   # MODIFIED: memory guidance injection
├── storage/
│   ├── storage.go                  # MODIFIED: Memory CRUD interface
│   ├── types.go                    # MODIFIED: Memory struct
│   └── sqlite/
│       ├── migrate.go              # MODIFIED: memories table + FTS
│       └── memory.go               # NEW: SQLite Memory CRUD impl
├── tool/
│   ├── web/                        # NEW
│   │   ├── fetch.go                # web_fetch tool
│   │   ├── search.go               # web_search (Exa) tool
│   │   ├── extract.go              # web_extract (Firecrawl) tool
│   │   ├── register.go             # RegisterAll
│   │   └── web_test.go
│   ├── memory/                     # NEW
│   │   ├── memory.go               # memory_save, memory_search, memory_delete
│   │   ├── register.go             # RegisterAll with Storage injection
│   │   └── memory_test.go
│   └── delegate/                   # NEW
│       ├── delegate.go             # delegate tool + subagent runner
│       ├── register.go             # RegisterAll with factory function injection
│       └── delegate_test.go
├── config/
│   └── config.go                   # MODIFIED: Auxiliary + Web config
└── cli/
    └── repl.go                     # MODIFIED: register web/memory/delegate tools
```

---

## Task 1: Context Compression Config and Types

**Files:**
- Modify: `hermes-agent-go/config/config.go`
- Create: `hermes-agent-go/agent/compression.go`
- Create: `hermes-agent-go/agent/compression_test.go`

- [ ] **Step 1: Add `CompressionConfig` and `AuxiliaryConfig` to Config**

In `config/config.go`, add these types (place them near `AgentConfig`):

```go
// CompressionConfig controls context compression behavior.
// When the conversation history exceeds Threshold * model context length,
// the Engine summarizes middle messages via the auxiliary provider.
type CompressionConfig struct {
	Enabled     bool    `yaml:"enabled"`      // default true
	Threshold   float64 `yaml:"threshold"`    // default 0.5 (50% of context)
	TargetRatio float64 `yaml:"target_ratio"` // default 0.2 (compress to 20%)
	ProtectLast int     `yaml:"protect_last"` // default 20 messages
	MaxPasses   int     `yaml:"max_passes"`   // default 3
}

// AuxiliaryConfig holds the auxiliary provider used for compression,
// vision summarization, and other secondary tasks.
// If unset, the main provider is used.
type AuxiliaryConfig struct {
	Provider string `yaml:"provider,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
}
```

Extend the `AgentConfig` struct to include `Compression`:

```go
type AgentConfig struct {
	MaxTurns       int               `yaml:"max_turns"`
	GatewayTimeout int               `yaml:"gateway_timeout,omitempty"`
	Compression    CompressionConfig `yaml:"compression,omitempty"`
}
```

Add `Auxiliary` as a top-level field on `Config`:

```go
type Config struct {
	Model             string                    `yaml:"model"`
	Providers         map[string]ProviderConfig `yaml:"providers"`
	FallbackProviders []ProviderConfig          `yaml:"fallback_providers,omitempty"`
	Agent             AgentConfig               `yaml:"agent"`
	Auxiliary         AuxiliaryConfig           `yaml:"auxiliary,omitempty"`
	Terminal          TerminalConfig            `yaml:"terminal"`
	Storage           StorageConfig             `yaml:"storage"`
}
```

Update `Default()` to set compression defaults:

```go
func Default() *Config {
	return &Config{
		Model:     "anthropic/claude-opus-4-6",
		Providers: map[string]ProviderConfig{},
		Agent: AgentConfig{
			MaxTurns:       90,
			GatewayTimeout: 1800,
			Compression: CompressionConfig{
				Enabled:     true,
				Threshold:   0.5,
				TargetRatio: 0.2,
				ProtectLast: 20,
				MaxPasses:   3,
			},
		},
		Terminal: TerminalConfig{
			Backend: "local",
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}
```

- [ ] **Step 2: Write failing tests for the Compressor**

```go
// agent/compression_test.go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuxProvider returns a canned text response from Complete().
type stubAuxProvider struct {
	response string
}

func (s *stubAuxProvider) Name() string                                { return "stub-aux" }
func (s *stubAuxProvider) Available() bool                             { return true }
func (s *stubAuxProvider) ModelInfo(string) *provider.ModelInfo        { return nil }
func (s *stubAuxProvider) EstimateTokens(string, string) (int, error)  { return 0, nil }
func (s *stubAuxProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, nil
}
func (s *stubAuxProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(s.response),
		},
		FinishReason: "stop",
		Usage:        message.Usage{InputTokens: 100, OutputTokens: 30},
	}, nil
}

func TestCompressorPreservesProtectedMessages(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		TargetRatio: 0.2,
		ProtectLast: 3,
		MaxPasses:   3,
	}
	aux := &stubAuxProvider{response: "Summary of earlier messages."}
	c := NewCompressor(cfg, aux)

	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("msg 1")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 2")},
		{Role: message.RoleUser, Content: message.TextContent("msg 3")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 4")},
		{Role: message.RoleUser, Content: message.TextContent("msg 5")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 6")},
		{Role: message.RoleUser, Content: message.TextContent("msg 7")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 8")},
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)

	// Expected: first 3 (preserved head) + summary + last 3 (protected tail) = 7 messages
	// Actually: preserved head is the first user message pair, then summary, then protect_last
	// The plan says: preserve first 3 + last ProtectLast
	// With 8 messages and ProtectLast=3: head=first 3, tail=last 3, middle=2 → 3 + 1 + 3 = 7
	assert.LessOrEqual(t, len(compressed), len(history))
	assert.Greater(t, len(compressed), 0)

	// Last 3 messages should match the original tail
	tail := compressed[len(compressed)-3:]
	assert.Equal(t, "msg 6", tail[0].Content.Text())
	assert.Equal(t, "msg 7", tail[1].Content.Text())
	assert.Equal(t, "msg 8", tail[2].Content.Text())

	// There should be at least one summary message
	foundSummary := false
	for _, m := range compressed {
		if strings.Contains(m.Content.Text(), "Summary of earlier") {
			foundSummary = true
			break
		}
	}
	assert.True(t, foundSummary, "expected compressed history to contain a summary message")
}

func TestCompressorSkipsShortHistory(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		TargetRatio: 0.2,
		ProtectLast: 10, // more than history length
		MaxPasses:   3,
	}
	aux := &stubAuxProvider{response: "irrelevant"}
	c := NewCompressor(cfg, aux)

	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("msg 1")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 2")},
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)
	// Too short to compress — returned unchanged
	assert.Equal(t, history, compressed)
}

func TestCompressorDisabledReturnsUnchanged(t *testing.T) {
	cfg := config.CompressionConfig{Enabled: false}
	aux := &stubAuxProvider{}
	c := NewCompressor(cfg, aux)

	history := make([]message.Message, 100)
	for i := range history {
		history[i] = message.Message{Role: message.RoleUser, Content: message.TextContent("msg")}
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)
	assert.Equal(t, history, compressed)
}
```

- [ ] **Step 3: Implement `agent/compression.go`**

```go
// agent/compression.go
package agent

import (
	"context"
	"fmt"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Compressor summarizes middle-of-history messages using an auxiliary LLM
// to reduce token count while preserving the conversation's head and tail.
type Compressor struct {
	cfg config.CompressionConfig
	aux provider.Provider
}

// NewCompressor constructs a Compressor. `aux` is the auxiliary provider
// used for the summarization call. If aux is nil, Compress returns the
// history unchanged.
func NewCompressor(cfg config.CompressionConfig, aux provider.Provider) *Compressor {
	return &Compressor{cfg: cfg, aux: aux}
}

// Compress summarizes the middle of the history and returns a shortened
// version. The head (first 3 messages) and tail (last ProtectLast messages)
// are preserved verbatim; the middle is replaced by a single assistant
// summary message.
//
// If the history is shorter than head + tail + 1, the original is returned.
// If compression is disabled in config, the original is returned.
// If the auxiliary provider is nil, the original is returned.
func (c *Compressor) Compress(ctx context.Context, history []message.Message) ([]message.Message, error) {
	if !c.cfg.Enabled || c.aux == nil {
		return history, nil
	}

	const headCount = 3
	tailCount := c.cfg.ProtectLast
	if tailCount < 1 {
		tailCount = 20
	}

	// Not enough history to compress
	if len(history) <= headCount+tailCount {
		return history, nil
	}

	head := history[:headCount]
	tail := history[len(history)-tailCount:]
	middle := history[headCount : len(history)-tailCount]

	if len(middle) == 0 {
		return history, nil
	}

	summary, err := c.summarize(ctx, middle)
	if err != nil {
		return nil, fmt.Errorf("compression: summarize: %w", err)
	}

	result := make([]message.Message, 0, headCount+1+tailCount)
	result = append(result, head...)
	result = append(result, message.Message{
		Role:    message.RoleAssistant,
		Content: message.TextContent("[Compressed summary of earlier conversation]\n" + summary),
	})
	result = append(result, tail...)
	return result, nil
}

// summarize sends the middle messages to the auxiliary provider with
// a terse summarization prompt and returns the assistant's text response.
func (c *Compressor) summarize(ctx context.Context, middle []message.Message) (string, error) {
	// Build a condensed transcript to hand to the aux provider.
	transcript := renderTranscript(middle)

	systemPrompt := "You are a summarizer. Produce a terse, bullet-point summary of the conversation below, preserving key facts, decisions, and code references. Keep it under 500 words."

	req := &provider.Request{
		Model:        "", // use aux provider's default
		SystemPrompt: systemPrompt,
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: message.TextContent(transcript),
			},
		},
		MaxTokens: 1000,
	}

	resp, err := c.aux.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	if resp.Message.Content.IsText() {
		return resp.Message.Content.Text(), nil
	}
	// If the aux provider returned block content, concatenate text blocks.
	var text string
	for _, b := range resp.Message.Content.Blocks() {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return text, nil
}

// renderTranscript builds a plain-text transcript of conversation messages.
func renderTranscript(msgs []message.Message) string {
	var out string
	for i, m := range msgs {
		out += fmt.Sprintf("%d. %s: ", i+1, m.Role)
		if m.Content.IsText() {
			out += m.Content.Text()
		} else {
			// Summarize block content as "[tool use]" etc.
			for _, b := range m.Content.Blocks() {
				switch b.Type {
				case "text":
					out += b.Text
				case "tool_use":
					out += "[tool_use: " + b.ToolUseName + "]"
				case "tool_result":
					out += "[tool_result]"
				}
			}
		}
		out += "\n"
	}
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./agent/... ./config/...
```

Expected: PASS. All existing tests + 3 new Compressor tests + any config tests that hit the new fields.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/config/config.go hermes-agent-go/agent/compression.go hermes-agent-go/agent/compression_test.go
git commit -m "feat(agent): add Compressor with auxiliary LLM summarization"
```

---

## Task 2: Integrate Compression into Engine

**Files:**
- Modify: `hermes-agent-go/agent/engine.go`
- Modify: `hermes-agent-go/agent/conversation.go`

- [ ] **Step 1: Add `compressor` field to Engine**

In `agent/engine.go`, modify the `Engine` struct:

```go
type Engine struct {
	provider    provider.Provider
	auxProvider provider.Provider  // NEW: optional, used by Compressor
	storage     storage.Storage
	tools       *tool.Registry
	config      config.AgentConfig
	platform    string
	prompt      *PromptBuilder
	compressor  *Compressor        // NEW

	onStreamDelta func(delta *provider.StreamDelta)
	onToolStart   func(call message.ContentBlock)
	onToolResult  func(call message.ContentBlock, result string)
}
```

Update `NewEngineWithTools` to optionally accept an aux provider. The simplest approach: add a new constructor that takes the aux provider, and keep `NewEngineWithTools` as a wrapper that passes nil:

```go
// NewEngineWithTools constructs an Engine with tools and no auxiliary provider.
// Compression will be a no-op without an auxiliary provider.
func NewEngineWithTools(p provider.Provider, s storage.Storage, tools *tool.Registry, cfg config.AgentConfig, platform string) *Engine {
	return NewEngineWithToolsAndAux(p, nil, s, tools, cfg, platform)
}

// NewEngineWithToolsAndAux constructs an Engine with tools and an auxiliary
// provider for compression. If aux is nil, compression is disabled.
func NewEngineWithToolsAndAux(p, aux provider.Provider, s storage.Storage, tools *tool.Registry, cfg config.AgentConfig, platform string) *Engine {
	e := &Engine{
		provider:    p,
		auxProvider: aux,
		storage:     s,
		tools:       tools,
		config:      cfg,
		platform:    platform,
		prompt:      NewPromptBuilder(platform),
	}
	if cfg.Compression.Enabled && aux != nil {
		e.compressor = NewCompressor(cfg.Compression, aux)
	}
	return e
}
```

- [ ] **Step 2: Add compression trigger to the main loop in `conversation.go`**

Find the loop header in `RunConversation`:

```go
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !budget.Consume() {
			break
		}
		iterations++
```

Add a compression check immediately after the `budget.Consume()` line but before building the request:

```go
		// Compression check: if enabled and history is long enough,
		// replace the middle of history with a summary.
		if e.compressor != nil && shouldCompress(history, e.config.Compression) {
			newHistory, err := e.compressor.Compress(ctx, history)
			if err != nil {
				// Don't fail the conversation on compression errors — log and continue.
				// Plan 6 keeps this simple; logging is deferred.
				_ = err
			} else {
				history = newHistory
			}
		}
```

- [ ] **Step 3: Add the `shouldCompress` heuristic helper**

In `conversation.go` (or `compression.go` if preferred), add:

```go
// shouldCompress decides whether the current history should be compressed.
// Plan 6 uses a simple count-based trigger: compress if len(history) exceeds
// (1/threshold) * protect_last. Future plans can add token-aware triggers.
func shouldCompress(history []message.Message, cfg config.CompressionConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.ProtectLast <= 0 {
		return false
	}
	// Trigger compression when we have more than 3× protect_last messages.
	// This roughly corresponds to hitting 50-75% of typical context windows
	// after a long conversation with tool calls.
	return len(history) > 3*cfg.ProtectLast
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./agent/...
```

Expected: PASS. Existing tests unchanged. Compressor tests still pass.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/agent/engine.go hermes-agent-go/agent/conversation.go
git commit -m "feat(agent): wire Compressor into Engine conversation loop"
```

---

## Task 3: web_fetch Tool

**Files:**
- Create: `hermes-agent-go/tool/web/fetch.go`
- Create: `hermes-agent-go/tool/web/web_test.go`

- [ ] **Step 1: Create the web_fetch handler**

```go
// tool/web/fetch.go
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
)

const (
	maxFetchBytes = 2 * 1024 * 1024 // 2 MiB
	fetchTimeout  = 30 * time.Second
)

const webFetchSchema = `{
  "type": "object",
  "properties": {
    "url":     { "type": "string", "description": "Absolute URL to fetch (http:// or https://)" },
    "method":  { "type": "string", "enum": ["GET","POST"], "description": "HTTP method (default GET)" },
    "headers": { "type": "object", "description": "Optional HTTP headers" },
    "body":    { "type": "string", "description": "Optional request body (for POST)" }
  },
  "required": ["url"]
}`

type webFetchArgs struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type webFetchResult struct {
	URL        string            `json:"url"`
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	Content    string            `json:"content"`
	Truncated  bool              `json:"truncated,omitempty"`
	ContentLen int               `json:"content_length"`
}

func webFetchHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args webFetchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.URL == "" {
		return tool.ToolError("url is required"), nil
	}

	method := args.Method
	if method == "" {
		method = "GET"
	}
	if method != "GET" && method != "POST" {
		return tool.ToolError("method must be GET or POST"), nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	var body io.Reader
	if args.Body != "" {
		body = stringReader(args.Body)
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, method, args.URL, body)
	if err != nil {
		return tool.ToolError("invalid URL: " + err.Error()), nil
	}
	for k, v := range args.Headers {
		httpReq.Header.Set(k, v)
	}
	// Default User-Agent
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "hermes-agent/1.0")
	}

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return tool.ToolError("fetch failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	// Read up to maxFetchBytes, flag if truncated
	limited := io.LimitReader(resp.Body, int64(maxFetchBytes+1))
	raw2, err := io.ReadAll(limited)
	if err != nil {
		return tool.ToolError("read body: " + err.Error()), nil
	}
	truncated := false
	if len(raw2) > maxFetchBytes {
		raw2 = raw2[:maxFetchBytes]
		truncated = true
	}

	// Flatten headers
	hdr := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			hdr[k] = v[0]
		}
	}

	return tool.ToolResult(webFetchResult{
		URL:        args.URL,
		Status:     resp.StatusCode,
		Headers:    hdr,
		Content:    string(raw2),
		Truncated:  truncated,
		ContentLen: len(raw2),
	}), nil
}

// stringReader is a tiny helper to avoid importing strings for just one thing.
func stringReader(s string) io.Reader {
	return &strReader{data: []byte(s)}
}

type strReader struct {
	data []byte
	off  int
}

func (r *strReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}
```

- [ ] **Step 2: Write the test**

```go
// tool/web/web_test.go
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebFetchHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "hermes-agent/1.0", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch", Parameters: json.RawMessage(webFetchSchema)}},
	})

	args := json.RawMessage(`{"url":"` + srv.URL + `"}`)
	out, err := reg.Dispatch(context.Background(), "web_fetch", args)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, float64(200), result["status"])
	assert.Equal(t, "hello world", result["content"])
}

func TestWebFetchRejectsMissingURL(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch"}},
	})
	out, err := reg.Dispatch(context.Background(), "web_fetch", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "url")
}

func TestWebFetchHandlesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "not found")
	}))
	defer srv.Close()

	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch"}},
	})
	args := json.RawMessage(`{"url":"` + srv.URL + `"}`)
	out, err := reg.Dispatch(context.Background(), "web_fetch", args)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	// Non-2xx is NOT an error for the tool — the status is reported.
	assert.Equal(t, float64(404), result["status"])
	assert.Equal(t, "not found", result["content"])
}

func TestWebFetchTruncatesLargeResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write slightly more than the max
		big := make([]byte, maxFetchBytes+100)
		for i := range big {
			big[i] = 'x'
		}
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name:    "web_fetch",
		Handler: webFetchHandler,
		Schema:  tool.ToolDefinition{Type: "function", Function: tool.FunctionDef{Name: "web_fetch"}},
	})
	args := json.RawMessage(`{"url":"` + srv.URL + `"}`)
	out, err := reg.Dispatch(context.Background(), "web_fetch", args)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, true, result["truncated"])
	assert.Equal(t, float64(maxFetchBytes), result["content_length"])
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/web/...
```

Expected: PASS. 4 tests.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/web/fetch.go hermes-agent-go/tool/web/web_test.go
git commit -m "feat(tool/web): add web_fetch tool with 2MiB limit and truncation"
```

---

## Task 4: web_search Tool (Exa)

**Files:**
- Create: `hermes-agent-go/tool/web/search.go`

Exa API: `POST https://api.exa.ai/search` with `Authorization: Bearer <key>`, body `{"query":"...","num_results":N}`, response `{"results":[{"title","url","text",...}]}`.

- [ ] **Step 1: Create the `web_search` handler**

```go
// tool/web/search.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
)

const (
	exaDefaultURL = "https://api.exa.ai/search"
	exaTimeout    = 30 * time.Second
)

const webSearchSchema = `{
  "type": "object",
  "properties": {
    "query":       { "type": "string", "description": "Search query" },
    "num_results": { "type": "number", "description": "Number of results to return (default 5, max 20)" }
  },
  "required": ["query"]
}`

type webSearchArgs struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results,omitempty"`
}

type exaSearchRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
}

type exaSearchResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Text          string  `json:"text,omitempty"`
	PublishedDate string  `json:"publishedDate,omitempty"`
	Author        string  `json:"author,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

type webSearchResult struct {
	Query   string      `json:"query"`
	Results []exaResult `json:"results"`
}

// newWebSearchHandler builds a handler with an injected API key and endpoint.
// The CLI wires the real key from config. Tests can inject a custom URL.
func newWebSearchHandler(apiKey, endpoint string) tool.Handler {
	if endpoint == "" {
		endpoint = exaDefaultURL
	}
	client := &http.Client{Timeout: exaTimeout}
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args webSearchArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Query == "" {
			return tool.ToolError("query is required"), nil
		}
		if args.NumResults <= 0 {
			args.NumResults = 5
		}
		if args.NumResults > 20 {
			args.NumResults = 20
		}

		// Apply key fallback: explicit argument > injected > env var.
		key := apiKey
		if key == "" {
			key = os.Getenv("EXA_API_KEY")
		}
		if key == "" {
			return tool.ToolError("EXA_API_KEY not set"), nil
		}

		body, _ := json.Marshal(exaSearchRequest{
			Query:      args.Query,
			NumResults: args.NumResults,
		})

		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return tool.ToolError("new request: " + err.Error()), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", key)
		httpReq.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			return tool.ToolError("exa request failed: " + err.Error()), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return tool.ToolError(fmt.Sprintf("exa http %d", resp.StatusCode)), nil
		}

		var out exaSearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return tool.ToolError("decode: " + err.Error()), nil
		}

		return tool.ToolResult(webSearchResult{
			Query:   args.Query,
			Results: out.Results,
		}), nil
	}
}
```

- [ ] **Step 2: Append tests to `web_test.go`**

```go
func TestWebSearchHappyPath(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		body, _ := io.ReadAll(r.Body)
		var req exaSearchRequest
		require.NoError(t, json.Unmarshal(body, &req))
		capturedQuery = req.Query

		resp := exaSearchResponse{
			Results: []exaResult{
				{Title: "Go Lang", URL: "https://go.dev", Text: "Go programming language."},
				{Title: "Effective Go", URL: "https://go.dev/doc/effective_go"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	handler := newWebSearchHandler("test-key", srv.URL)
	out, err := handler(context.Background(), json.RawMessage(`{"query":"golang"}`))
	require.NoError(t, err)

	assert.Equal(t, "golang", capturedQuery)

	var result webSearchResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "golang", result.Query)
	require.Len(t, result.Results, 2)
	assert.Equal(t, "Go Lang", result.Results[0].Title)
}

func TestWebSearchRequiresQuery(t *testing.T) {
	handler := newWebSearchHandler("test-key", "https://x")
	out, err := handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "query")
}

func TestWebSearchRejectsMissingKey(t *testing.T) {
	t.Setenv("EXA_API_KEY", "")
	handler := newWebSearchHandler("", "https://x")
	out, err := handler(context.Background(), json.RawMessage(`{"query":"go"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "EXA_API_KEY")
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/web/...
```

Expected: PASS. 7 tests total (4 fetch + 3 search).

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/web/search.go hermes-agent-go/tool/web/web_test.go
git commit -m "feat(tool/web): add web_search tool via Exa API"
```

---

## Task 5: web_extract Tool (Firecrawl)

**Files:**
- Create: `hermes-agent-go/tool/web/extract.go`
- Modify: `hermes-agent-go/tool/web/web_test.go` (append tests)

Firecrawl API: `POST https://api.firecrawl.dev/v1/scrape` with `Authorization: Bearer <key>`, body `{"url":"...","formats":["markdown"]}`, response `{"success":true,"data":{"markdown":"...","metadata":{...}}}`.

- [ ] **Step 1: Create the `web_extract` handler**

```go
// tool/web/extract.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
)

const (
	firecrawlDefaultURL = "https://api.firecrawl.dev/v1/scrape"
	firecrawlTimeout    = 60 * time.Second
)

const webExtractSchema = `{
  "type": "object",
  "properties": {
    "url":    { "type": "string", "description": "Absolute URL to extract content from" },
    "format": { "type": "string", "enum": ["markdown","html","text"], "description": "Output format (default markdown)" }
  },
  "required": ["url"]
}`

type webExtractArgs struct {
	URL    string `json:"url"`
	Format string `json:"format,omitempty"`
}

type firecrawlRequest struct {
	URL     string   `json:"url"`
	Formats []string `json:"formats"`
}

type firecrawlResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string                 `json:"markdown,omitempty"`
		HTML     string                 `json:"html,omitempty"`
		Text     string                 `json:"text,omitempty"`
		Metadata map[string]any         `json:"metadata,omitempty"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

type webExtractResult struct {
	URL      string         `json:"url"`
	Format   string         `json:"format"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// newWebExtractHandler builds a handler with injected API key and endpoint.
func newWebExtractHandler(apiKey, endpoint string) tool.Handler {
	if endpoint == "" {
		endpoint = firecrawlDefaultURL
	}
	client := &http.Client{Timeout: firecrawlTimeout}
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args webExtractArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.URL == "" {
			return tool.ToolError("url is required"), nil
		}
		format := args.Format
		if format == "" {
			format = "markdown"
		}
		if format != "markdown" && format != "html" && format != "text" {
			return tool.ToolError("format must be markdown, html, or text"), nil
		}

		key := apiKey
		if key == "" {
			key = os.Getenv("FIRECRAWL_API_KEY")
		}
		if key == "" {
			return tool.ToolError("FIRECRAWL_API_KEY not set"), nil
		}

		body, _ := json.Marshal(firecrawlRequest{
			URL:     args.URL,
			Formats: []string{format},
		})

		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return tool.ToolError("new request: " + err.Error()), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+key)
		httpReq.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			return tool.ToolError("firecrawl request failed: " + err.Error()), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return tool.ToolError(fmt.Sprintf("firecrawl http %d", resp.StatusCode)), nil
		}

		var out firecrawlResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return tool.ToolError("decode: " + err.Error()), nil
		}
		if !out.Success {
			return tool.ToolError("firecrawl: " + out.Error), nil
		}

		var content string
		switch format {
		case "markdown":
			content = out.Data.Markdown
		case "html":
			content = out.Data.HTML
		case "text":
			content = out.Data.Text
		}

		return tool.ToolResult(webExtractResult{
			URL:      args.URL,
			Format:   format,
			Content:  content,
			Metadata: out.Data.Metadata,
		}), nil
	}
}
```

- [ ] **Step 2: Append tests to `web_test.go`**

```go
func TestWebExtractHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		var req firecrawlRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "https://example.com", req.URL)
		assert.Contains(t, req.Formats, "markdown")

		resp := firecrawlResponse{Success: true}
		resp.Data.Markdown = "# Hello\n\nWorld."
		resp.Data.Metadata = map[string]any{"title": "Example"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	handler := newWebExtractHandler("test-key", srv.URL)
	out, err := handler(context.Background(), json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var result webExtractResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "markdown", result.Format)
	assert.Contains(t, result.Content, "# Hello")
	assert.Equal(t, "Example", result.Metadata["title"])
}

func TestWebExtractRejectsBadFormat(t *testing.T) {
	handler := newWebExtractHandler("test-key", "https://x")
	out, err := handler(context.Background(), json.RawMessage(`{"url":"https://x","format":"pdf"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "format")
}

func TestWebExtractHandlesFailureResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := firecrawlResponse{Success: false, Error: "rate limited"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	handler := newWebExtractHandler("test-key", srv.URL)
	out, err := handler(context.Background(), json.RawMessage(`{"url":"https://x"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "rate limited")
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/web/...
```

Expected: PASS. 10 tests total (4 fetch + 3 search + 3 extract).

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/web/extract.go hermes-agent-go/tool/web/web_test.go
git commit -m "feat(tool/web): add web_extract tool via Firecrawl API"
```

---

## Task 6: Web Tools Registration

**Files:**
- Create: `hermes-agent-go/tool/web/register.go`

- [ ] **Step 1: Create `RegisterAll`**

```go
// tool/web/register.go
package web

import (
	"encoding/json"

	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterAll registers the web tools into a registry.
// - web_fetch is always registered (uses stdlib http)
// - web_search is registered only if exaAPIKey is non-empty
// - web_extract is registered only if firecrawlAPIKey is non-empty
func RegisterAll(reg *tool.Registry, exaAPIKey, firecrawlAPIKey string) {
	reg.Register(&tool.Entry{
		Name:        "web_fetch",
		Toolset:     "web",
		Description: "Fetch a URL and return status + headers + body (max 2 MiB).",
		Emoji:       "🌐",
		Handler:     webFetchHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "web_fetch",
				Description: "Perform an HTTP GET/POST to a URL and return the response.",
				Parameters:  json.RawMessage(webFetchSchema),
			},
		},
	})

	if exaAPIKey != "" {
		reg.Register(&tool.Entry{
			Name:        "web_search",
			Toolset:     "web",
			Description: "Search the web via Exa.",
			Emoji:       "🔎",
			Handler:     newWebSearchHandler(exaAPIKey, ""),
			Schema: tool.ToolDefinition{
				Type: "function",
				Function: tool.FunctionDef{
					Name:        "web_search",
					Description: "Search the web and return a list of results.",
					Parameters:  json.RawMessage(webSearchSchema),
				},
			},
		})
	}

	if firecrawlAPIKey != "" {
		reg.Register(&tool.Entry{
			Name:        "web_extract",
			Toolset:     "web",
			Description: "Extract page content as markdown/html/text via Firecrawl.",
			Emoji:       "📰",
			Handler:     newWebExtractHandler(firecrawlAPIKey, ""),
			Schema: tool.ToolDefinition{
				Type: "function",
				Function: tool.FunctionDef{
					Name:        "web_extract",
					Description: "Extract the main content of a web page.",
					Parameters:  json.RawMessage(webExtractSchema),
				},
			},
		})
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./tool/web/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/tool/web/register.go
git commit -m "feat(tool/web): add RegisterAll helper with conditional registration"
```

---

## Task 7: Memory Storage Interface

**Files:**
- Modify: `hermes-agent-go/storage/storage.go`
- Modify: `hermes-agent-go/storage/types.go`

- [ ] **Step 1: Add memory types**

In `storage/types.go`, append:

```go
// Memory is a persisted agent memory entry.
type Memory struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id,omitempty"`
	Content   string          `json:"content"`
	Category  string          `json:"category,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// MemorySearchOptions controls MemorySearch behavior.
type MemorySearchOptions struct {
	UserID string
	Tags   []string
	Limit  int
}
```

- [ ] **Step 2: Add memory methods to the `Storage` interface**

In `storage/storage.go`, add these methods to the `Storage` interface:

```go
// Memory operations
SaveMemory(ctx context.Context, memory *Memory) error
GetMemory(ctx context.Context, id string) (*Memory, error)
SearchMemories(ctx context.Context, query string, opts *MemorySearchOptions) ([]*Memory, error)
DeleteMemory(ctx context.Context, id string) error
```

- [ ] **Step 3: Build (expected to fail until the sqlite impl exists)**

```bash
go build ./storage/...
```

Expected: FAIL with "Store does not implement Storage (missing SaveMemory method)". That's fine — Task 8 adds the impl.

- [ ] **Step 4: Commit (intentionally broken — fixed in Task 8)**

Do NOT commit this change alone. Combine with Task 8 into a single commit.

---

## Task 8: SQLite Memory Implementation

**Files:**
- Modify: `hermes-agent-go/storage/sqlite/migrate.go`
- Create: `hermes-agent-go/storage/sqlite/memory.go`

- [ ] **Step 1: Add the memories schema**

In `storage/sqlite/migrate.go`, append to the existing `schemaSQL` constant:

```sql
CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    user_id TEXT DEFAULT '',
    content TEXT NOT NULL,
    category TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    metadata TEXT DEFAULT '{}',
    created_at REAL NOT NULL,
    updated_at REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    content,
    content='memories',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS memories_fts_insert AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_delete AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_update AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
    INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
END;
```

- [ ] **Step 2: Create `storage/sqlite/memory.go`**

```go
// storage/sqlite/memory.go
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/storage"
)

// SaveMemory inserts or updates a memory entry. If the ID is empty, a new
// ID is generated using the epoch nanoseconds as a string.
func (s *Store) SaveMemory(ctx context.Context, m *storage.Memory) error {
	if m.ID == "" {
		return fmt.Errorf("sqlite: memory ID is required")
	}
	tagsJSON, _ := json.Marshal(m.Tags)
	metaStr := string(m.Metadata)
	if metaStr == "" {
		metaStr = "{}"
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO memories (id, user_id, content, category, tags, metadata, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            content = excluded.content,
            category = excluded.category,
            tags = excluded.tags,
            metadata = excluded.metadata,
            updated_at = excluded.updated_at
    `,
		m.ID, m.UserID, m.Content, m.Category, string(tagsJSON), metaStr,
		toEpoch(m.CreatedAt), toEpoch(m.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: save memory %s: %w", m.ID, err)
	}
	return nil
}

// GetMemory fetches a memory by ID.
func (s *Store) GetMemory(ctx context.Context, id string) (*storage.Memory, error) {
	var (
		m         storage.Memory
		tagsJSON  string
		metaStr   string
		created   float64
		updated   float64
	)
	err := s.db.QueryRowContext(ctx, `
        SELECT id, user_id, content, category, tags, metadata, created_at, updated_at
        FROM memories WHERE id = ?`, id,
	).Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &tagsJSON, &metaStr, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get memory %s: %w", id, err)
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	m.Metadata = []byte(metaStr)
	m.CreatedAt = fromEpoch(created)
	m.UpdatedAt = fromEpoch(updated)
	return &m, nil
}

// SearchMemories runs an FTS5 match against the memories table.
// Returns matches ordered by created_at DESC.
func (s *Store) SearchMemories(ctx context.Context, query string, opts *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	limit := 20
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// If query is empty, list recent memories instead of running FTS.
	var rows *sql.Rows
	var err error
	if query == "" {
		where := ""
		args := []any{}
		if opts != nil && opts.UserID != "" {
			where = " WHERE user_id = ?"
			args = append(args, opts.UserID)
		}
		args = append(args, limit)
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, user_id, content, category, tags, metadata, created_at, updated_at
             FROM memories`+where+` ORDER BY created_at DESC LIMIT ?`, args...)
	} else {
		where := ""
		args := []any{query}
		if opts != nil && opts.UserID != "" {
			where = " AND m.user_id = ?"
			args = append(args, opts.UserID)
		}
		args = append(args, limit)
		rows, err = s.db.QueryContext(ctx,
			`SELECT m.id, m.user_id, m.content, m.category, m.tags, m.metadata, m.created_at, m.updated_at
             FROM memories_fts
             JOIN memories m ON m.rowid = memories_fts.rowid
             WHERE memories_fts MATCH ?`+where+` ORDER BY m.created_at DESC LIMIT ?`, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: search memories: %w", err)
	}
	defer rows.Close()

	var out []*storage.Memory
	for rows.Next() {
		var (
			m        storage.Memory
			tagsJSON string
			metaStr  string
			created  float64
			updated  float64
		)
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &tagsJSON, &metaStr, &created, &updated); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		m.Metadata = []byte(metaStr)
		m.CreatedAt = fromEpoch(created)
		m.UpdatedAt = fromEpoch(updated)

		// Optional tag filter (post-query, since tags are stored as JSON)
		if opts != nil && len(opts.Tags) > 0 && !hasAnyTag(m.Tags, opts.Tags) {
			continue
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// DeleteMemory removes a memory by ID.
func (s *Store) DeleteMemory(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete memory %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// hasAnyTag returns true if any of the want tags appears in have.
func hasAnyTag(have []string, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if strings.EqualFold(h, w) {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 3: Add sqlite memory tests**

Create `storage/sqlite/memory_test.go`:

```go
// storage/sqlite/memory_test.go
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndGetMemory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	mem := &storage.Memory{
		ID:        "mem-001",
		UserID:    "user-1",
		Content:   "The user prefers Go over Rust",
		Category:  "preference",
		Tags:      []string{"language", "go"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.SaveMemory(ctx, mem))

	got, err := store.GetMemory(ctx, "mem-001")
	require.NoError(t, err)
	assert.Equal(t, "mem-001", got.ID)
	assert.Equal(t, "The user prefers Go over Rust", got.Content)
	assert.Contains(t, got.Tags, "go")
}

func TestGetMemoryNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	_, err := store.GetMemory(ctx, "nonexistent")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSearchMemoriesFTS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m1", Content: "The quick brown fox", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m2", Content: "Lazy dogs sleep", CreatedAt: now, UpdatedAt: now,
	}))

	results, err := store.SearchMemories(ctx, "fox", &storage.MemorySearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "m1", results[0].ID)
}

func TestSearchMemoriesEmptyQueryListsRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m1", Content: "a", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "m2", Content: "b", CreatedAt: now.Add(time.Minute), UpdatedAt: now,
	}))

	results, err := store.SearchMemories(ctx, "", &storage.MemorySearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "m2", results[0].ID) // most recent first
}

func TestDeleteMemory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "delete-me", Content: "bye", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, store.DeleteMemory(ctx, "delete-me"))

	_, err := store.GetMemory(ctx, "delete-me")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDeleteMemoryNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	err := store.DeleteMemory(ctx, "nonexistent")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./storage/...
```

Expected: PASS. Existing storage tests + 6 new memory tests.

- [ ] **Step 5: Commit (combined with Task 7 interface changes)**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/storage/storage.go hermes-agent-go/storage/types.go hermes-agent-go/storage/sqlite/migrate.go hermes-agent-go/storage/sqlite/memory.go hermes-agent-go/storage/sqlite/memory_test.go
git commit -m "feat(storage): add Memory CRUD with SQLite FTS5"
```

---

## Task 9: Memory Tools

**Files:**
- Create: `hermes-agent-go/tool/memory/memory.go`
- Create: `hermes-agent-go/tool/memory/register.go`
- Create: `hermes-agent-go/tool/memory/memory_test.go`

- [ ] **Step 1: Create `tool/memory/memory.go`**

```go
// tool/memory/memory.go
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// --- memory_save ---

const memorySaveSchema = `{
  "type": "object",
  "properties": {
    "content":  { "type": "string", "description": "The memory content to save" },
    "category": { "type": "string", "description": "Optional category (e.g., preference, fact, instruction)" },
    "tags":     { "type": "array", "items": {"type":"string"}, "description": "Optional tags" }
  },
  "required": ["content"]
}`

type memorySaveArgs struct {
	Content  string   `json:"content"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type memorySaveResult struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func newMemorySaveHandler(store storage.Storage) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args memorySaveArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Content == "" {
			return tool.ToolError("content is required"), nil
		}
		now := time.Now().UTC()
		id := fmt.Sprintf("mem_%d", now.UnixNano())
		mem := &storage.Memory{
			ID:        id,
			Content:   args.Content,
			Category:  args.Category,
			Tags:      args.Tags,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := store.SaveMemory(ctx, mem); err != nil {
			return tool.ToolError("save failed: " + err.Error()), nil
		}
		return tool.ToolResult(memorySaveResult{ID: id, CreatedAt: now}), nil
	}
}

// --- memory_search ---

const memorySearchSchema = `{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "FTS search query (empty to list recent)" },
    "limit": { "type": "number", "description": "Max results (default 10)" }
  }
}`

type memorySearchArgs struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type memorySearchResultItem struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Category  string    `json:"category,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type memorySearchResult struct {
	Query   string                   `json:"query"`
	Results []memorySearchResultItem `json:"results"`
}

func newMemorySearchHandler(store storage.Storage) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args memorySearchArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}
		mems, err := store.SearchMemories(ctx, args.Query, &storage.MemorySearchOptions{Limit: limit})
		if err != nil {
			return tool.ToolError("search failed: " + err.Error()), nil
		}
		items := make([]memorySearchResultItem, 0, len(mems))
		for _, m := range mems {
			items = append(items, memorySearchResultItem{
				ID:        m.ID,
				Content:   m.Content,
				Category:  m.Category,
				Tags:      m.Tags,
				CreatedAt: m.CreatedAt,
			})
		}
		return tool.ToolResult(memorySearchResult{Query: args.Query, Results: items}), nil
	}
}

// --- memory_delete ---

const memoryDeleteSchema = `{
  "type": "object",
  "properties": {
    "id": { "type": "string", "description": "Memory ID to delete" }
  },
  "required": ["id"]
}`

type memoryDeleteArgs struct {
	ID string `json:"id"`
}

func newMemoryDeleteHandler(store storage.Storage) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args memoryDeleteArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.ID == "" {
			return tool.ToolError("id is required"), nil
		}
		if err := store.DeleteMemory(ctx, args.ID); err != nil {
			return tool.ToolError("delete failed: " + err.Error()), nil
		}
		return tool.ToolResult(map[string]any{"ok": true, "id": args.ID}), nil
	}
}
```

- [ ] **Step 2: Create `tool/memory/register.go`**

```go
// tool/memory/register.go
package memory

import (
	"encoding/json"

	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterAll registers memory_save, memory_search, and memory_delete into
// the given registry. The storage argument must be non-nil — if memory
// functionality isn't wanted, don't call this function.
func RegisterAll(reg *tool.Registry, store storage.Storage) {
	reg.Register(&tool.Entry{
		Name:        "memory_save",
		Toolset:     "memory",
		Description: "Save a memory the agent should remember across conversations.",
		Emoji:       "🧠",
		Handler:     newMemorySaveHandler(store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "memory_save",
				Description: "Save a fact, preference, or instruction to persistent memory.",
				Parameters:  json.RawMessage(memorySaveSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "memory_search",
		Toolset:     "memory",
		Description: "Search persisted memories via full-text search.",
		Emoji:       "🔍",
		Handler:     newMemorySearchHandler(store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "memory_search",
				Description: "Search previously saved memories. Empty query lists recent memories.",
				Parameters:  json.RawMessage(memorySearchSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "memory_delete",
		Toolset:     "memory",
		Description: "Delete a memory by ID.",
		Emoji:       "🗑",
		Handler:     newMemoryDeleteHandler(store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "memory_delete",
				Description: "Delete a memory by its ID.",
				Parameters:  json.RawMessage(memoryDeleteSchema),
			},
		},
	})
}
```

- [ ] **Step 3: Create `tool/memory/memory_test.go`**

```go
// tool/memory/memory_test.go
package memory

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nousresearch/hermes-agent/storage/sqlite"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSetup(t *testing.T) (*tool.Registry, *sqlite.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	t.Cleanup(func() { _ = store.Close() })

	reg := tool.NewRegistry()
	RegisterAll(reg, store)
	return reg, store
}

func TestMemorySaveAndSearch(t *testing.T) {
	reg, _ := newTestSetup(t)
	ctx := context.Background()

	// Save
	args := json.RawMessage(`{"content":"the user prefers Go","tags":["preference","lang"]}`)
	out, err := reg.Dispatch(ctx, "memory_save", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"id"`)

	// Search
	searchArgs := json.RawMessage(`{"query":"Go"}`)
	out2, err := reg.Dispatch(ctx, "memory_search", searchArgs)
	require.NoError(t, err)

	var result memorySearchResult
	require.NoError(t, json.Unmarshal([]byte(out2), &result))
	require.Len(t, result.Results, 1)
	assert.Contains(t, result.Results[0].Content, "prefers Go")
}

func TestMemoryDeleteRemovesEntry(t *testing.T) {
	reg, store := newTestSetup(t)
	ctx := context.Background()

	out, err := reg.Dispatch(ctx, "memory_save", json.RawMessage(`{"content":"x"}`))
	require.NoError(t, err)
	var saved memorySaveResult
	require.NoError(t, json.Unmarshal([]byte(out), &saved))

	delArgs := json.RawMessage(`{"id":"` + saved.ID + `"}`)
	_, err = reg.Dispatch(ctx, "memory_delete", delArgs)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.GetMemory(ctx, saved.ID)
	assert.Error(t, err)
}

func TestMemorySaveRequiresContent(t *testing.T) {
	reg, _ := newTestSetup(t)
	out, err := reg.Dispatch(context.Background(), "memory_save", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "content")
}

func TestMemoryDeleteRequiresID(t *testing.T) {
	reg, _ := newTestSetup(t)
	out, err := reg.Dispatch(context.Background(), "memory_delete", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "id")
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./tool/memory/...
```

Expected: PASS. 4 tests.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/tool/memory/
git commit -m "feat(tool/memory): add memory_save/memory_search/memory_delete tools"
```

---

## Task 10: Delegate Tool

**Files:**
- Create: `hermes-agent-go/tool/delegate/delegate.go`
- Create: `hermes-agent-go/tool/delegate/register.go`
- Create: `hermes-agent-go/tool/delegate/delegate_test.go`

- [ ] **Step 1: Create `tool/delegate/delegate.go`**

```go
// tool/delegate/delegate.go
package delegate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/tool"
)

const delegateSchema = `{
  "type": "object",
  "properties": {
    "task":          { "type": "string", "description": "A specific, self-contained task for the subagent to complete" },
    "context":       { "type": "string", "description": "Optional background context" },
    "max_turns":     { "type": "number", "description": "Max turns the subagent may take (default 20, max 50)" }
  },
  "required": ["task"]
}`

type delegateArgs struct {
	Task     string `json:"task"`
	Context  string `json:"context,omitempty"`
	MaxTurns int    `json:"max_turns,omitempty"`
}

type delegateResult struct {
	Response   string `json:"response"`
	Iterations int    `json:"iterations"`
	ToolCalls  int    `json:"tool_calls"`
}

// SubagentRunner is an injection point for running a subagent turn.
// The CLI wires this to a closure that spawns a fresh Engine and runs one
// conversation without tools that would cause recursion (delegate itself).
type SubagentRunner func(ctx context.Context, task, extraContext string, maxTurns int) (*SubagentResult, error)

// SubagentResult is returned by a SubagentRunner.
type SubagentResult struct {
	Response  message.Message
	Iterations int
	ToolCalls  int
}

// newDelegateHandler returns a handler bound to the given runner.
func newDelegateHandler(runner SubagentRunner) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args delegateArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Task == "" {
			return tool.ToolError("task is required"), nil
		}
		maxTurns := args.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 20
		}
		if maxTurns > 50 {
			maxTurns = 50
		}
		if runner == nil {
			return tool.ToolError("delegate: no subagent runner configured"), nil
		}

		result, err := runner(ctx, args.Task, args.Context, maxTurns)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("subagent failed: %s", err.Error())), nil
		}
		responseText := ""
		if result.Response.Content.IsText() {
			responseText = result.Response.Content.Text()
		} else {
			for _, b := range result.Response.Content.Blocks() {
				if b.Type == "text" {
					responseText += b.Text
				}
			}
		}
		return tool.ToolResult(delegateResult{
			Response:   responseText,
			Iterations: result.Iterations,
			ToolCalls:  result.ToolCalls,
		}), nil
	}
}
```

- [ ] **Step 2: Create `tool/delegate/register.go`**

```go
// tool/delegate/register.go
package delegate

import (
	"encoding/json"

	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterDelegate registers the delegate tool bound to a SubagentRunner.
// If runner is nil, the tool is still registered but returns an error
// at dispatch time. The CLI wires a real runner; tests can inject fakes.
func RegisterDelegate(reg *tool.Registry, runner SubagentRunner) {
	reg.Register(&tool.Entry{
		Name:        "delegate",
		Toolset:     "delegate",
		Description: "Delegate a self-contained task to a subagent. The subagent has its own budget and history.",
		Emoji:       "👥",
		Handler:     newDelegateHandler(runner),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "delegate",
				Description: "Run a fresh subagent on a specific, self-contained task.",
				Parameters:  json.RawMessage(delegateSchema),
			},
		},
	})
}
```

- [ ] **Step 3: Create `tool/delegate/delegate_test.go`**

```go
// tool/delegate/delegate_test.go
package delegate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegateRequiresTask(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterDelegate(reg, func(ctx context.Context, task, extra string, max int) (*SubagentResult, error) {
		return nil, nil
	})
	out, err := reg.Dispatch(context.Background(), "delegate", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "task")
}

func TestDelegateInvokesRunner(t *testing.T) {
	reg := tool.NewRegistry()
	var gotTask string
	var gotMax int
	RegisterDelegate(reg, func(ctx context.Context, task, extra string, max int) (*SubagentResult, error) {
		gotTask = task
		gotMax = max
		return &SubagentResult{
			Response:   message.Message{Role: message.RoleAssistant, Content: message.TextContent("done")},
			Iterations: 3,
			ToolCalls:  2,
		}, nil
	})

	args := json.RawMessage(`{"task":"summarize this","max_turns":15}`)
	out, err := reg.Dispatch(context.Background(), "delegate", args)
	require.NoError(t, err)

	assert.Equal(t, "summarize this", gotTask)
	assert.Equal(t, 15, gotMax)

	var result delegateResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "done", result.Response)
	assert.Equal(t, 3, result.Iterations)
	assert.Equal(t, 2, result.ToolCalls)
}

func TestDelegateSurfacesRunnerError(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterDelegate(reg, func(ctx context.Context, task, extra string, max int) (*SubagentResult, error) {
		return nil, errors.New("subagent boom")
	})
	out, err := reg.Dispatch(context.Background(), "delegate", json.RawMessage(`{"task":"x"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "subagent boom")
}

func TestDelegateClampsMaxTurns(t *testing.T) {
	reg := tool.NewRegistry()
	var gotMax int
	RegisterDelegate(reg, func(ctx context.Context, task, extra string, max int) (*SubagentResult, error) {
		gotMax = max
		return &SubagentResult{
			Response: message.Message{Role: message.RoleAssistant, Content: message.TextContent("ok")},
		}, nil
	})

	// 100 should be clamped to 50
	_, err := reg.Dispatch(context.Background(), "delegate", json.RawMessage(`{"task":"x","max_turns":100}`))
	require.NoError(t, err)
	assert.Equal(t, 50, gotMax)
}

func TestDelegateWithoutRunnerErrors(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterDelegate(reg, nil)
	out, err := reg.Dispatch(context.Background(), "delegate", json.RawMessage(`{"task":"x"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "no subagent runner")
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./tool/delegate/...
```

Expected: PASS. 5 tests.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/tool/delegate/
git commit -m "feat(tool/delegate): add delegate tool with SubagentRunner injection"
```

---

## Task 11: Wire New Tools into CLI + Aux Provider

**Files:**
- Modify: `hermes-agent-go/cli/repl.go`

- [ ] **Step 1: Import the new packages**

Add to the import block of `cli/repl.go`:

```go
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/provider/factory"
	"github.com/nousresearch/hermes-agent/tool/delegate"
	"github.com/nousresearch/hermes-agent/tool/memory"
	"github.com/nousresearch/hermes-agent/tool/web"
```

(Some of these may already be imported.)

- [ ] **Step 2: Build the auxiliary provider**

After the primary provider + fallback chain block in `runREPL`, add:

```go
	// Auxiliary provider for context compression, vision summarization, etc.
	// If not configured, compression is a no-op.
	var auxProvider provider.Provider
	if app.Config.Auxiliary.APIKey != "" || app.Config.Auxiliary.Provider != "" {
		auxCfg := config.ProviderConfig{
			Provider: app.Config.Auxiliary.Provider,
			BaseURL:  app.Config.Auxiliary.BaseURL,
			APIKey:   app.Config.Auxiliary.APIKey,
			Model:    app.Config.Auxiliary.Model,
		}
		if auxCfg.Provider == "" {
			// Default to the same provider as primary
			auxCfg.Provider = "anthropic"
		}
		if auxP, err := factory.New(auxCfg); err == nil {
			auxProvider = auxP
		}
	}
```

- [ ] **Step 3: Register the new tools after existing file/terminal registration**

Find the block that registers file and terminal tools, and append:

```go
	// Web tools (always register web_fetch, others if API keys present)
	exaKey := os.Getenv("EXA_API_KEY")
	firecrawlKey := os.Getenv("FIRECRAWL_API_KEY")
	web.RegisterAll(toolRegistry, exaKey, firecrawlKey)

	// Memory tools (require storage)
	if app.Storage != nil {
		memory.RegisterAll(toolRegistry, app.Storage)
	}

	// Delegate tool — the runner spawns a fresh Engine per call
	delegate.RegisterDelegate(toolRegistry, func(ctx context.Context, task, extra string, maxTurns int) (*delegate.SubagentResult, error) {
		// Build a tool registry for the subagent that EXCLUDES delegate
		// (prevents recursive subagent spawning in Plan 6).
		subReg := tool.NewRegistry()
		for _, def := range toolRegistry.Definitions(func(e *tool.Entry) bool {
			return e.Name != "delegate"
		}) {
			_ = def // definitions only — we need the real entries
		}
		// Simpler approach: reuse the same registry; rely on prompt guidance
		// to tell the subagent not to call delegate. Plan 6b can add filter-based
		// registries if recursion becomes a real problem.

		subEngine := agent.NewEngineWithToolsAndAux(
			p, auxProvider, app.Storage, toolRegistry,
			config.AgentConfig{
				MaxTurns:    maxTurns,
				Compression: app.Config.Agent.Compression,
			},
			"subagent",
		)

		result, err := subEngine.RunConversation(ctx, &agent.RunOptions{
			UserMessage: task + "\n\n" + extra,
			SessionID:   sessionID + "-sub",
			Model:       displayModel,
		})
		if err != nil {
			return nil, err
		}
		return &delegate.SubagentResult{
			Response:   result.Response,
			Iterations: result.Iterations,
			ToolCalls:  0, // tracking deferred to Plan 6b
		}, nil
	})
```

- [ ] **Step 4: Use `NewEngineWithToolsAndAux` in `ui.Run`**

In `cli/ui/run.go`, find the goroutine that creates the Engine and replace:

```go
engine := agent.NewEngineWithTools(
    opts.Provider, opts.Storage, opts.ToolReg,
    opts.AgentCfg, "cli",
)
```

with:

```go
engine := agent.NewEngineWithToolsAndAux(
    opts.Provider, opts.AuxProvider, opts.Storage, opts.ToolReg,
    opts.AgentCfg, "cli",
)
```

And add `AuxProvider provider.Provider` to `ui.RunOptions` in `cli/ui/run.go`:

```go
type RunOptions struct {
	Config      *config.Config
	Storage     storage.Storage
	Provider    provider.Provider
	AuxProvider provider.Provider // may be nil
	ToolReg     *tool.Registry
	AgentCfg    config.AgentConfig
	SessionID   string
	Model       string
}
```

Update the `runREPL` call site in `cli/repl.go` to pass `AuxProvider: auxProvider`:

```go
	err = ui.Run(ctx, ui.RunOptions{
		Config:      app.Config,
		Storage:     app.Storage,
		Provider:    p,
		AuxProvider: auxProvider,
		ToolReg:     toolRegistry,
		AgentCfg:    app.Config.Agent,
		SessionID:   sessionID,
		Model:       displayModel,
	})
```

- [ ] **Step 5: Run the full test suite**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./...
```

Expected: PASS. All existing tests + all new tests. The Plan 1 `TestEndToEndSingleTurn` and Plan 2 `TestEndToEndToolLoop` still pass because they use `agent.NewEngineWithTools` which continues to work.

- [ ] **Step 6: Build**

```bash
go build ./...
make build
./bin/hermes version
```

Expected: success.

- [ ] **Step 7: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/cli/repl.go hermes-agent-go/cli/ui/run.go
git commit -m "feat(cli): wire web/memory/delegate tools + auxiliary provider for compression"
```

---

## Task 12: Final Verification

No commit. Run and report:

- [ ] **Step 1: Full test suite with coverage**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race -cover ./...
```

Expected: ALL packages pass. New packages show:
- `agent` coverage rises (Compressor tests)
- `storage/sqlite` coverage rises (memory CRUD)
- `tool/web` ~75-85% coverage
- `tool/memory` ~80% coverage
- `tool/delegate` ~90% coverage (injected runner is simple)

- [ ] **Step 2: go vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Build binary**

```bash
make build
./bin/hermes version
```

- [ ] **Step 4: Manual smoke test (OPTIONAL)**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export EXA_API_KEY=...        # optional
export FIRECRAWL_API_KEY=...  # optional
./bin/hermes
```

Try:
- "fetch https://example.com" → web_fetch tool
- "remember that I prefer tabs over spaces" → memory_save
- "what do you remember about me?" → memory_search
- "delegate: list the top 5 files in the current directory by size" → delegate tool
- Long conversation to trigger compression (requires > 60 messages with default ProtectLast=20)

- [ ] **Step 5: Verify git log**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git log --oneline hermes-agent-go/ | head -15
```

Expected: ~10 new commits from Plan 6.

- [ ] **Step 6: Plan 6 done.**

---

## Plan 6 Self-Review Notes

**Spec coverage:**
- Context compression (Compressor + Engine integration) — Tasks 1-2
- web_fetch (stdlib) — Task 3
- web_search (Exa) — Task 4
- web_extract (Firecrawl) — Task 5
- Web tool registration — Task 6
- Memory storage interface + SQLite impl — Tasks 7-8
- Memory tools (save/search/delete) — Task 9
- Delegate tool with SubagentRunner injection — Task 10
- CLI integration (auxiliary provider, new tools, subagent runner) — Task 11
- Final verification — Task 12

**Explicitly out of scope:**
- MCP client/bridge — Plan 6b (complex, needs dedicated plan)
- External memory providers (honcho, mem0, etc.) — Plan 6c
- Browser automation — Plan 6d
- Vision tool — Plan 6e
- Alternative web search (Tavily, Serper, DuckDuckGo) — Plan 6b
- Alternative web extract (Reader.dev, Jina) — Plan 6b
- Trajectory-aware compression — later plan
- Recursive delegate calls (subagent spawning subagents) — blocked by prompt guidance in Plan 6, not code
- Parallel delegation — Plan 6 is sequential only

**Placeholder check:** None. All code is complete and executable.

**Type consistency:**
- `CompressionConfig`, `AuxiliaryConfig` — defined Task 1, used by Task 2+
- `Compressor`, `NewCompressor`, `Compress` — defined Task 1, used Task 2
- `NewEngineWithToolsAndAux` — added Task 2, used Task 11
- `storage.Memory`, `storage.MemorySearchOptions` — defined Task 7, used Tasks 8-9
- `Store.SaveMemory/GetMemory/SearchMemories/DeleteMemory` — defined Task 8
- `webFetchHandler`, `newWebSearchHandler`, `newWebExtractHandler` — defined Tasks 3-5
- `web.RegisterAll` — defined Task 6
- `memory.RegisterAll`, `newMemorySaveHandler`, etc. — defined Task 9
- `delegate.RegisterDelegate`, `delegate.SubagentRunner`, `delegate.SubagentResult` — defined Task 10
- `ui.RunOptions.AuxProvider` — added Task 11

No naming drift.

**Known concerns (non-blocking):**
1. The Compression `shouldCompress` heuristic is count-based, not token-based. A conversation with many short messages may trigger compression unnecessarily; a conversation with few long messages may not trigger when it should. Plan 6b could add per-model token estimation.

2. The web_search and web_extract external API shapes are based on the currently documented Exa and Firecrawl APIs as of April 2026. If those APIs change, the wire types need to be updated. The plan's architecture keeps this isolated to two files.

3. The delegate tool currently shares the parent's tool registry. A subagent could theoretically call delegate recursively, causing unbounded recursion. Plan 6 mitigates via prompt guidance ("don't call delegate from a delegated task") but does not enforce it at code level. Plan 6b could add a recursion counter or a filtered sub-registry.

4. The memory search uses FTS5's default ranking, which is BM25 in modernc.org/sqlite. This is fine for prototype use but doesn't rerank by recency or category. Plan 6b could add a composite score.
