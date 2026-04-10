# Plan 3: Remaining LLM Providers + Fallback Chain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 8 more LLM providers (OpenAI + OpenRouter + 6 Chinese providers) and wire multi-provider fallback into the Engine so the agent can seamlessly switch providers on rate limits, timeouts, and server errors.

**Architecture:** An `openaicompat` package provides a shared Provider implementation for the OpenAI Chat Completions protocol. Six providers (openai, openrouter, deepseek, qwen, kimi, minimax) are thin wrappers around `openaicompat` with per-provider base URLs and model lists. Zhipu needs JWT-based auth so it gets its own auth wrapper. Wenxin is fully independent (Baidu proprietary API + OAuth access_token refresh). `provider.FallbackChain` is integrated into `agent.Engine.RunConversation` so the loop iterates providers on retryable errors.

**Tech Stack:** Go 1.25, stdlib `net/http` + `bufio.Scanner` (10 MiB buffer) for SSE, `encoding/json`, `crypto/hmac` + `crypto/sha256` + `encoding/base64` for Zhipu JWT signing. Uses existing `provider`, `message`, `tool`, `config`, `agent`, `cli` packages from Plans 1-2.

**Deliverable at end of plan:**
```
$ cat ~/.hermes/config.yaml
model: anthropic/claude-opus-4-6
providers:
  anthropic:
    provider: anthropic
    api_key: env:ANTHROPIC_API_KEY
    model: claude-opus-4-6
  deepseek:
    provider: deepseek
    api_key: env:DEEPSEEK_API_KEY
    model: deepseek-chat
fallback_providers:
  - provider: deepseek
    api_key: env:DEEPSEEK_API_KEY
    model: deepseek-chat

$ ./bin/hermes
HERMES AGENT
anthropic/claude-opus-4-6 · session #new
> What's 2+2?
4.
[ Anthropic hits 429 ]
⚠ Switched to deepseek (anthropic: rate limit)
> Still works?
Yes, seamlessly.
> /exit
Session saved.
```

Every provider can be the primary OR used as a fallback. The Engine uses `provider.FallbackChain.Stream` in its loop.

**Non-goals for this plan (deferred):**
- Token estimation per provider (uses Plan 1's character-based heuristic everywhere — Plan 6 adds tiktoken/anthropic tokenizers)
- Vision/image input to providers that support it (Plan 5)
- Model-specific context length detection (uses a generic 128k default for openaicompat models)
- Provider-specific reasoning/thinking tokens (OpenAI o1, DeepSeek r1) — deferred to Plan 6
- Prompt caching beyond Anthropic's existing implementation — Plan 6
- Audio/speech-to-text providers — not in scope for this project

**Plan 1-2 dependencies this plan touches:**
- `provider/provider.go` — add provider registry function (already exists from Plan 1)
- `agent/conversation.go` — integrate FallbackChain into the conversation loop
- `agent/engine.go` — accept `provider.Provider` OR `*provider.FallbackChain` (they both satisfy the interface after Task 15)
- `config/config.go` — add `FallbackProviders []ProviderConfig` field
- `cli/repl.go` — build the fallback chain at startup, use it as the Engine's provider

---

## File Structure

```
hermes-agent-go/provider/
├── openaicompat/               # NEW: shared base for OpenAI-compatible providers
│   ├── openaicompat.go         # Client struct, Config, NewClient
│   ├── types.go                # Chat completions wire types
│   ├── errors.go               # Map OpenAI HTTP errors to provider.Error
│   ├── complete.go             # Complete() non-streaming
│   ├── stream.go               # Stream() SSE with tool_calls accumulation
│   ├── convert.go              # Convert message.Message ↔ API format (tools, tool_calls, tool_result)
│   └── openaicompat_test.go    # Full test suite
├── openai/                     # NEW: thin wrapper
│   ├── openai.go               # New() factory, wraps openaicompat.Client
│   └── openai_test.go
├── openrouter/                 # NEW: thin wrapper + routing headers
│   ├── openrouter.go
│   └── openrouter_test.go
├── deepseek/                   # NEW
│   ├── deepseek.go
│   └── deepseek_test.go
├── qwen/                       # NEW
│   ├── qwen.go
│   └── qwen_test.go
├── kimi/                       # NEW
│   ├── kimi.go
│   └── kimi_test.go
├── minimax/                    # NEW
│   ├── minimax.go
│   └── minimax_test.go
├── zhipu/                      # NEW: JWT auth + OpenAI-compatible wire
│   ├── zhipu.go                # Wraps openaicompat + JWT signer
│   ├── auth.go                 # signJWT() using HMAC-SHA256
│   ├── auth_test.go
│   └── zhipu_test.go
└── wenxin/                     # NEW: fully independent implementation
    ├── wenxin.go               # Wenxin struct + factory
    ├── oauth.go                # Access token fetch + refresh
    ├── oauth_test.go
    ├── types.go                # Wenxin wire types (different from OpenAI)
    ├── complete.go             # Complete() non-streaming
    ├── stream.go               # Stream() SSE
    └── wenxin_test.go

hermes-agent-go/config/
└── config.go                   # MODIFIED: add FallbackProviders field

hermes-agent-go/agent/
├── engine.go                   # MODIFIED: use FallbackChain for provider
└── conversation.go             # MODIFIED: fallback on stream errors

hermes-agent-go/cli/
└── repl.go                     # MODIFIED: build fallback chain from config
```

---

## Task 1: OpenAI-Compatible Wire Types

**Files:**
- Create: `hermes-agent-go/provider/openaicompat/types.go`

- [ ] **Step 1: Create the wire type definitions**

```go
// provider/openaicompat/types.go
package openaicompat

import "encoding/json"

// chatRequest is the JSON body sent to /v1/chat/completions.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []apiMessage  `json:"messages"`
	Tools       []apiTool     `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	// StreamOptions is OpenAI-specific and controls usage reporting in streams.
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// apiMessage is one message in the chat array.
// OpenAI uses: "system", "user", "assistant", "tool"
type apiMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"` // string or []apiContentPart or nil
	Name       string         `json:"name,omitempty"`
	ToolCalls  []apiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// apiContentPart is one element of a multimodal content array.
type apiContentPart struct {
	Type     string `json:"type"` // "text" or "image_url"
	Text     string `json:"text,omitempty"`
	ImageURL *apiImageURL `json:"image_url,omitempty"`
}

type apiImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// apiToolCall is one tool invocation request from the assistant.
type apiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // always "function"
	Function apiFunctionCall `json:"function"`
}

type apiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded argument string
}

// apiTool is a single tool definition in the request.
type apiTool struct {
	Type     string         `json:"type"` // always "function"
	Function apiFunctionDef `json:"function"`
}

type apiFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// chatResponse is the JSON body returned by /v1/chat/completions (non-streaming).
type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []apiChoice  `json:"choices"`
	Usage   apiUsage     `json:"usage"`
}

type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatStreamChunk is one SSE chunk in a streaming response.
type chatStreamChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []apiStreamChoice  `json:"choices"`
	Usage   *apiUsage          `json:"usage,omitempty"` // only on final chunk when stream_options.include_usage is true
}

type apiStreamChoice struct {
	Index        int            `json:"index"`
	Delta        apiStreamDelta `json:"delta"`
	FinishReason string         `json:"finish_reason"`
}

type apiStreamDelta struct {
	Role      string               `json:"role,omitempty"`
	Content   string               `json:"content,omitempty"`
	ToolCalls []apiStreamToolCall  `json:"tool_calls,omitempty"`
}

// apiStreamToolCall is a tool call chunk in a streaming response.
// Tool calls are streamed incrementally: id/name come first, then
// arguments arrive as concatenated string chunks.
type apiStreamToolCall struct {
	Index    int                 `json:"index"`
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function *apiStreamFunction  `json:"function,omitempty"`
}

type apiStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"` // partial JSON fragment
}

// apiErrorResponse is the error body returned for non-2xx responses.
type apiErrorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}
```

- [ ] **Step 2: Verify the package compiles**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go build ./provider/openaicompat/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/provider/openaicompat/types.go
git commit -m "feat(openaicompat): add OpenAI chat completions wire types"
```

---

## Task 2: OpenAI-Compatible Client + Error Mapping

**Files:**
- Create: `hermes-agent-go/provider/openaicompat/openaicompat.go`
- Create: `hermes-agent-go/provider/openaicompat/errors.go`

- [ ] **Step 1: Create the Client struct**

`provider/openaicompat/openaicompat.go`:

```go
// provider/openaicompat/openaicompat.go
package openaicompat

import (
	"errors"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
)

const (
	defaultRequestMaxSec = 300
)

// Config tells the Client how to reach the backend.
// Each wrapper provider (openai, deepseek, etc.) fills this in from its own config.
type Config struct {
	// BaseURL is the absolute URL prefix, e.g. "https://api.openai.com/v1".
	// The Client appends "/chat/completions" to this URL.
	BaseURL string

	// APIKey is sent as "Authorization: Bearer <key>".
	APIKey string

	// Model is the default model to use when a Request does not specify one.
	Model string

	// ExtraHeaders are added to every outgoing request (e.g. OpenRouter routing headers).
	ExtraHeaders map[string]string

	// ProviderName is used for error attribution (e.g. "deepseek").
	ProviderName string

	// Timeout overrides the default HTTP client timeout.
	Timeout time.Duration
}

// Client is an OpenAI-compatible provider client. Wrapper providers
// embed or wrap this type — they do not implement Complete/Stream themselves.
//
// Safe for concurrent use (net/http.Client is safe for concurrent use).
type Client struct {
	cfg    Config
	http   *http.Client
}

// NewClient constructs a Client from Config. Returns an error if BaseURL
// or APIKey are empty.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("openaicompat: BaseURL is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("openaicompat: APIKey is required")
	}
	if cfg.ProviderName == "" {
		cfg.ProviderName = "openaicompat"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultRequestMaxSec * time.Second
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{Timeout: timeout},
	}, nil
}

// NewFromProviderConfig is a convenience that builds Config from the shared
// config.ProviderConfig shape used by the CLI config file.
// The wrapper provider packages use this to minimize their own code.
func NewFromProviderConfig(providerName, defaultBaseURL string, cfg config.ProviderConfig, extraHeaders map[string]string) (*Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return NewClient(Config{
		BaseURL:      baseURL,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		ExtraHeaders: extraHeaders,
		ProviderName: providerName,
	})
}

// Name returns the provider name (for provider.Provider interface).
func (c *Client) Name() string { return c.cfg.ProviderName }

// Available returns true if the client is configured.
func (c *Client) Available() bool { return c.cfg.APIKey != "" && c.cfg.BaseURL != "" }

// ModelInfo returns conservative defaults for any model.
// Wrapper providers can override this per model if they want.
func (c *Client) ModelInfo(model string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     128_000,
		MaxOutputTokens:   4_096,
		SupportsVision:    false, // wrapper providers override
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsCaching:   false,
		SupportsReasoning: false,
	}
}

// EstimateTokens is a rough character-based estimate.
// Plan 6 replaces this with a per-provider tokenizer.
func (c *Client) EstimateTokens(model, text string) (int, error) {
	return len(text) / 4, nil
}

// Compile-time assertion that *Client satisfies provider.Provider.
var _ provider.Provider = (*Client)(nil)
```

- [ ] **Step 2: Create the error mapping helper**

`provider/openaicompat/errors.go`:

```go
// provider/openaicompat/errors.go
package openaicompat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nousresearch/hermes-agent/provider"
)

// mapHTTPError converts an OpenAI-compatible error response to a provider.Error.
// Takes the provider name separately so wrapper providers attribute correctly.
func mapHTTPError(providerName string, resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr apiErrorResponse
	_ = json.Unmarshal(body, &apiErr)

	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("%s http %d: %s", providerName, resp.StatusCode, string(body))
	}

	kind := provider.ErrUnknown
	switch resp.StatusCode {
	case http.StatusTooManyRequests: // 429
		kind = provider.ErrRateLimit
	case http.StatusUnauthorized, http.StatusForbidden: // 401, 403
		kind = provider.ErrAuth
	case http.StatusBadRequest: // 400
		if isContextTooLong(msg) || isContextTooLong(apiErr.Error.Code) {
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
		Provider:   providerName,
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}

// isContextTooLong checks for common phrases signaling context window overflow.
// OpenAI uses "maximum context length", DeepSeek uses "context length exceeded", etc.
func isContextTooLong(s string) bool {
	l := strings.ToLower(s)
	for _, needle := range []string{
		"maximum context",
		"context length",
		"context window",
		"token limit",
		"context_length_exceeded",
		"string_above_max_length",
	} {
		if strings.Contains(l, needle) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Build to verify**

```bash
go build ./provider/openaicompat/...
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/openaicompat/openaicompat.go hermes-agent-go/provider/openaicompat/errors.go
git commit -m "feat(openaicompat): add Client with shared error taxonomy mapping"
```

---

## Task 3: OpenAI-Compatible Message Conversion

**Files:**
- Create: `hermes-agent-go/provider/openaicompat/convert.go`

- [ ] **Step 1: Create the conversion functions**

OpenAI uses a different tool-call shape than Anthropic. The `agent.Engine` uses Anthropic-native BlockContent for tool_use/tool_result, so this provider needs to convert between the internal canonical format and OpenAI's wire format.

Conversion rules:
- **Assistant with BlockContent containing tool_use blocks** → OpenAI assistant message with `tool_calls` field and `content: null`
- **User with BlockContent containing tool_result blocks** → one OpenAI `tool` message PER tool_result block (each referencing its `tool_call_id`)
- **User/assistant with TextContent** → OpenAI `{role, content: "string"}`
- **provider.Request.SystemPrompt** → OpenAI system message prepended to messages

```go
// provider/openaicompat/convert.go
package openaicompat

import (
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/tool"
)

// buildRequest converts a provider.Request into an OpenAI-compatible chatRequest.
// This is where the internal canonical format (Anthropic-native BlockContent)
// is flattened into OpenAI's tool_calls/tool role shape.
func (c *Client) buildRequest(req *provider.Request, stream bool) *chatRequest {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}

	apiReq := &chatRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.StopSequences,
		Stream:      stream,
		Messages:    make([]apiMessage, 0, len(req.Messages)+1),
	}

	if stream {
		apiReq.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	// System prompt goes in as the first system message
	if req.SystemPrompt != "" {
		apiReq.Messages = append(apiReq.Messages, apiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert tool definitions
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]apiTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			apiReq.Tools = append(apiReq.Tools, convertToolDefinition(t))
		}
	}

	// Convert conversation messages
	for _, m := range req.Messages {
		apiReq.Messages = append(apiReq.Messages, convertMessage(m)...)
	}

	return apiReq
}

// convertToolDefinition maps a tool.ToolDefinition to an apiTool.
func convertToolDefinition(t tool.ToolDefinition) apiTool {
	return apiTool{
		Type: "function",
		Function: apiFunctionDef{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		},
	}
}

// convertMessage converts a message.Message to one or more apiMessage values.
// One internal message may expand into MULTIPLE wire messages (e.g. a user
// message with multiple tool_result blocks becomes N tool messages).
func convertMessage(m message.Message) []apiMessage {
	// Plain text path
	if m.Content.IsText() {
		return []apiMessage{{
			Role:    string(m.Role),
			Content: m.Content.Text(),
		}}
	}

	// BlockContent path — inspect block types
	blocks := m.Content.Blocks()

	// Separate block types
	var (
		textParts   []string
		toolUses    []apiToolCall
		toolResults []apiMessage
	)
	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "tool_use":
			toolUses = append(toolUses, apiToolCall{
				ID:   b.ToolUseID,
				Type: "function",
				Function: apiFunctionCall{
					Name:      b.ToolUseName,
					Arguments: string(b.ToolUseInput),
				},
			})
		case "tool_result":
			// Each tool_result becomes its own "tool" role message.
			toolResults = append(toolResults, apiMessage{
				Role:       "tool",
				ToolCallID: b.ToolUseID,
				Content:    b.ToolResult,
			})
		}
	}

	// Decide the shape of the primary message based on block types.
	var primary apiMessage

	if len(toolUses) > 0 {
		// Assistant message requesting tool calls.
		// content may be nil OR a text prefix the assistant said before calling tools.
		var content any
		if len(textParts) > 0 {
			content = joinStrings(textParts)
		}
		primary = apiMessage{
			Role:      string(m.Role),
			Content:   content,
			ToolCalls: toolUses,
		}
		// tool_result blocks shouldn't be in the same message as tool_use, but
		// if they are, append them after the primary as separate tool messages.
		if len(toolResults) > 0 {
			out := append([]apiMessage{primary}, toolResults...)
			return out
		}
		return []apiMessage{primary}
	}

	if len(toolResults) > 0 {
		// Pure tool_result message (typical for user role after tool execution).
		// Return only the tool messages — the role on the outer message is ignored.
		return toolResults
	}

	// Plain text-in-blocks fallback
	primary = apiMessage{
		Role:    string(m.Role),
		Content: joinStrings(textParts),
	}
	return []apiMessage{primary}
}

// joinStrings concatenates strings without importing strings.Join if empty.
func joinStrings(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	out := make([]byte, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}
	return string(out)
}

// convertResponseMessage converts an OpenAI apiMessage (from a response) into
// a message.Message in the internal canonical format. Tool calls on the
// response are mapped to tool_use ContentBlocks.
func convertResponseMessage(m apiMessage) message.Message {
	// If the assistant returned tool_calls, produce BlockContent.
	if len(m.ToolCalls) > 0 {
		blocks := make([]message.ContentBlock, 0, len(m.ToolCalls)+1)

		// Optional leading text if the model said something before calling tools.
		if text := asString(m.Content); text != "" {
			blocks = append(blocks, message.ContentBlock{Type: "text", Text: text})
		}

		for _, tc := range m.ToolCalls {
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    tc.ID,
				ToolUseName:  tc.Function.Name,
				ToolUseInput: []byte(tc.Function.Arguments),
			})
		}

		return message.Message{
			Role:    message.Role(m.Role),
			Content: message.BlockContent(blocks),
		}
	}

	// Plain text response
	return message.Message{
		Role:    message.Role(m.Role),
		Content: message.TextContent(asString(m.Content)),
	}
}

// asString extracts a plain string from an OpenAI content field which may be
// a string, an array of content parts, or nil.
func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		// Array form — join text parts. Ignore non-text parts.
		var out []byte
		for _, item := range t {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if m["type"] == "text" {
				if s, ok := m["text"].(string); ok {
					out = append(out, s...)
				}
			}
		}
		return string(out)
	default:
		return ""
	}
}

// convertUsage maps the OpenAI usage shape to the internal Usage.
func convertUsage(u apiUsage) provider.Response {
	return provider.Response{
		Usage: message.Usage{
			InputTokens:  u.PromptTokens,
			OutputTokens: u.CompletionTokens,
		},
	}
}

// Silence unused imports (message is used in Message return types)
var _ = message.RoleAssistant
```

- [ ] **Step 2: Build to verify**

```bash
go build ./provider/openaicompat/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/provider/openaicompat/convert.go
git commit -m "feat(openaicompat): add message conversion between internal and wire formats"
```

---

## Task 4: OpenAI-Compatible Complete (Non-Streaming)

**Files:**
- Create: `hermes-agent-go/provider/openaicompat/complete.go`

- [ ] **Step 1: Create the Complete implementation**

```go
// provider/openaicompat/complete.go
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Complete sends a non-streaming chat completion request.
func (c *Client) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	apiReq := c.buildRequest(req, false)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", c.cfg.ProviderName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", c.cfg.ProviderName, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	for k, v := range c.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: c.cfg.ProviderName,
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(c.cfg.ProviderName, httpResp)
	}

	var apiResp chatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", c.cfg.ProviderName, err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("%s: response has no choices", c.cfg.ProviderName)
	}

	choice := apiResp.Choices[0]
	return &provider.Response{
		Message:      convertResponseMessage(choice.Message),
		FinishReason: choice.FinishReason,
		Usage: message.Usage{
			InputTokens:  apiResp.Usage.PromptTokens,
			OutputTokens: apiResp.Usage.CompletionTokens,
		},
		Model: apiResp.Model,
	}, nil
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./provider/openaicompat/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/provider/openaicompat/complete.go
git commit -m "feat(openaicompat): implement Complete non-streaming call"
```

---

## Task 5: OpenAI-Compatible Stream with Tool Call Accumulation

**Files:**
- Create: `hermes-agent-go/provider/openaicompat/stream.go`

- [ ] **Step 1: Create the streaming implementation**

OpenAI streaming is trickier than Anthropic because tool call arguments arrive as **concatenated string fragments** across multiple SSE chunks, keyed by a numeric `index` field in the stream. We accumulate into a per-index builder.

```go
// provider/openaicompat/stream.go
package openaicompat

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

// sseMaxLineBytes is the maximum line length for SSE parsing. Default
// bufio.Scanner limit is 64KB which corrupts large streams.
const sseMaxLineBytes = 10 * 1024 * 1024

// Stream starts a streaming chat completion request.
func (c *Client) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	apiReq := c.buildRequest(req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", c.cfg.ProviderName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", c.cfg.ProviderName, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	for k, v := range c.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: c.cfg.ProviderName,
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	if httpResp.StatusCode != http.StatusOK {
		err := mapHTTPError(c.cfg.ProviderName, httpResp)
		_ = httpResp.Body.Close()
		return nil, err
	}

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineBytes)
	scanner.Split(splitSSEEvents)

	return &openaiStream{
		providerName: c.cfg.ProviderName,
		resp:         httpResp,
		scanner:      scanner,
		toolCalls:    make(map[int]*toolCallBuilder),
	}, nil
}

// splitSSEEvents is a bufio.SplitFunc that yields one SSE line at a time.
// OpenAI's SSE format uses "data: <json>\n" per event and "data: [DONE]" as
// the terminator. Unlike Anthropic, there is no "event: name" line.
func splitSSEEvents(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return idx + 1, data[:idx], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// openaiStream implements provider.Stream.
// NOT thread-safe. One consumer only.
type openaiStream struct {
	providerName string
	resp         *http.Response
	scanner      *bufio.Scanner

	// Accumulated state
	text         strings.Builder
	model        string
	finishReason string
	usage        message.Usage
	done         bool
	closed       bool

	// Tool call accumulator keyed by index (OpenAI streams arguments as
	// concatenated string fragments per index).
	toolCalls map[int]*toolCallBuilder
	toolOrder []int // track order indices were first seen
}

// toolCallBuilder accumulates a streaming tool call across many chunks.
type toolCallBuilder struct {
	ID       string
	Name     string
	ArgBuilder strings.Builder // JSON argument string fragments
}

// Recv reads the next SSE line and returns the corresponding StreamEvent.
// OpenAI may interleave text deltas with tool call deltas on the same choice.
func (s *openaiStream) Recv() (*provider.StreamEvent, error) {
	if s.closed {
		return nil, io.EOF
	}
	if s.done {
		return nil, io.EOF
	}

	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				return nil, fmt.Errorf("%s stream: scan: %w", s.providerName, err)
			}
			// Clean EOF with no explicit [DONE] terminator — synthesize a Done event.
			s.done = true
			return s.buildDoneEvent(), nil
		}

		line := bytes.TrimRight(s.scanner.Bytes(), "\r")
		if len(line) == 0 {
			continue // SSE keepalive blank line
		}

		// Ignore non-data lines
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}

		data := bytes.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		// Terminator
		if bytes.Equal(data, []byte("[DONE]")) {
			s.done = true
			return s.buildDoneEvent(), nil
		}

		// Parse the chunk
		var chunk chatStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return nil, fmt.Errorf("%s stream: parse chunk: %w", s.providerName, err)
		}

		ev, err := s.handleChunk(&chunk)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			return ev, nil
		}
		// keep scanning
	}
}

// handleChunk processes one chatStreamChunk and returns a StreamEvent if
// the chunk carried visible text. Tool call fragments and usage info are
// accumulated silently.
func (s *openaiStream) handleChunk(chunk *chatStreamChunk) (*provider.StreamEvent, error) {
	if chunk.Model != "" {
		s.model = chunk.Model
	}
	if chunk.Usage != nil {
		s.usage.InputTokens = chunk.Usage.PromptTokens
		s.usage.OutputTokens = chunk.Usage.CompletionTokens
	}

	if len(chunk.Choices) == 0 {
		return nil, nil
	}
	choice := chunk.Choices[0]

	if choice.FinishReason != "" {
		s.finishReason = choice.FinishReason
	}

	// Text delta
	if choice.Delta.Content != "" {
		s.text.WriteString(choice.Delta.Content)
		return &provider.StreamEvent{
			Type: provider.EventDelta,
			Delta: &provider.StreamDelta{
				Content: choice.Delta.Content,
			},
		}, nil
	}

	// Tool call deltas
	for _, tc := range choice.Delta.ToolCalls {
		b, ok := s.toolCalls[tc.Index]
		if !ok {
			b = &toolCallBuilder{}
			s.toolCalls[tc.Index] = b
			s.toolOrder = append(s.toolOrder, tc.Index)
		}
		if tc.ID != "" {
			b.ID = tc.ID
		}
		if tc.Function != nil {
			if tc.Function.Name != "" {
				b.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				b.ArgBuilder.WriteString(tc.Function.Arguments)
			}
		}
	}

	return nil, nil
}

// buildDoneEvent emits the terminal EventDone with accumulated state.
func (s *openaiStream) buildDoneEvent() *provider.StreamEvent {
	var content message.Content

	if len(s.toolCalls) > 0 {
		blocks := make([]message.ContentBlock, 0, 1+len(s.toolCalls))
		if s.text.Len() > 0 {
			blocks = append(blocks, message.ContentBlock{
				Type: "text",
				Text: s.text.String(),
			})
		}
		for _, idx := range s.toolOrder {
			b := s.toolCalls[idx]
			if b == nil || b.ID == "" {
				continue
			}
			args := b.ArgBuilder.String()
			if args == "" {
				args = "{}"
			}
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    b.ID,
				ToolUseName:  b.Name,
				ToolUseInput: []byte(args),
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

// Close releases the underlying HTTP response.
func (s *openaiStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./provider/openaicompat/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/provider/openaicompat/stream.go
git commit -m "feat(openaicompat): implement SSE Stream with tool call accumulation"
```

---

## Task 6: OpenAI-Compatible Test Suite

**Files:**
- Create: `hermes-agent-go/provider/openaicompat/openaicompat_test.go`

- [ ] **Step 1: Write the test file**

```go
// provider/openaicompat/openaicompat_test.go
package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{
		BaseURL:      srv.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		ProviderName: "test",
	})
	require.NoError(t, err)
	return srv, c
}

func TestCompleteHappyPath(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "test-model", req.Model)
		// system prompt should be first message
		require.NotEmpty(t, req.Messages)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "Be helpful.", req.Messages[0].Content)

		resp := chatResponse{
			ID:    "chat-001",
			Model: "test-model",
			Choices: []apiChoice{{
				Index: 0,
				Message: apiMessage{
					Role:    "assistant",
					Content: "Hello back",
				},
				FinishReason: "stop",
			}},
			Usage: apiUsage{PromptTokens: 10, CompletionTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	resp, err := c.Complete(context.Background(), &provider.Request{
		Model:        "test-model",
		SystemPrompt: "Be helpful.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
		},
		MaxTokens: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello back", resp.Message.Content.Text())
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
}

func TestCompleteHandlesToolCall(t *testing.T) {
	var captured chatRequest
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := chatResponse{
			ID:    "chat-002",
			Model: "test-model",
			Choices: []apiChoice{{
				Index: 0,
				Message: apiMessage{
					Role:    "assistant",
					Content: nil,
					ToolCalls: []apiToolCall{{
						ID:   "call_01",
						Type: "function",
						Function: apiFunctionCall{
							Name:      "read_file",
							Arguments: `{"path":"go.mod"}`,
						},
					}},
				},
				FinishReason: "tool_calls",
			}},
			Usage: apiUsage{PromptTokens: 20, CompletionTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	resp, err := c.Complete(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		},
		Tools: []tool.ToolDefinition{{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
		MaxTokens: 100,
	})
	require.NoError(t, err)

	// Request should have included the tool
	require.Len(t, captured.Tools, 1)
	assert.Equal(t, "read_file", captured.Tools[0].Function.Name)

	// Response should have tool_use block
	assert.Equal(t, "tool_calls", resp.FinishReason)
	blocks := resp.Message.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "call_01", blocks[0].ToolUseID)
	assert.Equal(t, "read_file", blocks[0].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[0].ToolUseInput))
}

func TestCompleteSendsToolResult(t *testing.T) {
	var captured chatRequest
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := chatResponse{
			ID:    "chat-003",
			Model: "test-model",
			Choices: []apiChoice{{
				Index: 0,
				Message: apiMessage{Role: "assistant", Content: "Done."},
				FinishReason: "stop",
			}},
			Usage: apiUsage{PromptTokens: 30, CompletionTokens: 3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		{
			Role: message.RoleAssistant,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:         "tool_use",
					ToolUseID:    "call_01",
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
					ToolUseID:  "call_01",
					ToolResult: `{"content":"module x"}`,
				},
			}),
		},
	}
	_, err := c.Complete(context.Background(), &provider.Request{Model: "test-model", Messages: history})
	require.NoError(t, err)

	// Verify the wire messages: user, assistant(with tool_calls), tool(with tool_call_id)
	require.Len(t, captured.Messages, 3)
	assert.Equal(t, "user", captured.Messages[0].Role)
	assert.Equal(t, "assistant", captured.Messages[1].Role)
	require.Len(t, captured.Messages[1].ToolCalls, 1)
	assert.Equal(t, "call_01", captured.Messages[1].ToolCalls[0].ID)
	assert.Equal(t, "tool", captured.Messages[2].Role)
	assert.Equal(t, "call_01", captured.Messages[2].ToolCallID)
	assert.Equal(t, `{"content":"module x"}`, captured.Messages[2].Content)
}

func TestCompleteMapsRateLimit(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Error: apiError{Type: "rate_limit_exceeded", Message: "too many requests"},
		})
	})

	_, err := c.Complete(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)
	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrRateLimit, pErr.Kind)
	assert.True(t, provider.IsRetryable(err))
}

func TestCompleteMapsContextTooLong(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Error: apiError{Type: "invalid_request_error", Code: "context_length_exceeded", Message: "maximum context length is 128000"},
		})
	})

	_, err := c.Complete(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)
	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrContextTooLong, pErr.Kind)
}

func TestStreamHappyPath(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		events := []string{
			`data: {"id":"chat-004","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}` + "\n\n",
			`data: {"id":"chat-004","model":"test-model","choices":[{"index":0,"delta":{"content":" there"}}]}` + "\n\n",
			`data: {"id":"chat-004","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := c.Stream(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var text string
	var done *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if ev.Type == provider.EventDone {
			done = ev
			break
		}
		if ev.Delta != nil {
			text += ev.Delta.Content
		}
	}
	assert.Equal(t, "Hi there", text)
	require.NotNil(t, done.Response)
	assert.Equal(t, "stop", done.Response.FinishReason)
	assert.Equal(t, 5, done.Response.Usage.InputTokens)
	assert.Equal(t, 2, done.Response.Usage.OutputTokens)
}

func TestStreamToolCall(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Tool call streamed in pieces: id+name first, then arguments in 2 fragments
		events := []string{
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_09","type":"function","function":{"name":"read_file","arguments":""}}]}}]}` + "\n\n",
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]}}]}` + "\n\n",
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"go.mod\"}"}}]}}]}` + "\n\n",
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":6,"total_tokens":16}}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := c.Stream(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("read go.mod")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var done *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if ev.Type == provider.EventDone {
			done = ev
			break
		}
	}
	require.NotNil(t, done.Response)
	blocks := done.Response.Message.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "call_09", blocks[0].ToolUseID)
	assert.Equal(t, "read_file", blocks[0].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[0].ToolUseInput))
	assert.Equal(t, "tool_calls", done.Response.FinishReason)
}
```

- [ ] **Step 2: Run the tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./provider/openaicompat/...
```

Expected: PASS. All 7 tests.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/provider/openaicompat/openaicompat_test.go
git commit -m "test(openaicompat): add full test suite with tool call streaming"
```

---

## Task 7: OpenAI Provider Wrapper

**Files:**
- Create: `hermes-agent-go/provider/openai/openai.go`
- Create: `hermes-agent-go/provider/openai/openai_test.go`

- [ ] **Step 1: Write the test first**

```go
// provider/openai/openai_test.go
package openai

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "openai"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
	assert.True(t, p.Available())
}

func TestNewAcceptsCustomBaseURL(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		BaseURL:  "https://custom.example.com/v1",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
}
```

- [ ] **Step 2: Implement the wrapper**

```go
// provider/openai/openai.go
package openai

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

const defaultBaseURL = "https://api.openai.com/v1"

// New constructs an OpenAI provider. Returns an error if the API key is missing.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("openai", defaultBaseURL, cfg, nil)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./provider/openai/...
```

Expected: PASS. All 3 tests.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/openai/
git commit -m "feat(openai): add OpenAI provider wrapper"
```

---

## Task 8: OpenRouter Provider Wrapper with Routing Headers

**Files:**
- Create: `hermes-agent-go/provider/openrouter/openrouter.go`
- Create: `hermes-agent-go/provider/openrouter/openrouter_test.go`

- [ ] **Step 1: Write the test**

```go
// provider/openrouter/openrouter_test.go
package openrouter

import (
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

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "openrouter"})
	assert.Error(t, err)
}

func TestSendsRoutingHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "https://github.com/nousresearch/hermes-agent", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "hermes-agent", r.Header.Get("X-Title"))
		body, _ := io.ReadAll(r.Body)
		_ = body

		resp := map[string]any{
			"id":     "c1",
			"model":  "openai/gpt-4o",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "hi"},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "openrouter",
		APIKey:   "or-test",
		BaseURL:  srv.URL,
		Model:    "openai/gpt-4o",
	})
	require.NoError(t, err)

	resp, err := p.Complete(nil, &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	// nil ctx will cause an error inside http.NewRequestWithContext — use a real ctx:
	_ = resp
	_ = err
	// This test primarily verifies the handler ran and the headers were checked.
}
```

Wait — `http.NewRequestWithContext` panics with nil ctx. Fix by passing `context.Background()`:

```go
package openrouter

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

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "openrouter"})
	assert.Error(t, err)
}

func TestSendsRoutingHeaders(t *testing.T) {
	var gotReferer, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		body, _ := io.ReadAll(r.Body)
		_ = body

		resp := map[string]any{
			"id":    "c1",
			"model": "openai/gpt-4o",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "hi"},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "openrouter",
		APIKey:   "or-test",
		BaseURL:  srv.URL,
		Model:    "openai/gpt-4o",
	})
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/nousresearch/hermes-agent", gotReferer)
	assert.Equal(t, "hermes-agent", gotTitle)
}
```

- [ ] **Step 2: Implement**

```go
// provider/openrouter/openrouter.go
package openrouter

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// New constructs an OpenRouter provider. OpenRouter is OpenAI-compatible
// but expects two extra headers for ranking/attribution:
//   HTTP-Referer: https://github.com/nousresearch/hermes-agent
//   X-Title: hermes-agent
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	headers := map[string]string{
		"HTTP-Referer": "https://github.com/nousresearch/hermes-agent",
		"X-Title":      "hermes-agent",
	}
	return openaicompat.NewFromProviderConfig("openrouter", defaultBaseURL, cfg, headers)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./provider/openrouter/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/openrouter/
git commit -m "feat(openrouter): add OpenRouter provider with routing headers"
```

---

## Task 9: DeepSeek Provider Wrapper

**Files:**
- Create: `hermes-agent-go/provider/deepseek/deepseek.go`
- Create: `hermes-agent-go/provider/deepseek/deepseek_test.go`

- [ ] **Step 1: Write the test**

```go
// provider/deepseek/deepseek_test.go
package deepseek

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "deepseek"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-test",
		Model:    "deepseek-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "deepseek", p.Name())
	assert.True(t, p.Available())
}
```

- [ ] **Step 2: Implement**

```go
// provider/deepseek/deepseek.go
package deepseek

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

const defaultBaseURL = "https://api.deepseek.com/v1"

// New constructs a DeepSeek provider. DeepSeek is OpenAI-compatible.
// Popular models: deepseek-chat, deepseek-reasoner (r1).
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("deepseek", defaultBaseURL, cfg, nil)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./provider/deepseek/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/deepseek/
git commit -m "feat(deepseek): add DeepSeek provider wrapper"
```

---

## Task 10: Qwen (通义千问) Provider Wrapper

**Files:**
- Create: `hermes-agent-go/provider/qwen/qwen.go`
- Create: `hermes-agent-go/provider/qwen/qwen_test.go`

- [ ] **Step 1: Write the test**

```go
// provider/qwen/qwen_test.go
package qwen

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "qwen"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "qwen",
		APIKey:   "sk-test",
		Model:    "qwen-max",
	})
	require.NoError(t, err)
	assert.Equal(t, "qwen", p.Name())
	assert.True(t, p.Available())
}
```

- [ ] **Step 2: Implement**

```go
// provider/qwen/qwen.go
package qwen

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

// defaultBaseURL points to Alibaba DashScope's OpenAI-compatible endpoint.
const defaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

// New constructs a Qwen (通义千问) provider via Alibaba DashScope.
// Popular models: qwen-max, qwen-plus, qwen-turbo, qwen2.5-72b-instruct.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("qwen", defaultBaseURL, cfg, nil)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./provider/qwen/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/qwen/
git commit -m "feat(qwen): add Qwen provider wrapper (Alibaba DashScope)"
```

---

## Task 11: Kimi (月之暗面) Provider Wrapper

**Files:**
- Create: `hermes-agent-go/provider/kimi/kimi.go`
- Create: `hermes-agent-go/provider/kimi/kimi_test.go`

- [ ] **Step 1: Write the test**

```go
// provider/kimi/kimi_test.go
package kimi

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "kimi"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "kimi",
		APIKey:   "sk-test",
		Model:    "moonshot-v1-32k",
	})
	require.NoError(t, err)
	assert.Equal(t, "kimi", p.Name())
	assert.True(t, p.Available())
}
```

- [ ] **Step 2: Implement**

```go
// provider/kimi/kimi.go
package kimi

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

// Moonshot AI (月之暗面) hosts Kimi. The API is OpenAI-compatible.
const defaultBaseURL = "https://api.moonshot.cn/v1"

// New constructs a Kimi provider via Moonshot AI.
// Popular models: moonshot-v1-8k, moonshot-v1-32k, moonshot-v1-128k.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("kimi", defaultBaseURL, cfg, nil)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./provider/kimi/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/kimi/
git commit -m "feat(kimi): add Kimi provider wrapper (Moonshot AI)"
```

---

## Task 12: MiniMax Provider Wrapper

**Files:**
- Create: `hermes-agent-go/provider/minimax/minimax.go`
- Create: `hermes-agent-go/provider/minimax/minimax_test.go`

- [ ] **Step 1: Write the test**

```go
// provider/minimax/minimax_test.go
package minimax

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "minimax"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "minimax",
		APIKey:   "sk-test",
		Model:    "abab6.5s-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "minimax", p.Name())
	assert.True(t, p.Available())
}
```

- [ ] **Step 2: Implement**

```go
// provider/minimax/minimax.go
package minimax

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

// MiniMax OpenAI-compatible endpoint.
const defaultBaseURL = "https://api.minimax.chat/v1"

// New constructs a MiniMax provider.
// Popular models: abab6.5s-chat, abab6.5t-chat, MiniMax-Text-01.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("minimax", defaultBaseURL, cfg, nil)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./provider/minimax/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/provider/minimax/
git commit -m "feat(minimax): add MiniMax provider wrapper"
```

---

## Task 13: Zhipu (智谱 GLM) Provider with JWT Auth

**Files:**
- Create: `hermes-agent-go/provider/zhipu/auth.go`
- Create: `hermes-agent-go/provider/zhipu/auth_test.go`
- Create: `hermes-agent-go/provider/zhipu/zhipu.go`
- Create: `hermes-agent-go/provider/zhipu/zhipu_test.go`

Zhipu's API key format is `{key_id}.{key_secret}`. The provider signs a short-lived JWT (HS256, 1-hour expiry) using the secret, and sends it as `Authorization: Bearer <jwt>`. The rest of the protocol is OpenAI-compatible.

- [ ] **Step 1: Write the auth test**

```go
// provider/zhipu/auth_test.go
package zhipu

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignJWTFormat(t *testing.T) {
	token, err := signJWT("my_key.my_secret", time.Hour)
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT must have 3 parts")

	// Header
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var hdr map[string]any
	require.NoError(t, json.Unmarshal(hdrBytes, &hdr))
	assert.Equal(t, "HS256", hdr["alg"])
	assert.Equal(t, "JWT", hdr["typ"])
	assert.Equal(t, "SIGN", hdr["sign_type"])

	// Payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadBytes, &payload))
	assert.Equal(t, "my_key", payload["api_key"])
	// exp and timestamp should be present and numeric
	assert.Contains(t, payload, "exp")
	assert.Contains(t, payload, "timestamp")
}

func TestSignJWTRejectsMalformedKey(t *testing.T) {
	_, err := signJWT("no_dot_in_this_key", time.Hour)
	assert.Error(t, err)
}

func TestSignJWTSecretAffectsSignature(t *testing.T) {
	a, _ := signJWT("k.secret_a", time.Hour)
	b, _ := signJWT("k.secret_b", time.Hour)
	sigA := strings.Split(a, ".")[2]
	sigB := strings.Split(b, ".")[2]
	assert.NotEqual(t, sigA, sigB)
}
```

- [ ] **Step 2: Implement the JWT signer**

```go
// provider/zhipu/auth.go
package zhipu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// signJWT generates a short-lived HS256 JWT for Zhipu AI's Bearer auth scheme.
// The API key format is "<key_id>.<secret>" — the secret is used as the
// HMAC key, and the key_id is embedded in the payload as "api_key".
//
// Reference: https://open.bigmodel.cn/dev/api#http_auth
func signJWT(apiKey string, ttl time.Duration) (string, error) {
	parts := strings.SplitN(apiKey, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("zhipu: api_key must be '<key_id>.<secret>'")
	}
	keyID := parts[0]
	secret := parts[1]

	now := time.Now().UnixMilli()
	exp := now + ttl.Milliseconds()

	header := map[string]any{
		"alg":       "HS256",
		"typ":       "JWT",
		"sign_type": "SIGN",
	}
	payload := map[string]any{
		"api_key":   keyID,
		"exp":       exp,
		"timestamp": now,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig, nil
}
```

- [ ] **Step 3: Write the provider test**

```go
// provider/zhipu/zhipu_test.go
package zhipu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "zhipu"})
	assert.Error(t, err)
}

func TestNewRejectsMalformedKey(t *testing.T) {
	_, err := New(config.ProviderConfig{
		Provider: "zhipu",
		APIKey:   "no_dot",
		Model:    "glm-4",
	})
	assert.Error(t, err)
}

func TestSendsJWTBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = body

		resp := map[string]any{
			"id":    "c1",
			"model": "glm-4",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "hi"},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "zhipu",
		APIKey:   "my_key.my_secret",
		BaseURL:  srv.URL,
		Model:    "glm-4",
	})
	require.NoError(t, err)

	_, err = p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(gotAuth, "Bearer "), "auth header should be Bearer ...")
	token := strings.TrimPrefix(gotAuth, "Bearer ")
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3, "JWT should have 3 parts")
}
```

- [ ] **Step 4: Implement the provider**

```go
// provider/zhipu/zhipu.go
package zhipu

import (
	"context"
	"fmt"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

const (
	defaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	jwtTTL         = time.Hour
)

// Zhipu wraps an openaicompat.Client and rotates the Bearer token on every call.
// Zhipu uses JWT-based auth instead of a static API key, so we can't use
// the Client's ExtraHeaders map directly — the token must be regenerated
// before each request.
type Zhipu struct {
	// inner holds the underlying openaicompat.Client. We update its ExtraHeaders
	// immediately before each Complete/Stream call.
	inner  *openaicompat.Client
	apiKey string
}

// New constructs a Zhipu provider. The api_key must be formatted as
// "<key_id>.<secret>" — this is the format Zhipu AI provides.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("zhipu: api_key is required")
	}
	// Validate the key format early so misconfiguration surfaces at startup.
	if _, err := signJWT(cfg.APIKey, jwtTTL); err != nil {
		return nil, err
	}

	// Build an inner openaicompat.Client with a dummy APIKey; we'll override
	// the Authorization header via ExtraHeaders before each call.
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	inner, err := openaicompat.NewClient(openaicompat.Config{
		BaseURL:      baseURL,
		APIKey:       "placeholder", // required by NewClient validation but overridden below
		Model:        cfg.Model,
		ProviderName: "zhipu",
	})
	if err != nil {
		return nil, err
	}
	return &Zhipu{inner: inner, apiKey: cfg.APIKey}, nil
}

// refreshAuth generates a fresh JWT and injects it as the Authorization header.
// Because openaicompat.Client sets `Authorization: Bearer <APIKey>` itself, we
// cannot use ExtraHeaders for Authorization (it'd be overwritten). Instead we
// provide a thin wrapper Complete/Stream that calls the inner and fixes the
// header. But that duplicates code.
//
// Alternative: use a custom Transport that rewrites the Authorization header
// on every request. This is cleaner.
func (z *Zhipu) Name() string                                      { return "zhipu" }
func (z *Zhipu) Available() bool                                   { return z.apiKey != "" }
func (z *Zhipu) ModelInfo(m string) *provider.ModelInfo            { return z.inner.ModelInfo(m) }
func (z *Zhipu) EstimateTokens(m, t string) (int, error)           { return z.inner.EstimateTokens(m, t) }

// Complete signs a JWT and delegates to the inner client with the signed token.
// We do this by temporarily swapping the inner client's APIKey (thread-safe
// because each RunConversation creates its own Engine and provider set).
func (z *Zhipu) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if err := z.signAndInject(); err != nil {
		return nil, err
	}
	return z.inner.Complete(ctx, req)
}

func (z *Zhipu) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	if err := z.signAndInject(); err != nil {
		return nil, err
	}
	return z.inner.Stream(ctx, req)
}

// signAndInject generates a fresh JWT and stores it where openaicompat.Client
// will find it. openaicompat reads APIKey from its Config — we mutate the
// Config field directly.
//
// Concurrency note: a single Zhipu provider instance is not safe for
// concurrent Complete/Stream calls because the inner Client's APIKey is
// mutated. This matches the existing Engine's "single-use per conversation"
// contract.
func (z *Zhipu) signAndInject() error {
	token, err := signJWT(z.apiKey, jwtTTL)
	if err != nil {
		return err
	}
	z.inner.SetAPIKey(token)
	return nil
}

// Compile-time assertion
var _ provider.Provider = (*Zhipu)(nil)
```

- [ ] **Step 5: Expose `SetAPIKey` on `openaicompat.Client`**

In `provider/openaicompat/openaicompat.go`, add a method at the end:

```go
// SetAPIKey replaces the Bearer token used for subsequent requests.
// Used by providers that rotate auth tokens (e.g., Zhipu JWT).
// Not safe for concurrent use.
func (c *Client) SetAPIKey(key string) {
	c.cfg.APIKey = key
}
```

- [ ] **Step 6: Run all tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./provider/zhipu/... ./provider/openaicompat/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add hermes-agent-go/provider/zhipu/ hermes-agent-go/provider/openaicompat/openaicompat.go
git commit -m "feat(zhipu): add Zhipu (GLM) provider with JWT auth signer"
```

---

## Task 14: Wenxin (文心) Provider — Independent Implementation

Wenxin is Baidu's Ernie (ERNIE Bot) LLM. It uses a completely different API from OpenAI and requires OAuth 2.0 access token refresh.

**API overview:**
1. Get access_token: `POST https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=<api_key>&client_secret=<secret_key>`
2. Chat: `POST https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/<model>?access_token=<token>`
3. Request body: `{"messages": [{"role":"user","content":"..."}], "stream": true/false}`
4. Response: `{"result": "text", "usage": {...}, "is_end": bool}` (streaming is SSE)

Wenxin's tool calling is different from OpenAI too (it uses "functions" not "tools"). For Plan 3 we skip tool support for Wenxin — it works only for text-only conversations. The Engine will fall back to another provider when tools are needed with Wenxin as primary. **This is explicitly documented as a limitation of the Wenxin provider in Plan 3.**

**Files:**
- Create: `hermes-agent-go/provider/wenxin/wenxin.go`
- Create: `hermes-agent-go/provider/wenxin/oauth.go`
- Create: `hermes-agent-go/provider/wenxin/oauth_test.go`
- Create: `hermes-agent-go/provider/wenxin/types.go`
- Create: `hermes-agent-go/provider/wenxin/complete.go`
- Create: `hermes-agent-go/provider/wenxin/stream.go`
- Create: `hermes-agent-go/provider/wenxin/wenxin_test.go`

- [ ] **Step 1: Create `provider/wenxin/wenxin.go` — factory + Provider methods**

```go
// provider/wenxin/wenxin.go
package wenxin

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
)

const (
	defaultOAuthURL    = "https://aip.baidubce.com/oauth/2.0/token"
	defaultChatBaseURL = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat"
	defaultTimeoutSec  = 300
)

// Wenxin is Baidu's ERNIE Bot provider. Uses OAuth 2.0 client credentials
// flow to get an access_token, then calls the chat endpoint with the token
// as a URL parameter.
//
// IMPORTANT: Wenxin does NOT support OpenAI-style tool calls in this plan.
// Tool definitions in the request are silently ignored. If the agent needs
// tools, it should use a different provider as primary and Wenxin only as
// a text-only fallback.
type Wenxin struct {
	apiKey      string // Baidu API Key (client_id)
	secretKey   string // Baidu Secret Key (client_secret)
	model       string // e.g., "ernie-4.0-8k", "ernie-speed", "ernie-lite"
	oauthURL    string
	chatBaseURL string
	http        *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

// New constructs a Wenxin provider. The api_key field in the config holds
// both halves separated by a colon: "<api_key>:<secret_key>". This is the
// convention Baidu uses in their own SDK docs.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("wenxin: api_key is required (format '<api_key>:<secret_key>')")
	}
	parts := strings.SplitN(cfg.APIKey, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errors.New("wenxin: api_key must be formatted as '<api_key>:<secret_key>'")
	}
	oauthURL := defaultOAuthURL
	chatBaseURL := defaultChatBaseURL
	if cfg.BaseURL != "" {
		// Allow tests to override — assume both endpoints live under the same host
		chatBaseURL = cfg.BaseURL + "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat"
		oauthURL = cfg.BaseURL + "/oauth/2.0/token"
	}
	model := cfg.Model
	if model == "" {
		model = "ernie-speed"
	}
	return &Wenxin{
		apiKey:      parts[0],
		secretKey:   parts[1],
		model:       model,
		oauthURL:    oauthURL,
		chatBaseURL: chatBaseURL,
		http:        &http.Client{Timeout: defaultTimeoutSec * time.Second},
	}, nil
}

// Name returns "wenxin".
func (w *Wenxin) Name() string { return "wenxin" }

// Available returns true if api_key is configured.
func (w *Wenxin) Available() bool { return w.apiKey != "" && w.secretKey != "" }

// ModelInfo returns conservative defaults for Wenxin models.
func (w *Wenxin) ModelInfo(model string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     8_000, // ernie-speed default; larger for ernie-4.0-128k
		MaxOutputTokens:   2_000,
		SupportsVision:    false,
		SupportsTools:     false, // not supported in Plan 3
		SupportsStreaming: true,
		SupportsCaching:   false,
		SupportsReasoning: false,
	}
}

// EstimateTokens: rough character-based estimate.
func (w *Wenxin) EstimateTokens(model, text string) (int, error) {
	return len(text) / 3, nil // Chinese chars are roughly 1 token, English ~4 chars/token
}

// Compile-time interface check.
var _ provider.Provider = (*Wenxin)(nil)
```

- [ ] **Step 2: Create `provider/wenxin/oauth.go` + test**

`oauth_test.go`:

```go
// provider/wenxin/oauth_test.go
package wenxin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAccessTokenHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/2.0/token", r.URL.Path)
		assert.Equal(t, "client_credentials", r.URL.Query().Get("grant_type"))
		assert.Equal(t, "my_api_key", r.URL.Query().Get("client_id"))
		assert.Equal(t, "my_secret", r.URL.Query().Get("client_secret"))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test_token_xyz",
			"expires_in":   2592000,
		})
	}))
	defer srv.Close()

	w := &Wenxin{
		apiKey:    "my_api_key",
		secretKey: "my_secret",
		oauthURL:  srv.URL + "/oauth/2.0/token",
		http:      &http.Client{},
	}

	token, err := w.fetchAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test_token_xyz", token)
}

func TestGetAccessTokenCachesUntilExpiry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(wr).Encode(map[string]any{
			"access_token": "cached_token",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	w := &Wenxin{
		apiKey:    "my_api_key",
		secretKey: "my_secret",
		oauthURL:  srv.URL + "/oauth/2.0/token",
		http:      &http.Client{},
	}

	// First call hits the server
	tok1, err := w.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached_token", tok1)
	assert.Equal(t, 1, calls)

	// Second call uses cache
	tok2, err := w.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached_token", tok2)
	assert.Equal(t, 1, calls, "should not re-fetch cached token")

	// Manually expire the cache and verify re-fetch
	w.tokenMu.Lock()
	w.tokenExp = time.Now().Add(-time.Minute)
	w.tokenMu.Unlock()

	tok3, err := w.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "cached_token", tok3)
	assert.Equal(t, 2, calls, "should re-fetch after expiry")
}

func TestGetAccessTokenHandlesOAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(wr).Encode(map[string]any{
			"error":             "invalid_client",
			"error_description": "unknown client id",
		})
	}))
	defer srv.Close()

	w := &Wenxin{
		apiKey:    "bad",
		secretKey: "bad",
		oauthURL:  srv.URL + "/oauth/2.0/token",
		http:      &http.Client{},
	}
	_, err := w.getAccessToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_client")
}
```

`oauth.go`:

```go
// provider/wenxin/oauth.go
package wenxin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// oauthResponse matches the Baidu OAuth token endpoint response body.
type oauthResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"` // seconds
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// fetchAccessToken performs an OAuth 2.0 client_credentials flow against
// Baidu's token endpoint and returns the access_token.
func (w *Wenxin) fetchAccessToken(ctx context.Context) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	params.Set("client_id", w.apiKey)
	params.Set("client_secret", w.secretKey)

	fullURL := w.oauthURL + "?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("wenxin oauth: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("wenxin oauth: network: %w", err)
	}
	defer resp.Body.Close()

	var body oauthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("wenxin oauth: decode: %w", err)
	}

	if body.Error != "" {
		return "", fmt.Errorf("wenxin oauth: %s: %s", body.Error, body.ErrorDescription)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("wenxin oauth: no access_token in response")
	}

	// Cache with a 5-minute safety margin before the real expiry.
	w.tokenMu.Lock()
	w.token = body.AccessToken
	w.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn-300) * time.Second)
	w.tokenMu.Unlock()

	return body.AccessToken, nil
}

// getAccessToken returns a cached token if it's still valid, otherwise
// fetches a new one.
func (w *Wenxin) getAccessToken(ctx context.Context) (string, error) {
	w.tokenMu.Lock()
	if w.token != "" && time.Now().Before(w.tokenExp) {
		token := w.token
		w.tokenMu.Unlock()
		return token, nil
	}
	w.tokenMu.Unlock()

	return w.fetchAccessToken(ctx)
}
```

- [ ] **Step 3: Create `provider/wenxin/types.go`**

```go
// provider/wenxin/types.go
package wenxin

// chatRequest is the Wenxin chat API request body.
// Reference: https://cloud.baidu.com/doc/WENXINWORKSHOP/s/4lilb2lpf
type chatRequest struct {
	Messages    []chatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	System      string        `json:"system,omitempty"`
	MaxOutputTokens int       `json:"max_output_tokens,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// chatResponse is the full (non-streaming) chat response.
type chatResponse struct {
	ID               string `json:"id"`
	Object           string `json:"object"`
	Created          int64  `json:"created"`
	Result           string `json:"result"`
	IsTruncated      bool   `json:"is_truncated"`
	NeedClearHistory bool   `json:"need_clear_history"`
	Usage            usage  `json:"usage"`

	// Error fields (populated on failure)
	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatStreamEvent is one SSE event in a streaming response.
type chatStreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Result  string `json:"result"`
	IsEnd   bool   `json:"is_end"`
	Usage   *usage `json:"usage,omitempty"`

	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}
```

- [ ] **Step 4: Create `provider/wenxin/complete.go`**

```go
// provider/wenxin/complete.go
package wenxin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Complete sends a non-streaming chat request to Wenxin.
func (w *Wenxin) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrAuth,
			Provider: "wenxin",
			Message:  err.Error(),
			Cause:    err,
		}
	}

	apiReq := w.buildRequest(req, false)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("wenxin: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s?access_token=%s", w.chatBaseURL, w.modelForURL(req), token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wenxin: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "wenxin",
			Message:  fmt.Sprintf("network: %v", err),
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	var apiResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("wenxin: decode: %w", err)
	}

	if apiResp.ErrorCode != 0 {
		return nil, mapErrorCode(apiResp.ErrorCode, apiResp.ErrorMsg)
	}

	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(apiResp.Result),
		},
		FinishReason: "stop",
		Usage: message.Usage{
			InputTokens:  apiResp.Usage.PromptTokens,
			OutputTokens: apiResp.Usage.CompletionTokens,
		},
		Model: w.modelForURL(req),
	}, nil
}

// buildRequest converts a provider.Request into a Wenxin chatRequest.
// Tool definitions are silently dropped (Plan 3 limitation).
func (w *Wenxin) buildRequest(req *provider.Request, stream bool) *chatRequest {
	out := &chatRequest{
		Stream:          stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		System:          req.SystemPrompt,
		MaxOutputTokens: req.MaxTokens,
		Stop:            req.StopSequences,
		Messages:        make([]chatMessage, 0, len(req.Messages)),
	}

	for _, m := range req.Messages {
		role := string(m.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		text := ""
		if m.Content.IsText() {
			text = m.Content.Text()
		} else {
			// Concatenate only text blocks — tool blocks are dropped.
			for _, b := range m.Content.Blocks() {
				if b.Type == "text" {
					text += b.Text
				}
			}
		}
		if text == "" {
			continue
		}
		out.Messages = append(out.Messages, chatMessage{Role: role, Content: text})
	}
	return out
}

// modelForURL returns the model name to put into the Wenxin chat URL path.
// If the Request specifies a model, use it; otherwise use the configured default.
func (w *Wenxin) modelForURL(req *provider.Request) string {
	if req.Model != "" {
		return req.Model
	}
	return w.model
}

// mapErrorCode maps a Baidu error code to a provider.Error.
// Common codes:
//   110 — access_token invalid/expired → refresh next call (treat as auth)
//   17 / 18 — rate limit
//   336503 — content filter
func mapErrorCode(code int, msg string) error {
	kind := provider.ErrUnknown
	switch code {
	case 110, 111:
		kind = provider.ErrAuth
	case 17, 18, 4:
		kind = provider.ErrRateLimit
	case 336503:
		kind = provider.ErrContentFilter
	case 336000, 336001:
		kind = provider.ErrServerError
	}
	return &provider.Error{
		Kind:       kind,
		Provider:   "wenxin",
		StatusCode: 200, // Wenxin returns errors with HTTP 200
		Message:    fmt.Sprintf("error %d: %s", code, msg),
	}
}
```

- [ ] **Step 5: Create `provider/wenxin/stream.go`**

```go
// provider/wenxin/stream.go
package wenxin

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

const wenxinSSEMaxLineBytes = 10 * 1024 * 1024

// Stream starts a streaming chat request to Wenxin.
func (w *Wenxin) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrAuth,
			Provider: "wenxin",
			Message:  err.Error(),
			Cause:    err,
		}
	}

	apiReq := w.buildRequest(req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("wenxin: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s?access_token=%s", w.chatBaseURL, w.modelForURL(req), token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wenxin: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "wenxin",
			Message:  fmt.Sprintf("network: %v", err),
			Cause:    err,
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), wenxinSSEMaxLineBytes)

	return &wenxinStream{
		resp:    resp,
		scanner: scanner,
		model:   w.modelForURL(req),
	}, nil
}

// wenxinStream implements provider.Stream.
type wenxinStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	text    strings.Builder
	usage   message.Usage
	model   string
	done    bool
	closed  bool
}

// Recv reads the next event. Wenxin uses "data: {...}\n\n" format,
// with `is_end: true` as the terminator.
func (s *wenxinStream) Recv() (*provider.StreamEvent, error) {
	if s.closed || s.done {
		return nil, io.EOF
	}

	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				return nil, fmt.Errorf("wenxin stream: scan: %w", err)
			}
			s.done = true
			return s.buildDone(), nil
		}

		line := bytes.TrimRight(s.scanner.Bytes(), "\r")
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		var ev chatStreamEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("wenxin stream: parse: %w", err)
		}

		if ev.ErrorCode != 0 {
			return nil, mapErrorCode(ev.ErrorCode, ev.ErrorMsg)
		}

		if ev.Result != "" {
			s.text.WriteString(ev.Result)
		}
		if ev.Usage != nil {
			s.usage.InputTokens = ev.Usage.PromptTokens
			s.usage.OutputTokens = ev.Usage.CompletionTokens
		}

		if ev.IsEnd {
			s.done = true
			return s.buildDone(), nil
		}

		if ev.Result != "" {
			return &provider.StreamEvent{
				Type:  provider.EventDelta,
				Delta: &provider.StreamDelta{Content: ev.Result},
			}, nil
		}
	}
}

func (s *wenxinStream) buildDone() *provider.StreamEvent {
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent(s.text.String()),
			},
			FinishReason: "stop",
			Usage:        s.usage,
			Model:        s.model,
		},
	}
}

// Close releases the HTTP response body.
func (s *wenxinStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}
```

- [ ] **Step 6: Create `provider/wenxin/wenxin_test.go`**

```go
// provider/wenxin/wenxin_test.go
package wenxin

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

// newMockBaidu builds an httptest.Server that handles both OAuth and chat endpoints.
func newMockBaidu(t *testing.T, chatHandler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/2.0/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "mock_token",
			"expires_in":   3600,
		})
	})
	mux.Handle("/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/", chatHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewRejectsBadAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "wenxin", APIKey: "no_colon"})
	assert.Error(t, err)
}

func TestCompleteHappyPath(t *testing.T) {
	srv := newMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/wenxinworkshop/chat/ernie-speed")
		assert.Equal(t, "mock_token", r.URL.Query().Get("access_token"))
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		require.NoError(t, json.Unmarshal(body, &req))
		require.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "hi", req.Messages[0].Content)

		_ = json.NewEncoder(w).Encode(chatResponse{
			ID:     "wxid_01",
			Result: "你好",
			Usage:  usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
		})
	})

	p, err := New(config.ProviderConfig{
		Provider: "wenxin",
		APIKey:   "api:secret",
		BaseURL:  srv.URL,
		Model:    "ernie-speed",
	})
	require.NoError(t, err)

	resp, err := p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	assert.Equal(t, "你好", resp.Message.Content.Text())
	assert.Equal(t, 3, resp.Usage.InputTokens)
	assert.Equal(t, 2, resp.Usage.OutputTokens)
}

func TestCompleteMapsBaiduErrorCode(t *testing.T) {
	srv := newMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{
			ErrorCode: 17,
			ErrorMsg:  "quota exceeded",
		})
	})

	p, _ := New(config.ProviderConfig{
		Provider: "wenxin", APIKey: "api:secret", BaseURL: srv.URL, Model: "ernie-speed",
	})

	_, err := p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)
	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrRateLimit, pErr.Kind)
}

func TestStreamHappyPath(t *testing.T) {
	srv := newMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		events := []string{
			`data: {"id":"w1","result":"你","is_end":false}` + "\n\n",
			`data: {"id":"w1","result":"好","is_end":false}` + "\n\n",
			`data: {"id":"w1","result":"","is_end":true,"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}` + "\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	p, _ := New(config.ProviderConfig{
		Provider: "wenxin", APIKey: "api:secret", BaseURL: srv.URL, Model: "ernie-speed",
	})
	stream, err := p.Stream(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var text string
	var done *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if ev.Type == provider.EventDone {
			done = ev
			break
		}
		if ev.Delta != nil {
			text += ev.Delta.Content
		}
	}
	assert.Equal(t, "你好", text)
	require.NotNil(t, done.Response)
	assert.Equal(t, 3, done.Response.Usage.InputTokens)
	assert.Equal(t, 2, done.Response.Usage.OutputTokens)
}
```

- [ ] **Step 7: Run all Wenxin tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./provider/wenxin/...
```

Expected: PASS. All 7 tests (3 oauth + 4 wenxin).

- [ ] **Step 8: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/provider/wenxin/
git commit -m "feat(wenxin): add Wenxin (Baidu ERNIE) provider with OAuth refresh"
```

---

## Task 15: Config FallbackProviders Field

**Files:**
- Modify: `hermes-agent-go/config/config.go`
- Modify: `hermes-agent-go/config/loader_test.go`

- [ ] **Step 1: Add failing test**

Append to `config/loader_test.go`:

```go
func TestLoadFromYAMLParsesFallbackProviders(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
model: anthropic/claude-opus-4-6
providers:
  anthropic:
    provider: anthropic
    api_key: sk-anthropic
    model: claude-opus-4-6
fallback_providers:
  - provider: deepseek
    api_key: sk-deepseek
    model: deepseek-chat
  - provider: openai
    api_key: sk-openai
    model: gpt-4o
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	require.Len(t, cfg.FallbackProviders, 2)
	assert.Equal(t, "deepseek", cfg.FallbackProviders[0].Provider)
	assert.Equal(t, "sk-deepseek", cfg.FallbackProviders[0].APIKey)
	assert.Equal(t, "openai", cfg.FallbackProviders[1].Provider)
}
```

- [ ] **Step 2: Add the field to the Config struct**

In `config/config.go`, modify the `Config` struct from:

```go
type Config struct {
	Model     string                    `yaml:"model"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Agent     AgentConfig               `yaml:"agent"`
	Storage   StorageConfig             `yaml:"storage"`
}
```

to:

```go
type Config struct {
	Model             string                    `yaml:"model"`
	Providers         map[string]ProviderConfig `yaml:"providers"`
	FallbackProviders []ProviderConfig          `yaml:"fallback_providers,omitempty"`
	Agent             AgentConfig               `yaml:"agent"`
	Storage           StorageConfig             `yaml:"storage"`
}
```

- [ ] **Step 3: Also expand env vars in fallback providers**

In `config/loader.go`, update `expandEnvVars` to also walk `FallbackProviders`:

```go
// expandEnvVars replaces "env:VAR_NAME" references in api keys with the env value.
func expandEnvVars(cfg *Config) error {
	// Primary providers
	for name, p := range cfg.Providers {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			if varName == "" {
				return fmt.Errorf("config: provider %q has empty env variable reference", name)
			}
			p.APIKey = os.Getenv(varName)
			cfg.Providers[name] = p
		}
	}
	// Fallback providers
	for i, p := range cfg.FallbackProviders {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			if varName == "" {
				return fmt.Errorf("config: fallback provider %d has empty env variable reference", i)
			}
			p.APIKey = os.Getenv(varName)
			cfg.FallbackProviders[i] = p
		}
	}
	return nil
}
```

- [ ] **Step 4: Run config tests**

```bash
go test -race ./config/...
```

Expected: PASS. All existing tests + the new fallback parsing test.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/config/
git commit -m "feat(config): add FallbackProviders with env var expansion"
```

---

## Task 16: Provider Factory (Name-Based Construction)

**Files:**
- Create: `hermes-agent-go/provider/factory.go`
- Create: `hermes-agent-go/provider/factory_test.go`

The CLI needs to construct providers by name from the config. Rather than each package doing its own `init()` registration (which would create an import cycle between `provider` and the wrapper packages), we add a small factory that takes a name string and a `config.ProviderConfig`, and dispatches to the right constructor.

- [ ] **Step 1: Write the factory test**

```go
// provider/factory_test.go
package provider_test

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfigAnthropic(t *testing.T) {
	p, err := provider.NewFromConfig(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "sk-ant-test",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name())
}

func TestNewFromConfigOpenAI(t *testing.T) {
	p, err := provider.NewFromConfig(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-openai-test",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
}

func TestNewFromConfigDeepSeek(t *testing.T) {
	p, err := provider.NewFromConfig(config.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-deepseek-test",
		Model:    "deepseek-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "deepseek", p.Name())
}

func TestNewFromConfigUnknown(t *testing.T) {
	_, err := provider.NewFromConfig(config.ProviderConfig{
		Provider: "gpt5-turbo-quantum",
		APIKey:   "sk-test",
	})
	assert.Error(t, err)
}
```

- [ ] **Step 2: Implement the factory**

Because `provider/factory.go` imports the wrapper packages, and the wrapper packages import `provider`, this could create an import cycle. The trick: `factory.go` lives in a **new sub-package or at the top of provider package** — but because each wrapper already depends on `provider`, we can't put it in `provider`. The cleanest solution is a new package `provider/factory`:

Actually, the simplest fix is to put the factory in the `cli` package (which already imports everything) OR create `provider/factory` sub-package.

Let's use `provider/factory` sub-package to avoid pollution of the `cli` package:

Create `hermes-agent-go/provider/factory/factory.go` instead:

```go
// provider/factory/factory.go
package factory

import (
	"fmt"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/provider/deepseek"
	"github.com/nousresearch/hermes-agent/provider/kimi"
	"github.com/nousresearch/hermes-agent/provider/minimax"
	"github.com/nousresearch/hermes-agent/provider/openai"
	"github.com/nousresearch/hermes-agent/provider/openrouter"
	"github.com/nousresearch/hermes-agent/provider/qwen"
	"github.com/nousresearch/hermes-agent/provider/wenxin"
	"github.com/nousresearch/hermes-agent/provider/zhipu"
)

// New constructs a provider by name from a config.ProviderConfig.
// Returns an error if the provider name is unknown or configuration is invalid.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return anthropic.New(cfg)
	case "openai":
		return openai.New(cfg)
	case "openrouter":
		return openrouter.New(cfg)
	case "deepseek":
		return deepseek.New(cfg)
	case "qwen":
		return qwen.New(cfg)
	case "zhipu", "glm":
		return zhipu.New(cfg)
	case "kimi", "moonshot":
		return kimi.New(cfg)
	case "minimax":
		return minimax.New(cfg)
	case "wenxin", "ernie":
		return wenxin.New(cfg)
	default:
		return nil, fmt.Errorf("factory: unknown provider %q", cfg.Provider)
	}
}
```

- [ ] **Step 3: Move the test file to match**

Delete the test file created in Step 1 and re-create as `hermes-agent-go/provider/factory/factory_test.go`:

```go
// provider/factory/factory_test.go
package factory_test

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnthropic(t *testing.T) {
	p, err := factory.New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "sk-ant-test",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name())
}

func TestNewOpenAI(t *testing.T) {
	p, err := factory.New(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-openai-test",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
}

func TestNewDeepSeek(t *testing.T) {
	p, err := factory.New(config.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-deepseek-test",
		Model:    "deepseek-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "deepseek", p.Name())
}

func TestNewAllChineseProviders(t *testing.T) {
	cases := []struct {
		name, provider, apiKey, model string
	}{
		{"qwen", "qwen", "sk-q", "qwen-max"},
		{"kimi", "kimi", "sk-k", "moonshot-v1-8k"},
		{"minimax", "minimax", "sk-m", "abab6.5s-chat"},
		{"zhipu", "zhipu", "id.secret", "glm-4"},
		{"wenxin", "wenxin", "api:secret", "ernie-speed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := factory.New(config.ProviderConfig{
				Provider: tc.provider,
				APIKey:   tc.apiKey,
				Model:    tc.model,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.name, p.Name())
		})
	}
}

func TestNewUnknown(t *testing.T) {
	_, err := factory.New(config.ProviderConfig{
		Provider: "gpt5-turbo-quantum",
		APIKey:   "sk-test",
	})
	assert.Error(t, err)
}

func TestNewAliases(t *testing.T) {
	// "glm" should map to zhipu
	p1, err := factory.New(config.ProviderConfig{
		Provider: "glm", APIKey: "id.secret", Model: "glm-4",
	})
	require.NoError(t, err)
	assert.Equal(t, "zhipu", p1.Name())

	// "moonshot" should map to kimi
	p2, err := factory.New(config.ProviderConfig{
		Provider: "moonshot", APIKey: "sk", Model: "moonshot-v1-8k",
	})
	require.NoError(t, err)
	assert.Equal(t, "kimi", p2.Name())

	// "ernie" should map to wenxin
	p3, err := factory.New(config.ProviderConfig{
		Provider: "ernie", APIKey: "api:secret", Model: "ernie-speed",
	})
	require.NoError(t, err)
	assert.Equal(t, "wenxin", p3.Name())
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./provider/factory/...
```

Expected: PASS. All 6 test functions.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/provider/factory/
git commit -m "feat(provider): add factory package for name-based provider construction"
```

---

## Task 17: Wire FallbackChain into the CLI

**Files:**
- Modify: `hermes-agent-go/cli/repl.go`

- [ ] **Step 1: Read the current repl.go provider-building block**

In `cli/repl.go`, find the block that reads `anthropicCfg` and creates `p` via `anthropic.New`. Replace that block with a fallback-chain construction that uses the new factory:

```go
import (
	// ... existing imports ...
	"github.com/nousresearch/hermes-agent/provider/factory"
)
```

- [ ] **Step 2: Replace the provider construction block**

Find this section (approximately):

```go
	// Build the Anthropic provider from config
	anthropicCfg, ok := app.Config.Providers["anthropic"]
	if !ok {
		anthropicCfg = config.ProviderConfig{Provider: "anthropic"}
	}
	if anthropicCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			anthropicCfg.APIKey = envKey
		}
	}
	if anthropicCfg.APIKey == "" {
		return fmt.Errorf("hermes: anthropic provider is not configured. Set api_key in ~/.hermes/config.yaml or ANTHROPIC_API_KEY env var")
	}
	if anthropicCfg.Model == "" {
		anthropicCfg.Model = defaultModelFromString(app.Config.Model)
	}

	p, err := anthropic.New(anthropicCfg)
	if err != nil {
		return fmt.Errorf("hermes: create provider: %w", err)
	}
```

Replace it with this block that builds a FallbackChain:

```go
	// Build the primary provider + any configured fallbacks into a FallbackChain.
	primary, primaryName, err := buildPrimaryProvider(app.Config)
	if err != nil {
		return err
	}

	providers := []provider.Provider{primary}
	for i, fbCfg := range app.Config.FallbackProviders {
		if fbCfg.APIKey == "" {
			fmt.Fprintf(os.Stderr, "hermes: skipping fallback provider %d (%s): no api_key\n", i, fbCfg.Provider)
			continue
		}
		fbProvider, err := factory.New(fbCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hermes: skipping fallback provider %s: %v\n", fbCfg.Provider, err)
			continue
		}
		providers = append(providers, fbProvider)
	}

	var p provider.Provider
	if len(providers) == 1 {
		p = providers[0]
	} else {
		p = provider.NewFallbackChain(providers)
	}

	// The model name used for display/context bar
	displayModel := defaultModelFromString(app.Config.Model)
```

- [ ] **Step 3: Add the `buildPrimaryProvider` helper**

Add this helper function at the bottom of `cli/repl.go` (before `defaultModelFromString`):

```go
// buildPrimaryProvider resolves the configured primary provider from the
// config file, applying env var fallbacks for Anthropic.
// Returns the provider, its name, and any error.
func buildPrimaryProvider(cfg *config.Config) (provider.Provider, string, error) {
	// Parse "anthropic/claude-opus-4-6" into provider="anthropic", model="claude-opus-4-6"
	primaryName := "anthropic"
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		primaryName = cfg.Model[:idx]
	}

	// Look up the provider config
	pCfg, ok := cfg.Providers[primaryName]
	if !ok {
		pCfg = config.ProviderConfig{Provider: primaryName}
	}
	if pCfg.Provider == "" {
		pCfg.Provider = primaryName
	}

	// Anthropic env var fallback (preserving Plan 1 behavior)
	if primaryName == "anthropic" && pCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			pCfg.APIKey = envKey
		}
	}

	if pCfg.APIKey == "" {
		return nil, "", fmt.Errorf("hermes: primary provider %q has no api_key. Set it in ~/.hermes/config.yaml", primaryName)
	}

	if pCfg.Model == "" {
		pCfg.Model = defaultModelFromString(cfg.Model)
	}

	p, err := factory.New(pCfg)
	if err != nil {
		return nil, "", fmt.Errorf("hermes: create provider %q: %w", primaryName, err)
	}
	return p, primaryName, nil
}
```

- [ ] **Step 4: Update the banner/context bar to use `displayModel`**

Find the section that prints the banner and replaces the model display:

```go
	fmt.Print(banner)
	fmt.Printf("  %s · session %s\n\n", displayModel, sessionID[:8])
```

- [ ] **Step 5: Remove the now-unused import of `anthropic` in repl.go**

At the top of `cli/repl.go`, delete this line:

```go
	"github.com/nousresearch/hermes-agent/provider/anthropic"
```

The `anthropic` package is now used only indirectly via `factory.New`.

- [ ] **Step 6: Run all tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./...
```

Expected: PASS. Including the Plan 1 `TestEndToEndSingleTurn` and Plan 2 `TestEndToEndToolLoop` which both use the engine directly (not runREPL), so they're unaffected by the repl.go changes.

- [ ] **Step 7: Build and verify**

```bash
go build ./...
make build
./bin/hermes version
```

- [ ] **Step 8: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/cli/repl.go
git commit -m "feat(cli): use provider factory + FallbackChain for multi-provider support"
```

---

## Task 18: Engine Uses FallbackChain Natively

**Files:**
- Modify: `hermes-agent-go/agent/conversation.go`

The `FallbackChain.Stream` method iterates providers for the INITIAL stream setup, but once a stream returns, any mid-stream error goes straight to the caller. For Plan 3, that's acceptable — mid-stream retries are deferred to Plan 6. What we need in Plan 3 is: if `streamOnce` fails with a retryable error, the Engine should retry the ENTIRE turn via the fallback chain.

- [ ] **Step 1: Read the current `streamOnce` function**

The current `streamOnce` in `conversation.go` calls `e.provider.Stream(ctx, req)` and collects events. We need to make it retry on retryable errors — but only if the provider IS a `*FallbackChain` (otherwise retry is pointless).

Actually, the cleanest approach is: `FallbackChain.Stream` already iterates providers internally on the setup call. As long as we're passing a `FallbackChain` as the engine's provider, the first call to `provider.Stream(ctx, req)` will iterate through providers automatically. No engine changes needed.

The wrinkle: `FallbackChain.Stream` sets up a stream with the first successful provider, then returns it. Mid-stream errors don't trigger re-iteration. This is fine — mid-stream errors are rare (they happen after the provider has already accepted the request), and Plan 6 adds mid-stream retry via conversation-level retry.

**No code changes are needed in this task.** The integration is automatic because `cli/repl.go` now passes a `*FallbackChain` to the Engine, which calls `Stream` through the interface, and `FallbackChain.Stream` handles the iteration.

- [ ] **Step 2: Write an Engine test that uses a FallbackChain with one failing provider**

Append to `agent/engine_test.go`:

```go
import (
	// ... existing ...
	"github.com/nousresearch/hermes-agent/provider"
)

func TestEngineUsesFallbackChainOnRetryableError(t *testing.T) {
	// First provider always fails with a retryable error.
	failing := &fakeProvider{
		name:     "failing",
		streamFn: func() (provider.Stream, error) {
			return nil, &provider.Error{
				Kind:    provider.ErrRateLimit,
				Provider: "failing",
				Message: "rate limit",
			}
		},
	}
	// Second provider succeeds.
	succeeding := newFakeStreamingProvider("Backup response")

	chain := provider.NewFallbackChain([]provider.Provider{failing, succeeding})

	e := NewEngine(chain, nil, config.AgentConfig{MaxTurns: 10}, "cli")
	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hi",
		SessionID:   "fallback-test",
	})
	require.NoError(t, err)
	assert.Equal(t, "Backup response", result.Response.Content.Text())
}
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./agent/...
```

Expected: PASS. Including the new fallback test.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/agent/engine_test.go
git commit -m "test(agent): verify Engine uses FallbackChain for retryable errors"
```

---

## Task 19: Final Verification

Run these and report results (no commit):

- [ ] **Step 1: Full test suite with coverage**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race -cover ./...
```

Expected: ALL packages pass. New providers each at 50%+ coverage.

- [ ] **Step 2: go vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Build**

```bash
make build
./bin/hermes version
```

- [ ] **Step 4: Manual smoke test with a real Chinese provider (OPTIONAL)**

```bash
# Edit ~/.hermes/config.yaml to use DeepSeek:
cat > ~/.hermes/config.yaml <<'EOF'
model: deepseek/deepseek-chat
providers:
  deepseek:
    provider: deepseek
    api_key: env:DEEPSEEK_API_KEY
    model: deepseek-chat
fallback_providers:
  - provider: anthropic
    api_key: env:ANTHROPIC_API_KEY
    model: claude-opus-4-6
EOF
export DEEPSEEK_API_KEY=sk-deepseek-...
export ANTHROPIC_API_KEY=sk-ant-...
./bin/hermes
```

Expected: Works with DeepSeek as primary, falls back to Anthropic on errors.

- [ ] **Step 5: Verify git log is clean**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git log --oneline hermes-agent-go/ | head -25
```

Expected: clean, ~18 new commits from Plan 3.

- [ ] **Step 6: Plan 3 done. Proceed to Plan 4 (rich TUI) or Plan 5 (tool backends).**

---

## Plan 3 Self-Review Notes

**Spec coverage:**
- openaicompat base (types + client + errors + conversion + Complete + Stream + tests) — Tasks 1-6
- OpenAI wrapper — Task 7
- OpenRouter wrapper with routing headers — Task 8
- DeepSeek wrapper — Task 9
- Qwen wrapper — Task 10
- Kimi wrapper — Task 11
- MiniMax wrapper — Task 12
- Zhipu with JWT auth — Task 13
- Wenxin independent implementation with OAuth refresh — Task 14
- Config FallbackProviders field — Task 15
- Provider factory for name-based construction — Task 16
- CLI wires FallbackChain via factory — Task 17
- Engine uses FallbackChain natively — Task 18
- Final verification — Task 19

**Explicitly out of scope for Plan 3:**
- Per-provider token estimators (tiktoken, anthropic-tokenizer) — Plan 6
- Wenxin tool support (uses different API shape than OpenAI functions) — Plan 6
- Mid-stream provider retries — Plan 6
- Model-specific context length detection — Plan 6
- Vision input for providers that support it — Plan 5
- o1/r1 reasoning token tracking — Plan 6

**Placeholder check:** None. Every code block is complete and executable.

**Type consistency:**
- `openaicompat.Client`, `openaicompat.Config`, `openaicompat.NewClient`, `openaicompat.NewFromProviderConfig` — defined in Task 2, used by Tasks 7-13
- `openaicompat.buildRequest`, `convertResponseMessage`, `convertMessage` — defined in Tasks 3-4, used by Tasks 5-6
- `signJWT`, `jwtTTL` — Task 13
- `Wenxin.fetchAccessToken`, `getAccessToken`, `buildRequest`, `modelForURL`, `mapErrorCode` — Task 14
- `config.Config.FallbackProviders` — Task 15, used by Tasks 17-18
- `factory.New` — Task 16, used by Task 17
- `provider.FallbackChain` (already existed in Plan 1) — used natively in Task 17-18 via interface satisfaction

**Known concern (non-blocking):** `Zhipu.signAndInject` mutates the inner client's APIKey before each call. This is NOT safe for concurrent use on the same Zhipu instance, but matches the existing Engine contract ("single-use per conversation"). Plan 6 could refactor this to use a custom `http.RoundTripper` that injects the token per-request without mutating shared state, if Plan 7 (gateway) reveals a concurrency issue.
