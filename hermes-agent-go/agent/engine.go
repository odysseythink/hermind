// agent/engine.go
package agent

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// Engine is single-use per conversation. NOT thread-safe.
// The gateway creates a fresh Engine per incoming message.
// The CLI creates a fresh Engine per /run invocation.
type Engine struct {
	provider    provider.Provider
	auxProvider provider.Provider  // optional, used by Compressor
	storage     storage.Storage
	tools       *tool.Registry     // may be nil if no tools are available
	config      config.AgentConfig // value, not pointer — immutable snapshot
	platform    string
	prompt      *PromptBuilder
	compressor  *Compressor        // optional, nil means compression disabled

	// Callbacks — optional. Nil means no-op.
	onStreamDelta func(delta *provider.StreamDelta)
	onToolStart   func(call message.ContentBlock)                // fired before tool execution
	onToolResult  func(call message.ContentBlock, result string) // fired after
}

// NewEngine constructs an Engine without tools. Use NewEngineWithTools if
// you want the LLM to be able to invoke tools.
func NewEngine(p provider.Provider, s storage.Storage, cfg config.AgentConfig, platform string) *Engine {
	return NewEngineWithTools(p, s, nil, cfg, platform)
}

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

// SetStreamDeltaCallback registers a callback invoked for each streaming delta.
// Must be called before RunConversation. Calling after is undefined behavior.
func (e *Engine) SetStreamDeltaCallback(fn func(delta *provider.StreamDelta)) {
	e.onStreamDelta = fn
}

// SetToolStartCallback registers a callback invoked before each tool execution.
func (e *Engine) SetToolStartCallback(fn func(call message.ContentBlock)) {
	e.onToolStart = fn
}

// SetToolResultCallback registers a callback invoked after each tool execution.
func (e *Engine) SetToolResultCallback(fn func(call message.ContentBlock, result string)) {
	e.onToolResult = fn
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
