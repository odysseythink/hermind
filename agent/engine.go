// agent/engine.go
package agent

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// Engine is single-use per conversation turn. NOT thread-safe.
// Callers construct a fresh Engine per incoming user message.
type Engine struct {
	provider    provider.Provider
	auxProvider provider.Provider  // optional, used by Compressor
	storage     storage.Storage
	tools       *tool.Registry     // may be nil if no tools are available
	config      config.AgentConfig // value, not pointer — immutable snapshot
	platform    string
	prompt      *PromptBuilder
	compressor  *Compressor // optional, nil means compression disabled

	aux    *provider.AuxClient
	memory *MemoryManager

	// Callbacks — optional. Nil means no-op.
	onStreamDelta func(delta *provider.StreamDelta)
	onToolStart   func(call message.ContentBlock)
	onToolResult  func(call message.ContentBlock, result string)

	// activeSkills returns the skills whose bodies should be prepended to
	// the system prompt. Called once per turn.
	activeSkills func() []ActiveSkill
}

// NewEngine constructs an Engine without tools.
func NewEngine(p provider.Provider, s storage.Storage, cfg config.AgentConfig, platform string) *Engine {
	return NewEngineWithTools(p, s, nil, cfg, platform)
}

// NewEngineWithTools constructs an Engine with tools and no auxiliary provider.
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
		prompt:      NewPromptBuilder(platform, cfg.DefaultSystemPrompt),
	}
	if cfg.Compression.Enabled && aux != nil {
		e.compressor = NewCompressor(cfg.Compression, aux)
	}
	if aux != nil {
		e.aux = provider.NewAuxClient([]provider.Provider{aux, p})
	}
	e.memory = NewMemoryManager(nil)
	if e.aux != nil {
		e.memory.SetAuxClient(e.aux)
	}
	if e.compressor != nil {
		e.memory.SetCompressor(e.compressor)
	}
	return e
}

// Aux returns the engine's auxiliary client, or nil.
func (e *Engine) Aux() *provider.AuxClient { return e.aux }

// Memory returns the engine's memory manager. Always non-nil.
func (e *Engine) Memory() *MemoryManager { return e.memory }

// SetAuxClient overrides the engine's auxiliary client.
func (e *Engine) SetAuxClient(ac *provider.AuxClient) {
	e.aux = ac
	if e.memory != nil {
		e.memory.SetAuxClient(ac)
	}
}

// SetMemoryManager overrides the engine's memory manager.
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

// SetActiveSkillsProvider registers a callback that returns the currently
// active skills.
func (e *Engine) SetActiveSkillsProvider(fn func() []ActiveSkill) {
	e.activeSkills = fn
}

// RunOptions parameterizes a conversation run.
type RunOptions struct {
	UserMessage string
	Model       string

	// Ephemeral runs do not read or write storage. The engine uses only
	// the provided History for context. Used by cron jobs.
	Ephemeral bool

	// History is consulted only when Ephemeral=true. Non-ephemeral runs
	// load history from storage.
	History []message.Message
}

// ConversationResult is returned by RunConversation.
type ConversationResult struct {
	Response   message.Message
	Messages   []message.Message // full history after the run
	Usage      message.Usage
	Iterations int
}
