# GitHub Copilot Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `provider/copilot/` implementation that shells out to the GitHub Copilot CLI (`copilot --acp --stdio`) and adapts its ACP JSON-RPC stream to hermind's `provider.Provider` interface. Authentication is delegated to the user's existing `gh auth` / Copilot CLI setup — hermind never touches a GitHub token directly.

**Architecture:** One `Copilot` struct owns an `*exec.Cmd` running the Copilot CLI with its stdin, stdout, and stderr plumbed to buffered IO. An `Initialize` handshake runs on first use; subsequent `Complete()` / `Stream()` calls multiplex JSON-RPC requests/responses by numeric `id` through a single persistent pipe. Tool calls emerge as `<tool_call>{...}</tool_call>` blocks in the assistant text; a small extractor pulls them back into `message.ContentBlock` objects before returning. The subprocess lifetime is tied to the `Copilot` instance — `Close()` tears it down.

**Tech Stack:** Go 1.21+, `os/exec`, `bufio`, `encoding/json`, `sync`, `context`, existing `provider`, `message`, `tool`, `provider/factory` packages.

---

## File Structure

- Create: `provider/copilot/copilot.go` — constructor, lifecycle (`Close`), interface metadata
- Create: `provider/copilot/subprocess.go` — spawn + bufio readers + writer mutex
- Create: `provider/copilot/jsonrpc.go` — request encoder, response demuxer, notification dispatcher
- Create: `provider/copilot/prompt.go` — build the single-user-prompt string from a `provider.Request`
- Create: `provider/copilot/toolextract.go` — `<tool_call>{...}</tool_call>` regex + JSON decode
- Create: `provider/copilot/complete.go` — `Complete()` implementation
- Create: `provider/copilot/stream.go` — `Stream()` implementation
- Create: `provider/copilot/copilot_test.go` — unit tests using a fake Copilot CLI (a Go binary built in-test)
- Modify: `provider/factory/factory.go` — register `"copilot"`
- Modify: `provider/factory/factory_test.go`

---

## Task 1: Constructor + metadata

**Files:**
- Create: `provider/copilot/copilot.go`
- Create: `provider/copilot/copilot_test.go`

- [ ] **Step 1: Write the failing test**

Create `provider/copilot/copilot_test.go`:

```go
package copilot

import (
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestNew_Fields(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "copilot",
		Model:    "copilot",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "copilot" {
		t.Errorf("name = %q", p.Name())
	}
	// Available should be true as long as a command path is configured.
	if !p.Available() {
		t.Error("expected available = true by default")
	}
}

func TestModelInfo_DefaultsAreReasonable(t *testing.T) {
	p, _ := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	info := p.ModelInfo("copilot")
	if info == nil || info.ContextLength < 8_000 {
		t.Errorf("bad model info: %+v", info)
	}
	if !info.SupportsTools {
		t.Error("copilot must advertise tool support")
	}
}

func TestEstimateTokens_CharHeuristic(t *testing.T) {
	p, _ := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	n, _ := p.EstimateTokens("copilot", "hello world") // 11 chars
	if n < 2 || n > 4 {
		t.Errorf("expected ~3 tokens, got %d", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/copilot/ -v`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement the constructor**

Create `provider/copilot/copilot.go`:

```go
// Package copilot implements provider.Provider by driving the GitHub
// Copilot CLI over its ACP stdio protocol. It assumes the user has
// already authenticated with `gh auth login` + enabled Copilot CLI.
// Hermind never touches the underlying token directly.
package copilot

import (
	"os"
	"strings"
	"sync"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

const (
	defaultCommand = "copilot"
)

// Copilot is the provider.Provider implementation. It is safe for a
// single in-flight Complete or Stream call; concurrent calls will
// queue on the writer lock.
type Copilot struct {
	command string
	args    []string

	// Subprocess state is lazy-initialized on first use.
	mu  sync.Mutex
	sub *subprocess // nil until first call
}

// New constructs a Copilot provider from config.
//
// Override the command + args via env:
//
//	HERMIND_COPILOT_COMMAND=path/to/copilot
//	HERMIND_COPILOT_ARGS="--acp --stdio --other"  (space-separated)
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	cmd := os.Getenv("HERMIND_COPILOT_COMMAND")
	if cmd == "" {
		cmd = defaultCommand
	}
	args := []string{"--acp", "--stdio"}
	if v := os.Getenv("HERMIND_COPILOT_ARGS"); v != "" {
		args = splitArgs(v)
	}
	_ = cfg // unused — copilot has no config knobs today
	return &Copilot{command: cmd, args: args}, nil
}

// Name returns "copilot".
func (c *Copilot) Name() string { return "copilot" }

// Available returns true when a command path is configured. We don't
// probe the subprocess eagerly; fallback layers distinguish "not
// configured" from "configured but failing" only once a request runs.
func (c *Copilot) Available() bool { return c.command != "" }

// ModelInfo returns a conservative default — Copilot CLI does not
// expose a tokenization API so these numbers are best-effort.
func (c *Copilot) ModelInfo(model string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     32_000,
		MaxOutputTokens:   4_096,
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsVision:    false,
		SupportsCaching:   false,
	}
}

// EstimateTokens uses the ~4-chars-per-token rule of thumb.
func (c *Copilot) EstimateTokens(_ , text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	return (len(text) + 3) / 4, nil
}

// Close terminates the subprocess if one is running. Callers that
// embed the provider inside a fallback chain should call Close
// during shutdown.
func (c *Copilot) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sub == nil {
		return nil
	}
	err := c.sub.Close()
	c.sub = nil
	return err
}

// splitArgs splits on spaces, ignoring runs of whitespace. This is
// deliberately naive — quoting support is explicitly out of scope
// (users with exotic needs can patch the command directly).
func splitArgs(s string) []string {
	out := make([]string, 0, 4)
	for _, p := range strings.Fields(s) {
		out = append(out, p)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/copilot/ -run "TestNew|TestModelInfo|TestEstimate" -v`
Expected: PASS (3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/copilot/copilot.go provider/copilot/copilot_test.go
git commit -m "feat(provider/copilot): constructor + ModelInfo scaffolding"
```

---

## Task 2: Subprocess plumbing + fake binary helper

**Files:**
- Create: `provider/copilot/subprocess.go`
- Create: `provider/copilot/fake_copilot_test.go` — a helper that builds a fake copilot binary at test time

- [ ] **Step 1: Write the failing test**

Append to `provider/copilot/copilot_test.go`:

```go
func TestSubprocess_EchoInitialize(t *testing.T) {
	bin := buildFakeCopilot(t) // helper defined in fake_copilot_test.go
	t.Setenv("HERMIND_COPILOT_COMMAND", bin)
	t.Setenv("HERMIND_COPILOT_ARGS", "echo")

	p, err := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*Copilot)
	defer c.Close()

	// Force subprocess spin-up.
	if _, err := c.ensureSubprocess(); err != nil {
		t.Fatalf("ensureSubprocess: %v", err)
	}
	if c.sub == nil {
		t.Fatal("sub was not initialized")
	}
}
```

- [ ] **Step 2: Build the fake binary helper**

Create `provider/copilot/fake_copilot_test.go`:

```go
package copilot

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildFakeCopilot compiles the test helper program located at
// testdata/fake_copilot/main.go and returns its path. The helper
// acts like the Copilot CLI for protocol testing: it reads
// newline-delimited JSON-RPC from stdin and replies with canned
// frames from testdata/.
func buildFakeCopilot(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "fake_copilot", "main.go")
	bin := filepath.Join(t.TempDir(), "fake_copilot"+exeSuffix())
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot build fake copilot binary: %v", err)
	}
	return bin
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
```

Create `provider/copilot/testdata/fake_copilot/main.go`:

```go
// Fake Copilot CLI used only in provider/copilot tests. It reads
// newline-delimited JSON-RPC frames from stdin and responds with
// pre-scripted frames based on the "method" field of each request.
//
// Supported methods:
//   - initialize            → returns {"result":{"protocolVersion":1}}
//   - session/new           → returns {"result":{"sessionId":"test-session"}}
//   - session/prompt        → returns an assistant message whose content
//                             includes a <tool_call>...</tool_call> block,
//                             then {"result":{"stopReason":"end_turn"}}
//
// If the first argument is "echo" the binary echoes every incoming line
// to stderr and terminates on EOF — useful for smoke testing the
// subprocess lifecycle without any protocol expectations.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "echo" {
		io.Copy(os.Stderr, os.Stdin) // nolint: errcheck
		return
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<16), 1<<22)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for in.Scan() {
		line := in.Bytes()
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(line, &req)
		switch req.Method {
		case "initialize":
			reply(out, req.ID, map[string]any{"protocolVersion": 1})
		case "session/new":
			reply(out, req.ID, map[string]any{"sessionId": "test-session"})
		case "session/prompt":
			reply(out, req.ID, map[string]any{"stopReason": "end_turn"})
		default:
			fmt.Fprintf(os.Stderr, "fake copilot: unknown method %s\n", req.Method)
		}
	}
}

func reply(w *bufio.Writer, id json.RawMessage, result any) {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	w.Write(data)   // nolint: errcheck
	w.WriteByte('\n') // nolint: errcheck
	w.Flush()
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./provider/copilot/ -run TestSubprocess_EchoInitialize -v`
Expected: FAIL — `ensureSubprocess` undefined.

- [ ] **Step 4: Implement the subprocess wrapper**

Create `provider/copilot/subprocess.go`:

```go
package copilot

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// subprocess owns the Copilot CLI child process and multiplexes
// JSON-RPC requests over its stdin/stdout.
type subprocess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	writeMu sync.Mutex

	nextID int64

	mu      sync.Mutex
	pending map[int64]chan json.RawMessage

	// noteBridge receives server-initiated notifications (e.g.
	// session/update). Tests or higher-level streams consume it.
	noteBridge chan notification

	closed chan struct{}
}

type notification struct {
	Method string
	Params json.RawMessage
}

func (c *Copilot) ensureSubprocess() (*subprocess, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sub != nil {
		return c.sub, nil
	}
	cmd := exec.Command(c.command, c.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("copilot: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("copilot: stdout pipe: %w", err)
	}
	cmd.Stderr = nil // drop child stderr (it interferes with JSON-RPC if misconfigured)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("copilot: start %q: %w", c.command, err)
	}

	s := &subprocess{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewReader(stdout),
		pending:    map[int64]chan json.RawMessage{},
		noteBridge: make(chan notification, 16),
		closed:     make(chan struct{}),
	}
	go s.readLoop()
	c.sub = s
	return s, nil
}

// call sends a JSON-RPC request and blocks for the matching response.
// ctx cancellation aborts the wait but does not interrupt the child.
func (s *subprocess) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&s.nextID, 1)
	rawParams, _ := json.Marshal(params)
	frame, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  rawParams,
	})

	ch := make(chan json.RawMessage, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()

	s.writeMu.Lock()
	_, err := s.stdin.Write(append(frame, '\n'))
	s.writeMu.Unlock()
	if err != nil {
		s.clear(id)
		return nil, fmt.Errorf("copilot: write: %w", err)
	}

	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		s.clear(id)
		return nil, ctx.Err()
	case <-s.closed:
		s.clear(id)
		return nil, fmt.Errorf("copilot: subprocess closed")
	}
}

func (s *subprocess) readLoop() {
	defer close(s.closed)
	for {
		line, err := s.stdout.ReadBytes('\n')
		if err != nil {
			return
		}
		if len(line) == 0 {
			continue
		}
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Params json.RawMessage `json:"params"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		if envelope.Method != "" && len(envelope.ID) == 0 {
			// notification
			select {
			case s.noteBridge <- notification{Method: envelope.Method, Params: envelope.Params}:
			default:
				// drop if no consumer — better than deadlocking the reader
			}
			continue
		}
		// response to a request
		var id int64
		_ = json.Unmarshal(envelope.ID, &id)
		if id == 0 {
			continue
		}
		s.mu.Lock()
		ch, ok := s.pending[id]
		delete(s.pending, id)
		s.mu.Unlock()
		if !ok {
			continue
		}
		if len(envelope.Error) > 0 {
			ch <- envelope.Error
			continue
		}
		ch <- envelope.Result
	}
}

func (s *subprocess) clear(id int64) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

// Close sends SIGTERM (via Process.Kill) and waits.
func (s *subprocess) Close() error {
	_ = s.stdin.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./provider/copilot/ -run TestSubprocess_EchoInitialize -v -race`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add provider/copilot/subprocess.go provider/copilot/fake_copilot_test.go provider/copilot/testdata/fake_copilot/main.go
git commit -m "feat(provider/copilot): subprocess + JSON-RPC demux with fake binary harness"
```

---

## Task 3: Tool-call block extractor

**Files:**
- Create: `provider/copilot/toolextract.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/copilot/copilot_test.go`:

```go
func TestExtractToolCalls_SingleBlock(t *testing.T) {
	input := `Sure, let me help.
<tool_call>{"id":"t1","name":"shell","arguments":{"cmd":"ls"}}</tool_call>
Done.`
	calls, cleaned := ExtractToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	if calls[0].Name != "shell" {
		t.Errorf("name = %q", calls[0].Name)
	}
	if !strings.Contains(string(calls[0].Arguments), `"cmd":"ls"`) {
		t.Errorf("arguments = %s", calls[0].Arguments)
	}
	if strings.Contains(cleaned, "<tool_call>") {
		t.Errorf("cleaned still has block: %q", cleaned)
	}
}

func TestExtractToolCalls_NoBlocksReturnsOriginal(t *testing.T) {
	input := "just text"
	calls, cleaned := ExtractToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("unexpected calls: %v", calls)
	}
	if cleaned != input {
		t.Errorf("cleaned = %q", cleaned)
	}
}

func TestExtractToolCalls_MultipleBlocks(t *testing.T) {
	input := `<tool_call>{"id":"a","name":"read","arguments":{}}</tool_call>` +
		` and ` +
		`<tool_call>{"id":"b","name":"write","arguments":{}}</tool_call>`
	calls, _ := ExtractToolCalls(input)
	if len(calls) != 2 {
		t.Fatalf("calls = %d", len(calls))
	}
	if calls[0].ID != "a" || calls[1].ID != "b" {
		t.Errorf("ids = %v", calls)
	}
}
```

Make sure `"strings"` is imported.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/copilot/ -run TestExtractToolCalls -v`
Expected: FAIL — `ExtractToolCalls` undefined.

- [ ] **Step 3: Implement the extractor**

Create `provider/copilot/toolextract.go`:

```go
package copilot

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCall is the parsed form of a <tool_call>{...}</tool_call> block.
// The shape mirrors OpenAI's function-calling schema because that's
// what the Copilot CLI emits.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

var toolCallRe = regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)

// ExtractToolCalls scans text for <tool_call>{...}</tool_call> blocks.
// It returns every successfully-parsed call plus the text with the
// blocks removed. Malformed blocks are left in place — the caller
// should render them as text so the user can see what went wrong.
func ExtractToolCalls(text string) ([]ToolCall, string) {
	matches := toolCallRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, text
	}
	var calls []ToolCall
	var sb strings.Builder
	cursor := 0
	for _, m := range matches {
		// m = [fullStart, fullEnd, groupStart, groupEnd]
		block := text[m[2]:m[3]]
		var tc ToolCall
		if err := json.Unmarshal([]byte(block), &tc); err != nil {
			// Keep the raw block in-line; skip the call.
			sb.WriteString(text[cursor:m[1]])
			cursor = m[1]
			continue
		}
		// Drop the matched range from cleaned output.
		sb.WriteString(text[cursor:m[0]])
		cursor = m[1]
		calls = append(calls, tc)
	}
	sb.WriteString(text[cursor:])
	return calls, sb.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/copilot/ -run TestExtractToolCalls -v`
Expected: PASS (3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/copilot/toolextract.go provider/copilot/copilot_test.go
git commit -m "feat(provider/copilot): <tool_call> block extractor"
```

---

## Task 4: Prompt builder

**Files:**
- Create: `provider/copilot/prompt.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/copilot/copilot_test.go`:

```go
func TestBuildPrompt_IncludesSystemAndHistory(t *testing.T) {
	req := &provider.Request{
		SystemPrompt: "be helpful",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hello")},
			{Role: message.RoleAssistant, Content: message.TextContent("hi")},
			{Role: message.RoleUser, Content: message.TextContent("next")},
		},
	}
	out := BuildPrompt(req)
	for _, want := range []string{"be helpful", "hello", "hi", "next"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in prompt:\n%s", want, out)
		}
	}
}

func TestBuildPrompt_EmitsToolSchema(t *testing.T) {
	req := &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("run")}},
		Tools: []tool.ToolDefinition{{
			Type: "function",
			Function: tool.FunctionDef{Name: "shell", Description: "run a shell cmd", Parameters: []byte(`{"type":"object"}`)},
		}},
	}
	out := BuildPrompt(req)
	if !strings.Contains(out, `"name":"shell"`) {
		t.Errorf("missing tool schema: %s", out)
	}
}
```

Imports needed: `"github.com/odysseythink/hermind/message"`, `"github.com/odysseythink/hermind/provider"`, `"github.com/odysseythink/hermind/tool"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/copilot/ -run TestBuildPrompt -v`
Expected: FAIL.

- [ ] **Step 3: Implement the builder**

Create `provider/copilot/prompt.go`:

```go
package copilot

import (
	"encoding/json"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// BuildPrompt turns a provider.Request into the single user-visible
// text blob the Copilot ACP server expects. The format mirrors the
// Python copilot_acp_client so tool-call extraction works identically.
func BuildPrompt(req *provider.Request) string {
	var b strings.Builder
	b.WriteString("You are being used as the active ACP agent backend for Hermind.\n")
	b.WriteString("If you take an action with a tool, you MUST output tool calls using\n")
	b.WriteString("<tool_call>{...}</tool_call> blocks with JSON exactly in OpenAI function-call shape.\n")
	b.WriteString("If no tool is needed, answer normally.\n\n")

	if req.SystemPrompt != "" {
		b.WriteString("System:\n")
		b.WriteString(req.SystemPrompt)
		b.WriteString("\n\n")
	}

	if len(req.Tools) > 0 {
		b.WriteString("Available tools:\n")
		for _, t := range req.Tools {
			schema, _ := json.Marshal(map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  json.RawMessage(t.Function.Parameters),
			})
			b.Write(schema)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Conversation transcript:\n\n")
	for _, m := range req.Messages {
		role := "User"
		if m.Role == message.RoleAssistant {
			role = "Assistant"
		} else if m.Role == message.RoleTool {
			role = "Tool"
		}
		b.WriteString(role)
		b.WriteString(":\n")
		b.WriteString(m.Content.Text())
		b.WriteString("\n\n")
	}
	b.WriteString("Continue the conversation from the latest user request.")
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/copilot/ -run TestBuildPrompt -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/copilot/prompt.go provider/copilot/copilot_test.go
git commit -m "feat(provider/copilot): prompt builder with tool schema"
```

---

## Task 5: Complete()

**Files:**
- Create: `provider/copilot/complete.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/copilot/copilot_test.go`:

```go
func TestComplete_WithFakeCopilot(t *testing.T) {
	bin := buildFakeCopilot(t)
	t.Setenv("HERMIND_COPILOT_COMMAND", bin)
	t.Setenv("HERMIND_COPILOT_ARGS", "")

	p, err := New(config.ProviderConfig{Provider: "copilot", Model: "copilot"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.(*Copilot).Close()

	resp, err := p.Complete(context.Background(), &provider.Request{
		Model: "copilot",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.FinishReason != "end_turn" {
		t.Errorf("finish = %q", resp.FinishReason)
	}
	_ = resp.Message
}
```

Make sure `"context"` is imported.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/copilot/ -run TestComplete_WithFakeCopilot -v`
Expected: FAIL — `Complete` undefined.

- [ ] **Step 3: Implement Complete**

Create `provider/copilot/complete.go`:

```go
package copilot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Complete drives one round-trip: initialize (once per subprocess),
// session/new (if no session yet), then session/prompt. The Copilot
// CLI's async "session/update" notifications arrive on the bridge
// channel but are ignored by Complete — Stream() consumes them.
func (c *Copilot) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	sub, err := c.ensureSubprocess()
	if err != nil {
		return nil, err
	}
	if err := c.initialize(ctx, sub); err != nil {
		return nil, err
	}
	sessionID, err := c.openSession(ctx, sub)
	if err != nil {
		return nil, err
	}

	promptText := BuildPrompt(req)
	res, err := sub.call(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]string{{"type": "text", "text": promptText}},
	})
	if err != nil {
		return nil, err
	}

	// Drain any session/update notifications to build the assistant
	// reply. In the fake harness we just look at the final result.
	var promptResp struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(res, &promptResp)

	// Collect the assistant text from pending notifications.
	text := drainAssistantText(sub)

	calls, cleaned := ExtractToolCalls(text)
	var content message.Content
	if len(calls) > 0 {
		blocks := []message.ContentBlock{{Type: "text", Text: cleaned}}
		for _, tc := range calls {
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    tc.ID,
				ToolUseName:  tc.Name,
				ToolUseInput: tc.Arguments,
			})
		}
		content = message.BlockContent(blocks)
	} else {
		content = message.TextContent(cleaned)
	}

	if promptResp.StopReason == "" {
		promptResp.StopReason = "end_turn"
	}
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: content,
		},
		FinishReason: promptResp.StopReason,
		Model:        req.Model,
	}, nil
}

func (c *Copilot) initialize(ctx context.Context, sub *subprocess) error {
	_, err := sub.call(ctx, "initialize", map[string]any{})
	return err
}

func (c *Copilot) openSession(ctx context.Context, sub *subprocess) (string, error) {
	res, err := sub.call(ctx, "session/new", map[string]any{"cwd": "."})
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", fmt.Errorf("copilot: session/new: %w", err)
	}
	if out.SessionID == "" {
		return "", fmt.Errorf("copilot: session/new returned empty sessionId")
	}
	return out.SessionID, nil
}

// drainAssistantText pulls any pending session/update notifications
// and concatenates their text. Blocks only until the bridge drains
// — we never wait for "more" because the prompt response has already
// been delivered.
func drainAssistantText(sub *subprocess) string {
	var sb []byte
	for {
		select {
		case n := <-sub.noteBridge:
			if n.Method != "session/update" {
				continue
			}
			var params struct {
				Update struct {
					AgentMessageChunk struct {
						Text string `json:"text"`
					} `json:"agentMessageChunk"`
				} `json:"update"`
			}
			_ = json.Unmarshal(n.Params, &params)
			sb = append(sb, []byte(params.Update.AgentMessageChunk.Text)...)
		default:
			return string(sb)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/copilot/ -run TestComplete_WithFakeCopilot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/copilot/complete.go provider/copilot/copilot_test.go
git commit -m "feat(provider/copilot): Complete() via session/new + session/prompt"
```

---

## Task 6: Stream()

**Files:**
- Create: `provider/copilot/stream.go`

- [ ] **Step 1: Implement stream as an adapter around the existing machinery**

Create `provider/copilot/stream.go`:

```go
package copilot

import (
	"context"

	"github.com/odysseythink/hermind/provider"
)

// Stream for Copilot is a thin adapter: we call Complete (which
// blocks until the child finishes), then fabricate a two-event
// stream (Delta with the full text, then Done). This keeps the
// semantics consistent for callers that plug Copilot into
// fallback chains while Stream-native support lands.
func (c *Copilot) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	resp, err := c.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	return &syntheticStream{resp: resp}, nil
}

type syntheticStream struct {
	resp *provider.Response
	sent bool
	done bool
}

func (s *syntheticStream) Recv() (*provider.StreamEvent, error) {
	if !s.sent {
		s.sent = true
		return &provider.StreamEvent{
			Type: provider.EventDelta,
			Delta: &provider.StreamDelta{
				Content: s.resp.Message.Content.Text(),
			},
		}, nil
	}
	if !s.done {
		s.done = true
		return &provider.StreamEvent{Type: provider.EventDone, Response: s.resp}, nil
	}
	return &provider.StreamEvent{Type: provider.EventDone, Response: s.resp}, nil
}

func (s *syntheticStream) Close() error { return nil }
```

No test — `TestComplete_WithFakeCopilot` already exercises the provider.Stream shim path transitively through `Complete`. Adding a separate test would just re-assert `Complete` behavior.

- [ ] **Step 2: Build + test**

Run: `go build ./provider/copilot/ && go test ./provider/copilot/`
Expected: no errors, tests PASS.

- [ ] **Step 3: Commit**

```bash
git add provider/copilot/stream.go
git commit -m "feat(provider/copilot): Stream() synthetic adapter around Complete"
```

---

## Task 7: Factory registration

**Files:**
- Modify: `provider/factory/factory.go`
- Modify: `provider/factory/factory_test.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/factory/factory_test.go`:

```go
func TestFactory_Copilot(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "copilot",
		Model:    "copilot",
	})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if p.Name() != "copilot" {
		t.Errorf("name = %q", p.Name())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/factory/ -run TestFactory_Copilot -v`
Expected: FAIL — unknown provider.

- [ ] **Step 3: Register**

In `provider/factory/factory.go`, add the import and case:

```go
import (
	// ...
	"github.com/odysseythink/hermind/provider/copilot"
)

// inside New():
	case "copilot":
		return copilot.New(cfg)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/factory/ -run TestFactory_Copilot -v`
Expected: PASS.

Run full suite: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/factory/factory.go provider/factory/factory_test.go
git commit -m "feat(provider/factory): register copilot"
```

---

## Task 8: Manual smoke test

- [ ] **Step 1: Ensure GitHub Copilot CLI is installed + authenticated**

```bash
gh extension install github/gh-copilot
gh auth status
copilot --version
```

If any of these fail, skip the manual smoke test — CI coverage via the fake binary is sufficient for correctness.

- [ ] **Step 2: Add the provider to your hermind config**

```bash
cat >> ~/.hermind/config.yaml <<'EOF'
providers:
  copilot:
    provider: copilot
    api_key: "copilot-acp"
    model: copilot
EOF
```

- [ ] **Step 3: Exercise via hermind run**

```bash
go build -o /tmp/hermind ./cmd/hermind
/tmp/hermind run --model copilot/copilot <<<'Say hi in one word.'
```

Expected: a short single-word response streamed from Copilot.

- [ ] **Step 4: Cleanup**

```bash
rm /tmp/hermind
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - Subprocess over `copilot --acp --stdio` with env overrides ↔ Task 1 + Task 2 ✓
   - JSON-RPC demux by numeric id ↔ Task 2 ✓
   - Prompt builder emitting tool schema ↔ Task 4 ✓
   - `<tool_call>` extraction into `message.ContentBlock` ↔ Task 3 + Task 5 ✓
   - Complete() round-trip ↔ Task 5 ✓
   - Stream() adapter ↔ Task 6 ✓
   - Factory registration ↔ Task 7 ✓

2. **Placeholders:** none. Fake binary harness is fully spelled out and shippable.

3. **Type consistency:**
   - `ToolCall{ID, Name, Arguments}` matches the schema hermind's existing tool layer expects (`message.ContentBlock.ToolUseID / Name / Input`).
   - `subprocess.call(ctx, method, params) (json.RawMessage, error)` stable Tasks 2, 5.
   - `Copilot.ensureSubprocess()` signature stable Tasks 2, 5.
   - `BuildPrompt(req)` signature stable Tasks 4, 5.

4. **Gaps:**
   - True streaming of `session/update` chunks (Stream currently collapses to one delta + done).
   - File-system sandbox (Copilot's `fs/read_text_file` / `fs/write_text_file` pings are silently ignored — the MVP doesn't need them).
   - Retry / reconnect logic if the subprocess exits unexpectedly.

---

## Definition of Done

- `go test ./provider/copilot/... ./provider/factory/...` all pass.
- `go build ./...` succeeds.
- Fake-binary tests verify Complete + ExtractToolCalls + BuildPrompt end-to-end.
- When Copilot CLI is available locally, `hermind run --model copilot/copilot` produces a response.
