# Plan 2: Tool System + Engine Multi-Turn Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the single-turn CLI from Plan 1 into a working tool-using agent that can read files and execute local shell commands.

**Architecture:** Add a `tool/` package with a thread-safe registry. Implement the local terminal backend and core file tools. Extend `message.Content` to carry tool_use and tool_result blocks. Upgrade the Anthropic provider to emit tool_use and accept tool_result messages. Replace the Engine's single-turn `RunConversation` with a budget-enforced loop that executes tool calls and feeds results back to the LLM until the assistant stops issuing tools.

**Tech Stack:** Go 1.25, stdlib `os/exec` (local shell), `path/filepath` + `io/fs` (file tools), `golang.org/x/sync/errgroup` (parallel tool execution), `encoding/json` for tool arguments. Extends Plan 1 foundation: `message`, `provider/anthropic`, `agent`, `storage`, `cli`.

**Deliverable at end of plan:**
```
$ ./bin/hermes
HERMES AGENT
claude-opus-4-6 · session #new
> What files are in this directory?
◆ Thinking...
⚡ list_directory: .
│ agent/  cli/  cmd/  config/  go.mod  Makefile  message/  provider/  storage/  tool/
└ exit 0
This directory contains the hermes-agent-go Go module...
> Read go.mod and tell me the module path
⚡ read_file: go.mod
│ module github.com/nousresearch/hermes-agent
│ go 1.25.0
│ ...
└ exit 0
The module path is `github.com/nousresearch/hermes-agent`.
> /exit
Session saved. 4 messages, 2 tool calls. $0.003.
```

**Non-goals for this plan (deferred):**
- Docker, SSH, Modal, Daytona, Singularity terminal backends (Plan 5)
- Browser automation, code execution sandbox, delegate (Plan 5)
- Web tools: web_search, web_extract, web_fetch (Plan 5)
- Memory providers, skills, vision, MCP (Plan 6)
- Context compression, iteration budget warnings in UI (Plan 6)
- Persistent shell within local backend (Plan 5)
- Tool call parallelization beyond 1 concurrent call (Plan 5 adds 8-way parallelism)

**Plan 1 dependencies this plan touches:**
- `message/content.go` — add tool_use and tool_result ContentBlock types
- `provider/anthropic/types.go` — add tool_use content blocks to wire types
- `provider/anthropic/complete.go` — handle tools in buildRequest, parse tool_use from response
- `provider/anthropic/stream.go` — handle content_block_delta with tool_use input JSON
- `provider/provider.go` — add Tools field to Request
- `agent/engine.go` — add tools registry dependency
- `agent/conversation.go` — replace single-turn logic with tool-call loop
- `cli/repl.go` — register built-in tools in ensureStorage equivalent, display tool execution

---

## File Structure

```
hermes-agent-go/
├── tool/                            # NEW package
│   ├── registry.go                  # Registry, Entry, Handler, Dispatch
│   ├── registry_test.go
│   ├── helpers.go                   # ToolError, ToolResult, mustJSON
│   ├── helpers_test.go
│   ├── terminal/
│   │   ├── terminal.go              # Backend interface + factory
│   │   ├── local.go                 # Local exec backend (os/exec)
│   │   ├── local_test.go
│   │   └── tools.go                 # register shell_execute tool handler
│   └── file/
│       ├── read.go                  # read_file tool
│       ├── write.go                 # write_file tool
│       ├── list.go                  # list_directory tool
│       ├── search.go                # search_files tool (glob-based)
│       ├── file_test.go
│       └── register.go              # registers all file tools into a Registry
├── message/
│   └── content.go                   # MODIFIED: add ToolUseID, ToolUseInput, ToolResult fields
├── provider/
│   ├── provider.go                  # MODIFIED: Request.Tools []ToolDefinition
│   └── anthropic/
│       ├── types.go                 # MODIFIED: tool_use content item, tools field
│       ├── complete.go              # MODIFIED: emit tools, parse tool_use
│       └── stream.go                # MODIFIED: tool_use delta handling
├── agent/
│   ├── engine.go                    # MODIFIED: add tools *tool.Registry field
│   └── conversation.go              # REWRITTEN: budget-enforced loop with tool execution
└── cli/
    └── repl.go                      # MODIFIED: register tools, print tool execution
```

---

## Task 1: Extend Message Content for Tool Blocks

**Files:**
- Modify: `hermes-agent-go/message/content.go`
- Modify: `hermes-agent-go/message/content_test.go`

- [ ] **Step 1: Add failing tests for tool_use and tool_result blocks**

Append to `message/content_test.go`:

```go
func TestContentBlockToolUseRoundtrip(t *testing.T) {
	blocks := []ContentBlock{
		{
			Type:         "tool_use",
			ToolUseID:    "tool_abc123",
			ToolUseName:  "read_file",
			ToolUseInput: json.RawMessage(`{"path":"go.mod"}`),
		},
	}
	c := BlockContent(blocks)
	data, err := c.MarshalJSON()
	require.NoError(t, err)

	var decoded Content
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Len(t, decoded.Blocks(), 1)
	b := decoded.Blocks()[0]
	assert.Equal(t, "tool_use", b.Type)
	assert.Equal(t, "tool_abc123", b.ToolUseID)
	assert.Equal(t, "read_file", b.ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(b.ToolUseInput))
}

func TestContentBlockToolResultRoundtrip(t *testing.T) {
	blocks := []ContentBlock{
		{
			Type:       "tool_result",
			ToolUseID:  "tool_abc123",
			ToolResult: "file contents here",
		},
	}
	c := BlockContent(blocks)
	data, err := c.MarshalJSON()
	require.NoError(t, err)

	var decoded Content
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Len(t, decoded.Blocks(), 1)
	b := decoded.Blocks()[0]
	assert.Equal(t, "tool_result", b.Type)
	assert.Equal(t, "tool_abc123", b.ToolUseID)
	assert.Equal(t, "file contents here", b.ToolResult)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test ./message/...
```

Expected: FAIL with "unknown field ToolUseID".

- [ ] **Step 3: Extend `ContentBlock` struct**

In `message/content.go`, modify `ContentBlock` from:

```go
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *Image `json:"image_url,omitempty"`
}
```

to:

```go
// ContentBlock is one element of a structured content array.
// Used for multimodal content (images), tool use (tool_use), and tool
// results (tool_result).
type ContentBlock struct {
	Type string `json:"type"`

	// Text content (type: "text")
	Text string `json:"text,omitempty"`

	// Image content (type: "image_url")
	ImageURL *Image `json:"image_url,omitempty"`

	// Tool use — the assistant is asking to invoke a tool.
	// Type: "tool_use"
	ToolUseID    string          `json:"id,omitempty"`
	ToolUseName  string          `json:"name,omitempty"`
	ToolUseInput json.RawMessage `json:"input,omitempty"`

	// Tool result — the tool's output fed back to the assistant.
	// Type: "tool_result"
	// Note: ToolUseID is reused to identify which tool call this result belongs to.
	ToolResult string `json:"content,omitempty"`
}
```

Add `"encoding/json"` to the imports of `content.go` if not already present (it's already there for MarshalJSON/UnmarshalJSON).

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -race ./message/...
```

Expected: PASS. All message tests (including the two new tool-block tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/message/
git commit -m "feat(message): add tool_use and tool_result ContentBlock fields"
```

---

## Task 2: Tool Registry Core

**Files:**
- Create: `hermes-agent-go/tool/registry.go`
- Create: `hermes-agent-go/tool/helpers.go`
- Create: `hermes-agent-go/tool/registry_test.go`
- Create: `hermes-agent-go/tool/helpers_test.go`

- [ ] **Step 1: Write failing tests for the registry**

Create `tool/registry_test.go`:

```go
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func echoHandler(ctx context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

func failingHandler(ctx context.Context, args json.RawMessage) (string, error) {
	return "", errors.New("boom")
}

func TestRegisterAndDispatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "echo",
		Toolset: "test",
		Handler: echoHandler,
		Schema: ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        "echo",
				Description: "echo input",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	out, err := r.Dispatch(context.Background(), "echo", json.RawMessage(`{"hi":"there"}`))
	require.NoError(t, err)
	assert.Equal(t, `{"hi":"there"}`, out)
}

func TestDispatchUnknownTool(t *testing.T) {
	r := NewRegistry()
	out, err := r.Dispatch(context.Background(), "missing", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "unknown tool")
	assert.Contains(t, out, `"error"`)
}

func TestDispatchHandlerError(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "fail",
		Handler: failingHandler,
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "fail"}},
	})

	out, err := r.Dispatch(context.Background(), "fail", nil)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "boom")
}

func TestDefinitionsFiltersUnavailable(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "always",
		Handler: echoHandler,
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "always"}},
	})
	r.Register(&Entry{
		Name:    "hidden",
		Handler: echoHandler,
		CheckFn: func() bool { return false },
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "hidden"}},
	})

	defs := r.Definitions(nil)
	require.Len(t, defs, 1)
	assert.Equal(t, "always", defs[0].Function.Name)
}

func TestResultTruncation(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name: "big",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(make([]byte, 500)), nil
		},
		MaxResultChars: 100,
		Schema:         ToolDefinition{Type: "function", Function: FunctionDef{Name: "big"}},
	})

	out, err := r.Dispatch(context.Background(), "big", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "[truncated]")
	assert.LessOrEqual(t, len(out), 150) // truncation marker adds a bit
}

func TestConcurrentRegisterDispatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "concurrent",
		Handler: echoHandler,
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "concurrent"}},
	})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = r.Dispatch(context.Background(), "concurrent", json.RawMessage(`{}`))
		}()
		go func(n int) {
			defer func() { done <- struct{}{} }()
			r.Register(&Entry{
				Name:    nameForInt(n),
				Handler: echoHandler,
				Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: nameForInt(n)}},
			})
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

func nameForInt(n int) string {
	return string(rune('a'+n)) + "tool"
}
```

Create `tool/helpers_test.go`:

```go
package tool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolError(t *testing.T) {
	out := ToolError("not found")
	var decoded map[string]any
	require.NoError := assert.NoError
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "not found", decoded["error"])
}

func TestToolResultWithMap(t *testing.T) {
	out := ToolResult(map[string]any{"ok": true, "count": 3})
	var decoded map[string]any
	require.NoError := assert.NoError
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, true, decoded["ok"])
	assert.Equal(t, float64(3), decoded["count"]) // JSON numbers decode to float64
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./tool/...
```

Expected: FAIL. Package doesn't exist yet.

- [ ] **Step 3: Create `tool/registry.go`**

```go
package tool

import (
	"context"
	"encoding/json"
	"sync"
)

// Handler is the function signature every tool implements.
// Returns a JSON-encoded result string and an error. Errors from the
// handler are caught by Dispatch and returned as a ToolError JSON string
// so the LLM sees a structured error payload.
type Handler func(ctx context.Context, args json.RawMessage) (string, error)

// CheckFunc returns whether a tool is currently available (e.g., the
// required environment variables or external services are present).
type CheckFunc func() bool

// Entry describes a single tool registered in the Registry.
type Entry struct {
	Name           string
	Toolset        string // "terminal", "file", "web", ...
	Schema         ToolDefinition
	Handler        Handler
	CheckFn        CheckFunc
	RequiresEnv    []string
	IsInteractive  bool // interactive tools cannot run in parallel
	MaxResultChars int  // truncate results larger than this (0 = no limit)
	Description    string
	Emoji          string
}

// Registry holds all registered tools. Safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*Entry)}
}

// Register adds or replaces a tool entry. Safe to call concurrently.
func (r *Registry) Register(entry *Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[entry.Name] = entry
}

// Dispatch looks up a tool by name and invokes its handler with args.
// Returns a JSON string (always — errors are encoded as {"error": "..."}).
// The outer error return is reserved for fundamental dispatch failures
// that should not be fed back to the LLM (e.g., context canceled).
func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	entry, ok := r.entries[name]
	r.mu.RUnlock()
	if !ok {
		return ToolError("unknown tool: " + name), nil
	}

	// Execute the handler, catching errors and panics
	result, err := r.execHandler(ctx, entry, args)
	if err != nil {
		return ToolError(err.Error()), nil
	}

	// Truncate oversized results
	if entry.MaxResultChars > 0 && len(result) > entry.MaxResultChars {
		result = result[:entry.MaxResultChars] + "\n... [truncated]"
	}
	return result, nil
}

// execHandler invokes the handler with panic recovery.
func (r *Registry) execHandler(ctx context.Context, entry *Entry, args json.RawMessage) (result string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = newPanicError(entry.Name, p)
		}
	}()
	return entry.Handler(ctx, args)
}

// IsInteractive reports whether a tool is marked interactive (cannot run in parallel).
func (r *Registry) IsInteractive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[name]
	if !ok {
		return false
	}
	return entry.IsInteractive
}

// Definitions returns the OpenAI-format tool definitions for all registered
// tools that pass filter and whose CheckFn returns true.
func (r *Registry) Definitions(filter func(*Entry) bool) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.entries))
	for _, entry := range r.entries {
		if filter != nil && !filter(entry) {
			continue
		}
		if entry.CheckFn != nil && !entry.CheckFn() {
			continue
		}
		defs = append(defs, entry.Schema)
	}
	return defs
}

// ToolDefinition is the OpenAI function-calling schema shape.
// Providers convert this to their own wire format.
type ToolDefinition struct {
	Type     string      `json:"type"` // always "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef describes the callable function inside a ToolDefinition.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for the input
}
```

- [ ] **Step 4: Create `tool/helpers.go`**

```go
package tool

import (
	"encoding/json"
	"fmt"
)

// ToolError encodes an error message as a JSON object: {"error": "msg"}.
// All tool handlers should return errors via this helper (or via (string, error)
// which Dispatch converts automatically) so the LLM sees structured output.
func ToolError(msg string) string {
	return mustJSON(map[string]any{"error": msg})
}

// ToolResult encodes data as a JSON string. If data is already a string,
// it is returned as-is (not double-encoded).
func ToolResult(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	return mustJSON(data)
}

// mustJSON marshals v and panics only if the input is unmarshalable
// (which means a programming error, not a user error).
func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"tool: marshal: %s"}`, err.Error())
	}
	return string(data)
}

// newPanicError constructs an error from a recovered panic value.
func newPanicError(toolName string, p any) error {
	return fmt.Errorf("tool %q panicked: %v", toolName, p)
}
```

- [ ] **Step 5: Fix the helpers_test.go testing import issue**

The test I wrote in step 1 uses `require := assert.NoError` — that's wrong. Replace `tool/helpers_test.go` with:

```go
package tool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolError(t *testing.T) {
	out := ToolError("not found")
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "not found", decoded["error"])
}

func TestToolResultWithMap(t *testing.T) {
	out := ToolResult(map[string]any{"ok": true, "count": 3})
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, true, decoded["ok"])
	assert.Equal(t, float64(3), decoded["count"])
}

func TestToolResultWithString(t *testing.T) {
	out := ToolResult("already encoded")
	assert.Equal(t, "already encoded", out)
}
```

- [ ] **Step 6: Run all tool tests**

```bash
go test -race ./tool/...
```

Expected: PASS. All registry + helpers tests pass.

- [ ] **Step 7: Run go vet**

```bash
go vet ./tool/...
```

Expected: clean.

- [ ] **Step 8: Commit**

```bash
git add hermes-agent-go/tool/registry.go hermes-agent-go/tool/helpers.go hermes-agent-go/tool/registry_test.go hermes-agent-go/tool/helpers_test.go
git commit -m "feat(tool): add thread-safe Registry with Dispatch + helpers"
```

---

## Task 3: Terminal Backend Interface + Local Implementation

**Files:**
- Create: `hermes-agent-go/tool/terminal/terminal.go`
- Create: `hermes-agent-go/tool/terminal/local.go`
- Create: `hermes-agent-go/tool/terminal/local_test.go`

- [ ] **Step 1: Write failing tests for Local backend**

Create `tool/terminal/local_test.go`:

```go
package terminal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalExecSimpleCommand(t *testing.T) {
	b, err := NewLocal(Config{})
	require.NoError(t, err)
	defer b.Close()

	result, err := b.Execute(context.Background(), "echo hello", &ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello")
}

func TestLocalExecCaptureStderr(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "echo oops 1>&2", &ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Contains(t, result.Stderr, "oops")
}

func TestLocalExecPreservesExitCode(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "exit 7", &ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 7, result.ExitCode)
}

func TestLocalExecRespectsCwd(t *testing.T) {
	dir := t.TempDir()
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "pwd", &ExecOptions{
		Cwd:     dir,
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	// macOS may prepend /private to temp dirs — use HasSuffix to tolerate
	got := strings.TrimSpace(result.Stdout)
	assert.True(t, strings.HasSuffix(got, dir), "expected %q to end with %q", got, dir)
}

func TestLocalExecTimeout(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	start := time.Now()
	result, err := b.Execute(context.Background(), "sleep 10", &ExecOptions{Timeout: 200 * time.Millisecond})
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second, "timeout should kill the process quickly")
	if err == nil {
		// Some shells report timeout via non-zero exit code instead of error
		assert.NotEqual(t, 0, result.ExitCode)
	}
}

func TestLocalExecStdin(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "cat", &ExecOptions{
		Stdin:   "piped input",
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "piped input")
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./tool/terminal/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Create `tool/terminal/terminal.go`**

```go
package terminal

import (
	"context"
	"fmt"
	"time"
)

// Backend executes shell commands. Implementations may run locally, in a
// Docker container, over SSH, or via a serverless runtime.
// Implementations must be safe for concurrent use unless documented otherwise.
type Backend interface {
	// Execute runs a command and returns its result.
	Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error)
	// SupportsPersistentShell reports whether the backend maintains state
	// (cwd, env vars) across Execute calls in the same Backend instance.
	SupportsPersistentShell() bool
	// Close releases any resources held by the backend.
	Close() error
}

// ExecOptions control a single Execute invocation.
type ExecOptions struct {
	Cwd     string
	Env     map[string]string
	Timeout time.Duration
	Stdin   string
}

// ExecResult holds the outcome of Execute.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Config is the shared configuration for terminal backend factories.
// Only the fields relevant to a given backend are read.
type Config struct {
	Cwd             string
	DockerImage     string
	DockerVolumes   []string
	SSHHost         string
	SSHUser         string
	SSHKey          string
	PersistentShell bool // hint only — not all backends support it
	Timeout         time.Duration
}

// New constructs a backend by name. Only "local" is implemented in this plan.
// Plan 5 adds "docker", "ssh", "modal", "daytona", "singularity".
func New(backendType string, cfg Config) (Backend, error) {
	switch backendType {
	case "local", "":
		return NewLocal(cfg)
	default:
		return nil, fmt.Errorf("terminal: backend %q is not supported in this build", backendType)
	}
}
```

- [ ] **Step 4: Create `tool/terminal/local.go`**

```go
package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Local executes commands on the host OS via /bin/sh -c (or cmd /C on Windows).
// Stateless — no persistent shell support in this plan (Plan 5 will add one).
type Local struct {
	defaultCwd string
}

// NewLocal constructs a Local backend.
func NewLocal(cfg Config) (*Local, error) {
	cwd := cfg.Cwd
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	return &Local{defaultCwd: cwd}, nil
}

// Execute runs command via the system shell.
func (l *Local) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	// Apply timeout via context.WithTimeout if non-zero
	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	shell, shellFlag := defaultShell()
	cmd := exec.CommandContext(runCtx, shell, shellFlag, command)

	// Working directory
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	} else {
		cmd.Dir = l.defaultCwd
	}

	// Environment: inherit host env, then apply overrides
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), envToSlice(opts.Env)...)
	}

	// Stdin
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Non-exit-code errors (process couldn't start, timeout, etc.)
			// Report as a non-zero exit with the error in stderr.
			exitCode = -1
			if stderr.Len() == 0 {
				stderr.WriteString(runErr.Error())
			}
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// SupportsPersistentShell always returns false for the stateless local backend.
func (l *Local) SupportsPersistentShell() bool { return false }

// Close is a no-op for the local backend.
func (l *Local) Close() error { return nil }

// defaultShell returns the shell and its "execute this string" flag for the current OS.
func defaultShell() (shell, flag string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/C"
	}
	return "/bin/sh", "-c"
}

// envToSlice converts a map to os.Environ()-style "K=V" slice.
func envToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
```

- [ ] **Step 5: Run the tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS. All 6 local tests.

- [ ] **Step 6: Commit**

```bash
git add hermes-agent-go/tool/terminal/
git commit -m "feat(tool): add Backend interface with Local terminal implementation"
```

---

## Task 4: Shell Execute Tool (register via terminal package)

**Files:**
- Create: `hermes-agent-go/tool/terminal/tools.go`
- Modify: `hermes-agent-go/tool/terminal/local_test.go` (add shell_execute test)

- [ ] **Step 1: Append failing test for shell_execute tool registration**

Append to `tool/terminal/local_test.go`:

```go
import (
	"context"
	"encoding/json"
	// ... existing imports
	"github.com/nousresearch/hermes-agent/tool"
)

func TestShellExecuteToolRegisters(t *testing.T) {
	reg := tool.NewRegistry()
	backend, _ := NewLocal(Config{})
	defer backend.Close()

	RegisterShellExecute(reg, backend)

	defs := reg.Definitions(nil)
	require.Len(t, defs, 1)
	assert.Equal(t, "shell_execute", defs[0].Function.Name)
}

func TestShellExecuteToolDispatch(t *testing.T) {
	reg := tool.NewRegistry()
	backend, _ := NewLocal(Config{})
	defer backend.Close()
	RegisterShellExecute(reg, backend)

	args := json.RawMessage(`{"command":"echo hi"}`)
	result, err := reg.Dispatch(context.Background(), "shell_execute", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
	assert.Contains(t, decoded["stdout"], "hi")
	assert.Equal(t, float64(0), decoded["exit_code"])
}

func TestShellExecuteRejectsMissingCommand(t *testing.T) {
	reg := tool.NewRegistry()
	backend, _ := NewLocal(Config{})
	defer backend.Close()
	RegisterShellExecute(reg, backend)

	args := json.RawMessage(`{}`)
	result, err := reg.Dispatch(context.Background(), "shell_execute", args)
	require.NoError(t, err)
	assert.Contains(t, result, `"error"`)
	assert.Contains(t, result, "command")
}
```

- [ ] **Step 2: Create `tool/terminal/tools.go`**

```go
package terminal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
)

// shellExecuteSchema is the JSON Schema for shell_execute tool arguments.
// The LLM sees this when deciding how to call the tool.
const shellExecuteSchema = `{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "Shell command to run. Executed via /bin/sh -c or cmd /C."
    },
    "cwd": {
      "type": "string",
      "description": "Working directory for the command. Defaults to the agent's working directory."
    },
    "timeout_seconds": {
      "type": "number",
      "description": "Timeout in seconds. Default 180."
    },
    "stdin": {
      "type": "string",
      "description": "Input piped to the command's stdin."
    }
  },
  "required": ["command"]
}`

// shellExecuteArgs is the decoded argument shape.
type shellExecuteArgs struct {
	Command        string  `json:"command"`
	Cwd            string  `json:"cwd,omitempty"`
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty"`
	Stdin          string  `json:"stdin,omitempty"`
}

// shellExecuteResult is the encoded result shape returned to the LLM.
type shellExecuteResult struct {
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
	ExitCode   int     `json:"exit_code"`
	DurationMS int64   `json:"duration_ms"`
}

// RegisterShellExecute wires the shell_execute tool into a Registry using
// the provided Backend for execution.
func RegisterShellExecute(reg *tool.Registry, backend Backend) {
	reg.Register(&tool.Entry{
		Name:        "shell_execute",
		Toolset:     "terminal",
		Description: "Run a shell command on the host. Returns stdout, stderr, exit code.",
		Emoji:       "⚡",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "shell_execute",
				Description: "Run a shell command. Returns stdout, stderr, and exit code.",
				Parameters:  json.RawMessage(shellExecuteSchema),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args shellExecuteArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if args.Command == "" {
				return tool.ToolError("command is required"), nil
			}

			timeout := 180 * time.Second
			if args.TimeoutSeconds > 0 {
				timeout = time.Duration(args.TimeoutSeconds * float64(time.Second))
			}

			res, err := backend.Execute(ctx, args.Command, &ExecOptions{
				Cwd:     args.Cwd,
				Timeout: timeout,
				Stdin:   args.Stdin,
			})
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}

			return tool.ToolResult(shellExecuteResult{
				Stdout:     res.Stdout,
				Stderr:     res.Stderr,
				ExitCode:   res.ExitCode,
				DurationMS: res.Duration.Milliseconds(),
			}), nil
		},
	})
}
```

- [ ] **Step 3: Run the tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS. All previous tests plus the 3 new shell_execute tool tests.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/terminal/tools.go hermes-agent-go/tool/terminal/local_test.go
git commit -m "feat(tool): register shell_execute tool backed by Local backend"
```

---

## Task 5: File Tools — read_file + list_directory

**Files:**
- Create: `hermes-agent-go/tool/file/read.go`
- Create: `hermes-agent-go/tool/file/list.go`
- Create: `hermes-agent-go/tool/file/register.go`
- Create: `hermes-agent-go/tool/file/file_test.go`

- [ ] **Step 1: Write failing tests for read_file and list_directory**

Create `tool/file/file_test.go`:

```go
package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() *tool.Registry {
	r := tool.NewRegistry()
	RegisterAll(r)
	return r
}

func TestReadFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + path + `"}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "hello world", decoded["content"])
}

func TestReadFileMissing(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{"path":"/nonexistent/path/x.txt"}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "no such file")
}

func TestReadFileRejectsEmptyPath(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{}`)
	out, err := r.Dispatch(context.Background(), "read_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "path")
}

func TestListDirectoryHappyPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + dir + `"}`)
	out, err := r.Dispatch(context.Background(), "list_directory", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	entries, ok := decoded["entries"].([]any)
	require.True(t, ok, "entries should be an array")
	assert.Len(t, entries, 3)

	names := map[string]bool{}
	for _, e := range entries {
		m := e.(map[string]any)
		names[m["name"].(string)] = true
	}
	assert.True(t, names["a.txt"])
	assert.True(t, names["b.txt"])
	assert.True(t, names["sub"])
}

func TestListDirectoryMissing(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{"path":"/nonexistent/dir"}`)
	out, err := r.Dispatch(context.Background(), "list_directory", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}
```

- [ ] **Step 2: Create `tool/file/read.go`**

```go
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nousresearch/hermes-agent/tool"
)

const readFileSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Absolute or relative file path." }
  },
  "required": ["path"]
}`

type readFileArgs struct {
	Path string `json:"path"`
}

type readFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// readFileHandler reads the file at the given path and returns its contents.
// Files larger than 1 MiB are refused to protect the context window.
const maxReadFileBytes = 1 << 20

func readFileHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args readFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	info, err := os.Stat(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}
	if info.IsDir() {
		return tool.ToolError(fmt.Sprintf("%s is a directory, use list_directory", args.Path)), nil
	}
	if info.Size() > maxReadFileBytes {
		return tool.ToolError(fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxReadFileBytes)), nil
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(readFileResult{
		Path:    args.Path,
		Content: string(data),
		Size:    len(data),
	}), nil
}
```

- [ ] **Step 3: Create `tool/file/list.go`**

```go
package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/nousresearch/hermes-agent/tool"
)

const listDirectorySchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Directory path." }
  },
  "required": ["path"]
}`

type listDirectoryArgs struct {
	Path string `json:"path"`
}

type dirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

type listDirectoryResult struct {
	Path    string     `json:"path"`
	Entries []dirEntry `json:"entries"`
}

func listDirectoryHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args listDirectoryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	entries, err := os.ReadDir(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	out := listDirectoryResult{Path: args.Path, Entries: make([]dirEntry, 0, len(entries))}
	for _, e := range entries {
		de := dirEntry{Name: e.Name(), IsDir: e.IsDir()}
		if info, err := e.Info(); err == nil && !e.IsDir() {
			de.Size = info.Size()
		}
		out.Entries = append(out.Entries, de)
	}
	return tool.ToolResult(out), nil
}
```

- [ ] **Step 4: Create `tool/file/register.go`**

```go
package file

import (
	"encoding/json"

	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterAll adds every file tool (read_file, write_file, list_directory,
// search_files) to the registry. Call this once at startup.
func RegisterAll(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "read_file",
		Toolset:     "file",
		Description: "Read a file from the filesystem.",
		Emoji:       "📄",
		Handler:     readFileHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read the contents of a file. Max 1 MiB.",
				Parameters:  json.RawMessage(readFileSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "list_directory",
		Toolset:     "file",
		Description: "List files and subdirectories in a directory.",
		Emoji:       "📁",
		Handler:     listDirectoryHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "list_directory",
				Description: "List entries in a directory, showing name, type, and size.",
				Parameters:  json.RawMessage(listDirectorySchema),
			},
		},
	})

	// write_file and search_files added in Task 6
}
```

- [ ] **Step 5: Run the tests**

```bash
go test -race ./tool/file/...
```

Expected: PASS. All 5 read_file + list_directory tests.

- [ ] **Step 6: Commit**

```bash
git add hermes-agent-go/tool/file/
git commit -m "feat(tool): add read_file and list_directory file tools"
```

---

## Task 6: File Tools — write_file + search_files

**Files:**
- Create: `hermes-agent-go/tool/file/write.go`
- Create: `hermes-agent-go/tool/file/search.go`
- Modify: `hermes-agent-go/tool/file/register.go` (add both to RegisterAll)
- Modify: `hermes-agent-go/tool/file/file_test.go` (add tests)

- [ ] **Step 1: Append failing tests**

Append to `tool/file/file_test.go`:

```go
func TestWriteFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + path + `","content":"written"}`)
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, float64(7), decoded["bytes_written"])

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "written", string(data))
}

func TestWriteFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "deep.txt")
	r := newTestRegistry()
	args := json.RawMessage(`{"path":"` + path + `","content":"deep","create_dirs":true}`)
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.NotContains(t, out, `"error"`)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "deep", string(data))
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	r := newTestRegistry()
	args := json.RawMessage(`{"content":"nothing"}`)
	out, err := r.Dispatch(context.Background(), "write_file", args)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
}

func TestSearchFilesHappyPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644))

	r := newTestRegistry()
	args := json.RawMessage(`{"root":"` + dir + `","pattern":"*.go"}`)
	out, err := r.Dispatch(context.Background(), "search_files", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	matches, ok := decoded["matches"].([]any)
	require.True(t, ok)
	assert.Len(t, matches, 2)
}

func TestSearchFilesEmptyResults(t *testing.T) {
	dir := t.TempDir()
	r := newTestRegistry()
	args := json.RawMessage(`{"root":"` + dir + `","pattern":"*.nope"}`)
	out, err := r.Dispatch(context.Background(), "search_files", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	matches, _ := decoded["matches"].([]any)
	assert.Len(t, matches, 0)
}
```

- [ ] **Step 2: Create `tool/file/write.go`**

```go
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nousresearch/hermes-agent/tool"
)

const writeFileSchema = `{
  "type": "object",
  "properties": {
    "path":        { "type": "string", "description": "File path to write." },
    "content":     { "type": "string", "description": "Content to write." },
    "create_dirs": { "type": "boolean", "description": "Create parent directories if missing. Default false." }
  },
  "required": ["path", "content"]
}`

type writeFileArgs struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	CreateDirs bool   `json:"create_dirs"`
}

type writeFileResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

func writeFileHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args writeFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	if args.CreateDirs {
		if dir := filepath.Dir(args.Path); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return tool.ToolError(fmt.Sprintf("mkdir %s: %s", dir, err.Error())), nil
			}
		}
	}

	if err := os.WriteFile(args.Path, []byte(args.Content), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(writeFileResult{
		Path:         args.Path,
		BytesWritten: len(args.Content),
	}), nil
}
```

- [ ] **Step 3: Create `tool/file/search.go`**

```go
package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/nousresearch/hermes-agent/tool"
)

const searchFilesSchema = `{
  "type": "object",
  "properties": {
    "root":    { "type": "string", "description": "Directory to search in (recursive)." },
    "pattern": { "type": "string", "description": "Glob pattern, e.g. '*.go' or 'main.*'. Matched against filename only." }
  },
  "required": ["root", "pattern"]
}`

type searchFilesArgs struct {
	Root    string `json:"root"`
	Pattern string `json:"pattern"`
}

type searchMatch struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type searchFilesResult struct {
	Root    string        `json:"root"`
	Pattern string        `json:"pattern"`
	Matches []searchMatch `json:"matches"`
}

const maxSearchMatches = 500

func searchFilesHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args searchFilesArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Root == "" || args.Pattern == "" {
		return tool.ToolError("root and pattern are required"), nil
	}

	var matches []searchMatch
	err := filepath.WalkDir(args.Root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries, keep walking
		}
		if d.IsDir() {
			return nil
		}
		ok, matchErr := filepath.Match(args.Pattern, d.Name())
		if matchErr != nil {
			return matchErr
		}
		if !ok {
			return nil
		}
		info, _ := d.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		matches = append(matches, searchMatch{Path: path, Size: size})
		if len(matches) >= maxSearchMatches {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if matches == nil {
		matches = []searchMatch{}
	}
	return tool.ToolResult(searchFilesResult{
		Root:    args.Root,
		Pattern: args.Pattern,
		Matches: matches,
	}), nil
}
```

- [ ] **Step 4: Update `tool/file/register.go` to include the two new tools**

Replace the entire `RegisterAll` function with:

```go
// RegisterAll adds every file tool (read_file, write_file, list_directory,
// search_files) to the registry. Call this once at startup.
func RegisterAll(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "read_file",
		Toolset:     "file",
		Description: "Read a file from the filesystem.",
		Emoji:       "📄",
		Handler:     readFileHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read the contents of a file. Max 1 MiB.",
				Parameters:  json.RawMessage(readFileSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "write_file",
		Toolset:     "file",
		Description: "Write content to a file.",
		Emoji:       "✏️",
		Handler:     writeFileHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "write_file",
				Description: "Write content to a file, overwriting if it exists.",
				Parameters:  json.RawMessage(writeFileSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "list_directory",
		Toolset:     "file",
		Description: "List files and subdirectories in a directory.",
		Emoji:       "📁",
		Handler:     listDirectoryHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "list_directory",
				Description: "List entries in a directory, showing name, type, and size.",
				Parameters:  json.RawMessage(listDirectorySchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "search_files",
		Toolset:     "file",
		Description: "Recursively search for files by glob pattern.",
		Emoji:       "🔍",
		Handler:     searchFilesHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "search_files",
				Description: "Recursively find files in a directory matching a glob pattern.",
				Parameters:  json.RawMessage(searchFilesSchema),
			},
		},
	})
}
```

- [ ] **Step 5: Run tests**

```bash
go test -race ./tool/file/...
```

Expected: PASS. All file tests (9 total now).

- [ ] **Step 6: Commit**

```bash
git add hermes-agent-go/tool/file/
git commit -m "feat(tool): add write_file and search_files file tools"
```

---

## Task 7: Extend Provider.Request with Tools + Anthropic Wire Types for Tool Use

**Files:**
- Modify: `hermes-agent-go/provider/provider.go`
- Modify: `hermes-agent-go/provider/anthropic/types.go`

- [ ] **Step 1: Add `Tools` field to `provider.Request`**

In `provider/provider.go`, modify `Request` struct from:

```go
type Request struct {
	Model         string
	SystemPrompt  string
	Messages      []message.Message
	MaxTokens     int
	Temperature   *float64
	TopP          *float64
	StopSequences []string
}
```

to:

```go
type Request struct {
	Model         string
	SystemPrompt  string
	Messages      []message.Message
	// Tools is the set of tool definitions the LLM may invoke. May be empty.
	Tools         []tool.ToolDefinition
	MaxTokens     int
	Temperature   *float64
	TopP          *float64
	StopSequences []string
}
```

Add `"github.com/nousresearch/hermes-agent/tool"` to the imports of `provider.go`.

**Important:** This creates a new dependency direction: `provider` → `tool`. That's fine because `tool` does not import `provider`. Keep the dependency graph acyclic.

- [ ] **Step 2: Extend the Anthropic wire types to carry tool info**

In `provider/anthropic/types.go`, modify `messagesRequest` from:

```go
type messagesRequest struct {
	Model         string          `json:"model"`
	Messages      []apiMessage    `json:"messages"`
	System        string          `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
}
```

to:

```go
type messagesRequest struct {
	Model         string          `json:"model"`
	Messages      []apiMessage    `json:"messages"`
	System        string          `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Tools         []anthropicTool `json:"tools,omitempty"`
}

// anthropicTool is the Anthropic wire format for tool definitions.
// Ref: https://docs.anthropic.com/en/api/tool-use
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}
```

Also modify `apiContentItem` from:

```go
type apiContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
```

to:

```go
// apiContentItem represents one element of an Anthropic message content array.
// Anthropic supports "text", "image", "tool_use", "tool_result".
type apiContentItem struct {
	Type string `json:"type"`

	// text
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}
```

Add `"encoding/json"` to the imports of `types.go` if not already present.

- [ ] **Step 3: Build to check compilation**

```bash
go build ./provider/...
```

Expected: success. No tests yet — they come in Tasks 8 and 9.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/provider.go hermes-agent-go/provider/anthropic/types.go
git commit -m "feat(provider): add Tools field to Request + Anthropic tool wire types"
```

---

## Task 8: Anthropic Complete Handles Tools

**Files:**
- Modify: `hermes-agent-go/provider/anthropic/complete.go`
- Modify: `hermes-agent-go/provider/anthropic/anthropic_test.go`

- [ ] **Step 1: Append failing test for tool_use in Complete**

Append to `provider/anthropic/anthropic_test.go`:

```go
func TestCompleteEmitsTools(t *testing.T) {
	var capturedReq messagesRequest
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &capturedReq))

		resp := messagesResponse{
			ID:    "msg_01",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
			Content: []apiContentItem{
				{Type: "text", Text: "I'll read the file."},
				{Type: "tool_use", ID: "tool_01", Name: "read_file", Input: json.RawMessage(`{"path":"go.mod"}`)},
			},
			StopReason: "tool_use",
			Usage:      apiUsage{InputTokens: 20, OutputTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := &provider.Request{
		Model: "claude-opus-4-6",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		},
		Tools: []tool.ToolDefinition{
			{Type: "function", Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read a file",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
			}},
		},
		MaxTokens: 1024,
	}

	resp, err := a.Complete(context.Background(), req)
	require.NoError(t, err)

	// Request should have passed tools
	require.Len(t, capturedReq.Tools, 1)
	assert.Equal(t, "read_file", capturedReq.Tools[0].Name)
	assert.Equal(t, "Read a file", capturedReq.Tools[0].Description)

	// Response should have tool_use block and finish_reason
	assert.Equal(t, "tool_use", resp.FinishReason)
	require.Equal(t, message.RoleAssistant, resp.Message.Role)
	blocks := resp.Message.Content.Blocks()
	require.Len(t, blocks, 2)
	assert.Equal(t, "text", blocks[0].Type)
	assert.Equal(t, "I'll read the file.", blocks[0].Text)
	assert.Equal(t, "tool_use", blocks[1].Type)
	assert.Equal(t, "tool_01", blocks[1].ToolUseID)
	assert.Equal(t, "read_file", blocks[1].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[1].ToolUseInput))
}

func TestCompleteSendsToolResult(t *testing.T) {
	var capturedReq messagesRequest
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &capturedReq))

		resp := messagesResponse{
			ID: "msg_02", Type: "message", Role: "assistant", Model: "claude-opus-4-6",
			Content:    []apiContentItem{{Type: "text", Text: "Done."}},
			StopReason: "end_turn",
			Usage:      apiUsage{InputTokens: 30, OutputTokens: 3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// History: user message, assistant tool_use, user tool_result
	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		{
			Role: message.RoleAssistant,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:         "tool_use",
					ToolUseID:    "tool_01",
					ToolUseName:  "read_file",
					ToolUseInput: json.RawMessage(`{"path":"go.mod"}`),
				},
			}),
		},
		{
			Role: message.RoleUser,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:       "tool_result",
					ToolUseID:  "tool_01",
					ToolResult: `{"content":"module x"}`,
				},
			}),
		},
	}

	req := &provider.Request{Model: "claude-opus-4-6", Messages: history, MaxTokens: 1024}
	resp, err := a.Complete(context.Background(), req)
	require.NoError(t, err)

	// Verify the request sent to the server has all 3 messages with correct content types
	require.Len(t, capturedReq.Messages, 3)
	assert.Equal(t, "user", capturedReq.Messages[0].Role)
	assert.Equal(t, "assistant", capturedReq.Messages[1].Role)
	assert.Equal(t, "user", capturedReq.Messages[2].Role)

	// Assistant turn should contain a tool_use block
	assistantContent := capturedReq.Messages[1].Content
	require.Len(t, assistantContent, 1)
	assert.Equal(t, "tool_use", assistantContent[0].Type)
	assert.Equal(t, "tool_01", assistantContent[0].ID)

	// User turn 3 should contain a tool_result block
	userResult := capturedReq.Messages[2].Content
	require.Len(t, userResult, 1)
	assert.Equal(t, "tool_result", userResult[0].Type)
	assert.Equal(t, "tool_01", userResult[0].ToolUseID)

	assert.Equal(t, "end_turn", resp.FinishReason)
}
```

You'll need these imports in `anthropic_test.go` if they're not already there:
```go
import (
	// ... existing ...
	"github.com/nousresearch/hermes-agent/tool"
)
```

- [ ] **Step 2: Update `buildRequest` in `complete.go` to emit tools**

Find the existing `buildRequest` function and add tool handling. The full updated function:

```go
// buildRequest converts a provider.Request to the Anthropic wire format.
func (a *Anthropic) buildRequest(req *provider.Request, stream bool) *messagesRequest {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := &messagesRequest{
		Model:         req.Model,
		System:        req.SystemPrompt,
		MaxTokens:     maxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences,
		Stream:        stream,
		Messages:      make([]apiMessage, 0, len(req.Messages)),
	}

	// Convert tools
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			apiReq.Tools = append(apiReq.Tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
	}

	// Convert messages
	for _, m := range req.Messages {
		role := string(m.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		apiReq.Messages = append(apiReq.Messages, apiMessage{
			Role:    role,
			Content: contentToAPIItems(m.Content),
		})
	}
	return apiReq
}
```

- [ ] **Step 3: Update `contentToAPIItems` to handle tool_use and tool_result blocks**

Find the existing `contentToAPIItems` function and replace it with:

```go
// contentToAPIItems converts message.Content to Anthropic's content array format.
// Handles text, tool_use, and tool_result blocks.
func contentToAPIItems(c message.Content) []apiContentItem {
	if c.IsText() {
		return []apiContentItem{{Type: "text", Text: c.Text()}}
	}
	items := make([]apiContentItem, 0, len(c.Blocks()))
	for _, b := range c.Blocks() {
		switch b.Type {
		case "text":
			items = append(items, apiContentItem{Type: "text", Text: b.Text})
		case "tool_use":
			items = append(items, apiContentItem{
				Type:  "tool_use",
				ID:    b.ToolUseID,
				Name:  b.ToolUseName,
				Input: b.ToolUseInput,
			})
		case "tool_result":
			items = append(items, apiContentItem{
				Type:      "tool_result",
				ToolUseID: b.ToolUseID,
				Content:   b.ToolResult,
			})
		}
	}
	return items
}
```

- [ ] **Step 4: Update `convertResponse` to parse tool_use blocks**

Find `convertResponse` and replace it:

```go
// convertResponse converts an Anthropic wire response to the provider shape.
// If the response contains any tool_use blocks, Content is returned as
// BlockContent preserving all blocks. Otherwise, Content is TextContent
// with all text concatenated.
func (a *Anthropic) convertResponse(apiResp *messagesResponse) *provider.Response {
	hasToolUse := false
	for _, c := range apiResp.Content {
		if c.Type == "tool_use" {
			hasToolUse = true
			break
		}
	}

	var content message.Content
	if hasToolUse {
		blocks := make([]message.ContentBlock, 0, len(apiResp.Content))
		for _, c := range apiResp.Content {
			switch c.Type {
			case "text":
				blocks = append(blocks, message.ContentBlock{Type: "text", Text: c.Text})
			case "tool_use":
				blocks = append(blocks, message.ContentBlock{
					Type:         "tool_use",
					ToolUseID:    c.ID,
					ToolUseName:  c.Name,
					ToolUseInput: c.Input,
				})
			}
		}
		content = message.BlockContent(blocks)
	} else {
		var text strings.Builder
		for _, c := range apiResp.Content {
			if c.Type == "text" {
				text.WriteString(c.Text)
			}
		}
		content = message.TextContent(text.String())
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: content,
		},
		FinishReason: apiResp.StopReason,
		Usage: message.Usage{
			InputTokens:      apiResp.Usage.InputTokens,
			OutputTokens:     apiResp.Usage.OutputTokens,
			CacheReadTokens:  apiResp.Usage.CacheReadInputTokens,
			CacheWriteTokens: apiResp.Usage.CacheCreationInputTokens,
		},
		Model: apiResp.Model,
	}
}
```

Add `"strings"` to the imports of `complete.go` if not already present.

- [ ] **Step 5: Run the tests**

```bash
go test -race ./provider/anthropic/...
```

Expected: PASS. All previous tests plus the two new tool tests.

- [ ] **Step 6: Commit**

```bash
git add hermes-agent-go/provider/anthropic/complete.go hermes-agent-go/provider/anthropic/anthropic_test.go
git commit -m "feat(anthropic): Complete handles tool_use and tool_result blocks"
```

---

## Task 9: Anthropic Stream Handles Tool Use

**Files:**
- Modify: `hermes-agent-go/provider/anthropic/stream.go`
- Modify: `hermes-agent-go/provider/anthropic/anthropic_test.go`

- [ ] **Step 1: Append failing streaming tool_use test**

Append to `anthropic_test.go`:

```go
func TestStreamToolUse(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		// SSE sequence for a tool_use response
		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_03\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":20,\"output_tokens\":0}}}\n\n",
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_02\",\"name\":\"read_file\",\"input\":{}}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"go.mod\\\"}\"}}\n\n",
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":8}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := a.Stream(context.Background(), &provider.Request{
		Model: "claude-opus-4-6",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		},
	})
	require.NoError(t, err)
	defer stream.Close()

	var doneEvent *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.Type == provider.EventDone {
			doneEvent = ev
			break
		}
	}

	require.NotNil(t, doneEvent)
	require.NotNil(t, doneEvent.Response)
	assert.Equal(t, "tool_use", doneEvent.Response.FinishReason)

	blocks := doneEvent.Response.Message.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "tool_02", blocks[0].ToolUseID)
	assert.Equal(t, "read_file", blocks[0].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[0].ToolUseInput))
}
```

- [ ] **Step 2: Update `anthropicStream` to track tool_use blocks**

Replace the existing `anthropicStream` struct and its methods with the updated version. In `stream.go`, modify the struct:

```go
// anthropicStream implements provider.Stream.
// NOT thread-safe. One consumer only.
type anthropicStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	// accumulated state
	text         strings.Builder
	model        string
	finishReason string
	usage        message.Usage
	done         bool
	closed       bool

	// tool_use accumulator: index → partial tool_use block
	toolUses       map[int]*toolUseBuilder
	blockOrder     []int // order blocks were opened
}

// toolUseBuilder accumulates a streaming tool_use block across events.
type toolUseBuilder struct {
	ID         string
	Name       string
	InputJSON  strings.Builder // partial_json accumulates here
}
```

And update `handleEvent` to include tool_use cases. Find the existing `handleEvent` function and replace the relevant switch cases. Full updated function:

```go
// handleEvent dispatches an SSE event to the right handler based on its type.
// Returns a non-nil StreamEvent to surface to the caller, or nil to continue scanning.
func (s *anthropicStream) handleEvent(eventType string, data []byte) (*provider.StreamEvent, error) {
	switch eventType {
	case anthropicEventMessageStart:
		var ev struct {
			Message messagesResponse `json:"message"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse message_start: %w", err)
		}
		s.model = ev.Message.Model
		s.usage.InputTokens = ev.Message.Usage.InputTokens
		if s.toolUses == nil {
			s.toolUses = make(map[int]*toolUseBuilder)
		}
		return nil, nil

	case anthropicEventContentBlockStart:
		var ev struct {
			Index        int `json:"index"`
			ContentBlock struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse content_block_start: %w", err)
		}
		if ev.ContentBlock.Type == "tool_use" {
			if s.toolUses == nil {
				s.toolUses = make(map[int]*toolUseBuilder)
			}
			s.toolUses[ev.Index] = &toolUseBuilder{
				ID:   ev.ContentBlock.ID,
				Name: ev.ContentBlock.Name,
			}
			s.blockOrder = append(s.blockOrder, ev.Index)
		}
		return nil, nil

	case anthropicEventContentBlockDelta:
		var ev struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse delta: %w", err)
		}

		switch ev.Delta.Type {
		case "text_delta":
			if ev.Delta.Text != "" {
				s.text.WriteString(ev.Delta.Text)
				return &provider.StreamEvent{
					Type:  provider.EventDelta,
					Delta: &provider.StreamDelta{Content: ev.Delta.Text},
				}, nil
			}
		case "input_json_delta":
			// Accumulate into the matching tool_use builder
			if b, ok := s.toolUses[ev.Index]; ok {
				b.InputJSON.WriteString(ev.Delta.PartialJSON)
			}
		}
		return nil, nil

	case anthropicEventContentBlockStop:
		return nil, nil

	case anthropicEventMessageDelta:
		var ev struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage apiUsage `json:"usage"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse message_delta: %w", err)
		}
		if ev.Delta.StopReason != "" {
			s.finishReason = ev.Delta.StopReason
		}
		s.usage.OutputTokens = ev.Usage.OutputTokens
		return nil, nil

	case anthropicEventMessageStop:
		s.done = true
		return s.buildDoneEvent(), nil

	case anthropicEventError:
		return nil, &provider.Error{
			Kind:     provider.ErrUnknown,
			Provider: "anthropic",
			Message:  "stream error event: " + string(data),
		}

	case anthropicEventPing:
		return nil, nil

	default:
		return nil, nil
	}
}
```

- [ ] **Step 3: Update `buildDoneEvent` to emit BlockContent when tool_use present**

Replace the existing `buildDoneEvent`:

```go
// buildDoneEvent creates the terminal EventDone with accumulated state.
// If any tool_use blocks were collected, the response content is returned
// as BlockContent preserving all blocks; otherwise it's a plain TextContent.
func (s *anthropicStream) buildDoneEvent() *provider.StreamEvent {
	var content message.Content

	if len(s.toolUses) > 0 {
		blocks := make([]message.ContentBlock, 0, 1+len(s.toolUses))
		// Preserve text first if any
		if s.text.Len() > 0 {
			blocks = append(blocks, message.ContentBlock{
				Type: "text",
				Text: s.text.String(),
			})
		}
		// Then tool_use blocks in the order they were opened
		for _, idx := range s.blockOrder {
			b := s.toolUses[idx]
			if b == nil {
				continue
			}
			input := json.RawMessage(b.InputJSON.String())
			// If no partial_json was ever received, fall back to empty object
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    b.ID,
				ToolUseName:  b.Name,
				ToolUseInput: input,
			})
		}
		content = message.BlockContent(blocks)
	} else {
		content = message.TextContent(s.text.String())
	}

	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: content,
			},
			FinishReason: s.finishReason,
			Usage:        s.usage,
			Model:        s.model,
		},
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./provider/anthropic/...
```

Expected: PASS. All previous tests plus `TestStreamToolUse`.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/provider/anthropic/stream.go hermes-agent-go/provider/anthropic/anthropic_test.go
git commit -m "feat(anthropic): Stream handles tool_use content blocks"
```

---

## Task 10: Engine Multi-Turn Loop with Tool Execution

**Files:**
- Modify: `hermes-agent-go/agent/engine.go`
- Modify: `hermes-agent-go/agent/conversation.go`
- Modify: `hermes-agent-go/agent/engine_test.go`

- [ ] **Step 1: Append failing tests for multi-turn conversation with tools**

Append to `agent/engine_test.go`:

```go
import (
	// ... existing imports
	"github.com/nousresearch/hermes-agent/tool"
)

// newFakeProviderForScript replays a fixed sequence of responses on each Stream call.
func newFakeProviderForScript(responses []*provider.Response) *fakeProvider {
	idx := 0
	return &fakeProvider{
		name: "fake",
		streamFn: func() (provider.Stream, error) {
			if idx >= len(responses) {
				return nil, errors.New("unexpected extra stream call")
			}
			resp := responses[idx]
			idx++
			return &fakeStream{
				events: []*provider.StreamEvent{
					{Type: provider.EventDone, Response: resp},
				},
			}, nil
		},
	}
}

func TestEngineToolLoopSingleToolCall(t *testing.T) {
	// Prepare a registry with a fake "echo_args" tool
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo_args",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"echoed":true}`, nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "echo_args",
				Description: "Echo the arguments back",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	// Provider returns: turn 1 = tool_use, turn 2 = final text
	responses := []*provider.Response{
		{
			Message: message.Message{
				Role: message.RoleAssistant,
				Content: message.BlockContent([]message.ContentBlock{
					{
						Type:         "tool_use",
						ToolUseID:    "t1",
						ToolUseName:  "echo_args",
						ToolUseInput: json.RawMessage(`{}`),
					},
				}),
			},
			FinishReason: "tool_use",
			Usage:        message.Usage{InputTokens: 10, OutputTokens: 5},
		},
		{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent("Done. Got echoed=true."),
			},
			FinishReason: "end_turn",
			Usage:        message.Usage{InputTokens: 15, OutputTokens: 8},
		},
	}

	p := newFakeProviderForScript(responses)
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "run echo",
		SessionID:   "tool-test",
	})
	require.NoError(t, err)
	assert.Equal(t, "Done. Got echoed=true.", result.Response.Content.Text())
	assert.Equal(t, 2, result.Iterations)

	// History should have: user, assistant(tool_use), user(tool_result), assistant(text) = 4
	require.Len(t, result.Messages, 4)
	assert.Equal(t, message.RoleUser, result.Messages[0].Role)
	assert.Equal(t, message.RoleAssistant, result.Messages[1].Role)
	assert.Equal(t, message.RoleUser, result.Messages[2].Role) // tool_result
	assert.Equal(t, message.RoleAssistant, result.Messages[3].Role)

	// The tool_result message should be a BlockContent with a tool_result block
	require.False(t, result.Messages[2].Content.IsText())
	blocks := result.Messages[2].Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_result", blocks[0].Type)
	assert.Equal(t, "t1", blocks[0].ToolUseID)
	assert.Contains(t, blocks[0].ToolResult, "echoed")
}

func TestEngineBudgetExhaustion(t *testing.T) {
	// Provider that ALWAYS returns a tool_use — should exhaust budget
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "loop_tool",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:       "loop_tool",
				Parameters: json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	toolUseResp := &provider.Response{
		Message: message.Message{
			Role: message.RoleAssistant,
			Content: message.BlockContent([]message.ContentBlock{
				{Type: "tool_use", ToolUseID: "t1", ToolUseName: "loop_tool", ToolUseInput: json.RawMessage(`{}`)},
			}),
		},
		FinishReason: "tool_use",
	}

	// Script of 10 tool_use responses — budget is 3, so only 3 should execute
	responses := []*provider.Response{toolUseResp, toolUseResp, toolUseResp, toolUseResp, toolUseResp}
	p := newFakeProviderForScript(responses)

	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 3}, "cli")
	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "loop forever",
		SessionID:   "budget-test",
	})
	// Budget exhaustion is not an error — it returns the partial result
	require.NoError(t, err)
	assert.Equal(t, 3, result.Iterations, "should run exactly MaxTurns iterations")
}
```

- [ ] **Step 2: Update `Engine` to hold a tool registry**

In `agent/engine.go`, modify the `Engine` struct:

```go
// Engine is single-use per conversation. NOT thread-safe.
// The gateway creates a fresh Engine per incoming message.
// The CLI creates a fresh Engine per /run invocation.
type Engine struct {
	provider provider.Provider
	storage  storage.Storage
	tools    *tool.Registry      // may be nil if no tools are available
	config   config.AgentConfig  // value, not pointer — immutable snapshot
	platform string
	prompt   *PromptBuilder

	// Callbacks — optional. Nil means no-op.
	onStreamDelta func(delta *provider.StreamDelta)
	onToolStart   func(call message.ContentBlock) // fired before tool execution
	onToolResult  func(call message.ContentBlock, result string) // fired after
}
```

Replace `NewEngine` with two constructors:

```go
// NewEngine constructs an Engine without tools. Use NewEngineWithTools if
// you want the LLM to be able to invoke tools.
func NewEngine(p provider.Provider, s storage.Storage, cfg config.AgentConfig, platform string) *Engine {
	return NewEngineWithTools(p, s, nil, cfg, platform)
}

// NewEngineWithTools constructs an Engine with a tool registry.
// If tools is nil, the engine behaves exactly like NewEngine.
func NewEngineWithTools(p provider.Provider, s storage.Storage, tools *tool.Registry, cfg config.AgentConfig, platform string) *Engine {
	return &Engine{
		provider: p,
		storage:  s,
		tools:    tools,
		config:   cfg,
		platform: platform,
		prompt:   NewPromptBuilder(platform),
	}
}
```

Add callback setters:

```go
// SetToolStartCallback registers a callback invoked before each tool execution.
func (e *Engine) SetToolStartCallback(fn func(call message.ContentBlock)) {
	e.onToolStart = fn
}

// SetToolResultCallback registers a callback invoked after each tool execution.
func (e *Engine) SetToolResultCallback(fn func(call message.ContentBlock, result string)) {
	e.onToolResult = fn
}
```

Add the required imports to `engine.go`:
```go
import (
	// ... existing ...
	"github.com/nousresearch/hermes-agent/tool"
)
```

- [ ] **Step 3: Rewrite `RunConversation` as a budget-enforced loop**

Replace the entire `RunConversation` function in `conversation.go`:

```go
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
)

// RunConversation runs a conversation turn — or multiple turns if the LLM
// issues tool calls. Each LLM call is one turn. The loop continues until:
//   (1) the LLM responds without any tool_use blocks (final answer),
//   (2) the iteration budget is exhausted,
//   (3) the context is canceled,
//   (4) the provider returns a non-retryable error.
//
// Single-turn (no tools) behavior matches Plan 1 exactly.
func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model := opts.Model
	if model == "" {
		model = "claude-opus-4-6"
	}

	// Copy caller's history so we don't mutate it
	history := append([]message.Message{}, opts.History...)
	history = append(history, message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent(opts.UserMessage),
	})

	systemPrompt := e.prompt.Build(&PromptOptions{Model: model})

	// Persist the session + the incoming user message (if storage is configured)
	if e.storage != nil {
		if err := e.ensureSession(ctx, opts, systemPrompt, model); err != nil {
			return nil, fmt.Errorf("engine: ensure session: %w", err)
		}
		if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
			return nil, fmt.Errorf("engine: persist user message: %w", err)
		}
	}

	// Collect tool definitions from the registry (empty slice if nil)
	var toolDefs []tool.ToolDefinition
	if e.tools != nil {
		toolDefs = e.tools.Definitions(nil)
	}

	budget := NewBudget(e.config.MaxTurns)
	totalUsage := message.Usage{}
	iterations := 0
	var lastResponse message.Message

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !budget.Consume() {
			// Budget exhausted — return what we have so far
			break
		}
		iterations++

		req := &provider.Request{
			Model:        model,
			SystemPrompt: systemPrompt,
			Messages:     history,
			Tools:        toolDefs,
			MaxTokens:    4096,
		}

		resp, err := e.streamOnce(ctx, req)
		if err != nil {
			return nil, err
		}

		// Append assistant response to history
		history = append(history, resp.Message)
		lastResponse = resp.Message
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.CacheReadTokens += resp.Usage.CacheReadTokens
		totalUsage.CacheWriteTokens += resp.Usage.CacheWriteTokens

		// Persist the assistant message + usage atomically (if storage configured)
		if e.storage != nil {
			respCopy := resp // capture for closure
			txErr := e.storage.WithTx(ctx, func(tx storage.Tx) error {
				m := &history[len(history)-1]
				if err := e.persistMessageTx(ctx, tx, opts.SessionID, m); err != nil {
					return err
				}
				return tx.UpdateUsage(ctx, opts.SessionID, &storage.UsageUpdate{
					InputTokens:      respCopy.Usage.InputTokens,
					OutputTokens:     respCopy.Usage.OutputTokens,
					CacheReadTokens:  respCopy.Usage.CacheReadTokens,
					CacheWriteTokens: respCopy.Usage.CacheWriteTokens,
				})
			})
			if txErr != nil {
				return nil, fmt.Errorf("engine: persist response: %w", txErr)
			}
		}

		// Extract tool_use blocks from the response
		toolCalls := extractToolCalls(resp.Message.Content)
		if len(toolCalls) == 0 {
			// No tool calls → this is the final answer
			break
		}

		// Execute tool calls sequentially (Plan 5 adds parallelism)
		toolResults := e.executeToolCalls(ctx, toolCalls)

		// Append tool results as a user message with tool_result blocks
		toolResultMsg := message.Message{
			Role:    message.RoleUser,
			Content: message.BlockContent(toolResults),
		}
		history = append(history, toolResultMsg)

		if e.storage != nil {
			if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
				return nil, fmt.Errorf("engine: persist tool result: %w", err)
			}
		}
	}

	return &ConversationResult{
		Response:   lastResponse,
		Messages:   history,
		SessionID:  opts.SessionID,
		Usage:      totalUsage,
		Iterations: iterations,
	}, nil
}

// streamOnce runs a single provider stream and collects the full response.
// Fires the onStreamDelta callback for each delta.
func (e *Engine) streamOnce(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	stream, err := e.provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("engine: start stream: %w", err)
	}
	defer stream.Close()

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ev, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return nil, errors.New("engine: stream ended without a done event")
			}
			return nil, fmt.Errorf("engine: stream recv: %w", recvErr)
		}
		if ev == nil {
			continue
		}
		switch ev.Type {
		case provider.EventDelta:
			if e.onStreamDelta != nil && ev.Delta != nil {
				e.onStreamDelta(ev.Delta)
			}
		case provider.EventDone:
			if ev.Response == nil {
				return nil, errors.New("engine: done event has nil response")
			}
			return ev.Response, nil
		case provider.EventError:
			return nil, ev.Err
		}
	}
}

// extractToolCalls returns all tool_use blocks from a content union.
// If the content is plain text, returns nil.
func extractToolCalls(c message.Content) []message.ContentBlock {
	if c.IsText() {
		return nil
	}
	var calls []message.ContentBlock
	for _, b := range c.Blocks() {
		if b.Type == "tool_use" {
			calls = append(calls, b)
		}
	}
	return calls
}

// executeToolCalls dispatches each tool call through the registry and
// returns the results as tool_result content blocks.
// If the registry is nil, returns error results for every call.
func (e *Engine) executeToolCalls(ctx context.Context, calls []message.ContentBlock) []message.ContentBlock {
	results := make([]message.ContentBlock, 0, len(calls))
	for _, call := range calls {
		if e.onToolStart != nil {
			e.onToolStart(call)
		}

		var result string
		if e.tools == nil {
			result = `{"error":"no tool registry configured"}`
		} else {
			out, err := e.tools.Dispatch(ctx, call.ToolUseName, call.ToolUseInput)
			if err != nil {
				result = fmt.Sprintf(`{"error":"dispatch failed: %s"}`, err.Error())
			} else {
				result = out
			}
		}

		if e.onToolResult != nil {
			e.onToolResult(call, result)
		}

		results = append(results, message.ContentBlock{
			Type:       "tool_result",
			ToolUseID:  call.ToolUseID,
			ToolResult: result,
		})
	}
	return results
}

// ensureSession creates a new session row if it doesn't exist yet.
func (e *Engine) ensureSession(ctx context.Context, opts *RunOptions, systemPrompt, model string) error {
	_, err := e.storage.GetSession(ctx, opts.SessionID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	return e.storage.CreateSession(ctx, &storage.Session{
		ID:           opts.SessionID,
		Source:       e.platform,
		UserID:       opts.UserID,
		Model:        model,
		SystemPrompt: systemPrompt,
		StartedAt:    time.Now().UTC(),
	})
}

// persistMessage writes a single message outside a transaction.
func (e *Engine) persistMessage(ctx context.Context, sessionID string, m *message.Message) error {
	stored, err := storedFromMessage(sessionID, m)
	if err != nil {
		return err
	}
	return e.storage.AddMessage(ctx, sessionID, stored)
}

// persistMessageTx writes a single message inside an existing transaction.
func (e *Engine) persistMessageTx(ctx context.Context, tx storage.Tx, sessionID string, m *message.Message) error {
	stored, err := storedFromMessage(sessionID, m)
	if err != nil {
		return err
	}
	return tx.AddMessage(ctx, sessionID, stored)
}

// storedFromMessage converts a message.Message to a storage.StoredMessage.
func storedFromMessage(sessionID string, m *message.Message) (*storage.StoredMessage, error) {
	contentJSON, err := m.Content.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("engine: marshal content: %w", err)
	}
	return &storage.StoredMessage{
		SessionID:    sessionID,
		Role:         string(m.Role),
		Content:      string(contentJSON),
		ToolCallID:   m.ToolCallID,
		ToolName:     m.ToolName,
		Timestamp:    time.Now().UTC(),
		FinishReason: m.FinishReason,
		Reasoning:    m.Reasoning,
	}, nil
}
```

Add the new imports to `conversation.go`:
```go
import (
	// ... existing ...
	"github.com/nousresearch/hermes-agent/tool"
)
```

- [ ] **Step 4: Update the existing `TestEngineSingleTurn` test**

The existing `TestEngineSingleTurn` still needs to pass (single-turn = loop exits after one iteration). Verify it works unchanged. No edits needed — the Plan 1 test should still pass with the new loop because the fake provider returns a non-tool-use response, which exits the loop after iteration 1.

- [ ] **Step 5: Run all agent tests**

```bash
go test -race ./agent/...
```

Expected: PASS. All existing tests (budget, prompt, single-turn, cancellation) plus the two new tool-loop tests.

- [ ] **Step 6: Verify the whole module still builds**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 7: Commit**

```bash
git add hermes-agent-go/agent/
git commit -m "feat(agent): add tool-use loop with budget enforcement"
```

---

## Task 11: Wire Tools into the REPL

**Files:**
- Modify: `hermes-agent-go/cli/repl.go`

- [ ] **Step 1: Update `runREPL` to register tools and wire them into the engine**

In `cli/repl.go`, update the imports:

```go
import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/nousresearch/hermes-agent/tool/file"
	"github.com/nousresearch/hermes-agent/tool/terminal"
)
```

Find the existing `runREPL` function and modify it. Locate the block where the provider is created, and **after** that block, **before** the main REPL loop, insert tool registration:

```go
	// Register built-in tools
	toolRegistry := tool.NewRegistry()
	file.RegisterAll(toolRegistry)
	localBackend, err := terminal.NewLocal(terminal.Config{})
	if err != nil {
		return fmt.Errorf("hermes: create terminal backend: %w", err)
	}
	defer localBackend.Close()
	terminal.RegisterShellExecute(toolRegistry, localBackend)
```

Then find the line in the REPL loop where `agent.NewEngine` is called and replace it with `agent.NewEngineWithTools`. Also wire the tool-start and tool-result callbacks to print progress:

```go
		// Build engine fresh per turn (single-use semantics)
		engine := agent.NewEngineWithTools(p, app.Storage, toolRegistry, app.Config.Agent, "cli")

		// Register streaming callback: print deltas as they arrive
		engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
			if d != nil && d.Content != "" {
				fmt.Print(d.Content)
			}
		})
		// Tool start: print the tool call header
		engine.SetToolStartCallback(func(call message.ContentBlock) {
			fmt.Printf("\n⚡ %s: %s\n", call.ToolUseName, string(call.ToolUseInput))
		})
		// Tool result: print a truncated snippet of the result
		engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
			snippet := result
			if len(snippet) > 300 {
				snippet = snippet[:300] + "\n... [truncated]"
			}
			// Indent each line with "│ " and mark the end with "└"
			lines := strings.Split(snippet, "\n")
			for _, line := range lines {
				fmt.Printf("│ %s\n", line)
			}
			fmt.Println("└")
		})
```

- [ ] **Step 2: Update session summary to include tool call count**

Find the session summary print at the end of `runREPL` and update it to count tool calls:

```go
	// Session summary
	toolCallCount := 0
	for _, m := range history {
		// history isn't accessible here — we'll iterate the last result instead
	}

	// Simpler approach: track total tool calls in the REPL loop
```

Actually, the clean approach is to track tool calls in the loop. Add a counter before the loop:

```go
	var history []message.Message
	turnCount := 0
	toolCallCount := 0
	totalUsage := message.Usage{}
```

And inside the loop, after the result:

```go
		// Count tool calls in this turn's messages beyond the existing history
		newMessages := result.Messages[len(history):]
		for _, m := range newMessages {
			if !m.Content.IsText() {
				for _, b := range m.Content.Blocks() {
					if b.Type == "tool_use" {
						toolCallCount++
					}
				}
			}
		}
		history = result.Messages
```

And finally update the session summary print:

```go
	// Session summary
	fmt.Printf("\nSession #%s: %d messages, %d tool calls, %d in / %d out tokens · saved to %s\n",
		sessionID[:8], turnCount*2, toolCallCount,
		totalUsage.InputTokens, totalUsage.OutputTokens,
		app.Config.Storage.SQLitePath,
	)
```

- [ ] **Step 3: Build and verify it compiles**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go build ./...
```

Expected: success.

- [ ] **Step 4: Run all tests**

```bash
go test -race ./...
```

Expected: ALL tests in ALL packages pass. The existing `TestEndToEndSingleTurn` in `cli/repl_test.go` should still pass because it uses the engine directly, not through runREPL.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/cli/repl.go
git commit -m "feat(cli): register file and shell tools in REPL + tool call UI"
```

---

## Task 12: End-to-End Tool Loop Test

**Files:**
- Create: `hermes-agent-go/cli/repl_tool_test.go`

- [ ] **Step 1: Write a full-stack tool loop test**

Create `cli/repl_tool_test.go`:

```go
package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/nousresearch/hermes-agent/tool/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndToolLoop verifies the full stack: LLM (mocked) issues a tool
// call, the engine dispatches the tool via the registry, the result is fed
// back to the LLM, and the LLM's final answer is returned.
func TestEndToEndToolLoop(t *testing.T) {
	// Prepare a file the mocked LLM will "request" via read_file
	dir := t.TempDir()
	testFilePath := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(testFilePath, []byte("hi from tool"), 0o644))

	// Mock Anthropic server: turn 1 returns tool_use, turn 2 returns text
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		var events []string
		switch turn {
		case 1:
			// tool_use response
			events = []string{
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n",
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_e2e\",\"name\":\"read_file\",\"input\":{}}}\n\n",
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"" + testFilePath + "\\\"}\"}}\n\n",
				"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":5}}\n\n",
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			}
		case 2:
			// Final text response
			events = []string{
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_02\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":25,\"output_tokens\":0}}}\n\n",
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Got it.\"}}\n\n",
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n",
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			}
		default:
			t.Fatalf("unexpected turn %d", turn)
		}

		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
		_ = io.ReadAll(r.Body) // drain request body
	}))
	defer srv.Close()

	// Build provider
	p, err := anthropic.New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test",
		BaseURL:  srv.URL,
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	// Fresh storage + tool registry
	store, err := sqlite.Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	defer store.Close()

	reg := tool.NewRegistry()
	file.RegisterAll(reg)

	// Run the engine
	engine := agent.NewEngineWithTools(p, store, reg, config.AgentConfig{MaxTurns: 10}, "cli")
	result, err := engine.RunConversation(context.Background(), &agent.RunOptions{
		UserMessage: "read " + testFilePath,
		SessionID:   "e2e-tool-test",
		Model:       "claude-opus-4-6",
	})
	require.NoError(t, err)

	// 2 iterations: tool_use then final text
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "Got it.", result.Response.Content.Text())

	// Verify the tool was actually called — the tool_result message should
	// be the 3rd message (user, assistant_tool_use, user_tool_result, assistant_text)
	require.Len(t, result.Messages, 4)
	toolResultMsg := result.Messages[2]
	assert.Equal(t, message.RoleUser, toolResultMsg.Role)
	require.False(t, toolResultMsg.Content.IsText())
	blocks := toolResultMsg.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_result", blocks[0].Type)
	assert.Equal(t, "tool_e2e", blocks[0].ToolUseID)
	// The tool result should contain the file content
	var toolResultData map[string]any
	require.NoError(t, json.Unmarshal([]byte(blocks[0].ToolResult), &toolResultData))
	assert.Equal(t, "hi from tool", toolResultData["content"])
}
```

- [ ] **Step 2: Run the new test**

```bash
go test -race -run TestEndToEndToolLoop ./cli/...
```

Expected: PASS.

- [ ] **Step 3: Run the full test suite one more time**

```bash
go test -race ./...
```

Expected: ALL packages pass. No regressions.

- [ ] **Step 4: Run go vet and build**

```bash
go vet ./...
go build ./...
make build
./bin/hermes version
```

All should succeed. Binary prints version.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/cli/repl_tool_test.go
git commit -m "test(cli): add end-to-end tool loop test with mock Anthropic SSE"
```

---

## Task 13: Final Verification

- [ ] **Step 1: Run the full test suite with coverage**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race -cover ./...
```

Expected: ALL packages pass with coverage reported.

- [ ] **Step 2: Run go vet**

```bash
go vet ./...
```

Expected: no output (clean).

- [ ] **Step 3: Build the release binary**

```bash
make build
./bin/hermes version
```

Expected: binary builds and prints version.

- [ ] **Step 4: Manual smoke test with a real ANTHROPIC_API_KEY (OPTIONAL)**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./bin/hermes
```

Then type a message like: `list the files in the current directory`

Expected: The agent should display `⚡ list_directory: ...`, show the directory contents in a boxed output, and produce a final text response describing the files.

- [ ] **Step 5: Confirm git log is clean and commit history is correct**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git status
git log --oneline hermes-agent-go/ | head -20
```

Expected: clean working tree. Plan 2 contributes ~12 new commits on top of the Plan 1 baseline.

- [ ] **Step 6: Plan 2 done. Proceed to Plan 3 (remaining LLM providers) or Plan 4 (TUI) as the user decides.**

---

## Plan 2 Self-Review Notes

**Spec coverage:**
- Tool registry with Entry/Handler/Dispatch — Task 2
- Terminal backend interface — Task 3
- Local terminal backend — Task 3
- shell_execute tool — Task 4
- File tools (read_file, write_file, list_directory, search_files) — Tasks 5-6
- Content block extension for tool_use and tool_result — Task 1
- Provider.Request.Tools field — Task 7
- Anthropic Complete tool handling — Task 8
- Anthropic Stream tool_use handling — Task 9
- Engine multi-turn loop with tool execution — Task 10
- CLI tool registration — Task 11
- End-to-end tool loop test — Task 12

**Explicitly out of scope for Plan 2 (covered in later plans):**
- Docker, SSH, Modal, Daytona, Singularity terminal backends — Plan 5
- Web tools (search, extract, fetch) — Plan 5
- Browser automation — Plan 5
- Code execution sandbox — Plan 5
- Memory providers, skills, vision — Plan 6
- MCP bridge — Plan 6
- Parallel tool execution — Plan 5
- Persistent shell state — Plan 5
- Context compression — Plan 6
- Tool result truncation in the REPL beyond the 300-char snippet preview — Plan 4 (bubbletea TUI)

**Placeholder check:** No TBDs, no "implement later" phrases, no "similar to Task N" references. Every step has concrete file paths and executable code or commands.

**Type consistency:**
- `tool.Registry`, `tool.Entry`, `tool.Handler`, `tool.Dispatch`, `tool.ToolDefinition`, `tool.FunctionDef` — defined in Task 2, used consistently across Tasks 3-12
- `terminal.Backend`, `terminal.ExecOptions`, `terminal.ExecResult`, `terminal.Config` — defined in Task 3, used in Task 4 and Task 11
- `message.ContentBlock.ToolUseID/ToolUseName/ToolUseInput/ToolResult` — defined in Task 1, used in Tasks 8, 9, 10, 11, 12
- `provider.Request.Tools` — defined in Task 7, used in Tasks 8, 10
- `agent.NewEngineWithTools`, `SetToolStartCallback`, `SetToolResultCallback`, `extractToolCalls`, `executeToolCalls` — defined in Task 10, used in Task 11
- `anthropic.messagesRequest.Tools`, `anthropic.anthropicTool`, `apiContentItem.ID/Name/Input/ToolUseID/Content/IsError` — defined in Task 7, used in Tasks 8, 9

No naming drift across tasks.
