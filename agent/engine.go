// agent/engine.go
package agent

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
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

	// Auxiliary sub-capabilities: cheap LLM calls and composed memory
	// backends. Both are optional; when nil the engine behaves exactly
	// as before. Callers wire them via NewEngineWithToolsAndAux (which
	// constructs an AuxClient and an empty MemoryManager when aux is set)
	// or via SetAuxClient / SetMemoryManager for finer control.
	aux    *provider.AuxClient
	memory *MemoryManager

	// Callbacks — optional. Nil means no-op.
	onStreamDelta    func(delta *provider.StreamDelta)
	onToolStart      func(call message.ContentBlock)                // fired before tool execution
	onToolResult     func(call message.ContentBlock, result string) // fired after
	onSessionCreated func(*storage.Session)                         // fired on first ensureSession that creates a row

	// activeSkills returns the skills whose bodies should be prepended to
	// the system prompt. Called once per turn so the set can change as the
	// user runs /skill-name commands mid-session. Nil means no skills.
	activeSkills func() []ActiveSkill
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
	if aux != nil {
		// Aux client: prefer the auxiliary provider, fall back to the
		// primary model so cheap calls still succeed when the aux side
		// is misconfigured.
		e.aux = provider.NewAuxClient([]provider.Provider{aux, p})
	}
	// MemoryManager is always constructed — even without backends it
	// provides a built-in turn-history digest the engine can use.
	e.memory = NewMemoryManager(nil)
	if e.aux != nil {
		e.memory.SetAuxClient(e.aux)
	}
	if e.compressor != nil {
		e.memory.SetCompressor(e.compressor)
	}
	return e
}

// Aux returns the engine's auxiliary client, or nil if no auxiliary
// provider was supplied. Callers (e.g. MemoryManager, compressor) use
// this for cheap secondary LLM calls.
func (e *Engine) Aux() *provider.AuxClient { return e.aux }

// Memory returns the engine's memory manager. It is always non-nil
// (even without registered backends it provides a turn-history
// digest). Callers may AddProvider or ObserveTurn on it directly.
func (e *Engine) Memory() *MemoryManager { return e.memory }

// SetAuxClient overrides the engine's auxiliary client. Intended for
// callers that build a custom fallback chain (e.g. OpenRouter -> Nous
// -> Codex -> Anthropic) instead of relying on the default two-provider
// chain constructed in NewEngineWithToolsAndAux.
func (e *Engine) SetAuxClient(ac *provider.AuxClient) {
	e.aux = ac
	if e.memory != nil {
		e.memory.SetAuxClient(ac)
	}
}

// SetMemoryManager overrides the engine's memory manager. Intended for
// CLI / gateway bootstrap code that loads config-driven memprovider
// backends and wants to hand the engine a pre-populated manager.
func (e *Engine) SetMemoryManager(mm *MemoryManager) {
	e.memory = mm
	if mm != nil {
		if e.aux != nil {
			mm.SetAuxClient(e.aux)
		}
		if e.compressor != nil {
			mm.SetCompressor(e.compressor)
		}
	}
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

// SetSessionCreatedCallback registers a callback invoked exactly once per
// session, from RunConversation, immediately after ensureSession materializes
// a new row (and AFTER the user message has been persisted). Must be set
// before RunConversation. Calling after is undefined behavior.
//
// The callback runs synchronously on the turn's critical path before the
// first LLM turn. Keep it fast or dispatch heavy work to a goroutine.
func (e *Engine) SetSessionCreatedCallback(fn func(s *storage.Session)) {
	e.onSessionCreated = fn
}

// SetActiveSkillsProvider registers a callback that returns the currently
// active skills. The provider is invoked at the start of each turn and the
// bodies are prepended to the system prompt.
func (e *Engine) SetActiveSkillsProvider(fn func() []ActiveSkill) {
	e.activeSkills = fn
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
