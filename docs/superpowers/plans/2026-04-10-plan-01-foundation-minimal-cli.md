# Plan 1: Foundation + Minimal CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundational vertical slice of hermes-agent-go: a working CLI that opens a REPL, chats with Claude via the Anthropic API, streams responses, and persists conversations to SQLite.

**Architecture:** Modular monolith in Go 1.24. Single binary (`cmd/hermes/`). Packages: `message/`, `config/`, `storage/`, `provider/`, `agent/`, `cli/`. Concurrency via goroutines + `errgroup`. Streaming via SSE. Per-request FallbackChain (currently single-provider). `Storage.WithTx` for atomic writes. `Engine` is single-use, not thread-safe.

**Tech Stack:** Go 1.24, `cobra` (CLI), `modernc.org/sqlite` (pure-Go SQLite), `gopkg.in/yaml.v3` (config), `net/http` (stdlib HTTP), `bufio` with 10MB buffer (SSE), `log/slog` (logging), `testify` (tests). No `bubbletea` yet — Plan 4 adds the rich TUI. No tools yet — Plan 2 adds them.

**Deliverable at end of plan:**
```
$ ./hermes
HERMES AGENT
claude-opus-4-6 · session #new
> Hello, who are you?
I am Claude, an AI assistant made by Anthropic...
> /exit
Session saved. 2 messages. $0.001.
```

**Non-goals for this plan (deferred to later plans):**
- Tools of any kind (Plan 2)
- Multi-provider fallback (Plan 3)
- Rich bubbletea TUI (Plan 4)
- MCP, memory providers, compression (Plan 6)
- Gateway and platform adapters (Plan 7+)

---

## File Structure

```
hermes-agent-go/
├── go.mod
├── go.sum
├── Makefile
├── .golangci.yml
├── .github/workflows/test.yml
├── cmd/
│   └── hermes/
│       └── main.go              # Entry point with cobra root
├── message/
│   ├── message.go               # Role, Message, Content, ContentBlock
│   ├── message_test.go
│   ├── content.go               # Typed Content union
│   └── content_test.go
├── config/
│   ├── config.go                # Config struct + defaults
│   ├── loader.go                # LoadConfig reads ~/.hermes/config.yaml
│   └── loader_test.go
├── storage/
│   ├── storage.go               # Storage + Tx interfaces
│   ├── types.go                 # Session, StoredMessage types
│   ├── sqlite/
│   │   ├── sqlite.go            # Store struct, connection
│   │   ├── migrate.go           # Schema migrations
│   │   ├── session.go           # Session CRUD
│   │   ├── message.go           # Message CRUD
│   │   ├── tx.go                # WithTx wrapper
│   │   └── sqlite_test.go
│   └── storage_test.go          # Interface compliance tests
├── provider/
│   ├── provider.go              # Provider interface
│   ├── errors.go                # Error + ErrorKind taxonomy
│   ├── fallback.go              # FallbackChain (per-request)
│   ├── fallback_test.go
│   └── anthropic/
│       ├── anthropic.go         # Anthropic struct + factory
│       ├── client.go            # HTTP client with retry
│       ├── complete.go          # Complete() non-streaming
│       ├── stream.go            # Stream() with SSE parsing
│       ├── types.go             # Anthropic wire types
│       ├── errors.go            # Error mapping
│       └── anthropic_test.go
├── agent/
│   ├── budget.go                # Atomic iteration budget
│   ├── budget_test.go
│   ├── prompt.go                # PromptBuilder (minimal)
│   ├── prompt_test.go
│   ├── engine.go                # Engine struct
│   ├── conversation.go          # RunConversation (single-turn)
│   └── engine_test.go
└── cli/
    ├── app.go                   # App struct
    ├── root.go                  # cobra root command
    ├── run.go                   # "hermes run" command
    └── repl.go                  # Basic bufio.Scanner REPL
```

## Commit Convention

All commits follow: `<type>(<scope>): <message>`

Where `type` is one of `feat`, `fix`, `test`, `chore`, `refactor`, and `scope` is the package name (e.g., `message`, `storage`, `anthropic`).

---

## Task 1: Initialize Go Module and Repository Scaffolding

**Files:**
- Create: `hermes-agent-go/go.mod`
- Create: `hermes-agent-go/Makefile`
- Create: `hermes-agent-go/.gitignore`
- Create: `hermes-agent-go/.golangci.yml`
- Create: `hermes-agent-go/cmd/hermes/main.go`

- [ ] **Step 1: Create the hermes-agent-go directory and initialize the Go module**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
mkdir -p hermes-agent-go/cmd/hermes
cd hermes-agent-go
go mod init github.com/nousresearch/hermes-agent
```

Expected: `go.mod` file created with `module github.com/nousresearch/hermes-agent` and `go 1.24`.

- [ ] **Step 2: Create a placeholder `cmd/hermes/main.go` so the module compiles**

```go
// cmd/hermes/main.go
package main

import "fmt"

var Version = "dev"

func main() {
	fmt.Printf("hermes-agent %s\n", Version)
}
```

- [ ] **Step 3: Create a Makefile with basic targets**

```makefile
# Makefile
.PHONY: build test lint clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/hermes ./cmd/hermes

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
```

- [ ] **Step 4: Create `.gitignore` and `.golangci.yml`**

`.gitignore`:
```
bin/
*.test
*.out
coverage.txt
.env
```

`.golangci.yml`:
```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - ineffassign
    - gosimple
    - unused
    - gofmt
    - misspell

linters-settings:
  errcheck:
    check-type-assertions: true
```

- [ ] **Step 5: Build and run to verify the module compiles**

Run: `make build && ./bin/hermes`
Expected output: `hermes-agent dev`

- [ ] **Step 6: Commit**

```bash
git add hermes-agent-go/
git commit -m "chore(scaffold): initialize Go module and build pipeline"
```

---

## Task 2: Create Message Package Base Types

**Files:**
- Create: `hermes-agent-go/message/message.go`
- Create: `hermes-agent-go/message/message_test.go`

- [ ] **Step 1: Write the failing test for message types**

```go
// message/message_test.go
package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, Role("user"), RoleUser)
	assert.Equal(t, Role("assistant"), RoleAssistant)
	assert.Equal(t, Role("tool"), RoleTool)
	assert.Equal(t, Role("system"), RoleSystem)
}

func TestMessageJSONRoundtripText(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: TextContent("hello world"),
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, RoleUser, decoded.Role)
	assert.True(t, decoded.Content.IsText())
	assert.Equal(t, "hello world", decoded.Content.Text())
}

func TestUsageZeroValue(t *testing.T) {
	var u Usage
	assert.Equal(t, 0, u.InputTokens)
	assert.Equal(t, 0, u.OutputTokens)
}
```

- [ ] **Step 2: Add testify dependency and run tests to verify failure**

```bash
cd hermes-agent-go
go get github.com/stretchr/testify@latest
go test ./message/...
```

Expected: FAIL with "undefined: Role" / "undefined: Message".

- [ ] **Step 3: Create message package with Role and Message types**

```go
// message/message.go
package message

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// Message represents a single turn in a conversation.
// Content is a typed union — see content.go for the Content type.
type Message struct {
	Role         Role       `json:"role"`
	Content      Content    `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID   string     `json:"tool_call_id,omitempty"`
	ToolName     string     `json:"tool_name,omitempty"`
	Reasoning    string     `json:"reasoning,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// ToolCall represents a single tool invocation requested by the assistant.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded argument string
}

// Usage tracks token accounting for a single API call.
type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}
```

- [ ] **Step 4: Tests still fail because Content doesn't exist yet — that's expected, we add it in Task 3**

Run: `go test ./message/...`
Expected: FAIL with "undefined: Content" / "undefined: TextContent". This is correct — the Content type is the next task.

- [ ] **Step 5: Commit (intentionally broken build — will be fixed in Task 3)**

Skip the commit for this task. Task 3 completes the message package in one commit.

---

## Task 3: Implement Typed Content Union

**Files:**
- Create: `hermes-agent-go/message/content.go`
- Create: `hermes-agent-go/message/content_test.go`

- [ ] **Step 1: Write the failing tests for Content type**

```go
// message/content_test.go
package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextContentAccessors(t *testing.T) {
	c := TextContent("hello")
	assert.True(t, c.IsText())
	assert.Equal(t, "hello", c.Text())
	assert.Nil(t, c.Blocks())
}

func TestBlockContentAccessors(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "hello"},
		{Type: "image_url", ImageURL: &Image{URL: "http://x.png"}},
	}
	c := BlockContent(blocks)
	assert.False(t, c.IsText())
	assert.Equal(t, "", c.Text())
	assert.Len(t, c.Blocks(), 2)
}

func TestContentMarshalJSONText(t *testing.T) {
	c := TextContent("hello")
	data, err := c.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"hello"`, string(data))
}

func TestContentMarshalJSONBlocks(t *testing.T) {
	c := BlockContent([]ContentBlock{{Type: "text", Text: "hi"}})
	data, err := c.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"text","text":"hi"}]`, string(data))
}

func TestContentUnmarshalJSONAcceptsString(t *testing.T) {
	var c Content
	err := json.Unmarshal([]byte(`"hello"`), &c)
	require.NoError(t, err)
	assert.True(t, c.IsText())
	assert.Equal(t, "hello", c.Text())
}

func TestContentUnmarshalJSONAcceptsArray(t *testing.T) {
	var c Content
	err := json.Unmarshal([]byte(`[{"type":"text","text":"hi"}]`), &c)
	require.NoError(t, err)
	assert.False(t, c.IsText())
	require.Len(t, c.Blocks(), 1)
	assert.Equal(t, "hi", c.Blocks()[0].Text)
}

func TestContentUnmarshalJSONRejectsInvalid(t *testing.T) {
	var c Content
	err := json.Unmarshal([]byte(`123`), &c)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Create `content.go` with the typed union**

```go
// message/content.go
package message

import (
	"encoding/json"
	"fmt"
)

// Content is a typed union of plain text and structured content blocks.
// Exactly one of text or blocks is populated. Never both.
// Use TextContent(s) or BlockContent(b) to construct.
type Content struct {
	text   string
	blocks []ContentBlock
}

// TextContent creates a Content holding a plain text string.
func TextContent(s string) Content {
	return Content{text: s}
}

// BlockContent creates a Content holding a list of structured blocks.
func BlockContent(blocks []ContentBlock) Content {
	return Content{blocks: blocks}
}

// IsText reports whether the Content is the plain-text form.
func (c Content) IsText() bool { return c.blocks == nil }

// Text returns the plain-text form. Empty string if IsText() is false.
func (c Content) Text() string { return c.text }

// Blocks returns the structured-blocks form. Nil if IsText() is true.
func (c Content) Blocks() []ContentBlock { return c.blocks }

// MarshalJSON produces the OpenAI-compatible shape: string OR array.
func (c Content) MarshalJSON() ([]byte, error) {
	if c.IsText() {
		return json.Marshal(c.text)
	}
	return json.Marshal(c.blocks)
}

// UnmarshalJSON accepts either a string or an array of ContentBlock.
func (c *Content) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("message: empty content")
	}
	// Try string first (cheapest discriminator)
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("message: invalid string content: %w", err)
		}
		c.text = s
		c.blocks = nil
		return nil
	}
	// Fall back to array
	if data[0] == '[' {
		var blocks []ContentBlock
		if err := json.Unmarshal(data, &blocks); err != nil {
			return fmt.Errorf("message: invalid block content: %w", err)
		}
		c.text = ""
		c.blocks = blocks
		return nil
	}
	return fmt.Errorf("message: content must be string or array, got %q", data[:1])
}

// ContentBlock is one element of a structured content array.
// Used for multimodal content (images) and tool results.
type ContentBlock struct {
	Type     string `json:"type"` // "text", "image_url", "tool_use", "tool_result"
	Text     string `json:"text,omitempty"`
	ImageURL *Image `json:"image_url,omitempty"`
}

// Image represents an image reference in a content block.
type Image struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}
```

- [ ] **Step 3: Run the full message package tests**

Run: `go test -race ./message/...`
Expected: PASS. All tests from Task 2 and Task 3 should now pass.

- [ ] **Step 4: Run go vet and golangci-lint**

Run: `go vet ./message/... && golangci-lint run ./message/...`
Expected: no issues reported.

- [ ] **Step 5: Commit**

```bash
git add message/ go.mod go.sum
git commit -m "feat(message): add typed Message and Content types"
```

---

## Task 4: Create Minimal Config Package

**Files:**
- Create: `hermes-agent-go/config/config.go`
- Create: `hermes-agent-go/config/loader.go`
- Create: `hermes-agent-go/config/loader_test.go`

- [ ] **Step 1: Write the failing tests for config loading**

```go
// config/loader_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigHasSensibleDefaults(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "anthropic/claude-opus-4-6", cfg.Model)
	assert.Equal(t, 90, cfg.Agent.MaxTurns)
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
model: anthropic/claude-sonnet-4-6
providers:
  anthropic:
    provider: anthropic
    api_key: sk-test-abc
    model: claude-sonnet-4-6
agent:
  max_turns: 42
storage:
  driver: sqlite
  sqlite_path: /tmp/test.db
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", cfg.Model)
	assert.Equal(t, 42, cfg.Agent.MaxTurns)
	assert.Equal(t, "sk-test-abc", cfg.Providers["anthropic"].APIKey)
	assert.Equal(t, "/tmp/test.db", cfg.Storage.SQLitePath)
}

func TestLoadFromMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadFromPath("/nonexistent/path/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, Default().Model, cfg.Model)
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("HERMES_TEST_KEY", "sk-from-env")
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
providers:
  anthropic:
    provider: anthropic
    api_key: env:HERMES_TEST_KEY
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "sk-from-env", cfg.Providers["anthropic"].APIKey)
}
```

- [ ] **Step 2: Add yaml.v3 dependency**

```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 3: Create `config/config.go` with Config struct and Default()**

```go
// config/config.go
package config

// Config holds all user-configurable settings for hermes-agent.
// YAML tags mirror the existing Python hermes config.yaml format.
type Config struct {
	Model     string                    `yaml:"model"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Agent     AgentConfig               `yaml:"agent"`
	Storage   StorageConfig             `yaml:"storage"`
}

// ProviderConfig holds settings for a single LLM provider.
type ProviderConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
}

// AgentConfig holds engine-level settings.
type AgentConfig struct {
	MaxTurns       int `yaml:"max_turns"`
	GatewayTimeout int `yaml:"gateway_timeout,omitempty"`
}

// StorageConfig holds storage driver settings.
type StorageConfig struct {
	Driver      string `yaml:"driver"`
	SQLitePath  string `yaml:"sqlite_path,omitempty"`
	PostgresURL string `yaml:"postgres_url,omitempty"`
}

// Default returns a Config populated with sensible defaults.
// These match the Python hermes defaults.
func Default() *Config {
	return &Config{
		Model:     "anthropic/claude-opus-4-6",
		Providers: map[string]ProviderConfig{},
		Agent: AgentConfig{
			MaxTurns:       90,
			GatewayTimeout: 1800,
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}
```

- [ ] **Step 4: Create `config/loader.go` with LoadFromPath**

```go
// config/loader.go
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigDir is the default location for hermes config files.
const (
	DefaultConfigDir  = "~/.hermes"
	DefaultConfigFile = "config.yaml"
	DefaultDBFile     = "state.db"
)

// Load reads the default config file at ~/.hermes/config.yaml.
// Missing file is not an error — returns defaults.
func Load() (*Config, error) {
	path, err := expandPath(filepath.Join(DefaultConfigDir, DefaultConfigFile))
	if err != nil {
		return nil, err
	}
	return LoadFromPath(path)
}

// LoadFromPath reads a specific config file. Missing file returns defaults.
func LoadFromPath(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// Missing config file is OK — defaults apply
		resolveDefaults(cfg)
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := expandEnvVars(cfg); err != nil {
		return nil, err
	}
	resolveDefaults(cfg)
	return cfg, nil
}

// expandPath resolves ~ in paths to the user home directory.
func expandPath(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve home: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}

// expandEnvVars replaces "env:VAR_NAME" references in api keys with the env value.
func expandEnvVars(cfg *Config) error {
	for name, p := range cfg.Providers {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			p.APIKey = os.Getenv(varName)
			cfg.Providers[name] = p
		}
	}
	return nil
}

// resolveDefaults fills in missing values that depend on environment.
func resolveDefaults(cfg *Config) {
	if cfg.Storage.SQLitePath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.Storage.SQLitePath = filepath.Join(home, ".hermes", DefaultDBFile)
		}
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test -race ./config/...`
Expected: PASS. All four tests should pass.

- [ ] **Step 6: Commit**

```bash
git add config/ go.mod go.sum
git commit -m "feat(config): add YAML config loader with env var expansion"
```

---

## Task 5: Create Storage Interface and Types

**Files:**
- Create: `hermes-agent-go/storage/storage.go`
- Create: `hermes-agent-go/storage/types.go`

- [ ] **Step 1: Create `storage/types.go` with Session and StoredMessage**

```go
// storage/types.go
package storage

import (
	"encoding/json"
	"time"
)

// Session represents a conversation session persisted to storage.
// Fields mirror the Python hermes state.db schema for compatibility.
type Session struct {
	ID              string
	Source          string // "cli", "telegram", "discord", ...
	UserID          string
	Model           string
	ModelConfig     json.RawMessage
	SystemPrompt    string
	ParentSessionID string // non-empty for compression chains
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

// SessionUsage tracks aggregate token counts for a session.
type SessionUsage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
}

// StoredMessage is the persistence shape of a single conversation message.
type StoredMessage struct {
	ID               int64
	SessionID        string
	Role             string
	Content          string // JSON-encoded message.Content
	ToolCallID       string
	ToolCalls        json.RawMessage
	ToolName         string
	Timestamp        time.Time
	TokenCount       int
	FinishReason     string
	Reasoning        string
	ReasoningDetails string
}

// SessionUpdate holds partial fields for UpdateSession.
type SessionUpdate struct {
	EndedAt      *time.Time
	EndReason    string
	Title        string
	MessageCount *int
}

// UsageUpdate holds a usage delta to add to a session.
type UsageUpdate struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	CostUSD          float64
}

// ListOptions controls pagination and filtering for ListSessions.
type ListOptions struct {
	Source string
	UserID string
	Limit  int
	Before time.Time
}

// SearchOptions controls FTS message search.
type SearchOptions struct {
	SessionID string
	Limit     int
}

// SearchResult is a single hit from SearchMessages.
type SearchResult struct {
	Message   *StoredMessage
	SessionID string
	Snippet   string
	Rank      float64
}
```

- [ ] **Step 2: Create `storage/storage.go` with the interfaces**

```go
// storage/storage.go
package storage

import "context"

// Storage is the root storage interface. Implementations must be safe for
// concurrent use.
type Storage interface {
	// Session operations
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, id string, updates *SessionUpdate) error
	ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, error)

	// Message operations
	AddMessage(ctx context.Context, sessionID string, msg *StoredMessage) error
	GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error)
	SearchMessages(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)

	// System prompt cache (for Anthropic prefix caching)
	UpdateSystemPrompt(ctx context.Context, sessionID string, prompt string) error

	// Usage accounting
	UpdateUsage(ctx context.Context, sessionID string, usage *UsageUpdate) error

	// Transactions — group multiple operations atomically.
	// The function is called with a Tx scoped to a single SQL transaction.
	// Return an error to roll back. Return nil to commit.
	WithTx(ctx context.Context, fn func(tx Tx) error) error

	// Lifecycle
	Close() error
	Migrate() error
}

// Tx is the transaction-scoped interface. Operations are atomic: either
// all commit or all roll back. Do not retain a Tx reference after the
// callback returns.
type Tx interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, id string, updates *SessionUpdate) error
	AddMessage(ctx context.Context, sessionID string, msg *StoredMessage) error
	UpdateSystemPrompt(ctx context.Context, sessionID string, prompt string) error
	UpdateUsage(ctx context.Context, sessionID string, usage *UsageUpdate) error
}
```

- [ ] **Step 3: Verify the package compiles**

Run: `go build ./storage/...`
Expected: success, no compilation errors.

- [ ] **Step 4: Commit**

```bash
git add storage/
git commit -m "feat(storage): add Storage and Tx interfaces with session types"
```

---

## Task 6: Implement SQLite Store with Schema Migration

**Files:**
- Create: `hermes-agent-go/storage/sqlite/sqlite.go`
- Create: `hermes-agent-go/storage/sqlite/migrate.go`
- Create: `hermes-agent-go/storage/sqlite/sqlite_test.go`

- [ ] **Step 1: Write the failing migration test**

```go
// storage/sqlite/sqlite_test.go
package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpenCreatesDatabaseFile(t *testing.T) {
	store := newTestStore(t)
	assert.NotNil(t, store)
}

func TestMigrateIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	// Running migrate twice must not error.
	err := store.Migrate()
	assert.NoError(t, err)
}

func TestMigrateCreatesRequiredTables(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Query sqlite_master to confirm tables exist.
	rows, err := store.db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables[name] = true
	}
	assert.True(t, tables["sessions"], "sessions table should exist")
	assert.True(t, tables["messages"], "messages table should exist")
}
```

- [ ] **Step 2: Add SQLite driver dependency**

```bash
go get modernc.org/sqlite
```

- [ ] **Step 3: Create `storage/sqlite/sqlite.go` with Open and Store struct**

```go
// storage/sqlite/sqlite.go
package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store is the SQLite-backed implementation of storage.Storage.
// Safe for concurrent use. Uses WAL mode with BEGIN IMMEDIATE write txns.
type Store struct {
	db         *sql.DB
	path       string
	writeCount atomic.Int64
}

// Open creates or opens a SQLite database at the given path.
// The file is created if it does not exist. WAL mode is enabled.
// Call Migrate() after Open() to apply schema.
func Open(path string) (*Store, error) {
	// Ensure parent directory exists
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		// Caller is responsible for creating the parent directory.
	}

	// modernc.org/sqlite uses the name "sqlite" not "sqlite3"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}

	// Apply pragmas: WAL mode for concurrent reads, foreign keys on, 1s busy timeout
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=1000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite: pragma %q: %w", p, err)
		}
	}

	return &Store{db: db, path: path}, nil
}

// Close shuts down the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
```

- [ ] **Step 4: Create `storage/sqlite/migrate.go` with schema creation**

```go
// storage/sqlite/migrate.go
package sqlite

import (
	"fmt"
)

// schemaSQL is the full initial schema. The schema is designed to match
// the Python hermes state.db layout for compatibility.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL DEFAULT 'cli',
    user_id TEXT DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    model_config TEXT DEFAULT '{}',
    system_prompt TEXT DEFAULT '',
    parent_session_id TEXT DEFAULT '',
    started_at REAL NOT NULL,
    ended_at REAL,
    end_reason TEXT DEFAULT '',
    message_count INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    billing_provider TEXT DEFAULT '',
    billing_base_url TEXT DEFAULT '',
    estimated_cost_usd REAL NOT NULL DEFAULT 0,
    actual_cost_usd REAL NOT NULL DEFAULT 0,
    cost_status TEXT DEFAULT '',
    title TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_calls TEXT DEFAULT '',
    tool_name TEXT DEFAULT '',
    timestamp REAL NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    finish_reason TEXT DEFAULT '',
    reasoning TEXT DEFAULT '',
    reasoning_details TEXT DEFAULT '',
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);

-- FTS5 full-text search over message content
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    content='messages',
    content_rowid='id'
);

-- Triggers to keep FTS index in sync with messages table
CREATE TRIGGER IF NOT EXISTS messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_update AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;
`

// Migrate applies the schema to the database. Idempotent: safe to call
// multiple times. Does NOT yet support incremental migrations beyond v1 —
// future plans will add a migrations table.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("sqlite: migrate: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests**

Run: `go test -race ./storage/sqlite/...`
Expected: PASS. All three tests should pass.

- [ ] **Step 6: Commit**

```bash
git add storage/sqlite/ go.mod go.sum
git commit -m "feat(storage): add SQLite store with schema and FTS5 migrations"
```

---

## Task 7: Implement SQLite Session CRUD

**Files:**
- Create: `hermes-agent-go/storage/sqlite/session.go`
- Modify: `hermes-agent-go/storage/sqlite/sqlite_test.go`

- [ ] **Step 1: Add failing tests for session CRUD**

Append to `storage/sqlite/sqlite_test.go`:

```go
import (
	"time"

	"github.com/nousresearch/hermes-agent/storage"
)

func TestCreateAndGetSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	session := &storage.Session{
		ID:        "sess-001",
		Source:    "cli",
		UserID:    "user-1",
		Model:     "claude-opus-4-6",
		StartedAt: now,
		Title:     "my session",
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	got, err := store.GetSession(ctx, "sess-001")
	require.NoError(t, err)
	assert.Equal(t, "sess-001", got.ID)
	assert.Equal(t, "cli", got.Source)
	assert.Equal(t, "user-1", got.UserID)
	assert.Equal(t, "claude-opus-4-6", got.Model)
	assert.Equal(t, "my session", got.Title)
	assert.WithinDuration(t, now, got.StartedAt, time.Second)
}

func TestGetSessionNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetSession(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestUpdateSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "sess-002", Source: "cli", Model: "test", StartedAt: now,
	}))

	end := now.Add(time.Minute)
	err := store.UpdateSession(ctx, "sess-002", &storage.SessionUpdate{
		EndedAt:   &end,
		EndReason: "user_exit",
		Title:     "done",
	})
	require.NoError(t, err)

	got, err := store.GetSession(ctx, "sess-002")
	require.NoError(t, err)
	require.NotNil(t, got.EndedAt)
	assert.Equal(t, "user_exit", got.EndReason)
	assert.Equal(t, "done", got.Title)
}
```

- [ ] **Step 2: Add ErrNotFound to the storage package**

Append to `storage/storage.go`:

```go
import "errors"

// Sentinel errors returned by storage implementations.
var (
	// ErrNotFound is returned when a session or message does not exist.
	ErrNotFound = errors.New("storage: not found")
)
```

- [ ] **Step 3: Create `storage/sqlite/session.go` with CRUD implementations**

```go
// storage/sqlite/session.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nousresearch/hermes-agent/storage"
)

// toEpoch converts a time.Time to a float64 Unix timestamp with sub-second precision.
// This matches the Python hermes state.db storage format.
func toEpoch(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}

// fromEpoch converts a float64 Unix timestamp back to time.Time.
func fromEpoch(f float64) time.Time {
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}

// CreateSession inserts a new session row.
func (s *Store) CreateSession(ctx context.Context, session *storage.Session) error {
	modelConfig := string(session.ModelConfig)
	if modelConfig == "" {
		modelConfig = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO sessions (
            id, source, user_id, model, model_config, system_prompt,
            parent_session_id, started_at, message_count, tool_call_count,
            input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
            reasoning_tokens, title
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Source, session.UserID, session.Model,
		modelConfig, session.SystemPrompt,
		session.ParentSessionID, toEpoch(session.StartedAt),
		session.MessageCount, session.ToolCallCount,
		session.Usage.InputTokens, session.Usage.OutputTokens,
		session.Usage.CacheReadTokens, session.Usage.CacheWriteTokens,
		session.Usage.ReasoningTokens, session.Title,
	)
	if err != nil {
		return fmt.Errorf("sqlite: create session %s: %w", session.ID, err)
	}
	return nil
}

// GetSession fetches a session by ID. Returns storage.ErrNotFound if missing.
func (s *Store) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	var (
		sess          storage.Session
		modelConfig   string
		startedAt     float64
		endedAt       sql.NullFloat64
	)
	err := s.db.QueryRowContext(ctx, `
        SELECT id, source, user_id, model, model_config, system_prompt,
               parent_session_id, started_at, ended_at, end_reason,
               message_count, tool_call_count,
               input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
               reasoning_tokens, billing_provider, billing_base_url,
               estimated_cost_usd, actual_cost_usd, cost_status, title
        FROM sessions WHERE id = ?`, id,
	).Scan(
		&sess.ID, &sess.Source, &sess.UserID, &sess.Model, &modelConfig,
		&sess.SystemPrompt, &sess.ParentSessionID, &startedAt, &endedAt,
		&sess.EndReason, &sess.MessageCount, &sess.ToolCallCount,
		&sess.Usage.InputTokens, &sess.Usage.OutputTokens,
		&sess.Usage.CacheReadTokens, &sess.Usage.CacheWriteTokens,
		&sess.Usage.ReasoningTokens, &sess.BillingProvider, &sess.BillingBaseURL,
		&sess.EstimatedCost, &sess.ActualCost, &sess.CostStatus, &sess.Title,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get session %s: %w", id, err)
	}

	sess.ModelConfig = []byte(modelConfig)
	sess.StartedAt = fromEpoch(startedAt)
	if endedAt.Valid {
		t := fromEpoch(endedAt.Float64)
		sess.EndedAt = &t
	}
	return &sess, nil
}

// UpdateSession applies partial updates to a session.
func (s *Store) UpdateSession(ctx context.Context, id string, upd *storage.SessionUpdate) error {
	// Build dynamic UPDATE based on non-nil fields
	var (
		setClauses []string
		args       []any
	)
	if upd.EndedAt != nil {
		setClauses = append(setClauses, "ended_at = ?")
		args = append(args, toEpoch(*upd.EndedAt))
	}
	if upd.EndReason != "" {
		setClauses = append(setClauses, "end_reason = ?")
		args = append(args, upd.EndReason)
	}
	if upd.Title != "" {
		setClauses = append(setClauses, "title = ?")
		args = append(args, upd.Title)
	}
	if upd.MessageCount != nil {
		setClauses = append(setClauses, "message_count = ?")
		args = append(args, *upd.MessageCount)
	}
	if len(setClauses) == 0 {
		return nil
	}

	query := "UPDATE sessions SET " + joinWith(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("sqlite: update session %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// joinWith is a tiny helper for building dynamic SQL.
func joinWith(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./storage/...`
Expected: PASS. Session create/get/update tests pass, ErrNotFound test passes.

- [ ] **Step 5: Commit**

```bash
git add storage/
git commit -m "feat(storage): implement SQLite session CRUD"
```

---

## Task 8: Implement SQLite Message CRUD and ListSessions/Search Stubs

**Files:**
- Create: `hermes-agent-go/storage/sqlite/message.go`
- Modify: `hermes-agent-go/storage/sqlite/session.go` (add ListSessions)
- Modify: `hermes-agent-go/storage/sqlite/sqlite_test.go` (add message tests)

- [ ] **Step 1: Add failing tests for messages and list**

Append to `storage/sqlite/sqlite_test.go`:

```go
func TestAddAndGetMessages(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "sess-msg-1", Source: "cli", Model: "test", StartedAt: now,
	}))

	err := store.AddMessage(ctx, "sess-msg-1", &storage.StoredMessage{
		SessionID: "sess-msg-1",
		Role:      "user",
		Content:   `"hello"`,
		Timestamp: now,
	})
	require.NoError(t, err)

	err = store.AddMessage(ctx, "sess-msg-1", &storage.StoredMessage{
		SessionID: "sess-msg-1",
		Role:      "assistant",
		Content:   `"hi there"`,
		Timestamp: now.Add(time.Second),
	})
	require.NoError(t, err)

	msgs, err := store.GetMessages(ctx, "sess-msg-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
}

func TestSearchMessagesFTS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "sess-fts-1", Source: "cli", Model: "test", StartedAt: now,
	}))

	require.NoError(t, store.AddMessage(ctx, "sess-fts-1", &storage.StoredMessage{
		SessionID: "sess-fts-1", Role: "user",
		Content: "the quick brown fox jumps", Timestamp: now,
	}))
	require.NoError(t, store.AddMessage(ctx, "sess-fts-1", &storage.StoredMessage{
		SessionID: "sess-fts-1", Role: "assistant",
		Content: "lazy dogs sleep", Timestamp: now.Add(time.Second),
	}))

	results, err := store.SearchMessages(ctx, "fox", &storage.SearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Message.Content, "fox")
}
```

- [ ] **Step 2: Create `storage/sqlite/message.go`**

```go
// storage/sqlite/message.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nousresearch/hermes-agent/storage"
)

// AddMessage inserts a new message row. The messages_fts virtual table
// is kept in sync automatically via AFTER INSERT trigger.
func (s *Store) AddMessage(ctx context.Context, sessionID string, msg *storage.StoredMessage) error {
	toolCalls := string(msg.ToolCalls)
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO messages (
            session_id, role, content, tool_call_id, tool_calls, tool_name,
            timestamp, token_count, finish_reason, reasoning, reasoning_details
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, msg.ToolCallID, toolCalls, msg.ToolName,
		toEpoch(msg.Timestamp), msg.TokenCount, msg.FinishReason,
		msg.Reasoning, msg.ReasoningDetails,
	)
	if err != nil {
		return fmt.Errorf("sqlite: add message to %s: %w", sessionID, err)
	}
	s.writeCount.Add(1)
	return nil
}

// GetMessages fetches messages for a session in timestamp order.
func (s *Store) GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]*storage.StoredMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, session_id, role, content, tool_call_id, tool_calls, tool_name,
               timestamp, token_count, finish_reason, reasoning, reasoning_details
        FROM messages WHERE session_id = ?
        ORDER BY timestamp ASC, id ASC
        LIMIT ? OFFSET ?`, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get messages for %s: %w", sessionID, err)
	}
	defer rows.Close()

	var msgs []*storage.StoredMessage
	for rows.Next() {
		var (
			m         storage.StoredMessage
			toolCalls string
			ts        float64
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content,
			&m.ToolCallID, &toolCalls, &m.ToolName, &ts, &m.TokenCount,
			&m.FinishReason, &m.Reasoning, &m.ReasoningDetails); err != nil {
			return nil, fmt.Errorf("sqlite: scan message: %w", err)
		}
		if toolCalls != "" {
			m.ToolCalls = []byte(toolCalls)
		}
		m.Timestamp = fromEpoch(ts)
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

// SearchMessages performs FTS5 full-text search over message content.
func (s *Store) SearchMessages(ctx context.Context, query string, opts *storage.SearchOptions) ([]*storage.SearchResult, error) {
	limit := 50
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// MATCH query against the FTS5 virtual table
	sqlQuery := `
        SELECT m.id, m.session_id, m.role, m.content, m.timestamp,
               snippet(messages_fts, 0, '<mark>', '</mark>', '...', 16) AS snippet,
               rank
        FROM messages_fts
        JOIN messages m ON m.id = messages_fts.rowid
        WHERE messages_fts MATCH ?
    `
	args := []any{query}
	if opts != nil && opts.SessionID != "" {
		sqlQuery += " AND m.session_id = ?"
		args = append(args, opts.SessionID)
	}
	sqlQuery += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: search: %w", err)
	}
	defer rows.Close()

	var results []*storage.SearchResult
	for rows.Next() {
		var (
			m       storage.StoredMessage
			ts      float64
			snippet string
			rank    float64
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content,
			&ts, &snippet, &rank); err != nil {
			return nil, fmt.Errorf("sqlite: scan search result: %w", err)
		}
		m.Timestamp = fromEpoch(ts)
		results = append(results, &storage.SearchResult{
			Message:   &m,
			SessionID: m.SessionID,
			Snippet:   snippet,
			Rank:      rank,
		})
	}
	return results, rows.Err()
}

// ListSessions returns sessions ordered by started_at DESC.
func (s *Store) ListSessions(ctx context.Context, opts *storage.ListOptions) ([]*storage.Session, error) {
	limit := 50
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	query := `SELECT id, source, user_id, model, started_at, title
              FROM sessions`
	var (
		args       []any
		whereAdded bool
	)
	if opts != nil {
		if opts.Source != "" {
			query += " WHERE source = ?"
			args = append(args, opts.Source)
			whereAdded = true
		}
		if opts.UserID != "" {
			if whereAdded {
				query += " AND user_id = ?"
			} else {
				query += " WHERE user_id = ?"
				whereAdded = true
			}
			args = append(args, opts.UserID)
		}
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		var (
			sess      storage.Session
			startedAt float64
		)
		if err := rows.Scan(&sess.ID, &sess.Source, &sess.UserID,
			&sess.Model, &startedAt, &sess.Title); err != nil {
			return nil, fmt.Errorf("sqlite: scan session: %w", err)
		}
		sess.StartedAt = fromEpoch(startedAt)
		sessions = append(sessions, &sess)
	}
	return sessions, rows.Err()
}

// UpdateSystemPrompt caches the prompt for Anthropic prefix-caching reuse.
func (s *Store) UpdateSystemPrompt(ctx context.Context, sessionID, prompt string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET system_prompt = ? WHERE id = ?`, prompt, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite: update system prompt: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// UpdateUsage adds to the running usage counters for a session.
func (s *Store) UpdateUsage(ctx context.Context, sessionID string, usage *storage.UsageUpdate) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE sessions SET
            input_tokens = input_tokens + ?,
            output_tokens = output_tokens + ?,
            cache_read_tokens = cache_read_tokens + ?,
            cache_write_tokens = cache_write_tokens + ?,
            reasoning_tokens = reasoning_tokens + ?,
            actual_cost_usd = actual_cost_usd + ?
        WHERE id = ?`,
		usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens,
		usage.CacheWriteTokens, usage.ReasoningTokens, usage.CostUSD, sessionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update usage: %w", err)
	}
	return nil
}

// Helper to silence unused imports if needed
var _ = sql.ErrNoRows
```

- [ ] **Step 3: Run the tests**

Run: `go test -race ./storage/...`
Expected: PASS. All message and FTS tests pass.

- [ ] **Step 4: Commit**

```bash
git add storage/
git commit -m "feat(storage): implement SQLite message CRUD and FTS5 search"
```

---

## Task 9: Implement WithTx Transaction Wrapper

**Files:**
- Create: `hermes-agent-go/storage/sqlite/tx.go`
- Modify: `hermes-agent-go/storage/sqlite/sqlite_test.go` (add tx tests)

- [ ] **Step 1: Add failing tests for WithTx**

Append to `storage/sqlite/sqlite_test.go`:

```go
func TestWithTxCommitsOnSuccess(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.WithTx(ctx, func(tx storage.Tx) error {
		return tx.CreateSession(ctx, &storage.Session{
			ID:        "tx-commit",
			Source:    "cli",
			Model:     "test",
			StartedAt: time.Now().UTC(),
		})
	})
	require.NoError(t, err)

	sess, err := store.GetSession(ctx, "tx-commit")
	require.NoError(t, err)
	assert.Equal(t, "tx-commit", sess.ID)
}

func TestWithTxRollsBackOnError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	wantErr := errors.New("rollback me")
	err := store.WithTx(ctx, func(tx storage.Tx) error {
		if err := tx.CreateSession(ctx, &storage.Session{
			ID: "tx-rollback", Source: "cli", Model: "test", StartedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)

	_, err = store.GetSession(ctx, "tx-rollback")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	assert.Panics(t, func() {
		_ = store.WithTx(ctx, func(tx storage.Tx) error {
			_ = tx.CreateSession(ctx, &storage.Session{
				ID: "tx-panic", Source: "cli", Model: "test", StartedAt: time.Now().UTC(),
			})
			panic("boom")
		})
	})

	_, err := store.GetSession(ctx, "tx-panic")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
```

You also need to add `"errors"` to the import block of `sqlite_test.go` if not already present.

- [ ] **Step 2: Create `storage/sqlite/tx.go` with the Tx implementation**

```go
// storage/sqlite/tx.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nousresearch/hermes-agent/storage"
)

// WithTx runs fn inside a single SQL transaction. The transaction is
// committed if fn returns nil, rolled back if fn returns an error or panics.
// Panics are re-raised after rollback.
func (s *Store) WithTx(ctx context.Context, fn func(tx storage.Tx) error) error {
	sqlTx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}

	txWrapper := &txImpl{store: s, tx: sqlTx}

	defer func() {
		if r := recover(); r != nil {
			_ = sqlTx.Rollback()
			panic(r) // re-raise after rollback
		}
	}()

	if err := fn(txWrapper); err != nil {
		if rbErr := sqlTx.Rollback(); rbErr != nil {
			return fmt.Errorf("sqlite: rollback after error %v: %w", err, rbErr)
		}
		return err
	}

	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// txImpl implements storage.Tx by wrapping a *sql.Tx.
// It uses the same query logic as Store, but against the tx connection.
type txImpl struct {
	store *Store
	tx    *sql.Tx
}

// queryer is the common interface satisfied by both *sql.DB and *sql.Tx.
// This lets us share query logic between Store and txImpl.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (t *txImpl) CreateSession(ctx context.Context, session *storage.Session) error {
	modelConfig := string(session.ModelConfig)
	if modelConfig == "" {
		modelConfig = "{}"
	}
	_, err := t.tx.ExecContext(ctx, `
        INSERT INTO sessions (
            id, source, user_id, model, model_config, system_prompt,
            parent_session_id, started_at, message_count, tool_call_count,
            input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
            reasoning_tokens, title
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Source, session.UserID, session.Model,
		modelConfig, session.SystemPrompt, session.ParentSessionID,
		toEpoch(session.StartedAt), session.MessageCount, session.ToolCallCount,
		session.Usage.InputTokens, session.Usage.OutputTokens,
		session.Usage.CacheReadTokens, session.Usage.CacheWriteTokens,
		session.Usage.ReasoningTokens, session.Title,
	)
	if err != nil {
		return fmt.Errorf("sqlite tx: create session: %w", err)
	}
	return nil
}

func (t *txImpl) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	// Delegate to the store's non-tx logic but passing the tx connection
	// is tedious — for the minimal plan, reuse the query directly.
	var (
		sess        storage.Session
		modelConfig string
		startedAt   float64
		endedAt     sql.NullFloat64
	)
	err := t.tx.QueryRowContext(ctx, `
        SELECT id, source, user_id, model, model_config, system_prompt,
               parent_session_id, started_at, ended_at, end_reason,
               message_count, tool_call_count,
               input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
               reasoning_tokens, billing_provider, billing_base_url,
               estimated_cost_usd, actual_cost_usd, cost_status, title
        FROM sessions WHERE id = ?`, id,
	).Scan(
		&sess.ID, &sess.Source, &sess.UserID, &sess.Model, &modelConfig,
		&sess.SystemPrompt, &sess.ParentSessionID, &startedAt, &endedAt,
		&sess.EndReason, &sess.MessageCount, &sess.ToolCallCount,
		&sess.Usage.InputTokens, &sess.Usage.OutputTokens,
		&sess.Usage.CacheReadTokens, &sess.Usage.CacheWriteTokens,
		&sess.Usage.ReasoningTokens, &sess.BillingProvider, &sess.BillingBaseURL,
		&sess.EstimatedCost, &sess.ActualCost, &sess.CostStatus, &sess.Title,
	)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite tx: get session: %w", err)
	}
	sess.ModelConfig = []byte(modelConfig)
	sess.StartedAt = fromEpoch(startedAt)
	if endedAt.Valid {
		ts := fromEpoch(endedAt.Float64)
		sess.EndedAt = &ts
	}
	return &sess, nil
}

func (t *txImpl) UpdateSession(ctx context.Context, id string, upd *storage.SessionUpdate) error {
	var setClauses []string
	var args []any
	if upd.EndedAt != nil {
		setClauses = append(setClauses, "ended_at = ?")
		args = append(args, toEpoch(*upd.EndedAt))
	}
	if upd.EndReason != "" {
		setClauses = append(setClauses, "end_reason = ?")
		args = append(args, upd.EndReason)
	}
	if upd.Title != "" {
		setClauses = append(setClauses, "title = ?")
		args = append(args, upd.Title)
	}
	if upd.MessageCount != nil {
		setClauses = append(setClauses, "message_count = ?")
		args = append(args, *upd.MessageCount)
	}
	if len(setClauses) == 0 {
		return nil
	}
	query := "UPDATE sessions SET " + joinWith(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("sqlite tx: update session: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (t *txImpl) AddMessage(ctx context.Context, sessionID string, msg *storage.StoredMessage) error {
	toolCalls := string(msg.ToolCalls)
	_, err := t.tx.ExecContext(ctx, `
        INSERT INTO messages (
            session_id, role, content, tool_call_id, tool_calls, tool_name,
            timestamp, token_count, finish_reason, reasoning, reasoning_details
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, msg.ToolCallID, toolCalls, msg.ToolName,
		toEpoch(msg.Timestamp), msg.TokenCount, msg.FinishReason,
		msg.Reasoning, msg.ReasoningDetails,
	)
	if err != nil {
		return fmt.Errorf("sqlite tx: add message: %w", err)
	}
	return nil
}

func (t *txImpl) UpdateSystemPrompt(ctx context.Context, sessionID, prompt string) error {
	res, err := t.tx.ExecContext(ctx,
		`UPDATE sessions SET system_prompt = ? WHERE id = ?`, prompt, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite tx: update system prompt: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (t *txImpl) UpdateUsage(ctx context.Context, sessionID string, usage *storage.UsageUpdate) error {
	_, err := t.tx.ExecContext(ctx, `
        UPDATE sessions SET
            input_tokens = input_tokens + ?,
            output_tokens = output_tokens + ?,
            cache_read_tokens = cache_read_tokens + ?,
            cache_write_tokens = cache_write_tokens + ?,
            reasoning_tokens = reasoning_tokens + ?,
            actual_cost_usd = actual_cost_usd + ?
        WHERE id = ?`,
		usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens,
		usage.CacheWriteTokens, usage.ReasoningTokens, usage.CostUSD, sessionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite tx: update usage: %w", err)
	}
	return nil
}

// Compile-time check that txImpl satisfies storage.Tx
var _ storage.Tx = (*txImpl)(nil)

// Silence unused-helper warnings if the queryer type isn't used inline
var _ queryer = (*sql.DB)(nil)
var _ queryer = (*sql.Tx)(nil)
```

- [ ] **Step 3: Run the tests**

Run: `go test -race ./storage/...`
Expected: PASS. All three WithTx tests pass, and the session is absent after rollback/panic.

- [ ] **Step 4: Commit**

```bash
git add storage/
git commit -m "feat(storage): add WithTx atomic transaction wrapper"
```

---

## Task 10: Verify Store Satisfies Storage Interface

**Files:**
- Create: `hermes-agent-go/storage/sqlite/interface_test.go`

- [ ] **Step 1: Add a compile-time interface satisfaction check**

```go
// storage/sqlite/interface_test.go
package sqlite

import "github.com/nousresearch/hermes-agent/storage"

// Compile-time assertion that *Store satisfies storage.Storage.
// If any method is missing or has the wrong signature, this fails to compile.
var _ storage.Storage = (*Store)(nil)
```

- [ ] **Step 2: Run the build**

Run: `go build ./storage/...`
Expected: success. If any method is missing or misspelled, this line fails to compile with a clear error naming the missing method.

- [ ] **Step 3: Run full test suite to confirm nothing broke**

Run: `go test -race ./...`
Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add storage/sqlite/interface_test.go
git commit -m "test(storage): add compile-time Storage interface assertion"
```

---

## Task 11: Create Provider Interface and Error Taxonomy

**Files:**
- Create: `hermes-agent-go/provider/provider.go`
- Create: `hermes-agent-go/provider/errors.go`
- Create: `hermes-agent-go/provider/errors_test.go`

- [ ] **Step 1: Write failing tests for error classification**

```go
// provider/errors_test.go
package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableForRateLimit(t *testing.T) {
	err := &Error{Kind: ErrRateLimit, Message: "rate limited"}
	assert.True(t, IsRetryable(err))
}

func TestIsRetryableForServerError(t *testing.T) {
	err := &Error{Kind: ErrServerError, Message: "5xx"}
	assert.True(t, IsRetryable(err))
}

func TestIsRetryableForTimeout(t *testing.T) {
	err := &Error{Kind: ErrTimeout, Message: "timeout"}
	assert.True(t, IsRetryable(err))
}

func TestNotRetryableForAuth(t *testing.T) {
	err := &Error{Kind: ErrAuth, Message: "bad key"}
	assert.False(t, IsRetryable(err))
}

func TestNotRetryableForContentFilter(t *testing.T) {
	err := &Error{Kind: ErrContentFilter, Message: "blocked"}
	assert.False(t, IsRetryable(err))
}

func TestNotRetryableForContextTooLong(t *testing.T) {
	err := &Error{Kind: ErrContextTooLong, Message: "too long"}
	assert.False(t, IsRetryable(err))
}

func TestIsRetryableForNonProviderError(t *testing.T) {
	err := errors.New("random error")
	assert.False(t, IsRetryable(err))
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("network")
	err := &Error{Kind: ErrTimeout, Message: "slow", Cause: cause}
	assert.ErrorIs(t, err, cause)
}
```

- [ ] **Step 2: Create `provider/errors.go`**

```go
// provider/errors.go
package provider

import "errors"

// ErrorKind is the shared error taxonomy across all providers.
type ErrorKind int

const (
	ErrUnknown        ErrorKind = iota // fallback
	ErrRateLimit                       // 429: retry with backoff, eligible for fallback
	ErrAuth                            // 401/403: do not retry
	ErrContentFilter                   // content blocked: do not retry, return to user
	ErrInvalidRequest                  // 400: do not retry, likely a bug
	ErrTimeout                         // request timed out: retry once, then fallback
	ErrServerError                     // 5xx: retry with backoff, eligible for fallback
	ErrContextTooLong                  // context window exceeded: trigger compression
)

// Error is the shared error type returned by all provider implementations.
// Vendor-specific errors are mapped to this type in each provider package.
type Error struct {
	Kind       ErrorKind
	Provider   string // "anthropic", "openai", ...
	StatusCode int    // HTTP status if available, 0 otherwise
	Message    string
	Cause      error // wrapped original error
}

func (e *Error) Error() string {
	if e.Provider != "" {
		return e.Provider + ": " + e.Message
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error is worth retrying (possibly on
// a different provider via the fallback chain).
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

// ErrAllProvidersFailed is returned when the fallback chain exhausts
// all configured providers.
var ErrAllProvidersFailed = errors.New("provider: all providers failed")
```

- [ ] **Step 3: Create `provider/provider.go` with the Provider interface**

```go
// provider/provider.go
package provider

import (
	"context"

	"github.com/nousresearch/hermes-agent/message"
)

// Provider is the interface every LLM backend implements.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns the provider's canonical name (e.g., "anthropic").
	Name() string

	// Complete makes a non-streaming API call.
	Complete(ctx context.Context, req *Request) (*Response, error)

	// Stream makes a streaming API call. The returned Stream is not
	// safe for concurrent use.
	Stream(ctx context.Context, req *Request) (Stream, error)

	// ModelInfo returns capabilities for a specific model. Returns nil
	// if the model is unknown to this provider.
	ModelInfo(model string) *ModelInfo

	// EstimateTokens estimates the token count for a text string using
	// this provider's tokenizer.
	EstimateTokens(model string, text string) (int, error)

	// Available returns true if the provider has valid configuration
	// (e.g., API key is set) and is ready to accept requests.
	Available() bool
}

// Stream is an iterator over streaming API events.
// Call Recv repeatedly until EventDone or an error. Always Close when done.
type Stream interface {
	// Recv blocks until the next event or an error.
	Recv() (*StreamEvent, error)
	// Close releases any resources (connections, decoders).
	Close() error
}

// StreamEventType discriminates between streaming events.
type StreamEventType int

const (
	EventDelta StreamEventType = iota // incremental content or tool call
	EventDone                          // stream finished, Response populated
	EventError                         // error, Err populated
)

// StreamEvent is a single item produced by a streaming provider.
type StreamEvent struct {
	Type     StreamEventType
	Delta    *StreamDelta
	Response *Response
	Err      error
}

// StreamDelta is the incremental content of a streaming event.
type StreamDelta struct {
	Content   string            `json:"content,omitempty"`
	ToolCalls []message.ToolCall `json:"tool_calls,omitempty"`
	Reasoning string            `json:"reasoning,omitempty"`
}

// Request is a provider-agnostic chat completion request.
type Request struct {
	Model         string
	SystemPrompt  string
	Messages      []message.Message
	// Tools, MaxTokens, Temperature, etc. are added in later plans.
	MaxTokens     int
	Temperature   *float64
	TopP          *float64
	StopSequences []string
}

// Response is a provider-agnostic chat completion response.
type Response struct {
	Message      message.Message
	FinishReason string
	Usage        message.Usage
	Model        string // actual model used
}

// ModelInfo describes a model's capabilities.
type ModelInfo struct {
	ContextLength     int
	MaxOutputTokens   int
	SupportsVision    bool
	SupportsTools     bool
	SupportsStreaming bool
	SupportsCaching   bool
	SupportsReasoning bool
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./provider/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/provider.go provider/errors.go provider/errors_test.go
git commit -m "feat(provider): add Provider interface with shared error taxonomy"
```

---

## Task 12: Implement Per-Request FallbackChain

**Files:**
- Create: `hermes-agent-go/provider/fallback.go`
- Create: `hermes-agent-go/provider/fallback_test.go`

- [ ] **Step 1: Write failing tests for FallbackChain**

```go
// provider/fallback_test.go
package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a test helper implementing Provider.
type mockProvider struct {
	name       string
	err        error
	resp       *Response
	callCount  int
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *Request) (Stream, error) {
	return nil, errors.New("not implemented")
}
func (m *mockProvider) ModelInfo(string) *ModelInfo            { return nil }
func (m *mockProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (m *mockProvider) Available() bool                         { return true }

func TestFallbackFirstProviderSucceeds(t *testing.T) {
	p1 := &mockProvider{name: "primary", resp: &Response{Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent("ok")}}}
	p2 := &mockProvider{name: "secondary"}
	chain := NewFallbackChain([]Provider{p1, p2})

	resp, err := chain.Complete(context.Background(), &Request{Model: "test"})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Message.Content.Text())
	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 0, p2.callCount)
}

func TestFallbackFirstFailsRetryableSecondSucceeds(t *testing.T) {
	p1 := &mockProvider{name: "primary", err: &Error{Kind: ErrRateLimit, Message: "429"}}
	p2 := &mockProvider{name: "secondary", resp: &Response{Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent("fallback")}}}
	chain := NewFallbackChain([]Provider{p1, p2})

	resp, err := chain.Complete(context.Background(), &Request{Model: "test"})
	require.NoError(t, err)
	assert.Equal(t, "fallback", resp.Message.Content.Text())
	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 1, p2.callCount)
}

func TestFallbackAllFail(t *testing.T) {
	p1 := &mockProvider{name: "primary", err: &Error{Kind: ErrRateLimit, Message: "429"}}
	p2 := &mockProvider{name: "secondary", err: &Error{Kind: ErrServerError, Message: "500"}}
	chain := NewFallbackChain([]Provider{p1, p2})

	_, err := chain.Complete(context.Background(), &Request{Model: "test"})
	assert.ErrorIs(t, err, ErrAllProvidersFailed)
}

func TestFallbackStopsOnNonRetryable(t *testing.T) {
	p1 := &mockProvider{name: "primary", err: &Error{Kind: ErrAuth, Message: "bad key"}}
	p2 := &mockProvider{name: "secondary"}
	chain := NewFallbackChain([]Provider{p1, p2})

	_, err := chain.Complete(context.Background(), &Request{Model: "test"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAllProvidersFailed)
	assert.Equal(t, 0, p2.callCount)
}
```

- [ ] **Step 2: Create `provider/fallback.go`**

```go
// provider/fallback.go
package provider

import (
	"context"
	"errors"
	"fmt"
)

// FallbackChain tries each provider in order and stops at the first success
// or a non-retryable error. It is single-use and not thread-safe. Create
// a new chain per conversation to avoid shared mutable state.
type FallbackChain struct {
	providers []Provider
}

// NewFallbackChain constructs a chain from an ordered list of providers.
// The first provider is the primary; others are tried in order on failure.
func NewFallbackChain(providers []Provider) *FallbackChain {
	return &FallbackChain{providers: providers}
}

// Complete tries each provider in order until one succeeds or a
// non-retryable error is encountered.
func (fc *FallbackChain) Complete(ctx context.Context, req *Request) (*Response, error) {
	if len(fc.providers) == 0 {
		return nil, errors.New("provider: fallback chain is empty")
	}

	var lastErr error
	for i, p := range fc.providers {
		resp, err := p.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !IsRetryable(err) {
			// Stop on non-retryable errors (auth, content filter, bad request)
			return nil, err
		}
		// Continue to the next provider
		_ = i
	}
	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}

// Stream tries each provider in order for a streaming call.
// Note: once a stream is returned, subsequent failures during Recv() are
// the caller's responsibility to handle (typically by restarting the call).
func (fc *FallbackChain) Stream(ctx context.Context, req *Request) (Stream, error) {
	if len(fc.providers) == 0 {
		return nil, errors.New("provider: fallback chain is empty")
	}

	var lastErr error
	for _, p := range fc.providers {
		stream, err := p.Stream(ctx, req)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		if !IsRetryable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./provider/...`
Expected: PASS. All four fallback tests.

- [ ] **Step 4: Commit**

```bash
git add provider/fallback.go provider/fallback_test.go
git commit -m "feat(provider): add per-request FallbackChain"
```

---

## Task 13: Create Anthropic Provider Skeleton with Wire Types

**Files:**
- Create: `hermes-agent-go/provider/anthropic/anthropic.go`
- Create: `hermes-agent-go/provider/anthropic/types.go`
- Create: `hermes-agent-go/provider/anthropic/errors.go`

- [ ] **Step 1: Create `provider/anthropic/anthropic.go` with Anthropic struct**

```go
// provider/anthropic/anthropic.go
package anthropic

import (
	"errors"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
)

const (
	defaultBaseURL       = "https://api.anthropic.com"
	defaultAPIVersion    = "2023-06-01"
	defaultRequestMaxSec = 300
)

// Anthropic is the provider.Provider implementation for Claude models.
type Anthropic struct {
	apiKey   string
	baseURL  string
	model    string
	client   *http.Client
}

// New constructs an Anthropic provider from config. Returns an error if
// the API key is missing.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("anthropic: api_key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Anthropic{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: defaultRequestMaxSec * time.Second,
		},
	}, nil
}

// Name returns "anthropic".
func (a *Anthropic) Name() string { return "anthropic" }

// Available returns true when an API key is set.
func (a *Anthropic) Available() bool { return a.apiKey != "" }

// ModelInfo returns capabilities for known Claude models. For unknown
// models, returns a conservative default.
func (a *Anthropic) ModelInfo(model string) *provider.ModelInfo {
	// For the minimal plan, all Anthropic models get the same info.
	// Plan 3 will add per-model capability detection.
	return &provider.ModelInfo{
		ContextLength:     200_000,
		MaxOutputTokens:   8_192,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsCaching:   true,
		SupportsReasoning: false,
	}
}

// EstimateTokens provides a rough character-based estimate.
// Plan 3 will replace this with a proper tokenizer.
func (a *Anthropic) EstimateTokens(model, text string) (int, error) {
	// ~4 characters per token is the common rule of thumb for English.
	return len(text) / 4, nil
}
```

- [ ] **Step 2: Create `provider/anthropic/types.go` with wire types**

```go
// provider/anthropic/types.go
package anthropic

// These types match the Anthropic Messages API wire format.
// Ref: https://docs.anthropic.com/en/api/messages

// messagesRequest is the JSON body sent to /v1/messages.
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

// apiMessage is a single message in the Anthropic Messages API format.
// Anthropic uses "user"/"assistant" roles only — "system" is a separate field.
type apiMessage struct {
	Role    string           `json:"role"`
	Content []apiContentItem `json:"content"`
}

// apiContentItem represents one element of an Anthropic message content array.
// Anthropic supports "text", "image", "tool_use", "tool_result".
type apiContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// messagesResponse is the JSON body returned by /v1/messages.
type messagesResponse struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Role         string           `json:"role"`
	Model        string           `json:"model"`
	Content      []apiContentItem `json:"content"`
	StopReason   string           `json:"stop_reason"`
	StopSequence string           `json:"stop_sequence,omitempty"`
	Usage        apiUsage         `json:"usage"`
}

type apiUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// apiErrorResponse is the JSON body for error responses.
type apiErrorResponse struct {
	Type  string   `json:"type"`
	Error apiError `json:"error"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
```

- [ ] **Step 3: Create `provider/anthropic/errors.go` with error mapping**

```go
// provider/anthropic/errors.go
package anthropic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nousresearch/hermes-agent/provider"
)

// mapHTTPError converts an Anthropic HTTP error response to a provider.Error.
func mapHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr apiErrorResponse
	_ = json.Unmarshal(body, &apiErr)

	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("anthropic http %d: %s", resp.StatusCode, string(body))
	}

	kind := provider.ErrUnknown
	switch resp.StatusCode {
	case http.StatusTooManyRequests: // 429
		kind = provider.ErrRateLimit
	case http.StatusUnauthorized, http.StatusForbidden: // 401, 403
		kind = provider.ErrAuth
	case http.StatusBadRequest: // 400
		// Anthropic returns 400 for both invalid requests AND context-too-long
		if apiErr.Error.Type == "invalid_request_error" &&
			containsContextTooLong(msg) {
			kind = provider.ErrContextTooLong
		} else {
			kind = provider.ErrInvalidRequest
		}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout: // 408, 504
		kind = provider.ErrTimeout
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable: // 500, 502, 503
		kind = provider.ErrServerError
	}

	return &provider.Error{
		Kind:       kind,
		Provider:   "anthropic",
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}

// containsContextTooLong checks if an error message indicates context overflow.
func containsContextTooLong(msg string) bool {
	for _, needle := range []string{"context length", "too long", "maximum context", "context window"} {
		if containsIgnoreCase(msg, needle) {
			return true
		}
	}
	return false
}

// containsIgnoreCase is a tiny case-insensitive substring check.
func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Build to verify compilation**

Run: `go build ./provider/anthropic/...`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add provider/anthropic/
git commit -m "feat(anthropic): add provider skeleton with wire types and error mapping"
```

---

## Task 14: Implement Anthropic Complete (Non-Streaming)

**Files:**
- Create: `hermes-agent-go/provider/anthropic/complete.go`
- Create: `hermes-agent-go/provider/anthropic/anthropic_test.go`

- [ ] **Step 1: Write a failing test for Complete using httptest**

```go
// provider/anthropic/anthropic_test.go
package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Anthropic) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p, err := New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test-key",
		BaseURL:  srv.URL,
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)
	return srv, p.(*Anthropic)
}

func TestCompleteHappyPath(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, defaultAPIVersion, r.Header.Get("anthropic-version"))

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req messagesRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "claude-opus-4-6", req.Model)
		assert.Equal(t, "You are helpful.", req.System)
		require.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)

		// Send canned response
		resp := messagesResponse{
			ID:    "msg_01",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
			Content: []apiContentItem{
				{Type: "text", Text: "Hello back"},
			},
			StopReason: "end_turn",
			Usage:      apiUsage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := &provider.Request{
		Model:        "claude-opus-4-6",
		SystemPrompt: "You are helpful.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
		},
		MaxTokens: 1024,
	}

	resp, err := a.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, message.RoleAssistant, resp.Message.Role)
	assert.Equal(t, "Hello back", resp.Message.Content.Text())
	assert.Equal(t, "end_turn", resp.FinishReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
}

func TestCompleteMapsRateLimitError(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Type:  "error",
			Error: apiError{Type: "rate_limit_error", Message: "rate limited"},
		})
	})

	_, err := a.Complete(context.Background(), &provider.Request{
		Model:    "claude-opus-4-6",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)

	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrRateLimit, pErr.Kind)
	assert.Equal(t, 429, pErr.StatusCode)
	assert.True(t, provider.IsRetryable(err))
}
```

- [ ] **Step 2: Create `provider/anthropic/complete.go`**

```go
// provider/anthropic/complete.go
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Complete sends a non-streaming request to /v1/messages.
func (a *Anthropic) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	apiReq := a.buildRequest(req, false)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", defaultAPIVersion)

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		// Network errors map to ErrServerError (retryable)
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "anthropic",
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(httpResp)
	}

	var apiResp messagesResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	return a.convertResponse(&apiResp), nil
}

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

	for _, m := range req.Messages {
		// Anthropic only supports "user" and "assistant" roles in messages.
		// System messages go to the top-level System field.
		// Tool messages are handled in Plan 2.
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

// contentToAPIItems converts message.Content to Anthropic's content array format.
func contentToAPIItems(c message.Content) []apiContentItem {
	if c.IsText() {
		return []apiContentItem{{Type: "text", Text: c.Text()}}
	}
	items := make([]apiContentItem, 0, len(c.Blocks()))
	for _, b := range c.Blocks() {
		if b.Type == "text" {
			items = append(items, apiContentItem{Type: "text", Text: b.Text})
		}
		// Image and tool blocks handled in Plan 2/5.
	}
	return items
}

// convertResponse converts an Anthropic wire response to the provider shape.
func (a *Anthropic) convertResponse(apiResp *messagesResponse) *provider.Response {
	// Concatenate all text blocks into a single text response.
	// In Plan 2, tool_use blocks will be converted to ToolCalls.
	var text string
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(text),
		},
		FinishReason: apiResp.StopReason,
		Usage: message.Usage{
			InputTokens:     apiResp.Usage.InputTokens,
			OutputTokens:    apiResp.Usage.OutputTokens,
			CacheReadTokens: apiResp.Usage.CacheReadInputTokens,
			CacheWriteTokens: apiResp.Usage.CacheCreationInputTokens,
		},
		Model: apiResp.Model,
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./provider/anthropic/...`
Expected: PASS. Both tests should succeed.

- [ ] **Step 4: Commit**

```bash
git add provider/anthropic/complete.go provider/anthropic/anthropic_test.go
git commit -m "feat(anthropic): implement Complete with error mapping"
```

---

## Task 15: Implement Anthropic Stream with SSE Parsing

**Files:**
- Create: `hermes-agent-go/provider/anthropic/stream.go`
- Modify: `hermes-agent-go/provider/anthropic/anthropic_test.go` (add stream test)

- [ ] **Step 1: Append a failing stream test**

Append to `provider/anthropic/anthropic_test.go`:

```go
func TestStreamHappyPath(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Write SSE events matching Anthropic format
		events := []string{
			`event: message_start
data: {"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-opus-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

`,
			`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

`,
			`event: content_block_stop
data: {"type":"content_block_stop","index":0}

`,
			`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

`,
			`event: message_stop
data: {"type":"message_stop"}

`,
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := a.Stream(context.Background(), &provider.Request{
		Model:    "claude-opus-4-6",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var text string
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
		if ev.Type == provider.EventDelta && ev.Delta != nil {
			text += ev.Delta.Content
		}
	}

	assert.Equal(t, "Hello world", text)
	require.NotNil(t, doneEvent)
	require.NotNil(t, doneEvent.Response)
	assert.Equal(t, "end_turn", doneEvent.Response.FinishReason)
	assert.Equal(t, 10, doneEvent.Response.Usage.InputTokens)
	assert.Equal(t, 5, doneEvent.Response.Usage.OutputTokens)
}
```

- [ ] **Step 2: Create `provider/anthropic/stream.go`**

```go
// provider/anthropic/stream.go
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// SSE scanner buffer size: 10MB is enough for any realistic LLM streaming chunk.
// Default bufio.Scanner limit is 64KB which corrupts large tool-call streams.
const sseMaxLineBytes = 10 * 1024 * 1024

// streamEvent names used by Anthropic SSE.
const (
	anthropicEventMessageStart      = "message_start"
	anthropicEventContentBlockStart = "content_block_start"
	anthropicEventContentBlockDelta = "content_block_delta"
	anthropicEventContentBlockStop  = "content_block_stop"
	anthropicEventMessageDelta      = "message_delta"
	anthropicEventMessageStop       = "message_stop"
	anthropicEventPing              = "ping"
	anthropicEventError             = "error"
)

// Stream starts a streaming request to /v1/messages.
func (a *Anthropic) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	apiReq := a.buildRequest(req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", defaultAPIVersion)

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "anthropic",
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	if httpResp.StatusCode != http.StatusOK {
		err := mapHTTPError(httpResp)
		_ = httpResp.Body.Close()
		return nil, err
	}

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineBytes)
	// Custom split: SSE uses "\n\n" to separate events
	scanner.Split(splitSSEEvents)

	return &anthropicStream{
		resp:    httpResp,
		scanner: scanner,
		usage:   message.Usage{},
	}, nil
}

// splitSSEEvents is a bufio.SplitFunc that yields complete SSE events.
// An event is terminated by a blank line (\n\n).
func splitSSEEvents(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if idx := bytes.Index(data, []byte("\n\n")); idx >= 0 {
		return idx + 2, data[:idx], nil
	}
	// Also accept "\r\n\r\n" terminators
	if idx := bytes.Index(data, []byte("\r\n\r\n")); idx >= 0 {
		return idx + 4, data[:idx], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// anthropicStream implements provider.Stream.
// NOT thread-safe. One consumer only.
type anthropicStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	// accumulated state
	text          strings.Builder
	model         string
	finishReason  string
	usage         message.Usage
	done          bool
	closed        bool
}

// Recv reads the next SSE event and returns a StreamEvent.
func (s *anthropicStream) Recv() (*provider.StreamEvent, error) {
	if s.done {
		return nil, io.EOF
	}
	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				return nil, fmt.Errorf("anthropic stream: scan: %w", err)
			}
			// Clean EOF — synthesize a Done event
			s.done = true
			return s.buildDoneEvent(), nil
		}
		eventType, data := parseSSEEvent(s.scanner.Bytes())
		if eventType == "" {
			continue
		}
		ev, err := s.handleEvent(eventType, data)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			return ev, nil
		}
		// Keep scanning if the event produced no output (e.g., ping)
	}
}

// parseSSEEvent extracts "event:" and "data:" fields from an SSE frame.
func parseSSEEvent(frame []byte) (eventType string, data []byte) {
	for _, line := range bytes.Split(frame, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(line[len("event:"):]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			data = bytes.TrimSpace(line[len("data:"):])
		}
	}
	return eventType, data
}

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
		return nil, nil
	case anthropicEventContentBlockDelta:
		var ev struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse delta: %w", err)
		}
		if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
			s.text.WriteString(ev.Delta.Text)
			return &provider.StreamEvent{
				Type: provider.EventDelta,
				Delta: &provider.StreamDelta{
					Content: ev.Delta.Text,
				},
			}, nil
		}
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
	case anthropicEventPing, anthropicEventContentBlockStart, anthropicEventContentBlockStop:
		// Ignore these — they carry no output for our purposes
		return nil, nil
	default:
		return nil, nil
	}
}

// buildDoneEvent creates the terminal EventDone with accumulated state.
func (s *anthropicStream) buildDoneEvent() *provider.StreamEvent {
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent(s.text.String()),
			},
			FinishReason: s.finishReason,
			Usage:        s.usage,
			Model:        s.model,
		},
	}
}

// Close releases the underlying HTTP connection.
func (s *anthropicStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}

// Compile-time assertion that *Anthropic satisfies provider.Provider.
var _ provider.Provider = (*Anthropic)(nil)
```

- [ ] **Step 2: Run the tests**

Run: `go test -race ./provider/anthropic/...`
Expected: PASS. The stream test should collect "Hello world" and receive a Done event with correct usage.

- [ ] **Step 3: Commit**

```bash
git add provider/anthropic/stream.go provider/anthropic/anthropic_test.go
git commit -m "feat(anthropic): implement SSE streaming with 10MB buffer"
```

---

## Task 16: Create Agent Budget

**Files:**
- Create: `hermes-agent-go/agent/budget.go`
- Create: `hermes-agent-go/agent/budget_test.go`

- [ ] **Step 1: Write failing tests for Budget**

```go
// agent/budget_test.go
package agent

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBudgetConsume(t *testing.T) {
	b := NewBudget(3)
	assert.Equal(t, 3, b.Remaining())
	assert.True(t, b.Consume())
	assert.Equal(t, 2, b.Remaining())
	assert.True(t, b.Consume())
	assert.True(t, b.Consume())
	assert.False(t, b.Consume(), "budget exhausted")
	assert.Equal(t, -1, b.Remaining())
}

func TestBudgetRefund(t *testing.T) {
	b := NewBudget(2)
	b.Consume()
	b.Consume()
	assert.Equal(t, 0, b.Remaining())
	b.Refund()
	assert.Equal(t, 1, b.Remaining())
}

func TestBudgetRatio(t *testing.T) {
	b := NewBudget(10)
	assert.Equal(t, 0.0, b.Ratio())
	for i := 0; i < 7; i++ {
		b.Consume()
	}
	assert.InDelta(t, 0.7, b.Ratio(), 0.01)
}

func TestBudgetConcurrentConsume(t *testing.T) {
	b := NewBudget(1000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				b.Consume()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, b.Remaining())
}
```

- [ ] **Step 2: Create `agent/budget.go`**

```go
// agent/budget.go
package agent

import "sync/atomic"

// Budget tracks the remaining iteration budget for a conversation.
// Thread-safe via atomic.Int32. The zero value is not valid; use NewBudget.
type Budget struct {
	max       int
	remaining atomic.Int32
}

// NewBudget constructs a Budget with max iterations.
func NewBudget(max int) *Budget {
	b := &Budget{max: max}
	b.remaining.Store(int32(max))
	return b
}

// Consume attempts to use one iteration. Returns true if the budget was
// decremented while non-negative, false if it went negative.
func (b *Budget) Consume() bool {
	return b.remaining.Add(-1) >= 0
}

// Refund returns one iteration to the budget (used for tools that invoke
// code execution where programmatic tool calls should not count).
func (b *Budget) Refund() {
	b.remaining.Add(1)
}

// Remaining returns the current remaining iteration count.
// May be negative if Consume was called after exhaustion.
func (b *Budget) Remaining() int {
	return int(b.remaining.Load())
}

// Ratio returns the fraction of the budget consumed, from 0.0 (fresh)
// to 1.0 (exhausted).
func (b *Budget) Ratio() float64 {
	if b.max == 0 {
		return 0
	}
	used := b.max - int(b.remaining.Load())
	return float64(used) / float64(b.max)
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./agent/...`
Expected: PASS, including the concurrent test under the race detector.

- [ ] **Step 4: Commit**

```bash
git add agent/budget.go agent/budget_test.go
git commit -m "feat(agent): add thread-safe iteration Budget"
```

---

## Task 17: Create Minimal PromptBuilder

**Files:**
- Create: `hermes-agent-go/agent/prompt.go`
- Create: `hermes-agent-go/agent/prompt_test.go`

- [ ] **Step 1: Write failing tests for PromptBuilder**

```go
// agent/prompt_test.go
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptBuilderIncludesIdentity(t *testing.T) {
	pb := NewPromptBuilder("cli")
	prompt := pb.Build(&PromptOptions{Model: "claude-opus-4-6"})
	assert.Contains(t, prompt, "Hermes Agent")
	assert.Contains(t, prompt, "Nous Research")
}

func TestPromptBuilderPlatformHint(t *testing.T) {
	pb := NewPromptBuilder("telegram")
	prompt := pb.Build(&PromptOptions{Model: "claude-opus-4-6"})
	// Platform hints are added in Plan 6+. Minimum: the platform name appears.
	assert.NotEmpty(t, prompt)
}

func TestPromptBuilderStable(t *testing.T) {
	// Building the same prompt twice yields identical output.
	// This is required for Anthropic prefix caching.
	pb := NewPromptBuilder("cli")
	opts := &PromptOptions{Model: "claude-opus-4-6"}
	first := pb.Build(opts)
	second := pb.Build(opts)
	assert.Equal(t, first, second)
}
```

- [ ] **Step 2: Create `agent/prompt.go`**

```go
// agent/prompt.go
package agent

import "strings"

// defaultIdentity is the base personality/identity block.
// Ported from the Python hermes agent/prompt_builder.py DEFAULT_AGENT_IDENTITY.
const defaultIdentity = `You are Hermes Agent, created by Nous Research.

You are a helpful, knowledgeable AI assistant. You are direct and efficient.
You respond with markdown formatting when it aids clarity.`

// PromptOptions parameterize prompt generation.
// In later plans this will expand to include memory, skills, context files, etc.
type PromptOptions struct {
	Model       string
	SkipContext bool
}

// PromptBuilder assembles system prompts for the agent engine.
// Stateless — safe to share a single instance across conversations.
type PromptBuilder struct {
	platform string
}

// NewPromptBuilder creates a PromptBuilder for a specific platform.
// Valid platforms: "cli", "telegram", "discord", etc.
func NewPromptBuilder(platform string) *PromptBuilder {
	return &PromptBuilder{platform: platform}
}

// Build assembles the system prompt. The output is stable for equivalent
// inputs — this is required for Anthropic prefix caching to work.
func (pb *PromptBuilder) Build(opts *PromptOptions) string {
	var parts []string
	parts = append(parts, defaultIdentity)

	// Context files, memory guidance, skills guidance, platform hints,
	// and injection protection are added in later plans.
	// For Plan 1 we just want a stable minimal prompt.

	return strings.Join(parts, "\n\n")
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./agent/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add agent/prompt.go agent/prompt_test.go
git commit -m "feat(agent): add minimal stateless PromptBuilder"
```

---

## Task 18: Create Engine Struct and Single-Turn RunConversation

**Files:**
- Create: `hermes-agent-go/agent/engine.go`
- Create: `hermes-agent-go/agent/conversation.go`
- Create: `hermes-agent-go/agent/engine_test.go`

- [ ] **Step 1: Write failing tests for Engine.RunConversation**

```go
// agent/engine_test.go
package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider returns a canned response for tests.
type fakeProvider struct {
	name     string
	response *provider.Response
	err      error
	streamFn func() (provider.Stream, error)
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}
func (f *fakeProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	if f.streamFn != nil {
		return f.streamFn()
	}
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo            { return nil }
func (f *fakeProvider) EstimateTokens(string, string) (int, error)      { return 0, nil }
func (f *fakeProvider) Available() bool                                  { return true }

// fakeStream returns a single delta then Done.
type fakeStream struct {
	events []*provider.StreamEvent
	idx    int
}

func (s *fakeStream) Recv() (*provider.StreamEvent, error) {
	if s.idx >= len(s.events) {
		return nil, errors.New("EOF")
	}
	ev := s.events[s.idx]
	s.idx++
	return ev, nil
}
func (s *fakeStream) Close() error { return nil }

func newFakeStreamingProvider(text string) *fakeProvider {
	return &fakeProvider{
		name: "fake",
		streamFn: func() (provider.Stream, error) {
			return &fakeStream{
				events: []*provider.StreamEvent{
					{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: text}},
					{
						Type: provider.EventDone,
						Response: &provider.Response{
							Message: message.Message{
								Role:    message.RoleAssistant,
								Content: message.TextContent(text),
							},
							FinishReason: "end_turn",
							Usage:        message.Usage{InputTokens: 5, OutputTokens: 3},
						},
					},
				},
			}, nil
		},
	}
}

func TestEngineSingleTurn(t *testing.T) {
	p := newFakeStreamingProvider("Hello back!")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hi",
		SessionID:   "test-session",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello back!", result.Response.Content.Text())
	assert.Equal(t, 1, result.Iterations)
	require.Len(t, result.Messages, 2)
	assert.Equal(t, message.RoleUser, result.Messages[0].Role)
	assert.Equal(t, message.RoleAssistant, result.Messages[1].Role)
}

func TestEngineRespectsContextCancellation(t *testing.T) {
	p := newFakeStreamingProvider("should not get here")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before run

	_, err := e.RunConversation(ctx, &RunOptions{
		UserMessage: "hi",
		SessionID:   "cancelled-session",
	})
	assert.ErrorIs(t, err, context.Canceled)
}
```

- [ ] **Step 2: Create `agent/engine.go`**

```go
// agent/engine.go
package agent

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
)

// Engine is single-use per conversation. NOT thread-safe.
// The gateway creates a fresh Engine per incoming message.
// The CLI creates a fresh Engine per /run invocation.
type Engine struct {
	provider provider.Provider
	storage  storage.Storage
	config   config.AgentConfig // value, not pointer — immutable snapshot
	platform string
	prompt   *PromptBuilder

	// Callbacks — optional. Nil means no-op.
	onStreamDelta func(delta *provider.StreamDelta)
}

// NewEngine constructs a fresh Engine for one conversation.
// storage may be nil if the caller does not want persistence (e.g., unit tests).
func NewEngine(p provider.Provider, s storage.Storage, cfg config.AgentConfig, platform string) *Engine {
	return &Engine{
		provider: p,
		storage:  s,
		config:   cfg,
		platform: platform,
		prompt:   NewPromptBuilder(platform),
	}
}

// SetStreamDeltaCallback registers a callback invoked for each streaming delta.
// Must be called before RunConversation. Calling after is undefined behavior.
func (e *Engine) SetStreamDeltaCallback(fn func(delta *provider.StreamDelta)) {
	e.onStreamDelta = fn
}

// RunOptions parameterizes a conversation run.
type RunOptions struct {
	UserMessage string
	History     []message.Message // previous conversation turns, if any
	SessionID   string
	UserID      string
	Model       string
}

// ConversationResult is returned by RunConversation.
type ConversationResult struct {
	Response   message.Message
	Messages   []message.Message // full history after the run
	SessionID  string
	Usage      message.Usage
	Iterations int
}
```

- [ ] **Step 3: Create `agent/conversation.go` with RunConversation**

```go
// agent/conversation.go
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

// RunConversation executes a single conversation turn: sends the user
// message to the LLM, collects the streaming response, persists messages,
// and returns the assistant's reply.
//
// For Plan 1, this is single-turn only — no tools, no loop. Tool use is
// added in Plan 2, which turns this into the full iteration loop.
func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model := opts.Model
	if model == "" {
		model = "claude-opus-4-6"
	}

	// Build request
	history := append([]message.Message{}, opts.History...)
	history = append(history, message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent(opts.UserMessage),
	})

	systemPrompt := e.prompt.Build(&PromptOptions{Model: model})

	req := &provider.Request{
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     history,
		MaxTokens:    4096,
	}

	// Persist the session and user message if storage is configured
	if e.storage != nil {
		if err := e.ensureSession(ctx, opts, systemPrompt, model); err != nil {
			return nil, fmt.Errorf("engine: ensure session: %w", err)
		}
		if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
			return nil, fmt.Errorf("engine: persist user message: %w", err)
		}
	}

	// Stream the response
	stream, err := e.provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("engine: start stream: %w", err)
	}
	defer stream.Close()

	var doneEvent *provider.StreamEvent
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ev, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
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
			doneEvent = ev
			goto streamComplete
		case provider.EventError:
			return nil, ev.Err
		}
	}
streamComplete:

	if doneEvent == nil || doneEvent.Response == nil {
		return nil, errors.New("engine: stream ended without a done event")
	}

	// Append the assistant response to history
	history = append(history, doneEvent.Response.Message)

	// Persist the assistant message and usage atomically
	if e.storage != nil {
		err := e.storage.WithTx(ctx, func(tx storage.Tx) error {
			m := &history[len(history)-1]
			if err := e.persistMessageTx(ctx, tx, opts.SessionID, m); err != nil {
				return err
			}
			return tx.UpdateUsage(ctx, opts.SessionID, &storage.UsageUpdate{
				InputTokens:      doneEvent.Response.Usage.InputTokens,
				OutputTokens:     doneEvent.Response.Usage.OutputTokens,
				CacheReadTokens:  doneEvent.Response.Usage.CacheReadTokens,
				CacheWriteTokens: doneEvent.Response.Usage.CacheWriteTokens,
			})
		})
		if err != nil {
			return nil, fmt.Errorf("engine: persist response: %w", err)
		}
	}

	return &ConversationResult{
		Response:   doneEvent.Response.Message,
		Messages:   history,
		SessionID:  opts.SessionID,
		Usage:      doneEvent.Response.Usage,
		Iterations: 1,
	}, nil
}

// ensureSession creates a new session row if it doesn't exist yet.
func (e *Engine) ensureSession(ctx context.Context, opts *RunOptions, systemPrompt, model string) error {
	_, err := e.storage.GetSession(ctx, opts.SessionID)
	if err == nil {
		return nil // session exists
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
	// Serialize Content to JSON for storage
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

- [ ] **Step 4: Run tests**

Run: `go test -race ./agent/...`
Expected: PASS. Both engine tests pass.

- [ ] **Step 5: Commit**

```bash
git add agent/engine.go agent/conversation.go agent/engine_test.go
git commit -m "feat(agent): implement single-turn RunConversation with storage persistence"
```

---

## Task 19: Create CLI App Skeleton with Cobra

**Files:**
- Create: `hermes-agent-go/cli/app.go`
- Create: `hermes-agent-go/cli/root.go`
- Create: `hermes-agent-go/cli/run.go`
- Modify: `hermes-agent-go/cmd/hermes/main.go`

- [ ] **Step 1: Add cobra dependency**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 2: Create `cli/app.go`**

```go
// cli/app.go
package cli

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/storage"
)

// App bundles the shared resources (config, storage) across cobra commands.
type App struct {
	Config  *config.Config
	Storage storage.Storage
}

// NewApp constructs an App by loading config. Storage is opened lazily
// by the command that needs it.
func NewApp() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &App{Config: cfg}, nil
}

// Close releases all held resources.
func (a *App) Close() error {
	if a.Storage != nil {
		return a.Storage.Close()
	}
	return nil
}
```

- [ ] **Step 3: Create `cli/root.go`**

```go
// cli/root.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is injected at build time via -ldflags "-X main.Version=..."
// It is set from main.go.
var Version = "dev"

// NewRootCmd builds the cobra command tree.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "hermes",
		Short:         "Hermes Agent — Go port of the hermes AI agent framework",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newRunCmd(app),
		newVersionCmd(),
	)

	// Default subcommand: if no args, run the REPL
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return runREPL(cmd.Context(), app)
	}

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "hermes-agent %s\n", Version)
			return nil
		},
	}
}
```

- [ ] **Step 4: Create `cli/run.go` (cobra command wrapper — REPL in next task)**

```go
// cli/run.go
package cli

import (
	"github.com/spf13/cobra"
)

// newRunCmd creates the "hermes run" command. Both "hermes" and
// "hermes run" launch the same REPL.
func newRunCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the interactive REPL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runREPL(cmd.Context(), app)
		},
	}
}
```

- [ ] **Step 5: Update `cmd/hermes/main.go` to wire everything**

```go
// cmd/hermes/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nousresearch/hermes-agent/cli"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	cli.Version = Version

	app, err := cli.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hermes: init: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = app.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := cli.NewRootCmd(app)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "hermes: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Verify the binary builds (REPL function doesn't exist yet — that's the next task)**

At this point the build will FAIL because `runREPL` is referenced but not defined. That's expected — Task 20 adds it.

Skip the commit for this task. Task 20 completes the CLI in one commit.

---

## Task 20: Implement Basic REPL

**Files:**
- Create: `hermes-agent-go/cli/repl.go`

- [ ] **Step 1: Create `cli/repl.go` with runREPL**

```go
// cli/repl.go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
)

const banner = `
╭─────────────────────────╮
│    HERMES AGENT         │
╰─────────────────────────╯
`

// runREPL starts the interactive read-eval-print loop.
// For Plan 1 this is a minimal bufio.Scanner-based REPL.
// Plan 4 replaces this with a bubbletea TUI.
func runREPL(ctx context.Context, app *App) error {
	// Open storage lazily
	if err := ensureStorage(app); err != nil {
		return err
	}

	// Build the Anthropic provider from config
	anthropicCfg, ok := app.Config.Providers["anthropic"]
	if !ok || anthropicCfg.APIKey == "" {
		return fmt.Errorf("hermes: anthropic provider is not configured. Set api_key in ~/.hermes/config.yaml or ANTHROPIC_API_KEY env var")
	}

	// Allow ANTHROPIC_API_KEY env override
	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" && anthropicCfg.APIKey == "" {
		anthropicCfg.APIKey = envKey
	}
	if anthropicCfg.Model == "" {
		anthropicCfg.Model = defaultModelFromString(app.Config.Model)
	}

	p, err := anthropic.New(anthropicCfg)
	if err != nil {
		return fmt.Errorf("hermes: create provider: %w", err)
	}

	// Print the banner and context
	sessionID := uuid.NewString()
	fmt.Print(banner)
	fmt.Printf("  %s · session %s\n\n", anthropicCfg.Model, sessionID[:8])

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var history []message.Message
	turnCount := 0
	totalUsage := message.Usage{}

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			// Ctrl+D or EOF
			err := scanner.Err()
			if err != nil && err != io.EOF {
				return err
			}
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			break
		}
		if line == "/help" {
			fmt.Println("Commands: /exit /quit /help")
			continue
		}

		// Build engine fresh per turn (single-use semantics)
		engine := agent.NewEngine(p, app.Storage, app.Config.Agent, "cli")

		// Register streaming callback: print deltas as they arrive
		engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
			if d != nil && d.Content != "" {
				fmt.Print(d.Content)
			}
		})

		fmt.Println() // newline before streaming output

		result, err := engine.RunConversation(ctx, &agent.RunOptions{
			UserMessage: line,
			History:     history,
			SessionID:   sessionID,
			Model:       anthropicCfg.Model,
		})
		if err != nil {
			if ctx.Err() != nil {
				fmt.Println("\n[interrupted]")
				break
			}
			fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
			continue
		}

		fmt.Println() // newline after streaming
		history = result.Messages
		turnCount++
		totalUsage.InputTokens += result.Usage.InputTokens
		totalUsage.OutputTokens += result.Usage.OutputTokens
	}

	// Session summary
	fmt.Printf("\nSession #%s: %d messages, %d in / %d out tokens · saved to %s\n",
		sessionID[:8], turnCount*2,
		totalUsage.InputTokens, totalUsage.OutputTokens,
		app.Config.Storage.SQLitePath,
	)
	return nil
}

// ensureStorage opens the SQLite store on first use and runs migrations.
func ensureStorage(app *App) error {
	if app.Storage != nil {
		return nil
	}
	path := app.Config.Storage.SQLitePath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".hermes", "state.db")
	}

	// Ensure the parent directory exists
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("hermes: create db dir: %w", err)
		}
	}

	store, err := sqlite.Open(path)
	if err != nil {
		return fmt.Errorf("hermes: open storage: %w", err)
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		return fmt.Errorf("hermes: migrate: %w", err)
	}
	app.Storage = store
	return nil
}

// defaultModelFromString parses "anthropic/claude-opus-4-6" into just "claude-opus-4-6".
func defaultModelFromString(s string) string {
	if idx := strings.Index(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// (Unused import silencer when time isn't directly referenced in this file.)
var _ = time.Now
```

- [ ] **Step 2: Add the uuid dependency**

```bash
go get github.com/google/uuid
```

- [ ] **Step 3: Build the binary**

Run: `go build -o bin/hermes ./cmd/hermes`
Expected: success, no errors.

- [ ] **Step 4: Run the binary to verify it starts**

Run: `./bin/hermes version`
Expected output: `hermes-agent dev`

Running `./bin/hermes` without an API key should print the clear error:
`hermes: anthropic provider is not configured. Set api_key in ~/.hermes/config.yaml or ANTHROPIC_API_KEY env var`

- [ ] **Step 5: Commit**

```bash
git add cli/ cmd/hermes/main.go go.mod go.sum
git commit -m "feat(cli): wire cobra + basic REPL with Anthropic provider"
```

---

## Task 21: End-to-End Smoke Test with httptest

**Files:**
- Create: `hermes-agent-go/cli/repl_test.go`

- [ ] **Step 1: Write an end-to-end test that drives the REPL against a mock Anthropic server**

```go
// cli/repl_test.go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndSingleTurn proves the full stack works: user message →
// anthropic (mock) → stream → storage → ConversationResult.
func TestEndToEndSingleTurn(t *testing.T) {
	// Mock Anthropic server that returns a single-event SSE stream
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"stream":true`)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":8,\"output_tokens\":0}}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi!\"}}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	// Build provider pointing at the mock server
	p, err := anthropic.New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test",
		BaseURL:  srv.URL,
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	// Fresh SQLite store in a temp dir
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	defer store.Close()

	// Capture streaming output
	var streamed bytes.Buffer
	engine := agent.NewEngine(p, store, config.AgentConfig{MaxTurns: 10}, "cli")
	engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
		if d != nil {
			streamed.WriteString(d.Content)
		}
	})

	result, err := engine.RunConversation(context.Background(), &agent.RunOptions{
		UserMessage: "hi",
		SessionID:   "e2e-session",
		Model:       "claude-opus-4-6",
	})
	require.NoError(t, err)

	// Verify the response
	assert.Equal(t, "Hi!", result.Response.Content.Text())
	assert.Equal(t, "Hi!", streamed.String())
	assert.Equal(t, 8, result.Usage.InputTokens)
	assert.Equal(t, 2, result.Usage.OutputTokens)

	// Verify both messages were persisted
	msgs, err := store.GetMessages(context.Background(), "e2e-session", 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)

	// Verify session usage was updated
	sess, err := store.GetSession(context.Background(), "e2e-session")
	require.NoError(t, err)
	assert.Equal(t, 8, sess.Usage.InputTokens)
	assert.Equal(t, 2, sess.Usage.OutputTokens)

	// Verify content is JSON-encoded message.Content
	var userContent message.Content
	require.NoError(t, json.Unmarshal([]byte(msgs[0].Content), &userContent))
	assert.Equal(t, "hi", userContent.Text())

	// Silence unused-import warnings in test files
	_ = os.Stdout
	_ = strings.TrimSpace
}
```

- [ ] **Step 2: Run the full test suite**

Run: `go test -race ./...`
Expected: PASS. All tests across every package should pass.

- [ ] **Step 3: Build and verify the binary one more time**

Run: `make build && ./bin/hermes version`
Expected: clean build, version printed.

- [ ] **Step 4: Run golangci-lint on the whole module**

Run: `golangci-lint run ./...`
Expected: no issues. Fix any reported issues before committing.

- [ ] **Step 5: Commit**

```bash
git add cli/repl_test.go
git commit -m "test(cli): add end-to-end smoke test with mock Anthropic server"
```

---

## Task 22: Add GitHub Actions CI Workflow

**Files:**
- Create: `hermes-agent-go/.github/workflows/test.yml`

- [ ] **Step 1: Create the GitHub Actions workflow**

```yaml
# .github/workflows/test.yml
name: test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true

      - name: Download dependencies
        working-directory: hermes-agent-go
        run: go mod download

      - name: Build
        working-directory: hermes-agent-go
        run: go build ./...

      - name: Test with race detector
        working-directory: hermes-agent-go
        run: go test -race -cover ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          working-directory: hermes-agent-go
```

- [ ] **Step 2: Commit**

```bash
git add .github/
git commit -m "chore(ci): add GitHub Actions test workflow"
```

---

## Task 23: Final Build Verification and Plan 1 Wrap-Up

**Files:** None created; this task is a verification checklist.

- [ ] **Step 1: Run the full test suite**

Run: `cd hermes-agent-go && go test -race -cover ./...`
Expected: All packages pass. Total time < 30 seconds. Coverage should be reported per package.

- [ ] **Step 2: Run golangci-lint**

Run: `cd hermes-agent-go && golangci-lint run ./...`
Expected: No issues reported.

- [ ] **Step 3: Build the release binary**

Run: `cd hermes-agent-go && make build`
Expected: `bin/hermes` created. No build errors.

- [ ] **Step 4: Run the binary's version command**

Run: `./bin/hermes version`
Expected: `hermes-agent <version>` printed.

- [ ] **Step 5: Manual smoke test with a real Anthropic API key (OPTIONAL, requires valid key)**

```bash
mkdir -p ~/.hermes
cat > ~/.hermes/config.yaml <<'EOF'
model: anthropic/claude-opus-4-6
providers:
  anthropic:
    provider: anthropic
    api_key: env:ANTHROPIC_API_KEY
    model: claude-opus-4-6
EOF

export ANTHROPIC_API_KEY=sk-ant-...
./bin/hermes
```

Expected:
```
╭─────────────────────────╮
│    HERMES AGENT         │
╰─────────────────────────╯
  claude-opus-4-6 · session abc12345

> hello
Hi there! How can I help you today?

> /exit

Session #abc12345: 2 messages, N in / M out tokens · saved to /Users/.../.hermes/state.db
```

- [ ] **Step 6: Verify the SQLite database was created and has the expected schema**

```bash
sqlite3 ~/.hermes/state.db '.schema'
```

Expected: `sessions`, `messages`, `messages_fts`, and the FTS triggers are all present.

- [ ] **Step 7: Confirm there are no untracked files and all commits are in place**

```bash
cd hermes-agent-go && git status
git log --oneline
```

Expected: clean working tree, commit history matching the tasks in this plan.

- [ ] **Step 8: Celebrate. Plan 1 is done. Proceed to Plan 2 (tool system).**

---

## Plan 1 Self-Review Notes

**Spec coverage:**
- Core types (message package) — covered in Tasks 2-3
- Config loading — Task 4
- Storage interface + SQLite + WithTx — Tasks 5-10
- Provider interface + Error taxonomy + FallbackChain — Tasks 11-12
- Anthropic provider (Complete + Stream) — Tasks 13-15
- Agent engine (Budget + PromptBuilder + RunConversation single-turn) — Tasks 16-18
- CLI skeleton + REPL — Tasks 19-20
- End-to-end smoke test — Task 21
- CI — Task 22

**Explicitly out of scope for Plan 1 (covered in later plans):**
- Tools of any kind (Plan 2)
- Other 9 providers + fallback across providers (Plan 3)
- bubbletea TUI (Plan 4)
- MCP, memory providers, context compression (Plan 6)
- Gateway and platform adapters (Plan 7+)
- Port-fidelity golden file tests (Plan 2 when we have more to match against)
- goreleaser / multi-arch CI (Plan 9)

**Placeholder check:** No TBDs, no "implement later" phrases, no "similar to Task N" references. Every step has concrete file paths and executable code or commands.

**Type consistency:** `message.Content`, `storage.Session`, `storage.StoredMessage`, `provider.Request`, `agent.RunOptions`, `agent.ConversationResult` — all defined once and used consistently across tasks.
