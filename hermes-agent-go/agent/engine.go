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
